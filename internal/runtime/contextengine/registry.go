package contextengine

import (
	"context"
	"errors"
	"sync"

	"rtagent/internal/domain/contextual"
)

type LocalHandleRegistry struct {
	mu      sync.RWMutex
	handles map[string]contextual.ContextHandle
}

func NewLocalHandleRegistry() *LocalHandleRegistry {
	return &LocalHandleRegistry{
		handles: make(map[string]contextual.ContextHandle),
	}
}

func (r *LocalHandleRegistry) Register(ctx context.Context, handle contextual.ContextHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if handle.HandleID == "" {
		return errors.New("handle ID cannot be empty")
	}
	r.handles[handle.HandleID] = handle
	return nil
}

func (r *LocalHandleRegistry) Get(ctx context.Context, handleID string) (contextual.ContextHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handle, exists := r.handles[handleID]
	if !exists {
		return contextual.ContextHandle{}, errors.New("context handle not found")
	}
	return handle, nil
}

func (r *LocalHandleRegistry) ListByRunID(ctx context.Context, runID string) ([]contextual.ContextHandle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []contextual.ContextHandle
	for _, handle := range r.handles {
		if handle.RunID == runID {
			results = append(results, handle)
		}
	}
	return results, nil
}
