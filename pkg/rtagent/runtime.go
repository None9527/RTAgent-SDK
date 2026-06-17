package rtagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrRuntimeClosed = errors.New("rtagent runtime is closed")

// Compile-time check: Runtime satisfies the PermissionCenter interface so hosts
// can inject a custom PermissionCenter into HostPorts or fall back to the
// built-in Runtime implementation.
var _ PermissionCenter = (*Runtime)(nil)

// defaultMaxToolIterations is the fallback iteration budget when
// RuntimeConfig.MaxToolIterations is unset. It is a conservative middle ground
// between ngoagent's minimum (16) and default (64) — enough for realistic
// multi-step tool use without unbounded looping under the deterministic
// placeholder provider. Hosts running real tasks should tune it explicitly.
const defaultMaxToolIterations = 32

type Runtime struct {
	kernel  *runtimeKernel
	workDir string
	home    *RuntimeHomeLayout

	modelProvider       ModelProvider
	toolProvider        ToolProvider
	memoryProvider      MemoryProvider
	hypothesisProvider  HypothesisProvider
	mcpProvider         MCPProvider
	skillProvider       SkillProvider
	worldStateProviders []WorldStateProvider
	permissionCenter    PermissionCenter // optional custom permission center from HostPorts
	maxToolIterations   int
	maxContextMessages  int
	runLeaseTTL         time.Duration

	eventMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool

	wsCache *worldStateCache
}

func Open(ctx context.Context, cfg Config) (*Runtime, error) {
	runtimeCfg, workDir, homeLayout, err := resolveRuntimeConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	kernel, err := bootstrapRuntimeKernel(ctx, runtimeCfg.DSN, workDir)
	if err != nil {
		return nil, err
	}
	rt := newRuntimeFromKernel(runtimeCfg, workDir, cfg.Host, kernel)
	rt.home = homeLayout
	return rt, nil
}

// resolveRuntimeConfig applies the RuntimeHome resolver (if configured) before
// normalizing. When DSN is explicitly set, RuntimeHome is ignored — the host
// has taken full control of storage. When DSN is empty and RuntimeHome is nil,
// the ephemeral in-memory default is used (zero-breakage).
func resolveRuntimeConfig(ctx context.Context, cfg Config) (RuntimeConfig, string, *RuntimeHomeLayout, error) {
	runtimeCfg := cfg.Runtime
	var homeLayout *RuntimeHomeLayout
	if strings.TrimSpace(runtimeCfg.DSN) == "" && cfg.RuntimeHome != nil {
		layout, err := cfg.RuntimeHome.Resolve(ctx)
		if err != nil {
			return RuntimeConfig{}, "", nil, fmt.Errorf("resolve runtime home: %w", err)
		}
		if strings.TrimSpace(layout.DSN) != "" {
			runtimeCfg.DSN = layout.DSN
		}
		if strings.TrimSpace(runtimeCfg.WorkDir) == "" && strings.TrimSpace(layout.WorkDir) != "" {
			runtimeCfg.WorkDir = layout.WorkDir
		}
		homeLayout = &layout
	}
	normalized, workDir, err := normalizeRuntimeConfig(runtimeCfg)
	if err != nil {
		return RuntimeConfig{}, "", nil, err
	}
	return normalized, workDir, homeLayout, nil
}

// Home returns the resolved runtime home directory layout, if one was
// configured. Returns a zero-value RuntimeHomeLayout when no RuntimeHome was
// set (ephemeral or explicit-DSN mode). Host providers can read this to locate
// shared directories (skills, memory, config) under a common root.
func (r *Runtime) Home() RuntimeHomeLayout {
	if r == nil || r.home == nil {
		return RuntimeHomeLayout{}
	}
	return *r.home
}

func normalizeRuntimeConfig(runtimeCfg RuntimeConfig) (RuntimeConfig, string, error) {
	if strings.TrimSpace(runtimeCfg.DSN) == "" {
		runtimeCfg.DSN = defaultRuntimeDSN()
	}
	if strings.TrimSpace(runtimeCfg.WorkDir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return RuntimeConfig{}, "", fmt.Errorf("get working directory: %w", err)
		}
		runtimeCfg.WorkDir = wd
	}
	workDir, err := filepath.Abs(runtimeCfg.WorkDir)
	if err != nil {
		return RuntimeConfig{}, "", fmt.Errorf("resolve work directory: %w", err)
	}
	return runtimeCfg, workDir, nil
}

func newRuntimeFromKernel(runtimeCfg RuntimeConfig, workDir string, host HostPorts, kernel *runtimeKernel) *Runtime {
	modelProvider := host.Model
	if modelProvider == nil {
		modelProvider = echoModelProvider{}
	}
	maxToolIterations := runtimeCfg.MaxToolIterations
	if maxToolIterations <= 0 {
		maxToolIterations = defaultMaxToolIterations
	}
	maxContextMessages := runtimeCfg.MaxContextMessages
	if maxContextMessages < 0 {
		maxContextMessages = 0
	}
	// When the host did not set an explicit MaxContextMessages, derive one from
	// the model provider's declared context window (if any). This keeps the
	// context budget tied to the actual model capability rather than a generic
	// runtime config knob. Explicit MaxContextMessages always wins.
	if maxContextMessages == 0 {
		maxContextMessages = deriveContextMessageBudget(modelProvider)
	}
	runLeaseTTL := runtimeCfg.RunLeaseTTL
	if runLeaseTTL <= 0 {
		runLeaseTTL = 5 * time.Minute
	}
	return &Runtime{
		kernel:              kernel,
		workDir:             workDir,
		modelProvider:       modelProvider,
		toolProvider:        configuredToolProvider(host.Tools),
		memoryProvider:      host.Memory,
		hypothesisProvider:  host.Hypothesis,
		mcpProvider:         host.MCP,
		skillProvider:       host.Skill,
		worldStateProviders: append([]WorldStateProvider(nil), host.WorldState...),
		permissionCenter:    host.PermissionCenter,
		maxToolIterations:   maxToolIterations,
		maxContextMessages:  maxContextMessages,
		runLeaseTTL:         runLeaseTTL,
		wsCache:             newWorldStateCache(),
	}
}

func defaultRuntimeDSN() string {
	return fmt.Sprintf("file:rtagent-runtime-%d?mode=memory&cache=shared", time.Now().UTC().UnixNano())
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		if r.kernel != nil {
			r.closeErr = r.kernel.close()
		}
	})
	return r.closeErr
}

func (r *Runtime) ensureReady() error {
	if r == nil {
		return errors.New("rtagent runtime is not initialized")
	}
	if r.closed.Load() {
		return ErrRuntimeClosed
	}
	if r.kernel == nil || r.kernel.store == nil {
		return errors.New("rtagent runtime is not initialized")
	}
	return nil
}
