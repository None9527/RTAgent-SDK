package rtagent

import (
	"context"
	"errors"
)

// ModelProviderFunc adapts a function to ModelProvider.
type ModelProviderFunc func(context.Context, ModelRequest, ModelStreamHandler) (ModelResponse, error)

func (fn ModelProviderFunc) CompleteTurn(ctx context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error) {
	if fn == nil {
		return ModelResponse{}, errors.New("rtagent: nil ModelProviderFunc")
	}
	return fn(ctx, req, stream)
}

// ToolProviderAdapter adapts host functions to ToolProvider.
type ToolProviderAdapter struct {
	Specs   func(context.Context, ExecutionScope) ([]ToolSpec, error)
	Execute func(context.Context, ExecutionScope, ToolCall) (ToolObservation, error)
}

func (p ToolProviderAdapter) ToolSpecs(ctx context.Context, scope ExecutionScope) ([]ToolSpec, error) {
	if p.Specs == nil {
		return nil, nil
	}
	return p.Specs(ctx, scope)
}

func (p ToolProviderAdapter) ExecuteTool(ctx context.Context, scope ExecutionScope, call ToolCall) (ToolObservation, error) {
	if p.Execute == nil {
		return ToolObservation{}, errors.New("rtagent: ToolProviderAdapter.Execute is nil")
	}
	return p.Execute(ctx, scope, call)
}

// MemoryProviderFunc adapts a function to MemoryProvider.
type MemoryProviderFunc func(context.Context, ExecutionScope) ([]MemoryFact, error)

func (fn MemoryProviderFunc) MemoryFacts(ctx context.Context, scope ExecutionScope) ([]MemoryFact, error) {
	if fn == nil {
		return nil, errors.New("rtagent: nil MemoryProviderFunc")
	}
	return fn(ctx, scope)
}

// HypothesisProviderFunc adapts a function to HypothesisProvider.
type HypothesisProviderFunc func(context.Context, ExecutionScope) ([]HypothesisFact, error)

func (fn HypothesisProviderFunc) Hypotheses(ctx context.Context, scope ExecutionScope) ([]HypothesisFact, error) {
	if fn == nil {
		return nil, errors.New("rtagent: nil HypothesisProviderFunc")
	}
	return fn(ctx, scope)
}

// MCPProviderFunc adapts a function to MCPProvider.
type MCPProviderFunc func(context.Context, ExecutionScope) ([]CapabilityInventoryItem, error)

func (fn MCPProviderFunc) MCPInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error) {
	if fn == nil {
		return nil, errors.New("rtagent: nil MCPProviderFunc")
	}
	return fn(ctx, scope)
}

// SkillProviderFunc adapts a function to SkillProvider.
type SkillProviderFunc func(context.Context, ExecutionScope) ([]CapabilityInventoryItem, error)

func (fn SkillProviderFunc) SkillInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error) {
	if fn == nil {
		return nil, errors.New("rtagent: nil SkillProviderFunc")
	}
	return fn(ctx, scope)
}

// WorldStateProviderAdapter adapts a host function to WorldStateProvider.
type WorldStateProviderAdapter struct {
	PartitionName string
	ProviderName  string
	Build         func(context.Context, WorldStateProviderInput) (WorldStatePartition, error)
}

func (p WorldStateProviderAdapter) Partition() string {
	return p.PartitionName
}

func (p WorldStateProviderAdapter) BuildWorldState(ctx context.Context, input WorldStateProviderInput) (WorldStatePartition, error) {
	if p.Build == nil {
		return WorldStatePartition{
			Partition: p.PartitionName,
			Provider:  firstNonEmpty(p.ProviderName, "host_worldstate_provider"),
			Source:    firstNonEmpty(p.ProviderName, "host_worldstate_provider"),
		}, errors.New("rtagent: WorldStateProviderAdapter.Build is nil")
	}
	partition, err := p.Build(ctx, input)
	if partition.Partition == "" {
		partition.Partition = p.PartitionName
	}
	if partition.Provider == "" && p.ProviderName != "" {
		partition.Provider = p.ProviderName
	}
	if partition.Source == "" && p.ProviderName != "" {
		partition.Source = p.ProviderName
	}
	return partition, err
}
