# NGOAgent Current Runtime SDK Interface Analysis

## Read When

- Replanning RTAgent Runtime SDK from current ngoagent source.
- Deciding which public SDK interfaces should drive the runtime core.
- Checking whether older ngoagent architecture docs still match source.

## Owner

Runtime/SDK owner.

## Update Trigger

- `ngoagent/internal/runtime/app`, `runtime/host`, `agent`, `controlplane`, `runtime/state`, or `runtime/facts` changes.
- RTAgent public SDK interfaces under `pkg/rtagent` change.

## Validation

- Recheck cited ngoagent source files.
- `go test ./...`
- `go vet ./...`

## Current Source Conclusion

The goal is a runnable Runtime SDK extracted from ngoagent project experience, not a full ngoagent clone. It must include the core runtime loop; surrounding interfaces are only the port layer that makes that loop embeddable and replaceable. The 2026-05-21 ngoagent target architecture doc is stale for SDK planning. Current ngoagent source shows a clearer runtime split:

- `runtime/host` composes stores, projection service, app service, runner ports, active-run cancellation, profile/skill/MCP providers.
- `runtime/app` is the command/application layer: submit run, interrupt run, stop session, resolve approval, memory ingestion routing.
- `agent.RunnerImpl` is the execution kernel behind ngoagent's internal runner port, not the SDK facade itself.
- `runtime/state` is write-side truth: journal, run/session graph, leases, activities, approvals, grants, decisions, policy gates, tool schema/audit, context packet, evidence, plan, memory.
- `runtime/facts` is read-side fact access.
- `controlplane.RuntimeProjectionService` builds inspect snapshots, world state, permission, tool registry, scheduler, model context, and context packet projections.
- `interfaces/server` should translate transport requests into runtime/app calls; it should not own runtime truth.

## SDK-Driven Runtime Facade And Ports

The SDK should be driven by app-level commands first, not low-level event writes:

- `Runtime.SubmitRun(ctx, SubmitRunRequest, Identity)`: recommended external entry. Builds runtime command, session/run identity, permission mode, planning state, profile, target, role, scope, and args.
- `Runtime.Run(ctx, RuntimeCommand)`: lower-level runnable core loop entry after command normalization.
- `Runtime.Inspect(ctx, InspectQuery)`: read-side projection for UI/host clients. Returns status, events, world state, and derived runtime status.
- `Runtime.InterruptRun(ctx, runID)`: non-terminal run cancellation plus durable interrupted/canceled state; terminal runs are idempotent no-ops.
- `Runtime.ListEvents(ctx, EventQuery)`: journal tail for stream/SSE/TUI usage.
- `Runtime.WorldState(ctx, WorldStateQuery)`: derived read model; should stay source-watermarked.
- `Runtime.Emit`: low-level journal hook for adapters/tests, not the primary product API.
- `Runtime.ResolveApproval`: approval continuation method. RTAgent resumes permission-gated tool-call continuations and model/plan approval continuations through the same SDK approval path.
- `ToolProvider` and `ToolRegistry`: capability extension ports exposing `ToolSpec`, `ToolCall`, and `ToolObservation`, with v0 multi-provider composition plus schema snapshot persistence in RTAgent.
- `ModelProvider`: model-turn port exposing context packet, tool specs, observations, approval request, plan artifact, and output contracts.
- `Runtime.RegisterContextHandle` and `Runtime.MaterializeContext`: context handle registration/materialization.
- `Runtime.WriteFile`: audited workspace mutation.
- `Runtime.EvaluateProposal`: proposal/policy gate evaluation.

## Handling Plan

1. Keep `pkg/rtagent.Runtime` as the v0 embeddable object, but make `SubmitRun` the recommended public entry.
2. Build the runnable core loop milestone around `SubmitRun`: run lease, context packet, model turn, tool execution loop, approval suspension, journal append, result completion, and projection refresh. First slice is implemented in `pkg/rtagent/loop.go`.
3. Keep `Run` as the lower-level full-loop entry and `Emit` as the journal hook so tests, runner adapters, and future execution kernels can attach without exposing DB internals.
4. Add read-side `Inspect` instead of making callers assemble `RunRecord + events + WorldState` manually.
5. Keep `StopSession`, checkpoint resume, and `ResolveApproval` as SDK-owned lifecycle contracts, while product-level multi-run/session replay remains a host concern.
6. Keep tool extension contracts public and use `ToolRegistry` plus tool schema snapshots for multi-provider runnable loop behavior.
7. Keep DB/GORM and raw persistence behind internal adapters; public SDK types must not import adapter packages.
8. Avoid a broad exported `Core` interface and avoid freezing pre-grouped facade convenience interfaces. Hosts should define local narrow interfaces around the exact `Runtime` methods they consume, or use the concrete `Runtime` facade when they intentionally embed the SDK.

## Source Evidence

- `/Users/mac/Desktop/ngoagent/internal/runtime/host/host.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/app/service.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/app/submit.go`
- `/Users/mac/Desktop/ngoagent/internal/agent/runner.go`
- `/Users/mac/Desktop/ngoagent/internal/agent/session_engine.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/state/state.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/facts/facts.go`
- `/Users/mac/Desktop/ngoagent/internal/controlplane/runtime_projection_service.go`
- `/Users/mac/Desktop/ngoagent/internal/runtime/product_contract.go`
- `/Users/mac/Desktop/ngoagent/internal/architecture/runtime_app_boundary_test.go`
- `/Users/mac/Desktop/ngoagent/internal/architecture/agent_kernel_boundary_test.go`
- `/Users/mac/Desktop/ngoagent/internal/architecture/projection_boundary_test.go`
- `/Users/mac/Desktop/ngoagent/internal/architecture/db_adapter_boundary_test.go`
