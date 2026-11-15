package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/mocks"
	"autoshop/internal/worker"

	"github.com/stretchr/testify/assert"
)

type fakeSender struct {
	errs []error
	sent []string
}

func (s *fakeSender) Send(_ context.Context, queue string, _ []byte) error {
	if len(s.errs) > 0 {
		err := s.errs[0]
		s.errs = s.errs[1:]
		if err != nil {
			return err
		}
	}
	s.sent = append(s.sent, queue)
	return nil
}

func storeWith(n int) *mocks.SagaStoreFake {
	store := mocks.NewSagaStoreFake()
	for i := 0; i < n; i++ {
		store.Outbox = append(store.Outbox, domain.OutboxMessage{ID: string(rune('a' + i)), Queue: "billing-commands", Type: "X"})
	}
	return store
}

func TestOutboxRelay(t *testing.T) {
	store := storeWith(3)
	sender := &fakeSender{errs: []error{nil, errors.New("sqs fora")}}
	relay := worker.NewOutboxRelay(store, sender)

	assert.Equal(t, 2, relay.PublishPending(context.Background()))
	assert.False(t, store.Published["b"], "o que falhou continua pendente")

	assert.Equal(t, 1, relay.PublishPending(context.Background()))
	assert.Equal(t, 0, relay.PublishPending(context.Background()))
}

func TestOutboxRelayErrors(t *testing.T) {
	store := storeWith(1)
	relay := worker.NewOutboxRelay(store, &fakeSender{})

	store.OutboxErr = errors.New("banco fora")
	assert.Equal(t, 0, relay.PublishPending(context.Background()))

	store.OutboxErr = nil
	store.MarkErr = errors.New("banco fora")
	assert.Equal(t, 0, relay.PublishPending(context.Background()))
}

type fakeTimeouts struct {
	n   int
	err error
}

func (f fakeTimeouts) CheckTimeouts(time.Time) (int, error) { return f.n, f.err }

func TestCheckTimeouts(t *testing.T) {
	// Só registra em log; não pode quebrar em nenhum caso
	worker.CheckTimeouts(fakeTimeouts{n: 2}, time.Now())
	worker.CheckTimeouts(fakeTimeouts{}, time.Now())
	worker.CheckTimeouts(fakeTimeouts{err: errors.New("banco")}, time.Now())
}

func TestEvery(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Every(ctx, 10*time.Millisecond, func(time.Time) { calls.Add(1) })
		close(done)
	}()
	assert.Eventually(t, func() bool { return calls.Load() >= 2 }, 2*time.Second, 5*time.Millisecond)
	cancel()
	<-done
}
