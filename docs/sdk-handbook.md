# RTAgent-SDK Operation Handbook

**Module:** `github.com/None9527/RTAgent-SDK`
**Version:** v0.0.1
**Package:** `github.com/None9527/RTAgent-SDK/pkg/rtagent`

This handbook is the single comprehensive guide for embedding, configuring, extending, and operating the RTAgent runtime SDK. It consolidates the architecture, API, contract, and boundary documentation into one reference.

---

## Table of Contents

1. [Conceptual Model](#1-conceptual-model)
2. [Quick Start](#2-quick-start)
3. [Configuration](#3-configuration)
4. [Core API Reference](#4-core-api-reference)
5. [The Run Loop](#5-the-run-loop)
6. [Event Journal and Replay](#6-event-journal-and-replay)
7. [WorldState Projection](#7-worldstate-projection)
8. [Permission and Approval](#8-permission-and-approval)
9. [Session Lifecycle and Resume](#9-session-lifecycle-and-resume)
10. [Checkpoint Graph](#10-checkpoint-graph)
11. [Extending via Tentacles](#11-extending-via-tentacles)
12. [Upper-Layer Orchestration](#12-upper-layer-orchestration)
13. [Convergence Control](#13-convergence-control)
14. [Context Budget](#14-context-budget)
15. [Model Provider Contract](#15-model-provider-contract)
16. [Concurrency Contract](#16-concurrency-concurrency-contract)
17. [Public Compatibility Policy](#17-public-compatibility-policy)
18. [Validation and Release](#18-validation-and-release)
19. [Glossary](#19-glossary)

---

## 1. Conceptual Model

RTAgent-SDK is an **embeddable agent runtime kernel** — a reliable execution loop with event-sourced state, projection-based world model, and host-owned capabilities. It is not a product shell, a framework, or an orchestration engine.

### Layered architecture

```
┌─────────────────────────────────────────────────────────┐
│  Upper-layer orchestration (host-built, replaceable)     │
│  DAG scheduling · multi-agent · plan-execute · ToT       │
├─────────────────────────────────────────────────────────┤
│  RTAgent-SDK kernel (this SDK)                           │
│  event journal · run loop · convergence · permission ·   │
│  checkpoint/resume · WorldState projection · session     │
├─────────────────────────────────────────────────────────┤
│  Tentacles (host-provided capabilities)                  │
│  tools · memory · MCP · skills · file intelligence       │
├─────────────────────────────────────────────────────────┤
│  Truth sources (external world)                          │
│  file system · external memory stores · MCP servers      │
└─────────────────────────────────────────────────────────┘
```

### Five core principles

1. **Event sourcing — the journal is the only truth.** Every state change (run start, model turn, tool call, permission decision, session lifecycle) is an immutable event. The journal is the single source of truth; everything else is derived. This gives full trajectory replay, deterministic projection, and no hidden mutations.

2. **WorldState is a dynamic projection, not a truth store.** WorldState (capabilities, activities, governance, context, memory, tasks) is a read-side projection derived from the event journal and host-provided data sources. Nobody writes to WorldState directly. It is recomputed on demand with projection-aware caching. The kernel never owns your business truth — your files, memory, and tool side effects are the truth; the kernel observes and projects.

3. **The runtime agent grows naturally from real projections.** An agent is a loop (model → tool → observation → repeat) that grows organically from the data sources you wire in. The kernel does not decide what the agent "knows" — you do, by injecting providers.

4. **Single-threaded kernel, extensible foundation.** The SDK is one narrow kernel: one reliable run loop, one event journal, one projection engine, one permission boundary. It does not bake in a specific orchestration paradigm (DAG, plan-and-execute, multi-agent). Those are upper-layer concerns built on the kernel's run/session/checkpoint primitives.

5. **Capabilities are tentacles, not kernel internals.** Tools, memory, MCP servers, skills, and file intelligence are external data sources and capability providers that the kernel observes through well-defined ports. They are not compiled into the kernel.

---

## 2. Quick Start

### Install

```bash
go get github.com/None9527/RTAgent-SDK/pkg/rtagent@v0.0.1
```

### Minimal run

```go
package main

import (
    "context"
    "fmt"
    rtagent "github.com/None9527/RTAgent-SDK/pkg/rtagent"
)

func main() {
    ctx := context.Background()
    rt, err := rtagent.Open(ctx, rtagent.Config{})
    if err != nil { panic(err) }
    defer rt.Close()

    projection, err := rt.SubmitRun(ctx, rtagent.SubmitRunRequest{
        RunID:     "run-1",
        SessionID: "session-1",
        Input:     "hello runtime sdk",
    }, rtagent.Identity{ActorID: "user"})
    if err != nil { panic(err) }

    fmt.Printf("status=%s output=%s\n", projection.Status, projection.Output)
}
```

> Zero-config `Open(ctx, Config{})` uses ephemeral in-memory SQLite. For durable storage, see [Configuration](#3-configuration).

### Smoke command

```bash
go run ./cmd/rtagent
```

---

## 3. Configuration

The SDK is configured through a two-part `Config` struct separating kernel infrastructure from host integration ports.

### Config struct

```go
type Config struct {
    Runtime    RuntimeConfig  // kernel infrastructure settings
    Host       HostPorts      // host-provided capability providers
    RuntimeHome RuntimeHome   // optional: durable home directory resolver
}
```

### RuntimeConfig (kernel infrastructure)

| Field | Type | Default | Purpose |
|---|---|---|---|
| `DSN` | `string` | ephemeral in-memory SQLite | SQLite data source name. Empty = in-memory (lost on exit). Set a file path for durability. |
| `WorkDir` | `string` | current working directory | Workspace root for file operations. |
| `MaxToolIterations` | `int` | 32 | Hard cap on tool-call rounds. Convergence control fires a pre-flush finalize one iteration before this limit. |
| `MaxContextMessages` | `int` | 0 (auto-derive from model) | Explicit message-count window for context budget. 0 = derive from provider's `ContextWindowTokens`. Negative clamped to 0. |
| `RunLeaseTTL` | `time.Duration` | 5 minutes | Run lease time-to-live for crash recovery. |

### HostPorts (capability providers)

| Field | Type | Purpose |
|---|---|---|
| `Model` | `ModelProvider` | The model provider for the run loop. Required for real use; defaults to a deterministic placeholder. |
| `Tools` | `[]ToolProvider` | Tool providers composed via `ToolRegistry`. |
| `Memory` | `MemoryProvider` | Read-only memory facts source. |
| `Hypothesis` | `HypothesisProvider` | Read-only hypothesis facts source. |
| `MCP` | `MCPProvider` | MCP server inventory projection. |
| `Skill` | `SkillProvider` | Skill inventory projection. |
| `WorldState` | `[]WorldStateProvider` | Custom WorldState partition providers. |

### RuntimeHome (durable storage)

`RuntimeHome` is a host-injected resolver that owns the directory convention. When `DSN` is empty and `RuntimeHome` is set, the kernel uses the resolved layout for durable storage. When both are empty, ephemeral in-memory is used (zero-breakage).

```go
rt, _ := rtagent.Open(ctx, rtagent.Config{
    RuntimeHome: rtagent.DefaultUserHome("myagent"),
})
// Creates ~/.myagent/ with db/, workspace/, skills/, memory/, config/
// SQLite at db/myagent.db
```

The kernel **never creates directories** — the `RuntimeHome` implementation (e.g. `DefaultUserHome`) owns creation. Hosts can implement `RuntimeHome` for any convention.

Priority: explicit `DSN` > `RuntimeHome` resolved DSN > ephemeral in-memory.

`Runtime.Home()` returns the resolved `RuntimeHomeLayout` so tentacles can read shared paths (`SkillsDir`, `MemoryDir`, `ConfigDir`).

---

## 4. Core API Reference

The `Runtime` facade exposes 22 public methods grouped by responsibility:

### Lifecycle

| Method | Signature | Purpose |
|---|---|---|
| `Open` | `(ctx, Config) (*Runtime, error)` | Create and wire a runtime. |
| `Close` | `() error` | Idempotent close. All subsequent calls return `ErrRuntimeClosed`. |

### Run intake

| Method | Signature | Purpose |
|---|---|---|
| `SubmitRun` | `(ctx, SubmitRunRequest, Identity) (RuntimeStateProjection, error)` | App-level entry. Drives the full core loop. |
| `Run` | `(ctx, RuntimeCommand) (RuntimeStateProjection, error)` | Lower-level full-loop entry for hosts that build `RuntimeCommand`. |
| `InterruptRun` | `(ctx, runID) (InterruptRunResult, error)` | Cancel a non-terminal run. Idempotent for terminal runs. |

### Journal and events

| Method | Signature | Purpose |
|---|---|---|
| `Emit` | `(ctx, RuntimeEventDraft) (RuntimeEventEnvelope, error)` | Append a runtime event. Run-scoped: rejects unknown run IDs. |
| `ListEvents` | `(ctx, EventQuery) ([]RuntimeEventEnvelope, error)` | List events by `RunID` or aggregate all in a `SessionID`. |

### Read-side projections

| Method | Signature | Purpose |
|---|---|---|
| `Inspect` | `(ctx, InspectQuery) (RuntimeInspectSnapshot, error)` | Full run projection (status, events, WorldState, permission). |
| `InspectSession` | `(ctx, SessionQuery) (SessionSnapshot, error)` | Session-level projection with resume readiness. |
| `SessionGraph` | `(ctx, SessionGraphQuery) (SessionGraphSnapshot, error)` | Run graph within a session, with root-branch filtering. |
| `CheckpointGraph` | `(ctx, CheckpointGraphQuery) (CheckpointGraphSnapshot, error)` | Run-internal checkpoint graph with resume readiness. |
| `PermissionSnapshot` | `(ctx, PermissionSnapshotQuery) (PermissionSnapshot, error)` | Effective permission state (grants, pending, denied, rules). |
| `WorldState` | `(ctx, WorldStateQuery) (WorldStateSnapshot, error)` | Derived operational state projection. |

### Session lifecycle

| Method | Signature | Purpose |
|---|---|---|
| `StopSession` | `(ctx, StopSessionRequest) (StopSessionResult, error)` | Cancel active runs (`cancel_active`) or drain (`drain`). |

### Resume and approval

| Method | Signature | Purpose |
|---|---|---|
| `ResumeRun` | `(ctx, ResumeRunRequest) (RuntimeStateProjection, error)` | Resume from checkpoint/approval continuation. |
| `ResolveApproval` | `(ctx, approvalID, decision) (RuntimeStateProjection, error)` | Resolve an SDK-generated approval and resume the run. |

### Permission

| Method | Signature | Purpose |
|---|---|---|
| `CheckPermission` | `(ctx, PermissionCheckRequest) (PermissionCheckResult, error)` | Check if an action is allowed, denied, or requires approval. |
| `ResolvePermission` | `(ctx, PermissionDecisionRequest) (PermissionDecisionResult, error)` | Record a reviewer decision and grant; does not resume the run. |

### Context and workspace

| Method | Signature | Purpose |
|---|---|---|
| `RegisterContextHandle` | `(ctx, ContextHandle) error` | Register an external context reference. |
| `MaterializeContext` | `(ctx, handleID) (string, error)` | Materialize a registered context handle. |
| `WriteFile` | `(ctx, WriteFileRequest) (ArtifactRecord, error)` | Audited workspace write (gated by permission). |
| `EvaluateProposal` | `(ctx, agentID, action, activityID) error` | Evaluate a governance proposal. |

---

## 5. The Run Loop

The core loop is the heart of the kernel. When you call `SubmitRun`, the following sequence executes:

```
SubmitRun(ctx, req, identity)
  │
  ├─ 1. Normalize request (run ID, session ID, permission mode, planning state)
  ├─ 2. Initialize run (create run/session records, acquire lease)
  ├─ 3. Emit session.started, run.created, turn.started
  ├─ 4. Assemble context packet (tools, memory, events, handles)
  │     └─ persist tool schema snapshot
  ├─ 5. Emit context.packet.created, agent.started
  │
  └─ runModelToolLoop ──────────────────────────────────────┐
       │                                                      │
       for each iteration:                                    │
       ├─ 6. Apply context budget (trim messages to window)   │
       ├─ 7. If finalizing: strip tools, request final answer │
       ├─ 8. Call ModelProvider.CompleteTurn                   │
       │     └─ stream deltas persisted as model.delta events │
       ├─ 9. Emit model.requested / model.responded           │
       ├─ 10. If approval requested: suspend run ──────────── ►│ (exit)
       ├─ 11. If no tool calls: complete run ──────────────── ►│ (exit)
       ├─ 12. If iteration limit: finalize (convergence) ──── ►│ (exit)
       ├─ 13. Execute tool calls (with permission gates)       │
       │     ├─ CheckPermission per call                       │
       │     ├─ If denied: denyRun ────────────────────────── ►│ (exit)
       │     ├─ If approval required: suspend ─────────────── ►│ (exit)
       │     ├─ ExecuteTool → ToolObservation                  │
       │     └─ Emit tool.invoked / tool.succeeded / failed    │
       ├─ 14. Convergence control observes the turn           │
       │     ├─ Repeat detection → replan steering message     │
       │     ├─ No-progress detection → replan steering        │
       │     └─ Hard-budget pre-flush → finalize               │
       └─ loop ────────────────────────────────────────────────┘
```

### Loop exits (always graceful)

The loop never ends with a hard failure (`tool_iteration_limit_exceeded`). It always finds a graceful exit:

1. **Natural completion** — model returns no tool calls → `completeRun`.
2. **Convergence finalize** — hard-budget pre-flush strips tools and requests a final answer → `completeRun`.
3. **Approval suspension** — model or permission requests approval → `suspendRun` (resumable).
4. **Permission denial** — action denied → `denyRun`.
5. **Error** — model/tool/persistence failure → `failRun`.

---

## 6. Event Journal and Replay

### Event sourcing model

Every state change is an immutable event appended to the journal. Events have:

- **EventID**: unique per event.
- **RunID / SessionID**: scope anchoring.
- **Kind**: stable `EventKind*` constant (e.g. `run.created`, `tool.succeeded`).
- **Sequence**: run-local monotonic counter. SQLite enforces unique `(run_id, sequence)`.
- **OccurredAt**: caller-supplied or auto-assigned RFC3339 timestamp.
- **Payload**: structured, redaction-aware metadata.

### Event kinds (SDK-owned)

The SDK owns these event kinds. Hosts can emit custom event strings via `Emit` for any existing run.

| Category | Kinds |
|---|---|
| Run lifecycle | `run.created`, `run.interrupted`, `run.heartbeat` |
| Session | `session.started`, `session.ended` |
| Turn | `turn.started`, `turn.completed`, `turn.failed`, `turn.cancelled` |
| Model | `model.requested`, `model.responded`, `model.delta` |
| Tools | `tool.invoked`, `tool.succeeded`, `tool.failed` |
| Activity | `activity.started`, `activity.completed` |
| Context | `context.packet.created`, `context.compacted` |
| Permission | `permission.requested`, `permission.granted`, `permission.denied` |
| Agent | `agent.started`, `agent.plan.proposed` |
| Checkpoint | `checkpoint.created` |

### Replay

Because the journal is the single source of truth, any run can be fully replayed:

```go
events, _ := rt.ListEvents(ctx, rtagent.EventQuery{RunID: "run-1"})
// events is the complete ordered trajectory of the run
```

WorldState, Inspect, and all read-side projections are derived from this trajectory. There is no separate "stored state" to sync with.

### Sequence allocation

`Emit` serializes sequence allocation within one `Runtime` instance via an internal mutex. SQLite enforces unique `(run_id, sequence)` at the DB level as a backstop. Multi-process SQLite writers are not a committed v1 capability.

---

## 7. WorldState Projection

### What it is

WorldState is the agent's view of its current operational state — a derived, cached, partition-aware read model. It is **not** a truth store. It is recomputed from the event journal and host-provided data sources.

### Partitions

| Partition | Source | Content |
|---|---|---|
| `memory` | `MemoryProvider` + runtime memory records | Committed durable memory facts |
| `hypothesis` | `HypothesisProvider` | Proposed (unconfirmed) memory |
| `capability` | Tool schema snapshot | Available tools with authorization state |
| `activity` | Events (`activity.started/completed`) | Active/completed runtime activities |
| `task` | Events | Runtime trajectory and task state |
| `context` | Context packet + handles | Current workspace, context packets, handles |
| `governance` | `PermissionSnapshot` | Permission rules, grants, pending approvals |
| `artifact` | Legacy compatibility | Workspace artifacts |

### Querying

```go
snapshot, _ := rt.WorldState(ctx, rtagent.WorldStateQuery{
    RunID:     "run-1",
    Partition: "capability",  // optional: narrow to one partition
})
```

- Unfiltered queries are served from the in-memory cache (O(1) when fresh).
- Partition-filtered queries bypass the cache (external providers may narrow by partition).
- `WorldStateQuery.Partition` filters typed `Partitions`, top-level `Handles`, and legacy `Entries`.

### Determinism

Stable (deterministic for identical input): `SnapshotID`, `RuntimeEpoch`, `SourceWatermark`, partition order, entry/handle/warning sorting.

Non-deterministic (wall-clock): `BuildID`, `GeneratedAt`, `BuiltAt`. Hosts must not compare these for equality.

### Read cache (adaptive)

The cache uses **projection-aware freshness**: only events that can change a WorldState partition invalidate it. High-frequency events (`run.heartbeat`, `model.delta`, `checkpoint.created`, etc.) do NOT invalidate. This makes host `Inspect` calls cheap during an active loop.

### How tool execution flows into WorldState

```
ToolProvider.ExecuteTool → ToolObservation
  ↓
executeToolCall emits: tool.invoked → tool.succeeded (or tool.failed)
  ↓
These are projection-relevant events → advance cache watermark
  ↓
WorldState recompute: activity/task/capability partitions updated automatically
```

Tentacles do not need to manually update WorldState. The kernel observes tool results through events and projects them automatically.

---

## 8. Permission and Approval

### Permission modes

| Mode | Behavior |
|---|---|
| `default` | Side-effecting actions require approval. Read-only actions allowed. |
| `acceptEdits` | Workspace writes allowed without approval (per-run). Other side effects still gated. |
| `yolo` | All actions allowed (per-run). Use with caution. |

### Check flow

```go
result, _ := rt.CheckPermission(ctx, rtagent.PermissionCheckRequest{
    Scope: scope,
    Action: rtagent.ProposedAction{
        ActionID: "write:abc123",
        Kind:     rtagent.PermissionCapabilityWorkspaceWrite,
        Target:   "path/to/file",
    },
})
// result.Status is "allowed", "denied", or "requires_approval"
```

### Approval decisions

| Decision | Scope | Effect |
|---|---|---|
| `deny` | — | Action denied. |
| `allow_once` | single call | One-time grant. |
| `allow_for_run` | run | All future calls to this tool in this run. |
| `allow_for_session` | session | All future calls to this tool in this session. |
| `allow_all_for_run` | run | All tools in this run. |
| `allow_all_for_session` | session | All tools in this session. |

### Resuming from approval

When the loop suspends for approval, the host resolves it and the run resumes:

```go
// Option A: resolve and resume (the run continues)
projection, _ := rt.ResolveApproval(ctx, approvalID, "allow_for_run")

// Option B: resolve without resuming (grant only)
result, _ := rt.ResolvePermission(ctx, rtagent.PermissionDecisionRequest{...})
```

### Session lifecycle gates

Stopped/stopping sessions:
- Block non-read-only actions even with existing grants.
- `PermissionSnapshot` and WorldState treat their grants as inactive.
- Pending approvals expose deny-only choices.
- `ResolveApproval` with non-deny is blocked; deny remains allowed so a suspended run can terminate.

---

## 9. Session Lifecycle and Resume

### Model

- A **session** contains multiple **runs**.
- A **run** is one core-loop execution.
- Runs form a graph via `RootRunID` and `ParentRunID`.
- Public `SessionID` maps to internal `ThreadRecord.ResumeID`.

### InspectSession

```go
snapshot, _ := rt.InspectSession(ctx, rtagent.SessionQuery{SessionID: "session-1"})
// snapshot.CanResume, snapshot.ExternalResumeReady, snapshot.ResumeCommandHint
```

This gives a CLI/frontend everything needed to implement `--resume <session_id>` outside the core loop.

### SessionGraph

```go
graph, _ := rt.SessionGraph(ctx, rtagent.SessionGraphQuery{
    SessionID: "session-1",
    RootRunID: "run-root",  // optional: filter to a root branch
})
```

### StopSession

| Mode | Behavior |
|---|---|
| `cancel_active` | Immediately cancel all active runs in the session. |
| `drain` | Reject new runs, preserve active runs, auto-stop when the last active run completes. |

### External resume pattern

The SDK does not own a `--resume` command. The host implements it:

1. Call `InspectSession` to check `CanResume` / `ExternalResumeReady`.
2. Submit a new run with the same `SessionID`.
3. The kernel attaches it to the existing session graph.

See `examples/host_resume_cli` for a complete demo.

---

## 10. Checkpoint Graph

### What it is

The loop checkpoints its continuation state at key points: context packet, model request, model response, tool call, tool observation, approval pending, terminal. These checkpoints form a graph within a run.

### Querying

```go
graph, _ := rt.CheckpointGraph(ctx, rtagent.CheckpointGraphQuery{RunID: "run-1"})
// graph.Nodes — checkpoint nodes with ResumeReady flags
// graph.Warnings — lifecycle warnings
```

### Resume readiness

`ResumeReady` is an **effective projection**, not raw metadata. It is suppressed (false) with warnings when:
- The run is terminal (completed/failed/canceled).
- The owning session is `stopping` or `stopped`.

### ResumeRun

```go
projection, _ := rt.ResumeRun(ctx, rtagent.ResumeRunRequest{
    RunID:      "run-1",
    ApprovalID: "approval-1",  // optional
    Decision:   "allow_for_run",
})
```

`ResumeRun` rejects scope mismatches (run/session/root overrides that differ from the original approval continuation) before resuming.

---

## 11. Extending via Tentacles

Tentacles are host-owned capability providers. The kernel observes their data through ports; it does not own it.

### Model Provider

```go
provider, _ := rtagent.NewOpenAICompatibleProvider(rtagent.OpenAICompatibleProviderConfig{
    BaseURL:             "https://dashscope.aliyuncs.com/compatible-mode/v1",
    APIKey:              os.Getenv("DASHSCOPE_API_KEY"),
    Model:               "qwen3.6-plus",
    ContextWindowTokens: 131072,
})
```

When `ContextWindowTokens` is set, the provider declares `ModelCapabilities` and the kernel auto-derives a context-message budget.

For function-based providers:

```go
rt, _ := rtagent.Open(ctx, rtagent.Config{
    Host: rtagent.HostPorts{
        Model: rtagent.ModelProviderWithCapabilities{
            Inner: rtagent.ModelProviderFunc(myTurnFunc),
            Caps:  rtagent.ModelCapabilities{ContextWindowTokens: 32768},
        },
    },
})
```

### Tool Provider (the agent's hands)

```go
type myToolProvider struct{}
func (p *myToolProvider) ToolSpecs(ctx context.Context, scope rtagent.ExecutionScope) ([]rtagent.ToolSpec, error) { ... }
func (p *myToolProvider) ExecuteTool(ctx context.Context, scope rtagent.ExecutionScope, call rtagent.ToolCall) (rtagent.ToolObservation, error) { ... }
```

Tools write to **real external state** (file system, process, API). The kernel observes the result as `ToolObservation`, appends events, and WorldState updates automatically. Multiple providers are composed via `Config.Host.Tools` (a `ToolRegistry`); duplicate names get `namespace__name` aliases.

### Memory Provider (the agent's recall)

```go
rtagent.MemoryProviderFunc(func(ctx context.Context, scope rtagent.ExecutionScope) ([]rtagent.MemoryFact, error) {
    return loadMemoryFromExternalStore(scope)
})
```

Memory is an **external data source**. The kernel calls `MemoryFacts` during context packet assembly. The kernel never writes memory — it only reads what your memory system provides.

### MCP / Skill Providers

```go
rtagent.MCPProviderFunc(func(ctx context.Context, scope rtagent.ExecutionScope) ([]rtagent.CapabilityInventoryItem, error) { ... })
rtagent.SkillProviderFunc(func(ctx context.Context, scope rtagent.ExecutionScope) ([]rtagent.CapabilityInventoryItem, error) { ... })
```

These project inventory (what's available) into WorldState. Execution still goes through `ToolProvider` and `PermissionCenter`.

### WorldState Providers (custom projections)

```go
rtagent.WorldStateProviderAdapter{
    PartitionName: "my-custom",
    Build: func(ctx context.Context, input rtagent.WorldStateProviderInput) (rtagent.WorldStatePartition, error) { ... },
}
```

Add custom read-side projections without touching the write path.

### Wiring with RuntimeHome

```go
rt, _ := rtagent.Open(ctx, rtagent.Config{
    RuntimeHome: rtagent.DefaultUserHome("myagent"),
    Host: rtagent.HostPorts{
        Model:  provider,
        Tools:  []rtagent.ToolProvider{fileTools, shellTools},
        Skill: rtagent.SkillProviderFunc(func(ctx, scope) ([]rtagent.CapabilityInventoryItem, error) {
            return loadSkills(rt.Home().SkillsDir)
        }),
    },
})
```

---

## 12. Upper-Layer Orchestration

The kernel provides reliable single-run execution. Multi-run patterns are built **on top** of it:

| Pattern | Kernel primitives used |
|---|---|
| **Session continuity** | Multiple `SubmitRun` with same `SessionID`. `InspectSession`, `SessionGraph`. |
| **DAG / multi-step** | Sub-runs with `ParentRunID` / `RootRunID`. `CheckpointGraph` per branch. |
| **Plan-and-execute** | Run 1 produces `PlanArtifact`. Runs 2+ execute steps. `ResumeRun` for suspend/continue. |
| **Human-in-the-loop** | Loop suspends on `ApprovalRequest`. `ResolveApproval` / `ResumeRun` to continue. |
| **Multi-agent** | Separate `Runtime` instances or distinct profiles. Communicate via external state. |
| **Reflection** | Run 1 produces output. Run 2 (same session) critiques. Full event journal available. |

The kernel deliberately does not freeze any of these patterns. You compose them from `SubmitRun`, `ResumeRun`, `SessionGraph`, `CheckpointGraph`, and `ListEvents`.

---

## 13. Convergence Control

The loop tracks tool-interaction signatures to detect when the model is stuck and steers it toward a graceful exit.

### Mechanisms

| Mechanism | Trigger | Action |
|---|---|---|
| **Repeat detection** | Same tool call + observation signature seen ≥3 times | Replan steering message (tools stay enabled). Deduped per reason. |
| **No-progress detection** | ≥3 consecutive turns with no novel signature, past iteration 12 | Replan steering message. Deduped per reason. |
| **Hard-budget pre-flush** | `MaxToolIterations - 1` | Strip tools, inject finalization message, force text answer. Run completes. |

### Guarantee

The loop **always finds a graceful exit**. A run never ends with `tool_iteration_limit_exceeded` — it either completes naturally, replans, or finalizes.

### Thresholds (conservative, not host-tunable in v0.0.1)

- Repeat threshold: 3
- No-progress floor: iteration 12
- No-progress streak: 3
- Hard-budget pre-flush: `MaxToolIterations - 1`

---

## 14. Context Budget

### Purpose

Prevents the conversation message history from growing unbounded and overflowing the model's context window.

### Budget sources (priority order)

1. **Explicit `RuntimeConfig.MaxContextMessages`** — exact message-count window. Host wants precise control.
2. **Provider-declared context window** — when `MaxContextMessages` is unset and the provider declares `ContextWindowTokens > 0`, the kernel derives a budget automatically.
3. **No trimming** — when neither is available.

### Derivation heuristic (source 2)

`budget = (ContextWindowTokens × 0.75) / 500`

25% reserved for system prompt, tool schemas, and output. ~500 tokens per message assumed. Conservative; explicit `MaxContextMessages` always overrides.

### Trimming policy

When exceeded, the loop keeps the first `role:"user"` message (task context) plus the most recent `budget-1` messages. Older middle messages are dropped. Each trim emits a `context.compacted` event.

### Characteristics

- Message-count window, not token budget (kernel avoids tokenizer coupling).
- Irreversible and resume-visible (checkpoint stores trimmed state).
- Opt-in by default (0 = no trimming unless provider declares capabilities).

---

## 15. Model Provider Contract

### The single contract

```go
type ModelProvider interface {
    CompleteTurn(ctx context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error)
}
```

- `stream == nil`: non-streaming.
- `stream != nil`: provider emits `ModelStreamEvent` deltas and still returns the final `ModelResponse`.
- Stable stream event types: `ModelStreamEventTextDelta` (`text_delta`), `ModelStreamEventToolCallDelta` (`tool_call_delta`).

### Capabilities (optional)

Providers can optionally implement `ModelCapabilityProvider`:

```go
type ModelCapabilityProvider interface {
    Capabilities() ModelCapabilities
}
```

`ModelCapabilities` carries `ContextWindowTokens`, `MaxOutputTokens`, `SupportsStreaming`. The kernel uses `ContextWindowTokens` to derive the context budget.

### Retry and failure semantics

The SDK **does not retry** provider failures. `CompleteTurn` issues a single call. Failures surface as `ModelProviderError` with structured `ModelProviderErrorDetails` (provider, status, code, retryable, rate-limited, safe-for-model, body preview). Hosts own retry policy. A failed turn transitions the run to terminal `failed`.

### OpenAI-compatible provider

`NewOpenAICompatibleProvider` implements the contract over Chat Completions-compatible HTTP APIs. DashScope helpers: `NewDashScopeQwen37PlusProviderFromEnv` (reads `DASHSCOPE_API_KEY`, `DASHSCOPE_BASE_URL`, `DASHSCOPE_MODEL`).

---

## 16. Concurrency Contract

### What is safe

- `Runtime.Emit` is sequence-serialized within one `Runtime` instance (internal mutex around sequence allocation + journal append). Concurrent `Emit` calls get contiguous, non-overlapping sequences.
- `Runtime.Close` is idempotent and safe to call concurrently.

### What is not guaranteed

- Other `Runtime` facade methods (`SubmitRun`, `Run`, `Inspect`, `ListEvents`, `WorldState`, etc.) carry **no v1 concurrency guarantee**. A single `Runtime` instance is intended for use by one owner; hosts dispatching from multiple goroutines must serialize those calls themselves.
- Multi-process writers sharing a single SQLite DSN are not a committed v1 capability.

### Validation

Concurrent behavior is regression-covered by `TestRuntimeConcurrentEmitAndSubmitRunAreRaceFree` and `-race` validation in `scripts/validate_sdk.sh`.

---

## 17. Public Compatibility Policy

### Stable v1 surface (once module path is finalized)

The following are intended to be stable for v1:

- Runtime lifecycle: `Open`, `Runtime.Close`, `ErrRuntimeClosed`.
- Main run facade: `SubmitRun`, `Run`, `InterruptRun`, `SubmitRunRequest`, `RuntimeCommand`, `RuntimeStateProjection`, `RuntimeError`.
- Journal/read: `Emit`, `ListEvents`, `Inspect`, `RuntimeEventDraft`, `RuntimeEventEnvelope`, `EventQuery`, `InspectQuery`.
- Session/checkpoint: `InspectSession`, `SessionGraph`, `StopSession`, `CheckpointGraph`, `ResumeRun`, `ResolveApproval`.
- Config/ports: `Config`, `RuntimeConfig`, `HostPorts`, `ExecutionScope`, `Identity`.
- Model contract: `ModelProvider`, `ModelCapabilities`, `ModelCapabilityProvider`, `ModelRequest`, `ModelResponse`, `ModelStreamHandler`.
- Tool contract: `ToolProvider`, `ToolRegistry`, `ToolSpec`, `ToolCall`, `ToolObservation`.
- Permission: `PermissionCenter`, permission types, modes, grant scopes.
- WorldState: `WorldStateSnapshot`, partitions, entries, handles, capability state.
- Host ports: `MemoryProvider`, `HypothesisProvider`, `MCPProvider`, `SkillProvider`, `WorldStateProvider`.
- RuntimeHome: `RuntimeHome`, `RuntimeHomeLayout`, `DefaultUserHome`.

### Additive changes allowed in v1.x

- Add optional fields, types, constants, constructors, methods.
- Add new WorldState partitions, event kinds, payload fields, provider detail fields.
- Hosts should ignore unknown fields.

### Breaking changes require v2

- Removing/renaming exported symbols.
- Changing signatures.
- Adding methods to stable interfaces.
- Changing required fields, enum values, grant semantics, model/tool history semantics.

### Non-contract surfaces

- Anything under `internal/`.
- The unexported `runtimeKernel`, startup bootstrap, concrete SQLite adapters.
- Provider-specific error implementations (use `ModelProviderError`).
- Concrete SQLite schema shape.
- Exact error string wording.

---

## 18. Validation and Release

### Local validation

```bash
go test ./...                     # all tests
go test ./... -race -count=1      # with race detector
go vet ./...                      # static analysis
bash scripts/validate_sdk.sh      # full SDK validation (tests, vet, audits, examples)
bash scripts/release_preflight.sh # release gate checks
```

### Audits

| Script | Checks |
|---|---|
| `audit_sdk_boundary.sh` | Single ModelProvider contract, no legacy Execute, no public startup/persistence leakage |
| `audit_sdk_shape.sh` | File-size budgets, package split policy |
| `audit_sdk_docs.sh` | Docs index integrity, required metadata |
| `audit_sdk_examples.sh` | Example entrypoint style, validation coverage |
| `check_public_api_snapshot.sh` | Public API surface matches snapshot |

### Release gates

See `docs/release/v1-readiness.md` for the complete gate table. v0.0.1 has closed: module path, naming, README, validation, packaging. Remaining for v1.0: real-model multi-turn tool convergence, tentacle coverage.

---

## 19. Glossary

| Term | Definition |
|---|---|
| **Kernel** | The RTAgent-SDK runtime core: run loop, journal, projection, permission, session, checkpoint. |
| **Journal** | The immutable event log. The single source of truth for runtime state. |
| **WorldState** | A derived read-side projection of the agent's operational state. Not a truth store. |
| **Run** | One execution of the core loop (model → tool → observation → terminal). |
| **Session** | A collection of runs sharing continuity. |
| **Tentacle** | A host-owned capability provider (tool, memory, MCP, skill) plugged into a kernel port. |
| **Truth source** | External state the kernel observes but does not own (files, memory stores, MCP servers). |
| **Projection** | A derived view computed from the journal and host data sources. |
| **Checkpoint** | A run-internal continuation state point for resume. |
| **Approval** | A permission gate that suspends the run until a human/host decision. |
| **Convergence** | The loop's mechanism for detecting stalls and steering toward a graceful exit. |
| **Context budget** | A message-count window that bounds conversation history to prevent context overflow. |
| **RuntimeHome** | A host-injected resolver for the persistent home directory layout. |

---

*This handbook is the consolidated reference. For deeper detail on individual contracts, see the focused docs under `docs/api/` and `docs/architecture/`.*

## Read When

- Embedding RTAgent-SDK in a Go process for the first time.
- Looking up a specific API method, contract type, or behavior without hunting across multiple docs.
- Onboarding a new contributor who needs the full SDK surface in one place.
- Reviewing design principles (event sourcing, WorldState projection, tentacles, orchestration).

## Owner

Runtime/SDK owner.

## Update Trigger

- Any public `pkg/rtagent` API, type, method, constant, or contract changes.
- Design principles, convergence control, context budget, or WorldState cache behavior changes.
- A new tentacle type or orchestration pattern is documented.
- Release version or module path changes.

## Validation

- `go test ./...`
- `go vet ./...`
- `bash scripts/validate_sdk.sh`
- `bash scripts/audit_sdk_docs.sh`
