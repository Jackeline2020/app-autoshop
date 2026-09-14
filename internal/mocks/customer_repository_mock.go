package mocks

import (
	"autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type CustomerRepositoryMock struct {
	mock.Mock
}

func (m *CustomerRepositoryMock) Create(customer domain.Customer) (domain.Customer, error) {
	args := m.Called(customer)
	return args.Get(0).(domain.Customer), args.Error(1)
}

func (m *CustomerRepositoryMock) FindAll() ([]domain.Customer, error) {
	args := m.Called()
	return args.Get(0).([]domain.Customer), args.Error(1)
}

func (m *CustomerRepositoryMock) FindByID(id string) (domain.Customer, error) {
	args := m.Called(id)
	return args.Get(0).(domain.Customer), args.Error(1)
}

func (m *CustomerRepositoryMock) Update(customer domain.Customer) error {
	args := m.Called(customer)
	return args.Error(0)
}

func (m *CustomerRepositoryMock) UpdateStatus(id, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *CustomerRepositoryMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
