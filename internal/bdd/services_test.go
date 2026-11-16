package bdd_test

import (
	"context"
	"encoding/json"
	"fmt"

	"autoshop/internal/messaging"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"
)

// simulatedServices faz o papel do Execution e do Billing: lê os comandos
// que o orquestrador grava na outbox e responde com os mesmos eventos dos
// serviços reais (contratos de docs/saga-flow.md).
type simulatedServices struct {
	store        *mocks.SagaStoreFake
	orchestrator *usecase.SagaOrchestrator

	stock    map[string]int // estoque por peça
	reserved map[string]map[string]int
	budgets  map[string]string // situação do orçamento por OS

	executionReceived []string
	billingReceived   []string

	ignoreReservation bool // o Execution não responde à reserva
}

func newSimulatedServices(store *mocks.SagaStoreFake, o *usecase.SagaOrchestrator) *simulatedServices {
	return &simulatedServices{
		store: store, orchestrator: o,
		stock: map[string]int{}, reserved: map[string]map[string]int{}, budgets: map[string]string{},
	}
}

// deliver entrega os comandos pendentes aos serviços, até não sobrar nenhum
// (as respostas podem gerar novos comandos).
func (s *simulatedServices) deliver() error {
	for {
		pending, _ := s.store.PendingOutbox(100)
		if len(pending) == 0 {
			return nil
		}
		for _, m := range pending {
			_ = s.store.MarkPublished(m.ID)
			var cmd messaging.Envelope
			if err := json.Unmarshal(m.Body, &cmd); err != nil {
				return err
			}
			if err := s.handle(m.Queue, cmd); err != nil {
				return err
			}
		}
	}
}

func (s *simulatedServices) handle(queue string, cmd messaging.Envelope) error {
	if queue == messaging.QueueExecution {
		s.executionReceived = append(s.executionReceived, cmd.Type)
		return s.handleExecution(cmd)
	}
	s.billingReceived = append(s.billingReceived, cmd.Type)
	return s.handleBilling(cmd)
}

func (s *simulatedServices) handleExecution(cmd messaging.Envelope) error {
	switch cmd.Type {
	case messaging.CmdReserveParts:
		if s.ignoreReservation {
			return nil
		}
		return s.reserve(cmd)
	case messaging.CmdReleaseParts:
		for part, qty := range s.reserved[cmd.OrderID] {
			s.stock[part] += qty
		}
		delete(s.reserved, cmd.OrderID)
		return s.reply(cmd, messaging.EvtPartsReleased, nil)
	case messaging.CmdStartExecution:
		return s.reply(cmd, messaging.EvtExecutionQueued, nil)
	}
	return nil // StartDiagnosis: aguarda o mecânico
}

// reserve reserva todas as peças ou nenhuma, como o Execution Service.
func (s *simulatedServices) reserve(cmd messaging.Envelope) error {
	var p messaging.ReservePartsPayload
	if err := cmd.DecodePayload(&p); err != nil {
		return err
	}
	for _, item := range p.Items {
		if s.stock[item.PartID] < item.Quantity {
			return s.reply(cmd, messaging.EvtPartsReservationFailed, messaging.FailurePayload{
				Reason: fmt.Sprintf("estoque insuficiente da peça %s", item.PartID), PartID: item.PartID,
			})
		}
	}
	s.reserved[cmd.OrderID] = map[string]int{}
	for _, item := range p.Items {
		s.stock[item.PartID] -= item.Quantity
		s.reserved[cmd.OrderID][item.PartID] += item.Quantity
	}
	return s.reply(cmd, messaging.EvtPartsReserved, nil)
}

func (s *simulatedServices) handleBilling(cmd messaging.Envelope) error {
	switch cmd.Type {
	case messaging.CmdGenerateBudget:
		s.budgets[cmd.OrderID] = "gerado"
		return s.reply(cmd, messaging.EvtBudgetGenerated, nil)
	case messaging.CmdCancelBudget:
		s.budgets[cmd.OrderID] = "cancelado"
		return s.reply(cmd, messaging.EvtBudgetCancelled, nil)
	case messaging.CmdRequestPayment:
		return s.reply(cmd, messaging.EvtPaymentRequested, messaging.PaymentRequestedPayload{
			CheckoutURL: "https://pagamento.exemplo/" + cmd.OrderID, Amount: 290,
		})
	case messaging.CmdRefundPayment:
		return s.reply(cmd, messaging.EvtPaymentRefunded, nil)
	}
	return nil
}

// reply envia um evento ao orquestrador, como se viesse da fila os-events.
func (s *simulatedServices) reply(cmd messaging.Envelope, eventType string, payload any) error {
	return s.publish(cmd.SagaID, cmd.OrderID, eventType, payload)
}

func (s *simulatedServices) publish(sagaID, orderID, eventType string, payload any) error {
	env, err := messaging.NewMessage(eventType, sagaID, orderID, "corr-bdd", payload)
	if err != nil {
		return err
	}
	return s.orchestrator.HandleEvent(context.Background(), env)
}
