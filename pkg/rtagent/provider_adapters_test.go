package rtagent

import (
	"context"
	"strings"
	"testing"
)

var (
	_ ModelProvider      = ModelProviderFunc(nil)
	_ ToolProvider       = ToolProviderAdapter{}
	_ MemoryProvider     = MemoryProviderFunc(nil)
	_ HypothesisProvider = HypothesisProviderFunc(nil)
	_ MCPProvider        = MCPProviderFunc(nil)
	_ SkillProvider      = SkillProviderFunc(nil)
	_ WorldStateProvider = WorldStateProviderAdapter{}
)

func TestModelProviderFuncAdapter(t *testing.T) {
	provider := ModelProviderFunc(func(_ context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error) {
		if req.Input != "hello" {
			t.Fatalf("input = %q, want hello", req.Input)
		}
		if stream != nil {
			if err := stream(ModelStreamEvent{Type: ModelStreamEventTextDelta, Text: "hi"}); err != nil {
				return ModelResponse{}, err
			}
		}
		return ModelResponse{Output: "done"}, nil
	})

	var streamed []ModelStreamEvent
	resp, err := provider.CompleteTurn(context.Background(), ModelRequest{Input: "hello"}, func(event ModelStreamEvent) error {
		streamed = append(streamed, event)
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteTurn returned error: %v", err)
	}
	if resp.Output != "done" {
		t.Fatalf("Output = %q, want done", resp.Output)
	}
	if len(streamed) != 1 || streamed[0].Text != "hi" {
		t.Fatalf("streamed events = %#v", streamed)
	}
}

func TestToolProviderAdapter(t *testing.T) {
	provider := ToolProviderAdapter{
		Specs: func(context.Context, ExecutionScope) ([]ToolSpec, error) {
			return []ToolSpec{{Name: "echo", ReadOnly: true}}, nil
		},
		Execute: func(_ context.Context, _ ExecutionScope, call ToolCall) (ToolObservation, error) {
			return ToolObservation{
				ToolCallID:          call.ID,
				Name:                call.Name,
				Status:              RuntimeStatusOK,
				ModelVisibleSummary: "executed " + call.Name,
			}, nil
		},
	}

	specs, err := provider.ToolSpecs(context.Background(), ExecutionScope{})
	if err != nil {
		t.Fatalf("ToolSpecs returned error: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "echo" {
		t.Fatalf("specs = %#v", specs)
	}
	obs, err := provider.ExecuteTool(context.Background(), ExecutionScope{}, ToolCall{ID: "call-1", Name: "echo"})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if obs.ModelVisibleSummary != "executed echo" {
		t.Fatalf("summary = %q", obs.ModelVisibleSummary)
	}
}

func TestProviderAdaptersRejectNilFunctions(t *testing.T) {
	if _, err := (ModelProviderFunc(nil)).CompleteTurn(context.Background(), ModelRequest{}, nil); err == nil {
		t.Fatal("nil ModelProviderFunc returned nil error")
	}
	if specs, err := (ToolProviderAdapter{}).ToolSpecs(context.Background(), ExecutionScope{}); err != nil || specs != nil {
		t.Fatalf("nil ToolProviderAdapter.Specs = %#v, %v; want nil specs and nil error", specs, err)
	}
	if _, err := (ToolProviderAdapter{}).ExecuteTool(context.Background(), ExecutionScope{}, ToolCall{}); err == nil {
		t.Fatal("nil ToolProviderAdapter.Execute returned nil error")
	}
	if _, err := (MemoryProviderFunc(nil)).MemoryFacts(context.Background(), ExecutionScope{}); err == nil {
		t.Fatal("nil MemoryProviderFunc returned nil error")
	}
	if _, err := (HypothesisProviderFunc(nil)).Hypotheses(context.Background(), ExecutionScope{}); err == nil {
		t.Fatal("nil HypothesisProviderFunc returned nil error")
	}
	if _, err := (MCPProviderFunc(nil)).MCPInventory(context.Background(), ExecutionScope{}); err == nil {
		t.Fatal("nil MCPProviderFunc returned nil error")
	}
	if _, err := (SkillProviderFunc(nil)).SkillInventory(context.Background(), ExecutionScope{}); err == nil {
		t.Fatal("nil SkillProviderFunc returned nil error")
	}
	partition, err := (WorldStateProviderAdapter{
		PartitionName: WorldStatePartitionTask,
		ProviderName:  "host_tasks",
	}).BuildWorldState(context.Background(), WorldStateProviderInput{})
	if err == nil || !strings.Contains(err.Error(), "Build is nil") {
		t.Fatalf("nil WorldStateProviderAdapter.Build error = %v, want Build is nil", err)
	}
	if partition.Partition != WorldStatePartitionTask {
		t.Fatalf("nil WorldStateProviderAdapter partition = %q, want %q", partition.Partition, WorldStatePartitionTask)
	}
	if partition.Provider != "host_tasks" || partition.Source != "host_tasks" {
		t.Fatalf("nil WorldStateProviderAdapter provider/source = %q/%q, want host_tasks/host_tasks", partition.Provider, partition.Source)
	}
}

func TestWorldStateProviderAdapterDefaultsPartitionAndProvider(t *testing.T) {
	provider := WorldStateProviderAdapter{
		PartitionName: WorldStatePartitionTask,
		ProviderName:  "host_tasks",
		Build: func(context.Context, WorldStateProviderInput) (WorldStatePartition, error) {
			return WorldStatePartition{
				Entries: []WorldStateEntry{{ID: "task:1", Subject: "task"}},
			}, nil
		},
	}

	if got := provider.Partition(); got != WorldStatePartitionTask {
		t.Fatalf("Partition() = %q, want %q", got, WorldStatePartitionTask)
	}
	partition, err := provider.BuildWorldState(context.Background(), WorldStateProviderInput{})
	if err != nil {
		t.Fatalf("BuildWorldState returned error: %v", err)
	}
	if partition.Partition != WorldStatePartitionTask {
		t.Fatalf("partition = %q, want %q", partition.Partition, WorldStatePartitionTask)
	}
	if partition.Provider != "host_tasks" || partition.Source != "host_tasks" {
		t.Fatalf("provider/source = %q/%q, want host_tasks/host_tasks", partition.Provider, partition.Source)
	}
}
