package repository

import (
	"autoshop/internal/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PartRepository interface {
	Create(part domain.Part) (domain.Part, error)
	FindAll() ([]domain.Part, error)
	FindByID(id string) (domain.Part, error)
	FindLowStock() ([]domain.Part, error)
	Update(part domain.Part) error
	UpdateStock(id string, quantity int) error
	Delete(id string) error
}

type PartPostgresRepository struct {
	db *pgxpool.Pool
}

func NewPartPostgresRepository(db *pgxpool.Pool) *PartPostgresRepository {
	return &PartPostgresRepository{db: db}
}

func (r *PartPostgresRepository) Create(part domain.Part) (domain.Part, error) {
	createdAt, err := parseTime(part.CreatedAt)
	if err != nil {
		return part, fmt.Errorf("created_at inválido: %w", err)
	}

	_, err = r.db.Exec(context.Background(), `
		INSERT INTO parts (id, name, description, price, stock, min_stock, unit, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, part.ID, part.Name, part.Description, part.Price, part.Stock, part.MinStock, part.Unit, createdAt)
	if err != nil {
		return part, fmt.Errorf("erro ao criar peça: %w", err)
	}
	return part, nil
}

func (r *PartPostgresRepository) FindAll() ([]domain.Part, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, name, description, price, stock, min_stock, unit, created_at FROM parts ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParts(rows)
}

func (r *PartPostgresRepository) FindByID(id string) (domain.Part, error) {
	row := r.db.QueryRow(context.Background(), `
		SELECT id, name, description, price, stock, min_stock, unit, created_at FROM parts WHERE id = $1
	`, id)

	p, err := scanPart(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Part{}, fmt.Errorf("peça não encontrada")
		}
		return domain.Part{}, err
	}
	return p, nil
}

func (r *PartPostgresRepository) FindLowStock() ([]domain.Part, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, name, description, price, stock, min_stock, unit, created_at
		FROM parts WHERE stock <= min_stock ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParts(rows)
}

func (r *PartPostgresRepository) Update(part domain.Part) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE parts
		SET name = $1, description = $2, price = $3, min_stock = $4, unit = $5, updated_at = now()
		WHERE id = $6
	`, part.Name, part.Description, part.Price, part.MinStock, part.Unit, part.ID)
	return err
}

func (r *PartPostgresRepository) UpdateStock(id string, quantity int) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE parts SET stock = $1, updated_at = now() WHERE id = $2
	`, quantity, id)
	return err
}

func (r *PartPostgresRepository) Delete(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM parts WHERE id = $1`, id)
	return err
}

func scanPart(r rowScanner) (domain.Part, error) {
	var p domain.Part
	var createdAt timeScanner
	err := r.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.MinStock, &p.Unit, &createdAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt = createdAt.String()
	return p, nil
}

func scanParts(rows pgx.Rows) ([]domain.Part, error) {
	parts := make([]domain.Part, 0)
	for rows.Next() {
		p, err := scanPart(rows)
		if err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}
