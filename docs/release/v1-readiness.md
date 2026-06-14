# RTAgent Runtime SDK v1 Readiness

## Read When

- Deciding whether the SDK can be labeled `v1.0`.
- Planning the remaining work after core loop, provider, PermissionCenter, session/checkpoint, and WorldState milestones.
- Preparing release notes, package naming, or external repository publication.

## Owner

Runtime/SDK owner.

## Update Trigger

- A v1 gate is completed, removed, or reclassified.
- Public `pkg/rtagent` API, examples, module path, validation commands, or release boundary changes.
- A known risk becomes a committed v1 feature or an explicit non-goal.

## Validation

- `bash scripts/validate_sdk.sh`
- `bash scripts/release_preflight.sh` (expected to fail until module path and clean packaging blockers are closed)
- `while IFS= read -r script; do bash -n "$script"; done < <(find scripts -type f -name '*.sh' | sort)`
- `bash scripts/set_module_path.sh --check example.com/rtagent/sdk`
- `bash scripts/set_module_path.sh --dry-run example.com/rtagent/sdk`
- `bash scripts/validate_sdk.sh` module-path validation cases reject local, URL, host:port, credential-bearing, and non-domain paths before release migration.
- `bash scripts/check_public_api_snapshot.sh`
- `bash scripts/audit_sdk_boundary.sh`
- `bash scripts/audit_sdk_shape.sh`
- `bash scripts/audit_sdk_docs.sh`
- `bash scripts/audit_sdk_examples.sh`
- `GOCACHE=/private/tmp/rtagent-go-cache go test ./... -count=1`
- `GOCACHE=/private/tmp/rtagent-go-cache go test ./... -race -count=1`
- `GOCACHE=/private/tmp/rtagent-go-cache go vet ./...`
- `go doc ./pkg/rtagent`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./cmd/rtagent`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./examples/minimal_runtime`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./examples/approval_resume`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./examples/mcp_skill_inventory`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./examples/host_resume_cli --db /tmp/rtagent-host-resume-demo.db --session demo-session --input "first turn" --graph`
- `GOCACHE=/private/tmp/rtagent-go-cache go run ./examples/host_resume_cli --db /tmp/rtagent-host-resume-demo.db --resume demo-session --input "next turn" --graph`
- Optional live provider gate: `RTAGENT_RUN_DASHSCOPE_INTEGRATION=1 go test ./pkg/rtagent -run TestDashScopeQwen37PlusIntegration -count=1 -v`

## Current Decision

Current implementation is **v0.0.1**, published at `github.com/None9527/RTAgent-SDK`. The module import path is finalized. This is the first public release — the kernel core loop, convergence control, context budget, WorldState adaptive cache, permission/approval, session lifecycle, checkpoint resume, and model capability-driven budget are all complete and regression-covered.

The runtime SDK is usable for internal host integration and dogfooding. It has not yet been tagged `v1.0`; the remaining release gates below track the gap between v0.0.1 and v1.0 (primarily real-model multi-turn convergence validation and broader tentacle coverage).

## Done For v1 Core

| Gate | Status | Evidence |
|---|---|---|
| Runnable SDK core loop | Done | `pkg/rtagent/loop.go`, `pkg/rtagent/loop_context.go`, `pkg/rtagent/loop_tools.go`, `pkg/rtagent/loop_outcomes.go`, `pkg/rtagent/runtime_test.go` |
| Public runtime facade | Done | `go doc ./pkg/rtagent.Runtime`, `pkg/rtagent/runtime*.go`, `docs/architecture/sdk-core.md` |
| Runtime config separated from host ports | Done | `Config.Runtime`, `Config.Host`, `HostPorts`, `docs/architecture/sdk-core.md` |
| Startup container and aggregate store hidden from public API | Done | `pkg/rtagent/kernel.go`, unexported `runtimeKernel`, SDK-local `runtimeStore`, `scripts/audit_sdk_boundary.sh`, `go doc ./pkg/rtagent.Runtime` |
| Runtime close lifecycle contract covered | Done | `Close()` is idempotent and every public `Runtime` facade method returns `ErrRuntimeClosed` after close; covered by `TestRuntimeCloseMakesPublicAPIsUnavailable` |
| Single model provider contract with streaming handler | Done | `ModelProvider.CompleteTurn(ctx, req, stream)`, `docs/api/model-providers.md` |
| OpenAI-compatible provider and DashScope helper | Done | `pkg/rtagent/openai_provider*.go`, `pkg/rtagent/openai_provider_test.go`, `examples/dashscope_qwen` |
| Tool registry and schema snapshots | Done | `pkg/rtagent/tool_registry.go`, `pkg/rtagent/tool_schema.go`, `docs/api/tool-providers.md` |
| PermissionCenter in SDK core | Done | `pkg/rtagent/permission_center*.go`, `docs/api/permission-center.md` |
| Approval resume and run checkpoints | Done | `pkg/rtagent/approval_resume.go`, `pkg/rtagent/checkpoint.go`, `pkg/rtagent/checkpoint_test.go` |
| Session lifecycle and external resume pattern | Done | `pkg/rtagent/session.go`, `pkg/rtagent/session_graph.go`, `pkg/rtagent/session_stop.go`, `examples/host_resume_cli`, `docs/api/session-lifecycle.md` |
| WorldState provider projections | Done | `pkg/rtagent/world_state*.go`, `docs/api/world-state.md` |
| MCP/Skill inventory as WorldState projections | Done | `MCPProvider`, `SkillProvider`, `examples/mcp_skill_inventory` |
| Host adapter helpers | Done | `pkg/rtagent/provider_adapters.go`, `pkg/rtagent/provider_adapters_test.go`; nil/empty adapter behavior is covered and runtime tests dogfood public adapter helpers instead of local duplicate provider func types |
| Public examples compile through SDK facade | Done | `examples/*`, `go test ./...`; local non-persistent examples use temporary runtime directories and clean them up, while `host_resume_cli` remains the explicit persistent `--db` resume demo |
| Legacy product shell removed from core | Done | Removed internal REST/governed executor/sandbox demo paths; `cmd/rtagent` is SDK smoke only |
| Facade convenience interfaces removed | Done | `pkg/rtagent/interfaces.go`, `docs/api/public-compatibility.md`, `docs/api/public-api.snapshot.txt` |
| Misleading run-initialization facade removed | Done | Run initialization is internal; public run entry is `SubmitRun` or lower-level full-loop `Run` |
| Legacy sandbox wording removed from public tool contract | Done | `ToolSpec.ExecutionConstraints` replaces the old sandbox-specific field/type before v1 freeze |
| Provider-specific concrete error surface removed | Done | OpenAI-compatible provider errors now expose stable details through `ModelProviderError`, not a provider-specific exported struct |
| Legacy v0 schema constants removed | Done | Runtime events and WorldState now expose the actual v1 schema constants used by current SDK projections |
| Public request/query JSON contract aligned | Done | Host-facing run, runtime event, event/inspect/session/checkpoint/worldstate/permission, approval resume, stop-session, and workspace write request/query structs use stable snake_case JSON tags; covered by `TestPublicQueryAndRequestJSONContractUsesSnakeCase` |
| Session-scoped read queries honored | Done | `ListEvents(EventQuery{SessionID})` aggregates session run events, rejects unknown sessions, and `Inspect(InspectQuery{SessionID})` resolves latest run; covered by `TestRuntimeListEventsAndInspectBySessionID` and `TestRuntimeListEventsRejectsUnknownSession` |
| Run-scoped journal append covered | Done | `Runtime.Emit` rejects unknown run ids before append so host-defined events cannot create orphan runtime facts; covered by `TestRuntimeEmitRejectsUnknownRun` |
| Run-scoped mutation anchoring covered | Done | `RegisterContextHandle` rejects unknown run ids before registry mutation, `CheckPermission`/`ResolvePermission` reject unknown scoped run ids before creating grants, permission requests, reviewer decisions, or permission events, and `WriteFile` rejects before workspace artifacts; covered by `TestRuntimeRegisterContextHandleRejectsUnknownRun`, `TestPermissionCenterRejectsUnknownRunBeforeGrant`, `TestResolvePermissionRejectsStaleUnknownRunBeforeGrant`, and `TestWriteFileRejectsUnknownRunBeforeWorkspaceMutation` |
| Run-scoped projection existence covered | Done | `Inspect`, `PermissionSnapshot`, `WorldState`, and `CheckpointGraph` reject unknown run ids instead of fabricating empty projections; covered by `TestRuntimeReadProjectionsRejectUnknownRun` |
| Empty checkpoint graph covered | Done | `CheckpointGraph` returns an empty graph for existing runs without checkpoint records instead of panicking; covered by `TestCheckpointGraphHandlesRunWithoutCheckpoints` |
| Session graph root filtering covered | Done | `SessionGraph(SessionGraphQuery{RootRunID})` filters to the requested root branch; covered by `TestRuntimeSessionGraphFiltersByRootRunID` |
| Approval resume scope anchoring covered | Done | `ResumeRun` rejects approval resume requests whose supplied run/session/root scope differs from the original approval continuation before resolving permission, recording grants, or invoking tools; covered by `TestResumeRunRejectsApprovalScopeMismatch` |
| Terminal/stopped checkpoint projection covered | Done | `CheckpointGraph.ResumeReady` is suppressed with warnings when the run is terminal or the owning session is stopped; terminal resume returns `run_not_resumable`; covered by `TestCheckpointGraphDisablesResumeReadyForTerminalRun` and `TestCheckpointGraphDisablesResumeReadyAfterSessionStop` |
| Drain approval resume boundary covered | Done | `StopSession(drain)` rejects non-deny approval resume without recording an active grant, while deny can close the suspended run and auto-stop the drained session; `PermissionSnapshot` and WorldState governance projection expose deny-only pending choices during drain; covered by `TestRuntimeStopSessionDrainBlocksApprovalResumeButAllowsDeny` |
| Stopped-session permission gate covered | Done | Non-read-only `CheckPermission` rejects stopped/stopping sessions before existing grant reuse or mode shortcuts; covered by `TestPermissionCenterStoppedSessionBlocksExistingGrant` and `TestWriteFileStoppedSessionRejectedBeforeAcceptEditsGrant` |
| Stopped-session permission projection covered | Done | `PermissionSnapshot.ActiveGrants` and WorldState capability authorization are effective-state projections and do not expose historical grants as current authorization after session stop; covered by `TestPermissionCenterStoppedSessionBlocksExistingGrant` and `TestRuntimeWorldStateMarksGrantedCapabilityUnavailableAfterSessionStop` |
| Interrupt terminal-run idempotency covered | Done | `InterruptRun` returns the existing terminal status and does not append `run.interrupted` for terminal runs; covered by `TestRuntimeInterruptRunIsNoopForTerminalRun` |
| WorldState partition filter contract covered | Done | `WorldStateQuery.Partition` filters typed partitions, top-level handles, and legacy compatibility entries; covered by `TestRuntimeWorldStateProjectsTypedPartitions` |
| Legacy product event constants removed | Done | Public `EventKind*` constants are now limited to SDK-owned runtime events; host/product events can still be emitted as custom `EventKind` strings |
| Public compatibility policy | Done | `docs/api/public-compatibility.md`, `pkg/rtagent/doc.go`, `go doc ./pkg/rtagent` |
| Public API snapshot check | Done | `docs/api/public-api.snapshot.txt`, `scripts/check_public_api_snapshot.sh`, `scripts/update_public_api_snapshot.sh` |
| Local release validation script | Done | `scripts/validate_sdk.sh`, `bash scripts/validate_sdk.sh`; default host-resume validation database files are removed on exit unless `RTAGENT_HOST_RESUME_DB` is explicitly supplied |
| SDK boundary audit script | Done | `scripts/audit_sdk_boundary.sh`; checks the single `ModelProvider.CompleteTurn(ctx, req, stream)` contract, absence of legacy `StreamingModelProvider`/`Runtime.Execute`, no public startup/persistence implementation leakage, `internal/startup` imports confined to `pkg/rtagent/kernel.go`, aggregate persistence `Bundle` confined to the persistence contract and startup composition, no unused dataset-export/audit persistence surface, and removed product-shell files |
| SDK shape audit script | Done | `scripts/audit_sdk_shape.sh`; checks `pkg/rtagent` source/test file-size budgets, internal runtime source file-size budgets, internal persistence domain file-size budgets, SQLite adapter file-size budgets, and prevents the old public `types.go` monolith from returning |
| SDK docs audit script | Done | `scripts/audit_sdk_docs.sh`; checks docs index paths, required durable-doc metadata, and docs-index coverage for markdown docs |
| SDK examples audit script | Done | `scripts/audit_sdk_examples.sh`; checks local example and smoke command validation coverage, validation-friendly `run() error` entrypoints, panic-free examples/commands, explicit `RuntimeConfig`/DSN/WorkDir usage, documented opt-in external-provider examples, temp database cleanup policy, and no internal imports from host-facing commands/examples |
| Release preflight script | Done | `scripts/release_preflight.sh`, `docs/release/release-process.md`; blocks local or invalid module paths, dirty worktree, tracked or root-level generated artifacts, package-doc policy drift, public API snapshot drift, SDK boundary drift, SDK shape/docs/examples drift, premature README/v1-readiness v1.0 on a local module path, and stale README/v1-readiness v0.2 or v1-candidate status after final module path migration |
| Module path migration helper | Done | `scripts/set_module_path.sh --check <final module path>` validates release path shape without reading or mutating module state, and `--dry-run <final module path>` reports import rewrites before mutating `go.mod` or Go files |
| Shared module path validation | Done | `scripts/lib/module_path.sh` is reused by `scripts/set_module_path.sh`, `scripts/release_preflight.sh`, and `scripts/validate_sdk.sh`, so final module path rules do not drift between migration, release gates, and validation cases; local, URL, host:port, credential-bearing, whitespace, double-slash, trailing-slash, and non-domain paths are rejected before release migration |
| Concurrency safety contract and `-race` validation | Done | `docs/api/public-compatibility.md` Concurrency Contract documents that `Emit` is sequence-serialized within one `Runtime` while other facade methods carry no v1 concurrency guarantee; `scripts/validate_sdk.sh` runs `go test ./... -race`; covered by `TestRuntimeConcurrentEmitAndSubmitRunAreRaceFree` |
| Model provider retry/failure semantics | Done | `docs/api/model-providers.md` Retry and Failure Semantics documents that the SDK does not retry provider failures and `ModelProviderErrorDetails.Retryable`/`RateLimited` are host-facing hints; covered by `TestOpenAIProviderSurfacesRetryableFlagsWithoutRetrying` (429 returned as-is, exactly one request) |
| WorldState projection determinism contract | Done | `docs/api/world-state.md` Projection Determinism documents that partition order and `SnapshotID`/`RuntimeEpoch`/`SourceWatermark` are deterministic while `BuildID`/`GeneratedAt`/`BuiltAt` are wall-clock-derived; covered by `TestRuntimeWorldStateProjectionIsDeterministicExcludingTimestamps` |

## Remaining v1 Gates

| Gate | Status | Required Closure |
|---|---|---|
| Module import path | Done | Finalized to `github.com/None9527/RTAgent-SDK`; published at v0.0.1. |
| Release naming | Done | Repository `RTAgent-SDK` at `github.com/None9527`; version v0.0.1. |
| README status | Done | README updated to v0.0.1. |
| Final release validation pass | Done | `bash scripts/validate_sdk.sh` passes on the published module path. DashScope live integration verified with qwen3.6-plus (single-turn completed). |
| DashScope live provider | Conditional | Single-turn verified with qwen3.6-plus; multi-turn tool-call convergence under a real model is a v1.0 gate, not v0.0.1. |
| Dirty worktree packaging | Done | v0.0.1 tagged from a clean worktree on `main`. |
| Real-model multi-turn tool convergence | Blocking (v1.0) | v0.0.1 ships single-turn verified; multi-turn tool-call convergence under a real model (not mock) must pass before v1.0. |
| Tentacle coverage | Blocking (v1.0) | v0.0.1 ships the kernel only; tool/memory/MCP/skill tentacles must be integrated and tested before v1.0. |

## Explicit v1 Non-Goals

- Product shells: TUI, desktop app, frontend, HTTP server, and product-level `--resume <session_id>` command.
- Public kernel/store injection. v1 extension uses `Config.Host` ports.
- Multi-process SQLite event sequence allocation. Single `Runtime` instance sequencing is committed; shared multi-process writers are not.
- Default global unscoped `allow_all`. Session/run scoped grants and explicit permission modes are SDK-owned; broader bypass policy belongs to the host.
- WorldState as a truth store. WorldState remains a read-side projection over runtime facts.

## Stop Criteria

The SDK can stop at `v1.0` only when:

1. Every **Done For v1 Core** gate still has current source/test/doc evidence.
2. Every **Remaining v1 Gate** is either closed or explicitly moved to **v1 Non-Goals** with owner approval.
3. The Validation command set passes on the release candidate, with any skipped optional integration recorded.
4. README, docs index, package docs, and module path all describe the same release identity.
5. No public API depends on `internal/startup`, concrete SQLite adapters, or product shell packages.

## Evidence

- `README.md`
- `.ai_project.md`
- `go.mod`
- `scripts/validate_sdk.sh`
- `scripts/audit_sdk_boundary.sh`
- `scripts/audit_sdk_shape.sh`
- `scripts/audit_sdk_docs.sh`
- `scripts/audit_sdk_examples.sh`
- `scripts/set_module_path.sh`
- `scripts/lib/module_path.sh`
- `scripts/release_preflight.sh`
- `pkg/rtagent`
- `cmd/rtagent`
- `examples`
- `docs/INDEX.md`
- `docs/project-structure.md`
- `docs/release/release-process.md`
- `docs/architecture/sdk-core.md`
- `docs/api/public-compatibility.md`
- `docs/api/public-api.snapshot.txt`
- `docs/api/model-providers.md`
- `docs/api/tool-providers.md`
- `docs/api/permission-center.md`
- `docs/api/session-lifecycle.md`
- `docs/api/world-state.md`
