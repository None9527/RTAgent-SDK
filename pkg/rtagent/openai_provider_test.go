package rtagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenAICompatibleProviderCompletesTurn(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible-mode/v1/chat/completions" {
			t.Fatalf("path = %q, want /compatible-mode/v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"model":"qwen3.7-plus",
			"choices":[{
				"index":0,
				"finish_reason":"stop",
				"message":{"role":"assistant","content":"provider ok"}
			}],
			"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL + "/compatible-mode/v1",
		APIKey:  "test-key",
		Model:   DefaultDashScopeQwenModel,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}

	response, err := provider.CompleteTurn(context.Background(), ModelRequest{
		Input: "hello",
		Events: []RuntimeEventEnvelope{{
			Kind:    EventKindRunCreated,
			Message: "run created",
		}},
	}, nil)
	if err != nil {
		t.Fatalf("CompleteTurn() error = %v", err)
	}
	if response.Output != "provider ok" {
		t.Fatalf("Output = %q, want provider ok", response.Output)
	}
	if response.StopReason != "stop" {
		t.Fatalf("StopReason = %q, want stop", response.StopReason)
	}
	if captured["model"] != DefaultDashScopeQwenModel {
		t.Fatalf("model = %v, want %q", captured["model"], DefaultDashScopeQwenModel)
	}
	messages, ok := captured["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("messages = %#v, want 2 messages", captured["messages"])
	}
	userMessage := messages[1].(map[string]any)
	if !strings.Contains(userMessage["content"].(string), "hello") {
		t.Fatalf("user message did not include input: %q", userMessage["content"])
	}
}

func TestOpenAICompatibleProviderParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var captured map[string]any
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := captured["tools"].([]any); !ok {
			t.Fatalf("tools missing from request: %#v", captured)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{
				"index":0,
				"finish_reason":"tool_calls",
				"message":{
					"role":"assistant",
					"content":"",
					"tool_calls":[{
						"id":"call-1",
						"type":"function",
						"function":{"name":"echo","arguments":"{\"value\":\"hello\"}"}
					}]
				}
			}]
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}
	response, err := provider.CompleteTurn(context.Background(), ModelRequest{
		Input: "call tool",
		ToolSpecs: []ToolSpec{{
			Name:        "echo",
			Description: "echo a value",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{"type": "string"},
				},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("CompleteTurn() error = %v", err)
	}
	if len(response.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "echo" {
		t.Fatalf("tool name = %q, want echo", response.ToolCalls[0].Name)
	}
	if response.ToolCalls[0].Arguments["value"] != "hello" {
		t.Fatalf("tool arg value = %v, want hello", response.ToolCalls[0].Arguments["value"])
	}
}

func TestOpenAICompatibleProviderUsesModelMessageHistory(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{
				"index":0,
				"finish_reason":"stop",
				"message":{"role":"assistant","content":"history ok"}
			}]
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}
	_, err = provider.CompleteTurn(context.Background(), ModelRequest{
		Messages: []ModelMessage{
			{Role: "system", Content: "custom system"},
			{Role: "user", Content: "first"},
			{Role: "assistant", ToolCalls: []ToolCall{{
				ID:        "call-1",
				Name:      "echo",
				Arguments: map[string]any{"value": "first"},
			}}},
			{Role: "tool", ToolCallID: "call-1", Name: "echo", Content: `{"ok":true}`},
			{Role: "user", Content: "continue"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("CompleteTurn() error = %v", err)
	}
	messages, ok := captured["messages"].([]any)
	if !ok || len(messages) != 5 {
		t.Fatalf("messages = %#v, want complete 5-message history", captured["messages"])
	}
	if messages[0].(map[string]any)["content"] != "custom system" {
		t.Fatalf("first message = %#v, want caller system message", messages[0])
	}
	assistant := messages[2].(map[string]any)
	if _, ok := assistant["tool_calls"].([]any); !ok {
		t.Fatalf("assistant tool_calls missing: %#v", assistant)
	}
	tool := messages[3].(map[string]any)
	if tool["tool_call_id"] != "call-1" {
		t.Fatalf("tool_call_id = %v, want call-1", tool["tool_call_id"])
	}
}

func TestOpenAICompatibleProviderStreamsToolCalls(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		streamChunk(t, w, map[string]any{
			"id":    "chunk-1",
			"model": "qwen3.7-plus",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": "stream "},
			}},
		})
		streamChunk(t, w, map[string]any{
			"id": "chunk-2",
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{
					"tool_calls": []map[string]any{{
						"index": 0,
						"id":    "call-1",
						"type":  "function",
						"function": map[string]any{
							"name":      "echo",
							"arguments": `{"value"`,
						},
					}},
				},
			}},
		})
		streamChunk(t, w, map[string]any{
			"id": "chunk-3",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "tool_calls",
				"delta": map[string]any{
					"tool_calls": []map[string]any{{
						"index": 0,
						"function": map[string]any{
							"arguments": `:"hello"}`,
						},
					}},
				},
			}},
		})
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   DefaultDashScopeQwenModel,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}
	var deltas []ModelStreamEvent
	response, err := provider.CompleteTurn(context.Background(), ModelRequest{Input: "stream"}, func(delta ModelStreamEvent) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteTurn(stream) error = %v", err)
	}
	if captured["stream"] != true {
		t.Fatalf("stream = %v, want true", captured["stream"])
	}
	if response.Output != "stream " {
		t.Fatalf("Output = %q, want streamed content", response.Output)
	}
	if response.StopReason != "tool_calls" {
		t.Fatalf("StopReason = %q, want tool_calls", response.StopReason)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].Name != "echo" || response.ToolCalls[0].Arguments["value"] != "hello" {
		t.Fatalf("ToolCalls = %#v, want aggregated echo call", response.ToolCalls)
	}
	if len(deltas) != 3 {
		t.Fatalf("deltas = %#v, want text and two tool deltas", deltas)
	}
	if deltas[0].Type != ModelStreamEventTextDelta {
		t.Fatalf("deltas[0].Type = %q, want %q", deltas[0].Type, ModelStreamEventTextDelta)
	}
	if deltas[1].Type != ModelStreamEventToolCallDelta || deltas[2].Type != ModelStreamEventToolCallDelta {
		t.Fatalf("tool delta types = %q/%q, want %q", deltas[1].Type, deltas[2].Type, ModelStreamEventToolCallDelta)
	}
}

func TestOpenAICompatibleProviderMapsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"invalid_api_key","message":"bad key"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}
	_, err = provider.CompleteTurn(context.Background(), ModelRequest{Input: "hello"}, nil)
	if err == nil {
		t.Fatalf("CompleteTurn() error = nil, want provider error")
	}
	var modelErr ModelProviderError
	if !errors.As(err, &modelErr) {
		t.Fatalf("error does not implement ModelProviderError")
	}
	details := modelErr.ModelProviderErrorDetails()
	if details.Provider != "openai-compatible" {
		t.Fatalf("Provider = %q, want openai-compatible", details.Provider)
	}
	if details.StatusCode != http.StatusUnauthorized {
		t.Fatalf("details StatusCode = %d, want 401", details.StatusCode)
	}
	if details.Code != "invalid_api_key" {
		t.Fatalf("details Code = %q, want invalid_api_key", details.Code)
	}
	if details.Message != "bad key" {
		t.Fatalf("details Message = %q, want bad key", details.Message)
	}
}

func TestOpenAIProviderSurfacesRetryableFlagsWithoutRetrying(t *testing.T) {
	// v1 contract: the SDK does not retry provider failures. A 429 must be
	// returned as-is with Retryable/RateLimited classification hints, and the
	// provider must issue exactly one HTTP request (no retry loop). See
	// docs/api/model-providers.md Retry and Failure Semantics.
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":"Throttling","message":"rate limited"}}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider() error = %v", err)
	}

	_, err = provider.CompleteTurn(context.Background(), ModelRequest{Input: "hello"}, nil)
	if err == nil {
		t.Fatalf("CompleteTurn() error = nil, want provider error for 429")
	}
	var modelErr ModelProviderError
	if !errors.As(err, &modelErr) {
		t.Fatalf("error does not implement ModelProviderError")
	}
	details := modelErr.ModelProviderErrorDetails()
	if details.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want 429", details.StatusCode)
	}
	if !details.Retryable {
		t.Fatalf("Retryable = false, want true for 429")
	}
	if !details.RateLimited {
		t.Fatalf("RateLimited = false, want true for 429")
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("provider issued %d requests, want exactly 1 (SDK must not retry)", got)
	}
}

func streamChunk(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stream chunk: %v", err)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", bytes)
}

func TestDashScopeQwen37PlusIntegration(t *testing.T) {
	if os.Getenv("RTAGENT_RUN_DASHSCOPE_INTEGRATION") != "1" {
		t.Skip("set RTAGENT_RUN_DASHSCOPE_INTEGRATION=1 to run DashScope integration test")
	}
	if os.Getenv("DASHSCOPE_API_KEY") == "" {
		t.Skip("DASHSCOPE_API_KEY is not set")
	}
	provider, err := NewDashScopeQwen37PlusProviderFromEnv()
	if err != nil {
		t.Fatalf("NewDashScopeQwen37PlusProviderFromEnv() error = %v", err)
	}
	tmp := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	rt, err := Open(ctx, Config{
		Runtime: RuntimeConfig{
			DSN:     filepath.Join(tmp, "rtagent.db"),
			WorkDir: tmp,
		},
		Host: HostPorts{Model: provider},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := rt.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	projection, err := rt.SubmitRun(ctx, SubmitRunRequest{
		RunID:     "run-dashscope-qwen37-plus",
		SessionID: "session-dashscope",
		Input:     "请用一句中文回复：RTAgent SDK provider 测试成功。",
	}, Identity{ActorID: "integration-test"})
	if err != nil {
		t.Fatalf("SubmitRun() error = %v", err)
	}
	if projection.Status != RuntimeStatusCompleted {
		t.Fatalf("Status = %q, want %q", projection.Status, RuntimeStatusCompleted)
	}
	if strings.TrimSpace(projection.Output) == "" {
		t.Fatalf("Output is empty")
	}
}
