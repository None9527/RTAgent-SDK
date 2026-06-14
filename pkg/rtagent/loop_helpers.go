package rtagent

import (
	"context"
	"fmt"
	"strings"
)

type echoModelProvider struct{}

func (echoModelProvider) CompleteTurn(_ context.Context, req ModelRequest, _ ModelStreamHandler) (ModelResponse, error) {
	output := strings.TrimSpace(req.Input)
	if output == "" {
		output = "RTAgent core loop completed."
	}
	return ModelResponse{
		Output:     output,
		StopReason: RuntimeStatusCompleted,
	}, nil
}

func normalizeToolCall(call ToolCall, runID string, round, index int) ToolCall {
	if strings.TrimSpace(call.ID) == "" {
		call.ID = fmt.Sprintf("tool:%s:%d:%d", runID, round, index)
	}
	return call
}

func bindToolCallToSpec(call ToolCall, spec *ToolSpec) ToolCall {
	if spec == nil {
		return call
	}
	if strings.TrimSpace(call.SchemaHash) == "" {
		call.SchemaHash = strings.TrimSpace(spec.SchemaHash)
	}
	if strings.TrimSpace(call.Epoch) == "" {
		call.Epoch = firstNonEmpty(spec.Epoch, spec.Version, spec.SchemaHash)
	}
	return call
}

func normalizedPendingToolCalls(calls []ToolCall, runID string, round int) []ToolCall {
	pending := make([]ToolCall, 0, len(calls))
	for i, call := range calls {
		pending = append(pending, normalizeToolCall(call, runID, round, i))
	}
	return pending
}

func findToolSpec(specs []ToolSpec, name string) *ToolSpec {
	name = strings.TrimSpace(name)
	for i := range specs {
		if specs[i].Name == name {
			return &specs[i]
		}
	}
	return nil
}

func fillApprovalScope(approval *ApprovalRequest, scope ExecutionScope) {
	if approval.ID == "" {
		approval.ID = "approval:" + scope.RunID
	}
	if approval.RunID == "" {
		approval.RunID = scope.RunID
	}
	if approval.SessionID == "" {
		approval.SessionID = scope.SessionID
	}
	if approval.RootRunID == "" {
		approval.RootRunID = scope.RootRunID
	}
	if approval.TaskID == "" {
		approval.TaskID = scope.TaskID
	}
	if approval.ActorID == "" {
		approval.ActorID = scope.ActorID
	}
	if approval.OwnerID == "" {
		approval.OwnerID = scope.OwnerID
	}
}

func planArtifactID(plan *PlanArtifact) string {
	if plan == nil {
		return ""
	}
	return plan.ID
}

func previewString(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
