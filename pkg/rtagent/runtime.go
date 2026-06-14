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

type Runtime struct {
	kernel  *runtimeKernel
	workDir string

	modelProvider       ModelProvider
	toolProvider        ToolProvider
	memoryProvider      MemoryProvider
	hypothesisProvider  HypothesisProvider
	mcpProvider         MCPProvider
	skillProvider       SkillProvider
	worldStateProviders []WorldStateProvider
	maxToolIterations   int
	runLeaseTTL         time.Duration

	eventMu   sync.Mutex
	closeOnce sync.Once
	closeErr  error
	closed    atomic.Bool

	wsCache *worldStateCache
}

func Open(ctx context.Context, cfg Config) (*Runtime, error) {
	runtimeCfg, workDir, err := normalizeRuntimeConfig(cfg.Runtime)
	if err != nil {
		return nil, err
	}
	kernel, err := bootstrapRuntimeKernel(ctx, runtimeCfg.DSN, workDir)
	if err != nil {
		return nil, err
	}
	return newRuntimeFromKernel(runtimeCfg, workDir, cfg.Host, kernel), nil
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
		maxToolIterations = 4
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
		maxToolIterations:   maxToolIterations,
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
