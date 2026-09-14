package repository

import (
	"autoshop/internal/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CustomerRepository interface {
	Create(customer domain.Customer) (domain.Customer, error)
	FindAll() ([]domain.Customer, error)
	FindByID(id string) (domain.Customer, error)
	Update(customer domain.Customer) error
	UpdateStatus(id, status string) error
	Delete(id string) error
}

type CustomerPostgresRepository struct {
	db *pgxpool.Pool
}

func NewCustomerPostgresRepository(db *pgxpool.Pool) *CustomerPostgresRepository {
	return &CustomerPostgresRepository{db: db}
}

func (r *CustomerPostgresRepository) Create(customer domain.Customer) (domain.Customer, error) {
	_, err := r.db.Exec(context.Background(), `
		INSERT INTO customers
			(id, name, cpf, cnpj, email, phone,
			 address_street, address_number, address_complement, address_city, address_state, address_zip_code, status)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, NULLIF($9, ''), $10, $11, $12, $13)
	`,
		customer.ID, customer.Name, customer.CPF, customer.CNPJ, customer.Email, customer.Phone,
		customer.Address.Street, customer.Address.Number, customer.Address.Complement,
		customer.Address.City, customer.Address.State, customer.Address.ZipCode, customer.Status,
	)
	if err != nil {
		return customer, fmt.Errorf("erro ao criar cliente: %w", err)
	}

	return customer, nil
}

func (r *CustomerPostgresRepository) FindAll() ([]domain.Customer, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, name, COALESCE(cpf, ''), COALESCE(cnpj, ''), email, phone,
		       address_street, address_number, COALESCE(address_complement, ''), address_city, address_state, address_zip_code, status
		FROM customers
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanCustomers(rows)
}

func (r *CustomerPostgresRepository) FindByID(id string) (domain.Customer, error) {
	row := r.db.QueryRow(context.Background(), `
		SELECT id, name, COALESCE(cpf, ''), COALESCE(cnpj, ''), email, phone,
		       address_street, address_number, COALESCE(address_complement, ''), address_city, address_state, address_zip_code, status
		FROM customers
		WHERE id = $1
	`, id)

	customer, err := scanCustomer(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Customer{}, fmt.Errorf("cliente não encontrado")
		}
		return domain.Customer{}, err
	}

	return customer, nil
}

func (r *CustomerPostgresRepository) Update(customer domain.Customer) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE customers
		SET name = $1, cpf = NULLIF($2, ''), cnpj = NULLIF($3, ''), email = $4, phone = $5,
		    address_street = $6, address_number = $7, address_complement = NULLIF($8, ''),
		    address_city = $9, address_state = $10, address_zip_code = $11, updated_at = now()
		WHERE id = $12
	`,
		customer.Name, customer.CPF, customer.CNPJ, customer.Email, customer.Phone,
		customer.Address.Street, customer.Address.Number, customer.Address.Complement,
		customer.Address.City, customer.Address.State, customer.Address.ZipCode,
		customer.ID,
	)
	return err
}

// UpdateStatus ativa/inativa um cliente — usada pela oficina (não altera os
// demais dados) e consultada pelo fluxo de autenticação por CPF na Lambda
// (um cliente inativo não recebe token, ver lambda-auth-autoshop/internal/authflow).
func (r *CustomerPostgresRepository) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(context.Background(), `
		UPDATE customers SET status = $1, updated_at = now() WHERE id = $2
	`, status, id)
	return err
}

func (r *CustomerPostgresRepository) Delete(id string) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, id)
	return err
}

// row abstrai pgx.Row e pgx.Rows (ambos têm Scan) para reaproveitar o parse.
type row interface {
	Scan(dest ...any) error
}

func scanCustomer(r row) (domain.Customer, error) {
	var c domain.Customer
	err := r.Scan(
		&c.ID, &c.Name, &c.CPF, &c.CNPJ, &c.Email, &c.Phone,
		&c.Address.Street, &c.Address.Number, &c.Address.Complement,
		&c.Address.City, &c.Address.State, &c.Address.ZipCode, &c.Status,
	)
	return c, err
}

func scanCustomers(rows pgx.Rows) ([]domain.Customer, error) {
	customers := make([]domain.Customer, 0)
	for rows.Next() {
		c, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}
