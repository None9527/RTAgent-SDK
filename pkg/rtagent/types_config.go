package rtagent

import "time"

type Config struct {
	Runtime RuntimeConfig
	Host    HostPorts
}

type RuntimeConfig struct {
	DSN               string
	WorkDir           string
	MaxToolIterations int
	// MaxContextMessages bounds the conversation message history passed to the
	// model per turn. When > 0, the loop keeps the first user message (task
	// context) plus the most recent (MaxContextMessages-1) messages, dropping
	// older middle messages so the conversation cannot grow unbounded and
	// overflow the model's context window. When 0 (default), no trimming is
	// applied and behavior is unchanged. This is a message-count window, not a
	// token budget: the kernel intentionally avoids tokenizer coupling.
	// Hosts should set it based on their model's context window and typical
	// message size.
	MaxContextMessages int
	RunLeaseTTL        time.Duration
}

type HostPorts struct {
	Model      ModelProvider
	Tools      []ToolProvider
	Memory     MemoryProvider
	Hypothesis HypothesisProvider
	MCP        MCPProvider
	Skill      SkillProvider
	WorldState []WorldStateProvider
}
