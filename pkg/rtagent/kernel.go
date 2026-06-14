package rtagent

import (
	"context"
	"time"

	"rtagent/internal/domain/contextual"
	"rtagent/internal/domain/persistence"
	domainworld "rtagent/internal/domain/worldstate"
	"rtagent/internal/runtime/events"
	"rtagent/internal/startup"
)

type runtimeEventPublisher interface {
	Publish(ctx context.Context, event events.Event) error
}

type runLeaseManager interface {
	Acquire(ctx context.Context, resource string, activityID string, ttl time.Duration) (string, error)
	Release(ctx context.Context, leaseID string) error
}

type contextHandleRegistry interface {
	Register(ctx context.Context, handle contextual.ContextHandle) error
	ListByRunID(ctx context.Context, runID string) ([]contextual.ContextHandle, error)
}

type contextMaterializer interface {
	Materialize(ctx context.Context, handleID string) (string, error)
}

type workspaceWriter interface {
	WriteFile(ctx context.Context, relativePath string, content []byte, activeActivityID string, runID string) (persistence.ArtifactRecord, error)
}

type worldStateProjector interface {
	RebuildAll(ctx context.Context, runID string) error
	GetLatestSnapshot(ctx context.Context, runID string) ([]domainworld.Entry, error)
}

type runtimeStore interface {
	persistence.RunStore
	persistence.ThreadStore
	persistence.CheckpointStore
	persistence.RuntimeEventStore
	persistence.MemoryStore
	persistence.ActivityStore
	persistence.CapabilityStore
	persistence.ToolSchemaStore
	persistence.PermissionStore
	persistence.GrantStore
}

type runtimeKernel struct {
	store           runtimeStore
	eventBus        runtimeEventPublisher
	leaseManager    runLeaseManager
	contextRegistry contextHandleRegistry
	materializer    contextMaterializer
	workspace       workspaceWriter
	wsBuilder       worldStateProjector
	closeFn         func() error
}

func bootstrapRuntimeKernel(ctx context.Context, dsn, workDir string) (*runtimeKernel, error) {
	container, err := startup.BootstrapSystem(ctx, dsn, workDir)
	if err != nil {
		return nil, err
	}
	closeKernel := func() error {
		if container.EventBus != nil {
			container.EventBus.Close()
		}
		if container.DB == nil {
			return nil
		}
		sqlDB, err := container.DB.DB()
		if err != nil || sqlDB == nil {
			return err
		}
		return sqlDB.Close()
	}
	return &runtimeKernel{
		store:           container.Store,
		eventBus:        container.EventBus,
		leaseManager:    container.LeaseManager,
		contextRegistry: container.ContextRegistry,
		materializer:    container.Materializer,
		workspace:       container.Workspace,
		wsBuilder:       container.WSBuilder,
		closeFn:         closeKernel,
	}, nil
}

func (k *runtimeKernel) close() error {
	if k == nil {
		return nil
	}
	if k.closeFn == nil {
		return nil
	}
	return k.closeFn()
}
