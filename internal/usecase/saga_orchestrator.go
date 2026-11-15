package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/repository"
	"autoshop/pkg/observability"

	"github.com/google/uuid"
)

var (
	// ErrUnknownEvent indica um tipo de evento que o orquestrador não trata.
	ErrUnknownEvent = errors.New("evento desconhecido")
	// ErrSagaNotWaitingApproval indica que a OS não está no passo de aprovação.
	ErrSagaNotWaitingApproval = errors.New("o fluxo da OS não está aguardando a aprovação do orçamento")
)

// SagaConfig reúne os parâmetros do fluxo.
type SagaConfig struct {
	// StepTimeout é o prazo para um serviço responder a um passo automático.
	StepTimeout time.Duration
}

// compensationCommands indica, para cada passo, o comando que o desfaz e a
// fila do serviço responsável.
var compensationCommands = map[domain.SagaStep]struct{ queue, command string }{
	domain.StepBudget:      {messaging.QueueBilling, messaging.CmdCancelBudget},
	domain.StepReservation: {messaging.QueueExecution, messaging.CmdReleaseParts},
	domain.StepPayment:     {messaging.QueueBilling, messaging.CmdRefundPayment},
}

// lateSuccess são eventos de sucesso que podem chegar depois de a Saga já ter
// falhado (ex.: a reserva respondeu depois do prazo). Nesse caso o efeito é
// desfeito de novo, por segurança; os comandos de compensação são idempotentes.
var lateSuccess = map[string]domain.SagaStep{
	messaging.EvtBudgetGenerated:  domain.StepBudget,
	messaging.EvtPartsReserved:    domain.StepReservation,
	messaging.EvtPaymentConfirmed: domain.StepPayment,
}

// SagaOrchestrator conduz o fluxo da OS (ADR-005): decide o próximo passo a
// cada evento, envia os comandos pela outbox e, em caso de falha, dispara as
// compensações em ordem inversa.
type SagaOrchestrator struct {
	orders repository.OrderRepository
	sagas  repository.SagaRepository
	cfg    SagaConfig
}

func NewSagaOrchestrator(orders repository.OrderRepository, sagas repository.SagaRepository, cfg SagaConfig) *SagaOrchestrator {
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = 5 * time.Minute
	}
	return &SagaOrchestrator{orders: orders, sagas: sagas, cfg: cfg}
}

// decision acumula o que muda numa decisão, para ser gravado de uma vez.
type decision struct {
	order        domain.Order
	oldStatus    domain.OrderStatus
	orderChanged bool
	saga         domain.Saga
	outbox       []domain.OutboxMessage
	messageID    string
}

func newDecision(order domain.Order, saga domain.Saga, messageID string) *decision {
	return &decision{order: order, oldStatus: order.Status, saga: saga, messageID: messageID}
}

func (d *decision) transition(next domain.OrderStatus) error {
	if err := d.order.TransitionTo(next); err != nil {
		return err
	}
	d.orderChanged = true
	return nil
}

func (d *decision) send(queue, command string, payload any) error {
	env, err := messaging.NewMessage(command, d.saga.ID, d.saga.OrderID, d.saga.CorrelationID, payload)
	if err != nil {
		return err
	}
	body, err := json.Marshal(env)
	if err != nil {
		return err
	}
	d.outbox = append(d.outbox, domain.OutboxMessage{
		ID: env.MessageID, Queue: queue, Type: command, Body: body, CreatedAt: time.Now().UTC(),
	})
	return nil
}

// compensate encerra o fluxo por falha e envia os comandos que desfazem os
// passos já concluídos.
func (d *decision) compensate(reason string, includeCurrent bool) error {
	for _, step := range d.saga.StartCompensation(reason, includeCurrent) {
		c := compensationCommands[step]
		if err := d.send(c.queue, c.command, nil); err != nil {
			return err
		}
	}
	return nil
}

// cancel cancela a OS e compensa os passos concluídos.
func (d *decision) cancel(reason string, includeCurrent bool) error {
	if err := d.transition(domain.StatusCancelled); err != nil {
		return err
	}
	d.order.CancellationReason = reason
	return d.compensate(reason, includeCurrent)
}

// Start abre o fluxo de uma OS nova: a OS entra em diagnóstico e o Execution
// Service recebe o comando para iniciar o diagnóstico. OS, Saga e comando
// são gravados juntos.
func (o *SagaOrchestrator) Start(order domain.Order, correlationID string) (domain.Order, error) {
	if err := order.TransitionTo(domain.StatusDiagnosis); err != nil {
		return order, err
	}
	d := newDecision(order, domain.NewSaga(uuid.NewString(), order.ID, correlationID), "")
	if err := d.send(messaging.QueueExecution, messaging.CmdStartDiagnosis, nil); err != nil {
		return order, err
	}

	created, err := o.sagas.CreateOrderWithSaga(d.order, d.saga, d.outbox)
	if err != nil {
		return created, err
	}
	o.record(d, "saga_started")
	return created, nil
}

// Approve registra a resposta do cliente ao orçamento. Aprovado, o fluxo
// segue para a reserva das peças (ou direto para o pagamento, se a OS não
// tiver peças). Recusado, o orçamento é cancelado no Billing.
func (o *SagaOrchestrator) Approve(order domain.Order, approved bool, reason, correlationID string) (*domain.Order, error) {
	saga, err := o.sagas.FindByOrderID(order.ID)
	if err != nil {
		return nil, err
	}
	if !saga.IsActive() || saga.CurrentStep != domain.StepApproval {
		return nil, ErrSagaNotWaitingApproval
	}
	if correlationID != "" {
		saga.CorrelationID = correlationID
	}
	d := newDecision(order, saga, "")

	if approved {
		if err := d.transition(domain.StatusWaitingPayment); err != nil {
			return nil, err
		}
		if err := o.requestReservationOrPayment(d); err != nil {
			return nil, err
		}
	} else {
		if err := d.transition(domain.StatusRejected); err != nil {
			return nil, err
		}
		if reason != "" {
			d.order.Notes = d.order.Notes + " | Motivo da recusa: " + reason
		}
		if err := d.compensate("orçamento recusado pelo cliente", false); err != nil {
			return nil, err
		}
	}

	if err := o.save(d); err != nil {
		return nil, err
	}
	if approved {
		observability.RecordOrderEvent("approved", correlationID, order.ID, nil)
	} else {
		observability.RecordOrderEvent("rejected", correlationID, order.ID, map[string]interface{}{"reason": reason})
	}
	return &d.order, nil
}

func (o *SagaOrchestrator) requestReservationOrPayment(d *decision) error {
	if len(d.order.Parts) == 0 {
		d.saga.Advance(domain.StepPayment, o.cfg.StepTimeout)
		return d.send(messaging.QueueBilling, messaging.CmdRequestPayment, nil)
	}
	items := make([]messaging.ReservationItem, 0, len(d.order.Parts))
	for _, p := range d.order.Parts {
		items = append(items, messaging.ReservationItem{PartID: p.PartID, Quantity: p.Quantity})
	}
	d.saga.Advance(domain.StepReservation, o.cfg.StepTimeout)
	return d.send(messaging.QueueExecution, messaging.CmdReserveParts, messaging.ReservePartsPayload{Items: items})
}

// HandleEvent aplica um evento recebido de outro serviço. Eventos fora de
// ordem ou repetidos são ignorados; a decisão e o registro do evento são
// gravados juntos, então o mesmo evento nunca é aplicado duas vezes.
func (o *SagaOrchestrator) HandleEvent(_ context.Context, env messaging.Envelope) error {
	saga, err := o.sagas.FindByOrderID(env.OrderID)
	if err != nil {
		return err
	}
	if env.SagaID != "" && env.SagaID != saga.ID {
		slog.Warn("evento de outra saga ignorado", "type", env.Type, "order_id", env.OrderID, "saga_id", env.SagaID)
		return nil
	}
	order, err := o.orders.FindByID(env.OrderID)
	if err != nil {
		return err
	}

	d := newDecision(order, saga, env.MessageID)
	applied, err := o.decide(d, env)
	if err != nil {
		return err
	}
	if !applied {
		slog.Info("evento fora de ordem ignorado", "type", env.Type, "order_id", env.OrderID,
			"step", string(saga.CurrentStep), "saga_status", string(saga.Status), "correlation_id", env.CorrelationID)
		return nil
	}

	if err := o.save(d); errors.Is(err, repository.ErrMessageAlreadyProcessed) {
		return nil
	} else if err != nil {
		return err
	}
	o.record(d, env.Type)
	return nil
}

// eventHandler aplica um evento à decisão. Devolve false quando o evento
// não vale para o passo atual (repetido ou fora de ordem).
type eventHandler func(o *SagaOrchestrator, d *decision, env messaging.Envelope) (bool, error)

var eventHandlers = map[string]eventHandler{
	messaging.EvtDiagnosisFinished: (*SagaOrchestrator).onDiagnosisFinished,
	messaging.EvtBudgetGenerated:   (*SagaOrchestrator).onBudgetGenerated,
	messaging.EvtPartsReserved:     (*SagaOrchestrator).onPartsReserved,
	messaging.EvtPaymentRequested:  (*SagaOrchestrator).onPaymentRequested,
	messaging.EvtPaymentConfirmed:  (*SagaOrchestrator).onPaymentConfirmed,
	messaging.EvtExecutionQueued:   (*SagaOrchestrator).onExecutionQueued,
	messaging.EvtExecutionStarted:  (*SagaOrchestrator).onExecutionStarted,
	messaging.EvtExecutionFinished: (*SagaOrchestrator).onExecutionFinished,

	messaging.EvtBudgetGenerationFailed: onFailure(domain.StepBudget, "falha ao gerar o orçamento"),
	messaging.EvtPartsReservationFailed: onFailure(domain.StepReservation, "falha ao reservar as peças"),
	messaging.EvtPaymentFailed:          onFailure(domain.StepPayment, "pagamento não realizado"),

	messaging.EvtBudgetCancelled: onCompensated(domain.StepBudget),
	messaging.EvtPartsReleased:   onCompensated(domain.StepReservation),
	messaging.EvtPaymentRefunded: onCompensated(domain.StepPayment),
}

// decide aplica o evento à decisão.
func (o *SagaOrchestrator) decide(d *decision, env messaging.Envelope) (bool, error) {
	if step, ok := lateSuccess[env.Type]; ok && d.failed() {
		c := compensationCommands[step]
		return true, d.send(c.queue, c.command, nil)
	}
	handler, ok := eventHandlers[env.Type]
	if !ok {
		return false, fmt.Errorf("%w: %s", ErrUnknownEvent, env.Type)
	}
	return handler(o, d, env)
}

// at indica se a Saga está em andamento no passo informado.
func (d *decision) at(step domain.SagaStep) bool {
	return d.saga.IsActive() && d.saga.CurrentStep == step
}

// failed indica se a Saga já terminou por falha (compensando ou cancelada).
func (d *decision) failed() bool {
	return !d.saga.IsActive() && d.saga.Status != domain.SagaCompleted
}

func (o *SagaOrchestrator) onDiagnosisFinished(d *decision, env messaging.Envelope) (bool, error) {
	if !d.at(domain.StepDiagnosis) {
		return false, nil
	}
	var p messaging.DiagnosisFinishedPayload
	_ = env.DecodePayload(&p)
	d.order.Diagnosis = p.Diagnosis
	d.orderChanged = true
	d.saga.Advance(domain.StepBudget, o.cfg.StepTimeout)
	return true, d.send(messaging.QueueBilling, messaging.CmdGenerateBudget, budgetPayload(d.order))
}

func (o *SagaOrchestrator) onBudgetGenerated(d *decision, _ messaging.Envelope) (bool, error) {
	if !d.at(domain.StepBudget) {
		return false, nil
	}
	d.saga.Advance(domain.StepApproval, 0) // aprovação depende do cliente: sem prazo
	return true, d.transition(domain.StatusWaitingApproval)
}

func (o *SagaOrchestrator) onPartsReserved(d *decision, _ messaging.Envelope) (bool, error) {
	if !d.at(domain.StepReservation) {
		return false, nil
	}
	d.saga.Advance(domain.StepPayment, o.cfg.StepTimeout)
	return true, d.send(messaging.QueueBilling, messaging.CmdRequestPayment, nil)
}

func (o *SagaOrchestrator) onPaymentRequested(d *decision, env messaging.Envelope) (bool, error) {
	if !d.at(domain.StepPayment) {
		return false, nil
	}
	var p messaging.PaymentRequestedPayload
	_ = env.DecodePayload(&p)
	d.order.CheckoutURL = p.CheckoutURL
	d.orderChanged = true
	d.saga.ClearDeadline() // agora o prazo é o do pagamento, controlado pelo Billing
	return true, nil
}

func (o *SagaOrchestrator) onPaymentConfirmed(d *decision, _ messaging.Envelope) (bool, error) {
	if !d.at(domain.StepPayment) {
		return false, nil
	}
	d.saga.Advance(domain.StepExecutionQueue, o.cfg.StepTimeout)
	if err := d.transition(domain.StatusWaitingRepair); err != nil {
		return true, err
	}
	return true, d.send(messaging.QueueExecution, messaging.CmdStartExecution, nil)
}

func (o *SagaOrchestrator) onExecutionQueued(d *decision, _ messaging.Envelope) (bool, error) {
	if !d.at(domain.StepExecutionQueue) {
		return false, nil
	}
	d.saga.Advance(domain.StepRepair, 0) // o reparo depende da oficina: sem prazo
	return true, nil
}

func (o *SagaOrchestrator) onExecutionStarted(d *decision, _ messaging.Envelope) (bool, error) {
	if d.at(domain.StepExecutionQueue) { // a confirmação da fila pode chegar depois
		d.saga.Advance(domain.StepRepair, 0)
	} else if !d.at(domain.StepRepair) {
		return false, nil
	}
	return true, d.transition(domain.StatusInProgress)
}

func (o *SagaOrchestrator) onExecutionFinished(d *decision, _ messaging.Envelope) (bool, error) {
	if !d.at(domain.StepRepair) {
		return false, nil
	}
	// Se o início do reparo ainda não chegou, a transição falha e o evento
	// volta para a fila até o início ser processado.
	if err := d.transition(domain.StatusFinished); err != nil {
		return true, err
	}
	d.saga.Complete()
	return true, nil
}

// onFailure cancela a OS e compensa os passos concluídos quando o passo
// informado falha.
func onFailure(step domain.SagaStep, fallbackReason string) eventHandler {
	return func(_ *SagaOrchestrator, d *decision, env messaging.Envelope) (bool, error) {
		if !d.at(step) {
			return false, nil
		}
		return true, d.cancel(failureReason(env, fallbackReason), false)
	}
}

// onCompensated registra a confirmação de uma compensação.
func onCompensated(step domain.SagaStep) eventHandler {
	return func(_ *SagaOrchestrator, d *decision, _ messaging.Envelope) (bool, error) {
		d.saga.CompensationDone(step)
		return true, nil
	}
}

// CheckTimeouts cancela as OS cujo passo atual passou do prazo sem resposta.
// O passo atual também é compensado, porque o serviço pode ter feito a
// operação e apenas não respondido.
func (o *SagaOrchestrator) CheckTimeouts(now time.Time) (int, error) {
	expired, err := o.sagas.FindExpired(now)
	if err != nil {
		return 0, err
	}

	cancelled := 0
	for _, saga := range expired {
		order, err := o.orders.FindByID(saga.OrderID)
		if err != nil {
			slog.Error("OS da saga expirada não encontrada", "order_id", saga.OrderID, "error", err.Error())
			continue
		}
		d := newDecision(order, saga, "")
		reason := fmt.Sprintf("prazo esgotado no passo %s", saga.CurrentStep)
		if err := d.cancel(reason, true); err != nil {
			slog.Error("erro ao cancelar OS expirada", "order_id", saga.OrderID, "error", err.Error())
			continue
		}
		if err := o.save(d); err != nil {
			// ErrSagaConcurrentUpdate: um evento chegou ao mesmo tempo; a
			// próxima verificação decide de novo, já com o estado novo.
			slog.Warn("saga expirada não cancelada nesta rodada", "order_id", saga.OrderID, "error", err.Error())
			continue
		}
		o.record(d, "saga_timeout")
		cancelled++
	}
	return cancelled, nil
}

func (o *SagaOrchestrator) save(d *decision) error {
	change := domain.SagaChange{Saga: d.saga, Outbox: d.outbox, ProcessedMessageID: d.messageID}
	if d.orderChanged {
		change.Order = &d.order
	}
	return o.sagas.Save(change)
}

// record registra o avanço da Saga e, se a OS mudou de status, os mesmos
// eventos de observabilidade das mudanças manuais.
func (o *SagaOrchestrator) record(d *decision, trigger string) {
	cid := d.saga.CorrelationID
	observability.RecordOrderEvent("saga_progress", cid, d.order.ID, map[string]interface{}{
		"saga_id":     d.saga.ID,
		"trigger":     trigger,
		"step":        string(d.saga.CurrentStep),
		"saga_status": string(d.saga.Status),
	})
	if d.order.Status == d.oldStatus {
		return
	}
	observability.RecordOrderEvent("status_changed", cid, d.order.ID, map[string]interface{}{
		"from": string(d.oldStatus),
		"to":   string(d.order.Status),
	})
	if minutes, label, ok := statusDurationMinutes(d.oldStatus, d.order); ok {
		observability.RecordOrderEvent("status_duration", cid, d.order.ID, map[string]interface{}{
			"status":          label,
			"durationMinutes": minutes,
		})
	}
}

func budgetPayload(order domain.Order) messaging.GenerateBudgetPayload {
	p := messaging.GenerateBudgetPayload{
		Services: make([]messaging.BudgetService, 0, len(order.Services)),
		Parts:    make([]messaging.BudgetPart, 0, len(order.Parts)),
	}
	for _, s := range order.Services {
		p.Services = append(p.Services, messaging.BudgetService{ID: s.ServiceID, Name: s.ServiceName, Price: s.Price})
	}
	for _, part := range order.Parts {
		p.Parts = append(p.Parts, messaging.BudgetPart{ID: part.PartID, Name: part.PartName, Quantity: part.Quantity, UnitPrice: part.UnitPrice})
	}
	return p
}

func failureReason(env messaging.Envelope, fallback string) string {
	var p messaging.FailurePayload
	if err := env.DecodePayload(&p); err == nil && p.Reason != "" {
		return p.Reason
	}
	return fallback
}
