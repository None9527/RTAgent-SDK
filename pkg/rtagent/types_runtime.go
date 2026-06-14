package rtagent

import "time"

type ExecutionScope struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	TurnID      string `json:"turn_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	RootRunID   string `json:"root_run_id,omitempty"`
	ParentRunID string `json:"parent_run_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	ActorID     string `json:"actor_id,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
	ActorKind   string `json:"actor_kind,omitempty"`

	PermissionMode string `json:"permission_mode,omitempty"`
	PlanningState  string `json:"planning_state,omitempty"`
	TraceID        string `json:"trace_id,omitempty"`
}

type Identity struct {
	ActorID string `json:"actor_id,omitempty"`
	OwnerID string `json:"owner_id,omitempty"`
	Admin   bool   `json:"admin,omitempty"`
}

type SubmitRunRequest struct {
	Kind                  string         `json:"kind,omitempty"`
	RunID                 string         `json:"run_id,omitempty"`
	SessionID             string         `json:"session_id,omitempty"`
	SessionAlias          string         `json:"session_alias,omitempty"`
	WorkingDir            string         `json:"working_dir,omitempty"`
	Target                string         `json:"target,omitempty"`
	Mode                  string         `json:"mode,omitempty"`
	PlanningState         string         `json:"planning_state,omitempty"`
	PrePlanPermissionMode string         `json:"pre_plan_permission_mode,omitempty"`
	Profile               string         `json:"profile,omitempty"`
	RootRunID             string         `json:"root_run_id,omitempty"`
	ParentRunID           string         `json:"parent_run_id,omitempty"`
	TaskID                string         `json:"task_id,omitempty"`
	Role                  string         `json:"role,omitempty"`
	Scope                 map[string]any `json:"scope,omitempty"`
	Input                 string         `json:"input,omitempty"`
	Args                  map[string]any `json:"args,omitempty"`
}

type RuntimeCommand struct {
	ID             string         `json:"id,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Scope          ExecutionScope `json:"scope,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	RequestedBy    string         `json:"requested_by,omitempty"`
	CreatedAt      time.Time      `json:"created_at,omitempty"`
}

type RuntimeStateProjection struct {
	RunID           string           `json:"run_id"`
	SessionID       string           `json:"session_id,omitempty"`
	Status          string           `json:"status"`
	Resolution      string           `json:"resolution,omitempty"`
	LastCheckpoint  string           `json:"last_checkpoint,omitempty"`
	PermissionMode  string           `json:"permission_mode,omitempty"`
	PlanningState   string           `json:"planning_state,omitempty"`
	Output          string           `json:"output,omitempty"`
	ApprovalRequest *ApprovalRequest `json:"approval_request,omitempty"`
	PlanArtifact    *PlanArtifact    `json:"plan_artifact,omitempty"`
	Problem         *RuntimeError    `json:"problem,omitempty"`
}

type RuntimeError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	Provider     string `json:"provider,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	ProviderCode string `json:"provider_code,omitempty"`
	Retryable    bool   `json:"retryable,omitempty"`
	RateLimited  bool   `json:"rate_limited,omitempty"`
	SafeForModel bool   `json:"safe_for_model,omitempty"`
	BodyPreview  string `json:"body_preview,omitempty"`
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type RuntimeEventDraft struct {
	EventID    string         `json:"event_id,omitempty"`
	RunID      string         `json:"run_id"`
	Kind       EventKind      `json:"kind"`
	Sequence   int64          `json:"sequence,omitempty"`
	OccurredAt time.Time      `json:"occurred_at,omitempty"`
	Message    string         `json:"message,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
}

type RuntimeEventEnvelope struct {
	SchemaVersion string         `json:"schema_version"`
	EventID       string         `json:"event_id"`
	RunID         string         `json:"run_id"`
	Kind          EventKind      `json:"kind"`
	Sequence      int64          `json:"sequence"`
	OccurredAt    string         `json:"occurred_at,omitempty"`
	Message       string         `json:"message,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type EventQuery struct {
	RunID     string `json:"run_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	AfterSeq  int64  `json:"after_seq,omitempty"`
}

type InspectQuery struct {
	RunID     string `json:"run_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	AfterSeq  int64  `json:"after_seq,omitempty"`
}

type RuntimeInspectSnapshot struct {
	SchemaVersion   string                 `json:"schema_version"`
	SessionID       string                 `json:"session_id,omitempty"`
	RunID           string                 `json:"run_id,omitempty"`
	Status          string                 `json:"status"`
	Resolution      string                 `json:"resolution,omitempty"`
	Active          bool                   `json:"active"`
	Blocked         bool                   `json:"blocked,omitempty"`
	LastSeq         int64                  `json:"last_seq"`
	EventsAfterSeq  []RuntimeEventEnvelope `json:"events_after_seq,omitempty"`
	Events          []RuntimeEventEnvelope `json:"events,omitempty"`
	PermissionMode  string                 `json:"permission_mode,omitempty"`
	PlanningState   string                 `json:"planning_state,omitempty"`
	Profile         string                 `json:"profile,omitempty"`
	Workspace       string                 `json:"workspace,omitempty"`
	ActorID         string                 `json:"actor_id,omitempty"`
	OwnerID         string                 `json:"owner_id,omitempty"`
	WorldState      *WorldStateSnapshot    `json:"world_state,omitempty"`
	Permission      *PermissionSnapshot    `json:"permission,omitempty"`
	Warnings        []string               `json:"warnings,omitempty"`
	RuntimeContract string                 `json:"runtime_contract,omitempty"`
}

type InterruptRunResult struct {
	SchemaVersion   string `json:"schema_version"`
	Status          string `json:"status"`
	RunID           string `json:"run_id"`
	SessionID       string `json:"session_id,omitempty"`
	CancellationBy  string `json:"cancellation_by"`
	RuntimeContract string `json:"runtime_contract"`
}
