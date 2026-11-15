package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandsFor devolve os comandos pendentes na outbox para uma OS, em ordem.
func commandsFor(t *testing.T, orderID string) []string {
	t.Helper()
	msgs, err := sagaRepo.PendingOutbox(1000)
	require.NoError(t, err)
	types := []string{}
	for _, m := range msgs {
		var env messaging.Envelope
		require.NoError(t, json.Unmarshal(m.Body, &env))
		if env.OrderID == orderID {
			types = append(types, m.Type)
		}
	}
	return types
}

// sagaOrderUseCase monta o caso de uso de OS com a Saga ligada.
func sagaOrderUseCase(o *usecase.SagaOrchestrator) *usecase.OrderUseCase {
	return usecase.NewOrderUseCase(orderRepo,
		repository.NewCustomerPostgresRepository(pool),
		repository.NewVehiclePostgresRepository(pool),
		repository.NewServicePostgresRepository(pool),
		fakeCatalog{}).WithSaga(o)
}

func sendEvent(t *testing.T, o *usecase.SagaOrchestrator, orderID, evtType string, payload any) {
	t.Helper()
	env, err := messaging.NewMessage(evtType, "", orderID, "corr-integracao", payload)
	require.NoError(t, err)
	require.NoError(t, o.HandleEvent(context.Background(), env))
}

// O orquestrador real, gravando no Postgres: abertura da OS pela Saga,
// orçamento, aprovação sem peças e falha no pagamento com compensação.
func TestSagaOrchestratorIntegration_FlowWithCompensation(t *testing.T) {
	orchestrator := usecase.NewSagaOrchestrator(orderRepo, sagaRepo, usecase.SagaConfig{StepTimeout: time.Minute})
	uc := sagaOrderUseCase(orchestrator)

	customerID, vehicleID, serviceID := setupOrderData(t, "526.018.159-06", "SAG1A07")
	created, err := uc.Create(customerID, vehicleID, "", []string{serviceID}, nil)
	require.NoError(t, err)

	saved, err := uc.GetByID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDiagnosis, saved.Status)
	assert.NotEmpty(t, saved.DiagnosisAt, "a OS aberta pela Saga já nasce em diagnóstico")
	assert.Equal(t, []string{messaging.CmdStartDiagnosis}, commandsFor(t, created.ID))

	sendEvent(t, orchestrator, created.ID, messaging.EvtDiagnosisFinished, messaging.DiagnosisFinishedPayload{Diagnosis: "Óleo vencido"})
	sendEvent(t, orchestrator, created.ID, messaging.EvtBudgetGenerated, nil)

	saved, _ = uc.GetByID(created.ID)
	assert.Equal(t, domain.StatusWaitingApproval, saved.Status)
	assert.Equal(t, "Óleo vencido", saved.Diagnosis)

	_, err = uc.ApproveOrder(created.ID, true, "")
	require.NoError(t, err)
	sendEvent(t, orchestrator, created.ID, messaging.EvtPaymentFailed, messaging.FailurePayload{Reason: "pagamento expirado"})

	saved, _ = uc.GetByID(created.ID)
	assert.Equal(t, domain.StatusCancelled, saved.Status)
	assert.Equal(t, "pagamento expirado", saved.CancellationReason)
	assert.Equal(t, []string{
		messaging.CmdStartDiagnosis, messaging.CmdGenerateBudget, messaging.CmdRequestPayment, messaging.CmdCancelBudget,
	}, commandsFor(t, created.ID))

	saga, err := sagaRepo.FindByOrderID(created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SagaCompensating, saga.Status)

	sendEvent(t, orchestrator, created.ID, messaging.EvtBudgetCancelled, nil)
	saga, _ = sagaRepo.FindByOrderID(created.ID)
	assert.Equal(t, domain.SagaCancelled, saga.Status)
}

// Prazo esgotado: o passo atual também é desfeito.
func TestSagaOrchestratorIntegration_Timeout(t *testing.T) {
	orchestrator := usecase.NewSagaOrchestrator(orderRepo, sagaRepo, usecase.SagaConfig{StepTimeout: time.Minute})
	uc := sagaOrderUseCase(orchestrator)

	customerID, vehicleID, serviceID := setupOrderData(t, "083.016.613-05", "SAG1A08")
	created, err := uc.Create(customerID, vehicleID, "", []string{serviceID}, nil)
	require.NoError(t, err)
	sendEvent(t, orchestrator, created.ID, messaging.EvtDiagnosisFinished, nil)

	_, err = orchestrator.CheckTimeouts(time.Now().Add(2 * time.Minute))
	require.NoError(t, err)

	saved, _ := uc.GetByID(created.ID)
	assert.Equal(t, domain.StatusCancelled, saved.Status)
	assert.Contains(t, saved.CancellationReason, "orcamento")
	cmds := commandsFor(t, created.ID)
	assert.Equal(t, messaging.CmdCancelBudget, cmds[len(cmds)-1])
}
