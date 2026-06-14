package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Runtime) ResolveApproval(ctx context.Context, approvalID, decision string) (RuntimeStateProjection, error) {
	return r.ResumeRun(ctx, ResumeRunRequest{ApprovalID: approvalID, Decision: decision})
}

func (r *Runtime) ResumeRun(ctx context.Context, req ResumeRunRequest) (RuntimeStateProjection, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeStateProjection{}, err
	}
	if strings.TrimSpace(req.CheckpointID) != "" {
		return r.resumeFromCheckpoint(ctx, req)
	}
	approvalID := strings.TrimSpace(req.ApprovalID)
	if approvalID == "" {
		return RuntimeStateProjection{}, errors.New("approval_id or checkpoint_id is required")
	}
	scope := req.Scope
	if strings.TrimSpace(req.RunID) != "" && strings.TrimSpace(scope.RunID) == "" {
		scope.RunID = strings.TrimSpace(req.RunID)
	}
	return r.resumeApproval(ctx, approvalID, req.Decision, scope)
}

func (r *Runtime) resumeFromCheckpoint(ctx context.Context, req ResumeRunRequest) (RuntimeStateProjection, error) {
	runID := strings.TrimSpace(req.RunID)
	if runID == "" {
		runID = strings.TrimSpace(req.Scope.RunID)
	}
	if runID == "" {
		return RuntimeStateProjection{}, errors.New("run_id is required for checkpoint resume")
	}
	rec, err := r.kernel.store.GetCheckpoint(ctx, runID, strings.TrimSpace(req.CheckpointID))
	if err != nil {
		return RuntimeStateProjection{}, fmt.Errorf("get checkpoint: %w", err)
	}
	continuation, _, err := decodeCheckpointContinuation(rec)
	if err != nil {
		return RuntimeStateProjection{}, err
	}
	approvalID := firstNonEmpty(req.ApprovalID, continuation.ApprovalID)
	if approvalID != "" {
		if strings.TrimSpace(req.Decision) == "" {
			return RuntimeStateProjection{}, errors.New("decision is required to resume an approval checkpoint")
		}
		return r.resumeApproval(ctx, approvalID, req.Decision, continuation.Scope)
	}
	if strings.TrimSpace(continuation.Scope.RunID) == "" || strings.TrimSpace(continuation.Packet.ID) == "" {
		return RuntimeStateProjection{}, fmt.Errorf("checkpoint %s does not contain resumable loop state", rec.CheckpointID)
	}
	return r.resumeLoopContinuation(ctx, "checkpoint:"+rec.CheckpointID, continuation)
}

func (r *Runtime) resumeApproval(ctx context.Context, approvalID, decision string, overrideScope ExecutionScope) (RuntimeStateProjection, error) {
	rec, err := r.kernel.store.GetPermission(ctx, approvalID)
	if err != nil {
		return RuntimeStateProjection{}, fmt.Errorf("get approval: %w", err)
	}
	stored, err := decodePermissionRecordScope(rec.Scope)
	if err != nil {
		return RuntimeStateProjection{}, err
	}
	if strings.TrimSpace(stored.Scope.RunID) == "" {
		stored.Scope.RunID = strings.TrimSpace(rec.RunID)
	}
	if err := validateApprovalResumeScope(stored.Scope, overrideScope); err != nil {
		return RuntimeStateProjection{}, err
	}
	if overrideScope.RunID != "" {
		stored.Scope = mergePermissionScope(stored.Scope, overrideScope)
	}
	normalizedDecision := normalizeApprovalDecision(decision)

	if normalizedDecision == PermissionDecisionDeny {
		if _, err := r.ResolvePermission(ctx, PermissionDecisionRequest{
			ApprovalID: approvalID,
			Decision:   PermissionDecisionDeny,
			Scope:      stored.Scope,
		}); err != nil {
			return RuntimeStateProjection{}, err
		}
		activityID := "activity:" + stored.Scope.RunID + ":approval-deny:" + shortHash(approvalID)
		if err := r.emitActivityStarted(ctx, stored.Scope, activityID); err != nil {
			return RuntimeStateProjection{}, err
		}
		projection, err := r.denyRun(ctx, stored.Scope, activityID, "permission denied by reviewer")
		var runtimeErr *RuntimeError
		if errors.As(err, &runtimeErr) {
			return projection, nil
		}
		return projection, err
	}

	if stored.Continuation == nil {
		return RuntimeStateProjection{}, errors.New("approval does not contain a resumable loop continuation")
	}
	if _, err := r.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: approvalID,
		Decision:   normalizedDecision,
		Scope:      stored.Scope,
	}); err != nil {
		return RuntimeStateProjection{}, err
	}
	return r.resumeApprovedContinuation(ctx, approvalID, stored)
}

func validateApprovalResumeScope(stored, override ExecutionScope) error {
	checks := []struct {
		name     string
		stored   string
		override string
	}{
		{name: "run_id", stored: stored.RunID, override: override.RunID},
		{name: "session_id", stored: stored.SessionID, override: override.SessionID},
		{name: "root_run_id", stored: stored.RootRunID, override: override.RootRunID},
	}
	for _, check := range checks {
		storedValue := strings.TrimSpace(check.stored)
		overrideValue := strings.TrimSpace(check.override)
		if storedValue != "" && overrideValue != "" && storedValue != overrideValue {
			return &RuntimeError{
				Code:    "approval_scope_mismatch",
				Message: fmt.Sprintf("approval resume %s mismatch: stored %q, requested %q", check.name, storedValue, overrideValue),
			}
		}
	}
	return nil
}

func (r *Runtime) resumeApprovedContinuation(ctx context.Context, approvalID string, stored permissionRecordScope) (RuntimeStateProjection, error) {
	continuation := stored.Continuation
	if continuation == nil {
		return RuntimeStateProjection{}, errors.New("approval continuation is required")
	}
	state := loopContinuation{
		Scope:            stored.Scope,
		Messages:         append([]ModelMessage(nil), continuation.Messages...),
		Observations:     append([]ToolObservation(nil), continuation.Observations...),
		PendingToolCalls: append([]ToolCall(nil), continuation.PendingToolCalls...),
		PlanArtifact:     continuation.PlanArtifact,
		Input:            continuation.Input,
		Payload:          clonePayload(continuation.Payload),
		Iteration:        continuation.Iteration,
		ToolRounds:       continuation.ToolRounds,
		ApprovalID:       approvalID,
	}
	if continuation.Packet != nil {
		state.Packet = *continuation.Packet
	}
	if len(state.PendingToolCalls) == 0 && continuation.ToolCall != nil {
		state.PendingToolCalls = []ToolCall{*continuation.ToolCall}
	}
	if len(state.PendingToolCalls) == 0 {
		state.Messages = append(state.Messages, ModelMessage{
			Role:    "user",
			Content: "Approval granted. Continue from approval " + approvalID + ".",
			Metadata: map[string]any{
				"approval_id": approvalID,
				"kind":        "approval_granted",
			},
		})
	}
	return r.resumeLoopContinuation(ctx, "approval:"+approvalID, state)
}

func (r *Runtime) resumeLoopContinuation(ctx context.Context, resumeSource string, state loopContinuation) (RuntimeStateProjection, error) {
	scope := state.Scope
	if strings.TrimSpace(scope.RunID) == "" {
		return RuntimeStateProjection{}, errors.New("resume continuation missing run_id")
	}
	run, err := r.kernel.store.GetRun(ctx, scope.RunID)
	if err != nil {
		return RuntimeStateProjection{}, fmt.Errorf("get run for resume: %w", err)
	}
	if !runStatusCanResume(run.Status) {
		return RuntimeStateProjection{}, &RuntimeError{
			Code:    "run_not_resumable",
			Message: fmt.Sprintf("resume requires resumable run status for run %s, got %q", scope.RunID, run.Status),
		}
	}
	if err := r.ensureSessionCanAcceptRun(ctx, scope.SessionID); err != nil {
		return RuntimeStateProjection{}, err
	}
	if len(state.PendingToolCalls) > 0 && r.toolProvider == nil {
		return RuntimeStateProjection{}, errors.New("approval resume requires a tool provider")
	}
	if r.modelProvider == nil {
		return RuntimeStateProjection{}, errors.New("resume requires a model provider")
	}

	activityID := "activity:" + scope.RunID + ":resume:" + shortHash(resumeSource)
	if err := r.putRunStatus(ctx, scope, RuntimeStatusRunning, RuntimeStatusRunning, false); err != nil {
		return RuntimeStateProjection{}, err
	}
	if err := r.emitActivityStarted(ctx, scope, activityID); err != nil {
		return r.failRun(ctx, scope, activityID, "approval_resume_activity_start_failed", err)
	}
	leaseID, err := r.acquireRunLease(ctx, scope, activityID)
	if err != nil {
		return r.failRun(ctx, scope, activityID, "approval_resume_lease_acquire_failed", err)
	}
	defer func() {
		if leaseID != "" && r.kernel != nil && r.kernel.leaseManager != nil {
			_ = r.kernel.leaseManager.Release(ctx, leaseID)
		}
	}()

	if state.Packet.ID == "" {
		payload := clonePayload(state.Payload)
		if state.Input != "" && firstPayloadString(payload, "objective", "input", "message") == "" {
			payload["input"] = state.Input
		}
		packet, err := r.buildContextPacket(ctx, RuntimeCommand{
			Kind:      "resume",
			Scope:     scope,
			Payload:   payload,
			CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			return r.failRun(ctx, scope, activityID, "resume_context_packet_failed", err)
		}
		if state.Input != "" {
			packet.Input = state.Input
		}
		state.Packet = packet
	} else {
		state.Packet.Events, _ = r.ListEvents(ctx, EventQuery{RunID: scope.RunID})
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindContextPacketCreated,
		Message: "Context packet assembled for resume",
		Payload: map[string]any{
			"context_packet_id":       state.Packet.ID,
			"session_id":              scope.SessionID,
			"event_count":             len(state.Packet.Events),
			"tool_count":              len(state.Packet.ToolSpecs),
			"tool_schema_snapshot_id": state.Packet.ToolSchemaSnapshotID,
			"tool_schema_hash":        state.Packet.ToolSchemaHash,
			"resume_source":           resumeSource,
		},
	}); err != nil {
		return r.failRun(ctx, scope, activityID, "resume_context_packet_event_failed", err)
	}

	state.Scope = scope
	if len(state.Messages) == 0 {
		state.Messages = initialModelMessages(state.Packet.Input)
	}
	if state.Iteration <= 0 {
		state.Iteration = 1
	}
	if len(state.PendingToolCalls) > 0 {
		projection, suspended, err := r.executeToolCallsWithPermission(ctx, &state, activityID, state.PendingToolCalls, state.ToolRounds)
		if suspended || err != nil {
			return projection, err
		}
	}
	return r.runModelToolLoop(ctx, state, activityID)
}

func runStatusCanResume(status string) bool {
	switch status {
	case RuntimeStatusSuspended, RuntimeStatusRunning, RuntimeStatusFailed:
		return true
	default:
		return false
	}
}

func normalizeApprovalDecision(decision string) string {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approve", "approved", "allow", "allowed", "yes", "y":
		return PermissionDecisionAllowOnce
	case "reject", "rejected", "deny", "denied", "no", "n":
		return PermissionDecisionDeny
	default:
		return normalizePermissionDecision(decision)
	}
}
