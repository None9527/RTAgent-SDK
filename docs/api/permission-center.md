# PermissionCenter API

## Read When

- Changing SDK approval, permission, grant, tool execution, or workspace mutation behavior.
- Embedding RTAgent in a host app that needs approval UI or policy wiring.
- Continuing approval resume work.

## Owner

Runtime/SDK owner.

## Update Trigger

- `PermissionCenter`, `PermissionCheckRequest`, approval decision, grant scope, or permission event behavior changes.
- Tool loop or workspace mutation gates change.
- Permission persistence or resume semantics change.

## Validation

- `go test ./...`
- `go vet ./...`

## Boundary

`PermissionCenter` is part of the runtime SDK, not only an outer application concern. The SDK owns the canonical permission check, request, decision, grant persistence, and audit event behavior because those decisions must protect the core loop and SDK mutation APIs consistently.

Host applications remain responsible for the approval surface: show the `ApprovalRequest`, collect a reviewer decision, and call `ResolvePermission` or `ResolveApproval` depending on whether the host only wants to record a grant or also resume a suspended SDK run.

## Public Contract

- `CheckPermission(ctx, PermissionCheckRequest)` returns `allowed`, `denied`, or `requires_approval`.
- `ResolvePermission(ctx, PermissionDecisionRequest)` resolves an approval and records the matching grant.
- `ResolveApproval(ctx, approvalID, decision)` is a compatibility wrapper over `ResumeRun` for SDK-generated approvals.
- `PermissionSnapshot(ctx, PermissionSnapshotQuery)` returns currently effective active grants, pending decisions, denied decisions, resource rules, policy hash, and warnings for SDK/UI projection.
- `PermissionSnapshot` is run-scoped and requires an existing run id; it does not fabricate an empty permission state for missing runs.
- Tool approval requests include tool schema snapshot/hash/epoch metadata when available, so host approval UI can display and audit the exact schema boundary.
- Model or plan approval requests are persisted as `model.approval` continuations, so they use the same `ResolveApproval` path as tool approvals.
- `PermissionRequiredError` is returned by synchronous SDK mutation APIs when a caller must route approval through the host.
- `WriteFile` and `EvaluateProposal` now use the same PermissionCenter path as tool execution.
- Checks and decisions scoped to a `RunID` reject unknown runs before creating permission requests, reviewer decisions, grants, or permission events. Non-read-only checks require an existing run because they can authorize SDK side effects.
- Non-read-only permission checks are gated by session lifecycle before grant reuse or permission-mode shortcuts, so stopped/stopping sessions cannot keep executing SDK side effects through existing grants, `acceptEdits`, or `yolo`.
- Pending approvals projected through `PermissionSnapshot` become deny-only while the owning session is `stopping` or `stopped`, matching the runtime rule that non-deny approval resume is blocked but deny can close a suspended run.

## Modes

- `default`: allows read-only actions; side-effecting tools and workspace writes require approval.
- `acceptEdits`: allows workspace-write actions for the current run, while other non-read-only actions still require approval.
- `yolo`: records an all-actions grant for the current run and allows SDK-visible actions.

## Decisions And Grant Scope

- `deny`: reviewer denies the current approval.
- `allow_once`: grants only the exact action.
- `allow_for_run`: grants the same capability/resource for the current run.
- `allow_for_session`: grants the same capability/resource for the current session.
- `allow_all_for_run`: grants all capabilities for the current run.
- `allow_all_for_session`: grants all capabilities for the current session.

Grant ids are deterministic from actor, capability, resource, action/run/session scope, and decision. This lets the first version match grants without adding list-grant APIs.
Session-scoped grants are canonicalized without run, root-run, task, or action identifiers so they can be reused across runs in the same session. The SDK intentionally does not provide a default unscoped global `allow_all`; process-wide policy belongs in the host application or an explicit SDK permission mode such as `yolo`.

## Current Loop Behavior

1. The model requests tool calls.
2. The loop normalizes each `ToolCall`.
3. `CheckPermission` gates non-read-only actions by session lifecycle before evaluating permission modes, existing grants, and built-in deny rules.
4. `allowed` executes the tool and emits normal tool events.
5. `requires_approval` persists a permission request plus the pending tool continuation, emits `permission.requested`, and suspends the run.
6. `denied` emits `permission.denied` and closes the run with `denied`.
7. `ResolveApproval`/`ResumeRun` with an allow decision records the grant, restores the pending tool continuation, executes pending tool calls, sends observations through the model loop, and completes or re-suspends the run. If the caller supplies a mismatched run/session/root scope, the resume is rejected with `approval_scope_mismatch` before the approval is resolved. If the session is already `stopping` or `stopped`, the allow path is rejected before an active grant is recorded.
8. `ResolveApproval` with a deny decision records the denial and closes the suspended run with `denied` without invoking the tool. Deny remains valid during session drain so a suspended run can terminate.

`CheckPermission` applies the same stopped/stopping session gate even when an existing run/session grant would otherwise match. Read-only checks remain allowed because they do not authorize SDK side effects.

`PermissionSnapshot.ActiveGrants` is an effective current-state projection, not a raw grant history. When a session is `stopping` or `stopped`, historical non-read-only grants remain in the event journal but are omitted from `ActiveGrants`; the snapshot adds a warning and a `session_lifecycle` resource rule so hosts do not infer stale authorization.

`PermissionSnapshot.PendingDecisions` is also session-lifecycle-aware. During `stopping` or `stopped`, each pending approval's `AvailableDecisions` is filtered to deny decisions only and the snapshot includes a warning. This keeps approval UI and WorldState governance projection from advertising allow choices that `ResolveApproval`/`ResumeRun` will reject.

Model or plan approval follows the same outer path. When `ModelProvider` returns `ModelResponse.ApprovalRequest`, the loop fills run/session scope, persists a `model.approval` permission record plus loop continuation, emits `permission.requested`, and suspends. An allow decision appends an approval-granted model message and resumes the next model turn without requiring a `ToolProvider`; a deny decision closes the run with `denied`.

## Persistence

- `PermissionRecord` stores approval request state, run id, subject, encoded scope/action/grant, optional pending tool continuation, request time, decision state, reviewer, and resolved time.
- `CapabilityRecord` stores the granted capability and serialized grant scope.
- `GrantRecord` stores deterministic grant ids and round-trips granted/expiry timestamps.

## Projection

`Inspect` and WorldState governance/capability partitions use the same `PermissionSnapshot` projection. Capability state includes authorization reason, matched grant id/scope, required grants, and policy hash so hosts do not need to infer permission state from raw events. Stopped/stopping sessions mark non-read-only capabilities unauthorized even if a historical grant exists, and governance pending-decision entries expose the same deny-only choices as `PermissionSnapshot`.

## Current Limit

Host applications still own the reviewer UX, reviewer identity, and any product-specific approval copy. The SDK only owns the canonical approval record, grant, event, and run continuation semantics.

## Evidence

- `pkg/rtagent/approval_resume.go`
- `pkg/rtagent/permission_center.go`
- `pkg/rtagent/permission_center_policy.go`
- `pkg/rtagent/permission_center_grants.go`
- `pkg/rtagent/permission_center_store.go`
- `pkg/rtagent/permission_center_events.go`
- `pkg/rtagent/permission_center_types.go`
- `pkg/rtagent/loop.go`
- `pkg/rtagent/loop_tools.go`
- `pkg/rtagent/loop_outcomes.go`
- `pkg/rtagent/runtime.go`
- `pkg/rtagent/types_constants.go`
- `pkg/rtagent/types_runtime.go`
- `pkg/rtagent/types_permission.go`
- `pkg/rtagent/types_tool.go`
- `pkg/rtagent/interfaces.go`
- `pkg/rtagent/permission_center_test.go`
- `internal/domain/persistence/records_governance.go`
- `internal/domain/persistence/stores.go`
- `internal/infrastructure/persistence/sqlite/adapters/permission_governance_store.go`
- `internal/runtime/events/events.go`
- `internal/runtime/worldstate/builder.go`
