package rtagent

import (
	"context"
	"strings"
	"testing"
)

func TestToolRegistryMergesSpecsAndDispatchesByName(t *testing.T) {
	ctx := context.Background()
	echoProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo input", ReadOnly: true}},
	}
	editProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "edit", Description: "edit files", SideEffectKind: "workspace.write"}},
	}
	registry := NewToolRegistry(echoProvider, editProvider)

	specs, err := registry.ToolSpecs(ctx, ExecutionScope{RunID: "run-registry"})
	if err != nil {
		t.Fatalf("ToolSpecs() error = %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2", len(specs))
	}

	observation, err := registry.ExecuteTool(ctx, ExecutionScope{RunID: "run-registry"}, ToolCall{
		ID:        "tool-edit",
		Name:      "edit",
		Arguments: map[string]any{"value": "change"},
	})
	if err != nil {
		t.Fatalf("ExecuteTool(edit) error = %v", err)
	}
	if observation.ModelVisibleSummary != "echo: change" {
		t.Fatalf("ModelVisibleSummary = %q, want tool observation", observation.ModelVisibleSummary)
	}
	if len(echoProvider.calls) != 0 {
		t.Fatalf("echo calls = %d, want 0", len(echoProvider.calls))
	}
	if len(editProvider.calls) != 1 {
		t.Fatalf("edit calls = %d, want 1", len(editProvider.calls))
	}
}

func TestToolRegistryNamespacesDuplicateToolNames(t *testing.T) {
	ctx := context.Background()
	first := &recordingToolProvider{specs: []ToolSpec{{Name: "echo", Namespace: "memory"}}}
	second := &recordingToolProvider{specs: []ToolSpec{{Name: "echo", Namespace: "workspace"}}}
	registry := NewToolRegistry(first, second)

	specs, err := registry.ToolSpecs(ctx, ExecutionScope{RunID: "run-duplicate"})
	if err != nil {
		t.Fatalf("ToolSpecs() error = %v", err)
	}
	got := []string{specs[0].Name, specs[1].Name}
	want := []string{"memory__echo", "workspace__echo"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("spec names = %v, want %v", got, want)
	}

	observation, err := registry.ExecuteTool(ctx, ExecutionScope{RunID: "run-duplicate"}, ToolCall{
		Name:      "workspace__echo",
		Arguments: map[string]any{"value": "change"},
	})
	if err != nil {
		t.Fatalf("ExecuteTool(workspace__echo) error = %v", err)
	}
	if observation.Name != "workspace__echo" {
		t.Fatalf("observation name = %q, want workspace__echo", observation.Name)
	}
	if len(first.calls) != 0 {
		t.Fatalf("first calls = %d, want 0", len(first.calls))
	}
	if len(second.calls) != 1 {
		t.Fatalf("second calls = %d, want 1", len(second.calls))
	}

	_, err = registry.ExecuteTool(ctx, ExecutionScope{RunID: "run-duplicate"}, ToolCall{Name: "echo"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ExecuteTool(short echo) error = %v, want ambiguous", err)
	}
}

func TestRuntimeHostToolsExecutesRegisteredTool(t *testing.T) {
	ctx := context.Background()
	echoProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "echo", Description: "echo input", ReadOnly: true}},
	}
	lookupProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "lookup", Description: "lookup input", ReadOnly: true}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{echoProvider, lookupProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.ToolSpecs) != 2 {
				t.Fatalf("len(ToolSpecs) = %d, want 2", len(req.ToolSpecs))
			}
			if req.Iteration == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "lookup",
						Arguments: map[string]any{"value": req.Input},
						ReadOnly:  true,
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{
				Output:     "done: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-config-tool-providers",
		SessionID: "session-config-tool-providers",
		Input:     "registry input",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if projection.Output != "done: echo: registry input" {
		t.Fatalf("Output = %q, want registry tool output", projection.Output)
	}
	if len(echoProvider.calls) != 0 {
		t.Fatalf("echo calls = %d, want 0", len(echoProvider.calls))
	}
	if len(lookupProvider.calls) != 1 {
		t.Fatalf("lookup calls = %d, want 1", len(lookupProvider.calls))
	}
}

func TestRuntimeHostToolsExecutesNamespacedDuplicateTool(t *testing.T) {
	ctx := context.Background()
	memoryProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "lookup", Namespace: "memory", Description: "memory lookup", ReadOnly: true}},
	}
	workspaceProvider := &recordingToolProvider{
		specs: []ToolSpec{{Name: "lookup", Namespace: "workspace", Description: "workspace lookup", ReadOnly: true}},
	}
	rt := openTestRuntime(t, func(cfg *Config) {
		cfg.Host.Tools = []ToolProvider{memoryProvider, workspaceProvider}
		cfg.Host.Model = ModelProviderFunc(func(ctx context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
			if len(req.ToolSpecs) != 2 {
				t.Fatalf("len(ToolSpecs) = %d, want 2", len(req.ToolSpecs))
			}
			if req.ToolSpecs[0].Name != "memory__lookup" || req.ToolSpecs[1].Name != "workspace__lookup" {
				t.Fatalf("tool names = %q, %q; want namespaced lookup tools", req.ToolSpecs[0].Name, req.ToolSpecs[1].Name)
			}
			if req.Iteration == 0 {
				return ModelResponse{
					ToolCalls: []ToolCall{{
						Name:      "workspace__lookup",
						Arguments: map[string]any{"value": req.Input},
						ReadOnly:  true,
					}},
					StopReason: "tool_calls",
				}, nil
			}
			return ModelResponse{
				Output:     "done: " + req.Observations[0].ModelVisibleSummary,
				StopReason: RuntimeStatusCompleted,
			}, nil
		})
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-namespaced-tool-providers",
		SessionID: "session-namespaced-tool-providers",
		Input:     "registry input",
	}, Identity{ActorID: "tester"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if len(memoryProvider.calls) != 0 {
		t.Fatalf("memory calls = %d, want 0", len(memoryProvider.calls))
	}
	if len(workspaceProvider.calls) != 1 {
		t.Fatalf("workspace calls = %d, want 1", len(workspaceProvider.calls))
	}
	if workspaceProvider.calls[0].Name != "lookup" {
		t.Fatalf("provider call name = %q, want original lookup", workspaceProvider.calls[0].Name)
	}
}
