package validator_test

import (
	"autoshop/pkg/validator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidCPF_Valid(t *testing.T) {
	validCPFs := []string{
		"529.982.247-25",
		"52998224725",
		"111.444.777-35",
	}

	for _, cpf := range validCPFs {
		assert.True(t, validator.IsValidCPF(cpf), "CPF deveria ser válido: %s", cpf)
	}
}

func TestIsValidCPF_Invalid(t *testing.T) {
	invalidCPFs := []string{
		"111.111.111-11",
		"000.000.000-00",
		"123.456.789-00",
		"12345",
		"",
	}

	for _, cpf := range invalidCPFs {
		assert.False(t, validator.IsValidCPF(cpf), "CPF deveria ser inválido: %s", cpf)
	}
}

func TestIsValidCNPJ_Valid(t *testing.T) {
	validCNPJs := []string{
		"11.222.333/0001-81",
		"11222333000181",
	}

	for _, cnpj := range validCNPJs {
		assert.True(t, validator.IsValidCNPJ(cnpj), "CNPJ deveria ser válido: %s", cnpj)
	}
}

func TestIsValidCNPJ_Invalid(t *testing.T) {
	invalidCNPJs := []string{
		"11.111.111/1111-11",
		"00.000.000/0000-00",
		"12345",
		"",
	}

	for _, cnpj := range invalidCNPJs {
		assert.False(t, validator.IsValidCNPJ(cnpj), "CNPJ deveria ser inválido: %s", cnpj)
	}
}
