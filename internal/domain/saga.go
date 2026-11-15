package domain

import (
	"errors"
	"time"
)

type SagaStatus string

const (
	SagaRunning      SagaStatus = "em_andamento"
	SagaCompensating SagaStatus = "compensando"
	SagaCompleted    SagaStatus = "concluida"
	SagaCancelled    SagaStatus = "cancelada"
)

// SagaStep é um passo do fluxo da OS (ADR-005).
type SagaStep string

const (
	StepDiagnosis      SagaStep = "diagnostico"
	StepBudget         SagaStep = "orcamento"
	StepApproval       SagaStep = "aprovacao"
	StepReservation    SagaStep = "reserva_pecas"
	StepPayment        SagaStep = "pagamento"
	StepExecutionQueue SagaStep = "fila_execucao"
	StepRepair         SagaStep = "reparo"
)

var ErrSagaNotFound = errors.New("saga não encontrada")

// compensableSteps são os passos que alteram dados em outro serviço e,
// por isso, precisam ser desfeitos se a Saga falhar depois deles.
var compensableSteps = map[SagaStep]bool{
	StepBudget:      true,
	StepReservation: true,
	StepPayment:     true,
}

// Saga guarda o andamento do fluxo de uma OS: o passo atual, os passos já
// concluídos (é deles que sai o que compensar) e o prazo do passo atual.
type Saga struct {
	ID                   string     `json:"id"`
	OrderID              string     `json:"order_id"`
	Status               SagaStatus `json:"status"`
	CurrentStep          SagaStep   `json:"current_step"`
	CompletedSteps       []SagaStep `json:"completed_steps"`
	PendingCompensations []SagaStep `json:"pending_compensations"`
	DeadlineAt           *time.Time `json:"deadline_at,omitempty"`
	CorrelationID        string     `json:"correlation_id,omitempty"`
	FailureReason        string     `json:"failure_reason,omitempty"`
	Version              int        `json:"-"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// NewSaga inicia a Saga no primeiro passo (diagnóstico). O diagnóstico
// depende do mecânico, por isso não tem prazo.
func NewSaga(id, orderID, correlationID string) Saga {
	now := time.Now().UTC()
	return Saga{
		ID:                   id,
		OrderID:              orderID,
		Status:               SagaRunning,
		CurrentStep:          StepDiagnosis,
		CompletedSteps:       []SagaStep{},
		PendingCompensations: []SagaStep{},
		CorrelationID:        correlationID,
		Version:              1,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// Advance conclui o passo atual e passa para o próximo. Um timeout maior
// que zero define o prazo para o próximo passo responder.
func (s *Saga) Advance(next SagaStep, timeout time.Duration) {
	s.CompletedSteps = append(s.CompletedSteps, s.CurrentStep)
	s.CurrentStep = next
	s.setDeadline(timeout)
	s.touch()
}

// ClearDeadline remove o prazo do passo atual (ex.: o pedido de pagamento
// foi aceito e agora a Saga espera o cliente pagar).
func (s *Saga) ClearDeadline() {
	s.DeadlineAt = nil
	s.touch()
}

// Complete encerra a Saga com sucesso.
func (s *Saga) Complete() {
	s.CompletedSteps = append(s.CompletedSteps, s.CurrentStep)
	s.Status = SagaCompleted
	s.DeadlineAt = nil
	s.touch()
}

// StartCompensation encerra o fluxo por falha e devolve os passos que
// precisam ser desfeitos, em ordem inversa. includeCurrent inclui o passo
// atual quando ele pode ter tido efeito mesmo sem confirmação (ex.: prazo
// esgotado na reserva de peças). Se não houver nada a desfazer, a Saga já
// termina como cancelada.
func (s *Saga) StartCompensation(reason string, includeCurrent bool) []SagaStep {
	steps := append([]SagaStep{}, s.CompletedSteps...)
	if includeCurrent {
		steps = append(steps, s.CurrentStep)
	}

	pending := []SagaStep{}
	for i := len(steps) - 1; i >= 0; i-- {
		if compensableSteps[steps[i]] {
			pending = append(pending, steps[i])
		}
	}

	s.FailureReason = reason
	s.PendingCompensations = pending
	s.DeadlineAt = nil
	if len(pending) == 0 {
		s.Status = SagaCancelled
	} else {
		s.Status = SagaCompensating
	}
	s.touch()
	return pending
}

// CompensationDone registra a confirmação de uma compensação. Devolve true
// quando todas foram confirmadas e a Saga terminou.
func (s *Saga) CompensationDone(step SagaStep) bool {
	remaining := []SagaStep{}
	for _, p := range s.PendingCompensations {
		if p != step {
			remaining = append(remaining, p)
		}
	}
	s.PendingCompensations = remaining
	if len(remaining) == 0 && s.Status == SagaCompensating {
		s.Status = SagaCancelled
	}
	s.touch()
	return s.Status == SagaCancelled
}

// IsActive indica se a Saga ainda está em andamento (sem falha).
func (s *Saga) IsActive() bool {
	return s.Status == SagaRunning
}

// IsExpired indica se o passo atual passou do prazo.
func (s *Saga) IsExpired(now time.Time) bool {
	return s.IsActive() && s.DeadlineAt != nil && now.After(*s.DeadlineAt)
}

func (s *Saga) setDeadline(timeout time.Duration) {
	if timeout <= 0 {
		s.DeadlineAt = nil
		return
	}
	d := time.Now().UTC().Add(timeout)
	s.DeadlineAt = &d
}

func (s *Saga) touch() {
	s.UpdatedAt = time.Now().UTC()
}

// OutboxMessage é um comando gravado na mesma transação da Saga e
// publicado depois na fila de destino.
type OutboxMessage struct {
	ID        string
	Queue     string
	Type      string
	Body      []byte
	CreatedAt time.Time
}

// SagaChange reúne tudo que muda numa decisão da Saga e precisa ser gravado
// na mesma transação: a OS, a Saga, os comandos a publicar e, quando a
// decisão veio de um evento, o ID desse evento (para não processá-lo de
// novo).
type SagaChange struct {
	Order              *Order
	Saga               Saga
	Outbox             []OutboxMessage
	ProcessedMessageID string
}
