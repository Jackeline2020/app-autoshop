package mocks

import (
	"autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type ServiceRepositoryMock struct {
	mock.Mock
}

func (m *ServiceRepositoryMock) Create(service domain.Service) (domain.Service, error) {
	args := m.Called(service)
	return args.Get(0).(domain.Service), args.Error(1)
}

func (m *ServiceRepositoryMock) FindAll() ([]domain.Service, error) {
	args := m.Called()
	return args.Get(0).([]domain.Service), args.Error(1)
}

func (m *ServiceRepositoryMock) FindByID(id string) (domain.Service, error) {
	args := m.Called(id)
	return args.Get(0).(domain.Service), args.Error(1)
}

func (m *ServiceRepositoryMock) Update(service domain.Service) error {
	args := m.Called(service)
	return args.Error(0)
}

func (m *ServiceRepositoryMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
