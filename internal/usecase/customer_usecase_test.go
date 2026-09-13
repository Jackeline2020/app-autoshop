package usecase_test

import (
	"autoshop/internal/domain"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func makeValidAddress() domain.Address {
	return domain.Address{
		Street:  "Rua das Flores",
		Number:  "123",
		City:    "São Paulo",
		State:   "SP",
		ZipCode: "01310-100",
	}
}

func TestCustomerUseCase_Create_Success(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("Create", mock.AnythingOfType("domain.Customer")).
		Return(domain.Customer{
			ID:    "uuid",
			Name:  "João Silva",
			CPF:   "52998224725",
			Email: "joao@email.com",
			Phone: "11999999999",
		}, nil)

	customer, err := uc.Create("João Silva", "529.982.247-25", "", "joao@email.com", "11999999999", makeValidAddress())

	assert.NoError(t, err)
	assert.Equal(t, "João Silva", customer.Name)
	repo.AssertExpectations(t)
}

func TestCustomerUseCase_Create_InvalidCPF(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	_, err := uc.Create("João Silva", "111.111.111-11", "", "joao@email.com", "11999999999", makeValidAddress())

	assert.EqualError(t, err, "CPF inválido")
	repo.AssertNotCalled(t, "Create")
}

func TestCustomerUseCase_Create_MissingDocument(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	_, err := uc.Create("João Silva", "", "", "joao@email.com", "11999999999", makeValidAddress())

	assert.EqualError(t, err, "CPF ou CNPJ é obrigatório")
	repo.AssertNotCalled(t, "Create")
}

func TestCustomerUseCase_GetByID_NotFound(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "id-inexistente").
		Return(domain.Customer{}, errors.New("customer not found"))

	_, err := uc.GetByID("id-inexistente")

	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestCustomerUseCase_Delete_Success(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "uuid").
		Return(domain.Customer{ID: "uuid", Name: "João"}, nil)
	repo.On("Delete", "uuid").Return(nil)

	err := uc.Delete("uuid")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestCustomerUseCase_Delete_NotFound(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "id-inexistente").
		Return(domain.Customer{}, errors.New("customer not found"))

	err := uc.Delete("id-inexistente")

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Delete")
}

func TestCustomerUseCase_GetAll_Success(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindAll").Return([]domain.Customer{
		{ID: "1", Name: "João"},
		{ID: "2", Name: "Maria"},
	}, nil)

	customers, err := uc.GetAll()

	assert.NoError(t, err)
	assert.Len(t, customers, 2)
}

func TestCustomerUseCase_GetByID_Success(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid", Name: "João"}, nil)

	customer, err := uc.GetByID("uuid")

	assert.NoError(t, err)
	assert.Equal(t, "uuid", customer.ID)
}

func TestCustomerUseCase_Update_Success(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "uuid").Return(domain.Customer{
		ID:    "uuid",
		Name:  "João",
		CPF:   "52998224725",
		Email: "joao@email.com",
		Phone: "11999999999",
		Address: domain.Address{
			Street: "Rua A", Number: "1", City: "SP", State: "SP", ZipCode: "01310-100",
		},
	}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Customer")).Return(nil)

	customer, err := uc.Update("uuid", "João Atualizado", "52998224725", "", "novo@email.com", "11988888888", makeValidAddress())

	assert.NoError(t, err)
	assert.Equal(t, "João Atualizado", customer.Name)
}

func TestCustomerUseCase_Update_NotFound(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "uuid").Return(domain.Customer{}, errors.New("customer not found"))

	_, err := uc.Update("uuid", "João", "52998224725", "", "joao@email.com", "11999999999", makeValidAddress())

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Update")
}

func TestCustomerUseCase_Update_InvalidCPF(t *testing.T) {
	repo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewCustomerUseCase(repo)

	repo.On("FindByID", "uuid").Return(domain.Customer{ID: "uuid"}, nil)

	_, err := uc.Update("uuid", "João", "111.111.111-11", "", "joao@email.com", "11999999999", makeValidAddress())

	assert.EqualError(t, err, "CPF inválido")
	repo.AssertNotCalled(t, "Update")
}
