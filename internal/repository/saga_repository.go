package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"autoshop/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrSagaConcurrentUpdate indica que a Saga mudou entre a leitura e a
	// gravação (ex.: um evento e o prazo processados ao mesmo tempo).
	ErrSagaConcurrentUpdate = errors.New("a saga foi alterada por outra operação; tente novamente")
	// ErrMessageAlreadyProcessed indica que o evento já foi aplicado antes.
	ErrMessageAlreadyProcessed = errors.New("mensagem já processada")
)

const uniqueViolation = "23505"

type SagaRepository interface {
	CreateOrderWithSaga(order domain.Order, saga domain.Saga, outbox []domain.OutboxMessage) (domain.Order, error)
	FindByOrderID(orderID string) (domain.Saga, error)
	Save(change domain.SagaChange) error
	FindExpired(now time.Time) ([]domain.Saga, error)
	IsProcessed(messageID string) (bool, error)
	PendingOutbox(limit int) ([]domain.OutboxMessage, error)
	MarkPublished(id string) error
}

type SagaPostgresRepository struct {
	db *pgxpool.Pool
}

func NewSagaPostgresRepository(db *pgxpool.Pool) *SagaPostgresRepository {
	return &SagaPostgresRepository{db: db}
}

// CreateOrderWithSaga grava a OS, a Saga e o primeiro comando na mesma
// transação: não existe OS sem Saga, nem Saga sem o comando que a inicia.
func (r *SagaPostgresRepository) CreateOrderWithSaga(order domain.Order, saga domain.Saga, outbox []domain.OutboxMessage) (domain.Order, error) {
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return order, err
	}
	defer tx.Rollback(ctx)

	if err := insertOrder(ctx, tx, order); err != nil {
		return order, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO saga_instances (id, order_id, status, current_step, completed_steps, pending_compensations,
		                            deadline_at, correlation_id, failure_reason, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12)
	`, saga.ID, saga.OrderID, saga.Status, saga.CurrentStep, stepsToText(saga.CompletedSteps), stepsToText(saga.PendingCompensations),
		saga.DeadlineAt, saga.CorrelationID, saga.FailureReason, saga.Version, saga.CreatedAt, saga.UpdatedAt); err != nil {
		return order, fmt.Errorf("erro ao criar saga: %w", err)
	}
	if err := insertOutbox(ctx, tx, outbox); err != nil {
		return order, err
	}
	return order, tx.Commit(ctx)
}

func (r *SagaPostgresRepository) FindByOrderID(orderID string) (domain.Saga, error) {
	row := r.db.QueryRow(context.Background(), `SELECT `+sagaColumns+` FROM saga_instances WHERE order_id = $1`, orderID)
	s, err := scanSaga(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Saga{}, domain.ErrSagaNotFound
	}
	return s, err
}

// Save grava uma decisão da Saga de forma atômica: a OS (se mudou), a
// Saga, os comandos na outbox e o ID do evento que originou a decisão. Se
// o evento já tiver sido processado, ou se a Saga tiver mudado desde a
// leitura, nada é gravado.
func (r *SagaPostgresRepository) Save(change domain.SagaChange) error {
	ctx := context.Background()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if change.ProcessedMessageID != "" {
		_, err := tx.Exec(ctx, `INSERT INTO processed_messages (message_id) VALUES ($1)`, change.ProcessedMessageID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return ErrMessageAlreadyProcessed
		}
		if err != nil {
			return err
		}
	}

	if change.Order != nil {
		if err := updateOrderProgress(ctx, tx, *change.Order); err != nil {
			return err
		}
	}

	s := change.Saga
	tag, err := tx.Exec(ctx, `
		UPDATE saga_instances
		SET status = $1, current_step = $2, completed_steps = $3, pending_compensations = $4,
		    deadline_at = $5, failure_reason = NULLIF($6, ''), updated_at = $7, version = version + 1
		WHERE id = $8 AND version = $9
	`, s.Status, s.CurrentStep, stepsToText(s.CompletedSteps), stepsToText(s.PendingCompensations),
		s.DeadlineAt, s.FailureReason, s.UpdatedAt, s.ID, s.Version)
	if err != nil {
		return fmt.Errorf("erro ao atualizar saga: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSagaConcurrentUpdate
	}

	if err := insertOutbox(ctx, tx, change.Outbox); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindExpired devolve as Sagas em andamento cujo passo atual passou do prazo.
func (r *SagaPostgresRepository) FindExpired(now time.Time) ([]domain.Saga, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT `+sagaColumns+` FROM saga_instances
		WHERE status = $1 AND deadline_at IS NOT NULL AND deadline_at < $2
		ORDER BY deadline_at
	`, domain.SagaRunning, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Saga, 0)
	for rows.Next() {
		s, err := scanSaga(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (r *SagaPostgresRepository) IsProcessed(messageID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM processed_messages WHERE message_id = $1)`, messageID).Scan(&exists)
	return exists, err
}

// PendingOutbox devolve os comandos ainda não publicados, dos mais antigos
// para os mais novos.
func (r *SagaPostgresRepository) PendingOutbox(limit int) ([]domain.OutboxMessage, error) {
	rows, err := r.db.Query(context.Background(), `
		SELECT id, queue, message_type, body::text, created_at
		FROM outbox WHERE published_at IS NULL
		ORDER BY created_at LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.OutboxMessage, 0)
	for rows.Next() {
		var m domain.OutboxMessage
		var body string
		if err := rows.Scan(&m.ID, &m.Queue, &m.Type, &body, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.Body = []byte(body)
		result = append(result, m)
	}
	return result, rows.Err()
}

func (r *SagaPostgresRepository) MarkPublished(id string) error {
	_, err := r.db.Exec(context.Background(), `UPDATE outbox SET published_at = now() WHERE id = $1`, id)
	return err
}

const sagaColumns = `id, order_id, status, current_step, completed_steps, pending_compensations,
	deadline_at, COALESCE(correlation_id, ''), COALESCE(failure_reason, ''), version, created_at, updated_at`

func scanSaga(row pgx.Row) (domain.Saga, error) {
	var s domain.Saga
	var completed, pending []string
	err := row.Scan(&s.ID, &s.OrderID, &s.Status, &s.CurrentStep, &completed, &pending,
		&s.DeadlineAt, &s.CorrelationID, &s.FailureReason, &s.Version, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return domain.Saga{}, err
	}
	s.CompletedSteps = textToSteps(completed)
	s.PendingCompensations = textToSteps(pending)
	return s, nil
}

func insertOutbox(ctx context.Context, tx pgx.Tx, msgs []domain.OutboxMessage) error {
	for _, m := range msgs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox (id, queue, message_type, body, created_at) VALUES ($1, $2, $3, $4::jsonb, $5)
		`, m.ID, m.Queue, m.Type, string(m.Body), m.CreatedAt); err != nil {
			return fmt.Errorf("erro ao gravar comando na outbox: %w", err)
		}
	}
	return nil
}

func stepsToText(steps []domain.SagaStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = string(s)
	}
	return out
}

func textToSteps(values []string) []domain.SagaStep {
	out := make([]domain.SagaStep, len(values))
	for i, v := range values {
		out[i] = domain.SagaStep(v)
	}
	return out
}
