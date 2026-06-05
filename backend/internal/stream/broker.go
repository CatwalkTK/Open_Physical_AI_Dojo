package stream

import (
	"sync"
)

type Broker struct {
	mu      sync.Mutex
	clients map[string]map[chan any]struct{}
}

func NewBroker() *Broker {
	return &Broker{clients: map[string]map[chan any]struct{}{}}
}

func (b *Broker) Subscribe(taskID string) chan any {
	ch := make(chan any, 16)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.clients[taskID] == nil {
		b.clients[taskID] = map[chan any]struct{}{}
	}
	b.clients[taskID][ch] = struct{}{}
	return ch
}

func (b *Broker) Unsubscribe(taskID string, ch chan any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients[taskID], ch)
	close(ch)
}

func (b *Broker) Publish(taskID string, payload any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients[taskID] {
		select {
		case ch <- payload:
		default:
		}
	}
}
