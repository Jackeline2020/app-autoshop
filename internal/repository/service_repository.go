package repository

import (
	"autoshop/internal/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceRepository interface {
	Create(service domain.Service) (domain.Service, error)
	FindAll() ([]domain.Service, error)
	FindByID(id string) (domain.Service, error)
	Update(service domain.Service) error
	Delete(id string) error
}

type ServicePostgresRepository struct {
	db *pgxpool.Pool
}

func NewServicePostgresRepository(db *pgxpool.Pool) *ServicePostgresRepository {
	return &ServicePostgresRepository{db: db}
}

func (r *ServicePostgresRepository) Create(service domain.Service) (domain.Service, error) {
	createdAt, err := parseTime(service.CreatedAt)
	if err != nil {
		return service, fmt.Errorf("created_at inválido: %w", err)
	}

	_, err = r.db.Exec(context.Background(), `
		INSERT INTO services (id, name, description, price, estimated_time_minutes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, service.ID, service.Name, service.Description, service.Price, service.EstimatedTime, createdAt)
	if err != nil {
		return service, fmt.Errorf("erro ao criar serviço: %w", err)
	}
	return service, nil
}

func (r *ServicePostgresRepository) FindAll() ([]domain.Service, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, name, description, price, estimated_time_minutes, created_at FROM services ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanServices(rows)
}

func (r *ServicePostgresRepository) FindByID(id string) (domain.Service, error) {
	row := r.db.QueryRow(context.Background(), `
		SELECT id, name, description, price, estimated_time_minutes, created_at FROM services WHERE id = $1
	`, id)

	s, err := scanService(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Service{}, fmt.Errorf("serviço não encontrado")
		}
		return domain.Service{}, err
	}
	return s, nil
}

func (r *ServicePostgresRepository) Update(service domain.Service) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE services SET name = $1, description = $2, price = $3, estimated_time_minutes = $4 WHERE id = $5
	`, service.Name, service.Description, service.Price, service.EstimatedTime, service.ID)
	return err
}

func (r *ServicePostgresRepository) Delete(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM services WHERE id = $1`, id)
	return err
}

func scanService(r row) (domain.Service, error) {
	var s domain.Service
	var createdAt timeScanner
	err := r.Scan(&s.ID, &s.Name, &s.Description, &s.Price, &s.EstimatedTime, &createdAt)
	if err != nil {
		return s, err
	}
	s.CreatedAt = createdAt.String()
	return s, nil
}

func scanServices(rows pgx.Rows) ([]domain.Service, error) {
	services := make([]domain.Service, 0)
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}
