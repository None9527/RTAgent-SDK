package events

import (
	"context"
	"time"
)

// Kind is the canonical event kind type used throughout the SDK. The public
// package re-exports it as EventKind via a type alias so that host code and
// SDK internals share a single definition. See pkg/rtagent/types_constants.go.
type Kind string

const (
	KindRunCreated           Kind = "run.created"
	KindAgentStarted         Kind = "agent.started"
	KindAgentPlanProposed    Kind = "agent.plan.proposed"
	KindPermissionRequested  Kind = "permission.requested"
	KindPermissionGranted    Kind = "permission.granted"
	KindPermissionDenied     Kind = "permission.denied"
	KindToolInvoked          Kind = "tool.invoked"
	KindToolSucceeded        Kind = "tool.succeeded"
	KindToolFailed           Kind = "tool.failed"
	KindActivityStarted      Kind = "activity.started"
	KindActivityCompleted    Kind = "activity.completed"
	KindSessionStarted       Kind = "session.started"
	KindSessionEnded         Kind = "session.ended"
	KindTurnStarted          Kind = "turn.started"
	KindTurnCompleted        Kind = "turn.completed"
	KindTurnFailed           Kind = "turn.failed"
	KindTurnCancelled        Kind = "turn.cancelled"
	KindRunInterrupted       Kind = "run.interrupted"
	KindRunHeartbeat         Kind = "run.heartbeat"
	KindContextPacketCreated Kind = "context.packet.created"
	KindContextCompacted     Kind = "context.compacted"
	KindModelRequested       Kind = "model.requested"
	KindModelResponded       Kind = "model.responded"
	KindModelDelta           Kind = "model.delta"
	KindCheckpointCreated    Kind = "checkpoint.created"
)

type Event struct {
	ID         string                 `json:"event_id"`
	RunID      string                 `json:"run_id"`
	Kind       Kind                   `json:"kind"`
	Sequence   int64                  `json:"sequence"`
	OccurredAt time.Time              `json:"occurred_at"`
	Message    string                 `json:"message"`
	Payload    map[string]interface{} `json:"payload"`
	Causality  []string               `json:"causality_refs,omitempty"`
}

type Sink interface {
	Publish(context.Context, Event) error
}

func (k Kind) Valid() bool {
	switch k {
	case KindRunCreated,
		KindAgentStarted,
		KindAgentPlanProposed,
		KindPermissionRequested,
		KindPermissionGranted,
		KindPermissionDenied,
		KindToolInvoked,
		KindToolSucceeded,
		KindToolFailed,
		KindActivityStarted,
		KindActivityCompleted,
		KindSessionStarted,
		KindSessionEnded,
		KindTurnStarted,
		KindTurnCompleted,
		KindTurnFailed,
		KindTurnCancelled,
		KindRunInterrupted,
		KindRunHeartbeat,
		KindContextPacketCreated,
		KindContextCompacted,
		KindModelRequested,
		KindModelResponded,
		KindModelDelta,
		KindCheckpointCreated:
		return true
	default:
		return false
	}
}
