package mocks

import (
	"errors"
	"sync"
	"time"

	"autoshop/internal/domain"
	"autoshop/internal/repository"
)

// SagaStoreFake guarda OS, Sagas e outbox em memória, com as mesmas regras
// do repositório real: gravação atômica, controle de versão da Saga e
// registro de eventos já processados.
type SagaStoreFake struct {
	mu        sync.Mutex
	Orders    map[string]domain.Order
	Sagas     map[string]domain.Saga // por order_id
	Outbox    []domain.OutboxMessage
	Published map[string]bool
	Processed map[string]bool

	// Erros programados para os testes
	SaveErr, FindErr, OutboxErr, MarkErr, CreateErr error
}

func NewSagaStoreFake() *SagaStoreFake {
	return &SagaStoreFake{
		Orders:    map[string]domain.Order{},
		Sagas:     map[string]domain.Saga{},
		Published: map[string]bool{},
		Processed: map[string]bool{},
	}
}

// --- OrderRepository ---

func (f *SagaStoreFake) Create(o domain.Order) (domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Orders[o.ID] = o
	return o, nil
}

func (f *SagaStoreFake) FindAll() ([]domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []domain.Order{}
	for _, o := range f.Orders {
		out = append(out, o)
	}
	return out, nil
}

func (f *SagaStoreFake) FindByID(id string) (domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.Orders[id]
	if !ok {
		return domain.Order{}, errors.New("ordem de serviço não encontrada")
	}
	return o, nil
}

func (f *SagaStoreFake) FindByCustomerID(string) ([]domain.Order, error) { return nil, nil }

func (f *SagaStoreFake) FindByStatus(domain.OrderStatus) ([]domain.Order, error) { return nil, nil }

func (f *SagaStoreFake) Update(o domain.Order) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Orders[o.ID] = o
	return nil
}

func (f *SagaStoreFake) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Orders, id)
	return nil
}

// --- SagaRepository ---

func (f *SagaStoreFake) CreateOrderWithSaga(o domain.Order, s domain.Saga, outbox []domain.OutboxMessage) (domain.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateErr != nil {
		return o, f.CreateErr
	}
	f.Orders[o.ID] = o
	f.Sagas[o.ID] = s
	f.Outbox = append(f.Outbox, outbox...)
	return o, nil
}

func (f *SagaStoreFake) FindByOrderID(orderID string) (domain.Saga, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FindErr != nil {
		return domain.Saga{}, f.FindErr
	}
	s, ok := f.Sagas[orderID]
	if !ok {
		return domain.Saga{}, domain.ErrSagaNotFound
	}
	return s, nil
}

func (f *SagaStoreFake) Save(c domain.SagaChange) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SaveErr != nil {
		return f.SaveErr
	}
	if c.ProcessedMessageID != "" && f.Processed[c.ProcessedMessageID] {
		return repository.ErrMessageAlreadyProcessed
	}
	current := f.Sagas[c.Saga.OrderID]
	if current.Version != c.Saga.Version {
		return repository.ErrSagaConcurrentUpdate
	}
	if c.ProcessedMessageID != "" {
		f.Processed[c.ProcessedMessageID] = true
	}
	if c.Order != nil {
		f.Orders[c.Order.ID] = *c.Order
	}
	s := c.Saga
	s.Version++
	f.Sagas[s.OrderID] = s
	f.Outbox = append(f.Outbox, c.Outbox...)
	return nil
}

func (f *SagaStoreFake) FindExpired(now time.Time) ([]domain.Saga, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FindErr != nil {
		return nil, f.FindErr
	}
	out := []domain.Saga{}
	for _, s := range f.Sagas {
		if s.IsExpired(now) {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *SagaStoreFake) IsProcessed(id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Processed[id], nil
}

func (f *SagaStoreFake) PendingOutbox(limit int) ([]domain.OutboxMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.OutboxErr != nil {
		return nil, f.OutboxErr
	}
	out := []domain.OutboxMessage{}
	for _, m := range f.Outbox {
		if !f.Published[m.ID] && len(out) < limit {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *SagaStoreFake) MarkPublished(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.MarkErr != nil {
		return f.MarkErr
	}
	f.Published[id] = true
	return nil
}

// Commands devolve os tipos dos comandos gravados na outbox, em ordem.
func (f *SagaStoreFake) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for _, m := range f.Outbox {
		out = append(out, m.Type)
	}
	return out
}

// SetSaga altera a Saga gravada (ex.: simular prazo vencido).
func (f *SagaStoreFake) SetSaga(s domain.Saga) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Sagas[s.OrderID] = s
}
