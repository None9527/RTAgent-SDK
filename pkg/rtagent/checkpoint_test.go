package rtagent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckpointResumeContinuesFromModelResponse(t *testing.T) {
	ctx := context.Background()
	var modelRequests []ModelRequest
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			modelRequests = append(modelRequests, req)
			if len(req.Observations) == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "echo",
						ReadOnly:  true,
						Arguments: map[string]any{"value": req.Input},
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{
				Output:     "checkpoint resumed: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-checkpoint-resume",
		SessionID: "session-checkpoint-resume",
		Input:     "resume pending tool",
	}, Identity{ActorID: "tester"})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "tool_provider_missing" {
		t.Fatalf("SubmitRun() error = %v projection=%#v, want tool_provider_missing", err, projection)
	}
	if projection.Status != RuntimeStatusFailed {
		t.Fatalf("failed projection Status = %q, want %q", projection.Status, RuntimeStatusFailed)
	}

	graph, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-checkpoint-resume"})
	if err != nil {
		t.Fatalf("CheckpointGraph() error = %v", err)
	}
	checkpointID := ""
	for _, node := range graph.Nodes {
		if node.Node == checkpointNodeModelResponse && node.ResumeReady {
			checkpointID = node.CheckpointID
			break
		}
	}
	if checkpointID == "" {
		t.Fatalf("missing resumable model_response checkpoint: %#v", graph.Nodes)
	}

	toolProvider := &recordingToolProvider{}
	rt.toolProvider = toolProvider
	resumed, err := rt.ResumeRun(ctx, ResumeRunRequest{
		RunID:        "run-checkpoint-resume",
		CheckpointID: checkpointID,
	})
	if err != nil {
		t.Fatalf("ResumeRun() error = %v", err)
	}
	if resumed.Status != RuntimeStatusCompleted {
		t.Fatalf("resumed Status = %q, want %q", resumed.Status, RuntimeStatusCompleted)
	}
	if resumed.Output != "checkpoint resumed: echo: resume pending tool" {
		t.Fatalf("resumed Output = %q", resumed.Output)
	}
	if len(toolProvider.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolProvider.calls))
	}
	if len(modelRequests) != 2 {
		t.Fatalf("model calls = %d, want 2", len(modelRequests))
	}
	if modelRequests[1].Iteration != 1 {
		t.Fatalf("resumed model Iteration = %d, want 1", modelRequests[1].Iteration)
	}
	if len(modelRequests[1].Messages) < 3 {
		t.Fatalf("resumed model message count = %d, want assistant tool call and tool result history", len(modelRequests[1].Messages))
	}
}

func TestCheckpointGraphHandlesRunWithoutCheckpoints(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t)
	if _, err := rt.initializeRun(ctx, RuntimeCommand{
		Scope: ExecutionScope{
			SessionID: "session-checkpoint-empty",
			RunID:     "run-checkpoint-empty",
			ActorID:   "tester",
		},
		Payload: map[string]any{"objective": "empty checkpoint graph"},
	}); err != nil {
		t.Fatalf("initializeRun() error = %v", err)
	}

	graph, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-checkpoint-empty"})
	if err != nil {
		t.Fatalf("CheckpointGraph(empty) error = %v", err)
	}
	if graph.RunID != "run-checkpoint-empty" || graph.SessionID != "session-checkpoint-empty" {
		t.Fatalf("graph run/session = %q/%q, want run-checkpoint-empty/session-checkpoint-empty", graph.RunID, graph.SessionID)
	}
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
		t.Fatalf("graph nodes/edges = %#v/%#v, want empty graph", graph.Nodes, graph.Edges)
	}
}

func TestCheckpointGraphDisablesResumeReadyAfterSessionStop(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.Observations) == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "echo",
						ReadOnly:  true,
						Arguments: map[string]any{"value": req.Input},
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{Output: "resumed", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-checkpoint-stopped",
		SessionID: "session-checkpoint-stopped",
		Input:     "checkpoint should stop",
	}, Identity{ActorID: "tester"})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "tool_provider_missing" {
		t.Fatalf("SubmitRun() error = %v projection=%#v, want tool_provider_missing", err, projection)
	}

	graph, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-checkpoint-stopped"})
	if err != nil {
		t.Fatalf("CheckpointGraph() error = %v", err)
	}
	checkpointID := ""
	for _, node := range graph.Nodes {
		if node.Node == checkpointNodeModelResponse && node.ResumeReady {
			checkpointID = node.CheckpointID
			break
		}
	}
	if checkpointID == "" {
		t.Fatalf("missing pre-stop resumable checkpoint: %#v", graph.Nodes)
	}

	if _, err := rt.StopSession(ctx, StopSessionRequest{SessionID: "session-checkpoint-stopped"}); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}
	stoppedGraph, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-checkpoint-stopped"})
	if err != nil {
		t.Fatalf("CheckpointGraph(after stop) error = %v", err)
	}
	for _, node := range stoppedGraph.Nodes {
		if node.ResumeReady {
			t.Fatalf("checkpoint %s ResumeReady = true after session stop; nodes=%#v", node.CheckpointID, stoppedGraph.Nodes)
		}
	}
	if !hasCheckpointWarning(stoppedGraph, "session session-checkpoint-stopped is stopped") {
		t.Fatalf("Warnings = %#v, want stopped-session checkpoint warning", stoppedGraph.Warnings)
	}

	rt.toolProvider = &recordingToolProvider{}
	_, err = rt.ResumeRun(ctx, ResumeRunRequest{
		RunID:        "run-checkpoint-stopped",
		CheckpointID: checkpointID,
	})
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "session_not_accepting_runs" {
		t.Fatalf("ResumeRun(after stop) error = %v, want session_not_accepting_runs", err)
	}
}

func TestCheckpointGraphDisablesResumeReadyForTerminalRun(t *testing.T) {
	ctx := context.Background()
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{Output: "already done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-checkpoint-terminal",
		SessionID: "session-checkpoint-terminal",
		Input:     "complete immediately",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("projection Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}

	graph, err := rt.CheckpointGraph(ctx, CheckpointGraphQuery{RunID: "run-checkpoint-terminal"})
	if err != nil {
		t.Fatalf("CheckpointGraph() error = %v", err)
	}
	checkpointID := ""
	for _, node := range graph.Nodes {
		if node.Node == checkpointNodeModelRequest {
			checkpointID = node.CheckpointID
		}
		if node.ResumeReady {
			t.Fatalf("checkpoint %s ResumeReady = true for terminal run; nodes=%#v", node.CheckpointID, graph.Nodes)
		}
	}
	if checkpointID == "" {
		t.Fatalf("missing model_request checkpoint: %#v", graph.Nodes)
	}
	if !hasCheckpointWarning(graph, "run run-checkpoint-terminal is completed") {
		t.Fatalf("Warnings = %#v, want terminal-run checkpoint warning", graph.Warnings)
	}

	_, err = rt.ResumeRun(ctx, ResumeRunRequest{
		RunID:        "run-checkpoint-terminal",
		CheckpointID: checkpointID,
	})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Code != "run_not_resumable" {
		t.Fatalf("ResumeRun(terminal) error = %v, want run_not_resumable", err)
	}
}

func hasCheckpointWarning(snapshot CheckpointGraphSnapshot, fragment string) bool {
	for _, warning := range snapshot.Warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}
