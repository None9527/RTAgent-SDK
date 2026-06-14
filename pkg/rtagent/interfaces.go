package rtagent

import "context"

type PermissionCenter interface {
	CheckPermission(ctx context.Context, req PermissionCheckRequest) (PermissionCheckResult, error)
	ResolvePermission(ctx context.Context, req PermissionDecisionRequest) (PermissionDecisionResult, error)
}

type ToolProvider interface {
	ToolSpecs(ctx context.Context, scope ExecutionScope) ([]ToolSpec, error)
	ExecuteTool(ctx context.Context, scope ExecutionScope, call ToolCall) (ToolObservation, error)
}

type ModelProvider interface {
	CompleteTurn(ctx context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error)
}

type MemoryProvider interface {
	MemoryFacts(ctx context.Context, scope ExecutionScope) ([]MemoryFact, error)
}

type HypothesisProvider interface {
	Hypotheses(ctx context.Context, scope ExecutionScope) ([]HypothesisFact, error)
}

type MCPProvider interface {
	MCPInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error)
}

type SkillProvider interface {
	SkillInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error)
}

type WorldStateProvider interface {
	Partition() string
	BuildWorldState(ctx context.Context, input WorldStateProviderInput) (WorldStatePartition, error)
}
