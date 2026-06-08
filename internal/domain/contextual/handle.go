package contextual

import (
	"context"
)

type HandleKind string

const (
	HandleArtifact HandleKind = "artifact"
	HandleEvidence HandleKind = "evidence"
	HandleMemory   HandleKind = "memory"
	HandleActivity HandleKind = "activity"
)

type ContextHandle struct {
	HandleID              string     `json:"handle_id"`
	RunID                 string     `json:"run_id"`
	Kind                  HandleKind `json:"kind"`
	Title                 string     `json:"title"`
	Summary               string     `json:"summary"`
	SourceRef             string     `json:"source_ref"`
	TokenEstimate         int        `json:"token_estimate"`
	Freshness             float64    `json:"freshness"`
	MaterializationPolicy string     `json:"materialization_policy"` // "preload" | "demand_load"
	EvidenceRefs          []string   `json:"evidence_refs,omitempty"`
}

type HandleRegistry interface {
	Register(ctx context.Context, handle ContextHandle) error
	Get(ctx context.Context, handleID string) (ContextHandle, error)
	ListByRunID(ctx context.Context, runID string) ([]ContextHandle, error)
}

type Materializer interface {
	Materialize(ctx context.Context, handleID string) (string, error)
}
