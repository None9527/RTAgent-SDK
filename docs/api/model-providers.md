# Model Providers

## Read When

- Adding or changing `ModelProvider` implementations.
- Integrating OpenAI-compatible model APIs.
- Running or updating DashScope integration tests.

## Owner

Runtime/SDK owner.

## Update Trigger

- `ModelProvider`, `ModelRequest`, or `ModelResponse` changes.
- OpenAI-compatible request/response mapping changes.
- DashScope endpoint, model, or environment configuration changes.

## Validation

- `go test ./...`
- `go vet ./...`
- Optional real provider test: `RTAGENT_RUN_DASHSCOPE_INTEGRATION=1 go test ./pkg/rtagent -run TestDashScopeQwen37PlusIntegration -count=1 -v`

## Contract

`ModelProvider` is the model-turn port used by the runnable SDK loop. Hosts inject it through `Config.Host.Model`; if omitted, the SDK uses a deterministic local placeholder so the loop can run without credentials.

`ModelProvider.CompleteTurn(ctx, req, stream)` is the single provider contract. The final return value is always a complete `ModelResponse`. The optional `stream` handler is used for SSE/model deltas:

- `stream == nil`: non-streaming call.
- `stream != nil`: providers that support streaming emit `ModelStreamEvent` deltas and still return the final `ModelResponse`.
- Providers that do not support streaming can ignore `stream` and return the final response.
- Stable stream event types are `ModelStreamEventTextDelta` (`text_delta`) and `ModelStreamEventToolCallDelta` (`tool_call_delta`). The core loop persists these as `model.delta` journal events with `delta_type`, text, tool call, argument delta, and metadata payload fields.

`ModelProviderFunc` adapts a function to `ModelProvider` for simple host integrations and tests. It uses the same `CompleteTurn` contract and does not change runtime loop behavior.

`OpenAICompatibleProvider` implements `ModelProvider` over Chat Completions-compatible HTTP APIs:

- Request endpoint: `{base_url}/chat/completions`
- Auth: `Authorization: Bearer <api_key>`
- Input mapping: `ModelRequest.Messages` becomes standard `system/user/assistant/tool` Chat Completions history. If no messages are supplied, the provider falls back to a synthesized user message from input, recent events, and observations.
- Tool mapping: `ToolSpec` becomes OpenAI-compatible function tools; assistant `ModelMessage.ToolCalls` becomes assistant `tool_calls`; tool observations become `tool` messages through the core loop history.
- Output mapping: assistant content becomes `ModelResponse.Output`; function tool calls become `ModelResponse.ToolCalls`; usage maps to `ModelResponse.Usage`.
- Streaming mapping: OpenAI-compatible SSE `delta.content` emits `ModelStreamEventTextDelta`; SSE `delta.tool_calls` emits `ModelStreamEventToolCallDelta` and is aggregated into final `ModelResponse.ToolCalls`.
- Error mapping: providers can implement `ModelProviderError` to expose `ModelProviderErrorDetails` with provider, status, code, retry/rate-limit flags, safe-for-model flag, message, and body preview.
- OpenAI-compatible errors: non-2xx responses implement `ModelProviderError`; hosts should read `ModelProviderErrorDetails` instead of depending on provider-specific concrete error types.
- Core-loop error projection: when a provider error fails a runtime turn, the SDK keeps `RuntimeError.Code` as the SDK failure code such as `model_turn_failed`, and copies `ModelProviderErrorDetails` into additive `RuntimeError` fields: `Provider`, `StatusCode`, `ProviderCode`, `Retryable`, `RateLimited`, `SafeForModel`, and `BodyPreview`. The same fields are also emitted on the `turn.failed` event payload so hosts can drive retry/backoff UI without parsing error strings.

## Retry and Failure Semantics

The SDK does not retry model provider failures. This is a v1 stable contract, not an omission:

- `CompleteTurn` issues a single provider call. On a non-2xx response, the OpenAI-compatible provider maps it to a `ModelProviderError` and returns immediately — there is no retry loop, attempt counter, backoff, or `Retry-After` honoring anywhere in the SDK model path.
- `ModelProviderErrorDetails.Retryable` and `RateLimited` are classification hints for the host, not SDK actions. The OpenAI-compatible provider sets `Retryable=true` for HTTP 429 and any 5xx, and `RateLimited=true` for HTTP 429 specifically.
- Hosts own retry policy. Because provider failures surface as structured `ModelProviderErrorDetails` on both the returned `RuntimeError` and the `turn.failed` event payload, hosts can decide whether and how to retry (exponential backoff, jitter, budget) without parsing error strings or re-deriving the rate-limit classification.
- A failed turn transitions the run to terminal `failed`; the SDK does not auto-resubmit. Hosts that want to retry submit a new run.

This is regression-covered by `TestOpenAIProviderSurfacesRetryableFlagsWithoutRetrying`, which asserts a 429 response is returned as-is (no second request) with `Retryable=true` and `RateLimited=true`.

## Context Budget

The SDK loop accumulates conversation messages across iterations (user input, assistant turns, tool observations). Without a bound, a long multi-tool run grows the message history until it overflows the model's context window. `RuntimeConfig.MaxContextMessages` prevents this.

- **Message-count window, not token budget.** The kernel intentionally avoids tokenizer coupling. The window counts messages, so it is imprecise but predictable and zero-dependency. Hosts should set it based on their model's context window and typical message size (e.g. 50 for a model with a generous window and compact tool outputs).
- **Opt-in.** The default is `0` = no trimming; existing behavior is unchanged. Hosts running real models set `MaxContextMessages` explicitly.
- **Trimming policy.** When the conversation exceeds the window, the loop keeps the first `role:"user"` message (the task context — losing it would make the model forget the objective) plus the most recent `MaxContextMessages-1` messages. Older middle messages are dropped.
- **Irreversible and resume-visible.** Trimmed messages are gone from the loop state, so a checkpoint taken after trimming restores the trimmed window, not the full history. This is the correct trade-off for long conversations: the model sees the current window, consistent with what it saw before the run suspended.
- **Observability.** Each trim emits a `context.compacted` event with `before_count`, `after_count`, `dropped_count`, and `window_limit`, so hosts can monitor context pressure.
- **Iteration budget.** Separately, `RuntimeConfig.MaxToolIterations` (default 32) caps the number of tool-call rounds. Together these two bounds prevent both unbounded looping and unbounded context growth.

This is regression-covered by `TestTrimMessagesToWindow*` (pure function) and `TestRuntimeLoopTrimsMessagesToConfiguredWindow` (end-to-end).

## DashScope

DashScope OpenAI-compatible mode is configured through:

- `DASHSCOPE_API_KEY`: required for real integration.
- `DASHSCOPE_BASE_URL`: optional region override. Default is `https://dashscope.aliyuncs.com/compatible-mode/v1`.
- `DASHSCOPE_MODEL`: optional model override. Default helper uses `qwen3.7-plus`.

Use `NewDashScopeQwen37PlusProviderFromEnv()` for the current SDK integration test structure.

## Known Gaps

- Tool provider registry and tool schema snapshot persistence are implemented. OpenAI-compatible function calls are bound to the current context packet schema before execution, but ordinary function-call APIs still cannot prove which schema version the model actually saw unless the provider returns explicit schema hash/epoch metadata.

## Evidence

- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_config.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_model.go`
- `pkg/rtagent/types_tool.go`
- `pkg/rtagent/model_history.go`
- `pkg/rtagent/openai_provider.go`
- `pkg/rtagent/openai_provider_types.go`
- `pkg/rtagent/openai_provider_messages.go`
- `pkg/rtagent/openai_provider_decode.go`
- `pkg/rtagent/openai_provider_errors.go`
- `pkg/rtagent/openai_provider_test.go`
- `pkg/rtagent/loop.go`
- `pkg/rtagent/loop_context.go`
- `pkg/rtagent/loop_outcomes.go`
