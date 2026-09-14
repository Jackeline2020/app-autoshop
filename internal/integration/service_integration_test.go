package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceIntegration_CreateAndRetrieve(t *testing.T) {
	service, err := serviceUseCase.Create("Alinhamento", "Alinhamento e balanceamento", 80.0, 45)
	assert.NoError(t, err)
	assert.NotEmpty(t, service.ID)

	found, err := serviceUseCase.GetByID(service.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Alinhamento", found.Name)
}

func TestServiceIntegration_Update(t *testing.T) {
	service, err := serviceUseCase.Create("Troca de correia", "Correia dentada", 200.0, 90)
	assert.NoError(t, err)

	updated, err := serviceUseCase.Update(service.ID, "Troca de correia dentada", "Correia dentada + tensor", 250.0, 100)
	assert.NoError(t, err)
	assert.Equal(t, 250.0, updated.Price)
	assert.Equal(t, 100, updated.EstimatedTime)

	found, err := serviceUseCase.GetByID(service.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Troca de correia dentada", found.Name)
}

func TestServiceIntegration_Delete(t *testing.T) {
	service, err := serviceUseCase.Create("Revisão geral", "Revisão completa", 300.0, 120)
	assert.NoError(t, err)

	err = serviceUseCase.Delete(service.ID)
	assert.NoError(t, err)

	_, err = serviceUseCase.GetByID(service.ID)
	assert.Error(t, err)
}

func TestServiceIntegration_NotFound(t *testing.T) {
	_, err := serviceUseCase.GetByID("id-inexistente")
	assert.Error(t, err)
}
