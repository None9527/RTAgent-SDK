# Tool Providers

## Read When

- Adding or changing SDK tool provider contracts.
- Registering multiple tool providers for the core loop.
- Debugging tool schema lookup, namespaced duplicate tool names, or tool dispatch behavior.

## Owner

Runtime/SDK owner.

## Update Trigger

- `ToolProvider`, `ToolRegistry`, `ToolSpec`, `ToolCall`, or tool execution dispatch behavior changes.
- Runtime config for tool providers changes.
- Tool schema snapshot, schema hash, epoch validation, or provider registry persistence changes.

## Validation

- `go test ./...`
- `go vet ./...`

## Contract

`ToolProvider` is the runtime SDK tool extension port:

- `ToolSpecs(ctx, scope)` returns visible tool schemas for the current execution scope.
- `ExecuteTool(ctx, scope, call)` executes a named tool call and returns a `ToolObservation`.

Hosts inject runtime tools through `Config.Host.Tools`. A single provider is represented as a one-item slice; multiple providers are composed into an internal `ToolRegistry`. Hosts can also create `NewToolRegistry(...)` directly when they need dynamic registration.

`ToolProviderAdapter` adapts host functions to `ToolProvider` for small integrations and examples. It still routes execution through the normal tool loop, PermissionCenter, schema snapshot binding, and observation events. A nil `Specs` function behaves as an empty inventory, while a nil `Execute` function returns a stable adapter error instead of panicking.

MCP and Skill inventory are separate read-side projection ports: `MCPProvider` and `SkillProvider` expose visible/available/authorized inventory to WorldState, but they do not execute actions. If an MCP server or skill should be callable by the loop, the host must also expose it through `ToolProvider`/`ToolRegistry` so PermissionCenter and tool observations remain canonical.

## ToolRegistry

`ToolRegistry` is a `ToolProvider` implementation that composes multiple providers:

- `NewToolRegistry(providers...)` creates a registry and ignores nil providers.
- `Register(provider)` appends a provider at runtime and rejects nil providers.
- `ToolSpecs` merges schemas from all providers. Unique short names stay unchanged.
- Duplicate short names are exposed as `namespace__name`, where namespace is derived from `ToolSpec.Namespace`, then `ToolSpec.ProviderName`, then provider order.
- `ExecuteTool` resolves by exposed `ToolCall.Name`; namespaced calls are dispatched to the matching provider with the original provider-local tool name.
- Calling a duplicated short name without namespace returns an ambiguous-tool error listing the valid namespaced choices.

The core loop still depends on one `ToolProvider` internally. Runtime configuration composes `Config.Host.Tools` into that single internal provider when more than one provider is supplied.

## Schema Snapshot And Validation

The core loop persists a tool schema snapshot when it builds a `ContextPacket` with tools:

- `ToolSpec.SchemaHash` is filled when the provider did not supply one.
- `ToolSpec.Epoch` defaults to `ToolSpec.Version`, then to `ToolSpec.SchemaHash`.
- `ContextPacket.ToolSchemaSnapshotID` and `ContextPacket.ToolSchemaHash` identify the persisted snapshot.
- The `context.packet.created` event includes the snapshot id and hash for audit.
- Before permission checks or execution, the core loop binds missing `ToolCall.SchemaHash` and `ToolCall.Epoch` from the current `ToolSpec`. Tool providers receive the bound call, and approval continuations persist the same metadata for resume.
- `ToolCall.SchemaHash` and `ToolCall.Epoch` are validated when present.
- Hash or epoch mismatch fails the run before tool execution.

`ToolSpec.ExecutionConstraints` describes host-visible execution constraints such as profile id, file scope, and network expectation. It is metadata for host policy/projection and does not imply that the SDK ships a sandbox executor; side-effect control remains `PermissionCenter` plus the host's `ToolProvider` implementation.

## Current Limits

- Namespaced duplicate routing is deterministic by provider order. Hosts should still prefer explicit `ToolSpec.Namespace` for stable public tool names.
- OpenAI-compatible function calls do not naturally prove which schema the model saw. The SDK binds calls to the current context packet schema before execution, but hosts that need stronger stale-model detection should use providers that return explicit schema hash/epoch metadata.

## Evidence

- `pkg/rtagent/tool_registry.go`
- `pkg/rtagent/provider_adapters.go`
- `pkg/rtagent/tool_schema.go`
- `pkg/rtagent/tool_registry_test.go`
- `pkg/rtagent/tool_schema_test.go`
- `pkg/rtagent/runtime.go`
- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_config.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_tool.go`
- `pkg/rtagent/types_permission.go`
- `pkg/rtagent/loop.go`
- `pkg/rtagent/loop_context.go`
- `pkg/rtagent/loop_tools.go`
- `internal/domain/persistence/records_governance.go`
- `internal/domain/persistence/stores.go`
- `internal/infrastructure/persistence/sqlite/adapters/models.go`
- `internal/infrastructure/persistence/sqlite/adapters/permission_governance_store.go`
