package domain

import (
	"errors"
	"strings"
	"time"
)

type OrderStatus string

const (
	StatusReceived        OrderStatus = "recebida"
	StatusDiagnosis       OrderStatus = "em_diagnostico"
	StatusWaitingApproval OrderStatus = "aguardando_aprovacao"
	StatusInProgress      OrderStatus = "em_execucao"
	StatusFinished        OrderStatus = "finalizada"
	StatusDelivered       OrderStatus = "entregue"
	StatusRejected        OrderStatus = "recusada" // cliente recusou o orçamento
)

type OrderService struct {
	ServiceID   string  `json:"service_id"   dynamodbav:"service_id"`
	ServiceName string  `json:"service_name" dynamodbav:"service_name"`
	Price       float64 `json:"price"        dynamodbav:"price"`
	ExecutedAt  string  `json:"executed_at,omitempty" dynamodbav:"executed_at,omitempty"`
}

type OrderPart struct {
	PartID    string  `json:"part_id"    dynamodbav:"part_id"`
	PartName  string  `json:"part_name"  dynamodbav:"part_name"`
	Quantity  int     `json:"quantity"   dynamodbav:"quantity"`
	UnitPrice float64 `json:"unit_price" dynamodbav:"unit_price"`
}

type Order struct {
	ID            string         `json:"id"             dynamodbav:"id"`
	CustomerID    string         `json:"customer_id"    dynamodbav:"customer_id"`
	CustomerName  string         `json:"customer_name"  dynamodbav:"customer_name"`
	VehicleID     string         `json:"vehicle_id"     dynamodbav:"vehicle_id"`
	VehiclePlate  string         `json:"vehicle_plate"  dynamodbav:"vehicle_plate"`
	Services      []OrderService `json:"services"      dynamodbav:"services"`
	Parts         []OrderPart    `json:"parts"          dynamodbav:"parts"`
	Status        OrderStatus    `json:"status"         dynamodbav:"status"`
	TotalServices float64        `json:"total_services" dynamodbav:"total_services"`
	TotalParts    float64        `json:"total_parts"    dynamodbav:"total_parts"`
	Total         float64        `json:"total"          dynamodbav:"total"`
	Notes         string         `json:"notes,omitempty" dynamodbav:"notes,omitempty"`
	CreatedAt     string         `json:"created_at"     dynamodbav:"created_at"`
	UpdatedAt     string         `json:"updated_at"     dynamodbav:"updated_at"`
	StartedAt     string         `json:"started_at,omitempty"   dynamodbav:"started_at,omitempty"`
	FinishedAt    string         `json:"finished_at,omitempty"  dynamodbav:"finished_at,omitempty"`
	DeliveredAt   string         `json:"delivered_at,omitempty" dynamodbav:"delivered_at,omitempty"`
}

func (o *Order) Validate() error {
	if o.CustomerID == "" {
		return errors.New("cliente é obrigatório")
	}

	if o.VehicleID == "" {
		return errors.New("veículo é obrigatório")
	}

	if len(o.Services) == 0 {
		return errors.New("pelo menos um serviço é obrigatório")
	}

	for _, p := range o.Parts {
		if p.Quantity <= 0 {
			return errors.New("quantidade da peça deve ser maior que zero")
		}
	}

	return nil
}

func (o *Order) CalculateTotals() {
	o.TotalServices = 0
	for _, s := range o.Services {
		o.TotalServices += s.Price
	}

	o.TotalParts = 0
	for _, p := range o.Parts {
		o.TotalParts += p.UnitPrice * float64(p.Quantity)
	}

	o.Total = o.TotalServices + o.TotalParts
}

// Máquina de estados — define transições válidas
func (o *Order) CanTransitionTo(next OrderStatus) bool {
	transitions := map[OrderStatus][]OrderStatus{
		StatusReceived:        {StatusDiagnosis},
		StatusDiagnosis:       {StatusWaitingApproval},
		StatusWaitingApproval: {StatusInProgress, StatusRejected},
		StatusInProgress:      {StatusFinished},
		StatusFinished:        {StatusDelivered},
		StatusDelivered:       {},
		StatusRejected:        {},
	}

	allowed, ok := transitions[o.Status]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == next {
			return true
		}
	}

	return false
}

func (o *Order) TransitionTo(next OrderStatus) error {
	if !o.CanTransitionTo(next) {
		return errors.New("transição de status inválida: " + string(o.Status) + " → " + string(next))
	}

	now := time.Now().Format(time.RFC3339)

	switch next {
	case StatusInProgress:
		o.StartedAt = now
	case StatusFinished:
		o.FinishedAt = now
	case StatusDelivered:
		o.DeliveredAt = now
	}

	o.Status = next
	o.UpdatedAt = now

	return nil
}

func StatusPriority(status OrderStatus) int {
	priorities := map[OrderStatus]int{
		StatusInProgress:      1,
		StatusWaitingApproval: 2,
		StatusDiagnosis:       3,
		StatusReceived:        4,
		StatusRejected:        5,
	}

	if p, ok := priorities[status]; ok {
		return p
	}

	return 99
}

func (s OrderStatus) IsClosed() bool {
	return s == StatusFinished || s == StatusDelivered
}

func StatusLabel(status OrderStatus) string {
	labels := map[OrderStatus]string{
		StatusReceived:        "Recebida",
		StatusDiagnosis:       "Diagnóstico",
		StatusWaitingApproval: "Aguardando Aprovação",
		StatusInProgress:      "Execução",
		StatusFinished:        "Finalizada",
		StatusDelivered:       "Entregue",
		StatusRejected:        "Recusada",
	}

	if l, ok := labels[status]; ok {
		return l
	}

	return string(status)
}

func NormalizeStatusAlias(raw string) (OrderStatus, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = stripAccents(normalized)
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	status := OrderStatus(normalized)
	if validOrderStatuses[status] {
		return status, true
	}

	return "", false
}

var validOrderStatuses = map[OrderStatus]bool{
	StatusReceived:        true,
	StatusDiagnosis:       true,
	StatusWaitingApproval: true,
	StatusInProgress:      true,
	StatusFinished:        true,
	StatusDelivered:       true,
	StatusRejected:        true,
}

func IsValidStatus(status string) bool {
	return validOrderStatuses[OrderStatus(status)]
}

func stripAccents(s string) string {
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "ã", "a", "â", "a",
		"é", "e", "ê", "e",
		"í", "i",
		"ó", "o", "ô", "o", "õ", "o",
		"ú", "u",
		"ç", "c",
	)
	return replacer.Replace(s)
}
