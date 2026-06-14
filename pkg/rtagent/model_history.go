package rtagent

import (
	"context"
	"encoding/json"
	"strings"
)

func initialModelMessages(input string) []ModelMessage {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	return []ModelMessage{{Role: "user", Content: input}}
}

func appendAssistantMessage(messages []ModelMessage, response ModelResponse) []ModelMessage {
	if strings.TrimSpace(response.Output) == "" && len(response.ToolCalls) == 0 {
		return append([]ModelMessage(nil), messages...)
	}
	next := ModelMessage{
		Role:      "assistant",
		Content:   response.Output,
		ToolCalls: append([]ToolCall(nil), response.ToolCalls...),
		Metadata:  clonePayload(response.Metadata),
	}
	return append(append([]ModelMessage(nil), messages...), next)
}

func appendToolObservationMessage(messages []ModelMessage, observation ToolObservation) []ModelMessage {
	content := toolObservationContent(observation)
	if strings.TrimSpace(content) == "" {
		content = observation.ModelVisibleSummary
	}
	return append(append([]ModelMessage(nil), messages...), ModelMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: observation.ToolCallID,
		Name:       observation.Name,
		Metadata: map[string]any{
			"status":       observation.Status,
			"output_ref":   observation.OutputRef,
			"evidence_len": len(observation.EvidenceRefs),
		},
	})
}

func toolObservationContent(observation ToolObservation) string {
	payload := map[string]any{
		"tool_call_id":          observation.ToolCallID,
		"name":                  observation.Name,
		"status":                observation.Status,
		"model_visible_summary": observation.ModelVisibleSummary,
		"user_visible_summary":  observation.UserVisibleSummary,
		"output_ref":            observation.OutputRef,
		"evidence_refs":         observation.EvidenceRefs,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return observation.ModelVisibleSummary
	}
	return string(bytes)
}

func (r *Runtime) completeModelTurn(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	return r.modelProvider.CompleteTurn(ctx, req, func(delta ModelStreamEvent) error {
		if strings.TrimSpace(req.Scope.RunID) == "" {
			return nil
		}
		payload := map[string]any{
			"session_id":        req.Scope.SessionID,
			"context_packet_id": req.Context.ID,
			"iteration":         req.Iteration,
			"delta_type":        delta.Type,
			"text":              delta.Text,
			"tool_call_index":   delta.ToolCallIndex,
			"tool_call_id":      delta.ToolCallID,
			"tool_name":         delta.ToolName,
			"arguments_delta":   delta.ArgumentsDelta,
			"metadata":          clonePayload(delta.Metadata),
		}
		_, err := r.Emit(ctx, RuntimeEventDraft{
			RunID:   req.Scope.RunID,
			Kind:    EventKindModelDelta,
			Message: "Model stream delta",
			Payload: payload,
		})
		return err
	})
}
