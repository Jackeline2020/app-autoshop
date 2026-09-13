package domain_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeValidOrder() domain.Order {
	return domain.Order{
		ID:         "1",
		CustomerID: "123",
		VehicleID:  "456",
		Status:     domain.StatusReceived,
		Services: []domain.OrderService{
			{ServiceID: "789", ServiceName: "Troca de óleo", Price: 100.0},
		},
		Parts: []domain.OrderPart{
			{PartID: "111", PartName: "Filtro", Quantity: 1, UnitPrice: 50.0},
		},
	}
}

func TestOrderValidate_Success(t *testing.T) {
	order := makeValidOrder()
	err := order.Validate()
	assert.NoError(t, err)
}

func TestOrderValidate_MissingCustomer(t *testing.T) {
	order := makeValidOrder()
	order.CustomerID = ""

	err := order.Validate()
	assert.EqualError(t, err, "cliente é obrigatório")
}

func TestOrderValidate_MissingVehicle(t *testing.T) {
	order := makeValidOrder()
	order.VehicleID = ""

	err := order.Validate()
	assert.EqualError(t, err, "veículo é obrigatório")
}

func TestOrderValidate_MissingServices(t *testing.T) {
	order := makeValidOrder()
	order.Services = []domain.OrderService{}

	err := order.Validate()
	assert.EqualError(t, err, "pelo menos um serviço é obrigatório")
}

func TestOrderValidate_NegativePartQuantity(t *testing.T) {
	order := makeValidOrder()
	order.Parts = []domain.OrderPart{
		{PartID: "111", PartName: "Filtro", Quantity: -1, UnitPrice: 50.0},
	}

	err := order.Validate()
	assert.EqualError(t, err, "quantidade da peça deve ser maior que zero")
}

func TestOrderCalculateTotals(t *testing.T) {
	order := makeValidOrder()
	order.CalculateTotals()

	assert.Equal(t, 100.0, order.TotalServices)
	assert.Equal(t, 50.0, order.TotalParts)
	assert.Equal(t, 150.0, order.Total)
}

func TestOrderTransition_ValidFlow(t *testing.T) {
	order := makeValidOrder()

	assert.NoError(t, order.TransitionTo(domain.StatusDiagnosis))
	assert.Equal(t, domain.StatusDiagnosis, order.Status)

	assert.NoError(t, order.TransitionTo(domain.StatusWaitingApproval))
	assert.Equal(t, domain.StatusWaitingApproval, order.Status)

	assert.NoError(t, order.TransitionTo(domain.StatusInProgress))
	assert.Equal(t, domain.StatusInProgress, order.Status)

	assert.NoError(t, order.TransitionTo(domain.StatusFinished))
	assert.Equal(t, domain.StatusFinished, order.Status)

	assert.NoError(t, order.TransitionTo(domain.StatusDelivered))
	assert.Equal(t, domain.StatusDelivered, order.Status)
}

func TestOrderTransition_Rejected(t *testing.T) {
	order := makeValidOrder()

	order.TransitionTo(domain.StatusDiagnosis)
	order.TransitionTo(domain.StatusWaitingApproval)

	assert.NoError(t, order.TransitionTo(domain.StatusRejected))
	assert.Equal(t, domain.StatusRejected, order.Status)
}

func TestOrderTransition_InvalidFlow(t *testing.T) {
	order := makeValidOrder()

	err := order.TransitionTo(domain.StatusDelivered)
	assert.EqualError(t, err, "transição de status inválida: recebida → entregue")
}

func TestOrderTransition_AfterDelivered(t *testing.T) {
	order := makeValidOrder()
	order.Status = domain.StatusDelivered

	err := order.TransitionTo(domain.StatusFinished)
	assert.Error(t, err)
}

func TestOrderTransition_SetsStartedAt(t *testing.T) {
	order := makeValidOrder()
	order.Status = domain.StatusWaitingApproval

	order.TransitionTo(domain.StatusInProgress)

	assert.NotEmpty(t, order.StartedAt)
}

func TestOrderTransition_SetsFinishedAt(t *testing.T) {
	order := makeValidOrder()
	order.Status = domain.StatusInProgress

	order.TransitionTo(domain.StatusFinished)

	assert.NotEmpty(t, order.FinishedAt)
}

func TestOrderTransition_SetsDeliveredAt(t *testing.T) {
	order := makeValidOrder()
	order.Status = domain.StatusFinished

	order.TransitionTo(domain.StatusDelivered)

	assert.NotEmpty(t, order.DeliveredAt)
}

func TestOrderCalculateTotals_NoParts(t *testing.T) {
	order := makeValidOrder()
	order.Parts = []domain.OrderPart{}
	order.CalculateTotals()

	assert.Equal(t, 100.0, order.TotalServices)
	assert.Equal(t, 0.0, order.TotalParts)
	assert.Equal(t, 100.0, order.Total)
}

func TestStatusPriority_Order(t *testing.T) {
	assert.Less(t, domain.StatusPriority(domain.StatusInProgress), domain.StatusPriority(domain.StatusWaitingApproval))
	assert.Less(t, domain.StatusPriority(domain.StatusWaitingApproval), domain.StatusPriority(domain.StatusDiagnosis))
	assert.Less(t, domain.StatusPriority(domain.StatusDiagnosis), domain.StatusPriority(domain.StatusReceived))
}

func TestStatusPriority_UnknownStatus(t *testing.T) {
	assert.Equal(t, 99, domain.StatusPriority(domain.OrderStatus("status_desconhecido")))
}

func TestIsClosed(t *testing.T) {
	assert.True(t, domain.StatusFinished.IsClosed())
	assert.True(t, domain.StatusDelivered.IsClosed())
	assert.False(t, domain.StatusInProgress.IsClosed())
	assert.False(t, domain.StatusReceived.IsClosed())
}

func TestStatusLabel(t *testing.T) {
	assert.Equal(t, "Recebida", domain.StatusLabel(domain.StatusReceived))
	assert.Equal(t, "Diagnóstico", domain.StatusLabel(domain.StatusDiagnosis))
	assert.Equal(t, "Aguardando Aprovação", domain.StatusLabel(domain.StatusWaitingApproval))
	assert.Equal(t, "Execução", domain.StatusLabel(domain.StatusInProgress))
	assert.Equal(t, "Finalizada", domain.StatusLabel(domain.StatusFinished))
	assert.Equal(t, "Entregue", domain.StatusLabel(domain.StatusDelivered))
	assert.Equal(t, "Recusada", domain.StatusLabel(domain.StatusRejected))
}

func TestStatusLabel_Unknown(t *testing.T) {
	assert.Equal(t, "status_x", domain.StatusLabel(domain.OrderStatus("status_x")))
}

func TestNormalizeStatusAlias_OfficialNamesWithFormattingVariants(t *testing.T) {
	cases := map[string]domain.OrderStatus{
		"recebida":             domain.StatusReceived,
		"Recebida":             domain.StatusReceived,
		"em_diagnostico":       domain.StatusDiagnosis,
		"Em Diagnóstico":       domain.StatusDiagnosis,
		"em-diagnostico":       domain.StatusDiagnosis,
		"aguardando_aprovacao": domain.StatusWaitingApproval,
		"Aguardando Aprovação": domain.StatusWaitingApproval,
		"em_execucao":          domain.StatusInProgress,
		"Em Execução":          domain.StatusInProgress,
		"finalizada":           domain.StatusFinished,
		"Finalizada":           domain.StatusFinished,
		"entregue":             domain.StatusDelivered,
		"Entregue":             domain.StatusDelivered,
		"recusada":             domain.StatusRejected,
		"Recusada":             domain.StatusRejected,
	}

	for raw, expected := range cases {
		status, ok := domain.NormalizeStatusAlias(raw)
		assert.True(t, ok, "esperava reconhecer %q", raw)
		assert.Equal(t, expected, status, "para entrada %q", raw)
	}
}

func TestNormalizeStatusAlias_RejectsSynonymsNotInEnum(t *testing.T) {
	unknown := []string{
		"aprovado", "aprovada",
		"reprovado", "reprovada",
		"diagnostico",
		"execucao",
		"finalizado",
		"recusado",
		"status_que_nao_existe",
	}

	for _, raw := range unknown {
		_, ok := domain.NormalizeStatusAlias(raw)
		assert.False(t, ok, "não esperava reconhecer %q", raw)
	}
}

func TestIsValidStatus(t *testing.T) {
	assert.True(t, domain.IsValidStatus("recebida"))
	assert.True(t, domain.IsValidStatus("em_execucao"))
	assert.False(t, domain.IsValidStatus("status_invalido"))
	assert.False(t, domain.IsValidStatus(""))
}
