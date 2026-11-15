package usecase_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/mocks"
	"autoshop/internal/repository"
	"autoshop/internal/usecase"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomerUseCase_GetByCPF(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)
	repo.On("FindByCPF", "52998224725").Return(domain.Customer{ID: "c1", Status: domain.CustomerStatusActive}, nil)
	repo.On("FindByCPF", "11144477735").Return(domain.Customer{}, repository.ErrCustomerNotFound)

	c, err := uc.GetByCPF("529.982.247-25")
	assert.NoError(t, err)
	assert.Equal(t, "c1", c.ID)

	_, err = uc.GetByCPF("111.444.777-35")
	assert.ErrorIs(t, err, repository.ErrCustomerNotFound)

	_, err = uc.GetByCPF("123")
	assert.ErrorIs(t, err, usecase.ErrInvalidCPF)
	repo.AssertNumberOfCalls(t, "FindByCPF", 2)
}
