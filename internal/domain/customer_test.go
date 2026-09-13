package domain_test

import (
	"autoshop/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomerValidate_Success(t *testing.T) {
	customer := domain.Customer{
		ID:    "1",
		Name:  "João Silva",
		CPF:   "529.982.247-25",
		Email: "joao@email.com",
		Phone: "11999999999",
		Address: domain.Address{
			Street:  "Rua das Flores",
			Number:  "123",
			City:    "São Paulo",
			State:   "SP",
			ZipCode: "01310-100",
		},
	}

	err := customer.Validate()
	assert.NoError(t, err)
}

func TestCustomerValidate_MissingName(t *testing.T) {
	customer := domain.Customer{
		CPF:   "529.982.247-25",
		Email: "joao@email.com",
		Phone: "11999999999",
		Address: domain.Address{
			Street:  "Rua das Flores",
			Number:  "123",
			City:    "São Paulo",
			State:   "SP",
			ZipCode: "01310-100",
		},
	}

	err := customer.Validate()
	assert.EqualError(t, err, "nome é obrigatório")
}

func TestCustomerValidate_MissingDocument(t *testing.T) {
	customer := domain.Customer{
		Name:  "João Silva",
		Email: "joao@email.com",
		Phone: "11999999999",
		Address: domain.Address{
			Street:  "Rua das Flores",
			Number:  "123",
			City:    "São Paulo",
			State:   "SP",
			ZipCode: "01310-100",
		},
	}

	err := customer.Validate()
	assert.EqualError(t, err, "CPF ou CNPJ é obrigatório")
}

func TestCustomerValidate_InvalidCPF(t *testing.T) {
	customer := domain.Customer{
		Name:  "João Silva",
		CPF:   "111.111.111-11",
		Email: "joao@email.com",
		Phone: "11999999999",
		Address: domain.Address{
			Street:  "Rua das Flores",
			Number:  "123",
			City:    "São Paulo",
			State:   "SP",
			ZipCode: "01310-100",
		},
	}

	err := customer.Validate()
	assert.EqualError(t, err, "CPF inválido")
}

func TestCustomerValidate_InvalidEmail(t *testing.T) {
	customer := domain.Customer{
		Name:  "João Silva",
		CPF:   "529.982.247-25",
		Email: "email-invalido",
		Phone: "11999999999",
		Address: domain.Address{
			Street:  "Rua das Flores",
			Number:  "123",
			City:    "São Paulo",
			State:   "SP",
			ZipCode: "01310-100",
		},
	}

	err := customer.Validate()
	assert.EqualError(t, err, "email inválido")
}

func TestCustomerValidate_MissingAddress(t *testing.T) {
	customer := domain.Customer{
		Name:  "João Silva",
		CPF:   "529.982.247-25",
		Email: "joao@email.com",
		Phone: "11999999999",
	}

	err := customer.Validate()
	assert.EqualError(t, err, "endereço incompleto: rua, cidade, estado e CEP são obrigatórios")
}
