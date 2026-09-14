package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartIntegration_CreateAndRetrieve(t *testing.T) {
	part, err := partUseCase.Create("Pastilha de freio", "Pastilha dianteira", "par", 90.0, 20, 5)
	assert.NoError(t, err)
	assert.NotEmpty(t, part.ID)

	found, err := partUseCase.GetByID(part.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Pastilha de freio", found.Name)
	assert.Equal(t, 20, found.Stock)
}

func TestPartIntegration_LowStock(t *testing.T) {
	_, err := partUseCase.Create("Correia dentada", "Correia", "unidade", 120.0, 1, 5)
	assert.NoError(t, err)

	lowStock, err := partUseCase.GetLowStock()
	assert.NoError(t, err)

	found := false
	for _, p := range lowStock {
		if p.Name == "Correia dentada" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestPartIntegration_AdjustStock(t *testing.T) {
	part, err := partUseCase.Create("Óleo de motor", "Óleo sintético", "litro", 45.0, 10, 3)
	require.NoError(t, err)

	updated, err := partUseCase.AdjustStock(part.ID, -4, "saída")
	require.NoError(t, err)
	assert.Equal(t, 6, updated.Stock)

	_, err = partUseCase.AdjustStock(part.ID, -100, "saída")
	assert.Error(t, err)
}

func TestPartIntegration_Delete(t *testing.T) {
	part, err := partUseCase.Create("Parafuso", "Parafuso genérico", "unidade", 1.0, 100, 20)
	assert.NoError(t, err)

	err = partUseCase.Delete(part.ID)
	assert.NoError(t, err)

	_, err = partUseCase.GetByID(part.ID)
	assert.Error(t, err)
}
