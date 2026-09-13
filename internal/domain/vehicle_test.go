package domain_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVehicleValidate_Success(t *testing.T) {
	vehicle := domain.Vehicle{
		ID:         "1",
		CustomerID: "123",
		Plate:      "ABC1234",
		Brand:      "Toyota",
		Model:      "Corolla",
		Year:       2020,
	}

	err := vehicle.Validate()
	assert.NoError(t, err)
}

func TestVehicleValidate_MercosulPlate(t *testing.T) {
	vehicle := domain.Vehicle{
		ID:         "1",
		CustomerID: "123",
		Plate:      "ABC1D23",
		Brand:      "Toyota",
		Model:      "Corolla",
		Year:       2020,
	}

	err := vehicle.Validate()
	assert.NoError(t, err)
}

func TestVehicleValidate_InvalidPlate(t *testing.T) {
	vehicle := domain.Vehicle{
		CustomerID: "123",
		Plate:      "INVALIDA",
		Brand:      "Toyota",
		Model:      "Corolla",
		Year:       2020,
	}

	err := vehicle.Validate()
	assert.EqualError(t, err, "placa inválida. Use o formato antigo (ABC-1234) ou Mercosul (ABC1D23)")
}

func TestVehicleValidate_InvalidYear(t *testing.T) {
	vehicle := domain.Vehicle{
		CustomerID: "123",
		Plate:      "ABC1234",
		Brand:      "Toyota",
		Model:      "Corolla",
		Year:       1800,
	}

	err := vehicle.Validate()
	assert.EqualError(t, err, "ano do veículo inválido")
}

func TestVehicleValidate_MissingCustomer(t *testing.T) {
	vehicle := domain.Vehicle{
		Plate: "ABC1234",
		Brand: "Toyota",
		Model: "Corolla",
		Year:  2020,
	}

	err := vehicle.Validate()
	assert.EqualError(t, err, "cliente é obrigatório")
}
