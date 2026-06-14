# Project Structure

## Read When

- Locating RTAgent runtime SDK modules.
- Changing public SDK package boundaries, examples, docs, or internal runtime wiring.
- Reviewing which files own facade, kernel bootstrap, persistence, WorldState, permissions, and examples.

## Owner

Runtime/SDK owner.

## Update Trigger

- Public SDK files are added, split, renamed, or deleted.
- Runtime internal wiring, persistence adapters, examples, or docs directories change.
- A new top-level package or command becomes part of the SDK workflow.

## Validation

- `go test ./...`
- `go vet ./...`

## Top-Level Directories

- `pkg/rtagent`: public Go SDK facade, contracts, providers, runtime loop, session/checkpoint/permission/WorldState APIs, and SDK package tests.
- `internal/startup`: internal bootstrap wiring for SQLite, aggregate persistence `Bundle`, event bus, governance, context engine, workspace, and WorldState builder.
- `internal/runtime`: internal runtime building blocks that remain outside the public SDK surface.
- `internal/domain`: internal persistence, contextual, and WorldState domain contracts.
- `internal/infrastructure`: SQLite/GORM persistence adapter implementation.
- `examples`: host integration examples that must compile through the public SDK facade; local non-persistent examples clean up temporary runtime storage, while `host_resume_cli` intentionally demonstrates persistent `--db` resume state.
- `docs`: durable SDK architecture, API, and project memory.
- `cmd`: local command entry points; `cmd/rtagent` is the public SDK smoke command and the old REST console command is intentionally removed.
- `README.md`: public SDK entry document with quick start, host resume, provider, validation, and boundary notes.

## SDK Package Map

- `pkg/rtagent/runtime.go`: public `Runtime` struct, lifecycle/config bootstrap, `Open`, `Close`, and readiness checks.
- `pkg/rtagent/runtime_run.go`: app-level `SubmitRun`, internal run initialization, command/request normalization, identity and runtime mode normalization.
- `pkg/rtagent/runtime_events.go`: runtime journal `Emit`, `ListEvents`, sequence assignment, event-bus publishing, and event record conversion.
- `pkg/rtagent/runtime_projection.go`: read-side `Inspect`, `InterruptRun`, and `WorldState` facade methods.
- `pkg/rtagent/runtime_context.go`: context handle registration and materialization facade methods.
- `pkg/rtagent/runtime_side_effects.go`: audited side-effecting facade methods such as `WriteFile` and `EvaluateProposal`.
- `pkg/rtagent/runtime_helpers.go`: shared SDK-local helpers for terminal status, payload cloning/extraction, execution scope extraction, string defaults, and UTC timestamps.
- `pkg/rtagent/kernel.go`: unexported runtime dependency bundle; confines startup container mapping to kernel bootstrap and adapts concrete internal dependencies plus the aggregate store behind SDK-local narrow ports.
- `pkg/rtagent/types_constants.go`: public schema, status, permission, partition, and runtime event constants.
- `pkg/rtagent/types_config.go`: public SDK runtime config and host port config types.
- `pkg/rtagent/types_runtime.go`: public scope, identity, run command/request, runtime projection, event, inspect, interrupt, and runtime error contract types.
- `pkg/rtagent/types_model.go`: public model/context-packet/message/stream/provider-error schema types.
- `pkg/rtagent/types_session.go`: public session, run graph, checkpoint graph, resume, and stop-session schema types.
- `pkg/rtagent/types_permission.go`: public approval, permission, grant, plan, evidence, permission snapshot, and governance action schema types.
- `pkg/rtagent/types_tool.go`: public tool schema, tool call/observation, workspace write, and artifact schema types.
- `pkg/rtagent/types_world_state.go`: public WorldState, capability state, context handle, memory/hypothesis, inventory, and provider input schema types.
- `pkg/rtagent/interfaces.go`: public ports and smaller interface slices.
- `pkg/rtagent/provider_adapters.go`: lightweight function/struct adapters for host implementations of SDK ports.
- `pkg/rtagent/loop.go`: runnable core loop entry and model-turn/tool-round orchestration.
- `pkg/rtagent/loop_context.go`: context packet assembly, event history loading, tool schema snapshot persistence hookup.
- `pkg/rtagent/loop_tools.go`: tool-call normalization path, schema validation hook, PermissionCenter gating, approval suspension continuation, and tool execution events.
- `pkg/rtagent/loop_outcomes.go`: completed, suspended, denied, and failed run status transitions plus structured runtime error projection.
- `pkg/rtagent/loop_activity.go`: core-loop activity persistence and activity/plan runtime events.
- `pkg/rtagent/loop_helpers.go`: default echo model provider and loop-local helper functions.
- `pkg/rtagent/openai_provider.go`: public OpenAI-compatible provider config, constructors, `CompleteTurn`, HTTP request execution, and DashScope-compatible setup.
- `pkg/rtagent/openai_provider_types.go`: internal Chat Completions request/response DTOs.
- `pkg/rtagent/openai_provider_messages.go`: `ModelRequest`, `ModelMessage`, `ToolSpec`, and fallback prompt mapping into OpenAI-compatible request messages/tools.
- `pkg/rtagent/openai_provider_decode.go`: non-streaming and SSE response decoding, tool-call aggregation, content extraction, and usage mapping.
- `pkg/rtagent/openai_provider_errors.go`: non-2xx provider error parsing into the generic `ModelProviderError` contract.
- `pkg/rtagent/tool_registry.go`: multi-provider tool composition.
- `pkg/rtagent/tool_schema.go`: tool schema snapshot persistence and call validation.
- `pkg/rtagent/permission_center.go`: SDK-owned permission entry methods: `CheckPermission` and `ResolvePermission`.
- `pkg/rtagent/permission_center_policy.go`: permission check normalization, capability classification, read-only classification, and dangerous-command checks.
- `pkg/rtagent/permission_center_grants.go`: approval decision options, deterministic permission/grant/capability ids, grant normalization, scope merge, and payload hashing helpers.
- `pkg/rtagent/permission_center_store.go`: permission request persistence, approval continuation persistence, grant persistence, permission-record decoding, and grant expiry checks.
- `pkg/rtagent/permission_center_events.go`: permission requested/granted/denied runtime event emission.
- `pkg/rtagent/permission_center_types.go`: internal permission check, permission record scope, and approval continuation shapes.
- `pkg/rtagent/session.go`: session inspection, run/session persistence helpers, and session snapshot projection.
- `pkg/rtagent/session_graph.go`: session run graph projection and root-branch filtering.
- `pkg/rtagent/session_stop.go`: session stop/drain behavior, idle-drain completion, and `session.ended` emission.
- `pkg/rtagent/checkpoint.go` and `approval_resume.go`: run-internal checkpoint and approval continuation.
- `pkg/rtagent/world_state.go`: SDK WorldState snapshot assembly and partition draft aggregation.
- `pkg/rtagent/world_state_context.go`: context partition projection for workspace, context packet handles, and registered context handles.
- `pkg/rtagent/world_state_legacy.go`: compatibility projection from runtime-derived flat WorldState entries into public `WorldStateEntry` values.
- `pkg/rtagent/world_state_helpers.go`: shared WorldState event, scope, payload, ordering, and state helper functions.
- `pkg/rtagent/world_state_memory.go`: committed memory and hypothesis WorldState partitions.
- `pkg/rtagent/world_state_capability.go`: tool, MCP, skill capability projection and grant-aware capability state.
- `pkg/rtagent/world_state_external.go`: custom host `WorldStateProvider` projection integration.
- `pkg/rtagent/world_state_runtime.go`: governance, activity, and task runtime-event projections.
- `examples/host_resume_cli`: host-owned `--resume <session_id>` integration demo using persistent SQLite DSN, `InspectSession`, `SubmitRun`, and `SessionGraph`.

## Internal Domain Map

- `internal/domain/persistence/source.go`: shared source-reference kinds and `SourceRef`.
- `internal/domain/persistence/records_runtime.go`: run, session/thread, checkpoint, runtime event, artifact, and activity record shapes.
- `internal/domain/persistence/records_memory.go`: memory stage/kind/origin enums and `MemoryRecord`.
- `internal/domain/persistence/records_governance.go`: capability, tool schema, permission, grant, and lease record shapes.
- `internal/domain/persistence/stores.go`: persistence store interfaces and aggregate `Bundle`.
- `internal/domain/contextual/handle.go`: internal context handle records.
- `internal/domain/worldstate/worldstate.go`: internal WorldState partitions and runtime-derived flat entry shape.

## Internal Persistence Map

- `internal/infrastructure/persistence/sqlite/adapters/bundle.go`: `SQLiteBundle` construction and shared DB handle.
- `internal/infrastructure/persistence/sqlite/adapters/time_helpers.go`: RFC3339/time conversion helpers shared by store adapters.
- `internal/infrastructure/persistence/sqlite/adapters/run_thread_store.go`: run and thread persistence.
- `internal/infrastructure/persistence/sqlite/adapters/checkpoint_event_store.go`: checkpoint and runtime event persistence.
- `internal/infrastructure/persistence/sqlite/adapters/memory_artifact_store.go`: memory and artifact persistence.
- `internal/infrastructure/persistence/sqlite/adapters/activity_store.go`: activity persistence.
- `internal/infrastructure/persistence/sqlite/adapters/permission_governance_store.go`: capability, tool-schema, permission, grant, and lease persistence.
- `internal/infrastructure/persistence/sqlite/adapters/models.go`: GORM table models.
- `internal/infrastructure/persistence/sqlite/adapters/scanner.go`: SQLite scan helpers.

## Script Map

- `scripts/validate_sdk.sh`: release-candidate validation for docs, recursive shell syntax checks, final module-path validation cases, tests, vet, smoke command, examples, host resume, and default temp SQLite cleanup.
- `scripts/release_preflight.sh`: release gate for final repository module path shape, clean worktree, package docs, public API snapshot, SDK audits, tracked/root generated artifacts, and README/v1-readiness/module-path consistency.
- `scripts/audit_sdk_boundary.sh`: SDK public/internal boundary invariant audit.
- `scripts/audit_sdk_shape.sh`: public source/test, internal runtime, persistence domain, and adapter size/split-policy audit.
- `scripts/audit_sdk_docs.sh`: docs index and durable-doc metadata audit.
- `scripts/audit_sdk_examples.sh`: example and smoke command entrypoint style, explicit runtime config usage, local validation coverage, external-provider opt-in, temp database cleanup policy, and host-facing internal-import audit.
- `scripts/check_public_api_snapshot.sh`: public API snapshot drift check.
- `scripts/update_public_api_snapshot.sh`: public API snapshot regeneration after an intentional API change.
- `scripts/set_module_path.sh`: final module path check, dry-run, and migration helper.
- `scripts/lib/module_path.sh`: shared final module import-path validation helper used by release, migration, and SDK validation scripts.

## Editing Notes

- Keep host-facing API changes in `pkg/rtagent` reflected in `docs/architecture/sdk-core.md` and matching `docs/api/*` files.
- Keep durable docs listed in `docs/INDEX.md` with `Read When`, `Owner`, `Update Trigger`, and `Validation` sections.
- Keep examples on the current public facade; do not teach legacy API shapes or panic-based host integration there.
- Keep `cmd/` and `examples/` on public `pkg/rtagent` imports; they must not import `internal/` packages.
- Avoid exposing `internal/startup.RuntimeContainer` through public SDK types; `internal/startup` imports should stay confined to `pkg/rtagent/kernel.go`.
- Keep HTTP, TUI, desktop, and product shells outside the SDK core; do not reintroduce an internal REST server that bypasses `pkg/rtagent`.
- Keep `.gitignore` binary rules root-scoped, such as `/rtagent`, so source directories like `cmd/rtagent` and `pkg/rtagent` stay visible to git.
