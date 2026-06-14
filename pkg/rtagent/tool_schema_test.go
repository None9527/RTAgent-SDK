package rtagent

import (
	"context"
	"errors"
	"testing"
)

func TestRuntimePersistsToolSchemaSnapshot(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{
			Name:        "echo",
			Description: "echo input",
			Parameters:  map[string]any{"type": "object"},
			ReadOnly:    true,
			Version:     "v1",
		}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.ToolSpecs) != 1 {
				t.Fatalf("len(ToolSpecs) = %d, want 1", len(req.ToolSpecs))
			}
			if req.ToolSpecs[0].SchemaHash == "" {
				t.Fatalf("ToolSpec.SchemaHash is empty")
			}
			if req.ToolSpecs[0].Epoch != "v1" {
				t.Fatalf("ToolSpec.Epoch = %q, want v1", req.ToolSpecs[0].Epoch)
			}
			if req.Context.ToolSchemaSnapshotID == "" {
				t.Fatalf("Context.ToolSchemaSnapshotID is empty")
			}
			if req.Context.ToolSchemaHash == "" {
				t.Fatalf("Context.ToolSchemaHash is empty")
			}
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-tool-schema-snapshot",
		SessionID: "session-tool-schema-snapshot",
		Input:     "snapshot",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}

	events, err := rt.ListEvents(ctx, EventQuery{RunID: "run-tool-schema-snapshot"})
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	snapshotID := ""
	for _, event := range events {
		if event.Kind == EventKindContextPacketCreated {
			snapshotID = firstPayloadString(event.Payload, "tool_schema_snapshot_id")
			break
		}
	}
	if snapshotID == "" {
		t.Fatalf("context packet event did not include tool_schema_snapshot_id")
	}
	snapshot, err := rt.kernel.store.GetToolSchemaSnapshot(ctx, snapshotID)
	if err != nil {
		t.Fatalf("GetToolSchemaSnapshot() error = %v", err)
	}
	if snapshot.ToolCount != 1 {
		t.Fatalf("ToolCount = %d, want 1", snapshot.ToolCount)
	}
	if snapshot.SchemaHash == "" {
		t.Fatalf("SchemaHash is empty")
	}
	if snapshot.SnapshotJSON == "" {
		t.Fatalf("SnapshotJSON is empty")
	}
}

func TestRuntimeRejectsToolCallSchemaHashMismatch(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo input", ReadOnly: true}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{
				ToolCalls: []ToolCall{{
					Name:       "echo",
					Arguments:  map[string]any{"value": req.Input},
					ReadOnly:   true,
					SchemaHash: "sha256:stale",
				}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-tool-schema-hash-mismatch",
		SessionID: "session-tool-schema-hash-mismatch",
		Input:     "hash mismatch",
	}, Identity{ActorID: "tester"})
	assertToolSchemaValidationFailure(t, projection, err)
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0", len(toolProvider.calls))
	}
}

func TestRuntimeRejectsToolCallEpochMismatch(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo input", ReadOnly: true, Version: "v1"}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			return ModelResponse{
				ToolCalls: []ToolCall{{
					Name:      "echo",
					Arguments: map[string]any{"value": req.Input},
					ReadOnly:  true,
					Epoch:     "v0",
				}},
				StopReason: "tool_calls",
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-tool-epoch-mismatch",
		SessionID: "session-tool-epoch-mismatch",
		Input:     "epoch mismatch",
	}, Identity{ActorID: "tester"})
	assertToolSchemaValidationFailure(t, projection, err)
	if len(toolProvider.calls) != 0 {
		t.Fatalf("tool calls = %d, want 0", len(toolProvider.calls))
	}
}

func TestRuntimeBindsToolCallSchemaMetadataBeforeExecution(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{
			Name:        "echo",
			Description: "echo input",
			ReadOnly:    true,
			Version:     "v1",
		}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{toolProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
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
			return ModelResponse{Output: "done", StopReason: RuntimeStatusCompleted}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-tool-schema-bind",
		SessionID: "session-tool-schema-bind",
		Input:     "bind schema",
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
	if toolProvider.calls[0].SchemaHash == "" {
		t.Fatalf("executed ToolCall.SchemaHash is empty")
	}
	if toolProvider.calls[0].Epoch != "v1" {
		t.Fatalf("executed ToolCall.Epoch = %q, want v1", toolProvider.calls[0].Epoch)
	}
}

func TestRuntimeStoresBoundSchemaMetadataInApprovalContinuation(t *testing.T) {
	ctx := context.Background()
	toolProvider := &recordingToolProvider{
		specs: []ToolSpec{{
			Name:           "edit",
			Description:    "edit files",
			SideEffectKind: "workspace.write",
			Version:        "v1",
		}},
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
		RunID:     "run-tool-schema-approval-bind",
		SessionID: "session-tool-schema-approval-bind",
		Input:     "approval bind schema",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.ApprovalRequest == nil {
		t.Fatalf("ApprovalRequest = nil, want request")
	}
	if projection.ApprovalRequest.ToolSchemaHash == "" {
		t.Fatalf("ApprovalRequest.ToolSchemaHash is empty")
	}
	if projection.ApprovalRequest.ToolEpoch != "v1" {
		t.Fatalf("ApprovalRequest.ToolEpoch = %q, want v1", projection.ApprovalRequest.ToolEpoch)
	}

	resumed, err := rt.ResolveApproval(ctx, projection.ApprovalRequest.ID, PermissionDecisionAllowForRun)
	if err != nil {
		t.Fatalf("ResolveApproval() error = %v", err)
	}
	if resumed.Status != RuntimeStatusCompleted {
		t.Fatalf("resumed Status = %q, want %q", resumed.Status, RuntimeStatusCompleted)
	}
	if len(toolProvider.calls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(toolProvider.calls))
	}
	if toolProvider.calls[0].SchemaHash != projection.ApprovalRequest.ToolSchemaHash {
		t.Fatalf("executed SchemaHash = %q, want approval hash %q", toolProvider.calls[0].SchemaHash, projection.ApprovalRequest.ToolSchemaHash)
	}
	if toolProvider.calls[0].Epoch != "v1" {
		t.Fatalf("executed Epoch = %q, want v1", toolProvider.calls[0].Epoch)
	}
}

func assertToolSchemaValidationFailure(t *testing.T, projection RuntimeStateProjection, err error) {
	t.Helper()
	var problem *RuntimeError
	if !errors.As(err, &problem) {
		t.Fatalf("SubmitRun() error = %v, want RuntimeError", err)
	}
	if problem.Code != "tool_schema_validation_failed" {
		t.Fatalf("RuntimeError.Code = %q, want tool_schema_validation_failed", problem.Code)
	}
	if projection.Status != RuntimeStatusFailed {
		t.Fatalf("projection Status = %q, want %q", projection.Status, RuntimeStatusFailed)
	}
}
