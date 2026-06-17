package rtagent

import (
	"context"
	"time"
)

type Config struct {
	Runtime RuntimeConfig
	Host    HostPorts
	// RuntimeHome, when set, resolves a persistent home directory layout used
	// when RuntimeConfig.DSN is empty. If nil and DSN is empty, the SDK falls
	// back to ephemeral in-memory storage (current behavior, zero-breakage).
	// Inject this to make zero-config Open durable AND to give host providers a
	// shared directory convention. The SDK never creates directories itself —
	// the injected RuntimeHome implementation owns directory creation.
	RuntimeHome RuntimeHome
}

// RuntimeHome resolves a runtime home directory layout. It is the customization
// seam for "where does this runtime live on disk" — hosts inject an
// implementation that owns the directory convention (location, structure,
// permissions, creation). See DefaultUserHome for a ready-to-use implementation.
type RuntimeHome interface {
	// Resolve returns the resolved home layout. Called once during Open when
	// RuntimeConfig.DSN is empty. If Resolve returns an error, Open fails.
	Resolve(ctx context.Context) (RuntimeHomeLayout, error)
}

// RuntimeHomeLayout is the resolved directory layout. The kernel consumes DSN
// and WorkDir; host code (providers, examples) may read the other paths to
// locate skills, memory, MCP configs, etc. All paths are absolute.
type RuntimeHomeLayout struct {
	// HomeDir is the root directory of the runtime home (e.g. ~/.myagent).
	HomeDir string
	// DSN is the SQLite data source name. The kernel consumes this.
	DSN string
	// WorkDir is the workspace path. The kernel consumes this when
	// RuntimeConfig.WorkDir is empty.
	WorkDir string
	// SkillsDir is where skill inventory lives. Host SkillProvider reads this.
	// Empty if the layout does not define a skills directory.
	SkillsDir string
	// MemoryDir is where memory store lives. Host MemoryProvider reads this.
	// Empty if the layout does not define a memory directory.
	MemoryDir string
	// ConfigDir is where host config files live. Host reads this.
	// Empty if the layout does not define a config directory.
	ConfigDir string
}

// RuntimeHomeFunc adapts a function to RuntimeHome for simple host integrations.
type RuntimeHomeFunc func(ctx context.Context) (RuntimeHomeLayout, error)

func (f RuntimeHomeFunc) Resolve(ctx context.Context) (RuntimeHomeLayout, error) {
	return f(ctx)
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
	// PermissionCenter is an optional custom permission implementation. When
	// nil (default), the Runtime uses its built-in CheckPermission /
	// ResolvePermission methods. Hosts inject a PermissionCenter to customize
	// policy evaluation, grant storage, or approval workflows.
	PermissionCenter PermissionCenter
}
