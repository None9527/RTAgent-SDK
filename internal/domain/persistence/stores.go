package persistence

import "context"

type RunStore interface {
	PutRun(ctx context.Context, rec RunRecord) error
	GetRun(ctx context.Context, runID string) (RunRecord, error)
	ListRunsBySession(ctx context.Context, sessionID string) ([]RunRecord, error)
}

type ThreadStore interface {
	PutThread(ctx context.Context, rec ThreadRecord) error
	GetThread(ctx context.Context, resumeID string) (ThreadRecord, error)
}

type CheckpointStore interface {
	AppendCheckpoint(ctx context.Context, rec CheckpointRecord) error
	GetCheckpoint(ctx context.Context, runID, checkpointID string) (CheckpointRecord, error)
	ListCheckpointsByRunID(ctx context.Context, runID string) ([]CheckpointRecord, error)
}

type RuntimeEventStore interface {
	AppendRuntimeEvent(ctx context.Context, rec RuntimeEventRecord) error
	ListRuntimeEvents(ctx context.Context, runID string) ([]RuntimeEventRecord, error)
	MaxEventSequence(ctx context.Context, runID string) (int64, error)
}

type MemoryStore interface {
	PutMemory(ctx context.Context, rec MemoryRecord) error
	ListMemoriesByRunID(ctx context.Context, runID string) ([]MemoryRecord, error)
}

type ArtifactStore interface {
	PutArtifact(ctx context.Context, rec ArtifactRecord) error
	GetArtifact(ctx context.Context, artifactID string) (ArtifactRecord, error)
	GetLatestArtifactByPath(ctx context.Context, path string) (ArtifactRecord, error)
}

type ActivityStore interface {
	PutActivity(ctx context.Context, rec ActivityRecord) error
	GetActivity(ctx context.Context, activityID string) (ActivityRecord, error)
}

type CapabilityStore interface {
	PutCapability(ctx context.Context, rec CapabilityRecord) error
}

type ToolSchemaStore interface {
	PutToolSchemaSnapshot(ctx context.Context, rec ToolSchemaSnapshotRecord) error
	GetToolSchemaSnapshot(ctx context.Context, snapshotID string) (ToolSchemaSnapshotRecord, error)
}

type PermissionStore interface {
	PutPermission(ctx context.Context, rec PermissionRecord) error
	GetPermission(ctx context.Context, permissionID string) (PermissionRecord, error)
}

type GrantStore interface {
	PutGrant(ctx context.Context, rec GrantRecord) error
	GetGrant(ctx context.Context, grantID string) (GrantRecord, error)
}

type LeaseStore interface {
	PutLease(ctx context.Context, rec LeaseRecord) error
	GetLease(ctx context.Context, leaseID string) (LeaseRecord, error)
	GetActiveLeaseByResource(ctx context.Context, resource string) (LeaseRecord, error)
}

type Bundle interface {
	RunStore
	ThreadStore
	CheckpointStore
	RuntimeEventStore
	MemoryStore
	ArtifactStore
	ActivityStore
	CapabilityStore
	ToolSchemaStore
	PermissionStore
	GrantStore
	LeaseStore
}
