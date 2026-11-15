package integration_test

import (
	"testing"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newOrderWithSaga grava uma OS nova junto com a Saga e um comando na outbox.
func newOrderWithSaga(t *testing.T, cpf, plate string) (domain.Order, domain.Saga, domain.OutboxMessage) {
	t.Helper()
	customerID, vehicleID, serviceID := setupOrderData(t, cpf, plate)
	customer, _ := customerUseCase.GetByID(customerID)
	vehicle, _ := vehicleUseCase.GetByID(vehicleID)
	now := time.Now().UTC().Format(time.RFC3339)

	order := domain.Order{
		ID: uuid.NewString(), CustomerID: customerID, CustomerName: customer.Name,
		VehicleID: vehicleID, VehiclePlate: vehicle.Plate, Status: domain.StatusReceived,
		Services:  []domain.OrderService{{ServiceID: serviceID, ServiceName: "Troca de óleo", Price: 150}},
		CreatedAt: now, UpdatedAt: now,
	}
	order.CalculateTotals()
	saga := domain.NewSaga(uuid.NewString(), order.ID, "corr-"+order.ID[:8])
	msg := domain.OutboxMessage{
		ID: uuid.NewString(), Queue: "execution-commands", Type: "StartDiagnosis",
		Body: []byte(`{"type":"StartDiagnosis"}`), CreatedAt: time.Now().UTC(),
	}

	_, err := sagaRepo.CreateOrderWithSaga(order, saga, []domain.OutboxMessage{msg})
	require.NoError(t, err)
	return order, saga, msg
}

func pendingIDs(t *testing.T) map[string]domain.OutboxMessage {
	t.Helper()
	msgs, err := sagaRepo.PendingOutbox(1000)
	require.NoError(t, err)
	ids := map[string]domain.OutboxMessage{}
	for _, m := range msgs {
		ids[m.ID] = m
	}
	return ids
}

func TestSagaIntegration_CreateOrderWithSagaAndOutbox(t *testing.T) {
	order, saga, msg := newOrderWithSaga(t, "412.356.781-99", "SAG1A01")

	savedOrder, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, 150.0, savedOrder.Total)

	savedSaga, err := sagaRepo.FindByOrderID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, saga.ID, savedSaga.ID)
	assert.Equal(t, domain.StepDiagnosis, savedSaga.CurrentStep)
	assert.Equal(t, domain.SagaRunning, savedSaga.Status)
	assert.Empty(t, savedSaga.CompletedSteps)
	assert.Equal(t, saga.CorrelationID, savedSaga.CorrelationID)

	pending := pendingIDs(t)
	require.Contains(t, pending, msg.ID)
	assert.Equal(t, "execution-commands", pending[msg.ID].Queue)
	assert.JSONEq(t, `{"type":"StartDiagnosis"}`, string(pending[msg.ID].Body))

	require.NoError(t, sagaRepo.MarkPublished(msg.ID))
	assert.NotContains(t, pendingIDs(t), msg.ID)
}

func TestSagaIntegration_SaveIsAtomicAndIdempotent(t *testing.T) {
	order, _, _ := newOrderWithSaga(t, "523.467.892-08", "SAG1A02")
	saga, err := sagaRepo.FindByOrderID(order.ID)
	require.NoError(t, err)

	// Diagnóstico concluído: OS avança, Saga vai para o orçamento, comando na outbox
	require.NoError(t, order.TransitionTo(domain.StatusDiagnosis))
	order.Diagnosis = "Troca das pastilhas de freio"
	saga.Advance(domain.StepBudget, 5*time.Minute)
	cmd := domain.OutboxMessage{ID: uuid.NewString(), Queue: "billing-commands", Type: "GenerateBudget", Body: []byte(`{}`), CreatedAt: time.Now().UTC()}
	eventID := uuid.NewString()

	require.NoError(t, sagaRepo.Save(domain.SagaChange{Order: &order, Saga: saga, Outbox: []domain.OutboxMessage{cmd}, ProcessedMessageID: eventID}))

	savedOrder, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDiagnosis, savedOrder.Status)
	assert.Equal(t, "Troca das pastilhas de freio", savedOrder.Diagnosis)
	assert.NotEmpty(t, savedOrder.DiagnosisAt)

	savedSaga, err := sagaRepo.FindByOrderID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StepBudget, savedSaga.CurrentStep)
	assert.Equal(t, []domain.SagaStep{domain.StepDiagnosis}, savedSaga.CompletedSteps)
	assert.NotNil(t, savedSaga.DeadlineAt)
	assert.Equal(t, 2, savedSaga.Version)
	assert.Contains(t, pendingIDs(t), cmd.ID)

	processed, err := sagaRepo.IsProcessed(eventID)
	require.NoError(t, err)
	assert.True(t, processed)

	// O mesmo evento de novo: nada é gravado
	repeated := domain.OutboxMessage{ID: uuid.NewString(), Queue: "billing-commands", Type: "GenerateBudget", Body: []byte(`{}`), CreatedAt: time.Now().UTC()}
	savedSaga.Advance(domain.StepApproval, 0)
	err = sagaRepo.Save(domain.SagaChange{Saga: savedSaga, Outbox: []domain.OutboxMessage{repeated}, ProcessedMessageID: eventID})
	assert.ErrorIs(t, err, repository.ErrMessageAlreadyProcessed)
	assert.NotContains(t, pendingIDs(t), repeated.ID)

	// Gravação baseada numa leitura antiga é recusada
	saga.Advance(domain.StepApproval, 0) // saga ainda com Version 1
	err = sagaRepo.Save(domain.SagaChange{Saga: saga})
	assert.ErrorIs(t, err, repository.ErrSagaConcurrentUpdate)
}

func TestSagaIntegration_CompensationAndCancellation(t *testing.T) {
	order, _, _ := newOrderWithSaga(t, "634.578.913-54", "SAG1A03")
	saga, err := sagaRepo.FindByOrderID(order.ID)
	require.NoError(t, err)

	saga.Advance(domain.StepBudget, 0)
	saga.Advance(domain.StepApproval, 0)
	saga.StartCompensation("cliente recusou", false)
	require.NoError(t, order.TransitionTo(domain.StatusCancelled))
	order.CancellationReason = "cliente recusou"
	require.NoError(t, sagaRepo.Save(domain.SagaChange{Order: &order, Saga: saga}))

	savedSaga, err := sagaRepo.FindByOrderID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SagaCompensating, savedSaga.Status)
	assert.Equal(t, []domain.SagaStep{domain.StepBudget}, savedSaga.PendingCompensations)
	assert.Equal(t, "cliente recusou", savedSaga.FailureReason)

	savedOrder, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, savedOrder.Status)
	assert.Equal(t, "cliente recusou", savedOrder.CancellationReason)
}

func TestSagaIntegration_FindExpired(t *testing.T) {
	expiredOrder, _, _ := newOrderWithSaga(t, "745.689.124-90", "SAG1A04")
	validOrder, _, _ := newOrderWithSaga(t, "856.791.235-09", "SAG1A05")

	expired, err := sagaRepo.FindByOrderID(expiredOrder.ID)
	require.NoError(t, err)
	expired.Advance(domain.StepBudget, time.Millisecond)
	require.NoError(t, sagaRepo.Save(domain.SagaChange{Saga: expired}))

	valid, err := sagaRepo.FindByOrderID(validOrder.ID)
	require.NoError(t, err)
	valid.Advance(domain.StepBudget, time.Hour)
	require.NoError(t, sagaRepo.Save(domain.SagaChange{Saga: valid}))

	result, err := sagaRepo.FindExpired(time.Now().Add(time.Second))
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, s := range result {
		ids[s.OrderID] = true
	}
	assert.True(t, ids[expiredOrder.ID])
	assert.False(t, ids[validOrder.ID])
}

func TestSagaIntegration_NotFound(t *testing.T) {
	_, err := sagaRepo.FindByOrderID(uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrSagaNotFound)
}

// As datas de diagnóstico e de espera da aprovação agora são gravadas, e o
// tempo médio por status passa a ser calculado a partir do banco.
func TestSagaIntegration_AverageTimeByStatusFromDatabase(t *testing.T) {
	order, _, _ := newOrderWithSaga(t, "967.812.346-09", "SAG1A06")
	advance(t, order.ID, domain.StatusDiagnosis, domain.StatusWaitingApproval)

	saved, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.DiagnosisAt)
	assert.NotEmpty(t, saved.WaitingApprovalAt)

	averages, err := orderUseCase.GetAverageTimeByStatus()
	require.NoError(t, err)
	assert.Contains(t, averages, "Diagnóstico")
}
