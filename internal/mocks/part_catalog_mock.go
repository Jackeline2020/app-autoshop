package mocks

import (
	"autoshop/internal/client"
	"context"

	"github.com/stretchr/testify/mock"
)

// PartCatalogMock simula a consulta de peças no Execution Service.
type PartCatalogMock struct {
	mock.Mock
}

func (m *PartCatalogMock) FindPart(_ context.Context, id string) (client.CatalogPart, error) {
	args := m.Called(id)
	return args.Get(0).(client.CatalogPart), args.Error(1)
}
