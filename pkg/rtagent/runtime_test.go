package rtagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRuntimeInitializeRunRecordsRunAndEvents(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	projection, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind: "message",
		Scope: ExecutionScope{
			SessionID: "session-test",
			RunID:     "run-test",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "exercise sdk facade"},
	})
	if err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	if projection.RunID != "run-test" {
		t.Fatalf("RunID = %q, want run-test", projection.RunID)
	}
	if projection.SessionID != "session-test" {
		t.Fatalf("SessionID = %q, want session-test", projection.SessionID)
	}
	if projection.Status != RuntimeStatusRunning {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusRunning)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-test"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Kind != EventKindSessionStarted {
		t.Fatalf("event kind = %q, want %q", events[0].Kind, EventKindSessionStarted)
	}
	if events[0].Sequence != 1 {
		t.Fatalf("event sequence = %d, want 1", events[0].Sequence)
	}
	if events[1].Kind != EventKindRunCreated {
		t.Fatalf("event kind = %q, want %q", events[1].Kind, EventKindRunCreated)
	}
	if events[1].Sequence != 2 {
		t.Fatalf("event sequence = %d, want 2", events[1].Sequence)
	}
	if events[2].Kind != EventKindTurnStarted {
		t.Fatalf("event kind = %q, want %q", events[2].Kind, EventKindTurnStarted)
	}
	if events[2].Sequence != 3 {
		t.Fatalf("event sequence = %d, want 3", events[2].Sequence)
	}
}

func TestRuntimeListEventsAndInspectBySessionID(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	base := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind:      "message",
		CreatedAt: base,
		Scope: ExecutionScope{
			SessionID: "session-query",
			RunID:     "run-session-query-1",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "first session run"},
	}); err != nil {
		t.Fatalf("initializeRun(first) error = %v", err)
	}
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind:      "message",
		CreatedAt: base.Add(time.Second),
		Scope: ExecutionScope{
			SessionID: "session-query",
			RunID:     "run-session-query-2",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "second session run"},
	}); err != nil {
		t.Fatalf("initializeRun(second) error = %v", err)
	}

	events, err := rt.ListEvents(ctx, EventQuery{SessionID: "session-query"})
	if err != nil {
		t.Fatalf("ListEvents(session) error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("len(session events) = %d, want 5: %v", len(events), eventKinds(events))
	}
	if events[0].RunID != "run-session-query-1" || events[len(events)-1].RunID != "run-session-query-2" {
		t.Fatalf("session event run order = first %q last %q", events[0].RunID, events[len(events)-1].RunID)
	}

	filtered, err := rt.ListEvents(ctx, EventQuery{SessionID: "session-query", AfterSeq: 2})
	if err != nil {
		t.Fatalf("ListEvents(session after_seq) error = %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("len(filtered session events) = %d, want 1: %v", len(filtered), eventKinds(filtered))
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{SessionID: "session-query"})
	if err != nil {
		t.Fatalf("Inspect(session) error = %v", err)
	}
	if inspect.RunID != "run-session-query-2" {
		t.Fatalf("Inspect(session).RunID = %q, want latest run", inspect.RunID)
	}
	if inspect.SessionID != "session-query" {
		t.Fatalf("Inspect(session).SessionID = %q, want session-query", inspect.SessionID)
	}

	_, err = rt.ListEvents(ctx, EventQuery{RunID: "run-session-query-1", SessionID: "other-session"})
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("ListEvents(run/session mismatch) error = %v, want mismatch error", err)
	}
	_, err = rt.Inspect(ctx, InspectQuery{RunID: "run-session-query-1", SessionID: "other-session"})
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("Inspect(run/session mismatch) error = %v, want mismatch error", err)
	}
}

func TestRuntimeEmitRejectsUnknownRun(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	_, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:   "run-missing-for-emit",
		Kind:    EventKindRunHeartbeat,
		Message: "orphan event should be rejected",
	})
	if err == nil || !strings.Contains(err.Error(), "run_id") {
		t.Fatalf("Emit(unknown run) error = %v, want run_id error", err)
	}

	_, err = rt.ListEvents(ctx, EventQuery{RunID: "run-missing-for-emit"})
	if err == nil || !strings.Contains(err.Error(), "get run") {
		t.Fatalf("ListEvents(unknown run) error = %v, want get run error", err)
	}
}

func TestRuntimeListEventsRejectsUnknownSession(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	_, err := rt.ListEvents(ctx, EventQuery{SessionID: "session-missing-events"})
	if err == nil || !strings.Contains(err.Error(), "session session-missing-events not found") {
		t.Fatalf("ListEvents(unknown session) error = %v, want session not found", err)
	}
}

func TestRuntimeReadProjectionsRejectUnknownRun(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	checks := []struct {
		name string
		call func() error
	}{
		{
			name: "Inspect",
			call: func() error {
				_, err := rt.Inspect(ctx, InspectQuery{RunID: "run-missing-projection"})
				return err
			},
		},
		{
			name: "PermissionSnapshot",
			call: func() error {
				_, err := rt.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: "run-missing-projection"})
				return err
			},
		},
		{
			name: "WorldState",
			call: func() error {
				_, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-missing-projection"})
				return err
			},
		},
		{
			name: "CheckpointGraph",
			call: func() error {
				_, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-missing-projection"})
				return err
			},
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.call()
			if err == nil || !strings.Contains(err.Error(), "get run") {
				t.Fatalf("%s(unknown run) error = %v, want get run error", check.name, err)
			}
		})
	}
}

func TestRuntimeRegisterContextHandleRejectsUnknownRun(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	err := rt.RegisterContextHandle(ctx, ContextHandle{
		HandleID:  "orphan-context-handle",
		RunID:     "run-missing-context-handle",
		Kind:      "artifact",
		SourceRef: "artifact-missing-context-handle",
	})
	if err == nil || !strings.Contains(err.Error(), "get run") {
		t.Fatalf("RegisterContextHandle(unknown run) error = %v, want get run error", err)
	}

	_, err = rt.MaterializeContext(ctx, "orphan-context-handle")
	if err == nil || !strings.Contains(err.Error(), "context handle not found") {
		t.Fatalf("MaterializeContext(rejected handle) error = %v, want context handle not found", err)
	}
}

func TestRuntimeOpenDefaultConfigUsesEphemeralStorage(t *testing.T) {
	ctx := context.Background()
	cwd := t.TempDir()
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir(%s) error = %v", cwd, err)
	}
	defer func() {
		if err := os.Chdir(oldCwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	rt, err := Open(ctx, Config{})
	if err != nil {
		t.Fatalf("Open(default config) error = %v", err)
	}
	defer rt.Close()

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-default-config",
		SessionID: "session-default-config",
		Input:     "zero config runtime",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if _, err := os.Stat(filepath.Join(cwd, "rtagent.db")); !os.IsNotExist(err) {
		t.Fatalf("default config created rtagent.db in working directory: %v", err)
	}
}

func TestRuntimeCloseMakesPublicAPIsUnavailable(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	if err := rt.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	checks := []struct {
		name string
		call func() error
	}{
		{
			name: "SubmitRun",
			call: func() error {
				_, err := rt.SubmitRun(ctx, SubmitRunRequest{
					RunID:     "run-after-close",
					SessionID: "session-after-close",
					Input:     "closed runtime",
				}, Identity{ActorID: "tester"})
				return err
			},
		},
		{
			name: "Run",
			call: func() error {
				_, err := rt.Run(ctx, RuntimeCommand{
					Scope: ExecutionScope{RunID: "run-after-close", SessionID: "session-after-close"},
				})
				return err
			},
		},
		{
			name: "Emit",
			call: func() error {
				_, err := rt.Emit(ctx, RuntimeEventDraft{RunID: "run-after-close", Kind: EventKindRunHeartbeat})
				return err
			},
		},
		{
			name: "ListEvents",
			call: func() error {
				_, err := rt.ListEvents(ctx, EventQuery{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "Inspect",
			call: func() error {
				_, err := rt.Inspect(ctx, InspectQuery{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "InspectSession",
			call: func() error {
				_, err := rt.InspectSession(ctx, SessionQuery{SessionID: "session-after-close"})
				return err
			},
		},
		{
			name: "SessionGraph",
			call: func() error {
				_, err := rt.SessionGraph(ctx, SessionGraphQuery{SessionID: "session-after-close"})
				return err
			},
		},
		{
			name: "StopSession",
			call: func() error {
				_, err := rt.StopSession(ctx, StopSessionRequest{SessionID: "session-after-close"})
				return err
			},
		},
		{
			name: "InterruptRun",
			call: func() error {
				_, err := rt.InterruptRun(ctx, "run-after-close")
				return err
			},
		},
		{
			name: "CheckpointGraph",
			call: func() error {
				_, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "ResumeRun",
			call: func() error {
				_, err := rt.ResumeRun(ctx, ResumeRunRequest{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "ResolveApproval",
			call: func() error {
				_, err := rt.ResolveApproval(ctx, "approval-after-close", PermissionDecisionAllowOnce)
				return err
			},
		},
		{
			name: "CheckPermission",
			call: func() error {
				_, err := rt.CheckPermission(ctx, PermissionCheckRequest{
					Scope:  ExecutionScope{RunID: "run-after-close", SessionID: "session-after-close"},
					Action: ProposedAction{ActionID: "action-after-close", Kind: PermissionCapabilityToolCall, Target: "tool"},
				})
				return err
			},
		},
		{
			name: "ResolvePermission",
			call: func() error {
				_, err := rt.ResolvePermission(ctx, PermissionDecisionRequest{
					ApprovalID: "approval-after-close",
					Decision:   PermissionDecisionAllowOnce,
				})
				return err
			},
		},
		{
			name: "PermissionSnapshot",
			call: func() error {
				_, err := rt.PermissionSnapshot(ctx, PermissionSnapshotQuery{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "WorldState",
			call: func() error {
				_, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-after-close"})
				return err
			},
		},
		{
			name: "RegisterContextHandle",
			call: func() error {
				return rt.RegisterContextHandle(ctx, ContextHandle{
					HandleID: "handle-after-close",
					RunID:    "run-after-close",
					Kind:     "document",
				})
			},
		},
		{
			name: "MaterializeContext",
			call: func() error {
				_, err := rt.MaterializeContext(ctx, "handle-after-close")
				return err
			},
		},
		{
			name: "WriteFile",
			call: func() error {
				_, err := rt.WriteFile(ctx, WriteFileRequest{
					RelativePath: "closed.txt",
					Content:      []byte("closed"),
					RunID:        "run-after-close",
				})
				return err
			},
		},
		{
			name: "EvaluateProposal",
			call: func() error {
				return rt.EvaluateProposal(ctx, "agent-after-close", ProposedAction{
					ActionID: "proposal-after-close",
					Kind:     PermissionCapabilityToolCall,
					Target:   "tool",
				}, "activity-after-close")
			},
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.call(); !errors.Is(err, ErrRuntimeClosed) {
				t.Fatalf("%s(after Close) error = %v, want ErrRuntimeClosed", check.name, err)
			}
		})
	}
}

func TestNewRuntimeFromKernelKeepsStartupContainerInternal(t *testing.T) {
	ctx := context.Background()
	closeCalls := 0
	rt := newRuntimeFromKernel(RuntimeConfig{}, t.TempDir(), HostPorts{}, &runtimeKernel{
		closeFn: func() error {
			closeCalls++
			return nil
		},
	})

	if rt.modelProvider == nil {
		t.Fatalf("modelProvider = nil, want default provider")
	}
	if rt.maxToolIterations != defaultMaxToolIterations {
		t.Fatalf("maxToolIterations = %d, want %d", rt.maxToolIterations, defaultMaxToolIterations)
	}
	if rt.runLeaseTTL != 5*time.Minute {
		t.Fatalf("runLeaseTTL = %s, want 5m", rt.runLeaseTTL)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if closeCalls != 1 {
		t.Fatalf("closeFn calls = %d, want 1", closeCalls)
	}
	if _, err := rt.ListEvents(ctx, EventQuery{RunID: "closed"}); !errors.Is(err, ErrRuntimeClosed) {
		t.Fatalf("ListEvents(after Close) error = %v, want ErrRuntimeClosed", err)
	}
}

func TestRuntimeWorldStateIsRunScoped(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	for _, runID := range []string{"run-a", "run-b"} {
		if _, err := rt.initializeRun(ctx, RuntimeCommand{
			Scope: ExecutionScope{SessionID: "session-test", RunID: runID},
			Payload: map[string]any{
				"objective": "run scoped world state",
			},
		}); err != nil {
			t.Fatalf("initializeRun(%s) error = %v", runID, err)
		}
	}

	// Emit real kernel events into separate runs to test run-scope
	// isolation of the activity partition. After removing the legacy
	// WorldStateBuilder (which relied on never-emitted file.modified
	// events), artifact partition no longer exists. Activity partition
	// is the natural replacement for testing run-scope projection.
	if _, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:   "run-a",
		Kind:    EventKindToolInvoked,
		Message: "run a invoked tool-a",
		Payload: map[string]any{"tool_call_id": "call-a", "tool_name": "tool-a"},
	}); err != nil {
		t.Fatalf("Emit(run-a) error = %v", err)
	}
	if _, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:   "run-b",
		Kind:    EventKindToolInvoked,
		Message: "run b invoked tool-b",
		Payload: map[string]any{"tool_call_id": "call-b", "tool_name": "tool-b"},
	}); err != nil {
		t.Fatalf("Emit(run-b) error = %v", err)
	}

	snapshotA, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-a"})
	if err != nil {
		t.Fatalf("WorldState(run-a) error = %v", err)
	}
	// Run-a's WorldState should contain its own tool activity but not run-b's.
	foundCallA := false
	foundCallB := false
	for _, entry := range snapshotA.Entries {
		if entry.Subject == "call-a" {
			foundCallA = true
		}
		if entry.Subject == "call-b" {
			foundCallB = true
		}
	}
	if !foundCallA {
		t.Fatalf("WorldState(run-a) should contain call-a activity, entries: %v", snapshotA.Entries)
	}
	if foundCallB {
		t.Fatalf("WorldState(run-a) should NOT contain call-b activity (run-scope leak), entries: %v", snapshotA.Entries)
	}

	snapshotB, err := rt.WorldState(ctx, WorldStateQuery{RunID: "run-b"})
	if err != nil {
		t.Fatalf("WorldState(run-b) error = %v", err)
	}
	foundCallA = false
	foundCallB = false
	for _, entry := range snapshotB.Entries {
		if entry.Subject == "call-a" {
			foundCallA = true
		}
		if entry.Subject == "call-b" {
			foundCallB = true
		}
	}
	if !foundCallB {
		t.Fatalf("WorldState(run-b) should contain call-b activity, entries: %v", snapshotB.Entries)
	}
	if foundCallA {
		t.Fatalf("WorldState(run-b) should NOT contain call-a activity (run-scope leak), entries: %v", snapshotB.Entries)
	}
}

func TestRuntimeEmitPreservesOccurredAt(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	occurredAt := time.Date(2026, 6, 9, 10, 30, 0, 0, time.UTC)

	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope:     ExecutionScope{SessionID: "session-test", RunID: "run-time"},
		Payload:   map[string]any{"objective": "preserve event time"},
		CreatedAt: occurredAt.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}
	if _, err := rt.Emit(ctx, RuntimeEventDraft{
		RunID:      "run-time",
		Kind:       EventKindActivityStarted,
		OccurredAt: occurredAt,
		Message:    "activity started at caller supplied time",
	}); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-time", AfterSeq: 3})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].OccurredAt != occurredAt.Format(time.RFC3339) {
		t.Fatalf("OccurredAt = %q, want %q", events[0].OccurredAt, occurredAt.Format(time.RFC3339))
	}
}

func TestRuntimeEmitAssignsContiguousSequencesConcurrently(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope:   ExecutionScope{SessionID: "session-concurrent-emit", RunID: "run-concurrent-emit"},
		Payload: map[string]any{"objective": "concurrent emit"},
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}

	const emitCount = 64
	start := make(chan struct{})
	errs := make(chan error, emitCount)
	var wg sync.WaitGroup
	for i := 0; i < emitCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := rt.Emit(ctx, RuntimeEventDraft{
				RunID:   "run-concurrent-emit",
				Kind:    EventKindRunHeartbeat,
				Message: "concurrent heartbeat",
			})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Emit() error = %v", err)
		}
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-concurrent-emit"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(events) != emitCount+3 {
		t.Fatalf("len(events) = %d, want %d", len(events), emitCount+3)
	}
	for i, event := range events {
		want := int64(i + 1)
		if event.Sequence != want {
			t.Fatalf("events[%d].Sequence = %d, want %d", i, event.Sequence, want)
		}
	}
}

func TestRuntimeSubmitRunInspectAndInterrupt(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		Kind:          "message",
		RunID:         "run-submit",
		SessionID:     "session-submit",
		Mode:          PermissionAcceptEdits,
		PlanningState: PlanningPlan,
		Input:         "exercise app-level sdk facade",
	}, Identity{ActorID: "tester", OwnerID: "owner"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.RunID != "run-submit" {
		t.Fatalf("RunID = %q, want run-submit", projection.RunID)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if projection.Output != "exercise app-level sdk facade" {
		t.Fatalf("Output = %q, want submitted input", projection.Output)
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{RunID: "run-submit"})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspect.SchemaVersion != SchemaRuntimeInspectV1 {
		t.Fatalf("SchemaVersion = %q, want %q", inspect.SchemaVersion, SchemaRuntimeInspectV1)
	}
	if inspect.LastSeq < 8 {
		t.Fatalf("LastSeq = %d, want at least 8", inspect.LastSeq)
	}
	if inspect.PermissionMode != PermissionAcceptEdits {
		t.Fatalf("PermissionMode = %q, want %q", inspect.PermissionMode, PermissionAcceptEdits)
	}
	if inspect.PlanningState != PlanningPlan {
		t.Fatalf("PlanningState = %q, want %q", inspect.PlanningState, PlanningPlan)
	}
	if inspect.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", inspect.Status, RuntimeStatusCompleted)
	}
	if inspect.Active {
		t.Fatalf("Active = true, want false for completed run")
	}
	assertEventKinds(t, inspect.Events, []EventKind{
		EventKindSessionStarted,
		EventKindRunCreated,
		EventKindTurnStarted,
		EventKindActivityStarted,
		EventKindContextPacketCreated,
		EventKindAgentStarted,
		EventKindModelRequested,
		EventKindModelResponded,
		EventKindTurnCompleted,
		EventKindActivityCompleted,
	})

	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Kind:  "message",
		Scope: ExecutionScope{SessionID: "session-interrupt", RunID: "run-interrupt"},
		Payload: map[string]any{
			"objective": "interrupt low-level running command",
		},
	}); err != nil {
		t.Fatalf("initializeRun(run-interrupt) error = %v", err)
	}

	result, err := rt.InterruptRun(ctx, "run-interrupt")
	if err != nil {
		t.Fatalf("InterruptRun() error = %v", err)
	}
	if result.Status != "interrupted" {
		t.Fatalf("interrupt status = %q, want interrupted", result.Status)
	}
	afterInterrupt, err := rt.Inspect(ctx, InspectQuery{RunID: "run-interrupt"})
	if err != nil {
		t.Fatalf("Inspect(after interrupt) error = %v", err)
	}
	if afterInterrupt.Status != RuntimeStatusCanceled {
		t.Fatalf("Status = %q, want %q", afterInterrupt.Status, RuntimeStatusCanceled)
	}
	if afterInterrupt.Active {
		t.Fatalf("Active = true, want false")
	}
}

func TestRuntimeInterruptRunIsNoopForTerminalRun(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-interrupt-terminal",
		SessionID: "session-interrupt-terminal",
		Input:     "already done",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	before, err := rt.ListEvents(ctx, EventQuery{RunID: "run-interrupt-terminal"})
	if err != nil {
		t.Fatalf("ListEvents(before) error = %v", err)
	}

	result, err := rt.InterruptRun(ctx, "run-interrupt-terminal")
	if err != nil {
		t.Fatalf("InterruptRun(terminal) error = %v", err)
	}
	if result.Status != RuntimeStatusCompleted {
		t.Fatalf("interrupt terminal status = %q, want %q", result.Status, RuntimeStatusCompleted)
	}
	if result.CancellationBy != "already_terminal" {
		t.Fatalf("CancellationBy = %q, want already_terminal", result.CancellationBy)
	}

	after, err := rt.ListEvents(ctx, EventQuery{RunID: "run-interrupt-terminal"})
	if err != nil {
		t.Fatalf("ListEvents(after) error = %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("event count after terminal interrupt = %d, want unchanged %d: %v", len(after), len(before), eventKinds(after))
	}
	for _, event := range after {
		if event.Kind == EventKindRunInterrupted {
			t.Fatalf("terminal interrupt appended %q event: %v", EventKindRunInterrupted, eventKinds(after))
		}
	}
}

func TestRuntimeModelProviderErrorPreservesStructuredProblem(t *testing.T) {
	ctx := context.Background()
	providerErr := testModelProviderError{
		message: "custom provider is rate limited",
		details: ModelProviderErrorDetails{
			Provider:     "custom-model-provider",
			StatusCode:   429,
			Code:         "rate_limit_exceeded",
			Message:      "too many requests",
			Retryable:    true,
			RateLimited:  true,
			SafeForModel: false,
			BodyPreview:  `{"error":"too many requests"}`,
		},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(_ context.Context, _ ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{}, providerErr
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-provider-error",
		SessionID: "session-provider-error",
		Input:     "trigger provider error",
	}, Identity{ActorID: "tester"})
	if err == nil {
		t.Fatalf("SubmitRun() error = nil, want RuntimeError")
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("SubmitRun() error = %T, want RuntimeError", err)
	}
	if projection.Status != RuntimeStatusFailed {
		t.Fatalf("projection Status = %q, want %q", projection.Status, RuntimeStatusFailed)
	}
	if projection.Problem == nil {
		t.Fatalf("projection Problem = nil, want structured problem")
	}
	if projection.Problem.Code != "model_turn_failed" {
		t.Fatalf("Problem.Code = %q, want model_turn_failed", projection.Problem.Code)
	}
	if projection.Problem.Provider != providerErr.details.Provider {
		t.Fatalf("Problem.Provider = %q, want %q", projection.Problem.Provider, providerErr.details.Provider)
	}
	if projection.Problem.StatusCode != providerErr.details.StatusCode {
		t.Fatalf("Problem.StatusCode = %d, want %d", projection.Problem.StatusCode, providerErr.details.StatusCode)
	}
	if projection.Problem.ProviderCode != providerErr.details.Code {
		t.Fatalf("Problem.ProviderCode = %q, want %q", projection.Problem.ProviderCode, providerErr.details.Code)
	}
	if !projection.Problem.Retryable || !projection.Problem.RateLimited {
		t.Fatalf("Problem retry fields = retryable:%v rate_limited:%v, want true/true", projection.Problem.Retryable, projection.Problem.RateLimited)
	}
	if projection.Problem.SafeForModel {
		t.Fatalf("Problem.SafeForModel = true, want false")
	}
	if projection.Problem.BodyPreview != providerErr.details.BodyPreview {
		t.Fatalf("Problem.BodyPreview = %q, want %q", projection.Problem.BodyPreview, providerErr.details.BodyPreview)
	}
	if runtimeErr.Provider != projection.Problem.Provider {
		t.Fatalf("runtimeErr.Provider = %q, want projection provider %q", runtimeErr.Provider, projection.Problem.Provider)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-provider-error"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	var failed RuntimeEventEnvelope
	for _, event := range events {
		if event.Kind == EventKindTurnFailed {
			failed = event
			break
		}
	}
	if failed.EventID == "" {
		t.Fatalf("turn.failed event missing in %v", eventKinds(events))
	}
	if got := firstPayloadString(failed.Payload, "provider"); got != providerErr.details.Provider {
		t.Fatalf("event provider = %q, want %q", got, providerErr.details.Provider)
	}
	if got := firstPayloadString(failed.Payload, "provider_code"); got != providerErr.details.Code {
		t.Fatalf("event provider_code = %q, want %q", got, providerErr.details.Code)
	}
	if got := testPayloadInt(failed.Payload, "status_code"); got != providerErr.details.StatusCode {
		t.Fatalf("event status_code = %d, want %d", got, providerErr.details.StatusCode)
	}
}

func TestRuntimePersistsModelStreamDeltaEvents(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(_ context.Context, _ ModelRequest, stream ModelStreamHandler) (ModelResponse, error) {
			if err := stream(ModelStreamEvent{
				Type: ModelStreamEventTextDelta,
				Text: "partial",
			}); err != nil {
				return ModelResponse{}, err
			}
			if err := stream(ModelStreamEvent{
				Type:           ModelStreamEventToolCallDelta,
				ToolCallIndex:  1,
				ToolCallID:     "tool-call-1",
				ToolName:       "echo",
				ArgumentsDelta: `{"value":"p`,
				Metadata:       map[string]any{"source": "test"},
			}); err != nil {
				return ModelResponse{}, err
			}
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-stream-deltas",
		SessionID: "session-stream-deltas",
		Input:     "stream deltas",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-stream-deltas"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	deltas := make([]RuntimeEventEnvelope, 0)
	for _, event := range events {
		if event.Kind == EventKindModelDelta {
			deltas = append(deltas, event)
		}
	}
	if len(deltas) != 2 {
		t.Fatalf("model.delta events = %d, want 2 in %v", len(deltas), eventKinds(events))
	}
	if got := firstPayloadString(deltas[0].Payload, "delta_type"); got != ModelStreamEventTextDelta {
		t.Fatalf("first delta_type = %q, want %q", got, ModelStreamEventTextDelta)
	}
	if got := firstPayloadString(deltas[0].Payload, "text"); got != "partial" {
		t.Fatalf("first delta text = %q, want partial", got)
	}
	if got := firstPayloadString(deltas[1].Payload, "delta_type"); got != ModelStreamEventToolCallDelta {
		t.Fatalf("second delta_type = %q, want %q", got, ModelStreamEventToolCallDelta)
	}
	if got := testPayloadInt(deltas[1].Payload, "tool_call_index"); got != 1 {
		t.Fatalf("second tool_call_index = %d, want 1", got)
	}
	if got := firstPayloadString(deltas[1].Payload, "tool_call_id"); got != "tool-call-1" {
		t.Fatalf("second tool_call_id = %q, want tool-call-1", got)
	}
	if got := firstPayloadString(deltas[1].Payload, "tool_name"); got != "echo" {
		t.Fatalf("second tool_name = %q, want echo", got)
	}
	if got := firstPayloadString(deltas[1].Payload, "arguments_delta"); got != `{"value":"p` {
		t.Fatalf("second arguments_delta = %q, want partial args", got)
	}
}

func TestRuntimeCoreLoopExecutesToolCalls(t *testing.T) {
	ctx := context.Background()
	var modelRequests []ModelRequest
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo input", ReadOnly: true}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelRequests = append(modelRequests, req)
			if req.Iteration == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "echo",
						Arguments: map[string]any{"value": req.Input},
						ReadOnly:  true,
					}},
					StopReason: "tool_calls",
				}, nil
			}
			if len(req.Observations) != 1 {
				t.Fatalf("len(Observations) = %d, want 1", len(req.Observations))
			}
			return ModelResponse{
				Output:     "observed: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-tool-loop",
		SessionID: "session-tool-loop",
		Input:     "use tool",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if projection.Output != "observed: echo: use tool" {
		t.Fatalf("Output = %q, want tool observation output", projection.Output)
	}
	if len(modelRequests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(modelRequests))
	}
	if len(toolProvider.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolProvider.calls))
	}
	if toolProvider.calls[0].ID == "" {
		t.Fatalf("tool call ID was not normalized")
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-tool-loop"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	assertEventKinds(t, events, []EventKind{
		EventKindToolInvoked,
		EventKindToolSucceeded,
		EventKindTurnCompleted,
	})
}

func TestRuntimeCoreLoopSuspendsForApproval(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{
				ApprovalRequest: &ApprovalRequest{
					Kind:        "tool",
					Description: "approval required",
				},
				StopReason: RuntimeStatusSuspended,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-approval",
		SessionID: "session-approval",
		Input:     "needs approval",
	}, Identity{ActorID: "tester", OwnerID: "owner"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusSuspended {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusSuspended)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if projection.ApprovalRequest.RunID != "run-approval" {
		t.Fatalf("approval RunID = %q, want run-approval", projection.ApprovalRequest.RunID)
	}

	inspect, err := rt.Inspect(ctx, InspectQuery{RunID: "run-approval"})
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspect.Status != RuntimeStatusSuspended {
		t.Fatalf("inspect Status = %q, want %q", inspect.Status, RuntimeStatusSuspended)
	}
	if !inspect.Blocked {
		t.Fatalf("Blocked = false, want true for suspended approval")
	}
	if inspect.Active {
		t.Fatalf("Active = true, want false for suspended approval")
	}
	assertEventKinds(t, inspect.Events, []EventKind{EventKindPermissionRequested})
}

func openTestRuntime(t *testing.T, configure ...func(*Config)) *Runtime {
	t.Helper()
	tmp := t.TempDir()
	cfg := Config{
		Runtime: RuntimeConfig{
			DSN:     filepath.Join(tmp, "rtagent.db"),
			WorkDir: tmp,
		},
	}
	for _, apply := range configure {
		apply(&cfg)
	}
	rt, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := rt.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return rt
}

func assertEventKinds(t *testing.T, events []RuntimeEventEnvelope, want []EventKind) {
	t.Helper()
	seen := map[EventKind]bool{}
	for _, event := range events {
		seen[event.Kind] = true
	}
	for _, kind := range want {
		if !seen[kind] {
			t.Fatalf("missing event kind %q in %v", kind, eventKinds(events))
		}
	}
}

func eventKinds(events []RuntimeEventEnvelope) []EventKind {
	kinds := make([]EventKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func assertSubjects(t *testing.T, snapshot WorldStateSnapshot, want []string) {
	t.Helper()
	got := make([]string, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		got = append(got, entry.Subject)
	}
	if len(got) != len(want) {
		t.Fatalf("subjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("subjects = %v, want %v", got, want)
		}
	}
}

func testPayloadInt(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

type testModelProviderError struct {
	message string
	details ModelProviderErrorDetails
}

func (e testModelProviderError) Error() string {
	return e.message
}

func (e testModelProviderError) ModelProviderErrorDetails() ModelProviderErrorDetails {
	return e.details
}

type recordingToolProvider struct {
	specs []ToolSpec
	calls []ToolCall
}

func (p *recordingToolProvider) ToolSpecs(context.Context, ExecutionScope) ([]ToolSpec, error) {
	return append([]ToolSpec(nil), p.specs...), nil
}

func (p *recordingToolProvider) ExecuteTool(_ context.Context, _ ExecutionScope, call ToolCall) (ToolObservation, error) {
	p.calls = append(p.calls, call)
	value := ""
	if call.Arguments != nil {
		if raw, ok := call.Arguments["value"]; ok {
			value = raw.(string)
		}
	}
	return ToolObservation{
		ToolCallID:          call.ID,
		Name:                call.Name,
		Status:              RuntimeStatusOK,
		ModelVisibleSummary: "echo: " + value,
	}, nil
}
