# SDK Core Interface

## Read When

- Planning or changing `pkg/rtagent`.
- Embedding RTAgent in another Go process.
- Reviewing runtime command, event, world state, context, workspace, or governance contracts.

## Owner

Runtime/SDK owner.

## Update Trigger

- Public SDK types or methods change.
- Runtime event schema or WorldState schema changes.
- Persistence, context materialization, workspace write, or governance facade behavior changes.

## Validation

- `go test ./...`
- `go vet ./...`

## Current Facade

`pkg/rtagent` is the first public Runtime SDK boundary. The public shape follows the current ngoagent source split: app command surface first, runner/journal/projection/tool ports behind it. Internal startup wiring is confined to an unexported `runtimeKernel` bootstrap layer so the facade does not hold or expose the startup container. Inside that layer, the facade depends on narrow unexported kernel ports instead of concrete GORM, aggregate persistence `Bundle`, context engine, workspace, governance, or WorldState builder types. The SDK extracts reusable runtime capabilities from ngoagent project experience without copying ngoagent's whole product surface.

The SDK target is a runnable runtime core loop, not a facade-only interface package. Public interfaces are kept for host-provided ports such as model, tool, memory, hypothesis, MCP, skill, WorldState, and PermissionCenter. Runtime facade method groupings are intentionally not exported as convenience interfaces because host applications can define narrower local interfaces around the exact methods they consume.

- `Open(ctx, Config)` and `Close()` manage the embedded runtime lifecycle. `Close()` is idempotent; public runtime calls after close return `ErrRuntimeClosed`.
- `TestRuntimeCloseMakesPublicAPIsUnavailable` covers the public `Runtime` facade method set so lifecycle regressions surface before v1.
- `runtimeKernel` is an unexported dependency bundle that maps startup-wired store, event bus, lease manager, context engine, workspace, and WorldState builder into narrow SDK-local ports. Its `runtimeStore` includes only the store capabilities consumed by `pkg/rtagent`; importing `internal/startup` is confined to `pkg/rtagent/kernel.go`, and the aggregate `persistence.Bundle` stays in the persistence contract plus `internal/startup` composition.
- `Config.Runtime` owns SDK infrastructure settings such as SQLite DSN, work directory, max tool iterations, and run lease TTL.
- Empty `RuntimeConfig.DSN` uses an isolated in-memory SQLite database so zero-config embedding does not create files in the host working directory. Hosts that need durable runs, session resume, or cross-process inspection must pass an explicit DSN.
- `Config.Host` owns host-provided ports such as model, tool, memory, hypothesis, MCP, skill, and custom WorldState providers.
- `SubmitRun(ctx, SubmitRunRequest, Identity)` is the recommended app-level entry and drives the runnable core loop.
- `Run(ctx, RuntimeCommand)` runs the core loop from a normalized lower-level command.
- `Emit(ctx, RuntimeEventDraft)` appends runtime events with stable schema, sequence, and caller-supplied time; it is a run-scoped journal hook, not the main product API. The target `RunID` must already exist so host-defined events remain attached to the run/session graph.
- `ListEvents(ctx, EventQuery)` returns persisted runtime event envelopes by `RunID` or aggregates all run events in a `SessionID`.
- `Inspect(ctx, InspectQuery)` returns a read-side runtime projection for a run; if only `SessionID` is supplied, it resolves to the session's latest run.
- `InspectSession`, `SessionGraph`, and `StopSession` expose session lifecycle, run graph, root-branch filtering, and session-scoped cancellation/drain behavior.
- `CheckpointGraph` and `ResumeRun` expose run-internal checkpoint graph resume.
- `InterruptRun(ctx, runID)` marks a non-terminal run interrupted/canceled and appends `run.interrupted`; for terminal runs it is an idempotent no-op that returns the existing terminal status with `CancellationBy: "already_terminal"`.
- `WorldState(ctx, WorldStateQuery)` rebuilds and returns a run-scoped derived snapshot.
- `PermissionSnapshot(ctx, PermissionSnapshotQuery)` exposes the same permission projection used by Inspect and WorldState.
- `RegisterContextHandle` and `MaterializeContext` expose context handle registration and materialization.
- `CheckPermission` and `ResolvePermission` expose the SDK-owned permission center for action checks, approval decisions, and grant persistence.
- `ResolveApproval(ctx, approvalID, decision)` is the app-level approval callback for SDK-generated approval requests that should also resume a suspended run.
- `WriteFile` exposes audited workspace writes.
- `EvaluateProposal` exposes governance proposal evaluation.

## Contract Types

- `ExecutionScope`: workspace, session, run, root run, parent run, task, actor, and owner identifiers.
- `Identity`: caller actor/owner context.
- `SubmitRunRequest`: app-level run submission input.
- `RuntimeCommand`: minimal command envelope accepted by the SDK.
- `RuntimeStateProjection`: current run projection returned to SDK callers.
- `RuntimeError`, `ModelProviderError`, and `ModelProviderErrorDetails`: SDK failure code plus additive provider fields for model/provider failures, including provider name, HTTP status, provider code, retry/rate-limit flags, safe-for-model flag, and body preview.
- `SessionSnapshot`, `SessionGraphSnapshot`, and `StopSessionResult`: session lifecycle and run graph read/write contracts.
- `RuntimeEventEnvelope`: auditable event returned from persistence and SDK emits.
- `RuntimeInspectSnapshot`: read-side projection for status, events, and WorldState.
- `WorldStateSnapshot`, `WorldStatePartition`, `WorldStateEntry`, `CapabilityState`, and `WorldStateHandle`: typed, partitioned read model derived from runtime facts.
- `ContextHandle`: external context references that can be materialized on demand.
- `ToolSpec`, `ToolCall`, `ToolObservation`: tool-provider extension contract.
- `RuntimeConfig` and `HostPorts`: the two-part SDK configuration contract separating runtime infrastructure from host integration ports.
- `ErrRuntimeClosed`: stable sentinel returned when a host calls public runtime APIs after `Close`.
- `ToolRegistry`: multi-provider composition contract used by `Config.Host.Tools`.
- `ModelProviderFunc`, `ToolProviderAdapter`, `MemoryProviderFunc`, `HypothesisProviderFunc`, `MCPProviderFunc`, `SkillProviderFunc`, and `WorldStateProviderAdapter`: lightweight host adapter helpers for implementing SDK ports without boilerplate structs.
- `ModelProvider`, `ModelRequest`, `ModelMessage`, `ModelResponse`, `ModelUsage`, `ModelStreamEvent`, `ModelStreamEventTextDelta`, `ModelStreamEventToolCallDelta`, and `ContextPacket`: model-turn, message history, optional SSE streaming, and context packet contract for the runnable core loop.
- `OpenAICompatibleProvider`: HTTP provider for OpenAI Chat Completions-compatible endpoints, including DashScope OpenAI-compatible mode.
- `ApprovalRequest`, `ScopedPermissionGrant`: approval/governance contract types.
- `PermissionCenter`, `PermissionCheckRequest`, `PermissionCheckResult`, `PermissionDecisionRequest`, and `PermissionDecisionResult`: SDK-owned permission contract for default approval, scoped grants, and run/session scoped all-capability grants.
- `MemoryProvider`, `HypothesisProvider`, `MCPProvider`, `SkillProvider`, and `WorldStateProvider`: host projection ports for read-side WorldState inventory/facts. They do not execute tools.
- `WriteFileRequest` and `ArtifactRecord`: workspace mutation boundary.
- `ProposedAction`: governance evaluation input.

## Design Notes

- The runtime journal is the source of truth.
- `SubmitRun` should be preferred by external SDK callers; `Run` is the advanced full-loop entry for hosts that already build `RuntimeCommand`.
- Public request/query structs used by host adapters should serialize with stable snake_case JSON fields; run intake, runtime command/event drafts, event/inspect/session/checkpoint/worldstate/permission queries, approval resume/stop-session requests, and workspace write requests are covered by `TestPublicQueryAndRequestJSONContractUsesSnakeCase`.
- `EventQuery.SessionID` and `InspectQuery.SessionID` are honored runtime filters, not placeholder fields. `ListEvents` rejects unknown run/session ids, `RunID` + `SessionID` queries validate that the run belongs to the session, and session-only `EventQuery.AfterSeq` applies per run because runtime event sequence numbers are run-local.
- Run initialization is internal to the core loop and is not a separate public execution facade.
- `Inspect` should hide read-side projection assembly from callers.
- Run-scoped read projections must not fabricate state for missing runs. `ListEvents`, `Inspect`, `CheckpointGraph`, `PermissionSnapshot`, and `WorldState` reject unknown run ids instead of returning empty snapshots or event lists.
- `CheckpointGraph.ResumeReady` is an effective projection, not just raw checkpoint metadata. Terminal run status and stopped/stopping sessions suppress resume-ready checkpoint nodes and return warnings so host UIs do not offer continuation that `ResumeRun` will reject.
- The SDK does not expose a broad `Core` interface or pre-grouped facade convenience interfaces; hosts should depend on the concrete `Runtime` facade when embedding the whole runtime, or define narrow local interfaces around the exact Runtime methods they consume.
- Function/adapter helpers are optional host ergonomics. They implement the same public ports and do not bypass the runtime loop, PermissionCenter, ToolRegistry, or WorldState projection rules. Nil function adapters return stable errors instead of panicking; `ToolProviderAdapter` treats nil `Specs` as empty inventory and requires `Execute` for calls.
- WorldState is a source-watermarked read model, not a truth store. Runtime events, tool schema snapshots, context registry entries, runtime memories, permissions, activities, tasks, and other typed stores are the fact sources.
- Public `EventKind*` constants are limited to SDK-owned events generated or consumed by the core loop and read-side projections. Host-defined journal events can still be emitted with arbitrary `EventKind` strings, but product-shell event names are not part of the v1 constant surface.
- `Runtime.Emit` rejects unknown run ids before append. Custom host events are allowed only after `SubmitRun`, `Run`, or another SDK-owned path has created the run record.
- `RegisterContextHandle` rejects unknown `ContextHandle.RunID` before mutating the context registry, so context handles projected into WorldState remain anchored to the run/session graph.
- `CheckPermission` and `ResolvePermission` reject unknown scoped run ids before creating permission requests, reviewer decisions, grants, or permission events. Non-read-only permission checks require an existing run because they can authorize SDK side effects.
- WorldState v1 is partitioned into `memory`, `capability`, `activity`, `task`, `context`, `governance`, `hypothesis`, plus RTAgent's compatibility `artifact` partition.
- `WorldStateQuery.Partition` narrows the read-side view by exact partition name across typed `Partitions`, top-level `Handles`, and legacy compatibility `Entries`.
- `WorldStateSnapshot.Entries` remains a compatibility projection from the old flat builder; new callers should use `Partitions` and `Handles`.
- The current capability partition projects the latest persisted tool schema snapshot for the run. Tool specs become `capability:<tool>` entries and `tool:<tool>` handles, with authorization derived from read-only state, `yolo` mode, or matching active permission grants.
- Capability state now includes authorization reason, matched grant id/scope, required grants, and policy hash.
- The memory partition projects non-invalidated committed memory records associated with the run and optional host `MemoryProvider` facts. Proposed memory is projected into `hypothesis`.
- MCP and skill inventories enter WorldState through dedicated `MCPProvider` and `SkillProvider` projection ports, while execution remains through `ToolProvider`/`ToolRegistry`.
- WorldState cache keys include `run_id` so snapshots do not leak across runs.
- SQLite persistence preserves caller-supplied run/event timestamps when provided.
- `Runtime.Emit` serializes automatic event sequence assignment and append inside one `Runtime` instance. SQLite enforces unique `(run_id, sequence)` ordering; shared multi-process writers require a future store-level allocation contract.
- SQLite persistence now round-trips run `CompletedAt`, `LastCheckpoint`, and activity timestamps needed by loop status.
- SDK startup uses a silent GORM logger by default; embedders should observe failures through returned errors, runtime events, and host-provided logging rather than implicit stdout/stderr output from the library.
- Zero-config `Open(ctx, Config{})` uses ephemeral in-memory persistence; persistent SDK hosts should configure `RuntimeConfig.DSN`.
- Custom kernel/store injection remains an unexported SDK seam for v1. Public host extension should use `Config.Host` model, tool, memory, hypothesis, MCP, skill, and WorldState ports rather than depending on internal startup or persistence container types.
- Runtime components below the facade consume narrow store capabilities rather than aggregate `persistence.Bundle`: the facade kernel uses `runtimeStore`, `LocalMaterializer` consumes artifact read, `ManagedWorkspace` consumes artifact store, `LocalLeaseManager` consumes lease store, and the legacy WorldState builder consumes runtime event read. `internal/startup` remains the only composition layer that holds the aggregate store.
- `Close` marks the facade closed before releasing kernel resources so hosts receive a stable SDK-level `ErrRuntimeClosed` instead of lower-level closed DB or event bus errors.
- `SubmitRun` uses a deterministic local default model provider so the loop is runnable without credentials; production hosts should inject a real `ModelProvider` through `Config.Host.Model`.
- `NewOpenAICompatibleProvider` provides a standard Chat Completions-compatible model provider.
- `NewDashScopeQwen37PlusProviderFromEnv` wires DashScope OpenAI-compatible mode using `DASHSCOPE_API_KEY`, optional `DASHSCOPE_BASE_URL`, and model `qwen3.7-plus`.
- Streaming providers should emit stable `ModelStreamEvent` types; the core loop persists them as `model.delta` journal events for host UIs and event streams.
- Provider failures from the core loop preserve structured provider details through the generic `ModelProviderError` contract in `RuntimeStateProjection.Problem` and the `turn.failed` event payload while keeping SDK-level error codes stable.
- The current loop depends on one `ToolProvider` internally, while `Config.Host.Tools` and `NewToolRegistry` let host applications compose multiple providers. Duplicate tool short names are exposed as deterministic `namespace__name` aliases and dispatched back to the provider-local tool name.
- `ToolSpec.ExecutionConstraints` is host policy metadata, not an SDK-bundled sandbox. The old sandbox demo wording is intentionally not part of the v1 public contract.
- Context packet assembly persists tool schema snapshots and fills `ToolSpec.SchemaHash` / `ToolSpec.Epoch`. Before permission checks or execution, tool calls are bound to the current spec metadata; explicit mismatched `SchemaHash` or `Epoch` fails the run before execution.
- PermissionCenter now gates tool execution, workspace writes, and governance proposals through the same SDK contract.
- Permission decisions currently support `deny`, `allow_once`, `allow_for_run`, `allow_for_session`, `allow_all_for_run`, and `allow_all_for_session`.
- `ResolvePermission` persists reviewer decisions and deterministic grants.
- `ResolveApproval` is an approval wrapper over `ResumeRun`. Tool approvals persist pending tool continuations; model/plan approvals persist `model.approval` loop continuations. Both resume through the normal model/tool loop and reject mismatched run/session/root override scopes before permission resolution.
- `CheckpointGraph` exposes run-internal checkpoints with run/session-lifecycle-aware resume readiness; `ResumeRun` can continue from approval or checkpoint continuation state when the run is resumable and the owning session still accepts continuation.
- Session lifecycle now uses public `SessionID` while preserving the existing internal `ThreadRecord.ResumeID` storage name.
- `InspectSession` exposes `CanResume`, `ExternalResumeReady`, and `ResumeCommandHint` so a CLI/frontend can implement `--resume <session_id>` outside the core loop.
- `StopSession` supports `cancel_active` and `drain`; `drain` rejects new runs, preserves active runs, and auto-stops the session once active runs finish.
- `.gitignore` keeps binary names ignored while explicitly allowing `pkg/rtagent`.
- Product-level `--resume <session_id>` remains outside the SDK core and is demonstrated as a host integration pattern under `examples/host_resume_cli`.

## Target Core Loop

The v1 SDK should be able to run a complete runtime turn without requiring ngoagent product services around it:

1. Accept `SubmitRun` as the app-level command intake.
2. Create or inspect run/session state and acquire an execution lease.
3. Assemble a context packet from project state, registered handles, profile, memory, and prior events.
4. Call a pluggable model provider through a stable port.
5. Resolve tool schemas from registered tool providers and execute tool calls through the tool loop.
6. Check permissions before side-effecting tool calls and suspend on approval gates.
7. Persist approval decisions and scoped grants through `ResolvePermission`, or call `ResolveApproval` to resolve and resume a suspended tool or model approval continuation.
8. Persist loop checkpoints so run-internal continuation can be inspected or resumed.
9. Append all durable runtime facts to the journal.
10. Finish, cancel, deny, or interrupt the run with a stable result envelope.
11. Serve `Inspect`, `InspectSession`, `SessionGraph`, `CheckpointGraph`, `ListEvents`, `PermissionSnapshot`, and `WorldState` as read-side projections derived from journal/state.

Product shells such as TUI, HTTP server, desktop app, or workflow UI should sit outside this SDK and consume these runtime APIs.

## Evidence

- `pkg/rtagent/runtime.go`
- `pkg/rtagent/runtime_run.go`
- `pkg/rtagent/runtime_events.go`
- `pkg/rtagent/runtime_projection.go`
- `pkg/rtagent/runtime_context.go`
- `pkg/rtagent/runtime_side_effects.go`
- `pkg/rtagent/runtime_helpers.go`
- `pkg/rtagent/kernel.go`
- `pkg/rtagent/loop.go`
- `pkg/rtagent/loop_context.go`
- `pkg/rtagent/loop_tools.go`
- `pkg/rtagent/loop_outcomes.go`
- `pkg/rtagent/loop_activity.go`
- `pkg/rtagent/loop_helpers.go`
- `pkg/rtagent/world_state.go`
- `pkg/rtagent/world_state_context.go`
- `pkg/rtagent/world_state_legacy.go`
- `pkg/rtagent/world_state_helpers.go`
- `pkg/rtagent/world_state_memory.go`
- `pkg/rtagent/world_state_capability.go`
- `pkg/rtagent/world_state_external.go`
- `pkg/rtagent/world_state_runtime.go`
- `pkg/rtagent/world_state_test.go`
- `pkg/rtagent/session.go`
- `pkg/rtagent/session_graph.go`
- `pkg/rtagent/session_stop.go`
- `pkg/rtagent/permission_center.go`
- `pkg/rtagent/permission_center_policy.go`
- `pkg/rtagent/permission_center_grants.go`
- `pkg/rtagent/permission_center_store.go`
- `pkg/rtagent/permission_center_events.go`
- `pkg/rtagent/permission_center_types.go`
- `pkg/rtagent/tool_registry.go`
- `pkg/rtagent/tool_schema.go`
- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_config.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_model.go`
- `pkg/rtagent/types_session.go`
- `pkg/rtagent/types_permission.go`
- `pkg/rtagent/types_tool.go`
- `pkg/rtagent/types_world_state.go`
- `pkg/rtagent/interfaces.go`
- `pkg/rtagent/provider_adapters.go`
- `pkg/rtagent/openai_provider.go`
- `pkg/rtagent/openai_provider_types.go`
- `pkg/rtagent/openai_provider_messages.go`
- `pkg/rtagent/openai_provider_decode.go`
- `pkg/rtagent/openai_provider_errors.go`
- `pkg/rtagent/openai_provider_test.go`
- `pkg/rtagent/checkpoint.go`
- `pkg/rtagent/checkpoint_test.go`
- `pkg/rtagent/permission_projection.go`
- `pkg/rtagent/permission_center_test.go`
- `pkg/rtagent/session_test.go`
- `pkg/rtagent/runtime_test.go`
- `internal/runtime/worldstate/builder.go`
- `internal/infrastructure/persistence/sqlite/adapters/run_thread_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/checkpoint_event_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/memory_artifact_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/activity_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/permission_governance_store.go`
- `examples/`
- `docs/architecture/ngoagent-current-sdk-interface-analysis.md`
