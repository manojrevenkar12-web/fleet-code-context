// Package bus publishes and consumes events.
package bus

import (
	"context"
	"encoding/json"
	"sync"
)

// Handler handles one event payload.
type Handler func(ctx context.Context, payload []byte) error

// Bus is the event bus every service uses.
type Bus interface {
	Publish(ctx context.Context, topic string, msg any) error
	Subscribe(topic string, h Handler)
}

// Memory is an in-process bus for tests.
type Memory struct {
	mu   sync.Mutex
	subs map[string][]Handler
}

func NewMemory() *Memory { return &Memory{subs: map[string][]Handler{}} }

func (m *Memory) Publish(ctx context.Context, topic string, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	m.mu.Lock()
	hs := append([]Handler(nil), m.subs[topic]...)
	m.mu.Unlock()
	for _, h := range hs {
		if err := h(ctx, b); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memory) Subscribe(topic string, h Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs[topic] = append(m.subs[topic], h)
}
