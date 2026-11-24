package ext

import (
	"context"
	"sync"

	"personalcrm/internal/domain"
)

// EventHandler reacts to a published domain event.
type EventHandler func(ctx context.Context, ev domain.Event) error

// EventBus is a thread-safe in-process pub/sub bus connecting core services
// and pluggable modules without tight coupling.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[domain.EventType][]EventHandler
	wildcard    []EventHandler
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[domain.EventType][]EventHandler),
	}
}

// Subscribe registers a handler for a specific EventType.
func (b *EventBus) Subscribe(eventType domain.EventType, h EventHandler) {
	if h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], h)
}

// SubscribeAll registers a handler invoked for every domain event.
func (b *EventBus) SubscribeAll(h EventHandler) {
	if h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.wildcard = append(b.wildcard, h)
}

// Publish dispatches an event to all matching subscribers in registration order
// and returns any handler errors without stopping remaining subscribers.
func (b *EventBus) Publish(ctx context.Context, ev domain.Event) []error {
	b.mu.RLock()
	handlers := make([]EventHandler, 0, len(b.subscribers[ev.Type])+len(b.wildcard))
	handlers = append(handlers, b.subscribers[ev.Type]...)
	handlers = append(handlers, b.wildcard...)
	b.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, ev); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
