// Package worker contém as rotinas da Saga que rodam em segundo plano.
package worker

import (
	"context"
	"log/slog"
	"time"

	"autoshop/internal/domain"
)

// OutboxStore lê e marca os comandos gravados na outbox.
type OutboxStore interface {
	PendingOutbox(limit int) ([]domain.OutboxMessage, error)
	MarkPublished(id string) error
}

// QueueSender publica um comando na fila de destino.
type QueueSender interface {
	Send(ctx context.Context, queue string, body []byte) error
}

// OutboxRelay publica na fila os comandos gravados junto com as decisões da
// Saga. Se a publicação falhar, o comando continua pendente e é tentado na
// próxima rodada; se falhar só a marcação, o comando pode ser publicado de
// novo, e o serviço de destino ignora a repetição pelo message_id.
type OutboxRelay struct {
	store     OutboxStore
	sender    QueueSender
	batchSize int
}

func NewOutboxRelay(store OutboxStore, sender QueueSender) *OutboxRelay {
	return &OutboxRelay{store: store, sender: sender, batchSize: 50}
}

// PublishPending publica os comandos pendentes e devolve quantos foram
// publicados.
func (r *OutboxRelay) PublishPending(ctx context.Context) int {
	pending, err := r.store.PendingOutbox(r.batchSize)
	if err != nil {
		slog.Error("erro ao ler a outbox", "error", err.Error())
		return 0
	}
	published := 0
	for _, m := range pending {
		if err := r.sender.Send(ctx, m.Queue, m.Body); err != nil {
			slog.Warn("erro ao publicar comando; nova tentativa na próxima rodada",
				"type", m.Type, "queue", m.Queue, "message_id", m.ID, "error", err.Error())
			continue
		}
		if err := r.store.MarkPublished(m.ID); err != nil {
			slog.Warn("comando publicado, mas não marcado", "message_id", m.ID, "error", err.Error())
			continue
		}
		published++
	}
	return published
}

// TimeoutChecker cancela as Sagas com prazo vencido.
type TimeoutChecker interface {
	CheckTimeouts(now time.Time) (int, error)
}

// CheckTimeouts executa uma verificação de prazos.
func CheckTimeouts(t TimeoutChecker, now time.Time) {
	n, err := t.CheckTimeouts(now)
	if err != nil {
		slog.Error("erro ao verificar prazos da saga", "error", err.Error())
		return
	}
	if n > 0 {
		slog.Info("sagas canceladas por prazo", "total", n)
	}
}

// Every executa fn a cada intervalo, até o contexto ser cancelado.
func Every(ctx context.Context, interval time.Duration, fn func(now time.Time)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			fn(now)
		}
	}
}
