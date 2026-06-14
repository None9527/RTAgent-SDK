package rtagent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

func decodeOpenAIChatCompletion(body []byte) (ModelResponse, error) {
	var decoded openAIChatCompletionResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ModelResponse{}, fmt.Errorf("decode openai-compatible response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return ModelResponse{}, errors.New("openai-compatible response has no choices")
	}
	choice := decoded.Choices[0]
	toolCalls, err := decodeOpenAIToolCalls(choice.Message.ToolCalls)
	if err != nil {
		return ModelResponse{}, err
	}
	metadata := map[string]any{
		"id":     decoded.ID,
		"model":  decoded.Model,
		"usage":  decoded.Usage,
		"role":   choice.Message.Role,
		"choice": choice.Index,
	}
	return ModelResponse{
		Output:     openAIContentText(choice.Message.Content),
		ToolCalls:  toolCalls,
		Metadata:   metadata,
		StopReason: choice.FinishReason,
		Usage:      openAIUsage(decoded.Usage),
	}, nil
}

func decodeOpenAIChatCompletionStream(body io.Reader, emit ModelStreamHandler) (ModelResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	var content strings.Builder
	toolCalls := map[int]*openAIToolCall{}
	metadata := map[string]any{}
	stopReason := ""
	var usage *ModelUsage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk openAIChatCompletionResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return ModelResponse{}, fmt.Errorf("decode openai-compatible stream chunk: %w", err)
		}
		if chunk.ID != "" {
			metadata["id"] = chunk.ID
		}
		if chunk.Model != "" {
			metadata["model"] = chunk.Model
		}
		if chunk.Usage != nil {
			metadata["usage"] = chunk.Usage
			usage = openAIUsage(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				stopReason = choice.FinishReason
			}
			if text := openAIContentText(choice.Delta.Content); text != "" {
				content.WriteString(text)
				if emit != nil {
					if err := emit(ModelStreamEvent{Type: ModelStreamEventTextDelta, Text: text, Metadata: map[string]any{"choice": choice.Index}}); err != nil {
						return ModelResponse{}, err
					}
				}
			}
			for _, delta := range choice.Delta.ToolCalls {
				index := delta.Index
				current := toolCalls[index]
				if current == nil {
					current = &openAIToolCall{Type: "function"}
					toolCalls[index] = current
				}
				if delta.ID != "" {
					current.ID = delta.ID
				}
				if delta.Type != "" {
					current.Type = delta.Type
				}
				if delta.Function.Name != "" {
					current.Function.Name += delta.Function.Name
				}
				if delta.Function.Arguments != "" {
					current.Function.Arguments += delta.Function.Arguments
				}
				if emit != nil {
					if err := emit(ModelStreamEvent{
						Type:           ModelStreamEventToolCallDelta,
						ToolCallIndex:  index,
						ToolCallID:     current.ID,
						ToolName:       current.Function.Name,
						ArgumentsDelta: delta.Function.Arguments,
					}); err != nil {
						return ModelResponse{}, err
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ModelResponse{}, fmt.Errorf("read openai-compatible stream: %w", err)
	}
	ordered := make([]openAIToolCall, 0, len(toolCalls))
	indexes := make([]int, 0, len(toolCalls))
	for index := range toolCalls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		ordered = append(ordered, *toolCalls[index])
	}
	decodedToolCalls, err := decodeOpenAIToolCalls(ordered)
	if err != nil {
		return ModelResponse{}, err
	}
	return ModelResponse{
		Output:     content.String(),
		ToolCalls:  decodedToolCalls,
		Metadata:   metadata,
		StopReason: stopReason,
		Usage:      usage,
	}, nil
}

func decodeOpenAIToolCalls(calls []openAIToolCall) ([]ToolCall, error) {
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Function.Name)
		if name == "" {
			return nil, errors.New("openai-compatible tool call missing function name")
		}
		args := map[string]any{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("decode tool call arguments for %s: %w", name, err)
			}
		}
		out = append(out, ToolCall{
			ID:        call.ID,
			Name:      name,
			Arguments: args,
		})
	}
	return out, nil
}

func openAIContentText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if obj, ok := item.(map[string]any); ok {
				if text, ok := obj["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func openAIUsage(usage map[string]any) *ModelUsage {
	if len(usage) == 0 {
		return nil
	}
	return &ModelUsage{
		PromptTokens:     openAIUsageInt(usage["prompt_tokens"]),
		CompletionTokens: openAIUsageInt(usage["completion_tokens"]),
		TotalTokens:      openAIUsageInt(usage["total_tokens"]),
	}
}

func openAIUsageInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		var n int
		_, _ = fmt.Sscanf(fmt.Sprint(typed), "%d", &n)
		return n
	}
}
