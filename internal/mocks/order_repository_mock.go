package mocks

import (
	"autoshop/internal/domain"

	"github.com/stretchr/testify/mock"
)

type OrderRepositoryMock struct {
	mock.Mock
}

func (m *OrderRepositoryMock) Create(order domain.Order) (domain.Order, error) {
	args := m.Called(order)
	return args.Get(0).(domain.Order), args.Error(1)
}

func (m *OrderRepositoryMock) FindAll() ([]domain.Order, error) {
	args := m.Called()
	return args.Get(0).([]domain.Order), args.Error(1)
}

func (m *OrderRepositoryMock) FindByID(id string) (domain.Order, error) {
	args := m.Called(id)
	return args.Get(0).(domain.Order), args.Error(1)
}

func (m *OrderRepositoryMock) FindByCustomerID(customerID string) ([]domain.Order, error) {
	args := m.Called(customerID)
	return args.Get(0).([]domain.Order), args.Error(1)
}

func (m *OrderRepositoryMock) FindByStatus(status domain.OrderStatus) ([]domain.Order, error) {
	args := m.Called(status)
	return args.Get(0).([]domain.Order), args.Error(1)
}

func (m *OrderRepositoryMock) Update(order domain.Order) error {
	args := m.Called(order)
	return args.Error(0)
}

func (m *OrderRepositoryMock) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
