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

func setupServiceUseCase() (*usecase.ServiceUseCase, *mocks.ServiceRepositoryMock) {
	repo := new(mocks.ServiceRepositoryMock)
	uc := usecase.NewServiceUseCase(repo)
	return uc, repo
}

func TestServiceUseCase_Create_Success(t *testing.T) {
	uc, repo := setupServiceUseCase()

	repo.On("Create", mock.AnythingOfType("domain.Service")).
		Return(domain.Service{ID: "sid", Name: "Troca de óleo", Price: 150.0}, nil)

	service, err := uc.Create("Troca de óleo", "Troca completa", 150.0, 60)

	assert.NoError(t, err)
	assert.Equal(t, "Troca de óleo", service.Name)
	repo.AssertExpectations(t)
}

func TestServiceUseCase_Create_InvalidPrice(t *testing.T) {
	uc, repo := setupServiceUseCase()

	_, err := uc.Create("Troca de óleo", "", 0, 60)

	assert.EqualError(t, err, "preço do serviço deve ser maior que zero")
	repo.AssertNotCalled(t, "Create")
}

func TestServiceUseCase_Create_InvalidTime(t *testing.T) {
	uc, repo := setupServiceUseCase()

	_, err := uc.Create("Troca de óleo", "", 150.0, 0)

	assert.EqualError(t, err, "tempo estimado deve ser maior que zero")
	repo.AssertNotCalled(t, "Create")
}

func TestServiceUseCase_GetAll_Success(t *testing.T) {
	uc, repo := setupServiceUseCase()

	repo.On("FindAll").Return([]domain.Service{
		{ID: "sid1", Name: "Troca de óleo"},
		{ID: "sid2", Name: "Alinhamento"},
	}, nil)

	services, err := uc.GetAll()

	assert.NoError(t, err)
	assert.Len(t, services, 2)
}

func TestServiceUseCase_Update_Success(t *testing.T) {
	uc, repo := setupServiceUseCase()

	repo.On("FindByID", "sid").Return(domain.Service{
		ID: "sid", Name: "Troca de óleo", Price: 150.0, EstimatedTime: 60,
	}, nil)
	repo.On("Update", mock.AnythingOfType("domain.Service")).Return(nil)

	service, err := uc.Update("sid", "Alinhamento", "Alinhamento completo", 200.0, 90)

	assert.NoError(t, err)
	assert.Equal(t, "Alinhamento", service.Name)
	assert.Equal(t, 200.0, service.Price)
}

func TestServiceUseCase_Delete_Success(t *testing.T) {
	uc, repo := setupServiceUseCase()

	repo.On("FindByID", "sid").Return(domain.Service{ID: "sid"}, nil)
	repo.On("Delete", "sid").Return(nil)

	err := uc.Delete("sid")

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestServiceUseCase_Delete_NotFound(t *testing.T) {
	uc, repo := setupServiceUseCase()

	repo.On("FindByID", "sid").Return(domain.Service{}, errors.New("serviço não encontrado"))

	err := uc.Delete("sid")

	assert.Error(t, err)
	repo.AssertNotCalled(t, "Delete")
}
