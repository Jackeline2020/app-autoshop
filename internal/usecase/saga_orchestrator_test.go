package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/mocks"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sagaOrder(withParts bool) domain.Order {
	o := domain.Order{
		ID:       "os-1",
		Status:   domain.StatusReceived,
		Services: []domain.OrderService{{ServiceID: "s1", ServiceName: "Troca de óleo", Price: 100}},
	}
	if withParts {
		o.Parts = []domain.OrderPart{{PartID: "p1", PartName: "Filtro", Quantity: 2, UnitPrice: 25}}
	}
	o.CalculateTotals()
	return o
}

func startSaga(t *testing.T, withParts bool) (*usecase.SagaOrchestrator, *mocks.SagaStoreFake) {
	store := mocks.NewSagaStoreFake()
	o := usecase.NewSagaOrchestrator(store, store, usecase.SagaConfig{StepTimeout: time.Minute})
	created, err := o.Start(sagaOrder(withParts), "corr-1")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDiagnosis, created.Status)
	return o, store
}

func event(t *testing.T, o *usecase.SagaOrchestrator, store *mocks.SagaStoreFake, evtType string, payload any) error {
	env, err := messaging.NewMessage(evtType, store.Sagas["os-1"].ID, "os-1", "corr-1", payload)
	require.NoError(t, err)
	return o.HandleEvent(context.Background(), env)
}

func mustEvent(t *testing.T, o *usecase.SagaOrchestrator, store *mocks.SagaStoreFake, evtType string, payload any) {
	require.NoError(t, event(t, o, store, evtType, payload))
}

func approve(t *testing.T, o *usecase.SagaOrchestrator, store *mocks.SagaStoreFake, approved bool) {
	_, err := o.Approve(store.Orders["os-1"], approved, "caro demais", "corr-2")
	require.NoError(t, err)
}

func reachApproval(t *testing.T, o *usecase.SagaOrchestrator, store *mocks.SagaStoreFake) {
	mustEvent(t, o, store, messaging.EvtDiagnosisFinished, messaging.DiagnosisFinishedPayload{Diagnosis: "Filtro entupido"})
	mustEvent(t, o, store, messaging.EvtBudgetGenerated, nil)
}

func TestSaga_HappyPath(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	assert.Equal(t, domain.StatusWaitingApproval, store.Orders["os-1"].Status)
	assert.Equal(t, "Filtro entupido", store.Orders["os-1"].Diagnosis)
	assert.NotEmpty(t, store.Orders["os-1"].WaitingApprovalAt)
	assert.Nil(t, store.Sagas["os-1"].DeadlineAt, "aprovação do cliente não tem prazo")

	approve(t, o, store, true)
	assert.Equal(t, domain.StatusWaitingPayment, store.Orders["os-1"].Status)

	mustEvent(t, o, store, messaging.EvtPartsReserved, nil)
	mustEvent(t, o, store, messaging.EvtPaymentRequested, messaging.PaymentRequestedPayload{CheckoutURL: "http://pagar/1", Amount: 150})
	assert.Equal(t, "http://pagar/1", store.Orders["os-1"].CheckoutURL)
	assert.Nil(t, store.Sagas["os-1"].DeadlineAt)

	mustEvent(t, o, store, messaging.EvtPaymentConfirmed, nil)
	assert.Equal(t, domain.StatusWaitingRepair, store.Orders["os-1"].Status)
	mustEvent(t, o, store, messaging.EvtExecutionQueued, nil)
	mustEvent(t, o, store, messaging.EvtExecutionStarted, nil)
	assert.Equal(t, domain.StatusInProgress, store.Orders["os-1"].Status)
	mustEvent(t, o, store, messaging.EvtExecutionFinished, nil)

	assert.Equal(t, domain.StatusFinished, store.Orders["os-1"].Status)
	assert.Equal(t, domain.SagaCompleted, store.Sagas["os-1"].Status)
	assert.Equal(t, []string{
		messaging.CmdStartDiagnosis, messaging.CmdGenerateBudget, messaging.CmdReserveParts,
		messaging.CmdRequestPayment, messaging.CmdStartExecution,
	}, store.Commands())
}

func TestSaga_CommandPayloads(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, true)

	var budget, reserve messaging.Envelope
	require.NoError(t, json.Unmarshal(store.Outbox[1].Body, &budget))
	require.NoError(t, json.Unmarshal(store.Outbox[2].Body, &reserve))
	assert.Equal(t, messaging.QueueBilling, store.Outbox[1].Queue)
	assert.Equal(t, messaging.QueueExecution, store.Outbox[2].Queue)
	assert.Equal(t, store.Sagas["os-1"].ID, budget.SagaID)
	assert.Equal(t, "corr-2", reserve.CorrelationID, "a aprovação passa a usar o correlation_id da requisição")

	var bp messaging.GenerateBudgetPayload
	require.NoError(t, budget.DecodePayload(&bp))
	assert.Equal(t, "Troca de óleo", bp.Services[0].Name)
	assert.Equal(t, 25.0, bp.Parts[0].UnitPrice)

	var rp messaging.ReservePartsPayload
	require.NoError(t, reserve.DecodePayload(&rp))
	assert.Equal(t, []messaging.ReservationItem{{PartID: "p1", Quantity: 2}}, rp.Items)
}

func TestSaga_ApprovalWithoutPartsGoesToPayment(t *testing.T) {
	o, store := startSaga(t, false)
	reachApproval(t, o, store)
	approve(t, o, store, true)
	assert.Equal(t, domain.StepPayment, store.Sagas["os-1"].CurrentStep)
	assert.Equal(t, messaging.CmdRequestPayment, store.Commands()[2])

	// Sem reserva, a falha no pagamento só cancela o orçamento
	mustEvent(t, o, store, messaging.EvtPaymentFailed, messaging.FailurePayload{Reason: "pagamento recusado"})
	assert.Equal(t, messaging.CmdCancelBudget, store.Commands()[3])
	assert.Len(t, store.Commands(), 4)
}

func TestSaga_RejectedBudgetCancelsBudget(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, false)

	assert.Equal(t, domain.StatusRejected, store.Orders["os-1"].Status)
	assert.Contains(t, store.Orders["os-1"].Notes, "caro demais")
	assert.Equal(t, domain.SagaCompensating, store.Sagas["os-1"].Status)
	assert.Equal(t, messaging.CmdCancelBudget, store.Commands()[2])

	mustEvent(t, o, store, messaging.EvtBudgetCancelled, nil)
	assert.Equal(t, domain.SagaCancelled, store.Sagas["os-1"].Status)
}

func TestSaga_ApproveOutsideApprovalStep(t *testing.T) {
	o, store := startSaga(t, true)
	_, err := o.Approve(store.Orders["os-1"], true, "", "")
	assert.ErrorIs(t, err, usecase.ErrSagaNotWaitingApproval)

	store.FindErr = errors.New("banco fora")
	_, err = o.Approve(store.Orders["os-1"], true, "", "")
	assert.Error(t, err)
}

func TestSaga_ApproveSaveError(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	store.SaveErr = errors.New("banco fora")
	_, err := o.Approve(store.Orders["os-1"], true, "", "")
	assert.Error(t, err)
	_, err = o.Approve(store.Orders["os-1"], false, "", "")
	assert.Error(t, err)
}

func TestSaga_BudgetGenerationFailedCancelsWithoutCompensation(t *testing.T) {
	o, store := startSaga(t, true)
	mustEvent(t, o, store, messaging.EvtDiagnosisFinished, nil)
	mustEvent(t, o, store, messaging.EvtBudgetGenerationFailed, messaging.FailurePayload{Reason: "serviço sem preço"})

	assert.Equal(t, domain.StatusCancelled, store.Orders["os-1"].Status)
	assert.Equal(t, "serviço sem preço", store.Orders["os-1"].CancellationReason)
	assert.Equal(t, domain.SagaCancelled, store.Sagas["os-1"].Status)
	assert.Len(t, store.Commands(), 2, "nada a desfazer")
}

func TestSaga_ReservationFailedCancelsBudget(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, true)
	mustEvent(t, o, store, messaging.EvtPartsReservationFailed, nil)

	assert.Equal(t, domain.StatusCancelled, store.Orders["os-1"].Status)
	assert.Equal(t, "falha ao reservar as peças", store.Orders["os-1"].CancellationReason)
	assert.Equal(t, []string{messaging.CmdCancelBudget}, store.Commands()[3:])
}

func TestSaga_PaymentFailedCompensatesInReverseOrder(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, true)
	mustEvent(t, o, store, messaging.EvtPartsReserved, nil)
	mustEvent(t, o, store, messaging.EvtPaymentFailed, messaging.FailurePayload{Reason: "pagamento expirado"})

	assert.Equal(t, domain.StatusCancelled, store.Orders["os-1"].Status)
	assert.Equal(t, []string{messaging.CmdReleaseParts, messaging.CmdCancelBudget}, store.Commands()[4:])

	mustEvent(t, o, store, messaging.EvtPartsReleased, nil)
	assert.Equal(t, domain.SagaCompensating, store.Sagas["os-1"].Status)
	mustEvent(t, o, store, messaging.EvtBudgetCancelled, nil)
	assert.Equal(t, domain.SagaCancelled, store.Sagas["os-1"].Status)
}

func TestSaga_TimeoutCompensatesCurrentStep(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, true) // passo atual: reserva, com prazo

	n, err := o.CheckTimeouts(time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, n, "ainda no prazo")

	n, err = o.CheckTimeouts(time.Now().Add(2 * time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, domain.StatusCancelled, store.Orders["os-1"].Status)
	assert.Contains(t, store.Orders["os-1"].CancellationReason, "reserva_pecas")
	assert.Equal(t, []string{messaging.CmdReleaseParts, messaging.CmdCancelBudget}, store.Commands()[3:])

	// A reserva respondeu depois do prazo: é desfeita de novo
	mustEvent(t, o, store, messaging.EvtPartsReserved, nil)
	assert.Equal(t, messaging.CmdReleaseParts, store.Commands()[5])
}

func TestSaga_TimeoutErrors(t *testing.T) {
	o, store := startSaga(t, true)
	mustEvent(t, o, store, messaging.EvtDiagnosisFinished, nil)
	later := time.Now().Add(2 * time.Minute)

	store.SaveErr = errors.New("conflito")
	n, err := o.CheckTimeouts(later)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	store.SaveErr = nil

	delete(store.Orders, "os-1")
	n, _ = o.CheckTimeouts(later)
	assert.Equal(t, 0, n)

	store.FindErr = errors.New("banco fora")
	_, err = o.CheckTimeouts(later)
	assert.Error(t, err)
}

func TestSaga_OutOfOrderAndRepeatedEventsAreIgnored(t *testing.T) {
	o, store := startSaga(t, true)

	// Evento de um passo que ainda não chegou
	mustEvent(t, o, store, messaging.EvtPaymentConfirmed, nil)
	mustEvent(t, o, store, messaging.EvtExecutionQueued, nil)
	assert.Equal(t, domain.StepDiagnosis, store.Sagas["os-1"].CurrentStep)

	// Mesmo evento duas vezes: aplicado uma vez só
	env, _ := messaging.NewMessage(messaging.EvtDiagnosisFinished, store.Sagas["os-1"].ID, "os-1", "c", nil)
	require.NoError(t, o.HandleEvent(context.Background(), env))
	store.SetSaga(func() domain.Saga { s := store.Sagas["os-1"]; s.CurrentStep = domain.StepDiagnosis; return s }())
	require.NoError(t, o.HandleEvent(context.Background(), env))
	assert.Len(t, store.Commands(), 2)

	// Evento de outra saga
	other, _ := messaging.NewMessage(messaging.EvtBudgetGenerated, "outra-saga", "os-1", "c", nil)
	require.NoError(t, o.HandleEvent(context.Background(), other))
}

func TestSaga_ExecutionEventsArrivingEarly(t *testing.T) {
	o, store := startSaga(t, true)
	reachApproval(t, o, store)
	approve(t, o, store, true)
	mustEvent(t, o, store, messaging.EvtPartsReserved, nil)
	mustEvent(t, o, store, messaging.EvtPaymentConfirmed, nil)

	// O início do reparo chega antes da confirmação da fila
	mustEvent(t, o, store, messaging.EvtExecutionStarted, nil)
	assert.Equal(t, domain.StepRepair, store.Sagas["os-1"].CurrentStep)
	mustEvent(t, o, store, messaging.EvtExecutionQueued, nil) // fora de ordem: ignorado

	// O fim do reparo chegando antes do início volta para a fila
	o2, store2 := startSaga(t, true)
	reachApproval(t, o2, store2)
	approve(t, o2, store2, true)
	mustEvent(t, o2, store2, messaging.EvtPartsReserved, nil)
	mustEvent(t, o2, store2, messaging.EvtPaymentConfirmed, nil)
	mustEvent(t, o2, store2, messaging.EvtExecutionQueued, nil)
	assert.Error(t, event(t, o2, store2, messaging.EvtExecutionFinished, nil))
}

func TestSaga_EventErrors(t *testing.T) {
	o, store := startSaga(t, true)

	err := event(t, o, store, "EventoNovo", nil)
	assert.ErrorIs(t, err, usecase.ErrUnknownEvent)

	env, _ := messaging.NewMessage(messaging.EvtBudgetGenerated, "", "os-inexistente", "c", nil)
	assert.ErrorIs(t, o.HandleEvent(context.Background(), env), domain.ErrSagaNotFound)

	saga := store.Sagas["os-1"]
	delete(store.Orders, "os-1")
	assert.Error(t, event(t, o, store, messaging.EvtDiagnosisFinished, nil))
	store.Orders["os-1"] = sagaOrder(true)
	store.SetSaga(saga)

	store.SaveErr = repository.ErrSagaConcurrentUpdate
	assert.ErrorIs(t, event(t, o, store, messaging.EvtDiagnosisFinished, nil), repository.ErrSagaConcurrentUpdate)
}

func TestSaga_StartErrors(t *testing.T) {
	store := mocks.NewSagaStoreFake()
	o := usecase.NewSagaOrchestrator(store, store, usecase.SagaConfig{})

	invalid := sagaOrder(true)
	invalid.Status = domain.StatusFinished
	_, err := o.Start(invalid, "")
	assert.Error(t, err)

	store.CreateErr = errors.New("banco fora")
	_, err = o.Start(sagaOrder(true), "")
	assert.Error(t, err)
}

// --- integração com o OrderUseCase ---

type fakeFlow struct {
	started  bool
	approved *bool
}

func (f *fakeFlow) Start(o domain.Order, _ string) (domain.Order, error) {
	f.started = true
	o.Status = domain.StatusDiagnosis
	return o, nil
}

func (f *fakeFlow) Approve(o domain.Order, approved bool, _, _ string) (*domain.Order, error) {
	f.approved = &approved
	return &o, nil
}

func TestOrderUseCase_WithSaga(t *testing.T) {
	uc, orderRepo, customerRepo, vehicleRepo, serviceRepo, _ := setupOrderUseCase()
	flow := &fakeFlow{}
	uc.WithSaga(flow)

	customerRepo.On("FindByID", "c1").Return(domain.Customer{ID: "c1", Name: "Ana"}, nil)
	vehicleRepo.On("FindByID", "v1").Return(domain.Vehicle{ID: "v1", Plate: "ABC1D23"}, nil)
	serviceRepo.On("FindByID", "s1").Return(domain.Service{ID: "s1", Name: "Revisão", Price: 100}, nil)

	created, err := uc.Create("c1", "v1", "", []string{"s1"}, nil)
	require.NoError(t, err)
	assert.True(t, flow.started)
	assert.Equal(t, domain.StatusDiagnosis, created.Status)
	orderRepo.AssertNotCalled(t, "Create")

	orderRepo.On("FindByID", "os-1").Return(domain.Order{ID: "os-1", Status: domain.StatusWaitingApproval}, nil)
	_, err = uc.ApproveOrder("os-1", true, "")
	require.NoError(t, err)
	assert.True(t, *flow.approved)
}
