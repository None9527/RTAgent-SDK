# World State

## Read When

- Changing `WorldStateSnapshot`, `WorldStatePartition`, `WorldStateEntry`, `CapabilityState`, or `WorldStateHandle`.
- Debugging `Runtime.WorldState`, `Inspect`, context packet projection, or tool capability visibility.
- Deciding how tool, MCP, skill, permission, context, activity, task, memory, or hypothesis state should enter the SDK read model.

## Owner

Runtime/SDK owner.

## Update Trigger

- WorldState public schema or partition behavior changes.
- Runtime fact sources for WorldState change.
- Tool schema snapshot, context packet, permission, activity, task, memory, MCP, or skill projection behavior changes.

## Validation

- `go test ./pkg/rtagent -run TestRuntimeWorldState -count=1`
- `go test ./...`
- `go vet ./...`

## Contract

WorldState is the SDK read-side operational projection of runtime facts. It is not the truth store and does not directly own execution state. Runtime journal events and typed stores remain the facts; WorldState is the governed, typed, source-watermarked view used by hosts, inspect surfaces, and future planning loops to decide the next action.

WorldState is run-scoped and requires an existing run id. A missing run is an error, not an empty snapshot, because the SDK should not fabricate operational state outside the run/session graph.

The public v1 shape is additive over the original flat snapshot:

- `WorldStateSnapshot` includes `SnapshotID`, `BuildID`, `RuntimeEpoch`, `SourceWatermark`, `BuiltAt`, `Partitions`, `Handles`, `Warnings`, and legacy `Entries`.
- `WorldStatePartition` groups entries and handles by source-backed partition.
- `WorldStateEntry` carries `StateOrPredicate`, `Source`, `Authority`, `ObservedAt`, evidence refs, optional `Capability`, and metadata.
- `CapabilityState` describes visible/available/authorized tool capability state.
- `WorldStateHandle` is a reloadable reference into context, tool, or other partition-backed sources.

Legacy `Entries` remain a compatibility view of the old internal builder output. New code should prefer `Partitions` and `Handles`.

## Partitions

The SDK exposes the same core partition vocabulary derived from ngoagent:

- `memory`: committed durable memory projection from runtime memory records and optional host `MemoryProvider`.
- `capability`: available tools plus optional MCP/skill inventory projections.
- `activity`: active or completed runtime/tool activity.
- `task`: recent runtime trajectory and task state.
- `context`: current workspace, context packets, and registered context handles.
- `governance`: permission snapshot, approval state, active grants, denied decisions, and resource rules.
- `hypothesis`: proposed memory and optional host `HypothesisProvider` projections.
- `artifact`: RTAgent-specific compatibility partition for workspace artifacts.

## Current Projection Sources

Current RTAgent builds WorldState from:

- Runtime event journal through `ListEvents`.
- The legacy in-memory/internal WorldState builder for compatibility entries.
- Persisted tool schema snapshots referenced by `context.packet.created`.
- Registered SDK context handles from the context registry. Context handle registration is run-scoped and rejects unknown run ids before registry mutation, so projected handles cannot become orphan WorldState facts.
- Runtime memory records listed by run id, with proposed memory separated into the `hypothesis` partition.
- Host `MemoryProvider` and `HypothesisProvider` projection ports.
- PermissionSnapshot, including effective active grants, pending decisions, denied decisions, resource rules, warnings, and policy hash.
- Permission grants matched through the same deterministic grant logic used by `PermissionCenter`.
- Host `MCPProvider` and `SkillProvider` inventory ports. They project visibility/availability/authorization only; execution still goes through `ToolProvider`/`ToolRegistry`.
- Optional custom `WorldStateProvider` implementations that can contribute partition entries and handles.

Hosts can implement those projection ports directly or use `MemoryProviderFunc`, `HypothesisProviderFunc`, `MCPProviderFunc`, `SkillProviderFunc`, and `WorldStateProviderAdapter` for small integrations. These adapters only feed the read-side projection; they do not create a new execution path. Nil projection function adapters return stable adapter errors instead of panicking; `WorldStateProviderAdapter` preserves partition/provider metadata even when its `Build` function is missing.

The capability partition currently uses the latest persisted tool schema snapshot for the run. Each `ToolSpec` becomes:

- A `capability:<tool>` entry.
- A `tool:<tool>` handle.
- A `CapabilityState` with `visible=true`, `available=true`, and `authorized=true` when the tool is read-only, the run permission mode is `yolo`, or a matching effective active grant exists.

Stopped or stopping sessions invalidate non-read-only capability authorization in the projection. Historical grant events remain in the journal, but `PermissionSnapshot.ActiveGrants` and `CapabilityState.MatchedGrantID` do not expose them as current authorization; `AuthorizationReason` explains that the session lifecycle blocks the capability.

Governance projection uses the same effective `PermissionSnapshot` as `Inspect`. Pending approval entries expose `available_choices` after session-lifecycle filtering, so a draining or stopped session shows deny-only choices instead of allow options that runtime resume will reject.

The memory partition projects non-invalidated committed memory records associated with the run. Rejected and superseded memories are skipped because WorldState is the current operational projection, not historical storage. Proposed memory is intentionally projected into `hypothesis`, not `memory`.

MCP and skill are treated as capability inventory sources. They are not separate truth owners and do not execute through WorldState. Host inventory providers expose source-watermarked visibility and authorization state; actual execution remains gated by `ToolProvider` and `PermissionCenter`.

## Read/Write Separation

WorldState write behavior is intentionally absent. Runtime facts are written through `SubmitRun`, `Run`, `Emit`, tool execution, permission resolution, context registration, and workspace APIs. Run-scoped writers such as `Emit`, context registration, non-read-only permission checks, and permission decisions validate the run before persisting derived facts. `WorldState(ctx, query)` rebuilds a view from those sources and is safe to call repeatedly.

`WorldStateQuery.Partition` filters the returned partition view by exact partition name. When set, `Partitions` contains only matching typed partitions, top-level `Handles` contains only handles from those partitions, and legacy compatibility `Entries` is narrowed to entries with the same `Partition` value. The legacy `Entries` field is still compatibility-oriented and should not be used as the complete v1 read model.

## Current Limits

- Permission-aware capability authorization is run/session grant-aware and session-lifecycle-aware. It includes authorization reason, matched grant id/scope, required grants, and policy hash. Host-specific policy explanation can still be richer at the UI layer.
- Context packet projection is summary-level; full materialization still goes through context handle APIs.

## Evidence

- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_world_state.go`
- `pkg/rtagent/types_permission.go`
- `pkg/rtagent/types_tool.go`
- `pkg/rtagent/provider_adapters.go`
- `pkg/rtagent/runtime.go`
- `pkg/rtagent/world_state.go`
- `pkg/rtagent/world_state_context.go`
- `pkg/rtagent/world_state_legacy.go`
- `pkg/rtagent/world_state_helpers.go`
- `pkg/rtagent/world_state_memory.go`
- `pkg/rtagent/world_state_capability.go`
- `pkg/rtagent/world_state_external.go`
- `pkg/rtagent/world_state_runtime.go`
- `pkg/rtagent/world_state_test.go`
- `pkg/rtagent/tool_schema.go`
- `pkg/rtagent/loop.go`
- `pkg/rtagent/loop_context.go`
- `pkg/rtagent/loop_activity.go`
- `pkg/rtagent/loop_tools.go`
- `internal/domain/persistence/records_memory.go`
- `internal/domain/persistence/records_runtime.go`
- `internal/domain/persistence/stores.go`
- `internal/domain/worldstate/worldstate.go`
- `internal/infrastructure/persistence/sqlite/adapters/checkpoint_event_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/memory_artifact_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/activity_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/permission_governance_store.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/product_contract.go`
- `/Users/mac/Desktop/ngoagent/internal/worldstate/builder.go`
- `/Users/mac/Desktop/ngoagent/internal/worldstate/capability_provider.go`
- `/Users/mac/Desktop/ngoagent/internal/worldstate/context_provider.go`
- `/Users/mac/Desktop/ngoagent/docs/2026-06-06-world-state-runtime-contract.md`
