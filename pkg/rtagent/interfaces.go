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

// ModelCapabilities describes what a model can do. The kernel uses it to drive
// loop behavior (e.g. deriving a context-message budget from the model's token
// window). Providers declare their own capabilities — see
// ModelCapabilityProvider.
type ModelCapabilities struct {
	// ContextWindowTokens is the model's total context window in tokens
	// (input + output). When >0, the kernel uses it to derive a default
	// context-message budget so the loop does not overflow the window. When 0,
	// the kernel cannot derive a budget from capabilities and falls back to the
	// explicit RuntimeConfig.MaxContextMessages (if set) or no trimming.
	ContextWindowTokens int
	// MaxOutputTokens is the model's per-turn output token limit, if known.
	// Reserved for future use (output budgeting); 0 means unknown.
	MaxOutputTokens int
	// SupportsStreaming indicates whether the provider emits SSE/stream deltas.
	SupportsStreaming bool
}

// ModelCapabilityProvider is an OPTIONAL interface that a ModelProvider may
// implement to declare its capabilities. The kernel checks for it at Open time:
// if the provider declares ContextWindowTokens > 0, the kernel derives a
// context-message budget from it. This keeps model attributes (window size,
// output limits) with the model provider, not in generic runtime config.
//
// Providers that do not implement this interface are unaffected — the kernel
// falls back to explicit RuntimeConfig.MaxContextMessages or no trimming.
type ModelCapabilityProvider interface {
	Capabilities() ModelCapabilities
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
