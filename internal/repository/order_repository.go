package repository

import (
	"autoshop/internal/domain"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderRepository interface {
	Create(order domain.Order) (domain.Order, error)
	FindAll() ([]domain.Order, error)
	FindByID(id string) (domain.Order, error)
	FindByCustomerID(customerID string) ([]domain.Order, error)
	FindByStatus(status domain.OrderStatus) ([]domain.Order, error)
	Update(order domain.Order) error
	Delete(id string) error
}

type OrderPostgresRepository struct {
	db *pgxpool.Pool
}

func NewOrderPostgresRepository(db *pgxpool.Pool) *OrderPostgresRepository {
	return &OrderPostgresRepository{db: db}
}

// Create grava a OS e os itens (serviços/peças) em uma única transação:
// se qualquer item falhar, nada é persistido — não existe OS "pela metade".
func (r *OrderPostgresRepository) Create(order domain.Order) (domain.Order, error) {
	ctx := context.Background()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return order, err
	}
	defer tx.Rollback(ctx) // no-op se o commit já tiver acontecido

	createdAt, err := parseTime(order.CreatedAt)
	if err != nil {
		return order, fmt.Errorf("created_at inválido: %w", err)
	}
	updatedAt, err := parseTime(order.UpdatedAt)
	if err != nil {
		return order, fmt.Errorf("updated_at inválido: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO orders
			(id, customer_id, customer_name, vehicle_id, vehicle_plate, status,
			 total_services, total_parts, total, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		order.ID, order.CustomerID, order.CustomerName, order.VehicleID, order.VehiclePlate, order.Status,
		order.TotalServices, order.TotalParts, order.Total, order.Notes, createdAt, updatedAt,
	)
	if err != nil {
		return order, fmt.Errorf("erro ao criar ordem de serviço: %w", err)
	}

	if err := insertOrderServices(ctx, tx, order.ID, order.Services); err != nil {
		return order, err
	}
	if err := insertOrderParts(ctx, tx, order.ID, order.Parts); err != nil {
		return order, err
	}

	if err := tx.Commit(ctx); err != nil {
		return order, err
	}

	return order, nil
}

func (r *OrderPostgresRepository) FindAll() ([]domain.Order, error) {
	return r.findByFilter("")
}

func (r *OrderPostgresRepository) FindByCustomerID(customerID string) ([]domain.Order, error) {
	return r.findByFilter("WHERE customer_id = $1", customerID)
}

func (r *OrderPostgresRepository) FindByStatus(status domain.OrderStatus) ([]domain.Order, error) {
	return r.findByFilter("WHERE status = $1", string(status))
}

func (r *OrderPostgresRepository) findByFilter(whereClause string, args ...any) ([]domain.Order, error) {
	ctx := context.Background()

	query := `
		SELECT id, customer_id, customer_name, vehicle_id, vehicle_plate, status,
		       total_services, total_parts, total, COALESCE(notes, ''),
		       created_at, updated_at, started_at, finished_at, delivered_at
		FROM orders ` + whereClause + ` ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	orders := make([]domain.Order, 0)
	ids := make([]string, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		orders = append(orders, o)
		ids = append(ids, o.ID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	servicesByOrder, err := findOrderServicesByOrderIDs(ctx, r.db, ids)
	if err != nil {
		return nil, err
	}
	partsByOrder, err := findOrderPartsByOrderIDs(ctx, r.db, ids)
	if err != nil {
		return nil, err
	}

	for i := range orders {
		orders[i].Services = servicesByOrder[orders[i].ID]
		orders[i].Parts = partsByOrder[orders[i].ID]
	}

	return orders, nil
}

func (r *OrderPostgresRepository) FindByID(id string) (domain.Order, error) {
	ctx := context.Background()

	row := r.db.QueryRow(ctx, `
		SELECT id, customer_id, customer_name, vehicle_id, vehicle_plate, status,
		       total_services, total_parts, total, COALESCE(notes, ''),
		       created_at, updated_at, started_at, finished_at, delivered_at
		FROM orders WHERE id = $1
	`, id)

	order, err := scanOrder(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.Order{}, fmt.Errorf("ordem de serviço não encontrada")
		}
		return domain.Order{}, err
	}

	services, err := findOrderServicesByOrderIDs(ctx, r.db, []string{id})
	if err != nil {
		return domain.Order{}, err
	}
	parts, err := findOrderPartsByOrderIDs(ctx, r.db, []string{id})
	if err != nil {
		return domain.Order{}, err
	}

	order.Services = services[id]
	order.Parts = parts[id]

	return order, nil
}

// Update reescreve o cabeçalho da OS e os itens associados (delete + insert
// dentro da mesma transação) — mais simples e seguro que tentar calcular um
// diff de itens, e o volume por OS é pequeno.
func (r *OrderPostgresRepository) Update(order domain.Order) error {
	ctx := context.Background()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	startedAt := parseOptionalTime(order.StartedAt)
	finishedAt := parseOptionalTime(order.FinishedAt)
	deliveredAt := parseOptionalTime(order.DeliveredAt)

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET status = $1, total_services = $2, total_parts = $3, total = $4, notes = $5,
		    updated_at = now(), started_at = $6, finished_at = $7, delivered_at = $8
		WHERE id = $9
	`, order.Status, order.TotalServices, order.TotalParts, order.Total, order.Notes,
		startedAt, finishedAt, deliveredAt, order.ID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar ordem de serviço: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM order_services WHERE order_id = $1`, order.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM order_parts WHERE order_id = $1`, order.ID); err != nil {
		return err
	}
	if err := insertOrderServices(ctx, tx, order.ID, order.Services); err != nil {
		return err
	}
	if err := insertOrderParts(ctx, tx, order.ID, order.Parts); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *OrderPostgresRepository) Delete(id string) error {
	// order_services/order_parts têm ON DELETE CASCADE — não precisam de delete explícito.
	_, err := r.db.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, id)
	return err
}

// --- helpers de itens (order_services / order_parts) ---

func insertOrderServices(ctx context.Context, tx pgx.Tx, orderID string, services []domain.OrderService) error {
	for _, s := range services {
		executedAt := parseOptionalTime(s.ExecutedAt)
		_, err := tx.Exec(ctx, `
			INSERT INTO order_services (order_id, service_id, service_name, price, executed_at)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, s.ServiceID, s.ServiceName, s.Price, executedAt)
		if err != nil {
			return fmt.Errorf("erro ao gravar serviço da OS: %w", err)
		}
	}
	return nil
}

func insertOrderParts(ctx context.Context, tx pgx.Tx, orderID string, parts []domain.OrderPart) error {
	for _, p := range parts {
		_, err := tx.Exec(ctx, `
			INSERT INTO order_parts (order_id, part_id, part_name, quantity, unit_price)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, p.PartID, p.PartName, p.Quantity, p.UnitPrice)
		if err != nil {
			return fmt.Errorf("erro ao gravar peça da OS: %w", err)
		}
	}
	return nil
}

func findOrderServicesByOrderIDs(ctx context.Context, db *pgxpool.Pool, orderIDs []string) (map[string][]domain.OrderService, error) {
	result := make(map[string][]domain.OrderService)
	if len(orderIDs) == 0 {
		return result, nil
	}

	rows, err := db.Query(ctx, `
		SELECT order_id, service_id, service_name, price, executed_at
		FROM order_services WHERE order_id = ANY($1)
	`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orderID string
		var s domain.OrderService
		var executedAt timeScanner
		if err := rows.Scan(&orderID, &s.ServiceID, &s.ServiceName, &s.Price, &executedAt); err != nil {
			return nil, err
		}
		s.ExecutedAt = executedAt.String()
		result[orderID] = append(result[orderID], s)
	}
	return result, rows.Err()
}

func findOrderPartsByOrderIDs(ctx context.Context, db *pgxpool.Pool, orderIDs []string) (map[string][]domain.OrderPart, error) {
	result := make(map[string][]domain.OrderPart)
	if len(orderIDs) == 0 {
		return result, nil
	}

	rows, err := db.Query(ctx, `
		SELECT order_id, part_id, part_name, quantity, unit_price
		FROM order_parts WHERE order_id = ANY($1)
	`, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orderID string
		var p domain.OrderPart
		if err := rows.Scan(&orderID, &p.PartID, &p.PartName, &p.Quantity, &p.UnitPrice); err != nil {
			return nil, err
		}
		result[orderID] = append(result[orderID], p)
	}
	return result, rows.Err()
}

func scanOrder(r row) (domain.Order, error) {
	var o domain.Order
	var createdAt, updatedAt timeScanner
	var startedAt, finishedAt, deliveredAt timeScanner

	err := r.Scan(
		&o.ID, &o.CustomerID, &o.CustomerName, &o.VehicleID, &o.VehiclePlate, &o.Status,
		&o.TotalServices, &o.TotalParts, &o.Total, &o.Notes,
		&createdAt, &updatedAt, &startedAt, &finishedAt, &deliveredAt,
	)
	if err != nil {
		return o, err
	}

	o.CreatedAt = createdAt.String()
	o.UpdatedAt = updatedAt.String()
	o.StartedAt = startedAt.String()
	o.FinishedAt = finishedAt.String()
	o.DeliveredAt = deliveredAt.String()

	return o, nil
}
