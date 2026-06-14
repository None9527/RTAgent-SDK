package rtagent

type ToolSpec struct {
	Name                 string                  `json:"name"`
	Description          string                  `json:"description,omitempty"`
	Parameters           map[string]any          `json:"parameters,omitempty"`
	FreeformGrammar      string                  `json:"freeform_grammar,omitempty"`
	Strict               bool                    `json:"strict,omitempty"`
	Examples             []map[string]any        `json:"examples,omitempty"`
	Namespace            string                  `json:"namespace,omitempty"`
	ProviderName         string                  `json:"provider_name,omitempty"`
	Deferred             bool                    `json:"deferred,omitempty"`
	AlwaysVisible        bool                    `json:"always_visible,omitempty"`
	SearchHint           string                  `json:"search_hint,omitempty"`
	ReadOnly             bool                    `json:"read_only,omitempty"`
	SideEffectKind       string                  `json:"side_effect_kind,omitempty"`
	RiskLevel            string                  `json:"risk_level,omitempty"`
	ConcurrencySafe      bool                    `json:"concurrency_safe,omitempty"`
	InterruptBehavior    string                  `json:"interrupt_behavior,omitempty"`
	ResourceLocks        []ResourceLock          `json:"resource_locks,omitempty"`
	OutputPolicy         ToolOutputPolicy        `json:"output_policy,omitempty"`
	OutputSchema         map[string]any          `json:"output_schema,omitempty"`
	ExecutionConstraints ExecutionConstraints    `json:"execution_constraints,omitempty"`
	RequiredGrants       []ScopedPermissionGrant `json:"required_grants,omitempty"`
	Version              string                  `json:"version,omitempty"`
	Epoch                string                  `json:"epoch,omitempty"`
	SchemaHash           string                  `json:"schema_hash,omitempty"`
}

type ResourceLock struct {
	Kind string `json:"kind"`
	Key  string `json:"key,omitempty"`
	Mode string `json:"mode"`
}

type ToolOutputPolicy struct {
	MaxModelBytes int    `json:"max_model_bytes,omitempty"`
	MaxUserBytes  int    `json:"max_user_bytes,omitempty"`
	Spill         bool   `json:"spill,omitempty"`
	SummaryMode   string `json:"summary_mode,omitempty"`
}

type ExecutionConstraints struct {
	ProfileID string   `json:"profile_id,omitempty"`
	FileScope []string `json:"file_scope,omitempty"`
	Network   string   `json:"network,omitempty"`
}

type ToolCall struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Arguments   map[string]any `json:"arguments,omitempty"`
	ReadOnly    bool           `json:"read_only,omitempty"`
	SchemaHash  string         `json:"schema_hash,omitempty"`
	Epoch       string         `json:"epoch,omitempty"`
	EpochClosed bool           `json:"epoch_closed,omitempty"`
}

type ToolObservation struct {
	ToolCallID          string        `json:"tool_call_id"`
	Name                string        `json:"name"`
	Status              string        `json:"status"`
	ModelVisibleSummary string        `json:"model_visible_summary"`
	UserVisibleSummary  string        `json:"user_visible_summary,omitempty"`
	OutputRef           string        `json:"output_ref,omitempty"`
	EvidenceRefs        []EvidenceRef `json:"evidence_refs,omitempty"`
}

type WriteFileRequest struct {
	RelativePath     string         `json:"relative_path"`
	Content          []byte         `json:"content,omitempty"`
	ActiveActivityID string         `json:"active_activity_id,omitempty"`
	RunID            string         `json:"run_id,omitempty"`
	Scope            ExecutionScope `json:"scope,omitempty"`
}

type ArtifactRecord struct {
	ArtifactID string `json:"artifact_id,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	ByteSize   int    `json:"byte_size,omitempty"`
	Preview    string `json:"preview,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}
