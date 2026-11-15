package integration_test

import (
	"testing"

	"autoshop/internal/domain"
	"autoshop/internal/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A consulta interna por CPF encontra o cliente com ou sem pontuação, como
// a função de autenticação faz.
func TestCustomerIntegration_GetByCPF(t *testing.T) {
	created, err := customerUseCase.Create("Bruno Lima", "104.332.181-00", "", "bruno@email.com", "11988887777", domain.Address{
		Street: "Rua A", Number: "10", City: "São Paulo", State: "SP", ZipCode: "01000-000",
	})
	require.NoError(t, err)

	for _, cpf := range []string{"104.332.181-00", "10433218100"} {
		found, err := customerUseCase.GetByCPF(cpf)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, domain.CustomerStatusActive, found.Status)
	}

	_, err = customerUseCase.GetByCPF("960.013.389-14")
	assert.ErrorIs(t, err, repository.ErrCustomerNotFound)
}
