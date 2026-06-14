package rtagent

type SessionQuery struct {
	SessionID string `json:"session_id"`
}

type SessionRunSummary struct {
	RunID          string `json:"run_id"`
	SessionID      string `json:"session_id,omitempty"`
	RootRunID      string `json:"root_run_id,omitempty"`
	ParentRunID    string `json:"parent_run_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	Status         string `json:"status"`
	Resolution     string `json:"resolution,omitempty"`
	Title          string `json:"title,omitempty"`
	Objective      string `json:"objective,omitempty"`
	IngressKind    string `json:"ingress_kind,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
	LastCheckpoint string `json:"last_checkpoint,omitempty"`
}

type SessionSnapshot struct {
	SchemaVersion       string              `json:"schema_version"`
	SessionID           string              `json:"session_id"`
	Status              string              `json:"status"`
	Active              bool                `json:"active"`
	CanResume           bool                `json:"can_resume"`
	LatestRunID         string              `json:"latest_run_id,omitempty"`
	LatestCheckpointID  string              `json:"latest_checkpoint_id,omitempty"`
	LatestMessageAt     string              `json:"latest_message_at,omitempty"`
	CreatedAt           string              `json:"created_at,omitempty"`
	UpdatedAt           string              `json:"updated_at,omitempty"`
	RunCount            int                 `json:"run_count"`
	ActiveRunIDs        []string            `json:"active_run_ids,omitempty"`
	Runs                []SessionRunSummary `json:"runs,omitempty"`
	RuntimeContract     string              `json:"runtime_contract"`
	ResumeCommandHint   string              `json:"resume_command_hint,omitempty"`
	ExternalResumeReady bool                `json:"external_resume_ready"`
	Warnings            []string            `json:"warnings,omitempty"`
}

type SessionGraphQuery struct {
	SessionID string `json:"session_id"`
	RootRunID string `json:"root_run_id,omitempty"`
}

type SessionRunNode struct {
	RunID          string `json:"run_id"`
	SessionID      string `json:"session_id,omitempty"`
	RootRunID      string `json:"root_run_id,omitempty"`
	ParentRunID    string `json:"parent_run_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	Status         string `json:"status"`
	Resolution     string `json:"resolution,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
	LastCheckpoint string `json:"last_checkpoint,omitempty"`
}

type SessionRunEdge struct {
	FromRunID string `json:"from_run_id"`
	ToRunID   string `json:"to_run_id"`
	Kind      string `json:"kind"`
}

type SessionGraphSnapshot struct {
	SchemaVersion   string           `json:"schema_version"`
	SessionID       string           `json:"session_id"`
	Status          string           `json:"status"`
	LatestRunID     string           `json:"latest_run_id,omitempty"`
	RootRunID       string           `json:"root_run_id,omitempty"`
	Nodes           []SessionRunNode `json:"nodes,omitempty"`
	Edges           []SessionRunEdge `json:"edges,omitempty"`
	ActiveRunIDs    []string         `json:"active_run_ids,omitempty"`
	RuntimeContract string           `json:"runtime_contract"`
}

type ResumeRunRequest struct {
	RunID        string         `json:"run_id,omitempty"`
	CheckpointID string         `json:"checkpoint_id,omitempty"`
	ApprovalID   string         `json:"approval_id,omitempty"`
	Decision     string         `json:"decision,omitempty"`
	Scope        ExecutionScope `json:"scope,omitempty"`
}

type CheckpointGraphQuery struct {
	RunID string `json:"run_id"`
}

type CheckpointNode struct {
	CheckpointID string         `json:"checkpoint_id"`
	GraphID      string         `json:"graph_id,omitempty"`
	Node         string         `json:"node"`
	Route        string         `json:"route,omitempty"`
	NextNode     string         `json:"next_node,omitempty"`
	Status       string         `json:"status,omitempty"`
	Source       string         `json:"source,omitempty"`
	CreatedAt    string         `json:"created_at,omitempty"`
	ResumeReady  bool           `json:"resume_ready,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type CheckpointEdge struct {
	FromCheckpointID string `json:"from_checkpoint_id"`
	ToCheckpointID   string `json:"to_checkpoint_id"`
	Kind             string `json:"kind"`
}

type CheckpointGraphSnapshot struct {
	SchemaVersion      string           `json:"schema_version"`
	RunID              string           `json:"run_id"`
	SessionID          string           `json:"session_id,omitempty"`
	LatestCheckpointID string           `json:"latest_checkpoint_id,omitempty"`
	Nodes              []CheckpointNode `json:"nodes,omitempty"`
	Edges              []CheckpointEdge `json:"edges,omitempty"`
	RuntimeContract    string           `json:"runtime_contract"`
	Warnings           []string         `json:"warnings,omitempty"`
}

type StopSessionRequest struct {
	SessionID   string `json:"session_id"`
	Mode        string `json:"mode,omitempty"`
	Reason      string `json:"reason,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
}

type StopSessionResult struct {
	SchemaVersion     string   `json:"schema_version"`
	SessionID         string   `json:"session_id"`
	Status            string   `json:"status"`
	Mode              string   `json:"mode"`
	AlreadyStopped    bool     `json:"already_stopped,omitempty"`
	InterruptedRunIDs []string `json:"interrupted_run_ids,omitempty"`
	RuntimeContract   string   `json:"runtime_contract"`
}
