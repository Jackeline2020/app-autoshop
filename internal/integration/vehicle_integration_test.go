package integration_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVehicleIntegration_CreateAndRetrieve(t *testing.T) {
	address := domain.Address{
		Street: "Rua das Flores", Number: "123", City: "São Paulo", State: "SP", ZipCode: "01310-100",
	}
	customer, err := customerUseCase.Create("Dono do Carro", "052.573.070-06", "", "dono@email.com", "11999999999", address)
	assert.NoError(t, err)

	vehicle, err := vehicleUseCase.Create(customer.ID, "ABC1234", "Fiat", "Uno", 2020)
	assert.NoError(t, err)
	assert.NotEmpty(t, vehicle.ID)
	assert.Equal(t, customer.ID, vehicle.CustomerID)

	found, err := vehicleUseCase.GetByID(vehicle.ID)
	assert.NoError(t, err)
	assert.Equal(t, "ABC1234", found.Plate)
}

func TestVehicleIntegration_Create_CustomerNotFound(t *testing.T) {
	_, err := vehicleUseCase.Create("cliente-inexistente", "XYZ9876", "Fiat", "Uno", 2020)
	assert.Error(t, err)
}

func TestVehicleIntegration_GetByCustomerID(t *testing.T) {
	address := domain.Address{
		Street: "Rua das Flores", Number: "123", City: "São Paulo", State: "SP", ZipCode: "01310-100",
	}
	customer, err := customerUseCase.Create("Dono de Vários Carros", "216.243.760-72", "", "varios@email.com", "11999999999", address)
	assert.NoError(t, err)

	_, err = vehicleUseCase.Create(customer.ID, "AAA1111", "Fiat", "Uno", 2019)
	assert.NoError(t, err)
	_, err = vehicleUseCase.Create(customer.ID, "BBB2222", "VW", "Gol", 2021)
	assert.NoError(t, err)

	vehicles, err := vehicleUseCase.GetByCustomerID(customer.ID)
	assert.NoError(t, err)
	assert.Len(t, vehicles, 2)
}

func TestVehicleIntegration_Update(t *testing.T) {
	address := domain.Address{
		Street: "Rua das Flores", Number: "123", City: "São Paulo", State: "SP", ZipCode: "01310-100",
	}
	customer, err := customerUseCase.Create("Dono Atualiza", "364.649.480-25", "", "atualiza@email.com", "11999999999", address)
	assert.NoError(t, err)

	vehicle, err := vehicleUseCase.Create(customer.ID, "CCC3333", "Fiat", "Uno", 2018)
	assert.NoError(t, err)

	updated, err := vehicleUseCase.Update(vehicle.ID, "DDD4444", "Fiat", "Palio", 2019)
	assert.NoError(t, err)
	assert.Equal(t, "DDD4444", updated.Plate)
	assert.Equal(t, "Palio", updated.Model)
}

func TestVehicleIntegration_Delete(t *testing.T) {
	address := domain.Address{
		Street: "Rua das Flores", Number: "123", City: "São Paulo", State: "SP", ZipCode: "01310-100",
	}
	customer, err := customerUseCase.Create("Dono Deleta", "263.470.310-16", "", "deleta@email.com", "11999999999", address)
	assert.NoError(t, err)

	vehicle, err := vehicleUseCase.Create(customer.ID, "EEE5555", "Fiat", "Uno", 2018)
	assert.NoError(t, err)

	err = vehicleUseCase.Delete(vehicle.ID)
	assert.NoError(t, err)

	_, err = vehicleUseCase.GetByID(vehicle.ID)
	assert.Error(t, err)
}
