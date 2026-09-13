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

func setupVehicleUseCase() (*usecase.VehicleUseCase, *mocks.VehicleRepositoryMock, *mocks.CustomerRepositoryMock) {
	vehicleRepo := new(mocks.VehicleRepositoryMock)
	customerRepo := new(mocks.CustomerRepositoryMock)
	uc := usecase.NewVehicleUseCase(vehicleRepo, customerRepo)
	return uc, vehicleRepo, customerRepo
}

func TestVehicleUseCase_Create_Success(t *testing.T) {
	uc, vehicleRepo, customerRepo := setupVehicleUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)
	vehicleRepo.On("Create", mock.AnythingOfType("domain.Vehicle")).
		Return(domain.Vehicle{ID: "vid", Plate: "ABC1234"}, nil)

	vehicle, err := uc.Create("cid", "ABC-1234", "Toyota", "Corolla", 2020)

	assert.NoError(t, err)
	assert.Equal(t, "ABC1234", vehicle.Plate)
	vehicleRepo.AssertExpectations(t)
}

func TestVehicleUseCase_Create_CustomerNotFound(t *testing.T) {
	uc, _, customerRepo := setupVehicleUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{}, errors.New("not found"))

	_, err := uc.Create("cid", "ABC1234", "Toyota", "Corolla", 2020)

	assert.EqualError(t, err, "cliente não encontrado")
}

func TestVehicleUseCase_Create_InvalidPlate(t *testing.T) {
	uc, _, customerRepo := setupVehicleUseCase()

	customerRepo.On("FindByID", "cid").Return(domain.Customer{ID: "cid"}, nil)

	_, err := uc.Create("cid", "INVALIDA", "Toyota", "Corolla", 2020)

	assert.ErrorContains(t, err, "placa inválida")
}

func TestVehicleUseCase_GetByID_Success(t *testing.T) {
	uc, vehicleRepo, _ := setupVehicleUseCase()

	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid", Plate: "ABC1234"}, nil)

	vehicle, err := uc.GetByID("vid")

	assert.NoError(t, err)
	assert.Equal(t, "vid", vehicle.ID)
}

func TestVehicleUseCase_GetByID_NotFound(t *testing.T) {
	uc, vehicleRepo, _ := setupVehicleUseCase()

	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{}, errors.New("veículo não encontrado"))

	_, err := uc.GetByID("vid")

	assert.Error(t, err)
}

func TestVehicleUseCase_Update_Success(t *testing.T) {
	uc, vehicleRepo, _ := setupVehicleUseCase()

	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{
		ID: "vid", CustomerID: "cid", Plate: "ABC1234", Brand: "Toyota", Model: "Corolla", Year: 2020,
	}, nil)
	vehicleRepo.On("Update", mock.AnythingOfType("domain.Vehicle")).Return(nil)

	vehicle, err := uc.Update("vid", "ABC1D23", "Honda", "Civic", 2021)

	assert.NoError(t, err)
	assert.Equal(t, "Honda", vehicle.Brand)
}

func TestVehicleUseCase_Delete_Success(t *testing.T) {
	uc, vehicleRepo, _ := setupVehicleUseCase()

	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{ID: "vid"}, nil)
	vehicleRepo.On("Delete", "vid").Return(nil)

	err := uc.Delete("vid")

	assert.NoError(t, err)
	vehicleRepo.AssertExpectations(t)
}

func TestVehicleUseCase_Delete_NotFound(t *testing.T) {
	uc, vehicleRepo, _ := setupVehicleUseCase()

	vehicleRepo.On("FindByID", "vid").Return(domain.Vehicle{}, errors.New("veículo não encontrado"))

	err := uc.Delete("vid")

	assert.Error(t, err)
	vehicleRepo.AssertNotCalled(t, "Delete")
}
