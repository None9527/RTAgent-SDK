package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/None9527/RTAgent-SDK/internal/domain/persistence"
)

func TestPermissionCenterSuspendsSideEffectToolInDefaultMode(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{
				ToolCalls: []ToolCall{{
					Name:      "edit",
					Arguments: map[string]any{"value": "change"},
				}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-permission-tool",
		SessionID: "session-permission-tool",
		Input:     "edit a file",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusSuspended {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusSuspended)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0 before approval", len(toolProvider.calls))
	}
	assertDecisionAvailable(t, projection.ApprovalRequest.AvailableDecisions, PermissionDecisionAllowForRun)
	assertDecisionAvailable(t, projection.ApprovalRequest.AvailableDecisions, PermissionDecisionAllowAllForSession)

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-permission-tool"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	assertEventKinds(t, events, []EventKind{EventKindPermissionRequested})
	if hasEventKind(events, EventKindToolInvoked) {
		t.Fatalf("tool was invoked before permission approval")
	}
}

func TestResolveApprovalResumesApprovedToolCall(t *testing.T) {
	ctx := context.Background()
	var modelRequests []ModelRequest
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelRequests = append(modelRequests, req)
			if len(req.Observations) == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "edit",
						Arguments: map[string]any{"value": req.Input},
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{
				Output:     "resumed: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-approval-resume",
		SessionID: "session-approval-resume",
		Input:     "edit a file",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusSuspended {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusSuspended)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	resumed, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionAllowForRun)
	if err != nil {
		t.Fatalf("ResolveApproval() error = %v", err)
	}
	if resumed.Status != RuntimeStatusCompleted {
		t.Fatalf("resumed Status = %q, want %q", resumed.Status, RuntimeStatusCompleted)
	}
	if resumed.Output != "resumed: echo: edit a file" {
		t.Fatalf("resumed Output = %q, want tool observation output", resumed.Output)
	}
	if len(toolProvider.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolProvider.calls))
	}
	if len(modelRequests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(modelRequests))
	}
	if modelRequests[1].Iteration != 1 {
		t.Fatalf("resume Iteration = %d, want 1", modelRequests[1].Iteration)
	}
	if len(modelRequests[1].Observations) != 1 {
		t.Fatalf("resume Observations = %d, want 1", len(modelRequests[1].Observations))
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{RunID: "run-approval-resume"})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspect.Status != RuntimeStatusCompleted {
		t.Fatalf("inspect Status = %q, want %q", inspect.Status, RuntimeStatusCompleted)
	}
	assertEventKinds(t, inspect.Events, []EventKind{
		EventKindPermissionGranted,
		EventKindToolInvoked,
		EventKindToolSucceeded,
		EventKindModelRequested,
		EventKindModelResponded,
		EventKindTurnCompleted,
	})
}

func TestResumeRunRejectsApprovalScopeMismatch(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.Observations) == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "edit",
						Arguments: map[string]any{"value": req.Input},
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-approval-scope-a",
		SessionID: "session-approval-scope-a",
		Input:     "edit from run a",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun(run a) error = %v", err)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope: ExecutionScope{
			RunID:     "run-approval-scope-b",
			SessionID: "session-approval-scope-b",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "unrelated run b"},
	}); err != nil {
		t.Fatalf("initializeRun(run b) error = %v", err)
	}

	_, err = rt.ResumeRun(ctx, ResumeRunRequest{
		ApprovalID: projection.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		Scope: ExecutionScope{
			RunID:     "run-approval-scope-b",
			SessionID: "session-approval-scope-b",
			ActorID:   "tester",
		},
	})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "approval_scope_mismatch" {
		t.Fatalf("ResumeRun(scope mismatch) error = %v, want approval_scope_mismatch", err)
	}
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want none after rejected scope mismatch", len(toolProvider.calls))
	}
	rec, err := rt.kernel.store.GetPermission(ctx, projection.ApprovalRequest.ID)
	if err != nil {
		t.Fatalf("GetPermission(approval) error = %v", err)
	}
	if rec.Granted || rec.AuthorizedBy != "" || rec.ResolvedAt != "" {
		t.Fatalf("approval mutated after rejected scope mismatch: %#v", rec)
	}
}

func TestResolveApprovalDenyEndsSuspendedRun(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{
				ToolCalls: []ToolCall{{
					Name:      "edit",
					Arguments: map[string]any{"value": req.Input},
				}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-approval-deny",
		SessionID: "session-approval-deny",
		Input:     "edit a file",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	denied, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, "reject")
	if err != nil {
		t.Fatalf("ResolveApproval(reject) error = %v", err)
	}
	if denied.Status != RuntimeStatusDenied {
		t.Fatalf("denied Status = %q, want %q", denied.Status, RuntimeStatusDenied)
	}
	if denied.Problem == nil || denied.Problem.Code != "permission_denied" {
		t.Fatalf("denied Problem = %#v, want permission_denied", denied.Problem)
	}
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0 after denial", len(toolProvider.calls))
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{RunID: "run-approval-deny"})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspect.Status != RuntimeStatusDenied {
		t.Fatalf("inspect Status = %q, want %q", inspect.Status, RuntimeStatusDenied)
	}
	assertEventKinds(t, inspect.Events, []EventKind{EventKindPermissionDenied, EventKindTurnFailed})
	if hasEventKind(inspect.Events, EventKindToolInvoked) {
		t.Fatalf("tool was invoked after denial")
	}
}

func TestResolveApprovalResumesModelApprovalContinuation(t *testing.T) {
	ctx := context.Background()
	var modelRequests []ModelRequest
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelRequests = append(modelRequests, req)
			if len(modelRequests) == 1 {
				return ModelResponse{
					ApprovalRequest: &ApprovalRequest{
						Kind:        "plan",
						Description: "review plan before continuing",
					},
					PlanArtifact: &PlanArtifact{
						ID:        "plan-model-approval",
						RunID:     req.Scope.RunID,
						SessionID: req.Scope.SessionID,
						State:     "proposed",
						Goal:      req.Input,
					},
					StopReason: RuntimeStatusSuspended,
				}, nil
			}
			return ModelResponse{
				Output:     "continued after model approval",
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-model-approval",
		SessionID: "session-model-approval",
		Input:     "draft a plan",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusSuspended {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusSuspended)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if projection.ApprovalRequest.Permission != PermissionCapabilityModelApproval {
		t.Fatalf("approval Permission = %q, want %q", projection.ApprovalRequest.Permission, PermissionCapabilityModelApproval)
	}
	if projection.PlanArtifact == nil || projection.PlanArtifact.ID != "plan-model-approval" {
		t.Fatalf("PlanArtifact = %#v, want proposed plan", projection.PlanArtifact)
	}

	resumed, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionAllowOnce)
	if err != nil {
		t.Fatalf("ResolveApproval() error = %v", err)
	}
	if resumed.Status != RuntimeStatusCompleted {
		t.Fatalf("resumed Status = %q, want %q", resumed.Status, RuntimeStatusCompleted)
	}
	if resumed.Output != "continued after model approval" {
		t.Fatalf("resumed Output = %q", resumed.Output)
	}
	if len(modelRequests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(modelRequests))
	}
	if modelRequests[1].Iteration != 1 {
		t.Fatalf("resume Iteration = %d, want 1", modelRequests[1].Iteration)
	}
	if len(modelRequests[1].Messages) == 0 || modelRequests[1].Messages[len(modelRequests[1].Messages)-1].Metadata["approval_id"] != projection.ApprovalRequest.ID {
		t.Fatalf("resume messages missing approval grant marker: %#v", modelRequests[1].Messages)
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{RunID: "run-model-approval"})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	assertEventKinds(t, inspect.Events, []EventKind{EventKindPermissionRequested, EventKindPermissionGranted, EventKindTurnCompleted})
}

func TestResolveApprovalDenyEndsModelApprovalContinuation(t *testing.T) {
	ctx := context.Background()
	modelCalls := 0
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelCalls++
			return ModelResponse{
				ApprovalRequest: &ApprovalRequest{
					Kind:        "plan",
					Description: "review plan before continuing",
				},
				StopReason: RuntimeStatusSuspended,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-model-approval-deny",
		SessionID: "session-model-approval-deny",
		Input:     "draft a plan",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	denied, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionDeny)
	if err != nil {
		t.Fatalf("ResolveApproval(deny) error = %v", err)
	}
	if denied.Status != RuntimeStatusDenied {
		t.Fatalf("denied Status = %q, want %q", denied.Status, RuntimeStatusDenied)
	}
	if modelCalls != 1 {
		t.Fatalf("model calls = %d, want no resume call after denial", modelCalls)
	}
}

func TestPermissionCenterYoloAllowsSideEffectTool(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if req.Iteration == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "edit",
						Arguments: map[string]any{"value": "change"},
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-permission-yolo",
		SessionID: "session-permission-yolo",
		Mode:      PermissionYolo,
		Input:     "edit a file",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if len(toolProvider.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolProvider.calls))
	}
	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-permission-yolo"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	assertEventKinds(t, events, []EventKind{EventKindPermissionGranted, EventKindToolInvoked, EventKindToolSucceeded})
}

func TestPermissionCenterRejectsUnknownRunBeforeGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-missing-permission",
		SessionID:      "session-missing-permission",
		ActorID:        "tester",
		PermissionMode: PermissionYolo,
	}
	action := ProposedAction{
		ActionID: "write-missing-run",
		Kind:     PermissionCapabilityWorkspaceWrite,
		Target:   "missing.txt",
	}

	_, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	if err == nil || !strings.Contains(err.Error(), "get run") {
		t.Fatalf("CheckPermission(unknown run) error = %v, want get run error", err)
	}

	check := normalizePermissionCheck(PermissionCheckRequest{Scope: scope, Action: action})
	grant := permissionGrantForDecision(check, PermissionDecisionAllowAllForRun)
	if _, err := rt.kernel.store.GetGrant(ctx, grant.ID); err == nil {
		t.Fatalf("CheckPermission(unknown run) recorded grant %q", grant.ID)
	}
}

func TestResolvePermissionRejectsStaleUnknownRunBeforeGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-missing-resolve-permission",
		SessionID:      "session-missing-resolve-permission",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}
	action := ProposedAction{
		ActionID: "write-stale-permission",
		Kind:     PermissionCapabilityWorkspaceWrite,
		Target:   "stale.txt",
	}
	check := normalizePermissionCheck(PermissionCheckRequest{Scope: scope, Action: action})
	grant := permissionGrantForDecision(check, PermissionDecisionAllowForRun)
	approvalID := permissionIDForCheck(check)
	stored := permissionRecordScope{
		Scope:      check.scope,
		Action:     check.action,
		Grant:      grant,
		ActivityID: "activity-stale-permission",
		Reason:     "stale permission fixture",
	}
	scopeJSON, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("Marshal(stored) error = %v", err)
	}
	if err := rt.kernel.store.PutPermission(ctx, persistence.PermissionRecord{
		PermissionID:   approvalID,
		RunID:          scope.RunID,
		Subject:        scope.ActorID,
		Scope:          string(scopeJSON),
		Granted:        false,
		RequestedAt:    "2026-06-12T00:00:00Z",
		PolicyWarnings: "stale permission fixture",
	}); err != nil {
		t.Fatalf("PutPermission(stale) error = %v", err)
	}

	_, err = rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: approvalID,
		Decision:   PermissionDecisionAllowForRun,
		ActorID:    "reviewer",
	})
	if err == nil || !strings.Contains(err.Error(), "get run") {
		t.Fatalf("ResolvePermission(stale run) error = %v, want get run error", err)
	}
	if _, err := rt.kernel.store.GetGrant(ctx, grant.ID); err == nil {
		t.Fatalf("ResolvePermission(stale run) recorded grant %q", grant.ID)
	}
	rec, err := rt.kernel.store.GetPermission(ctx, approvalID)
	if err != nil {
		t.Fatalf("GetPermission(stale) error = %v", err)
	}
	if rec.Granted || rec.AuthorizedBy != "" || rec.ResolvedAt != "" {
		t.Fatalf("stale permission mutated after rejected resolve: %#v", rec)
	}
}

func TestPermissionCenterResolveCreatesReusableRunGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-permission-resolve",
		SessionID:      "session-permission-resolve",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: scope, Payload: map[string]any{"objective": "permission resolve"}}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	action := ProposedAction{
		ActionID: "write-config",
		Kind:     PermissionCapabilityWorkspaceWrite,
		Target:   "config.json",
	}
	first, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	if err != nil {
		t.Fatalf("CheckPermission(first) error = %v", err)
	}
	if first.Status != PermissionStatusRequiresApproval {
		t.Fatalf("Status = %q, want %q", first.Status, PermissionStatusRequiresApproval)
	}
	if first.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	resolved, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: first.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		ActorID:    "reviewer",
	})
	if err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if resolved.Status != PermissionStatusAllowed {
		t.Fatalf("resolved Status = %q, want %q", resolved.Status, PermissionStatusAllowed)
	}
	if resolved.Grant == nil || resolved.Grant.Scope != PermissionGrantScopeRun {
		t.Fatalf("resolved grant = %#v, want run-scoped grant", resolved.Grant)
	}

	second, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	if err != nil {
		t.Fatalf("CheckPermission(second) error = %v", err)
	}
	if second.Status != PermissionStatusAllowed {
		t.Fatalf("second Status = %q, want %q", second.Status, PermissionStatusAllowed)
	}
	if second.Grant == nil || second.Grant.ID != resolved.Grant.ID {
		t.Fatalf("second grant = %#v, want %#v", second.Grant, resolved.Grant)
	}
}

func TestPermissionCenterStoppedSessionBlocksExistingGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-permission-stopped-grant",
		SessionID:      "session-permission-stopped-grant",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: scope, Payload: map[string]any{"objective": "permission stopped grant"}}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	action := ProposedAction{
		ActionID: "write-config",
		Kind:     PermissionCapabilityWorkspaceWrite,
		Target:   "config.json",
	}
	first, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	if err != nil {
		t.Fatalf("CheckPermission(first) error = %v", err)
	}
	if first.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	resolved, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: first.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		ActorID:    "reviewer",
	})
	if err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if resolved.Grant == nil {
		t.Fatalf("resolved Grant = nil, want grant")
	}
	if _, err := rt.StopSession(ctx, StopSessionRequest{SessionID: scope.SessionID}); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	_, err = rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("CheckPermission(after stop) error = %v, want session_not_accepting_runs", err)
	}

	snapshot, err := rt.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: scope.RunID})
	if err != nil {
		t.Fatalf("PermissionSnapshot(after stop) error = %v", err)
	}
	if len(snapshot.ActiveGrants) != 0 {
		t.Fatalf("ActiveGrants after stop = %#v, want none", snapshot.ActiveGrants)
	}
	if !hasPermissionWarning(snapshot, "inactive because session is stopped") {
		t.Fatalf("Warnings after stop = %#v, want inactive stopped-session warning", snapshot.Warnings)
	}
}

func TestPermissionCenterResolveCreatesReusableSessionGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-permission-session-1",
		SessionID:      "session-permission-session",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: scope, Payload: map[string]any{"objective": "permission session"}}); err != nil {
		t.Fatalf("initializeRun(first) error = %v", err)
	}
	action := ProposedAction{
		ActionID: "write-config",
		Kind:     PermissionCapabilityWorkspaceWrite,
		Target:   "config.json",
	}
	first, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: scope, Action: action})
	if err != nil {
		t.Fatalf("CheckPermission(first) error = %v", err)
	}
	if first.Status != PermissionStatusRequiresApproval {
		t.Fatalf("first Status = %q, want %q", first.Status, PermissionStatusRequiresApproval)
	}
	if first.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	resolved, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: first.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForSession,
		ActorID:    "reviewer",
	})
	if err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if resolved.Grant == nil || resolved.Grant.Scope != PermissionGrantScopeSession {
		t.Fatalf("resolved grant = %#v, want session-scoped grant", resolved.Grant)
	}
	if resolved.Grant.RunID != "" || resolved.Grant.RootRunID != "" || resolved.Grant.TaskID != "" {
		t.Fatalf("resolved grant should not be run/root/task scoped: %#v", resolved.Grant)
	}

	sameSessionScope := scope
	sameSessionScope.RunID = "run-permission-session-2"
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: sameSessionScope, Payload: map[string]any{"objective": "permission session reuse"}}); err != nil {
		t.Fatalf("initializeRun(same session) error = %v", err)
	}
	sameResource, err := rt.CheckPermission(ctx, PermissionCheckRequest{Scope: sameSessionScope, Action: action})
	if err != nil {
		t.Fatalf("CheckPermission(same resource) error = %v", err)
	}
	if sameResource.Status != PermissionStatusAllowed {
		t.Fatalf("sameResource Status = %q, want %q", sameResource.Status, PermissionStatusAllowed)
	}
	if sameResource.Grant == nil || sameResource.Grant.ID != resolved.Grant.ID {
		t.Fatalf("sameResource grant = %#v, want %#v", sameResource.Grant, resolved.Grant)
	}

	otherResource, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: sameSessionScope,
		Action: ProposedAction{
			ActionID: "write-other",
			Kind:     PermissionCapabilityWorkspaceWrite,
			Target:   "other.json",
		},
	})
	if err != nil {
		t.Fatalf("CheckPermission(other resource) error = %v", err)
	}
	if otherResource.Status != PermissionStatusRequiresApproval {
		t.Fatalf("otherResource Status = %q, want %q", otherResource.Status, PermissionStatusRequiresApproval)
	}
}

func TestPermissionCenterResolveCreatesReusableSessionAllGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-permission-session-all-1",
		SessionID:      "session-permission-session-all",
		ActorID:        "tester",
		OwnerID:        "owner",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: scope, Payload: map[string]any{"objective": "permission session all"}}); err != nil {
		t.Fatalf("initializeRun(first) error = %v", err)
	}
	first, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: scope,
		Action: ProposedAction{
			ActionID: "write-config",
			Kind:     PermissionCapabilityWorkspaceWrite,
			Target:   "config.json",
		},
	})
	if err != nil {
		t.Fatalf("CheckPermission(first) error = %v", err)
	}
	if first.Status != PermissionStatusRequiresApproval {
		t.Fatalf("first Status = %q, want %q", first.Status, PermissionStatusRequiresApproval)
	}
	if first.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}

	resolved, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: first.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowAllForSession,
		ActorID:    "reviewer",
	})
	if err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if resolved.Status != PermissionStatusAllowed {
		t.Fatalf("resolved Status = %q, want %q", resolved.Status, PermissionStatusAllowed)
	}
	if resolved.Grant == nil {
		t.Fatalf("resolved Grant = nil, want grant")
	}
	if resolved.Grant.Scope != PermissionGrantScopeSession {
		t.Fatalf("grant Scope = %q, want %q", resolved.Grant.Scope, PermissionGrantScopeSession)
	}
	if resolved.Grant.Capability != PermissionCapabilityAny {
		t.Fatalf("grant Capability = %q, want %q", resolved.Grant.Capability, PermissionCapabilityAny)
	}
	if resolved.Grant.RunID != "" {
		t.Fatalf("grant RunID = %q, want empty for session-scoped all grant", resolved.Grant.RunID)
	}
	if resolved.Grant.SessionID != scope.SessionID {
		t.Fatalf("grant SessionID = %q, want %q", resolved.Grant.SessionID, scope.SessionID)
	}

	sameSessionScope := scope
	sameSessionScope.RunID = "run-permission-session-all-2"
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: sameSessionScope, Payload: map[string]any{"objective": "permission session all reuse"}}); err != nil {
		t.Fatalf("initializeRun(same session) error = %v", err)
	}
	sameSession, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: sameSessionScope,
		Action: ProposedAction{
			ActionID: "mcp-lookup",
			Kind:     PermissionCapabilityMCPCall,
			Target:   "mcp.filesystem.lookup",
		},
	})
	if err != nil {
		t.Fatalf("CheckPermission(same session) error = %v", err)
	}
	if sameSession.Status != PermissionStatusAllowed {
		t.Fatalf("sameSession Status = %q, want %q", sameSession.Status, PermissionStatusAllowed)
	}
	if sameSession.Grant == nil || sameSession.Grant.ID != resolved.Grant.ID {
		t.Fatalf("sameSession grant = %#v, want %#v", sameSession.Grant, resolved.Grant)
	}

	otherSessionScope := scope
	otherSessionScope.RunID = "run-permission-session-all-3"
	otherSessionScope.SessionID = "session-permission-session-all-other"
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: otherSessionScope, Payload: map[string]any{"objective": "permission session all isolated"}}); err != nil {
		t.Fatalf("initializeRun(other session) error = %v", err)
	}
	otherSession, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: otherSessionScope,
		Action: ProposedAction{
			ActionID: "mcp-lookup-other",
			Kind:     PermissionCapabilityMCPCall,
			Target:   "mcp.filesystem.lookup",
		},
	})
	if err != nil {
		t.Fatalf("CheckPermission(other session) error = %v", err)
	}
	if otherSession.Status != PermissionStatusRequiresApproval {
		t.Fatalf("otherSession Status = %q, want %q", otherSession.Status, PermissionStatusRequiresApproval)
	}
}

func TestWriteFileUsesPermissionCenter(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	defaultScope := ExecutionScope{
		RunID:          "run-write-default",
		SessionID:      "session-write-default",
		ActorID:        "tester",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: defaultScope, Payload: map[string]any{"objective": "write default"}}); err != nil {
		t.Fatalf("initializeRun(default) error = %v", err)
	}
	_, err := rt.WriteFile(ctx, WriteFileRequest{
		RelativePath: "blocked.txt",
		Content:      []byte("blocked"),
		Scope:        defaultScope,
	})
	var permissionErr *PermissionRequiredError
	if !errors.As(err, &permissionErr) {
		t.Fatalf("WriteFile(default) error = %v, want PermissionRequiredError", err)
	}
	if permissionErr.ApprovalRequest == nil {
		t.Fatalf("PermissionRequiredError approval = nil, want request")
	}

	acceptScope := ExecutionScope{
		RunID:          "run-write-accept",
		SessionID:      "session-write-accept",
		ActorID:        "tester",
		PermissionMode: PermissionAcceptEdits,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: acceptScope, Payload: map[string]any{"objective": "write accept"}}); err != nil {
		t.Fatalf("initializeRun(accept) error = %v", err)
	}
	artifact, err := rt.WriteFile(ctx, WriteFileRequest{
		RelativePath: "allowed.txt",
		Content:      []byte("allowed"),
		Scope:        acceptScope,
	})
	if err != nil {
		t.Fatalf("WriteFile(accept) error = %v", err)
	}
	if artifact.Path != "allowed.txt" {
		t.Fatalf("artifact Path = %q, want allowed.txt", artifact.Path)
	}
	contents, err := os.ReadFile(filepath.Join(rt.workDir, "allowed.txt"))
	if err != nil {
		t.Fatalf("ReadFile(allowed.txt) error = %v", err)
	}
	if string(contents) != "allowed" {
		t.Fatalf("contents = %q, want allowed", contents)
	}
}

func TestWriteFileStoppedSessionRejectedBeforeAcceptEditsGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		RunID:          "run-write-stopped",
		SessionID:      "session-write-stopped",
		ActorID:        "tester",
		PermissionMode: PermissionAcceptEdits,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{Scope: scope, Payload: map[string]any{"objective": "write stopped"}}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	if _, err := rt.StopSession(ctx, StopSessionRequest{SessionID: scope.SessionID}); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	_, err := rt.WriteFile(ctx, WriteFileRequest{
		RelativePath: "stopped.txt",
		Content:      []byte("blocked"),
		Scope:        scope,
	})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("WriteFile(stopped) error = %v, want session_not_accepting_runs", err)
	}
	if _, err := os.Stat(filepath.Join(rt.workDir, "stopped.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stopped.txt stat error = %v, want not exist", err)
	}
}

func TestWriteFileRejectsUnknownRunBeforeWorkspaceMutation(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	_, err := rt.WriteFile(ctx, WriteFileRequest{
		RelativePath: "missing-run.txt",
		Content:      []byte("should not be written"),
		Scope: ExecutionScope{
			RunID:          "run-missing-write",
			SessionID:      "session-missing-write",
			ActorID:        "tester",
			PermissionMode: PermissionAcceptEdits,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "get run") {
		t.Fatalf("WriteFile(unknown run) error = %v, want get run error", err)
	}
	if _, err := os.Stat(filepath.Join(rt.workDir, "missing-run.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing-run.txt stat error = %v, want not exist", err)
	}
}

func assertDecisionAvailable(t *testing.T, options []ApprovalDecisionOption, decision string) {
	t.Helper()
	for _, option := range options {
		if option.Decision == decision {
			return
		}
	}
	t.Fatalf("decision %q missing in %#v", decision, options)
}

func assertOnlyDenyDecisions(t *testing.T, options []ApprovalDecisionOption) {
	t.Helper()
	if len(options) == 0 {
		t.Fatalf("approval decisions are empty, want deny-only")
	}
	for _, option := range options {
		if option.Decision != PermissionDecisionDeny {
			t.Fatalf("approval decision %q present in %#v, want deny-only", option.Decision, options)
		}
	}
}

func hasEventKind(events []RuntimeEventEnvelope, kind EventKind) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func hasPermissionWarning(snapshot PermissionSnapshot, fragment string) bool {
	for _, warning := range snapshot.Warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
