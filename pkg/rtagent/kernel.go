package rtagent

import (
	"context"
	"time"

	"github.com/None9527/RTAgent-SDK/internal/domain/contextual"
	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
	"github.com/None9527/RTAgent-SDK/internal/runtime/events"
	"github.com/None9527/RTAgent-SDK/internal/startup"
)

type runtimeEventPublisher interface {
	Publish(ctx context.Context, event events.Event) error
}

type runLeaseManager interface {
	Acquire(ctx context.Context, resource string, activityID string, ttl time.Duration) (string, error)
	Release(ctx context.Context, leaseID string) error
	Renew(ctx context.Context, leaseID string, ttl time.Duration) error
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
	persistence.ContextHandleStore
}

type runtimeKernel struct {
	store           runtimeStore
	eventBus        runtimeEventPublisher
	leaseManager    runLeaseManager
	contextRegistry contextHandleRegistry
	materializer    contextMaterializer
	workspace       workspaceWriter
	closeFn         func() error
}

func bootstrapRuntimeKernel(ctx context.Context, dsn, workDir string) (*runtimeKernel, error) {
	container, err := startup.BootstrapSystem(ctx, dsn, workDir)
	if err != nil {
		return nil, err
	}
closeKernel := func() error {
			return container.Close()
		}
	return &runtimeKernel{
		store:           container.Store,
		eventBus:        container.EventBus,
		leaseManager:    container.LeaseManager,
		contextRegistry: container.ContextRegistry,
		materializer:    container.Materializer,
		workspace:       container.Workspace,
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
