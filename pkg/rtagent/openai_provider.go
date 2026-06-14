package rtagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DefaultOpenAICompatibleBaseURL    = "https://api.openai.com/v1"
	DefaultDashScopeCompatibleBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	DefaultDashScopeQwenModel         = "qwen3.7-plus"
)

type OpenAICompatibleProviderConfig struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string
	HTTPClient   *http.Client
	Timeout      time.Duration
	Headers      map[string]string
	ExtraBody    map[string]any
	// ContextWindowTokens is the model's total context window (input + output)
	// in tokens. When set, the provider declares it via Capabilities() so the
	// kernel can derive a context-message budget automatically. 0 = unknown
	// (the kernel falls back to explicit config or no trimming).
	ContextWindowTokens int
	// MaxOutputTokens is the model's per-turn output token limit, if known.
	// Declared via Capabilities(); reserved for future output budgeting.
	MaxOutputTokens int
}

type OpenAICompatibleProvider struct {
	baseURL             string
	apiKey              string
	model               string
	systemPrompt        string
	httpClient          *http.Client
	headers             map[string]string
	extraBody           map[string]any
	contextWindowTokens int
	maxOutputTokens     int
}

type openAICompatibleProviderError struct {
	Provider     string
	StatusCode   int
	Code         string
	Message      string
	Retryable    bool
	RateLimited  bool
	SafeForModel bool
	BodyPreview  string
	Body         string
}

func (e *openAICompatibleProviderError) ModelProviderErrorDetails() ModelProviderErrorDetails {
	if e == nil {
		return ModelProviderErrorDetails{}
	}
	return ModelProviderErrorDetails{
		Provider:     e.Provider,
		StatusCode:   e.StatusCode,
		Code:         e.Code,
		Message:      e.Message,
		Retryable:    e.Retryable,
		RateLimited:  e.RateLimited,
		SafeForModel: e.SafeForModel,
		BodyPreview:  e.BodyPreview,
	}
}

func (e *openAICompatibleProviderError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("openai-compatible provider error: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openai-compatible provider error: status=%d message=%s", e.StatusCode, e.Message)
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleProviderConfig) (*OpenAICompatibleProvider, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultOpenAICompatibleBaseURL
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid openai-compatible base url: %w", err)
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("openai-compatible api key is required")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, errors.New("openai-compatible model is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	systemPrompt := strings.TrimSpace(cfg.SystemPrompt)
	if systemPrompt == "" {
		systemPrompt = "You are the model provider for RTAgent. Complete the user's runtime turn. If tools are available and needed, request tool calls using the provided tool schemas."
	}
	return &OpenAICompatibleProvider{
		baseURL:             baseURL,
		apiKey:              apiKey,
		model:               model,
		systemPrompt:        systemPrompt,
		httpClient:          httpClient,
		headers:             cloneStringMap(cfg.Headers),
		extraBody:           clonePayload(cfg.ExtraBody),
		contextWindowTokens: cfg.ContextWindowTokens,
		maxOutputTokens:     cfg.MaxOutputTokens,
	}, nil
}

// Capabilities declares the provider's model capabilities. It implements
// ModelCapabilityProvider so the kernel can derive a context-message budget
// from ContextWindowTokens when the host does not set an explicit
// RuntimeConfig.MaxContextMessages.
func (p *OpenAICompatibleProvider) Capabilities() ModelCapabilities {
	return ModelCapabilities{
		ContextWindowTokens: p.contextWindowTokens,
		MaxOutputTokens:     p.maxOutputTokens,
		SupportsStreaming:   true,
	}
}

func NewDashScopeOpenAICompatibleProviderFromEnv(model string) (*OpenAICompatibleProvider, error) {
	if strings.TrimSpace(model) == "" {
		model = firstNonEmpty(os.Getenv("DASHSCOPE_MODEL"), DefaultDashScopeQwenModel)
	}
	return NewOpenAICompatibleProvider(OpenAICompatibleProviderConfig{
		BaseURL: firstNonEmpty(os.Getenv("DASHSCOPE_BASE_URL"), DefaultDashScopeCompatibleBaseURL),
		APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
		Model:   model,
	})
}

func NewDashScopeQwen37PlusProviderFromEnv() (*OpenAICompatibleProvider, error) {
	return NewDashScopeOpenAICompatibleProviderFromEnv(DefaultDashScopeQwenModel)
}

func (p *OpenAICompatibleProvider) CompleteTurn(ctx context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error) {
	if p == nil {
		return ModelResponse{}, errors.New("openai-compatible provider is nil")
	}
	if stream != nil {
		return p.doStreamingChatCompletion(ctx, req, stream)
	}
	return p.doChatCompletion(ctx, req, false)
}

func (p *OpenAICompatibleProvider) doChatCompletion(ctx context.Context, req ModelRequest, stream bool) (ModelResponse, error) {
	payload := p.buildRequestPayload(req, stream)
	body, err := json.Marshal(payload)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("marshal openai-compatible request: %w", err)
	}
	httpReq, err := p.newHTTPRequest(ctx, body)
	if err != nil {
		return ModelResponse{}, err
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("openai-compatible request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return ModelResponse{}, fmt.Errorf("read openai-compatible response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelResponse{}, parseOpenAIProviderError(resp.StatusCode, respBody)
	}
	return decodeOpenAIChatCompletion(respBody)
}

func (p *OpenAICompatibleProvider) doStreamingChatCompletion(ctx context.Context, req ModelRequest, emit ModelStreamHandler) (ModelResponse, error) {
	payload := p.buildRequestPayload(req, true)
	body, err := json.Marshal(payload)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("marshal openai-compatible streaming request: %w", err)
	}
	httpReq, err := p.newHTTPRequest(ctx, body)
	if err != nil {
		return ModelResponse{}, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("openai-compatible streaming request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		if err != nil {
			return ModelResponse{}, fmt.Errorf("read openai-compatible streaming error: %w", err)
		}
		return ModelResponse{}, parseOpenAIProviderError(resp.StatusCode, respBody)
	}
	return decodeOpenAIChatCompletionStream(resp.Body, emit)
}

func (p *OpenAICompatibleProvider) newHTTPRequest(ctx context.Context, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create openai-compatible request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	for key, value := range p.headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			httpReq.Header.Set(key, value)
		}
	}
	return httpReq, nil
}

func (p *OpenAICompatibleProvider) chatCompletionsURL() string {
	if strings.HasSuffix(p.baseURL, "/chat/completions") {
		return p.baseURL
	}
	return p.baseURL + "/chat/completions"
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
