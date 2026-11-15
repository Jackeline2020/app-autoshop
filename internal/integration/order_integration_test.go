package integration_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/usecase"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CPF e placa têm UNIQUE constraint no Postgres, então cada
// chamada precisa de valores próprios para não colidir com as outras.
func setupOrderData(t *testing.T, cpf, plate string) (customerID, vehicleID, serviceID string) {
	address := domain.Address{
		Street: "Rua A", Number: "1", City: "SP", State: "SP", ZipCode: "01310-100",
	}

	customer, err := customerUseCase.Create("Cliente Teste", cpf, "", "teste@email.com", "11999999999", address)
	require.NoError(t, err)

	vehicle, err := vehicleUseCase.Create(customer.ID, plate, "Toyota", "Corolla", 2020)
	require.NoError(t, err)

	service, err := serviceUseCase.Create("Troca de óleo", "Troca completa", 150.0, 60)
	require.NoError(t, err)

	return customer.ID, vehicle.ID, service.ID
}

// advance aplica transições controladas pela Saga diretamente no
// repositório (o orquestrador é testado separadamente).
func advance(t *testing.T, orderID string, statuses ...domain.OrderStatus) {
	t.Helper()
	for _, s := range statuses {
		order, err := orderRepo.FindByID(orderID)
		require.NoError(t, err)
		require.NoError(t, order.TransitionTo(s))
		require.NoError(t, orderRepo.Update(order))
	}
}

func TestOrderIntegration_CreateWithCatalogParts(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "111.444.777-35", "ABC-1234")

	order, err := orderUseCase.Create(
		customerID, vehicleID, "Barulho no motor",
		[]string{serviceID},
		[]struct {
			PartID   string
			Quantity int
		}{{PartID: "peca-filtro", Quantity: 2}},
	)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReceived, order.Status)
	assert.Equal(t, 250.0, order.Total)

	// A cópia da peça (nome e preço do catálogo) fica gravada na OS
	saved, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	require.Len(t, saved.Parts, 1)
	assert.Equal(t, "peca-filtro", saved.Parts[0].PartID)
	assert.Equal(t, "Filtro de óleo", saved.Parts[0].PartName)
	assert.Equal(t, 50.0, saved.Parts[0].UnitPrice)
}

func TestOrderIntegration_UnknownPart(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "714.285.039-60", "MNO9Z12")

	_, err := orderUseCase.Create(customerID, vehicleID, "", []string{serviceID},
		[]struct {
			PartID   string
			Quantity int
		}{{PartID: "nao-existe", Quantity: 1}})
	assert.ErrorContains(t, err, "peça não encontrada")
}

func TestOrderIntegration_ApprovalAndDelivery(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "861.234.570-71", "DEF4G56")

	order, err := orderUseCase.Create(customerID, vehicleID, "", []string{serviceID}, nil)
	require.NoError(t, err)
	advance(t, order.ID, domain.StatusDiagnosis, domain.StatusWaitingApproval)

	approved, err := orderUseCase.ApproveOrder(order.ID, true, "")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusWaitingPayment, approved.Status)

	advance(t, order.ID, domain.StatusWaitingRepair, domain.StatusInProgress, domain.StatusFinished)

	delivered, err := orderUseCase.UpdateStatus(order.ID, string(domain.StatusDelivered))
	require.NoError(t, err)
	assert.Equal(t, domain.StatusDelivered, delivered.Status)
	assert.NotEmpty(t, delivered.DeliveredAt)
}

func TestOrderIntegration_RejectedFlow(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "333.444.555-08", "XYZ-5678")

	order, err := orderUseCase.Create(customerID, vehicleID, "Revisão geral", []string{serviceID}, nil)
	require.NoError(t, err)
	advance(t, order.ID, domain.StatusDiagnosis, domain.StatusWaitingApproval)

	rejected, err := orderUseCase.ApproveOrder(order.ID, false, "Valor alto demais")
	require.NoError(t, err)
	assert.Equal(t, domain.StatusRejected, rejected.Status)
	assert.Contains(t, rejected.Notes, "Valor alto demais")
}

func TestOrderIntegration_CancelledStatusIsPersisted(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "390.533.447-05", "JKL7M89")

	order, err := orderUseCase.Create(customerID, vehicleID, "", []string{serviceID}, nil)
	require.NoError(t, err)
	advance(t, order.ID, domain.StatusCancelled)

	saved, err := orderUseCase.GetByID(order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, saved.Status)
}

func TestOrderIntegration_ManualStatusOnlyDelivery(t *testing.T) {
	customerID, vehicleID, serviceID := setupOrderData(t, "248.438.034-80", "PQR1S23")

	order, err := orderUseCase.Create(customerID, vehicleID, "", []string{serviceID}, nil)
	require.NoError(t, err)

	_, err = orderUseCase.UpdateStatus(order.ID, string(domain.StatusDiagnosis))
	assert.ErrorIs(t, err, usecase.ErrManualStatusNotAllowed)
}
