package domain_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceValidate_Success(t *testing.T) {
	service := domain.NewService("Troca de óleo", "Troca completa", 150.0, 60)
	service.ID = "1"

	err := service.Validate()
	assert.NoError(t, err)
}

func TestServiceValidate_MissingName(t *testing.T) {
	service := domain.NewService("", "Troca completa", 150.0, 60)

	err := service.Validate()
	assert.EqualError(t, err, "nome do serviço é obrigatório")
}

func TestServiceValidate_InvalidPrice(t *testing.T) {
	service := domain.NewService("Troca de óleo", "Troca completa", 0, 60)

	err := service.Validate()
	assert.EqualError(t, err, "preço do serviço deve ser maior que zero")
}

func TestServiceValidate_InvalidTime(t *testing.T) {
	service := domain.NewService("Troca de óleo", "Troca completa", 150.0, 0)

	err := service.Validate()
	assert.EqualError(t, err, "tempo estimado deve ser maior que zero")
}
