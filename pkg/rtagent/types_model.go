package rtagent

import "time"

type ContextPacket struct {
	ID                   string                 `json:"id"`
	RunID                string                 `json:"run_id"`
	SessionID            string                 `json:"session_id,omitempty"`
	Scope                ExecutionScope         `json:"scope"`
	Input                string                 `json:"input,omitempty"`
	Payload              map[string]any         `json:"payload,omitempty"`
	Events               []RuntimeEventEnvelope `json:"events,omitempty"`
	ToolSpecs            []ToolSpec             `json:"tool_specs,omitempty"`
	ToolSchemaSnapshotID string                 `json:"tool_schema_snapshot_id,omitempty"`
	ToolSchemaHash       string                 `json:"tool_schema_hash,omitempty"`
	GeneratedAt          time.Time              `json:"generated_at,omitempty"`
}

type ModelRequest struct {
	Scope        ExecutionScope         `json:"scope"`
	Input        string                 `json:"input,omitempty"`
	Context      ContextPacket          `json:"context"`
	Messages     []ModelMessage         `json:"messages,omitempty"`
	ToolSpecs    []ToolSpec             `json:"tool_specs,omitempty"`
	Observations []ToolObservation      `json:"observations,omitempty"`
	Iteration    int                    `json:"iteration"`
	Metadata     map[string]any         `json:"metadata,omitempty"`
	Events       []RuntimeEventEnvelope `json:"events,omitempty"`
}

type ModelResponse struct {
	Output          string           `json:"output,omitempty"`
	ToolCalls       []ToolCall       `json:"tool_calls,omitempty"`
	ApprovalRequest *ApprovalRequest `json:"approval_request,omitempty"`
	PlanArtifact    *PlanArtifact    `json:"plan_artifact,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	StopReason      string           `json:"stop_reason,omitempty"`
	Usage           *ModelUsage      `json:"usage,omitempty"`
}

type ModelMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	Name       string         `json:"name,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ModelUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type ModelStreamEvent struct {
	Type           string         `json:"type"`
	Text           string         `json:"text,omitempty"`
	ToolCallIndex  int            `json:"tool_call_index,omitempty"`
	ToolCallID     string         `json:"tool_call_id,omitempty"`
	ToolName       string         `json:"tool_name,omitempty"`
	ArgumentsDelta string         `json:"arguments_delta,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ModelStreamHandler func(ModelStreamEvent) error

const (
	ModelStreamEventTextDelta     = "text_delta"
	ModelStreamEventToolCallDelta = "tool_call_delta"
)

type ModelProviderError interface {
	error
	ModelProviderErrorDetails() ModelProviderErrorDetails
}

type ModelProviderErrorDetails struct {
	Provider     string `json:"provider,omitempty"`
	StatusCode   int    `json:"status_code,omitempty"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
	Retryable    bool   `json:"retryable,omitempty"`
	RateLimited  bool   `json:"rate_limited,omitempty"`
	SafeForModel bool   `json:"safe_for_model,omitempty"`
	BodyPreview  string `json:"body_preview,omitempty"`
}
