package events

import (
	"context"
	"time"
)

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
	KindFileModified         Kind = "file.modified"
	KindTaskBlocked          Kind = "task.blocked"
	KindTaskResumed          Kind = "task.resumed"
	KindSessionStarted       Kind = "session.started"
	KindSessionEnded         Kind = "session.ended"
	KindContextPacketCreated Kind = "context.packet.created"
	KindModelRequested       Kind = "model.requested"
	KindModelResponded       Kind = "model.responded"
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
		KindFileModified,
		KindTaskBlocked,
		KindTaskResumed,
		KindSessionStarted,
		KindSessionEnded,
		KindContextPacketCreated,
		KindModelRequested,
		KindModelResponded:
		return true
	default:
		return false
	}
}
