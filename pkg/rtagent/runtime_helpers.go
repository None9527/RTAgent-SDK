package rtagent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func isTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case RuntimeStatusCompleted, RuntimeStatusFailed, RuntimeStatusCanceled, RuntimeStatusDenied:
		return true
	default:
		return false
	}
}

func lastEventPayload(events []RuntimeEventEnvelope) map[string]any {
	for i := len(events) - 1; i >= 0; i-- {
		if len(events[i].Payload) > 0 {
			return events[i].Payload
		}
	}
	return nil
}

func firstEventPayloadString(events []RuntimeEventEnvelope, keys ...string) string {
	for i := len(events) - 1; i >= 0; i-- {
		if text := firstPayloadString(events[i].Payload, keys...); text != "" {
			return text
		}
	}
	return ""
}

func clonePayload(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = value
	}
	return out
}

func firstPayloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func executionScopeFromAction(agentID string, action ProposedAction) ExecutionScope {
	scope := ExecutionScope{
		RunID:          firstPayloadString(action.Args, "run_id"),
		SessionID:      firstPayloadString(action.Args, "session_id"),
		RootRunID:      firstPayloadString(action.Args, "root_run_id"),
		TaskID:         firstPayloadString(action.Args, "task_id"),
		ActorID:        firstNonEmpty(agentID, firstPayloadString(action.Args, "actor_id"), "local"),
		OwnerID:        firstPayloadString(action.Args, "owner_id"),
		PermissionMode: firstPayloadString(action.Args, "permission_mode", "mode"),
	}
	if scope.SessionID == "" {
		scope.SessionID = scope.RunID
	}
	if scope.RootRunID == "" {
		scope.RootRunID = scope.RunID
	}
	if scope.OwnerID == "" {
		scope.OwnerID = scope.ActorID
	}
	if scope.PermissionMode == "" {
		scope.PermissionMode = PermissionDefault
	}
	return scope
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nowUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func (r *Runtime) ensureRunExists(ctx context.Context, runID string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("get run for run_id: run_id is required")
	}
	if _, err := r.kernel.store.GetRun(ctx, runID); err != nil {
		return fmt.Errorf("get run for run_id %s: %w", runID, err)
	}
	return nil
}
