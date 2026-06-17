package contextengine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/None9527/RTAgent-SDK/internal/domain/contextual"
	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

type LocalHandleRegistry struct {
	mu      sync.RWMutex
	handles map[string]contextual.ContextHandle
	store   persistence.ContextHandleStore
}

func NewLocalHandleRegistry() *LocalHandleRegistry {
	return &LocalHandleRegistry{
		handles: make(map[string]contextual.ContextHandle),
	}
}

// NewLocalHandleRegistryWithStore creates a registry backed by a persistent
// store. When store is non-nil, Register writes through to the store and Get /
// ListByRunID fall back to the store when the handle is not in the in-memory
// cache.
func NewLocalHandleRegistryWithStore(store persistence.ContextHandleStore) *LocalHandleRegistry {
	return &LocalHandleRegistry{
		handles: make(map[string]contextual.ContextHandle),
		store:   store,
	}
}

func (r *LocalHandleRegistry) Register(ctx context.Context, handle contextual.ContextHandle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if handle.HandleID == "" {
		return errors.New("handle ID cannot be empty")
	}
	r.handles[handle.HandleID] = handle

	if r.store != nil {
		rec := persistence.ContextHandleRecord{
			HandleID:              handle.HandleID,
			RunID:                 handle.RunID,
			Kind:                  string(handle.Kind),
			Title:                 handle.Title,
			Summary:               handle.Summary,
			SourceRef:             handle.SourceRef,
			TokenEstimate:         handle.TokenEstimate,
			Freshness:             handle.Freshness,
			MaterializationPolicy: handle.MaterializationPolicy,
			EvidenceRefs:          append([]string(nil), handle.EvidenceRefs...),
		}
		if err := r.store.PutContextHandle(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func (r *LocalHandleRegistry) Get(ctx context.Context, handleID string) (contextual.ContextHandle, error) {
	r.mu.RLock()
	handle, exists := r.handles[handleID]
	r.mu.RUnlock()

	if exists {
		return handle, nil
	}

	// Fall back to persistent store.
	if r.store != nil {
		rec, err := r.store.GetContextHandle(ctx, handleID)
		if err != nil {
			// Distinguish "not found" (benign cache miss) from real store
			// errors (DB failures) so the latter surface instead of being
			// masked as a missing handle.
			if isContextHandleNotFound(err) {
				return contextual.ContextHandle{}, errors.New("context handle not found")
			}
			return contextual.ContextHandle{}, fmt.Errorf("get context handle from store: %w", err)
		}
		ch := contextual.ContextHandle{
			HandleID:              rec.HandleID,
			RunID:                 rec.RunID,
			Kind:                  contextual.HandleKind(rec.Kind),
			Title:                 rec.Title,
			Summary:               rec.Summary,
			SourceRef:             rec.SourceRef,
			TokenEstimate:         rec.TokenEstimate,
			Freshness:             rec.Freshness,
			MaterializationPolicy: rec.MaterializationPolicy,
			EvidenceRefs:          append([]string(nil), rec.EvidenceRefs...),
		}
		// Populate in-memory cache for subsequent lookups.
		r.mu.Lock()
		r.handles[handleID] = ch
		r.mu.Unlock()
		return ch, nil
	}

	return contextual.ContextHandle{}, errors.New("context handle not found")
}

func (r *LocalHandleRegistry) ListByRunID(ctx context.Context, runID string) ([]contextual.ContextHandle, error) {
	r.mu.RLock()
	var results []contextual.ContextHandle
	for _, handle := range r.handles {
		if handle.RunID == runID {
			results = append(results, handle)
		}
	}
	r.mu.RUnlock()

	// Merge with persistent store results.
	if r.store != nil {
		recs, err := r.store.ListContextHandlesByRunID(ctx, runID)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool, len(results))
		for _, h := range results {
			seen[h.HandleID] = true
		}
		for _, rec := range recs {
			if seen[rec.HandleID] {
				continue
			}
			ch := contextual.ContextHandle{
				HandleID:              rec.HandleID,
				RunID:                 rec.RunID,
				Kind:                  contextual.HandleKind(rec.Kind),
				Title:                 rec.Title,
				Summary:               rec.Summary,
				SourceRef:             rec.SourceRef,
				TokenEstimate:         rec.TokenEstimate,
				Freshness:             rec.Freshness,
				MaterializationPolicy: rec.MaterializationPolicy,
				EvidenceRefs:          append([]string(nil), rec.EvidenceRefs...),
			}
			results = append(results, ch)
			// Populate in-memory cache.
			r.mu.Lock()
			r.handles[rec.HandleID] = ch
			r.mu.Unlock()
		}
	}
	return results, nil
}

// isContextHandleNotFound reports whether err represents a missing-record
// condition from the backing store (e.g. gorm.ErrRecordNotFound surfaced as
// "record not found"). It avoids importing the gorm package by matching the
// well-known error substring.
func isContextHandleNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}
