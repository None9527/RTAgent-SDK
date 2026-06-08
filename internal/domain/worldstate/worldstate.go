package worldstate

import (
	"context"

	"rtagent/internal/domain/persistence"
)

type Partition string

const (
	PartitionActivity   Partition = "activity"
	PartitionTask       Partition = "task"
	PartitionCapability Partition = "capability"
	PartitionGovernance Partition = "governance"
	PartitionMemory     Partition = "memory"
	PartitionContext     Partition = "context"
	PartitionHypothesis Partition = "hypothesis"
	PartitionArtifact   Partition = "artifact"
)

type WorldStateQuery struct {
	RunID     string    `json:"run_id,omitempty"`
	Partition Partition `json:"partition,omitempty"`
	Subject   string    `json:"subject,omitempty"`
}

type ProjectionQuery interface {
	GetWorldStateSnapshot(ctx context.Context, runID string) ([]persistence.WorldStateEntry, error)
	GetWorldStatePartition(ctx context.Context, runID string, partition Partition) ([]persistence.WorldStateEntry, error)
	Rebuild(ctx context.Context, runID string) error
}

type WorldStateBuilder interface {
	HandleEvent(ctx context.Context, kind string, runID string, seq int64, payload []byte) error
	RebuildAll(ctx context.Context, runID string) error
	GetLatestSnapshot(ctx context.Context, runID string) ([]persistence.WorldStateEntry, error)
}
