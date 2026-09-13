package integration_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

// CPF e placa têm UNIQUE constraint no Postgres, então cada
// chamada precisa de valores próprios para não colidir com as outras.
func setupOrderData(t *testing.T, cpf, plate string) (customerID, vehicleID, serviceID, partID string) {
	address := domain.Address{
		Street: "Rua A", Number: "1", City: "SP", State: "SP", ZipCode: "01310-100",
	}

	customer, err := customerUseCase.Create("Cliente Teste", cpf, "", "teste@email.com", "11999999999", address)
	assert.NoError(t, err)

	vehicle, err := vehicleUseCase.Create(customer.ID, plate, "Toyota", "Corolla", 2020)
	assert.NoError(t, err)

	service, err := serviceUseCase.Create("Troca de óleo", "Troca completa", 150.0, 60)
	assert.NoError(t, err)

	part, err := partUseCase.Create("Filtro de óleo", "Filtro", "unidade", 50.0, 10, 2)
	assert.NoError(t, err)

	return customer.ID, vehicle.ID, service.ID, part.ID
}

func TestOrderIntegration_FullFlow(t *testing.T) {
	customerID, vehicleID, serviceID, partID := setupOrderData(t, "111.444.777-35", "ABC-1234")

	// 1. Cria OS
	order, err := orderUseCase.Create(
		customerID, vehicleID, "Barulho no motor",
		[]string{serviceID},
		[]struct {
			PartID   string
			Quantity int
		}{{PartID: partID, Quantity: 2}},
	)
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusReceived, order.Status)
	assert.Equal(t, 250.0, order.Total)

	// 2. Avança para diagnóstico
	updatedOrder, err := orderUseCase.UpdateStatus(order.ID, string(domain.StatusDiagnosis))
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDiagnosis, updatedOrder.Status)

	// 3. Avança para aguardando aprovação
	updatedOrder, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusWaitingApproval))
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusWaitingApproval, updatedOrder.Status)

	// 4. Cliente aprova
	updatedOrder, err = orderUseCase.ApproveOrder(order.ID, true, "")
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusInProgress, updatedOrder.Status)
	assert.NotEmpty(t, updatedOrder.StartedAt)

	// 5. Finaliza
	updatedOrder, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusFinished))
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusFinished, updatedOrder.Status)
	assert.NotEmpty(t, updatedOrder.FinishedAt)

	// 6. Entrega
	updatedOrder, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusDelivered))
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusDelivered, updatedOrder.Status)
	assert.NotEmpty(t, updatedOrder.DeliveredAt)
}

func TestOrderIntegration_RejectedFlow(t *testing.T) {
	customerID, vehicleID, serviceID, _ := setupOrderData(t, "333.444.555-08", "XYZ-5678")

	order, err := orderUseCase.Create(
		customerID, vehicleID, "Revisão geral",
		[]string{serviceID}, nil,
	)
	assert.NoError(t, err)

	orderUseCase.UpdateStatus(order.ID, string(domain.StatusDiagnosis))
	orderUseCase.UpdateStatus(order.ID, string(domain.StatusWaitingApproval))

	updatedOrder, err := orderUseCase.ApproveOrder(order.ID, false, "Valor alto demais")
	assert.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, updatedOrder.Status)
	assert.Contains(t, updatedOrder.Notes, "Valor alto demais")
}

func TestOrderIntegration_StockDeducted(t *testing.T) {
	customerID, vehicleID, serviceID, partID := setupOrderData(t, "714.285.039-60", "MNO9Z12")

	// Busca estoque inicial diretamente do banco
	part, err := partUseCase.GetByID(partID)
	assert.NoError(t, err)
	initialStock := part.Stock
	t.Logf("Part ID: %s, Initial Stock: %d", partID, initialStock)

	order, err := orderUseCase.Create(
		customerID, vehicleID, "",
		[]string{serviceID},
		[]struct {
			PartID   string
			Quantity int
		}{{PartID: partID, Quantity: 3}},
	)
	assert.NoError(t, err)
	t.Logf("Order ID: %s, Parts: %+v", order.ID, order.Parts)

	_, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusDiagnosis))
	assert.NoError(t, err)

	_, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusWaitingApproval))
	assert.NoError(t, err)

	updatedOrder, err := orderUseCase.ApproveOrder(order.ID, true, "")
	assert.NoError(t, err)
	t.Logf("After approve - Status: %s", updatedOrder.Status)

	updatedPart, err := partUseCase.GetByID(partID)
	assert.NoError(t, err)
	t.Logf("Updated Stock: %d, Expected: %d", updatedPart.Stock, initialStock-3)

	assert.Equal(t, initialStock-3, updatedPart.Stock)
}
