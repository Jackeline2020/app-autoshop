package domain_test

import (
	"testing"
	"time"

	"autoshop/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestNewSaga(t *testing.T) {
	s := domain.NewSaga("s1", "os-1", "corr-1")
	assert.Equal(t, domain.SagaRunning, s.Status)
	assert.Equal(t, domain.StepDiagnosis, s.CurrentStep)
	assert.Empty(t, s.CompletedSteps)
	assert.Nil(t, s.DeadlineAt)
	assert.True(t, s.IsActive())
	assert.Equal(t, 1, s.Version)
}

func TestSaga_AdvanceWithAndWithoutDeadline(t *testing.T) {
	s := domain.NewSaga("s1", "os-1", "c")

	s.Advance(domain.StepBudget, 5*time.Minute)
	assert.Equal(t, domain.StepBudget, s.CurrentStep)
	assert.Equal(t, []domain.SagaStep{domain.StepDiagnosis}, s.CompletedSteps)
	assert.NotNil(t, s.DeadlineAt)
	assert.False(t, s.IsExpired(time.Now()))
	assert.True(t, s.IsExpired(time.Now().Add(6*time.Minute)))

	s.Advance(domain.StepApproval, 0)
	assert.Nil(t, s.DeadlineAt)
	assert.False(t, s.IsExpired(time.Now().Add(time.Hour)))
}

func TestSaga_ClearDeadline(t *testing.T) {
	s := domain.NewSaga("s1", "os-1", "c")
	s.Advance(domain.StepPayment, time.Minute)
	s.ClearDeadline()
	assert.Nil(t, s.DeadlineAt)
}

func TestSaga_Complete(t *testing.T) {
	s := domain.NewSaga("s1", "os-1", "c")
	s.Advance(domain.StepRepair, 0)
	s.Complete()
	assert.Equal(t, domain.SagaCompleted, s.Status)
	assert.Contains(t, s.CompletedSteps, domain.StepRepair)
	assert.False(t, s.IsActive())
}

// sagaAt monta uma Saga parada no passo indicado, com os anteriores concluídos.
func sagaAt(steps ...domain.SagaStep) domain.Saga {
	s := domain.NewSaga("s1", "os-1", "c")
	for _, st := range steps {
		s.Advance(st, time.Minute)
	}
	return s
}

func TestSaga_CompensationInReverseOrder(t *testing.T) {
	// Falha ao entrar na fila de execução, depois de pago
	s := sagaAt(domain.StepBudget, domain.StepApproval, domain.StepReservation, domain.StepPayment, domain.StepExecutionQueue)

	pending := s.StartCompensation("prazo esgotado", false)
	assert.Equal(t, []domain.SagaStep{domain.StepPayment, domain.StepReservation, domain.StepBudget}, pending)
	assert.Equal(t, domain.SagaCompensating, s.Status)
	assert.Equal(t, "prazo esgotado", s.FailureReason)
	assert.Nil(t, s.DeadlineAt)

	assert.False(t, s.CompensationDone(domain.StepPayment))
	assert.False(t, s.CompensationDone(domain.StepReservation))
	assert.True(t, s.CompensationDone(domain.StepBudget))
	assert.Equal(t, domain.SagaCancelled, s.Status)
}

func TestSaga_PaymentFailedDoesNotRefund(t *testing.T) {
	// Pagamento recusado: o passo atual (pagamento) não é compensado
	s := sagaAt(domain.StepBudget, domain.StepApproval, domain.StepReservation, domain.StepPayment)
	pending := s.StartCompensation("pagamento recusado", false)
	assert.Equal(t, []domain.SagaStep{domain.StepReservation, domain.StepBudget}, pending)
}

func TestSaga_ReservationTimeoutReleasesPossiblePartialReservation(t *testing.T) {
	s := sagaAt(domain.StepBudget, domain.StepApproval, domain.StepReservation)
	pending := s.StartCompensation("prazo esgotado", true)
	assert.Equal(t, []domain.SagaStep{domain.StepReservation, domain.StepBudget}, pending)
}

func TestSaga_NothingToCompensate(t *testing.T) {
	// Falha no diagnóstico: nada foi feito em outro serviço
	s := domain.NewSaga("s1", "os-1", "c")
	pending := s.StartCompensation("diagnóstico não concluído", true)
	assert.Empty(t, pending)
	assert.Equal(t, domain.SagaCancelled, s.Status)
}

func TestSaga_CompensationDoneIgnoresUnknownStep(t *testing.T) {
	s := sagaAt(domain.StepBudget, domain.StepApproval)
	s.StartCompensation("recusado", false)
	assert.False(t, s.CompensationDone(domain.StepPayment))
	assert.Equal(t, []domain.SagaStep{domain.StepBudget}, s.PendingCompensations)
	assert.True(t, s.CompensationDone(domain.StepBudget))
}
