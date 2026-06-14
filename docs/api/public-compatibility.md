# Public Compatibility Policy

## Read When

- Deciding whether an exported `pkg/rtagent` symbol can change before or after v1.
- Reviewing pull requests that touch public SDK contracts.
- Preparing v1 release notes or a module path/package documentation update.

## Owner

Runtime/SDK owner.

## Update Trigger

- Any exported `pkg/rtagent` type, function, method, constant, or error changes.
- Runtime event schema, WorldState schema, provider contract, permission contract, or host port behavior changes.
- A previously experimental surface becomes part of the v1 stable contract.

## Validation

- `go doc -all ./pkg/rtagent`
- `bash scripts/check_public_api_snapshot.sh`
- `go test ./...`
- `go vet ./...`

## Compatibility Level

Until the final module path is chosen, RTAgent is a **v1-candidate** SDK. The policy below defines what must be true once the package is tagged `v1.0`.

After v1, SDK consumers should be able to upgrade within the same major version without source changes when they stay on the stable surface and do not depend on explicitly experimental behavior, internal packages, example command output, or undocumented event payload details.

## Stable v1 Surface

The following `pkg/rtagent` surfaces are intended to be stable for v1 once the module path is finalized:

- Runtime lifecycle: `Open`, `Runtime.Close`, and `ErrRuntimeClosed`.
- Main run facade: `Runtime.SubmitRun`, `Runtime.Run`, `Runtime.InterruptRun`, `SubmitRunRequest`, `RuntimeCommand`, `RuntimeStateProjection`, and `RuntimeError`.
- Runtime journal/read projection: `Runtime.Emit`, `Runtime.ListEvents`, `Runtime.Inspect`, `RuntimeEventDraft`, `RuntimeEventEnvelope`, `EventQuery`, `InspectQuery`, and schema constants such as `SchemaRuntimeEventEnvelopeV1`.
- Session/checkpoint lifecycle: `InspectSession`, `SessionGraph`, `StopSession`, `CheckpointGraph`, `ResumeRun`, `ResolveApproval`, and their request/result/snapshot types.
- Configuration and host ports: `Config`, `RuntimeConfig`, `HostPorts`, `ExecutionScope`, and `Identity`.
- Model provider contract: `ModelProvider.CompleteTurn(ctx, req, stream)`, `ModelRequest`, `ModelMessage`, `ModelResponse`, `ModelStreamHandler`, `ModelStreamEvent`, `ModelUsage`, and stable stream event constants.
- OpenAI-compatible provider constructor surface: `NewOpenAICompatibleProvider`, `OpenAICompatibleProviderConfig`, `NewDashScopeOpenAICompatibleProviderFromEnv`, and `NewDashScopeQwen37PlusProviderFromEnv`.
- Tool provider contract: `ToolProvider`, `ToolRegistry`, `ToolSpec`, `ToolCall`, `ToolObservation`, `ToolOutputPolicy`, `ExecutionConstraints`, and tool schema hash/epoch semantics.
- Permission contract: `PermissionCenter`, permission request/result/decision/snapshot types, permission modes, decision actions, grant scope semantics, and `PermissionRequiredError`.
- WorldState read model: `WorldStateSnapshot`, `WorldStatePartition`, `WorldStateEntry`, `WorldStateHandle`, `CapabilityState`, partition constants, and source-watermark/read-only semantics.
- WorldState query semantics: `WorldStateQuery.Partition` filters typed `Partitions`, top-level `Handles`, and legacy compatibility `Entries` by exact partition name.
- Projection-only host inventory ports: `MemoryProvider`, `HypothesisProvider`, `MCPProvider`, `SkillProvider`, `WorldStateProvider`, and their function/adapter helpers.
- Workspace/governance methods exposed through the SDK facade: `WriteFile`, `EvaluateProposal`, `WriteFileRequest`, `ArtifactRecord`, and `ProposedAction`.

Stable public request/query structs that are likely to cross host process or HTTP/CLI adapter boundaries use snake_case JSON field names. This includes run intake, runtime command/event drafts, event/inspect/session/checkpoint/worldstate/permission queries, approval resume/stop-session requests, and workspace write requests; Go-only configuration structs are not treated as serialized wire contracts.

`EventQuery.SessionID` and `InspectQuery.SessionID` are stable read-side semantics. Session-scoped event queries aggregate runs in the session; session-scoped inspect resolves to the latest run. Run-scoped and session-scoped event queries reject unknown run/session ids instead of returning fabricated empty event lists. If `RunID` and `SessionID` are both provided, the SDK validates the run/session relationship.

Run-scoped read projections are stable error semantics. `Inspect`, `CheckpointGraph`, `PermissionSnapshot`, and `WorldState` require an existing run id and must not return fabricated empty projection snapshots for missing runs. `CheckpointGraph` returns an empty graph for an existing run that has no checkpoint records.

`CheckpointGraph.ResumeReady` is a stable effective projection. When the run is terminal or the owning session is `stopping` or `stopped`, resumable checkpoint nodes are projected with `ResumeReady=false` and snapshot warnings, matching the `ResumeRun` lifecycle gate. A terminal run resume attempt returns `RuntimeError.Code=run_not_resumable`.

Approval resume scope is stable error semantics. `ResumeRun` and `ResolveApproval` resume an approval only within the original approval run/session/root scope. A supplied `RunID` or `Scope` that points at a different run, session, or root run is rejected with `RuntimeError.Code=approval_scope_mismatch` before permission resolution, grant persistence, tool execution, or loop continuation.

`Runtime.InterruptRun` is idempotent for terminal runs. Non-terminal runs are marked `canceled` with an interrupted resolution and emit `run.interrupted`; terminal runs keep their existing terminal status and do not append another interrupt event.

Exported `EventKind*` constants describe event kinds generated or consumed by the SDK core loop and projections. `Runtime.Emit` still accepts any `EventKind` string for host-defined journal events on an existing run, but host/product event names are not frozen as SDK constants unless the runtime itself owns their semantics.

`Runtime.Emit` is run-scoped. It rejects unknown run ids before appending, so host-defined events cannot create orphan journal facts outside the SDK run/session graph.

Run-scoped mutation APIs are stable error semantics. `RegisterContextHandle` rejects unknown `ContextHandle.RunID` before mutating the context registry, and `CheckPermission`/`ResolvePermission` reject unknown scoped run ids before creating permission requests, reviewer decisions, grants, or permission events. Non-read-only permission checks require an existing run because they can authorize SDK side effects.

## Additive Changes Allowed In v1.x

The SDK may make these changes in a minor or patch release:

- Add optional fields to exported structs.
- Add new exported concrete types, constants, constructors, helper adapters, or concrete `Runtime` methods.
- Add new WorldState partitions, entries, handles, warnings, metadata fields, capability metadata, or source watermarks.
- Add new runtime event kinds or additional event payload fields.
- Add new permission decision reasons, warnings, policy metadata, or grant explanations without changing existing grant semantics.
- Add new provider error detail fields while preserving existing `ModelProviderErrorDetails` fields.
- Add new OpenAI-compatible provider options when the zero value preserves current behavior.

Hosts should ignore unknown metadata, event payload fields, WorldState partitions, warning codes, and provider detail fields they do not understand.

## Breaking Changes Requiring v2

These changes require a new major version after v1:

- Removing or renaming exported stable symbols.
- Changing stable function or method signatures.
- Adding methods to stable exported interfaces such as `ModelProvider`, `ToolProvider`, `PermissionCenter`, or projection providers.
- Changing required fields, enum string values, permission grant scope semantics, model/tool message history semantics, or WorldState read/write separation.
- Changing zero-config `Open(ctx, Config{})` from ephemeral storage to durable host-working-directory storage.
- Exposing or requiring `internal/` packages, concrete SQLite adapters, or the unexported runtime kernel as public integration dependencies.

## Experimental Or Non-Contract Surfaces

The following are useful implementation details but are not stable v1 API commitments:

- Any package under `internal/`.
- The unexported `runtimeKernel`, internal startup bootstrap, and concrete persistence adapters.
- Provider-specific concrete error implementations; stable provider error semantics are exposed through `ModelProviderError` and `ModelProviderErrorDetails`.
- Concrete SQLite schema shape, GORM model names, and internal migration layout.
- Exact wording of error strings when structured fields are available.
- Exact default placeholder model output beyond being deterministic and local.
- Example CLI stdout formatting, including `examples/host_resume_cli`.
- Product shell UX such as TUI, desktop, frontend, HTTP server, or a product-owned `--resume <session_id>` command.
- Multi-process SQLite event sequence allocation.
- Live DashScope service availability, regional endpoint behavior, model naming outside the documented environment variables, or account-specific rate limits.

## Host Responsibilities

Hosts should integrate through `Config.Host` ports and the concrete `Runtime` facade. Hosts should not import `internal/`, depend on SQLite implementation details, parse human-readable errors when structured fields exist, or treat WorldState as writable truth.

Hosts that expose user approvals should use `PermissionSnapshot`, `ResolvePermission`, and `ResolveApproval` instead of inferring authorization from UI state. Hosts that need product-level resume should persist a DSN and call `InspectSession`/`SessionGraph` before submitting a new run with the same `SessionID`.

Hosts that want narrow dependencies around Runtime facade methods should define small local interfaces in their own package. The SDK deliberately does not export convenience facade interfaces for `Run`, `Emit`, `Inspect`, session management, checkpoint resume, context materialization, workspace writes, or governance evaluation; exporting those would freeze method groupings that hosts can define more accurately themselves.

## API Snapshot

`docs/api/public-api.snapshot.txt` records the normalized `go doc -all ./pkg/rtagent` public surface, including exported struct fields, interface methods, constants, functions, and method sets. The validation script compares the current package docs to that detailed snapshot. If the diff is intentional, update the compatibility policy if needed, then run:

```bash
bash scripts/update_public_api_snapshot.sh
```

Module import path is normalized to `<module>/pkg/rtagent` so choosing the final repository path does not create snapshot noise; `scripts/release_preflight.sh` remains responsible for checking the real module path before v1.

## Evidence

- `pkg/rtagent/doc.go`
- `pkg/rtagent/interfaces.go`
- `pkg/rtagent/runtime*.go`
- `pkg/rtagent/types*.go`
- `pkg/rtagent/openai_provider*.go`
- `pkg/rtagent/permission_center*.go`
- `pkg/rtagent/world_state*.go`
- `docs/api/public-api.snapshot.txt`
- `scripts/check_public_api_snapshot.sh`
- `scripts/update_public_api_snapshot.sh`
- `docs/architecture/sdk-core.md`
- `docs/release/v1-readiness.md`
