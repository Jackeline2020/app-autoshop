package usecase

import (
	"autoshop/internal/client"
	"autoshop/internal/domain"
	"autoshop/internal/repository"
	"autoshop/pkg/observability"
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// corrID extrai o correlation_id opcional passado pelos handlers HTTP (ver
// middleware.CorrelationID) sem quebrar as assinaturas existentes — os
// métodos abaixo aceitam um parâmetro variádico só pra isso, então chamadas
// já existentes (inclusive nos testes) continuam compilando sem alteração.
func corrID(ids []string) string {
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

var emailSubjectPattern = regexp.MustCompile(`(?i)OS\s+([0-9a-fA-F-]{6,36})\s*(?:->|:|para)\s*(.+)`)

// ErrCustomerInactive é devolvido por GetByCustomerIDChecked quando o cliente
// dono das OS está inativo — mesma mensagem usada pela autenticação por CPF
// (lambda-auth-autoshop/internal/authflow), pra manter a experiência
// consistente entre "não consigo nem pegar um token novo" e "meu token
// antigo ainda é válido mas minha conta foi desativada".
var ErrCustomerInactive = errors.New("cliente inativo — procure a oficina")

// ErrManualStatusNotAllowed é devolvido quando alguém tenta mudar
// manualmente um status controlado pela Saga. Só a entrega do veículo
// (finalizada → entregue) é registrada manualmente.
var ErrManualStatusNotAllowed = errors.New("apenas a entrega (finalizada → entregue) pode ser registrada manualmente; os demais status são atualizados pelo fluxo da OS")

// PartFinder consulta as peças no Execution Service, dono do catálogo e
// do estoque.
type PartFinder interface {
	FindPart(ctx context.Context, id string) (client.CatalogPart, error)
}

// SagaFlow é o fluxo da OS conduzido pela Saga (SagaOrchestrator).
type SagaFlow interface {
	Start(order domain.Order, correlationID string) (domain.Order, error)
	Approve(order domain.Order, approved bool, reason, correlationID string) (*domain.Order, error)
}

type OrderUseCase struct {
	saga         SagaFlow
	repo         repository.OrderRepository
	customerRepo repository.CustomerRepository
	vehicleRepo  repository.VehicleRepository
	serviceRepo  repository.ServiceRepository
	parts        PartFinder
}

func NewOrderUseCase(
	repo repository.OrderRepository,
	customerRepo repository.CustomerRepository,
	vehicleRepo repository.VehicleRepository,
	serviceRepo repository.ServiceRepository,
	parts PartFinder,
) *OrderUseCase {
	return &OrderUseCase{
		repo:         repo,
		customerRepo: customerRepo,
		vehicleRepo:  vehicleRepo,
		serviceRepo:  serviceRepo,
		parts:        parts,
	}
}

// WithSaga liga o fluxo da Saga: a abertura e a aprovação da OS passam a
// ser conduzidas pelo orquestrador.
func (u *OrderUseCase) WithSaga(flow SagaFlow) *OrderUseCase {
	u.saga = flow
	return u
}

func (u *OrderUseCase) Create(
	customerID, vehicleID, notes string,
	serviceIDs []string,
	partRequests []struct {
		PartID   string
		Quantity int
	},
	correlationID ...string,
) (domain.Order, error) {
	cid := corrID(correlationID)

	// Valida cliente
	customer, err := u.customerRepo.FindByID(customerID)
	if err != nil {
		observability.RecordOrderFailure("create_validate_customer", cid, "", "cliente não encontrado: "+customerID)
		return domain.Order{}, fmt.Errorf("cliente não encontrado")
	}

	// Valida veículo
	vehicle, err := u.vehicleRepo.FindByID(vehicleID)
	if err != nil {
		observability.RecordOrderFailure("create_validate_vehicle", cid, "", "veículo não encontrado: "+vehicleID)
		return domain.Order{}, fmt.Errorf("veículo não encontrado")
	}

	// Monta serviços
	var orderServices []domain.OrderService
	for _, sid := range serviceIDs {
		svc, err := u.serviceRepo.FindByID(sid)
		if err != nil {
			observability.RecordOrderFailure("create_validate_service", cid, "", "serviço não encontrado: "+sid)
			return domain.Order{}, fmt.Errorf("serviço não encontrado: %s", sid)
		}
		orderServices = append(orderServices, domain.OrderService{
			ServiceID:   svc.ID,
			ServiceName: svc.Name,
			Price:       svc.Price,
		})
	}

	// Monta peças com nome e preço do catálogo do Execution Service. O
	// estoque é reservado depois, quando o cliente aprova o orçamento.
	var orderParts []domain.OrderPart
	for _, pr := range partRequests {
		part, err := u.parts.FindPart(context.Background(), pr.PartID)
		if errors.Is(err, client.ErrPartNotFound) {
			observability.RecordOrderFailure("create_validate_part", cid, "", "peça não encontrada: "+pr.PartID)
			return domain.Order{}, fmt.Errorf("peça não encontrada: %s", pr.PartID)
		}
		if err != nil {
			observability.RecordOrderFailure("create_validate_part", cid, "", err.Error())
			return domain.Order{}, fmt.Errorf("não foi possível consultar a peça %s: %w", pr.PartID, err)
		}

		orderParts = append(orderParts, domain.OrderPart{
			PartID:    part.ID,
			PartName:  part.Name,
			Quantity:  pr.Quantity,
			UnitPrice: part.Price,
		})
	}

	now := time.Now().Format(time.RFC3339)

	order := domain.Order{
		ID:           uuid.New().String(),
		CustomerID:   customer.ID,
		CustomerName: customer.Name,
		VehicleID:    vehicle.ID,
		VehiclePlate: vehicle.Plate,
		Services:     orderServices,
		Parts:        orderParts,
		Status:       domain.StatusReceived,
		Notes:        notes,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := order.Validate(); err != nil {
		observability.RecordOrderFailure("create_validate_order", cid, "", err.Error())
		return domain.Order{}, err
	}

	order.CalculateTotals()

	var created domain.Order
	if u.saga != nil {
		// A OS é gravada junto com a Saga e o comando de início do diagnóstico
		created, err = u.saga.Start(order, cid)
	} else {
		created, err = u.repo.Create(order)
	}
	if err != nil {
		observability.RecordOrderFailure("create_persist", cid, order.ID, err.Error())
		return created, err
	}

	// Alimenta o painel de "volume diário de ordens de serviço" (evento
	// OrderLifecycle no New Relic, consultável por dia via NRQL).
	observability.RecordOrderEvent("order_created", cid, created.ID, map[string]interface{}{
		"customerId": created.CustomerID,
		"total":      created.Total,
	})

	return created, nil
}

func (u *OrderUseCase) GetAll() ([]domain.Order, error) {
	return u.repo.FindAll()
}

func (u *OrderUseCase) GetAllActive() ([]domain.Order, error) {
	orders, err := u.repo.FindAll()
	if err != nil {
		return nil, err
	}

	active := make([]domain.Order, 0, len(orders))
	for _, o := range orders {
		if o.Status.IsClosed() {
			continue
		}
		active = append(active, o)
	}

	sort.SliceStable(active, func(i, j int) bool {
		pi := domain.StatusPriority(active[i].Status)
		pj := domain.StatusPriority(active[j].Status)
		if pi != pj {
			return pi < pj
		}
		return active[i].CreatedAt < active[j].CreatedAt
	})

	return active, nil
}

func (u *OrderUseCase) GetByID(id string) (domain.Order, error) {
	return u.repo.FindByID(id)
}

func (u *OrderUseCase) GetByCustomerID(customerID string) ([]domain.Order, error) {
	return u.repo.FindByCustomerID(customerID)
}

// GetByCustomerIDChecked é a versão usada pela rota de auto-atendimento do
// cliente (GET /orders/customer/:customer_id com o próprio token de
// cliente). Diferente de GetByCustomerID (usada por funcionários, que
// continuam precisando enxergar OS de clientes inativos pra dar suporte),
// aqui o status do cliente é revalidado a cada chamada: o JWT emitido pela
// lambda-auth não expira quando o cliente é inativado depois, então sem essa
// checagem aqui um cliente inativado continuaria enxergando as próprias OS
// normalmente até o token expirar, sem nenhum aviso.
func (u *OrderUseCase) GetByCustomerIDChecked(customerID string) ([]domain.Order, error) {
	customer, err := u.customerRepo.FindByID(customerID)
	if err != nil {
		return nil, fmt.Errorf("cliente não encontrado")
	}

	if customer.Status == domain.CustomerStatusInactive {
		return nil, ErrCustomerInactive
	}

	return u.repo.FindByCustomerID(customerID)
}

func (u *OrderUseCase) GetByStatus(status string) ([]domain.Order, error) {
	if !domain.IsValidStatus(status) {
		return nil, fmt.Errorf("status inválido. Use: %s", strings.Join(domain.ValidStatusNames(), ", "))
	}

	return u.repo.FindByStatus(domain.OrderStatus(status))
}

// UpdateStatus registra uma mudança manual de status. Com a Saga
// controlando o fluxo da OS, só a entrega do veículo é manual.
func (u *OrderUseCase) UpdateStatus(id, status string, correlationID ...string) (*domain.Order, error) {
	if domain.OrderStatus(status) != domain.StatusDelivered {
		observability.RecordOrderFailure("manual_status_not_allowed", corrID(correlationID), id, status)
		return nil, ErrManualStatusNotAllowed
	}
	return u.changeStatus(id, domain.OrderStatus(status), corrID(correlationID))
}

// changeStatus aplica uma transição válida da máquina de estados e registra
// os eventos de observabilidade.
func (u *OrderUseCase) changeStatus(id string, next domain.OrderStatus, cid string) (*domain.Order, error) {
	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	oldStatus := order.Status

	if err := order.TransitionTo(next); err != nil {
		observability.RecordOrderFailure("status_transition", cid, id, err.Error())
		return nil, err
	}

	if err := u.repo.Update(order); err != nil {
		observability.RecordOrderFailure("status_transition_persist", cid, id, err.Error())
		return nil, err
	}

	observability.RecordOrderEvent("status_changed", cid, id, map[string]interface{}{
		"from": string(oldStatus),
		"to":   string(order.Status),
	})

	// Fecha a duração do status que acabou de ser deixado (Diagnóstico,
	// Execução ou Finalização — os 3 exigidos no dashboard da Fase 3) e manda
	// como evento separado (eventName "status_duration"), sem mexer no
	// cálculo por tipo de serviço que já existia (GetAverageServiceTime).
	if minutes, statusLabel, ok := statusDurationMinutes(oldStatus, order); ok {
		observability.RecordOrderEvent("status_duration", cid, id, map[string]interface{}{
			"status":          statusLabel,
			"durationMinutes": minutes,
		})
	}

	return &order, nil
}

// statusDurationMinutes calcula quanto tempo a OS ficou no status que
// acabou de ser deixado, usando os timestamps já gravados pelo
// domain.Order.TransitionTo. Só cobre os 3 status pedidos explicitamente
// no PDF da Fase 3 (Diagnóstico, Execução, Finalização) — outras
// transições (ex: Recebida → Diagnóstico) não geram esse evento.
func statusDurationMinutes(exitedStatus domain.OrderStatus, order domain.Order) (float64, string, bool) {
	var from, to, label string

	switch exitedStatus {
	case domain.StatusDiagnosis:
		from, to, label = order.DiagnosisAt, order.WaitingApprovalAt, "Diagnóstico"
	case domain.StatusInProgress:
		from, to, label = order.StartedAt, order.FinishedAt, "Execução"
	case domain.StatusFinished:
		from, to, label = order.FinishedAt, order.DeliveredAt, "Finalização"
	default:
		return 0, "", false
	}

	if from == "" || to == "" {
		return 0, "", false
	}

	start, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return 0, "", false
	}
	end, err := time.Parse(time.RFC3339, to)
	if err != nil {
		return 0, "", false
	}

	return end.Sub(start).Minutes(), label, true
}

// GetAverageTimeByStatus calcula o tempo médio de execução por status
// (Diagnóstico, Execução, Finalização) exigido no dashboard da Fase 3 — é um
// cálculo adicional, separado de GetAverageServiceTime (tempo médio por tipo
// de serviço), que continua funcionando exatamente como antes.
func (u *OrderUseCase) GetAverageTimeByStatus() (map[string]float64, error) {
	orders, err := u.repo.FindAll()
	if err != nil {
		return nil, err
	}

	statusTimes := map[string][]float64{
		"Diagnóstico": {},
		"Execução":    {},
		"Finalização": {},
	}

	for _, order := range orders {
		if minutes, label, ok := statusDurationMinutes(domain.StatusDiagnosis, order); ok {
			statusTimes[label] = append(statusTimes[label], minutes)
		}
		if minutes, label, ok := statusDurationMinutes(domain.StatusInProgress, order); ok {
			statusTimes[label] = append(statusTimes[label], minutes)
		}
		if minutes, label, ok := statusDurationMinutes(domain.StatusFinished, order); ok {
			statusTimes[label] = append(statusTimes[label], minutes)
		}
	}

	averages := map[string]float64{}
	for label, times := range statusTimes {
		if len(times) == 0 {
			continue
		}
		total := 0.0
		for _, t := range times {
			total += t
		}
		averages[label] = total / float64(len(times))
	}

	return averages, nil
}

func (u *OrderUseCase) GetAverageServiceTime() (map[string]float64, error) {
	orders, err := u.repo.FindAll()
	if err != nil {
		return nil, err
	}

	serviceTimes := map[string][]float64{}

	for _, order := range orders {
		if order.StartedAt == "" || order.FinishedAt == "" {
			continue
		}

		started, err := time.Parse(time.RFC3339, order.StartedAt)
		if err != nil {
			continue
		}

		finished, err := time.Parse(time.RFC3339, order.FinishedAt)
		if err != nil {
			continue
		}

		duration := finished.Sub(started).Minutes()

		for _, svc := range order.Services {
			serviceTimes[svc.ServiceName] = append(serviceTimes[svc.ServiceName], duration)
		}
	}

	averages := map[string]float64{}
	for name, times := range serviceTimes {
		total := 0.0
		for _, t := range times {
			total += t
		}
		averages[name] = total / float64(len(times))
	}

	return averages, nil
}

func (u *OrderUseCase) ApproveOrder(id string, approved bool, reason string, correlationID ...string) (*domain.Order, error) {
	cid := corrID(correlationID)

	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if order.Status != domain.StatusWaitingApproval {
		return nil, fmt.Errorf("OS não está aguardando aprovação. Status atual: %s", order.Status)
	}

	if u.saga != nil {
		return u.saga.Approve(order, approved, reason, cid)
	}

	if approved {
		return u.approve(order, cid)
	}

	return u.rejectOrder(order, reason, cid)
}

// approve registra a aprovação do cliente. A reserva das peças e a
// cobrança seguem pelo fluxo da OS (Saga).
func (u *OrderUseCase) approve(order domain.Order, cid string) (*domain.Order, error) {
	if err := order.TransitionTo(domain.StatusWaitingPayment); err != nil {
		observability.RecordOrderFailure("approval_transition", cid, order.ID, err.Error())
		return nil, err
	}

	if err := u.repo.Update(order); err != nil {
		observability.RecordOrderFailure("approval_persist", cid, order.ID, err.Error())
		return nil, err
	}

	observability.RecordOrderEvent("approved", cid, order.ID, nil)

	return &order, nil
}

func (u *OrderUseCase) rejectOrder(order domain.Order, reason string, cid string) (*domain.Order, error) {
	if err := order.TransitionTo(domain.StatusRejected); err != nil {
		observability.RecordOrderFailure("rejection_transition", cid, order.ID, err.Error())
		return nil, err
	}

	if reason != "" {
		order.Notes = order.Notes + " | Motivo da recusa: " + reason
	}

	if err := u.repo.Update(order); err != nil {
		observability.RecordOrderFailure("rejection_persist", cid, order.ID, err.Error())
		return nil, err
	}

	observability.RecordOrderEvent("rejected", cid, order.ID, map[string]interface{}{"reason": reason})

	return &order, nil
}

func (u *OrderUseCase) Delete(id string) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}
	return u.repo.Delete(id)
}

func (u *OrderUseCase) UpdateStatusFromEmail(orderID, status, subject string) (*domain.Order, error) {
	if orderID == "" || status == "" {
		if subject == "" {
			return nil, fmt.Errorf("e-mail sem order_id/status e sem assunto para interpretar")
		}

		matches := emailSubjectPattern.FindStringSubmatch(subject)
		if len(matches) != 3 {
			return nil, fmt.Errorf("não foi possível identificar a OS e o novo status no assunto do e-mail")
		}

		if orderID == "" {
			orderID = matches[1]
		}
		if status == "" {
			status = matches[2]
		}
	}

	normalized, ok := domain.NormalizeStatusAlias(status)
	if !ok {
		return nil, fmt.Errorf("status \"%s\" informado no e-mail não é reconhecido", status)
	}

	return u.UpdateStatus(orderID, string(normalized))
}
