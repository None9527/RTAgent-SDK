package worldstate

type Partition string

const (
	PartitionActivity   Partition = "activity"
	PartitionTask       Partition = "task"
	PartitionGovernance Partition = "governance"
	PartitionArtifact   Partition = "artifact"
)

type Entry struct {
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
