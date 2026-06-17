package adapters

import (
	"time"
)

type RunModel struct {
	RunID          string    `gorm:"primaryKey;size:255"`
	ResumeID       string    `gorm:"column:resume_id;size:255;index"`
	RootRunID      string    `gorm:"size:255;index"`
	ParentRunID    string    `gorm:"size:255;index"`
	TaskID         string    `gorm:"size:255;index"`
	UserObjective  string    `gorm:"type:text"`
	IngressKind    string    `gorm:"size:64"`
	Title          string    `gorm:"size:512"`
	Status         string    `gorm:"size:64;index"`
	Resolution     string    `gorm:"size:64;index"`
	CreatedAt      time.Time `gorm:"index"`
	UpdatedAt      time.Time `gorm:"index"`
	CompletedAt    *time.Time
	LastCheckpoint string `gorm:"size:255"`
}

func (RunModel) TableName() string { return "v2_runs" }

type ThreadModel struct {
	ResumeID           string    `gorm:"primaryKey;column:resume_id;size:255"`
	Title              string    `gorm:"size:512"`
	Status             string    `gorm:"size:64;index"`
	LatestRunID        string    `gorm:"size:255"`
	LatestCheckpointID string    `gorm:"size:255"`
	LatestMessageAt    time.Time `gorm:"index"`
	CreatedAt          time.Time `gorm:"index"`
	UpdatedAt          time.Time
}

func (ThreadModel) TableName() string { return "v2_threads" }

type CheckpointModel struct {
	RunID        string    `gorm:"primaryKey;size:255"`
	CheckpointID string    `gorm:"primaryKey;size:255"`
	GraphID      string    `gorm:"size:255"`
	Node         string    `gorm:"size:128"`
	Route        string    `gorm:"size:128"`
	NextNode     string    `gorm:"size:128"`
	Status       string    `gorm:"size:64"`
	RouteTrace   string    `gorm:"type:text"`
	StatePayload []byte    `gorm:"type:blob"`
	CreatedAt    time.Time `gorm:"index"`
	Source       string    `gorm:"size:128"`
}

func (CheckpointModel) TableName() string { return "v2_checkpoints" }

type ActivityModel struct {
	ActivityID       string    `gorm:"primaryKey;size:255"`
	Kind             string    `gorm:"size:64;index"`
	Status           string    `gorm:"size:64;index"`
	Owner            string    `gorm:"size:255"`
	ParentActivityID string    `gorm:"size:255"`
	RunID            string    `gorm:"size:255;index"`
	StartedAt        time.Time `gorm:"index"`
	UpdatedAt        time.Time
	CompletedAt      *time.Time
	InputRefsJSON    string `gorm:"type:text"`
	OutputRefsJSON   string `gorm:"type:text"`
	EvidenceRefsJSON string `gorm:"type:text"`
	ErrorText        string `gorm:"type:text"`
	Authority        string `gorm:"size:255"`
}

func (ActivityModel) TableName() string { return "v2_activities" }

type EventModel struct {
	EventID          string    `gorm:"primaryKey;size:255"`
	RunID            string    `gorm:"size:255;index;uniqueIndex:idx_runtime_events_run_sequence"`
	Kind             string    `gorm:"size:64;index"`
	Sequence         int64     `gorm:"index;uniqueIndex:idx_runtime_events_run_sequence"`
	OccurredAt       time.Time `gorm:"index"`
	Message          string    `gorm:"type:text"`
	PayloadJSON      []byte    `gorm:"type:blob"`
	EvidenceRefsJSON string    `gorm:"type:text"`
	CausalityRefs    string    `gorm:"type:text"`
	Authority        string    `gorm:"size:255"`
}

func (EventModel) TableName() string { return "v2_runtime_events" }

type PermissionModel struct {
	PermissionID   string     `gorm:"primaryKey;size:255"`
	RunID          string     `gorm:"size:255;index"`
	Subject        string     `gorm:"size:255"`
	Scope          string     `gorm:"type:text"`
	Granted        bool       `gorm:"index"`
	AuthorizedBy   string     `gorm:"size:255"`
	RequestedAt    time.Time  `gorm:"index"`
	ResolvedAt     *time.Time `gorm:"index"`
	PolicyWarnings string     `gorm:"type:text"`
}

func (PermissionModel) TableName() string { return "v2_permissions" }

type GrantModel struct {
	GrantID      string    `gorm:"primaryKey;size:255"`
	CapabilityID string    `gorm:"size:255;index"`
	Grantee      string    `gorm:"size:255;index"`
	GrantedBy    string    `gorm:"size:255"`
	GrantedAt    time.Time `gorm:"index"`
	ExpiresAt    time.Time `gorm:"index"`
}

func (GrantModel) TableName() string { return "v2_grants" }

type LeaseModel struct {
	LeaseID          string     `gorm:"primaryKey;size:255"`
	Resource         string     `gorm:"size:512;index"`
	HolderActivityID string     `gorm:"size:255;index"`
	AcquiredAt       time.Time  `gorm:"index"`
	ExpiresAt        time.Time  `gorm:"index"`
	ReleasedAt       *time.Time `gorm:"index"`
}

func (LeaseModel) TableName() string { return "v2_leases" }

type CapabilityModel struct {
	CapabilityID string    `gorm:"primaryKey;size:255"`
	Family       string    `gorm:"size:64;index"`
	TargetScope  string    `gorm:"type:text"`
	PolicyRule   string    `gorm:"type:text"`
	Authority    string    `gorm:"size:255"`
	CreatedAt    time.Time `gorm:"index"`
}

func (CapabilityModel) TableName() string { return "v2_capabilities" }

type ToolSchemaSnapshotModel struct {
	SnapshotID      string `gorm:"primaryKey;size:255"`
	RunID           string `gorm:"size:255;index"`
	ContextPacketID string `gorm:"size:255;index"`
	SchemaHash      string `gorm:"size:128;index"`
	ToolCount       int
	SnapshotJSON    string    `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"index"`
}

func (ToolSchemaSnapshotModel) TableName() string { return "v2_tool_schema_snapshots" }

type MemoryModel struct {
	RecordID         string    `gorm:"primaryKey;size:255"`
	Stage            string    `gorm:"size:64;index"`
	Kind             string    `gorm:"size:64;index"`
	Origin           string    `gorm:"size:64;index"`
	Scope            string    `gorm:"size:255;index"`
	Topic            string    `gorm:"size:255;index"`
	Content          string    `gorm:"type:text"`
	Confidence       float64   `gorm:"index"`
	FreshnessTTL     string    `gorm:"size:64"`
	Invalidated      bool      `gorm:"index"`
	SupersedesID     string    `gorm:"size:255;index"`
	SourceKind       string    `gorm:"size:64"`
	SourceRunID      string    `gorm:"size:255;index"`
	SourceCheckpoint string    `gorm:"size:255"`
	SourceRecordID   string    `gorm:"size:255"`
	CitationIDsJSON  string    `gorm:"type:text"`
	CreatedAt        time.Time `gorm:"index"`
}

func (MemoryModel) TableName() string { return "v2_memory_records" }

type ArtifactModel struct {
	ArtifactID       string `gorm:"primaryKey;size:255"`
	RunID            string `gorm:"size:255;index"`
	ActivityID       string `gorm:"size:255;index"`
	Kind             string `gorm:"size:64;index"`
	FilePath         string `gorm:"type:text"`
	SHA256           string `gorm:"size:64"`
	ByteSize         int64
	PreviewText      string    `gorm:"type:text"`
	SourceKind       string    `gorm:"size:64"`
	SourceRunID      string    `gorm:"size:255"`
	SourceCheckpoint string    `gorm:"size:255"`
	SourceRecordID   string    `gorm:"size:255"`
	CreatedAt        time.Time `gorm:"index"`
}

func (ArtifactModel) TableName() string { return "v2_artifact_records" }

type ContextHandleModel struct {
	HandleID              string `gorm:"primaryKey;size:255"`
	RunID                 string `gorm:"size:255;index"`
	Kind                  string `gorm:"size:64"`
	Title                 string `gorm:"size:512"`
	Summary               string `gorm:"type:text"`
	SourceRef             string `gorm:"size:512"`
	TokenEstimate         int
	Freshness             float64
	MaterializationPolicy string `gorm:"size:64"`
	EvidenceRefsJSON      string `gorm:"type:text"`
}

func (ContextHandleModel) TableName() string { return "v2_context_handles" }
