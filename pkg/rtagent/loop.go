package rtagent

import (
	"context"
	"errors"
	"fmt"

)

func (r *Runtime) Run(ctx context.Context, cmd RuntimeCommand) (RuntimeStateProjection, error) {
	if err := r.ensureReady(); err != nil {
		return RuntimeStateProjection{}, err
	}
	cmd = r.normalizeCommand(cmd)
	scope := cmd.Scope
	projection, err := r.initializeRun(ctx, cmd)
	if err != nil {
		return projection, err
	}

	activityID := "activity:" + scope.RunID + ":core-loop"
	if err := r.emitActivityStarted(ctx, scope, activityID); err != nil {
		return r.failRun(ctx, scope, activityID, "activity_start_failed", err)
	}

	leaseID, err := r.acquireRunLease(ctx, scope, activityID)
	if err != nil {
		return r.failRun(ctx, scope, activityID, "lease_acquire_failed", err)
	}
	// Start lease renewal to prevent expiry during long-running loops.
	defer r.startLeaseRenewal(ctx, leaseID)()
	defer func() {
		if leaseID != "" && r.kernel != nil && r.kernel.leaseManager != nil {
			_ = r.kernel.leaseManager.Release(ctx, leaseID)
		}
	}()

	packet, err := r.buildContextPacket(ctx, cmd)
	if err != nil {
		return r.failRun(ctx, scope, activityID, "context_packet_failed", err)
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindContextPacketCreated,
		Message: "Context packet assembled for core loop",
		Payload: map[string]any{
			"context_packet_id":       packet.ID,
			"session_id":              scope.SessionID,
			"event_count":             len(packet.Events),
			"tool_count":              len(packet.ToolSpecs),
			"tool_schema_snapshot_id": packet.ToolSchemaSnapshotID,
			"tool_schema_hash":        packet.ToolSchemaHash,
		},
	}); err != nil {
		return r.failRun(ctx, scope, activityID, "context_packet_event_failed", err)
	}
	if _, err := r.Emit(ctx, RuntimeEventDraft{
		RunID:   scope.RunID,
		Kind:    EventKindAgentStarted,
		Message: "Core loop agent started",
		Payload: map[string]any{
			"session_id":              scope.SessionID,
			"activity_id":             activityID,
			"lease_id":                leaseID,
			"context_packet_id":       packet.ID,
			"tool_schema_snapshot_id": packet.ToolSchemaSnapshotID,
		},
	}); err != nil {
		return r.failRun(ctx, scope, activityID, "agent_start_event_failed", err)
	}

	state := loopContinuation{
		Scope:                scope,
		Packet:               packet,
		Messages:             initialModelMessages(packet.Input),
		Input:                packet.Input,
		Payload:              clonePayload(packet.Payload),
		ToolSchemaSnapshotID: packet.ToolSchemaSnapshotID,
		ToolSchemaHash:       packet.ToolSchemaHash,
	}
	if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeContextPacket, "run", checkpointNodeModelRequest, checkpointStatusCompleted, state, map[string]any{
		"context_packet_id":       packet.ID,
		"tool_schema_snapshot_id": packet.ToolSchemaSnapshotID,
		"tool_schema_hash":        packet.ToolSchemaHash,
	}); err != nil {
		return r.failRun(ctx, scope, activityID, "checkpoint_context_packet_failed", err)
	}
	projection, err = r.runModelToolLoop(ctx, state, activityID)
	// Enrich the terminal projection with WorldState and Permission
	// snapshots so callers of Run / SubmitRun get a complete picture
	// without a separate Inspect call. Best-effort: errors are silently
	// swallowed so the projection is never lost.
	r.enrichProjection(ctx, scope.RunID, &projection)
	return projection, err
}

func (r *Runtime) runModelToolLoop(ctx context.Context, state loopContinuation, activityID string) (RuntimeStateProjection, error) {
	scope := state.Scope
	packet := state.Packet
	if len(state.Messages) == 0 {
		state.Messages = initialModelMessages(packet.Input)
	}
	convergence := newConvergenceController()
	finalizing := false
	finalizationReason := ""
	for {
		// Apply the configured context-message window before each model turn.
		// This bounds conversation growth so a long multi-tool run cannot
		// overflow the model's context window. When MaxContextMessages is 0
		// (default) this is a no-op. See loop_context_budget.go.
		trimmed, dropped := r.trimMessagesIfConfigured(state.Messages)
		state.Messages = trimmed
		if dropped > 0 {
			if _, err := r.Emit(ctx, RuntimeEventDraft{
				RunID:   scope.RunID,
				Kind:    EventKindContextCompacted,
				Message: "Context message window applied",
				Payload: map[string]any{
					"session_id":    scope.SessionID,
					"iteration":     state.Iteration,
					"before_count":  len(state.Messages) + dropped,
					"after_count":   len(state.Messages),
					"dropped_count": dropped,
					"window_limit":  r.maxContextMessages,
				},
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "context_compact_event_failed", err)
			}
		}
		if r.modelProvider == nil {
			return r.failRun(ctx, scope, activityID, "model_provider_missing", errors.New("model provider is required"))
		}
		if _, err := r.Emit(ctx, RuntimeEventDraft{
			RunID:   scope.RunID,
			Kind:    EventKindModelRequested,
			Message: "Model turn requested",
			Payload: map[string]any{
				"session_id":        scope.SessionID,
				"context_packet_id": packet.ID,
				"iteration":         state.Iteration,
				"observation_count": len(state.Observations),
				"message_count":     len(state.Messages),
				"tool_count":        len(packet.ToolSpecs),
			},
		}); err != nil {
			return r.failRun(ctx, scope, activityID, "model_request_event_failed", err)
		}
		if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeModelRequest, "model", checkpointNodeModelResponse, checkpointStatusReady, state, map[string]any{
			"context_packet_id": packet.ID,
			"iteration":         state.Iteration,
			"message_count":     len(state.Messages),
		}); err != nil {
			return r.failRun(ctx, scope, activityID, "checkpoint_model_request_failed", err)
		}

		response, err := r.completeModelTurn(ctx, ModelRequest{
			Scope:        scope,
			Input:        packet.Input,
			Context:      packet,
			Messages:     append([]ModelMessage(nil), state.Messages...),
			ToolSpecs:    convergenceToolSpecs(finalizing, packet.ToolSpecs),
			Observations: append([]ToolObservation(nil), state.Observations...),
			Iteration:    state.Iteration,
			Events:       append([]RuntimeEventEnvelope(nil), packet.Events...),
		})
		if err != nil {
			return r.failRun(ctx, scope, activityID, "model_turn_failed", err)
		}
		// In finalization mode the model was asked to produce a final answer
		// (tools stripped). Route directly to completion regardless of any
		// tool calls in the response — they are discarded by design.
		if finalizing {
			if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeTerminal, "complete", "", checkpointStatusTerminal, state, map[string]any{
				"status":             RuntimeStatusCompleted,
				"finalization":       true,
				"convergence_reason": finalizationReason,
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "checkpoint_terminal_failed", err)
			}
			if _, err := r.Emit(ctx, RuntimeEventDraft{
				RunID:   scope.RunID,
				Kind:    EventKindTurnCompleted,
				Message: "Convergence finalization completed",
				Payload: map[string]any{
					"session_id":         scope.SessionID,
					"finalization":       true,
					"convergence_reason": finalizationReason,
					"ignored_tool_calls": len(response.ToolCalls),
					"output_preview":     previewString(response.Output, 500),
				},
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "finalization_event_failed", err)
			}
			response.ToolCalls = nil
			return r.completeRun(ctx, scope, activityID, response)
		}
		if _, err := r.Emit(ctx, RuntimeEventDraft{
			RunID:   scope.RunID,
			Kind:    EventKindModelResponded,
			Message: "Model turn completed",
			Payload: map[string]any{
				"session_id":        scope.SessionID,
				"iteration":         state.Iteration,
				"tool_call_count":   len(response.ToolCalls),
				"approval_required": response.ApprovalRequest != nil,
				"plan_artifact_id":  planArtifactID(response.PlanArtifact),
				"stop_reason":       response.StopReason,
				"output_preview":    previewString(response.Output, 500),
				"response_metadata": clonePayload(response.Metadata),
			},
		}); err != nil {
			return r.failRun(ctx, scope, activityID, "model_response_event_failed", err)
		}
		state.Messages = appendAssistantMessage(state.Messages, response)
		state.PlanArtifact = response.PlanArtifact
		checkpointState := state
		if len(response.ToolCalls) > 0 {
			checkpointState.PendingToolCalls = normalizedPendingToolCalls(response.ToolCalls, scope.RunID, state.ToolRounds)
			checkpointState.Iteration = state.Iteration + 1
		}
		if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeModelResponse, "model", checkpointNodeToolCall, checkpointStatusCompleted, checkpointState, map[string]any{
			"iteration":         state.Iteration,
			"tool_call_count":   len(response.ToolCalls),
			"approval_required": response.ApprovalRequest != nil,
			"stop_reason":       response.StopReason,
		}); err != nil {
			return r.failRun(ctx, scope, activityID, "checkpoint_model_response_failed", err)
		}

		if response.PlanArtifact != nil {
			if err := r.emitPlanProposed(ctx, scope, response.PlanArtifact); err != nil {
				return r.failRun(ctx, scope, activityID, "plan_event_failed", err)
			}
		}
		if response.ApprovalRequest != nil {
			approval, err := r.storeModelApprovalContinuation(ctx, *response.ApprovalRequest, scope, activityID, approvalContinuation{
				PlanArtifact: state.PlanArtifact,
				Input:        packet.Input,
				Payload:      packet.Payload,
				Iteration:    state.Iteration + 1,
				ToolRounds:   state.ToolRounds,
				Messages:     append([]ModelMessage(nil), state.Messages...),
				Observations: append([]ToolObservation(nil), state.Observations...),
				Packet:       &packet,
			})
			if err != nil {
				return r.failRun(ctx, scope, activityID, "model_approval_continuation_failed", err)
			}
			response.ApprovalRequest = &approval
			state.ApprovalID = approval.ID
			if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeApprovalPending, "approval", checkpointNodeToolCall, checkpointStatusBlocked, state, map[string]any{
				"approval_id": approval.ID,
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "checkpoint_model_approval_failed", err)
			}
			return r.suspendRun(ctx, scope, activityID, response)
		}
		if len(response.ToolCalls) == 0 {
			if _, err := r.appendLoopCheckpoint(ctx, scope, checkpointNodeTerminal, "complete", "", checkpointStatusTerminal, state, map[string]any{
				"status": RuntimeStatusCompleted,
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "checkpoint_terminal_failed", err)
			}
			return r.completeRun(ctx, scope, activityID, response)
		}
		if state.ToolRounds >= r.maxToolIterations {
			return r.failRun(ctx, scope, activityID, "tool_iteration_limit_exceeded", fmt.Errorf("tool iteration limit exceeded: %d", r.maxToolIterations))
		}
		if r.toolProvider == nil {
			return r.failRun(ctx, scope, activityID, "tool_provider_missing", errors.New("model requested tool calls but no tool provider is configured"))
		}
		projection, suspended, err := r.executeToolCallsWithPermission(ctx, &state, activityID, response.ToolCalls, state.ToolRounds)
		if suspended || err != nil {
			return projection, err
		}
		// Convergence control: observe this tool turn and decide whether to
		// steer the loop. The controller tracks tool-interaction signatures
		// across the run; see convergence.go.
		newObservationCount := len(response.ToolCalls)
		var roundObservations []ToolObservation
		if newObservationCount > 0 && len(state.Observations) >= newObservationCount {
			roundObservations = state.Observations[len(state.Observations)-newObservationCount:]
		}
		decision := convergence.observe(state.Iteration+1, r.maxToolIterations, response.ToolCalls, roundObservations)
		if decision.ShouldFinalize {
			finalizing = true
			finalizationReason = decision.Reason
			state.Messages = append(state.Messages, ModelMessage{Role: "user", Content: convergenceFinalizationMessage(decision)})
			if _, err := r.Emit(ctx, RuntimeEventDraft{
				RunID:   scope.RunID,
				Kind:    EventKindRunHeartbeat,
				Message: "Convergence finalization requested",
				Payload: map[string]any{
					"session_id":         scope.SessionID,
					"kind":               "runtime.convergence_finalize",
					"convergence_reason": decision.Reason,
					"detail":             decision.Detail,
					"iteration":          state.Iteration,
				},
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "convergence_finalize_event_failed", err)
			}
		} else if decision.ShouldReplan {
			state.Messages = append(state.Messages, ModelMessage{Role: "user", Content: convergenceReplanMessage(decision)})
			if _, err := r.Emit(ctx, RuntimeEventDraft{
				RunID:   scope.RunID,
				Kind:    EventKindRunHeartbeat,
				Message: "Convergence replan requested",
				Payload: map[string]any{
					"session_id":           scope.SessionID,
					"kind":                 "runtime.convergence_replan",
					"convergence_reason":   decision.Reason,
					"detail":               decision.Detail,
					"iteration":            state.Iteration,
					"tools_remain_enabled": true,
				},
			}); err != nil {
				return r.failRun(ctx, scope, activityID, "convergence_replan_event_failed", err)
			}
		}
		state.Iteration++
	}
}
