package events

import (
	"context"
	"errors"
	"sync"
	"time"
)

type EventListener func(ctx context.Context, ev Event)

type InProcessEventBus struct {
	mu        sync.RWMutex
	listeners map[Kind]map[uintptr]EventListener
	buffer    chan Event
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    bool
	nextID    uintptr
}

func NewInProcessEventBus(bufSize int) *InProcessEventBus {
	ctx, cancel := context.WithCancel(context.Background())
	bus := &InProcessEventBus{
		listeners: make(map[Kind]map[uintptr]EventListener),
		buffer:    make(chan Event, bufSize),
		ctx:       ctx,
		cancel:    cancel,
	}
	bus.wg.Add(1)
	go bus.dispatchLoop()
	return bus
}

func (b *InProcessEventBus) Publish(ctx context.Context, ev Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ev.Kind == "" {
		return errors.New("event kind is required")
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now().UTC()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("event bus is closed")
	}

	select {
	case b.buffer <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("event bus buffer overflow: backpressure active")
	}
}

func (b *InProcessEventBus) Subscribe(kind Kind, listener EventListener) uintptr {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.listeners[kind] == nil {
		b.listeners[kind] = make(map[uintptr]EventListener)
	}

	b.nextID++
	id := b.nextID
	b.listeners[kind][id] = listener
	return id
}

func (b *InProcessEventBus) Unsubscribe(kind Kind, id uintptr) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.listeners[kind] != nil {
		delete(b.listeners[kind], id)
	}
}

func (b *InProcessEventBus) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.cancel()
	close(b.buffer)
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *InProcessEventBus) dispatchLoop() {
	defer b.wg.Done()
	for {
		select {
		case ev, ok := <-b.buffer:
			if !ok {
				return
			}

			b.mu.RLock()
			var targets []EventListener
			for _, listener := range b.listeners[ev.Kind] {
				targets = append(targets, listener)
			}
			for _, listener := range b.listeners["*"] {
				targets = append(targets, listener)
			}
			b.mu.RUnlock()

			if len(targets) == 0 {
				continue
			}

			b.wg.Add(1)
			go func(event Event, list []EventListener) {
				defer b.wg.Done()
				ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
				defer cancel()

				for _, listener := range list {
					if err := ctx.Err(); err != nil {
						break
					}
					listener(ctx, event)
				}
			}(ev, targets)

		case <-b.ctx.Done():
			return
		}
	}
}
