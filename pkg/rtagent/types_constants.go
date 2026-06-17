package rtagent

import "github.com/None9527/RTAgent-SDK/internal/runtime/events"

const (
	SchemaRuntimeEventEnvelopeV1 = "runtime_event_envelope.v1"
	SchemaRuntimeInspectV1       = "runtime_inspect.v1"
	SchemaSessionSnapshotV1      = "session_snapshot.v1"
	SchemaSessionGraphV1         = "session_graph.v1"
	SchemaCheckpointGraphV1      = "checkpoint_graph.v1"
	SchemaPermissionSnapshotV1   = "permission_snapshot.v1"
	SchemaWorldStateV1           = "world_state.v1"
)

const (
	WorldStatePartitionMemory     = "memory"
	WorldStatePartitionCapability = "capability"
	WorldStatePartitionActivity   = "activity"
	WorldStatePartitionTask       = "task"
	WorldStatePartitionContext    = "context"
	WorldStatePartitionGovernance = "governance"
	WorldStatePartitionHypothesis = "hypothesis"
)

const (
	CapabilityStateVisible    = "visible"
	CapabilityStateAvailable  = "available"
	CapabilityStateAuthorized = "authorized"
)

const (
	RuntimeStatusOK        = "ok"
	RuntimeStatusQueued    = "queued"
	RuntimeStatusRunning   = "running"
	RuntimeStatusBlocked   = "blocked"
	RuntimeStatusSuspended = "suspended"
	RuntimeStatusDenied    = "denied"
	RuntimeStatusCompleted = "completed"
	RuntimeStatusFailed    = "failed"
	RuntimeStatusCanceled  = "canceled"
)

const (
	SessionStatusActive   = "active"
	SessionStatusStopping = "stopping"
	SessionStatusStopped  = "stopped"

	StopSessionModeDrain        = "drain"
	StopSessionModeCancelActive = "cancel_active"
)

const (
	PermissionDefault     = "default"
	PermissionAcceptEdits = "acceptEdits"
	PermissionYolo        = "yolo"

	PlanningOff  = "off"
	PlanningPlan = "plan"
)

const (
	PermissionStatusAllowed          = "allowed"
	PermissionStatusDenied           = "denied"
	PermissionStatusRequiresApproval = "requires_approval"

	PermissionDecisionDeny               = "deny"
	PermissionDecisionAllowOnce          = "allow_once"
	PermissionDecisionAllowForRun        = "allow_for_run"
	PermissionDecisionAllowForSession    = "allow_for_session"
	PermissionDecisionAllowAllForRun     = "allow_all_for_run"
	PermissionDecisionAllowAllForSession = "allow_all_for_session"

	PermissionGrantScopeAction  = "action"
	PermissionGrantScopeRun     = "run"
	PermissionGrantScopeSession = "session"

	PermissionCapabilityAny            = "*"
	PermissionCapabilityToolCall       = "tool.call"
	PermissionCapabilityWorkspaceWrite = "workspace.write"
	PermissionCapabilityShellExec      = "shell.exec"
	PermissionCapabilityMCPCall        = "mcp.call"
	PermissionCapabilityModelApproval  = "model.approval"
)

// EventKind is a type alias for events.Kind, the canonical event kind type
// shared across the SDK. Keep using EventKind* constants as before.
type EventKind = events.Kind

const (
	EventKindRunCreated          = events.KindRunCreated
	EventKindAgentStarted        = events.KindAgentStarted
	EventKindAgentPlanProposed   = events.KindAgentPlanProposed
	EventKindPermissionRequested = events.KindPermissionRequested
	EventKindPermissionGranted   = events.KindPermissionGranted
	EventKindPermissionDenied    = events.KindPermissionDenied
	EventKindToolInvoked         = events.KindToolInvoked
	EventKindToolSucceeded       = events.KindToolSucceeded
	EventKindToolFailed          = events.KindToolFailed
	EventKindActivityStarted     = events.KindActivityStarted
	EventKindActivityCompleted   = events.KindActivityCompleted

	EventKindSessionStarted = events.KindSessionStarted
	EventKindSessionEnded   = events.KindSessionEnded
	EventKindTurnStarted    = events.KindTurnStarted
	EventKindTurnCompleted  = events.KindTurnCompleted
	EventKindTurnFailed     = events.KindTurnFailed
	EventKindTurnCancelled  = events.KindTurnCancelled
	EventKindRunInterrupted = events.KindRunInterrupted
	EventKindRunHeartbeat   = events.KindRunHeartbeat

	EventKindContextPacketCreated = events.KindContextPacketCreated
	EventKindContextCompacted     = events.KindContextCompacted
	EventKindModelRequested       = events.KindModelRequested
	EventKindModelResponded       = events.KindModelResponded
	EventKindModelDelta           = events.KindModelDelta
	EventKindCheckpointCreated    = events.KindCheckpointCreated
)
