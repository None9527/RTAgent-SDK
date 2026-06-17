package persistence

type RunRecord struct {
	RunID          string `json:"run_id,omitempty"`
	ResumeID       string `json:"resume_id,omitempty"`
	RootRunID      string `json:"root_run_id,omitempty"`
	ParentRunID    string `json:"parent_run_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	UserObjective  string `json:"user_objective,omitempty"`
	IngressKind    string `json:"ingress_kind,omitempty"`
	Title          string `json:"title,omitempty"`
	Status         string `json:"status,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
	LastCheckpoint string `json:"last_checkpoint,omitempty"`
}

type ThreadRecord struct {
	ResumeID           string `json:"resume_id,omitempty"`
	Title              string `json:"title,omitempty"`
	Status             string `json:"status,omitempty"`
	LatestRunID        string `json:"latest_run_id,omitempty"`
	LatestCheckpointID string `json:"latest_checkpoint_id,omitempty"`
	LatestMessageAt    string `json:"latest_message_at,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

type CheckpointRecord struct {
	RunID        string `json:"run_id,omitempty"`
	CheckpointID string `json:"checkpoint_id,omitempty"`
	GraphID      string `json:"graph_id,omitempty"`
	Node         string `json:"node,omitempty"`
	Route        string `json:"route,omitempty"`
	NextNode     string `json:"next_node,omitempty"`
	Status       string `json:"status,omitempty"`
	RouteTrace   string `json:"route_trace,omitempty"`
	StatePayload []byte `json:"state_payload,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	Source       string `json:"source,omitempty"`
}

type RuntimeEventRecord struct {
	EventID     string `json:"event_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Sequence    int64  `json:"sequence,omitempty"`
	OccurredAt  string `json:"occurred_at,omitempty"`
	Message     string `json:"message,omitempty"`
	PayloadJSON []byte `json:"payload_json,omitempty"`
}

type ArtifactRecord struct {
	ArtifactID string    `json:"artifact_id,omitempty"`
	Source     SourceRef `json:"source,omitempty"`
	Kind       string    `json:"kind,omitempty"`
	Path       string    `json:"path,omitempty"`
	SHA256     string    `json:"sha256,omitempty"`
	MediaType  string    `json:"media_type,omitempty"`
	ByteSize   int       `json:"byte_size,omitempty"`
	Preview    string    `json:"preview,omitempty"`
	CreatedAt  string    `json:"created_at,omitempty"`
}

type ActivityRecord struct {
	ActivityID       string   `json:"activity_id,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	Status           string   `json:"status,omitempty"`
	Owner            string   `json:"owner,omitempty"`
	ParentActivityID string   `json:"parent_activity_id,omitempty"`
	RunID            string   `json:"run_id,omitempty"`
	StartedAt        string   `json:"started_at,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
	CompletedAt      string   `json:"completed_at,omitempty"`
	InputRefs        []string `json:"input_refs,omitempty"`
	OutputRefs       []string `json:"output_refs,omitempty"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
	Error            string   `json:"error,omitempty"`
	Authority        string   `json:"authority,omitempty"`
}

type ContextHandleRecord struct {
	HandleID              string   `json:"handle_id,omitempty"`
	RunID                 string   `json:"run_id,omitempty"`
	Kind                  string   `json:"kind,omitempty"`
	Title                 string   `json:"title,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	SourceRef             string   `json:"source_ref,omitempty"`
	TokenEstimate         int      `json:"token_estimate,omitempty"`
	Freshness             float64  `json:"freshness,omitempty"`
	MaterializationPolicy string   `json:"materialization_policy,omitempty"`
	EvidenceRefs          []string `json:"evidence_refs,omitempty"`
}
