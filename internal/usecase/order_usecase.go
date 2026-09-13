package usecase

import (
	"autoshop/internal/domain"
	"autoshop/internal/repository"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/google/uuid"
)

var emailSubjectPattern = regexp.MustCompile(`(?i)OS\s+([0-9a-fA-F-]{6,36})\s*(?:->|:|para)\s*(.+)`)

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
) (domain.Order, error) {

	// Valida cliente
	customer, err := u.customerRepo.FindByID(customerID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("cliente não encontrado")
	}

	// Valida veículo
	vehicle, err := u.vehicleRepo.FindByID(vehicleID)
	if err != nil {
		return domain.Order{}, fmt.Errorf("veículo não encontrado")
	}

	// Monta serviços
	var orderServices []domain.OrderService
	for _, sid := range serviceIDs {
		svc, err := u.serviceRepo.FindByID(sid)
		if err != nil {
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
			return domain.Order{}, fmt.Errorf("peça não encontrada: %s", pr.PartID)
		}

		if part.Stock < pr.Quantity {
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
		return domain.Order{}, err
	}

	order.CalculateTotals()

	return u.repo.Create(order)
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

func (u *OrderUseCase) GetByStatus(status string) ([]domain.Order, error) {
	if !domain.IsValidStatus(status) {
		return nil, fmt.Errorf("status inválido. Use: recebida, em_diagnostico, aguardando_aprovacao, em_execucao, finalizada, entregue, recusada")
	}

	return u.repo.FindByStatus(domain.OrderStatus(status))
}

func (u *OrderUseCase) UpdateStatus(id, status string) (*domain.Order, error) {
	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if err := order.TransitionTo(domain.OrderStatus(status)); err != nil {
		return nil, err
	}

	if err := u.repo.Update(order); err != nil {
		return nil, err
	}

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

func (u *OrderUseCase) ApproveOrder(id string, approved bool, reason string) (*domain.Order, error) {
	order, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if order.Status != domain.StatusWaitingApproval {
		return nil, fmt.Errorf("OS não está aguardando aprovação. Status atual: %s", order.Status)
	}

	if approved {
		return u.approveAndExecute(order)
	}

	return u.rejectOrder(order, reason)
}

func (u *OrderUseCase) approveAndExecute(order domain.Order) (*domain.Order, error) {
	if err := order.TransitionTo(domain.StatusInProgress); err != nil {
		return nil, err
	}

	if err := u.deductStock(order); err != nil {
		return nil, err
	}

	if err := u.repo.Update(order); err != nil {
		return nil, err
	}

	return &order, nil
}

func (u *OrderUseCase) rejectOrder(order domain.Order, reason string) (*domain.Order, error) {
	if err := order.TransitionTo(domain.StatusRejected); err != nil {
		return nil, err
	}

	if reason != "" {
		order.Notes = order.Notes + " | Motivo da recusa: " + reason
	}

	if err := u.repo.Update(order); err != nil {
		return nil, err
	}

	return &order, nil
}

func (u *OrderUseCase) deductStock(order domain.Order) error {
	for _, p := range order.Parts {
		part, err := u.partRepo.FindByID(p.PartID)
		if err != nil {
			return fmt.Errorf("erro ao verificar peça %s", p.PartID)
		}

		newStock := part.Stock - p.Quantity
		if newStock < 0 {
			return fmt.Errorf("estoque insuficiente para %s no momento da execução", p.PartName)
		}

		if err := u.partRepo.UpdateStock(p.PartID, newStock); err != nil {
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
