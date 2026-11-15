// Package consumer lê a fila os-events, onde o Execution e o Billing
// publicam os resultados dos comandos da Saga.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/usecase"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// EventHandler aplica um evento à Saga (SagaOrchestrator).
type EventHandler interface {
	HandleEvent(ctx context.Context, env messaging.Envelope) error
}

// ProcessedChecker informa se um evento já foi aplicado.
type ProcessedChecker interface {
	IsProcessed(messageID string) (bool, error)
}

// SQSConsumer lê os eventos continuamente. A mensagem só é apagada depois de
// processada; se falhar, volta para a fila após o tempo de visibilidade e,
// depois de 5 tentativas, vai para a DLQ.
type SQSConsumer struct {
	client      messaging.SQSAPI
	queueURL    string
	handler     EventHandler
	processed   ProcessedChecker
	waitSeconds int32
	retryDelay  time.Duration
}

func NewSQSConsumer(client messaging.SQSAPI, queueURL string, h EventHandler, p ProcessedChecker) *SQSConsumer {
	return &SQSConsumer{client: client, queueURL: queueURL, handler: h, processed: p, waitSeconds: 20, retryDelay: 2 * time.Second}
}

// Run lê a fila até o contexto ser cancelado (encerramento do serviço).
func (c *SQSConsumer) Run(ctx context.Context) {
	slog.Info("consumidor iniciado", "queue", c.queueURL)
	for ctx.Err() == nil {
		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(c.queueURL),
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     c.waitSeconds,
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			slog.Error("erro ao ler a fila", "queue", c.queueURL, "error", err.Error())
			select {
			case <-ctx.Done():
			case <-time.After(c.retryDelay):
			}
			continue
		}
		for _, m := range out.Messages {
			c.Process(ctx, m)
		}
	}
	slog.Info("consumidor encerrado", "queue", c.queueURL)
}

// Process trata uma mensagem: ignora repetidas, aplica o evento e apaga a
// mensagem quando não há mais nada a fazer com ela.
func (c *SQSConsumer) Process(ctx context.Context, m types.Message) {
	var env messaging.Envelope
	if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &env); err != nil || env.MessageID == "" || env.OrderID == "" {
		slog.Error("mensagem inválida descartada", "body", aws.ToString(m.Body))
		c.delete(ctx, m)
		return
	}
	attrs := []any{"type", env.Type, "message_id", env.MessageID, "order_id", env.OrderID, "correlation_id", env.CorrelationID}

	done, err := c.processed.IsProcessed(env.MessageID)
	if err != nil {
		slog.Warn("erro ao verificar mensagem; será tentada de novo", append(attrs, "error", err.Error())...)
		return
	}
	if done {
		slog.Info("evento repetido ignorado", attrs...)
		c.delete(ctx, m)
		return
	}

	err = c.handler.HandleEvent(ctx, env)
	switch {
	case isPermanent(err):
		slog.Error("evento descartado", append(attrs, "error", err.Error())...)
	case err != nil:
		slog.Warn("falha ao processar evento; será tentado de novo", append(attrs, "error", err.Error())...)
		return
	default:
		slog.Info("evento processado", attrs...)
	}
	c.delete(ctx, m)
}

// isPermanent indica erros que não se resolvem tentando de novo.
func isPermanent(err error) bool {
	return errors.Is(err, domain.ErrSagaNotFound) || errors.Is(err, usecase.ErrUnknownEvent)
}

func (c *SQSConsumer) delete(ctx context.Context, m types.Message) {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	})
	if err != nil {
		slog.Warn("erro ao apagar mensagem da fila", "error", err.Error())
	}
}
