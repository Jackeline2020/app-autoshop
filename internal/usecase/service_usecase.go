package usecase

import (
	"autoshop/internal/domain"
	"autoshop/internal/repository"

	"github.com/google/uuid"
)

type ServiceUseCase struct {
	repo repository.ServiceRepository
}

func NewServiceUseCase(r repository.ServiceRepository) *ServiceUseCase {
	return &ServiceUseCase{repo: r}
}

func (u *ServiceUseCase) Create(name, description string, price float64, estimatedTime int) (domain.Service, error) {
	service := domain.NewService(name, description, price, estimatedTime)
	service.ID = uuid.New().String()

	if err := service.Validate(); err != nil {
		return domain.Service{}, err
	}

	return u.repo.Create(service)
}

func (u *ServiceUseCase) GetAll() ([]domain.Service, error) {
	return u.repo.FindAll()
}

func (u *ServiceUseCase) GetByID(id string) (domain.Service, error) {
	return u.repo.FindByID(id)
}

func (u *ServiceUseCase) Update(id, name, description string, price float64, estimatedTime int) (*domain.Service, error) {
	service, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	service.Name = name
	service.Description = description
	service.Price = price
	service.EstimatedTime = estimatedTime

	if err := service.Validate(); err != nil {
		return nil, err
	}

	if err := u.repo.Update(service); err != nil {
		return nil, err
	}

	return &service, nil
}

func (u *ServiceUseCase) Delete(id string) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}
	return u.repo.Delete(id)
}
