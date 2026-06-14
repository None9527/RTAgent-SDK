package rtagent

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (r *Runtime) executeToolCallsWithPermission(ctx context.Context, state *loopContinuation, activityID string, calls []ToolCall, toolRound int) (RuntimeStateProjection, bool, error) {
	scope := state.Scope
	packet := state.Packet
	for i, call := range calls {
		normalizedCall := normalizeToolCall(call, scope.RunID, toolRound, i)
		toolSpec := findToolSpec(packet.ToolSpecs, normalizedCall.Name)
		normalizedCall = bindToolCallToSpec(normalizedCall, toolSpec)
		if err := validateToolCallAgainstSpec(normalizedCall, toolSpec); err != nil {
			projection, err := r.failRun(ctx, scope, activityID, "tool_schema_validation_failed", err)
			return projection, false, err
		}
		state.PendingToolCalls = []ToolCall{normalizedCall}
		if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeToolCall, "tool", checkpointNodeToolObservation, checkpointStatusReady, *state, map[string]any{
			"tool_call_id": normalizedCall.ID,
			"tool_name":    normalizedCall.Name,
		}); err != nil {
			projection, err := r.failRun(ctx, scope, activityID, "checkpoint_tool_call_failed", err)
			return projection, false, err
		}
		permission, err := r.CheckPermission(ctx, PermissionCheckRequest{
			Scope: scope,
			Action: ProposedAction{
				ActionID: normalizedCall.ID,
				Kind:     PermissionCapabilityToolCall,
				Target:   normalizedCall.Name,
				Args:     clonePayload(normalizedCall.Arguments),
			},
			ToolCall:             &normalizedCall,
			ToolSpec:             toolSpec,
			ToolSchemaSnapshotID: packet.ToolSchemaSnapshotID,
			ActivityID:           activityID,
			Reason:               "model requested tool execution",
		})
		if err != nil {
			projection, err := r.failRun(ctx, scope, activityID, "permission_check_failed", err)
			return projection, false, err
		}
		switch permission.Status {
		case PermissionStatusDenied:
			projection, err := r.denyRun(ctx, scope, activityID, firstNonEmpty(permission.Reason, "permission denied"))
			return projection, false, err
		case PermissionStatusRequiresApproval:
			if permission.ApprovalRequest == nil {
				projection, err := r.failRun(ctx, scope, activityID, "permission_request_missing", errors.New("permission check requires approval but no approval request was returned"))
				return projection, false, err
			}
			pending := make([]ToolCall, 0, len(calls)-i)
			pending = append(pending, normalizedCall)
			for offset, remaining := range calls[i+1:] {
				remainingCall := normalizeToolCall(remaining, scope.RunID, toolRound, i+offset+1)
				remainingCall = bindToolCallToSpec(remainingCall, findToolSpec(packet.ToolSpecs, remainingCall.Name))
				pending = append(pending, remainingCall)
			}
			continuation := *state
			continuation.PendingToolCalls = pending
			continuation.Iteration = state.Iteration + 1
			continuation.ToolRounds = toolRound
			continuation.ApprovalID = permission.ApprovalRequest.ID
			if err := r.storePermissionApprovalContinuation(ctx, permission.ApprovalRequest.ID, approvalContinuation{
				ToolCall:         &normalizedCall,
				PendingToolCalls: pending,
				PlanArtifact:     state.PlanArtifact,
				Input:            packet.Input,
				Payload:          packet.Payload,
				Iteration:        state.Iteration + 1,
				ToolRounds:       toolRound,
				Messages:         append([]ModelMessage(nil), state.Messages...),
				Observations:     append([]ToolObservation(nil), state.Observations...),
				Packet:           &packet,
			}); err != nil {
				projection, err := r.failRun(ctx, scope, activityID, "approval_continuation_failed", err)
				return projection, false, err
			}
			if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeApprovalPending, "approval", checkpointNodeToolCall, checkpointStatusBlocked, continuation, map[string]any{
				"approval_id":   permission.ApprovalRequest.ID,
				"tool_call_id":  normalizedCall.ID,
				"pending_count": len(pending),
			}); err != nil {
				projection, err := r.failRun(ctx, scope, activityID, "checkpoint_tool_approval_failed", err)
				return projection, false, err
			}
			projection, err := r.suspendRunWithApproval(ctx, scope, activityID, permission.ApprovalRequest, state.PlanArtifact, false)
			return projection, true, err
		}
		observation, err := r.executeToolCall(ctx, scope, normalizedCall)
		if err != nil {
			projection, err := r.failRun(ctx, scope, activityID, "tool_call_failed", err)
			return projection, false, err
		}
		state.Observations = append(state.Observations, observation)
		state.Messages = appendToolObservationMessage(state.Messages, observation)
		state.PendingToolCalls = nil
		if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeToolObservation, "tool", checkpointNodeModelRequest, checkpointStatusCompleted, *state, map[string]any{
			"tool_call_id": observation.ToolCallID,
			"tool_name":    observation.Name,
			"status":       observation.Status,
		}); err != nil {
			projection, err := r.failRun(ctx, scope, activityID, "checkpoint_tool_observation_failed", err)
			return projection, false, err
		}
	}
	state.ToolRounds = toolRound + 1
	return RuntimeStateProjection{}, false, nil
}

func (r *Runtime) acquireRunLease(ctx context.Context, scope ExecutionScope, activityID string) (string, error) {
	if r.kernel.leaseManager == nil {
		return "", nil
	}
	return r.kernel.leaseManager.Acquire(ctx, "run:"+scope.RunID, activityID, r.runLeaseTTL)
}

func (r *Runtime) executeToolCall(ctx context.Context, scope ExecutionScope, call ToolCall) (ToolObservation, error) {
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindToolInvoked,
		Message: "Tool invoked by core loop",
		Payload: map[string]any{
			"session_id":   scope.SessionID,
			"tool_call_id": call.ID,
			"tool_name":    call.Name,
			"read_only":    call.ReadOnly,
			"schema_hash":  call.SchemaHash,
			"epoch":        call.Epoch,
			"arguments":    clonePayload(call.Arguments),
		},
	}); err != nil {
		return ToolObservation{}, err
	}
	observation, err := r.toolProvider.ExecuteTool(ctx, scope, call)
	if err != nil {
		_, _ = r.Emit(ctx, RuntimeEventDraft{
			RunID:   scope.RunID,
			Kind:    EventKindToolFailed,
			Message: "Tool execution failed",
			Payload: map[string]any{
				"session_id":   scope.SessionID,
				"tool_call_id": call.ID,
				"tool_name":    call.Name,
				"error":        err.Error(),
			},
		})
		return ToolObservation{}, fmt.Errorf("execute tool %s: %w", call.Name, err)
	}
	if strings.TrimSpace(observation.ToolCallID) == "" {
		observation.ToolCallID = call.ID
	}
	if strings.TrimSpace(observation.Name) == "" {
		observation.Name = call.Name
	}
	if strings.TrimSpace(observation.Status) == "" {
		observation.Status = RuntimeStatusOK
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindToolSucceeded,
		Message: "Tool execution completed",
		Payload: map[string]any{
			"session_id":            scope.SessionID,
			"tool_call_id":          observation.ToolCallID,
			"tool_name":             observation.Name,
			"status":                observation.Status,
			"model_visible_summary": observation.ModelVisibleSummary,
			"user_visible_summary":  observation.UserVisibleSummary,
			"output_ref":            observation.OutputRef,
			"evidence_ref_count":    len(observation.EvidenceRefs),
		},
	}); err != nil {
		return ToolObservation{}, err
	}
	return observation, nil
}
