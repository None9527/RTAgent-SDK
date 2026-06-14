package rtagent

import (
	"encoding/json"
	"strings"
)

func (p *OpenAICompatibleProvider) buildRequestPayload(req ModelRequest, stream bool) map[string]any {
	payload := map[string]any{
		"model":    p.model,
		"messages": p.openAIMessages(req),
		"stream":   stream,
	}
	if len(req.ToolSpecs) > 0 {
		payload["tools"] = openAIToolsFromSpecs(req.ToolSpecs)
		payload["tool_choice"] = "auto"
	}
	for key, value := range p.extraBody {
		payload[key] = value
	}
	return payload
}

func (p *OpenAICompatibleProvider) openAIMessages(req ModelRequest) []openAIChatMessage {
	messages := make([]openAIChatMessage, 0, len(req.Messages)+2)
	hasSystem := false
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Role) == "" {
			continue
		}
		if message.Role == "system" {
			hasSystem = true
		}
		messages = append(messages, openAIMessageFromModelMessage(message))
	}
	if len(messages) == 0 {
		messages = append(messages, openAIChatMessage{Role: "user", Content: modelRequestUserContent(req)})
	}
	if !hasSystem && strings.TrimSpace(p.systemPrompt) != "" {
		messages = append([]openAIChatMessage{{Role: "system", Content: p.systemPrompt}}, messages...)
	}
	return messages
}

func openAIToolsFromSpecs(specs []ToolSpec) []openAITool {
	tools := make([]openAITool, 0, len(specs))
	for _, spec := range specs {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		parameters := clonePayload(spec.Parameters)
		if len(parameters) == 0 {
			parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
		tools = append(tools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        name,
				Description: spec.Description,
				Parameters:  parameters,
			},
		})
	}
	return tools
}

func openAIMessageFromModelMessage(message ModelMessage) openAIChatMessage {
	out := openAIChatMessage{
		Role:       strings.TrimSpace(message.Role),
		Content:    message.Content,
		Name:       strings.TrimSpace(message.Name),
		ToolCallID: strings.TrimSpace(message.ToolCallID),
	}
	if out.Role == "assistant" && len(message.ToolCalls) > 0 {
		out.ToolCalls = openAIToolCallsFromModelToolCalls(message.ToolCalls)
	}
	if out.Role == "tool" && out.ToolCallID == "" && len(message.ToolCalls) > 0 {
		out.ToolCallID = message.ToolCalls[0].ID
	}
	return out
}

func openAIToolCallsFromModelToolCalls(calls []ToolCall) []openAIToolCall {
	out := make([]openAIToolCall, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			continue
		}
		args := "{}"
		if len(call.Arguments) > 0 {
			if encoded, err := json.Marshal(call.Arguments); err == nil {
				args = string(encoded)
			}
		}
		out = append(out, openAIToolCall{
			ID:   call.ID,
			Type: "function",
			Function: openAIToolFunction{
				Name:      name,
				Arguments: args,
			},
		})
	}
	return out
}

func modelRequestUserContent(req ModelRequest) string {
	var b strings.Builder
	if req.Input != "" {
		b.WriteString("User input:\n")
		b.WriteString(req.Input)
		b.WriteString("\n\n")
	}
	if len(req.Observations) > 0 {
		b.WriteString("Tool observations:\n")
		for _, obs := range req.Observations {
			b.WriteString("- ")
			b.WriteString(firstNonEmpty(obs.ToolCallID, obs.Name))
			if obs.ModelVisibleSummary != "" {
				b.WriteString(": ")
				b.WriteString(obs.ModelVisibleSummary)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if len(req.Events) > 0 {
		b.WriteString("Recent runtime events:\n")
		start := len(req.Events) - 8
		if start < 0 {
			start = 0
		}
		for _, event := range req.Events[start:] {
			b.WriteString("- ")
			b.WriteString(string(event.Kind))
			if event.Message != "" {
				b.WriteString(": ")
				b.WriteString(event.Message)
			}
			b.WriteString("\n")
		}
	}
	content := strings.TrimSpace(b.String())
	if content == "" {
		return "Complete the current RTAgent runtime turn."
	}
	return content
}
