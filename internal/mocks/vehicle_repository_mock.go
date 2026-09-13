package mocks

import (
	"autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type VehicleRepositoryMock struct {
	mock.Mock
}

func (m *VehicleRepositoryMock) Create(vehicle domain.Vehicle) (domain.Vehicle, error) {
	args := m.Called(vehicle)
	return args.Get(0).(domain.Vehicle), args.Error(1)
}

func (m *VehicleRepositoryMock) FindAll() ([]domain.Vehicle, error) {
	args := m.Called()
	return args.Get(0).([]domain.Vehicle), args.Error(1)
}

func (m *VehicleRepositoryMock) FindByID(id string) (domain.Vehicle, error) {
	args := m.Called(id)
	return args.Get(0).(domain.Vehicle), args.Error(1)
}

func (m *VehicleRepositoryMock) FindByCustomerID(customerID string) ([]domain.Vehicle, error) {
	args := m.Called(customerID)
	return args.Get(0).([]domain.Vehicle), args.Error(1)
}

func (m *VehicleRepositoryMock) Update(vehicle domain.Vehicle) error {
	args := m.Called(vehicle)
	return args.Error(0)
}

func (m *VehicleRepositoryMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
