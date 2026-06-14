package rtagent

import (
	"context"
	"strings"
)

func (r *Runtime) emitPermissionRequested(ctx context.Context, check permissionCheck, approval ApprovalRequest) error {
	if strings.TrimSpace(check.scope.RunID) == "" {
		return nil
	}
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   check.scope.RunID,
		Kind:    EventKindPermissionRequested,
		Message: "Permission approval requested",
		Payload: map[string]any{
			"session_id":       check.scope.SessionID,
			"run_id":           check.scope.RunID,
			"activity_id":      check.activityID,
			"permission_id":    approval.ID,
			"subject":          check.scope.ActorID,
			"granted":          false,
			"capability":       check.capability,
			"tool_target":      check.toolTarget,
			"resource":         check.resource,
			"risk":             check.risk,
			"approval_request": approval,
		},
	})
	return err
}

func (r *Runtime) emitPermissionGranted(ctx context.Context, check permissionCheck, decision string, grant ScopedPermissionGrant) error {
	if strings.TrimSpace(check.scope.RunID) == "" {
		return nil
	}
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   check.scope.RunID,
		Kind:    EventKindPermissionGranted,
		Message: "Permission granted",
		Payload: map[string]any{
			"session_id":    check.scope.SessionID,
			"run_id":        check.scope.RunID,
			"activity_id":   check.activityID,
			"permission_id": permissionIDForCheck(check),
			"subject":       check.scope.ActorID,
			"granted":       true,
			"decision":      decision,
			"capability":    check.capability,
			"tool_target":   check.toolTarget,
			"resource":      check.resource,
			"grant":         grant,
		},
	})
	return err
}

func (r *Runtime) emitPermissionDenied(ctx context.Context, check permissionCheck, reason string) error {
	if strings.TrimSpace(check.scope.RunID) == "" {
		return nil
	}
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   check.scope.RunID,
		Kind:    EventKindPermissionDenied,
		Message: "Permission denied",
		Payload: map[string]any{
			"session_id":    check.scope.SessionID,
			"run_id":        check.scope.RunID,
			"activity_id":   check.activityID,
			"permission_id": permissionIDForCheck(check),
			"subject":       check.scope.ActorID,
			"granted":       false,
			"decision":      PermissionDecisionDeny,
			"capability":    check.capability,
			"tool_target":   check.toolTarget,
			"resource":      check.resource,
			"reason":        reason,
		},
	})
	return err
}
