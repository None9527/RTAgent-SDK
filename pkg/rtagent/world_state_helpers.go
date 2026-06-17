package rtagent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func latestToolSchemaSnapshotEvent(events []RuntimeEventEnvelope) (RuntimeEventEnvelope, string) {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind != EventKindContextPacketCreated {
			continue
		}
		if snapshotID := firstPayloadString(events[i].Payload, "tool_schema_snapshot_id"); snapshotID != "" {
			return events[i], snapshotID
		}
	}
	return RuntimeEventEnvelope{}, ""
}

func lastRuntimeEventSeq(events []RuntimeEventEnvelope) int64 {
	var maxSeq int64
	for _, event := range events {
		if event.Sequence > maxSeq {
			maxSeq = event.Sequence
		}
	}
	return maxSeq
}

func eventObservedAt(events []RuntimeEventEnvelope, seq int64) string {
	for _, event := range events {
		if event.Sequence == seq {
			return event.OccurredAt
		}
	}
	return ""
}

func latestObservedAt(events []RuntimeEventEnvelope) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].OccurredAt
}

func worldStateEventRef(event RuntimeEventEnvelope) string {
	if strings.TrimSpace(event.EventID) != "" {
		return event.EventID
	}
	return fmt.Sprintf("seq:%d", event.Sequence)
}

func toolActivityState(event RuntimeEventEnvelope) string {
	switch event.Kind {
	case EventKindToolSucceeded:
		return RuntimeStatusCompleted
	case EventKindToolFailed:
		return RuntimeStatusFailed
	default:
		return RuntimeStatusRunning
	}
}

func eventState(event RuntimeEventEnvelope) string {
	switch event.Kind {
	case EventKindTurnCompleted, EventKindActivityCompleted, EventKindToolSucceeded:
		return RuntimeStatusCompleted
	case EventKindTurnFailed, EventKindToolFailed:
		return RuntimeStatusFailed
	case EventKindPermissionRequested:
		return RuntimeStatusSuspended
	case EventKindRunInterrupted, EventKindTurnCancelled:
		return RuntimeStatusCanceled
	default:
		return "observed"
	}
}

func worldStateScopeFromEvents(runID string, events []RuntimeEventEnvelope) ExecutionScope {
	scope := ExecutionScope{
		RunID:          runID,
		SessionID:      firstEventPayloadString(events, "session_id"),
		RootRunID:      firstEventPayloadString(events, "root_run_id"),
		ParentRunID:    firstEventPayloadString(events, "parent_run_id"),
		TaskID:         firstEventPayloadString(events, "task_id"),
		ActorID:        firstEventPayloadString(events, "actor_id"),
		OwnerID:        firstEventPayloadString(events, "owner_id"),
		PermissionMode: firstEventPayloadString(events, "permission_mode", "mode"),
	}
	if scope.RootRunID == "" {
		scope.RootRunID = runID
	}
	if scope.ActorID == "" {
		scope.ActorID = "local"
	}
	if scope.OwnerID == "" {
		scope.OwnerID = scope.ActorID
	}
	if scope.PermissionMode == "" {
		scope.PermissionMode = PermissionDefault
	}
	return scope
}

func worldStatePayloadInt(payload map[string]any, key string) int {
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		var n int
		_, _ = fmt.Sscanf(fmt.Sprint(typed), "%d", &n)
		return n
	}
}

func firstNonZeroInt64(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func payloadKeys(payload map[string]any) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func worldStatePartitionOrder() []string {
	return []string{
		WorldStatePartitionMemory,
		WorldStatePartitionCapability,
		WorldStatePartitionActivity,
		WorldStatePartitionTask,
		WorldStatePartitionContext,
		WorldStatePartitionGovernance,
		WorldStatePartitionHypothesis,
	}
}
