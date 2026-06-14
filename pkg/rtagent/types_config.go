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
	RunLeaseTTL       time.Duration
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
