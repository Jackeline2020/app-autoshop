package repository

import (
	"autoshop/internal/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VehicleRepository interface {
	Create(vehicle domain.Vehicle) (domain.Vehicle, error)
	FindAll() ([]domain.Vehicle, error)
	FindByID(id string) (domain.Vehicle, error)
	FindByCustomerID(customerID string) ([]domain.Vehicle, error)
	Update(vehicle domain.Vehicle) error
	Delete(id string) error
}

type VehiclePostgresRepository struct {
	db *pgxpool.Pool
}

func NewVehiclePostgresRepository(db *pgxpool.Pool) *VehiclePostgresRepository {
	return &VehiclePostgresRepository{db: db}
}

func (r *VehiclePostgresRepository) Create(vehicle domain.Vehicle) (domain.Vehicle, error) {
	_, err := r.db.Exec(context.Background(), `
		INSERT INTO vehicles (id, customer_id, plate, brand, model, year)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, vehicle.ID, vehicle.CustomerID, vehicle.Plate, vehicle.Brand, vehicle.Model, vehicle.Year)
	if err != nil {
		return vehicle, fmt.Errorf("erro ao criar veículo: %w", err)
	}
	return vehicle, nil
}

func (r *VehiclePostgresRepository) FindAll() ([]domain.Vehicle, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, customer_id, plate, brand, model, year FROM vehicles ORDER BY plate
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVehicles(rows)
}

func (r *VehiclePostgresRepository) FindByID(id string) (domain.Vehicle, error) {
	row := r.db.QueryRow(context.Background(), `
		SELECT id, customer_id, plate, brand, model, year FROM vehicles WHERE id = $1
	`, id)

	v, err := scanVehicle(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Vehicle{}, fmt.Errorf("veículo não encontrado")
		}
		return domain.Vehicle{}, err
	}
	return v, nil
}

func (r *VehiclePostgresRepository) FindByCustomerID(customerID string) ([]domain.Vehicle, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, customer_id, plate, brand, model, year FROM vehicles WHERE customer_id = $1 ORDER BY plate
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVehicles(rows)
}

func (r *VehiclePostgresRepository) Update(vehicle domain.Vehicle) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE vehicles SET plate = $1, brand = $2, model = $3, year = $4 WHERE id = $5
	`, vehicle.Plate, vehicle.Brand, vehicle.Model, vehicle.Year, vehicle.ID)
	return err
}

func (r *VehiclePostgresRepository) Delete(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM vehicles WHERE id = $1`, id)
	return err
}

func scanVehicle(r row) (domain.Vehicle, error) {
	var v domain.Vehicle
	err := r.Scan(&v.ID, &v.CustomerID, &v.Plate, &v.Brand, &v.Model, &v.Year)
	return v, err
}

func scanVehicles(rows pgx.Rows) ([]domain.Vehicle, error) {
	vehicles := make([]domain.Vehicle, 0)
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, rows.Err()
}
