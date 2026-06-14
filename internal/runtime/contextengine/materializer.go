package contextengine

import (
	"context"
	"errors"
	"sort"
	"sync"

	"rtagent/internal/domain/contextual"
	"rtagent/internal/domain/persistence"
)

type LocalMaterializer struct {
	mu       sync.RWMutex
	registry contextual.HandleRegistry
	store    artifactReader
}

type artifactReader interface {
	GetArtifact(ctx context.Context, artifactID string) (persistence.ArtifactRecord, error)
}

func NewLocalMaterializer(registry contextual.HandleRegistry, store artifactReader) *LocalMaterializer {
	return &LocalMaterializer{
		registry: registry,
		store:    store,
	}
}

func (m *LocalMaterializer) Materialize(ctx context.Context, handleID string) (string, error) {
	handle, err := m.registry.Get(ctx, handleID)
	if err != nil {
		return "", err
	}

	switch handle.Kind {
	case contextual.HandleArtifact:
		art, err := m.store.GetArtifact(ctx, handle.SourceRef)
		if err != nil {
			return "", err
		}
		return art.Preview, nil
	default:
		return "", errors.New("unsupported handle kind for materialization")
	}
}

type TokenBudgetManager struct {
	MaxPromptTokens int
	ReservedForLLM  int
}

func NewTokenBudgetManager(maxPrompt, reserved int) *TokenBudgetManager {
	return &TokenBudgetManager{
		MaxPromptTokens: maxPrompt,
		ReservedForLLM:  reserved,
	}
}

func (m *TokenBudgetManager) AllocatePolicies(handles []contextual.ContextHandle) []contextual.ContextHandle {
	availableBudget := m.MaxPromptTokens - m.ReservedForLLM
	if availableBudget <= 0 {
		availableBudget = 8000
	}

	sortedHandles := make([]contextual.ContextHandle, len(handles))
	copy(sortedHandles, handles)
	sort.Slice(sortedHandles, func(i, j int) bool {
		return sortedHandles[i].TokenEstimate < sortedHandles[j].TokenEstimate
	})

	currentUsed := 0
	result := make([]contextual.ContextHandle, 0, len(handles))

	for _, handle := range sortedHandles {
		if handle.MaterializationPolicy == "required" {
			handle.MaterializationPolicy = "preload"
			currentUsed += handle.TokenEstimate
			result = append(result, handle)
			continue
		}

		if currentUsed+handle.TokenEstimate <= availableBudget {
			handle.MaterializationPolicy = "preload"
			currentUsed += handle.TokenEstimate
		} else {
			handle.MaterializationPolicy = "demand_load"
		}
		result = append(result, handle)
	}

	return result
}
