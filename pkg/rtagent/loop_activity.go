package rtagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rtagent/internal/domain/persistence"
)

func (r *Runtime) emitActivityStarted(ctx context.Context, scope ExecutionScope, activityID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := r.kernel.store.PutActivity(ctx, persistence.ActivityRecord{
		ActivityID: activityID,
		Kind:       "core_loop",
		Status:     RuntimeStatusRunning,
		Owner:      scope.ActorID,
		RunID:      scope.RunID,
		StartedAt:  now,
		UpdatedAt:  now,
		Authority:  "rtagent.sdk",
	}); err != nil {
		return fmt.Errorf("put loop activity: %w", err)
	}
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindActivityStarted,
		Message: "Core loop activity started",
		Payload: map[string]any{
			"session_id":  scope.SessionID,
			"activity_id": activityID,
			"kind":        "core_loop",
		},
	})
	return err
}

func (r *Runtime) emitActivityCompleted(ctx context.Context, scope ExecutionScope, activityID, status, errText string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	startedAt := now
	if existing, err := r.kernel.store.GetActivity(ctx, activityID); err == nil && strings.TrimSpace(existing.StartedAt) != "" {
		startedAt = existing.StartedAt
	}
	if err := r.kernel.store.PutActivity(ctx, persistence.ActivityRecord{
		ActivityID:  activityID,
		Kind:        "core_loop",
		Status:      status,
		Owner:       scope.ActorID,
		RunID:       scope.RunID,
		StartedAt:   startedAt,
		UpdatedAt:   now,
		CompletedAt: now,
		Error:       errText,
		Authority:   "rtagent.sdk",
	}); err != nil {
		return fmt.Errorf("put completed loop activity: %w", err)
	}
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindActivityCompleted,
		Message: "Core loop activity completed",
		Payload: map[string]any{
			"session_id":  scope.SessionID,
			"activity_id": activityID,
			"status":      status,
			"error":       errText,
		},
	})
	if err != nil {
		return err
	}
	if isTerminalStatus(status) {
		return r.completeDrainedSessionIfIdle(ctx, scope.SessionID)
	}
	return nil
}

func (r *Runtime) emitPlanProposed(ctx context.Context, scope ExecutionScope, plan *PlanArtifact) error {
	_, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindAgentPlanProposed,
		Message: "Plan artifact proposed by model",
		Payload: map[string]any{
			"session_id":       scope.SessionID,
			"plan_artifact_id": plan.ID,
			"revision":         plan.Revision,
			"state":            plan.State,
			"goal":             plan.Goal,
			"source":           plan.Source,
		},
	})
	return err
}
