package persistence

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
