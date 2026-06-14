package rtagent

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
	WorldStatePartitionArtifact   = "artifact"
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

type EventKind string

const (
	EventKindRunCreated          EventKind = "run.created"
	EventKindAgentStarted        EventKind = "agent.started"
	EventKindAgentPlanProposed   EventKind = "agent.plan.proposed"
	EventKindPermissionRequested EventKind = "permission.requested"
	EventKindPermissionGranted   EventKind = "permission.granted"
	EventKindPermissionDenied    EventKind = "permission.denied"
	EventKindToolInvoked         EventKind = "tool.invoked"
	EventKindToolSucceeded       EventKind = "tool.succeeded"
	EventKindToolFailed          EventKind = "tool.failed"
	EventKindActivityStarted     EventKind = "activity.started"
	EventKindActivityCompleted   EventKind = "activity.completed"

	EventKindSessionStarted EventKind = "session.started"
	EventKindSessionEnded   EventKind = "session.ended"
	EventKindTurnStarted    EventKind = "turn.started"
	EventKindTurnCompleted  EventKind = "turn.completed"
	EventKindTurnFailed     EventKind = "turn.failed"
	EventKindTurnCancelled  EventKind = "turn.cancelled"
	EventKindRunInterrupted EventKind = "run.interrupted"
	EventKindRunHeartbeat   EventKind = "run.heartbeat"

	EventKindContextPacketCreated EventKind = "context.packet.created"
	EventKindContextCompacted     EventKind = "context.compacted"
	EventKindModelRequested       EventKind = "model.requested"
	EventKindModelResponded       EventKind = "model.responded"
	EventKindModelDelta           EventKind = "model.delta"
	EventKindCheckpointCreated    EventKind = "checkpoint.created"
)
