package domain

import (
	"autoshop/pkg/validator"
	"errors"
	"regexp"
)

type Address struct {
	Street     string `json:"street"                dynamodbav:"street"`
	Number     string `json:"number"                dynamodbav:"number"`
	Complement string `json:"complement,omitempty"  dynamodbav:"complement,omitempty"`
	City       string `json:"city"                  dynamodbav:"city"`
	State      string `json:"state"                 dynamodbav:"state"`
	ZipCode    string `json:"zip_code"              dynamodbav:"zip_code"`
}

// Status do cliente na base — usado pelo fluxo de autenticação por CPF
// (lambda-auth-autoshop/internal/authflow) além do "existe/não existe":
// um cliente inativo não recebe token mesmo tendo CPF válido e cadastrado.
const (
	CustomerStatusActive   = "ativo"
	CustomerStatusInactive = "inativo"
)

func IsValidCustomerStatus(status string) bool {
	return status == CustomerStatusActive || status == CustomerStatusInactive
}

type Customer struct {
	ID      string  `json:"id"                dynamodbav:"id"`
	Name    string  `json:"name"              dynamodbav:"name"`
	CPF     string  `json:"cpf,omitempty"     dynamodbav:"cpf,omitempty"`
	CNPJ    string  `json:"cnpj,omitempty"    dynamodbav:"cnpj,omitempty"`
	Email   string  `json:"email"             dynamodbav:"email"`
	Phone   string  `json:"phone"             dynamodbav:"phone"`
	Address Address `json:"address"           dynamodbav:"address"`
	Status  string  `json:"status"            dynamodbav:"status"`
}

func (c *Customer) Validate() error {
	if c.Name == "" {
		return errors.New("nome é obrigatório")
	}

	if c.CPF == "" && c.CNPJ == "" {
		return errors.New("CPF ou CNPJ é obrigatório")
	}

	if c.CPF != "" && !validator.IsValidCPF(c.CPF) {
		return errors.New("CPF inválido")
	}

	if c.CNPJ != "" && !validator.IsValidCNPJ(c.CNPJ) {
		return errors.New("CNPJ inválido")
	}

	if c.Email == "" {
		return errors.New("email é obrigatório")
	}

	if !isValidEmail(c.Email) {
		return errors.New("email inválido")
	}

	if c.Phone == "" {
		return errors.New("telefone é obrigatório")
	}

	if c.Address.Street == "" || c.Address.City == "" || c.Address.State == "" || c.Address.ZipCode == "" {
		return errors.New("endereço incompleto: rua, cidade, estado e CEP são obrigatórios")
	}

	return nil
}

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}
