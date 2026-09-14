package usecase

import (
	"autoshop/internal/domain"
	"autoshop/internal/repository"
	"errors"

	"github.com/google/uuid"
)

type CustomerUseCase struct {
	repo repository.CustomerRepository
}

func NewCustomerUseCase(r repository.CustomerRepository) *CustomerUseCase {
	return &CustomerUseCase{repo: r}
}

func (u *CustomerUseCase) Create(name, cpf, cnpj, email, phone string, address domain.Address) (domain.Customer, error) {
	customer := domain.Customer{
		ID:      uuid.New().String(),
		Name:    name,
		CPF:     cpf,
		CNPJ:    cnpj,
		Email:   email,
		Phone:   phone,
		Address: address,
		Status:  domain.CustomerStatusActive,
	}

	if err := customer.Validate(); err != nil {
		return domain.Customer{}, err
	}

	return u.repo.Create(customer)
}

func (u *CustomerUseCase) GetAll() ([]domain.Customer, error) {
	return u.repo.FindAll()
}

func (u *CustomerUseCase) GetByID(id string) (domain.Customer, error) {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return domain.Customer{}, err
	}

	return u.repo.FindByID(id)
}

func (u *CustomerUseCase) Update(id, name, cpf, cnpj, email, phone string, address domain.Address) (*domain.Customer, error) {
	customer, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	customer.Name = name
	customer.CPF = cpf
	customer.CNPJ = cnpj
	customer.Email = email
	customer.Phone = phone
	customer.Address = address

	if err := customer.Validate(); err != nil {
		return nil, err
	}

	err = u.repo.Update(customer)
	if err != nil {
		return nil, err
	}

	return &customer, nil
}

// UpdateStatus ativa/inativa um cliente. Um cliente "inativo" continua
// existindo na base (histórico de OS preservado), mas deixa de conseguir
// autenticar via CPF — ver lambda-auth-autoshop/internal/authflow.
func (u *CustomerUseCase) UpdateStatus(id, status string) (domain.Customer, error) {
	if !domain.IsValidCustomerStatus(status) {
		return domain.Customer{}, errors.New("status inválido: use \"ativo\" ou \"inativo\"")
	}

	customer, err := u.repo.FindByID(id)
	if err != nil {
		return domain.Customer{}, err
	}

	if err := u.repo.UpdateStatus(id, status); err != nil {
		return domain.Customer{}, err
	}

	customer.Status = status
	return customer, nil
}

func (u *CustomerUseCase) Delete(id string) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}

	return u.repo.Delete(id)
}
