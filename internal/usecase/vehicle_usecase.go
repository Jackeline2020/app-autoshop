package usecase

import (
	"autoshop/internal/domain"
	"autoshop/internal/repository"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type VehicleUseCase struct {
	repo         repository.VehicleRepository
	customerRepo repository.CustomerRepository
}

func NewVehicleUseCase(r repository.VehicleRepository, cr repository.CustomerRepository) *VehicleUseCase {
	return &VehicleUseCase{repo: r, customerRepo: cr}
}

func (u *VehicleUseCase) Create(customerID, plate, brand, model string, year int) (domain.Vehicle, error) {
	// Verifica se o cliente existe
	_, err := u.customerRepo.FindByID(customerID)
	if err != nil {
		return domain.Vehicle{}, fmt.Errorf("cliente não encontrado")
	}

	vehicle := domain.Vehicle{
		ID:         uuid.New().String(),
		CustomerID: customerID,
		Plate:      strings.ToUpper(strings.ReplaceAll(plate, "-", "")),
		Brand:      brand,
		Model:      model,
		Year:       year,
	}

	if err := vehicle.Validate(); err != nil {
		return domain.Vehicle{}, err
	}

	return u.repo.Create(vehicle)
}

func (u *VehicleUseCase) GetAll() ([]domain.Vehicle, error) {
	return u.repo.FindAll()
}

func (u *VehicleUseCase) GetByID(id string) (domain.Vehicle, error) {
	return u.repo.FindByID(id)
}

func (u *VehicleUseCase) GetByCustomerID(customerID string) ([]domain.Vehicle, error) {
	return u.repo.FindByCustomerID(customerID)
}

func (u *VehicleUseCase) Update(id, plate, brand, model string, year int) (*domain.Vehicle, error) {
	vehicle, err := u.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	vehicle.Plate = strings.ToUpper(strings.ReplaceAll(plate, "-", ""))
	vehicle.Brand = brand
	vehicle.Model = model
	vehicle.Year = year

	if err := vehicle.Validate(); err != nil {
		return nil, err
	}

	err = u.repo.Update(vehicle)
	if err != nil {
		return nil, err
	}

	return &vehicle, nil
}

func (u *VehicleUseCase) Delete(id string) error {
	_, err := u.repo.FindByID(id)
	if err != nil {
		return err
	}
	return u.repo.Delete(id)
}
