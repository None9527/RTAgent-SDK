package rtagent

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (r *Runtime) completeRun(ctx context.Context, scope ExecutionScope, activityID string, response ModelResponse) (RuntimeStateProjection, error) {
	if err := r.putRunStatus(ctx, scope, RuntimeStatusCompleted, RuntimeStatusCompleted, true); err != nil {
		return RuntimeStateProjection{}, err
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindTurnCompleted,
		Message: "Turn completed by core loop",
		Payload: map[string]any{
			"session_id":        scope.SessionID,
			"run_id":            scope.RunID,
			"activity_id":       activityID,
			"output_preview":    previewString(response.Output, 1000),
			"stop_reason":       response.StopReason,
			"response_metadata": clonePayload(response.Metadata),
		},
	}); err != nil {
		return RuntimeStateProjection{}, err
	}
	if err := r.emitActivityCompleted(ctx, scope, activityID, RuntimeStatusCompleted, ""); err != nil {
		return RuntimeStateProjection{}, err
	}
	return RuntimeStateProjection{
		RunID:          scope.RunID,
		SessionID:      scope.SessionID,
		Status:         RuntimeStatusCompleted,
		Resolution:     RuntimeStatusCompleted,
		PermissionMode: scope.PermissionMode,
		PlanningState:  scope.PlanningState,
		Output:         response.Output,
		PlanArtifact:   response.PlanArtifact,
	}, nil
}

func (r *Runtime) suspendRun(ctx context.Context, scope ExecutionScope, activityID string, response ModelResponse) (RuntimeStateProjection, error) {
	return r.suspendRunWithApproval(ctx, scope, activityID, response.ApprovalRequest, response.PlanArtifact, true)
}

func (r *Runtime) suspendRunWithApproval(ctx context.Context, scope ExecutionScope, activityID string, request *ApprovalRequest, plan *PlanArtifact, emitRequest bool) (RuntimeStateProjection, error) {
	if request == nil {
		return r.failRun(ctx, scope, activityID, "approval_request_missing", errors.New("approval request is required"))
	}
	approval := *request
	fillApprovalScope(&approval, scope)
	if err := r.putRunStatus(ctx, scope, RuntimeStatusSuspended, RuntimeStatusSuspended, false); err != nil {
		return RuntimeStateProjection{}, err
	}
	if emitRequest {
		if _, err := r.Emit(ctx, RuntimeEventDraft{
			RunID:   scope.RunID,
			Kind:    EventKindPermissionRequested,
			Message: "Core loop suspended for approval",
			Payload: map[string]any{
				"session_id":       scope.SessionID,
				"run_id":           scope.RunID,
				"activity_id":      activityID,
				"permission_id":    approval.ID,
				"subject":          scope.ActorID,
				"granted":          false,
				"approval_request": approval,
			},
		}); err != nil {
			return RuntimeStateProjection{}, err
		}
	}
	if err := r.emitActivityCompleted(ctx, scope, activityID, RuntimeStatusSuspended, "approval required"); err != nil {
		return RuntimeStateProjection{}, err
	}
	return RuntimeStateProjection{
		RunID:           scope.RunID,
		SessionID:       scope.SessionID,
		Status:          RuntimeStatusSuspended,
		Resolution:      RuntimeStatusSuspended,
		PermissionMode:  scope.PermissionMode,
		PlanningState:   scope.PlanningState,
		ApprovalRequest: &approval,
		PlanArtifact:    plan,
	}, nil
}

func (r *Runtime) denyRun(ctx context.Context, scope ExecutionScope, activityID, reason string) (RuntimeStateProjection, error) {
	problem := RuntimeError{Code: "permission_denied", Message: reason}
	if err := r.putRunStatus(ctx, scope, RuntimeStatusDenied, RuntimeStatusDenied, true); err != nil {
		return RuntimeStateProjection{}, err
	}
	_, _ = r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindTurnFailed,
		Message: "Turn denied by permission center",
		Payload: map[string]any{
			"session_id":  scope.SessionID,
			"run_id":      scope.RunID,
			"activity_id": activityID,
			"error_code":  problem.Code,
			"error":       problem.Message,
		},
	})
	_ = r.emitActivityCompleted(ctx, scope, activityID, RuntimeStatusDenied, problem.Message)
	return RuntimeStateProjection{
		RunID:          scope.RunID,
		SessionID:      scope.SessionID,
		Status:         RuntimeStatusDenied,
		Resolution:     RuntimeStatusDenied,
		PermissionMode: scope.PermissionMode,
		PlanningState:  scope.PlanningState,
		Problem:        &problem,
	}, &problem
}

func (r *Runtime) failRun(ctx context.Context, scope ExecutionScope, activityID, code string, cause error) (RuntimeStateProjection, error) {
	problem := runtimeErrorForCause(code, cause)
	if err := r.putRunStatus(ctx, scope, RuntimeStatusFailed, RuntimeStatusFailed, true); err != nil {
		return RuntimeStateProjection{}, err
	}
	_, _ = r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindTurnFailed,
		Message: "Turn failed in core loop",
		Payload: map[string]any{
			"session_id":     scope.SessionID,
			"run_id":         scope.RunID,
			"activity_id":    activityID,
			"error_code":     problem.Code,
			"error":          problem.Message,
			"provider":       problem.Provider,
			"status_code":    problem.StatusCode,
			"provider_code":  problem.ProviderCode,
			"retryable":      problem.Retryable,
			"rate_limited":   problem.RateLimited,
			"safe_for_model": problem.SafeForModel,
			"body_preview":   problem.BodyPreview,
		},
	})
	_ = r.emitActivityCompleted(ctx, scope, activityID, RuntimeStatusFailed, problem.Message)
	return RuntimeStateProjection{
		RunID:          scope.RunID,
		SessionID:      scope.SessionID,
		Status:         RuntimeStatusFailed,
		Resolution:     RuntimeStatusFailed,
		PermissionMode: scope.PermissionMode,
		PlanningState:  scope.PlanningState,
		Problem:        &problem,
	}, &problem
}

func runtimeErrorForCause(code string, cause error) RuntimeError {
	problem := RuntimeError{Code: code}
	if cause != nil {
		problem.Message = cause.Error()
	}
	var providerErr ModelProviderError
	if errors.As(cause, &providerErr) && providerErr != nil {
		details := providerErr.ModelProviderErrorDetails()
		problem.Provider = details.Provider
		problem.StatusCode = details.StatusCode
		problem.ProviderCode = details.Code
		problem.Retryable = details.Retryable
		problem.RateLimited = details.RateLimited
		problem.SafeForModel = details.SafeForModel
		problem.BodyPreview = details.BodyPreview
	}
	return problem
}

func (r *Runtime) putRunStatus(ctx context.Context, scope ExecutionScope, status, resolution string, completed bool) error {
	rec, err := r.kernel.store.GetRun(ctx, scope.RunID)
	if err != nil {
		return fmt.Errorf("get run for status update: %w", err)
	}
	rec.Status = status
	rec.Resolution = resolution
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if completed {
		rec.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := r.kernel.store.PutRun(ctx, rec); err != nil {
		return fmt.Errorf("put run status: %w", err)
	}
	return nil
}
