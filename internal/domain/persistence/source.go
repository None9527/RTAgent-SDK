package persistence

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
