package integration_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomerIntegration_CreateAndRetrieve(t *testing.T) {
	address := domain.Address{
		Street:  "Rua das Flores",
		Number:  "123",
		City:    "São Paulo",
		State:   "SP",
		ZipCode: "01310-100",
	}

	// Cria cliente
	customer, err := customerUseCase.Create("João Silva", "529.982.247-25", "", "joao@email.com", "11999999999", address)
	assert.NoError(t, err)
	assert.NotEmpty(t, customer.ID)

	// Busca pelo ID
	found, err := customerUseCase.GetByID(customer.ID)
	assert.NoError(t, err)
	assert.Equal(t, customer.ID, found.ID)
	assert.Equal(t, "João Silva", found.Name)
}

func TestCustomerIntegration_Update(t *testing.T) {
	address := domain.Address{
		Street:  "Rua das Flores",
		Number:  "123",
		City:    "São Paulo",
		State:   "SP",
		ZipCode: "01310-100",
	}

	customer, err := customerUseCase.Create("Maria Silva", "987.654.321-00", "", "maria@email.com", "11999999999", address)
	assert.NoError(t, err)
	assert.NotEmpty(t, customer.ID)

	updated, err := customerUseCase.Update(customer.ID, "Maria Souza", "987.654.321-00", "", "mariasouza@email.com", "11999999999", address)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "Maria Souza", updated.Name)

	found, err := customerUseCase.GetByID(customer.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Maria Souza", found.Name)
}

func TestCustomerIntegration_Delete(t *testing.T) {
	address := domain.Address{
		Street:  "Rua das Flores",
		Number:  "123",
		City:    "São Paulo",
		State:   "SP",
		ZipCode: "01310-100",
	}

	customer, _ := customerUseCase.Create("Pedro Lima", "135.792.468-28", "", "pedro@email.com", "11999999999", address)

	err := customerUseCase.Delete(customer.ID)
	assert.NoError(t, err)

	_, err = customerUseCase.GetByID(customer.ID)
	assert.Error(t, err)
}

func TestCustomerIntegration_NotFound(t *testing.T) {
	_, err := customerUseCase.GetByID("id-inexistente")
	assert.Error(t, err)
}
