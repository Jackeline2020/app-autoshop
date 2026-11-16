// Package bdd_test descreve o fluxo completo da ordem de serviço em
// cenários BDD (features/saga.feature): o orquestrador real conduz a OS e o
// Execution e o Billing são simulados, respondendo pelos mesmos contratos
// de mensagem dos serviços reais.
package bdd_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"autoshop/internal/client"
	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"

	"github.com/cucumber/godog"
	"github.com/stretchr/testify/mock"
)

const stepTimeout = 5 * time.Minute

func TestSagaFeatures(t *testing.T) {
	suite := godog.TestSuite{
		Name:                "saga",
		ScenarioInitializer: initializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("cenários BDD com falha")
	}
}

type partCatalog struct{}

func (partCatalog) FindPart(_ context.Context, id string) (client.CatalogPart, error) {
	return client.CatalogPart{ID: id, Name: "Pastilha de freio", Price: 95}, nil
}

// world guarda o estado de um cenário.
type world struct {
	store        *mocks.SagaStoreFake
	orchestrator *usecase.SagaOrchestrator
	orders       *usecase.OrderUseCase
	services     *simulatedServices
	orderID      string
}

func newWorld() *world {
	store := mocks.NewSagaStoreFake()
	orchestrator := usecase.NewSagaOrchestrator(store, store, usecase.SagaConfig{StepTimeout: stepTimeout})

	customers := new(mocks.CustomerRepositoryMock)
	customers.On("FindByID", mock.Anything).Return(domain.Customer{ID: "cliente-1", Name: "Ana Souza"}, nil)
	vehicles := new(mocks.VehicleRepositoryMock)
	vehicles.On("FindByID", mock.Anything).Return(domain.Vehicle{ID: "veiculo-1", Plate: "BDD1A23"}, nil)
	services := new(mocks.ServiceRepositoryMock)
	services.On("FindByID", mock.Anything).Return(domain.Service{ID: "servico-1", Name: "Troca de pastilhas", Price: 100}, nil)

	return &world{
		store:        store,
		orchestrator: orchestrator,
		orders:       usecase.NewOrderUseCase(store, customers, vehicles, services, partCatalog{}).WithSaga(orchestrator),
		services:     newSimulatedServices(store, orchestrator),
	}
}

func (w *world) order() domain.Order { return w.store.Orders[w.orderID] }
func (w *world) saga() domain.Saga   { return w.store.Sagas[w.orderID] }

// --- Dado ---

func (w *world) partInStock(part string, qty int) error {
	w.services.stock[part] = qty
	return nil
}

func (w *world) executionIgnoresReservation() error {
	w.services.ignoreReservation = true
	return nil
}

func (w *world) openOrder(_ string, qty int, part string) error {
	created, err := w.orders.Create("cliente-1", "veiculo-1", "", []string{"servico-1"},
		[]struct {
			PartID   string
			Quantity int
		}{{PartID: part, Quantity: qty}}, "corr-bdd")
	if err != nil {
		return err
	}
	w.orderID = created.ID
	return w.services.deliver()
}

// --- Quando ---

func (w *world) mechanicDiagnoses(diagnosis string) error {
	return w.mechanicEvent(messaging.EvtDiagnosisFinished, messaging.DiagnosisFinishedPayload{Diagnosis: diagnosis})
}

func (w *world) mechanicEvent(eventType string, payload any) error {
	if err := w.services.publish(w.saga().ID, w.orderID, eventType, payload); err != nil {
		return err
	}
	return w.services.deliver()
}

func (w *world) customerAnswers(approved bool) error {
	if _, err := w.orders.ApproveOrder(w.orderID, approved, "valor acima do esperado"); err != nil {
		return err
	}
	return w.services.deliver()
}

func (w *world) payment(approved bool) error {
	if approved {
		return w.mechanicEvent(messaging.EvtPaymentConfirmed, nil)
	}
	return w.mechanicEvent(messaging.EvtPaymentFailed, messaging.FailurePayload{Reason: "pagamento recusado", Status: "recusado"})
}

func (w *world) deadlinePasses() error {
	if _, err := w.orchestrator.CheckTimeouts(time.Now().Add(stepTimeout + time.Minute)); err != nil {
		return err
	}
	return w.services.deliver()
}

// --- Então ---

func (w *world) orderStatusIs(status string) error {
	if got := string(w.order().Status); got != status {
		return fmt.Errorf("status da OS: esperado %q, obtido %q (motivo: %q)", status, got, w.order().CancellationReason)
	}
	return nil
}

func (w *world) orderWaitingPaymentWithLink(status string) error {
	if err := w.orderStatusIs(status); err != nil {
		return err
	}
	if w.order().CheckoutURL == "" {
		return fmt.Errorf("a OS deveria ter o link de pagamento")
	}
	return nil
}

func (w *world) sagaStatusIs(status string) error {
	if got := string(w.saga().Status); got != status {
		return fmt.Errorf("status da Saga: esperado %q, obtido %q", status, got)
	}
	return nil
}

func (w *world) stockIs(part string, qty int) error {
	if got := w.services.stock[part]; got != qty {
		return fmt.Errorf("estoque da peça %s: esperado %d, obtido %d", part, qty, got)
	}
	return nil
}

func (w *world) budgetCancelled() error {
	if got := w.services.budgets[w.orderID]; got != "cancelado" {
		return fmt.Errorf("orçamento deveria estar cancelado, está %q", got)
	}
	return nil
}

func received(service string, list []string, command string) error {
	if !slices.Contains(list, command) {
		return fmt.Errorf("o %s deveria ter recebido %s; recebeu %v", service, command, list)
	}
	return nil
}

func initializeScenario(sc *godog.ScenarioContext) {
	var w *world
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		w = newWorld()
		return ctx, nil
	})

	sc.Step(`^que a peça "([^"]*)" tem (\d+) unidades em estoque$`, func(p string, q int) error { return w.partInStock(p, q) })
	sc.Step(`^que o Execution Service não responde à reserva de peças$`, func() error { return w.executionIgnoresReservation() })
	sc.Step(`^uma OS aberta com o serviço "([^"]*)" e (\d+) unidades da peça "([^"]*)"$`, func(s string, q int, p string) error { return w.openOrder(s, q, p) })

	sc.Step(`^o mecânico registra o diagnóstico "([^"]*)"$`, func(d string) error { return w.mechanicDiagnoses(d) })
	sc.Step(`^o cliente aprova o orçamento$`, func() error { return w.customerAnswers(true) })
	sc.Step(`^o cliente recusa o orçamento$`, func() error { return w.customerAnswers(false) })
	sc.Step(`^o pagamento é aprovado$`, func() error { return w.payment(true) })
	sc.Step(`^o pagamento é recusado$`, func() error { return w.payment(false) })
	sc.Step(`^o mecânico inicia o reparo$`, func() error { return w.mechanicEvent(messaging.EvtExecutionStarted, nil) })
	sc.Step(`^o mecânico finaliza o reparo$`, func() error { return w.mechanicEvent(messaging.EvtExecutionFinished, nil) })
	sc.Step(`^o prazo do passo se esgota$`, func() error { return w.deadlinePasses() })

	sc.Step(`^a OS deve estar "([^"]*)"$`, func(s string) error { return w.orderStatusIs(s) })
	sc.Step(`^a OS deve estar "([^"]*)" com o link de pagamento$`, func(s string) error { return w.orderWaitingPaymentWithLink(s) })
	sc.Step(`^a Saga deve estar "([^"]*)"$`, func(s string) error { return w.sagaStatusIs(s) })
	sc.Step(`^a peça "([^"]*)" deve ter (\d+) unidades em estoque$`, func(p string, q int) error { return w.stockIs(p, q) })
	sc.Step(`^o orçamento deve estar cancelado$`, func() error { return w.budgetCancelled() })
	sc.Step(`^o Execution Service deve ter recebido "([^"]*)"$`, func(c string) error { return received("Execution Service", w.services.executionReceived, c) })
	sc.Step(`^o Billing Service deve ter recebido "([^"]*)"$`, func(c string) error { return received("Billing Service", w.services.billingReceived, c) })
}
