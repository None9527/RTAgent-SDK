package persistence

import (
	"context"
)

type SourceKind string

const (
	SourceUnknown    SourceKind = ""
	SourceRun        SourceKind = "run"
	SourceCheckpoint SourceKind = "checkpoint"
	SourceMemory     SourceKind = "memory"
	SourceArtifact   SourceKind = "artifact"
)

type SourceRef struct {
	Kind         SourceKind `json:"kind,omitempty"`
	RunID        string     `json:"run_id,omitempty"`
	CheckpointID string     `json:"checkpoint_id,omitempty"`
	RecordID     string     `json:"record_id,omitempty"`
}

type RunRecord struct {
	RunID          string `json:"run_id,omitempty"`
	ResumeID       string `json:"resume_id,omitempty"`
	UserObjective  string `json:"user_objective,omitempty"`
	IngressKind    string `json:"ingress_kind,omitempty"`
	Title          string `json:"title,omitempty"`
	Status         string `json:"status,omitempty"`
	Resolution     string `json:"resolution,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
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

type MessageRecord struct {
	MessageID   string `json:"message_id,omitempty"`
	ResumeID    string `json:"resume_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Role        string `json:"role,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Sequence    int64  `json:"sequence,omitempty"`
	Content     string `json:"content,omitempty"`
	PayloadJSON []byte `json:"payload_json,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
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

type EvidenceRecord struct {
	RecordID     string    `json:"record_id,omitempty"`
	Source       SourceRef `json:"source,omitempty"`
	Kind         string    `json:"kind,omitempty"`
	Strength     string    `json:"strength,omitempty"`
	ActionResult string    `json:"action_result,omitempty"`
	Observation  string    `json:"observation,omitempty"`
	ArtifactID   string    `json:"artifact_id,omitempty"`
	CreatedAt    string    `json:"created_at,omitempty"`
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

type MemoryStage string

const (
	MemoryStageUnknown    MemoryStage = ""
	MemoryStageProposed   MemoryStage = "proposed"
	MemoryStageCommitted  MemoryStage = "committed"
	MemoryStageRejected   MemoryStage = "rejected"
	MemoryStageSuperseded MemoryStage = "superseded"
)

type MemoryKind string

const (
	MemoryKindUnknown           MemoryKind = ""
	MemoryKindUserCorrection    MemoryKind = "user_correction"
	MemoryKindStablePreference  MemoryKind = "stable_preference"
	MemoryKindHardConstraint    MemoryKind = "hard_constraint"
	MemoryKindSuccessfulPattern MemoryKind = "successful_pattern"
	MemoryKindValidatedFact     MemoryKind = "validated_fact"
	MemoryKindWorkflowLearning  MemoryKind = "workflow_learning"
)

type MemoryOrigin string

const (
	MemoryOriginUnknown               MemoryOrigin = ""
	MemoryOriginExplicitUserStatement MemoryOrigin = "explicit_user_statement"
	MemoryOriginUserCorrection        MemoryOrigin = "user_correction"
	MemoryOriginModelInference        MemoryOrigin = "model_inference"
	MemoryOriginSystemImport          MemoryOrigin = "system_import"
	MemoryOriginValidatedRuntimeFact  MemoryOrigin = "validated_runtime_fact"
)

type MemoryRecord struct {
	RecordID     string       `json:"record_id,omitempty"`
	Stage        MemoryStage  `json:"stage,omitempty"`
	Kind         MemoryKind   `json:"kind,omitempty"`
	Origin       MemoryOrigin `json:"origin,omitempty"`
	Scope        string       `json:"scope,omitempty"`
	Topic        string       `json:"topic,omitempty"`
	Content      string       `json:"content,omitempty"`
	Confidence   float64      `json:"confidence,omitempty"`
	FreshnessTTL string       `json:"freshness_ttl,omitempty"`
	Invalidated  bool         `json:"invalidated,omitempty"`
	SupersedesID string       `json:"supersedes_id,omitempty"`
	Source       SourceRef    `json:"source,omitempty"`
	CitationIDs  []string     `json:"citation_ids,omitempty"`
	CreatedAt    string       `json:"created_at,omitempty"`
}

type DatasetExportRecord struct {
	ExportID      string   `json:"export_id,omitempty"`
	GeneratedAt   string   `json:"generated_at,omitempty"`
	RunIDs        []string `json:"run_ids,omitempty"`
	PositiveCount int      `json:"positive_count,omitempty"`
	NegativeCount int      `json:"negative_count,omitempty"`
	BlockedCount  int      `json:"blocked_count,omitempty"`
	ManifestPath  string   `json:"manifest_path,omitempty"`
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

type TaskRecord struct {
	TaskID       string   `json:"task_id,omitempty"`
	Objective    string   `json:"objective,omitempty"`
	Status       string   `json:"status,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	ParentID     string   `json:"parent_id,omitempty"`
	RunID        string   `json:"run_id,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
	CompletedAt  string   `json:"completed_at,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type CapabilityRecord struct {
	CapabilityID string `json:"capability_id,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Authority    string `json:"authority,omitempty"`
	Policy       string `json:"policy,omitempty"`
}

type PermissionRecord struct {
	PermissionID   string `json:"permission_id,omitempty"`
	Subject        string `json:"subject,omitempty"`
	Scope          string `json:"scope,omitempty"`
	Granted        bool   `json:"granted,omitempty"`
	AuthorizedBy   string `json:"authorized_by,omitempty"`
	RequestedAt    string `json:"requested_at,omitempty"`
	ResolvedAt     string `json:"resolved_at,omitempty"`
	PolicyWarnings string `json:"policy_warnings,omitempty"`
}

type GrantRecord struct {
	GrantID      string `json:"grant_id,omitempty"`
	CapabilityID string `json:"capability_id,omitempty"`
	Grantee      string `json:"grantee,omitempty"`
	GrantedBy    string `json:"granted_by,omitempty"`
	GrantedAt    string `json:"granted_at,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

type LeaseRecord struct {
	LeaseID          string `json:"lease_id,omitempty"`
	Resource         string `json:"resource,omitempty"`
	HolderActivityID string `json:"holder_activity_id,omitempty"`
	AcquiredAt       string `json:"acquired_at,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	ReleasedAt       string `json:"released_at,omitempty"`
}

type AuditRecord struct {
	AuditID       string   `json:"audit_id,omitempty"`
	Actor         string   `json:"actor,omitempty"`
	Action        string   `json:"action,omitempty"`
	Subject       string   `json:"subject,omitempty"`
	Result        string   `json:"result,omitempty"`
	Timestamp     string   `json:"timestamp,omitempty"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
	PolicyApplied string   `json:"policy_applied,omitempty"`
}

type WorldStateEntry struct {
	ID           string   `json:"id,omitempty"`
	Partition    string   `json:"partition,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	StateJSON    string   `json:"state_json,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	SourceID     string   `json:"source_id,omitempty"`
	SourceSeq    int64    `json:"source_seq,omitempty"`
	Confidence   float64  `json:"confidence,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	Version      int64    `json:"version,omitempty"`
}

type RunStore interface {
	PutRun(ctx context.Context, rec RunRecord) error
	GetRun(ctx context.Context, runID string) (RunRecord, error)
	DeleteRun(ctx context.Context, runID string) error
}

type ThreadStore interface {
	PutThread(ctx context.Context, rec ThreadRecord) error
	GetThread(ctx context.Context, resumeID string) (ThreadRecord, error)
	DeleteThread(ctx context.Context, resumeID string) error
}

type MessageStore interface {
	AppendMessage(ctx context.Context, rec MessageRecord) error
}

type CheckpointStore interface {
	AppendCheckpoint(ctx context.Context, rec CheckpointRecord) error
	GetCheckpoint(ctx context.Context, runID, checkpointID string) (CheckpointRecord, error)
}

type EvidenceStore interface {
	AppendEvidence(ctx context.Context, rec EvidenceRecord) error
	ListEvidence(ctx context.Context, runID string) ([]EvidenceRecord, error)
}

type RuntimeEventStore interface {
	AppendRuntimeEvent(ctx context.Context, rec RuntimeEventRecord) error
	ListRuntimeEvents(ctx context.Context, runID string) ([]RuntimeEventRecord, error)
}

type MemoryStore interface {
	PutMemory(ctx context.Context, rec MemoryRecord) error
	GetMemory(ctx context.Context, recordID string) (MemoryRecord, error)
}

type ArtifactStore interface {
	PutArtifact(ctx context.Context, rec ArtifactRecord) error
	GetArtifact(ctx context.Context, artifactID string) (ArtifactRecord, error)
	GetLatestArtifactByPath(ctx context.Context, path string) (ArtifactRecord, error)
}

type DatasetStore interface {
	PutDatasetExport(ctx context.Context, rec DatasetExportRecord) error
	GetDatasetExport(ctx context.Context, exportID string) (DatasetExportRecord, error)
}

type ActivityStore interface {
	PutActivity(ctx context.Context, rec ActivityRecord) error
	GetActivity(ctx context.Context, activityID string) (ActivityRecord, error)
	ListActivitiesByRunID(ctx context.Context, runID string) ([]ActivityRecord, error)
}

type TaskStore interface {
	PutTask(ctx context.Context, rec TaskRecord) error
	GetTask(ctx context.Context, taskID string) (TaskRecord, error)
	ListTasksByRunID(ctx context.Context, runID string) ([]TaskRecord, error)
}

type CapabilityStore interface {
	PutCapability(ctx context.Context, rec CapabilityRecord) error
	GetCapability(ctx context.Context, capabilityID string) (CapabilityRecord, error)
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

type AuditStore interface {
	PutAuditLog(ctx context.Context, rec AuditRecord) error
}

type Bundle interface {
	RunStore
	ThreadStore
	MessageStore
	CheckpointStore
	EvidenceStore
	RuntimeEventStore
	MemoryStore
	ArtifactStore
	DatasetStore
	ActivityStore
	TaskStore
	CapabilityStore
	PermissionStore
	GrantStore
	LeaseStore
	AuditStore
}
