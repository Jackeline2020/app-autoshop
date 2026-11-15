package consumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"autoshop/internal/consumer"
	"autoshop/internal/domain"
	"autoshop/internal/messaging"
	"autoshop/internal/mocks"
	"autoshop/internal/usecase"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/assert"
)

type fakeHandler struct {
	err   error
	calls int
}

func (h *fakeHandler) HandleEvent(context.Context, messaging.Envelope) error {
	h.calls++
	return h.err
}

type fakeProcessed struct {
	done bool
	err  error
}

func (p fakeProcessed) IsProcessed(string) (bool, error) { return p.done, p.err }

func msg(body string) types.Message {
	return types.Message{Body: aws.String(body), ReceiptHandle: aws.String("rh")}
}

const validEvent = `{"message_id":"m1","type":"BudgetGenerated","saga_id":"s1","order_id":"os-1"}`

func TestProcess(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		handlerErr  error
		processed   fakeProcessed
		wantCalls   int
		wantDeleted int
	}{
		{"evento aplicado", validEvent, nil, fakeProcessed{}, 1, 1},
		{"json inválido", `{`, nil, fakeProcessed{}, 0, 1},
		{"sem order_id", `{"message_id":"m1"}`, nil, fakeProcessed{}, 0, 1},
		{"evento repetido", validEvent, nil, fakeProcessed{done: true}, 0, 1},
		{"erro ao verificar repetição", validEvent, nil, fakeProcessed{err: errors.New("banco")}, 0, 0},
		{"erro técnico volta para a fila", validEvent, errors.New("banco"), fakeProcessed{}, 1, 0},
		{"saga inexistente é descartada", validEvent, domain.ErrSagaNotFound, fakeProcessed{}, 1, 1},
		{"evento desconhecido é descartado", validEvent, usecase.ErrUnknownEvent, fakeProcessed{}, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sqsFake := &mocks.SQSFake{}
			h := &fakeHandler{err: tc.handlerErr}
			c := consumer.NewSQSConsumer(sqsFake, "http://queue/os-events", h, tc.processed)
			c.Process(context.Background(), msg(tc.body))
			assert.Equal(t, tc.wantCalls, h.calls)
			assert.Equal(t, tc.wantDeleted, sqsFake.DeletedCount())
		})
	}
}

func TestRun(t *testing.T) {
	sqsFake := &mocks.SQSFake{Receives: []mocks.ReceiveResult{
		{Err: errors.New("fila indisponível")},
		{Messages: []types.Message{msg(validEvent)}},
	}}
	h := &fakeHandler{}
	c := consumer.NewSQSConsumer(sqsFake, "q", h, fakeProcessed{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	assert.Eventually(t, func() bool { return sqsFake.DeletedCount() == 1 }, 5*time.Second, 20*time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, 1, h.calls)
}
