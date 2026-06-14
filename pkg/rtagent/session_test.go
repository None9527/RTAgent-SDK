package rtagent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRuntimeSessionGraphAndInspect(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind: "message",
		Scope: ExecutionScope{
			SessionID: "session-graph",
			RunID:     "run-root",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "root run"},
	}); err != nil {
		t.Fatalf("initializeRun(root) error = %v", err)
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind: "message",
		Scope: ExecutionScope{
			SessionID:   "session-graph",
			RunID:       "run-child",
			RootRunID:   "run-root",
			ParentRunID: "run-root",
			TaskID:      "task-child",
			ActorID:     "tester",
		},
		Payload: map[string]any{"objective": "child run"},
	}); err != nil {
		t.Fatalf("initializeRun(child) error = %v", err)
	}

	snapshot, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-graph"})
	if err != nil {
		t.Fatalf("InspectSession() error = %v", err)
	}
	if snapshot.SchemaVersion != SchemaSessionSnapshotV1 {
		t.Fatalf("SchemaVersion = %q, want %q", snapshot.SchemaVersion, SchemaSessionSnapshotV1)
	}
	if snapshot.Status != SessionStatusActive {
		t.Fatalf("Status = %q, want %q", snapshot.Status, SessionStatusActive)
	}
	if !snapshot.CanResume || !snapshot.ExternalResumeReady {
		t.Fatalf("CanResume/ExternalResumeReady = %v/%v, want true/true", snapshot.CanResume, snapshot.ExternalResumeReady)
	}
	if snapshot.ResumeCommandHint != "--resume session-graph" {
		t.Fatalf("ResumeCommandHint = %q", snapshot.ResumeCommandHint)
	}
	if snapshot.LatestRunID != "run-child" {
		t.Fatalf("LatestRunID = %q, want run-child", snapshot.LatestRunID)
	}
	if snapshot.RunCount != 2 {
		t.Fatalf("RunCount = %d, want 2", snapshot.RunCount)
	}
	if len(snapshot.ActiveRunIDs) != 2 {
		t.Fatalf("ActiveRunIDs = %v, want 2 active runs", snapshot.ActiveRunIDs)
	}

	graph, err := rt.SessionGraph(ctx, SessionGraphQuery{SessionID: "session-graph"})
	if err != nil {
		t.Fatalf("SessionGraph() error = %v", err)
	}
	if graph.SchemaVersion != SchemaSessionGraphV1 {
		t.Fatalf("graph SchemaVersion = %q, want %q", graph.SchemaVersion, SchemaSessionGraphV1)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("graph nodes = %d, want 2", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("graph edges = %#v, want one parent edge", graph.Edges)
	}
	if graph.Edges[0].FromRunID != "run-root" || graph.Edges[0].ToRunID != "run-child" || graph.Edges[0].Kind != "parent" {
		t.Fatalf("graph edge = %#v, want run-root -> run-child parent", graph.Edges[0])
	}
}

func TestRuntimeSessionGraphFiltersByRootRunID(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	runs := []RuntimeCommand{
		{
			Kind:  "message",
			Scope: ExecutionScope{SessionID: "session-graph-filter", RunID: "run-root-a", ActorID: "tester"},
			Payload: map[string]any{
				"objective": "root branch a",
			},
		},
		{
			Kind: "message",
			Scope: ExecutionScope{
				SessionID:   "session-graph-filter",
				RunID:       "run-child-a",
				RootRunID:   "run-root-a",
				ParentRunID: "run-root-a",
				ActorID:     "tester",
			},
			Payload: map[string]any{
				"objective": "child branch a",
			},
		},
		{
			Kind:  "message",
			Scope: ExecutionScope{SessionID: "session-graph-filter", RunID: "run-root-b", ActorID: "tester"},
			Payload: map[string]any{
				"objective": "root branch b",
			},
		},
	}
	for _, cmd := range runs {
		if _, err := rt.initializeRun(ctx, cmd); err != nil {
			t.Fatalf("initializeRun(%s) error = %v", cmd.Scope.RunID, err)
		}
	}

	graph, err := rt.SessionGraph(ctx, SessionGraphQuery{
		SessionID: "session-graph-filter",
		RootRunID: "run-root-a",
	})
	if err != nil {
		t.Fatalf("SessionGraph(root filter) error = %v", err)
	}
	if graph.RootRunID != "run-root-a" {
		t.Fatalf("RootRunID = %q, want run-root-a", graph.RootRunID)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("filtered graph nodes = %#v, want root and child branch a", graph.Nodes)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("filtered graph edges = %#v, want one parent edge", graph.Edges)
	}
	nodeIDs := map[string]bool{}
	for _, node := range graph.Nodes {
		nodeIDs[node.RunID] = true
	}
	if !nodeIDs["run-root-a"] || !nodeIDs["run-child-a"] || nodeIDs["run-root-b"] {
		t.Fatalf("filtered graph node ids = %#v, want only branch a", nodeIDs)
	}
	if graph.Edges[0].FromRunID != "run-root-a" || graph.Edges[0].ToRunID != "run-child-a" || graph.Edges[0].Kind != "parent" {
		t.Fatalf("filtered graph edge = %#v, want run-root-a -> run-child-a parent", graph.Edges[0])
	}
}

func TestRuntimeStopSessionCancelActive(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	for _, runID := range []string{"run-stop-a", "run-stop-b"} {
		if _, err := rt.initializeRun(ctx, RuntimeCommand{
			Scope: ExecutionScope{
				SessionID: "session-stop",
				RunID:     runID,
				ActorID:   "tester",
			},
			Payload: map[string]any{"objective": "active run"},
		}); err != nil {
			t.Fatalf("initializeRun(%s) error = %v", runID, err)
		}
	}

	result, err := rt.StopSession(ctx, StopSessionRequest{
		SessionID:   "session-stop",
		Mode:        StopSessionModeCancelActive,
		Reason:      "test stop",
		RequestedBy: "tester",
	})
	if err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}
	if result.Status != SessionStatusStopped {
		t.Fatalf("Status = %q, want %q", result.Status, SessionStatusStopped)
	}
	if len(result.InterruptedRunIDs) != 2 {
		t.Fatalf("InterruptedRunIDs = %v, want 2", result.InterruptedRunIDs)
	}

	snapshot, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-stop"})
	if err != nil {
		t.Fatalf("InspectSession(after stop) error = %v", err)
	}
	if snapshot.CanResume {
		t.Fatalf("CanResume = true, want false for stopped session")
	}
	for _, run := range snapshot.Runs {
		if run.Status != RuntimeStatusCanceled {
			t.Fatalf("run %s status = %q, want %q", run.RunID, run.Status, RuntimeStatusCanceled)
		}
	}

	_, err = rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-after-stop",
		SessionID: "session-stop",
		Input:     "should fail",
	}, Identity{ActorID: "tester"})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("SubmitRun(after stop) error = %v, want RuntimeError", err)
	}
	if runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("RuntimeError.Code = %q, want session_not_accepting_runs", runtimeErr.Code)
	}

	again, err := rt.StopSession(ctx, StopSessionRequest{SessionID: "session-stop"})
	if err != nil {
		t.Fatalf("StopSession(second) error = %v", err)
	}
	if !again.AlreadyStopped {
		t.Fatalf("AlreadyStopped = false, want true")
	}
}

func TestRuntimeStopSessionDrainRejectsNewRuns(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope: ExecutionScope{
			SessionID: "session-drain",
			RunID:     "run-drain",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "active run"},
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}

	result, err := rt.StopSession(ctx, StopSessionRequest{
		SessionID: "session-drain",
		Mode:      StopSessionModeDrain,
	})
	if err != nil {
		t.Fatalf("StopSession(drain) error = %v", err)
	}
	if result.Status != SessionStatusStopping {
		t.Fatalf("Status = %q, want %q", result.Status, SessionStatusStopping)
	}
	if len(result.InterruptedRunIDs) != 0 {
		t.Fatalf("InterruptedRunIDs = %v, want none for drain", result.InterruptedRunIDs)
	}

	snapshot, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-drain"})
	if err != nil {
		t.Fatalf("InspectSession(drain) error = %v", err)
	}
	if snapshot.CanResume {
		t.Fatalf("CanResume = true, want false for stopping session")
	}
	if len(snapshot.ActiveRunIDs) != 1 {
		t.Fatalf("ActiveRunIDs = %v, want active run preserved during drain", snapshot.ActiveRunIDs)
	}

	_, err = rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-after-drain",
		SessionID: "session-drain",
		Input:     "should fail",
	}, Identity{ActorID: "tester"})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("SubmitRun(after drain) error = %v, want session_not_accepting_runs", err)
	}
}

func TestRuntimeStopSessionDrainAutoStopsAfterActiveRunCompletes(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			select {
			case <-started:
			default:
				close(started)
			}
			select {
			case <-release:
				return ModelResponse{Output: "drained", StopReason: RuntimeStatusCompleted}, nil
			case <-ctx.Done():
				return ModelResponse{}, ctx.Err()
			}
		})
	})

	type runResult struct {
		projection RuntimeStateProjection
		err        error
	}
	resultCh := make(chan runResult, 1)
	go func() {
		projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
			RunID:     "run-drain-complete",
			SessionID: "session-drain-complete",
			Input:     "finish while draining",
		}, Identity{ActorID: "tester"})
		resultCh <- runResult{projection: projection, err: err}
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("model provider did not start")
	}

	draining, err := rt.StopSession(ctx, StopSessionRequest{
		SessionID: "session-drain-complete",
		Mode:      StopSessionModeDrain,
	})
	if err != nil {
		t.Fatalf("StopSession(drain) error = %v", err)
	}
	if draining.Status != SessionStatusStopping {
		t.Fatalf("draining Status = %q, want %q", draining.Status, SessionStatusStopping)
	}

	close(release)
	var run runResult
	select {
	case run = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("run did not complete after drain release")
	}
	if run.err != nil {
		t.Fatalf("SubmitRun() error = %v", run.err)
	}
	if run.projection.Status != RuntimeStatusCompleted {
		t.Fatalf("run Status = %q, want %q", run.projection.Status, RuntimeStatusCompleted)
	}

	snapshot, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-drain-complete"})
	if err != nil {
		t.Fatalf("InspectSession(after drain completion) error = %v", err)
	}
	if snapshot.Status != SessionStatusStopped {
		t.Fatalf("session Status = %q, want %q", snapshot.Status, SessionStatusStopped)
	}
	if snapshot.CanResume || snapshot.ExternalResumeReady {
		t.Fatalf("CanResume/ExternalResumeReady = %v/%v, want false/false", snapshot.CanResume, snapshot.ExternalResumeReady)
	}
	if len(snapshot.ActiveRunIDs) != 0 {
		t.Fatalf("ActiveRunIDs = %v, want none after drained run completes", snapshot.ActiveRunIDs)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-drain-complete"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	assertEventKinds(t, events, []EventKind{EventKindSessionEnded})
}

func TestRuntimeStopSessionDrainStopsImmediatelyWhenIdle(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	if _, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-drain-idle",
		SessionID: "session-drain-idle",
		Input:     "already complete",
	}, Identity{ActorID: "tester"}); err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}

	result, err := rt.StopSession(ctx, StopSessionRequest{
		SessionID: "session-drain-idle",
		Mode:      StopSessionModeDrain,
	})
	if err != nil {
		t.Fatalf("StopSession(drain idle) error = %v", err)
	}
	if result.Status != SessionStatusStopped {
		t.Fatalf("Status = %q, want %q", result.Status, SessionStatusStopped)
	}
	if len(result.InterruptedRunIDs) != 0 {
		t.Fatalf("InterruptedRunIDs = %v, want none for idle drain", result.InterruptedRunIDs)
	}
}

func TestRuntimeStopSessionBlocksPermissionGrant(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	scope := ExecutionScope{
		SessionID:      "session-stop-permission",
		RunID:          "run-stop-permission",
		ActorID:        "tester",
		PermissionMode: PermissionDefault,
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope:   scope,
		Payload: map[string]any{"objective": "pending permission"},
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	check, err := rt.CheckPermission(ctx, PermissionCheckRequest{
		Scope: scope,
		Action: ProposedAction{
			ActionID: "write-after-stop",
			Kind:     PermissionCapabilityWorkspaceWrite,
			Target:   "stopped.txt",
		},
	})
	if err != nil {
		t.Fatalf("CheckPermission() error = %v", err)
	}
	if check.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if _, err := rt.StopSession(ctx, StopSessionRequest{SessionID: scope.SessionID}); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	_, err = rt.ResolvePermission(ctx, PermissionDecisionRequest{
		ApprovalID: check.ApprovalRequest.ID,
		Decision:   PermissionDecisionAllowForRun,
		ActorID:    "reviewer",
	})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("ResolvePermission(after stop) error = %v, want session_not_accepting_runs", err)
	}
}

func TestRuntimeStopSessionDrainBlocksApprovalResumeButAllowsDeny(t *testing.T) {
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
			return ModelResponse{
				Output:     "resumed: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-drain-approval",
		SessionID: "session-drain-approval",
		Input:     "edit while draining",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusSuspended || projection.ApprovalRequest == nil {
		t.Fatalf("projection = %#v, want suspended approval", projection)
	}

	draining, err := rt.StopSession(ctx, StopSessionRequest{
		SessionID: "session-drain-approval",
		Mode:      StopSessionModeDrain,
	})
	if err != nil {
		t.Fatalf("StopSession(drain) error = %v", err)
	}
	if draining.Status != SessionStatusStopping {
		t.Fatalf("draining Status = %q, want %q", draining.Status, SessionStatusStopping)
	}

	permissionDuringDrain, err := rt.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: "run-drain-approval"})
	if err != nil {
		t.Fatalf("PermissionSnapshot(during drain) error = %v", err)
	}
	if len(permissionDuringDrain.PendingDecisions) != 1 {
		t.Fatalf("PendingDecisions = %#v, want one pending approval during drain", permissionDuringDrain.PendingDecisions)
	}
	assertOnlyDenyDecisions(t, permissionDuringDrain.PendingDecisions[0].AvailableDecisions)

	world, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-drain-approval", Partition: WorldStatePartitionGovernance})
	if err != nil {
		t.Fatalf("WorldState(governance during drain) error = %v", err)
	}
	pendingEntry := findWorldStateEntry(world, WorldStatePartitionGovernance, projection.ApprovalRequest.ID)
	if pendingEntry == nil {
		t.Fatalf("missing governance pending entry for %q", projection.ApprovalRequest.ID)
	}
	choices, ok := pendingEntry.Metadata["available_choices"].([]ApprovalDecisionOption)
	if !ok {
		t.Fatalf("governance available_choices = %#v, want []ApprovalDecisionOption", pendingEntry.Metadata["available_choices"])
	}
	assertOnlyDenyDecisions(t, choices)

	_, err = rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionAllowForRun)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("ResolveApproval(allow after drain) error = %v, want session_not_accepting_runs", err)
	}
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0 after blocked approval resume", len(toolProvider.calls))
	}
	permission, err := rt.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: "run-drain-approval"})
	if err != nil {
		t.Fatalf("PermissionSnapshot(after blocked resume) error = %v", err)
	}
	if len(permission.ActiveGrants) != 0 {
		t.Fatalf("ActiveGrants = %#v, want none after blocked approval resume", permission.ActiveGrants)
	}

	denied, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionDeny)
	if err != nil {
		t.Fatalf("ResolveApproval(deny after drain) error = %v", err)
	}
	if denied.Status != RuntimeStatusDenied {
		t.Fatalf("denied Status = %q, want %q", denied.Status, RuntimeStatusDenied)
	}
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0 after denial", len(toolProvider.calls))
	}

	snapshot, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-drain-approval"})
	if err != nil {
		t.Fatalf("InspectSession(after denial) error = %v", err)
	}
	if snapshot.Status != SessionStatusStopped {
		t.Fatalf("session Status = %q, want %q", snapshot.Status, SessionStatusStopped)
	}
	if snapshot.CanResume || snapshot.ExternalResumeReady {
		t.Fatalf("CanResume/ExternalResumeReady = %v/%v, want false/false", snapshot.CanResume, snapshot.ExternalResumeReady)
	}
}
