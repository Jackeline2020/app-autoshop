package dto

type AddressRequest struct {
	Street     string `json:"street"       binding:"required"`
	Number     string `json:"number"       binding:"required"`
	Complement string `json:"complement"`
	City       string `json:"city"         binding:"required"`
	State      string `json:"state"        binding:"required"`
	ZipCode    string `json:"zip_code"     binding:"required"`
}

type CreateCustomerRequest struct {
	Name    string         `json:"name"     binding:"required"`
	CPF     string         `json:"cpf"`
	CNPJ    string         `json:"cnpj"`
	Email   string         `json:"email"    binding:"required"`
	Phone   string         `json:"phone"    binding:"required"`
	Address AddressRequest `json:"address"  binding:"required"`
}

type UpdateCustomerRequest struct {
	Name    string         `json:"name"     binding:"required"`
	CPF     string         `json:"cpf"`
	CNPJ    string         `json:"cnpj"`
	Email   string         `json:"email"    binding:"required"`
	Phone   string         `json:"phone"    binding:"required"`
	Address AddressRequest `json:"address"  binding:"required"`
}

// UpdateCustomerStatusRequest ativa/inativa um cliente — consultado pelo
// fluxo de autenticação por CPF (lambda-auth-autoshop).
type UpdateCustomerStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=ativo inativo"`
}
