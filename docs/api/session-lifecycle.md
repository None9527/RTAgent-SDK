# Session Lifecycle API

## Read When

- Changing SDK session, run graph, `StopSession`, resume, or conversation-return behavior.
- Building a CLI/frontend command such as `--resume <session_id>`.
- Implementing approval resume or session-scoped cancellation.

## Owner

Runtime/SDK owner.

## Update Trigger

- `InspectSession`, `SessionGraph`, `StopSession`, run/session persistence, or session status semantics change.
- CLI/frontend resume behavior changes.
- Approval resume starts depending on session state.

## Validation

- `go test ./...`
- `go vet ./...`

## Boundary

The SDK owns the canonical session lifecycle and run graph. A product shell, CLI, or frontend owns user-facing commands such as:

```bash
rtagent --resume 7b00af52-edbc-4da7-88f0-f2ea6d0041f0
```

That command should not be hardcoded into the runtime loop. It should use SDK read APIs to recover conversation state and then submit a new run with the same `SessionID`.

## Storage Model

- Public name: `SessionID`.
- Current internal storage name: `ThreadRecord.ResumeID`.
- A session contains multiple runs.
- A run is one core-loop execution.
- Run graph fields are stored on `RunRecord`: `RootRunID`, `ParentRunID`, and `TaskID`.

This preserves the existing `ThreadRecord`/`ResumeID` storage while moving public SDK language to session.

## Public Contract

- `InspectSession(ctx, SessionQuery)` returns a `SessionSnapshot` with status, latest run, active runs, run summaries, and resume readiness.
- `SessionGraph(ctx, SessionGraphQuery)` returns run nodes and parent/root edges for a session. Supplying `RootRunID` filters the graph to that root branch, including the root run and matching descendants.
- `StopSession(ctx, StopSessionRequest)` stops or drains a session.
- `InterruptRun(ctx, runID)` cancels a non-terminal run and emits `run.interrupted`; terminal runs are idempotent no-ops and do not append a new interrupt event.
- `ListEvents(ctx, EventQuery{SessionID: ...})` returns the persisted events for every run in the session, in session run order. Run-scoped and session-scoped event queries reject unknown run/session ids instead of returning fabricated empty event lists.
- `Inspect(ctx, InspectQuery{SessionID: ...})` resolves to the session's latest run projection.
- `ResolveApproval(ctx, approvalID, decision)` honors session lifecycle gates: non-deny approval resumes are rejected once the session is `stopping` or `stopped`, and the rejected approval does not create an active grant.
- Approval resume is bound to the original approval scope. If `ResumeRunRequest.RunID` or `ResumeRunRequest.Scope` supplies a different run, session, or root run, `ResumeRun` rejects it with `RuntimeError.Code=approval_scope_mismatch` before resolving the permission or resuming the loop.
- Deny decisions remain accepted for suspended approvals during drain/stop so the run can terminate without executing the pending action.
- `PermissionSnapshot` and WorldState governance projection expose deny-only choices for pending approvals while a session is `stopping` or `stopped`.
- `CheckpointGraph(ctx, CheckpointGraphQuery)` returns run-internal checkpoint nodes and sequence edges. Existing runs with no checkpoint records return an empty graph. `ResumeReady` is an effective lifecycle-aware projection: terminal run status or stopped/stopping sessions mark resumable checkpoints unavailable and include warnings.
- `ResumeRun(ctx, ResumeRunRequest)` resumes either an approval continuation or a resumable checkpoint continuation. It rejects checkpoint continuation while the run is terminal with `RuntimeError.Code=run_not_resumable`, and rejects continuation while the owning session is `stopping` or `stopped`.

## Session Status

- `active`: accepts new runs and can be resumed when a latest run exists.
- `stopping`: drain mode; preserves active runs, rejects new runs, blocks non-read-only permission checks/grants, and still accepts deny decisions for suspended approvals.
- `stopped`: rejects new runs and blocks non-read-only permission checks/grants.

## Stop Modes

- `cancel_active`: default. Marks the session stopped and interrupts active runs with status `canceled`.
- `drain`: if active runs exist, marks the session `stopping`, rejects new runs, and lets active runs finish. Suspended approvals can still be denied to close the active run; allow decisions are rejected without recording grants and are omitted from approval projection choices. When the last active run reaches a terminal status, the SDK automatically marks the session `stopped` and emits `session.ended`. If no active runs exist when drain is requested, the session stops immediately.

`StopSession` is idempotent: calling it on an already stopped session returns `AlreadyStopped`.

## Resume Feasibility

`--resume <session_id>` is feasible outside the SDK core loop:

1. CLI/frontend receives a session id.
2. It calls `InspectSession`.
3. If `CanResume` and `ExternalResumeReady` are true, it loads prior run/session state through existing read APIs and checkpoint graph APIs.
4. It submits a new run with the same `SessionID`.

The SDK supports this by returning `ResumeCommandHint`, `CanResume`, `ExternalResumeReady`, `LatestRunID`, and run summaries. The frontend still owns UI state restoration, input focus, and command parsing.

When a host supplies both `RunID` and `SessionID` to `ListEvents` or `Inspect`, the SDK validates that the run belongs to that session. `EventQuery.AfterSeq` is a run-local sequence cursor; in session aggregation it is applied to each run independently, not as a global session cursor.

`examples/host_resume_cli` is the runnable host integration example. It uses a persistent `--db`, creates or continues a session with `--session`, resumes an existing session with `--resume`, and can print the SDK run graph with `--graph`.

## Checkpoint Resume

The core loop now writes checkpoints for context packet, model request, model response, tool call, tool observation, approval pending, and terminal states. Checkpoint payloads persist the loop continuation: scope, context packet, message history, observations, pending tool calls, tool schema snapshot/hash, iteration, and tool round.

`CheckpointGraph` exposes those nodes with `ResumeReady` when the checkpoint can continue the run. `ResumeReady` is filtered by run status and session lifecycle, so a terminal run or stopped/stopping session does not advertise checkpoint continuation that `ResumeRun` will reject. `ResumeRun` can restore a pending tool continuation from a checkpoint after a host process recovers or after an approval is resolved.

## Current Limit

- `StopSession` does not release all possible external tool resources beyond interrupting active SDK runs.
- Multi-run graph resume is still a host/product concern; SDK checkpoint resume is run-internal loop continuation.

## Evidence

- `pkg/rtagent/session.go`
- `pkg/rtagent/session_graph.go`
- `pkg/rtagent/session_stop.go`
- `pkg/rtagent/approval_resume.go`
- `pkg/rtagent/session_test.go`
- `pkg/rtagent/runtime.go`
- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_session.go`
- `pkg/rtagent/types_permission.go`
- `pkg/rtagent/interfaces.go`
- `internal/domain/persistence/records_runtime.go`
- `internal/domain/persistence/stores.go`
- `internal/infrastructure/persistence/sqlite/adapters/models.go`
- `internal/infrastructure/persistence/sqlite/adapters/run_thread_store.go`
- `internal/infrastructure/persistence/sqlite/adapters/checkpoint_event_store.go`
