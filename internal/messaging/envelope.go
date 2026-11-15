// Package messaging define o formato das mensagens da Saga (ADR-006),
// compartilhado com o Execution e o Billing Service.
package messaging

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Filas de destino dos comandos
const (
	QueueExecution = "execution-commands"
	QueueBilling   = "billing-commands"
)

// Comandos enviados pelo orquestrador
const (
	CmdStartDiagnosis = "StartDiagnosis"
	CmdGenerateBudget = "GenerateBudget"
	CmdReserveParts   = "ReserveParts"
	CmdRequestPayment = "RequestPayment"
	CmdStartExecution = "StartExecution"
	CmdCancelBudget   = "CancelBudget"
	CmdReleaseParts   = "ReleaseParts"
	CmdRefundPayment  = "RefundPayment"
)

// Eventos recebidos dos serviços
const (
	EvtDiagnosisFinished      = "DiagnosisFinished"
	EvtBudgetGenerated        = "BudgetGenerated"
	EvtBudgetGenerationFailed = "BudgetGenerationFailed"
	EvtPartsReserved          = "PartsReserved"
	EvtPartsReservationFailed = "PartsReservationFailed"
	EvtPaymentRequested       = "PaymentRequested"
	EvtPaymentConfirmed       = "PaymentConfirmed"
	EvtPaymentFailed          = "PaymentFailed"
	EvtExecutionQueued        = "ExecutionQueued"
	EvtExecutionStarted       = "ExecutionStarted"
	EvtExecutionFinished      = "ExecutionFinished"
	EvtBudgetCancelled        = "BudgetCancelled"
	EvtPartsReleased          = "PartsReleased"
	EvtPaymentRefunded        = "PaymentRefunded"
)

var ErrEmptyPayload = errors.New("mensagem sem payload")

// Envelope é o formato comum de todas as mensagens da Saga.
type Envelope struct {
	MessageID     string          `json:"message_id"`
	Type          string          `json:"type"`
	SagaID        string          `json:"saga_id"`
	OrderID       string          `json:"order_id"`
	CorrelationID string          `json:"correlation_id"`
	OccurredAt    string          `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// --- payloads dos comandos ---

type BudgetService struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type BudgetPart struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type GenerateBudgetPayload struct {
	Services []BudgetService `json:"services"`
	Parts    []BudgetPart    `json:"parts"`
}

type ReservationItem struct {
	PartID   string `json:"part_id"`
	Quantity int    `json:"quantity"`
}

type ReservePartsPayload struct {
	Items []ReservationItem `json:"items"`
}

// --- payloads dos eventos ---

type DiagnosisFinishedPayload struct {
	Diagnosis string `json:"diagnosis"`
}

type FailurePayload struct {
	Reason string `json:"reason"`
	PartID string `json:"part_id,omitempty"`
	Status string `json:"status,omitempty"`
}

type PaymentRequestedPayload struct {
	CheckoutURL string  `json:"checkout_url"`
	Amount      float64 `json:"amount"`
}

// NewMessage monta uma mensagem com ID e data próprios.
func NewMessage(msgType, sagaID, orderID, correlationID string, payload any) (Envelope, error) {
	env := Envelope{
		MessageID:     uuid.NewString(),
		Type:          msgType,
		SagaID:        sagaID,
		OrderID:       orderID,
		CorrelationID: correlationID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, err
		}
		env.Payload = raw
	}
	return env, nil
}

// DecodePayload lê o payload no formato indicado.
func (e Envelope) DecodePayload(v any) error {
	if len(e.Payload) == 0 {
		return ErrEmptyPayload
	}
	return json.Unmarshal(e.Payload, v)
}
