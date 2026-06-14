package rtagent

type WorldStateQuery struct {
	RunID     string `json:"run_id,omitempty"`
	Partition string `json:"partition,omitempty"`
}

type WorldStateSnapshot struct {
	SchemaVersion   string                `json:"schema_version"`
	SnapshotID      string                `json:"snapshot_id"`
	BuildID         string                `json:"build_id,omitempty"`
	RuntimeEpoch    string                `json:"runtime_epoch,omitempty"`
	SessionID       string                `json:"session_id,omitempty"`
	RunID           string                `json:"run_id"`
	Version         int                   `json:"version"`
	GeneratedAt     string                `json:"generated_at"`
	BuiltAt         string                `json:"built_at,omitempty"`
	SourceWatermark string                `json:"source_watermark,omitempty"`
	Partitions      []WorldStatePartition `json:"partitions,omitempty"`
	Handles         []WorldStateHandle    `json:"handles,omitempty"`
	Warnings        []string              `json:"warnings,omitempty"`
	Entries         []WorldStateEntry     `json:"entries,omitempty"`
}

type WorldStatePartition struct {
	SchemaVersion   string             `json:"schema_version,omitempty"`
	SnapshotID      string             `json:"snapshot_id,omitempty"`
	Partition       string             `json:"partition"`
	Provider        string             `json:"provider,omitempty"`
	Source          string             `json:"source,omitempty"`
	SourceSeq       int64              `json:"source_seq,omitempty"`
	SourceWatermark string             `json:"source_watermark,omitempty"`
	BuiltAt         string             `json:"built_at,omitempty"`
	Entries         []WorldStateEntry  `json:"entries,omitempty"`
	Handles         []WorldStateHandle `json:"handles,omitempty"`
	Warnings        []string           `json:"warnings,omitempty"`
}

type WorldStateEntry struct {
	ID               string           `json:"id,omitempty"`
	Partition        string           `json:"partition,omitempty"`
	Kind             string           `json:"kind,omitempty"`
	Subject          string           `json:"subject,omitempty"`
	StateOrPredicate string           `json:"state_or_predicate,omitempty"`
	StateJSON        string           `json:"state_json,omitempty"`
	Summary          string           `json:"summary,omitempty"`
	Source           string           `json:"source,omitempty"`
	SourceID         string           `json:"source_id,omitempty"`
	SourceSeq        int64            `json:"source_seq,omitempty"`
	Authority        string           `json:"authority,omitempty"`
	Confidence       float64          `json:"confidence,omitempty"`
	ObservedAt       string           `json:"observed_at,omitempty"`
	EvidenceRefs     []string         `json:"evidence_refs,omitempty"`
	ExpiresAt        string           `json:"expires_at,omitempty"`
	Version          int64            `json:"version,omitempty"`
	Capability       *CapabilityState `json:"capability,omitempty"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

type CapabilityState struct {
	ID                  string                  `json:"id"`
	Kind                string                  `json:"kind,omitempty"`
	Namespace           string                  `json:"namespace,omitempty"`
	Version             string                  `json:"version,omitempty"`
	Status              string                  `json:"status,omitempty"`
	Visible             bool                    `json:"visible"`
	Available           bool                    `json:"available"`
	Authorized          bool                    `json:"authorized"`
	Source              string                  `json:"source,omitempty"`
	SourceID            string                  `json:"source_id,omitempty"`
	SourceSeq           int64                   `json:"source_seq,omitempty"`
	Authority           string                  `json:"authority,omitempty"`
	Summary             string                  `json:"summary,omitempty"`
	Risk                string                  `json:"risk,omitempty"`
	Boundary            string                  `json:"boundary,omitempty"`
	Permission          string                  `json:"permission,omitempty"`
	SchemaHash          string                  `json:"schema_hash,omitempty"`
	ProviderName        string                  `json:"provider_name,omitempty"`
	ReadOnly            bool                    `json:"read_only,omitempty"`
	ConcurrencySafe     bool                    `json:"concurrency_safe,omitempty"`
	ResourceLocks       []ResourceLock          `json:"resource_locks,omitempty"`
	Handle              string                  `json:"handle,omitempty"`
	AuthorizationReason string                  `json:"authorization_reason,omitempty"`
	MatchedGrantID      string                  `json:"matched_grant_id,omitempty"`
	MatchedGrantScope   string                  `json:"matched_grant_scope,omitempty"`
	PolicyHash          string                  `json:"policy_hash,omitempty"`
	RequiredGrants      []ScopedPermissionGrant `json:"required_grants,omitempty"`
}

type WorldStateHandle struct {
	Handle              string `json:"handle"`
	Kind                string `json:"kind,omitempty"`
	Partition           string `json:"partition,omitempty"`
	Summary             string `json:"summary,omitempty"`
	Source              string `json:"source,omitempty"`
	SourceID            string `json:"source_id,omitempty"`
	SourceSeq           int64  `json:"source_seq,omitempty"`
	SnapshotID          string `json:"snapshot_id,omitempty"`
	ContentHash         string `json:"content_hash,omitempty"`
	ValidWithinSnapshot bool   `json:"valid_within_snapshot"`
	ValidUntilSourceSeq int64  `json:"valid_until_source_seq,omitempty"`
	Reloadable          bool   `json:"reloadable"`
	RequiresPermission  bool   `json:"requires_permission"`
	Permission          string `json:"permission,omitempty"`
	RedactionLevel      string `json:"redaction_level,omitempty"`
	ExpiresAt           string `json:"expires_at,omitempty"`
}

type ContextHandle struct {
	HandleID              string   `json:"handle_id"`
	RunID                 string   `json:"run_id"`
	Kind                  string   `json:"kind"`
	Title                 string   `json:"title"`
	Summary               string   `json:"summary"`
	SourceRef             string   `json:"source_ref"`
	TokenEstimate         int      `json:"token_estimate"`
	Freshness             float64  `json:"freshness"`
	MaterializationPolicy string   `json:"materialization_policy"`
	EvidenceRefs          []string `json:"evidence_refs,omitempty"`
}

type MemoryFact struct {
	ID           string         `json:"id,omitempty"`
	Kind         string         `json:"kind,omitempty"`
	Subject      string         `json:"subject,omitempty"`
	State        string         `json:"state,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Content      string         `json:"content,omitempty"`
	Source       string         `json:"source,omitempty"`
	SourceID     string         `json:"source_id,omitempty"`
	ObservedAt   string         `json:"observed_at,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type HypothesisFact struct {
	ID           string         `json:"id,omitempty"`
	Kind         string         `json:"kind,omitempty"`
	Subject      string         `json:"subject,omitempty"`
	Predicate    string         `json:"predicate,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Source       string         `json:"source,omitempty"`
	SourceID     string         `json:"source_id,omitempty"`
	ObservedAt   string         `json:"observed_at,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type CapabilityInventoryItem struct {
	ID             string                  `json:"id"`
	Kind           string                  `json:"kind,omitempty"`
	Namespace      string                  `json:"namespace,omitempty"`
	Version        string                  `json:"version,omitempty"`
	Summary        string                  `json:"summary,omitempty"`
	ProviderName   string                  `json:"provider_name,omitempty"`
	Visible        bool                    `json:"visible"`
	Available      bool                    `json:"available"`
	ReadOnly       bool                    `json:"read_only,omitempty"`
	Risk           string                  `json:"risk,omitempty"`
	Permission     string                  `json:"permission,omitempty"`
	Boundary       string                  `json:"boundary,omitempty"`
	SchemaHash     string                  `json:"schema_hash,omitempty"`
	RequiredGrants []ScopedPermissionGrant `json:"required_grants,omitempty"`
	Metadata       map[string]any          `json:"metadata,omitempty"`
}

type WorldStateProviderInput struct {
	Query           WorldStateQuery        `json:"query"`
	Scope           ExecutionScope         `json:"scope"`
	Events          []RuntimeEventEnvelope `json:"events,omitempty"`
	SnapshotID      string                 `json:"snapshot_id,omitempty"`
	SourceWatermark string                 `json:"source_watermark,omitempty"`
	SourceSeq       int64                  `json:"source_seq,omitempty"`
	BuiltAt         string                 `json:"built_at,omitempty"`
	Permission      *PermissionSnapshot    `json:"permission,omitempty"`
}
