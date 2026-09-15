package usecase

import (
	"autoshop/internal/domain"
	"autoshop/internal/repository"
	"autoshop/pkg/observability"
	"errors"
	"fmt"
	"regexp"
	"sort"
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

type OrderUseCase struct {
	repo         repository.OrderRepository
	customerRepo repository.CustomerRepository
	vehicleRepo  repository.VehicleRepository
	serviceRepo  repository.ServiceRepository
	partRepo     repository.PartRepository
}

func NewOrderUseCase(
	repo repository.OrderRepository,
	customerRepo repository.CustomerRepository,
	vehicleRepo repository.VehicleRepository,
	serviceRepo repository.ServiceRepository,
	partRepo repository.PartRepository,
) *OrderUseCase {
	return &OrderUseCase{
		repo:         repo,
		customerRepo: customerRepo,
		vehicleRepo:  vehicleRepo,
		serviceRepo:  serviceRepo,
		partRepo:     partRepo,
	}
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

	// Monta peças e valida estoque
	var orderParts []domain.OrderPart
	for _, pr := range partRequests {
		part, err := u.partRepo.FindByID(pr.PartID)
		if err != nil {
			observability.RecordOrderFailure("create_validate_part", cid, "", "peça não encontrada: "+pr.PartID)
			return domain.Order{}, fmt.Errorf("peça não encontrada: %s", pr.PartID)
		}

		if part.Stock < pr.Quantity {
			observability.RecordOrderFailure("create_validate_stock", cid, "", "estoque insuficiente para "+part.Name)
			return domain.Order{}, fmt.Errorf("estoque insuficiente para %s. Disponível: %d, solicitado: %d",
				part.Name, part.Stock, pr.Quantity)
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

	created, err := u.repo.Create(order)
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
		return nil, fmt.Errorf("status inválido. Use: recebida, em_diagnostico, aguardando_aprovacao, em_execucao, finalizada, entregue, recusada")
	}

	return u.repo.FindByStatus(domain.OrderStatus(status))
}

func (u *OrderUseCase) UpdateStatus(id, status string, correlationID ...string) (*domain.Order, error) {
	cid := corrID(correlationID)

	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	oldStatus := order.Status

	if err := order.TransitionTo(domain.OrderStatus(status)); err != nil {
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

	return &order, nil
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

	if approved {
		return u.approveAndExecute(order, cid)
	}

	return u.rejectOrder(order, reason, cid)
}

func (u *OrderUseCase) approveAndExecute(order domain.Order, cid string) (*domain.Order, error) {
	if err := order.TransitionTo(domain.StatusInProgress); err != nil {
		observability.RecordOrderFailure("approval_transition", cid, order.ID, err.Error())
		return nil, err
	}

	if err := u.deductStock(order, cid); err != nil {
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

func (u *OrderUseCase) deductStock(order domain.Order, cid string) error {
	for _, p := range order.Parts {
		part, err := u.partRepo.FindByID(p.PartID)
		if err != nil {
			observability.RecordOrderFailure("stock_deduction", cid, order.ID, "peça não encontrada: "+p.PartID)
			return fmt.Errorf("erro ao verificar peça %s", p.PartID)
		}

		newStock := part.Stock - p.Quantity
		if newStock < 0 {
			observability.RecordOrderFailure("stock_deduction", cid, order.ID, "estoque insuficiente para "+p.PartName)
			return fmt.Errorf("estoque insuficiente para %s no momento da execução", p.PartName)
		}

		if err := u.partRepo.UpdateStock(p.PartID, newStock); err != nil {
			observability.RecordOrderFailure("stock_deduction", cid, order.ID, "erro ao baixar estoque de "+p.PartName)
			return fmt.Errorf("erro ao baixar estoque de %s", p.PartName)
		}
	}
	return nil
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
