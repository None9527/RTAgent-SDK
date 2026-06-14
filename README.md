# RTAgent Runtime SDK

A Go runtime SDK for building agents. It provides an embeddable agent runtime kernel — a reliable execution loop with event-sourced state, projection-based world model, and host-owned capabilities — not a product shell or a framework that owns your business logic.

**Current status: v0.0.1.** The kernel is complete and published. Tentacles (tools, memory, MCP, skills) and upper-layer orchestration are the next phase.

**Module:** `github.com/None9527/RTAgent-SDK`

## Design Philosophy

RTAgent-SDK is extracted from the [ngoagent](https://github.com/None9527/NGOAgent) project, carrying forward its core design principles while stripping away product-shell coupling. The ideas below are not aspirational — they are implemented in the code you are reading.

### 1. Event Sourcing: the journal is the only truth

Every state change — run start, model turn, tool call, permission decision, session lifecycle — is an immutable event appended to a journal. The journal is the single source of truth. Nothing else "writes" runtime state; everything else is derived from it.

This gives you, for free:
- **Full trajectory replay.** Every run can be replayed event-by-event from the journal. You can reconstruct any past state, debug any decision, audit any action.
- **Deterministic projection.** WorldState (below) is recomputed from events, not stored as mutable state. Two hosts reading the same events see the same world.
- **No hidden mutations.** If it changed runtime state, there is an event for it. If there is no event, it did not happen.

### 2. WorldState: a dynamic projection, not a truth store

WorldState is the agent's view of the current operational state — capabilities, activities, governance, context, memory, tasks. It is a **read-side projection** derived from the event journal and host-provided data sources. Nobody writes to WorldState directly. It is recomputed on demand, and cached with projection-aware invalidation.

This separation (events = truth, WorldState = derived view) is the CQRS/event-sourcing pattern applied to agent runtime. It means:
- The kernel never owns your business truth. Your files, your memory system, your tool side effects — those are the truth. The kernel observes and projects.
- WorldState is always consistent with the journal. There is no "sync" problem.
- You can add new partitions, new projections, new read models without touching the write path.

### 3. The runtime agent grows naturally from real projections

An agent is not a hardcoded pipeline. It is a loop (model → tool → observation → repeat) that grows organically from the projections it has access to. The more data sources (memory, skills, MCP servers, file intelligence) you wire in as host providers, the richer the agent's world becomes — and the better its decisions.

The kernel does not decide what the agent "knows." You do, by injecting providers. The kernel provides the reliable execution substrate; the agent emerges from the data you feed it.

### 4. Single-threaded SDK kernel, extensible foundation

The SDK is intentionally a **single, narrow kernel**: one reliable run loop, one event journal, one projection engine, one permission boundary. It does not bake in a specific orchestration paradigm (DAG, plan-and-execute, tree-of-thought, multi-agent). Those are **upper-layer concerns** built on top of the kernel's run/session/checkpoint primitives.

```
┌─────────────────────────────────────────────────────────┐
│  Upper-layer orchestration (host-built, replaceable)     │
│  DAG scheduling · multi-agent · plan-execute · ToT       │
├─────────────────────────────────────────────────────────┤
│  RTAgent-SDK kernel (this repo)                          │
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

Each layer is replaceable:
- **Kernel** stays small and stable. It is the foundation you build on.
- **Tentacles** are host-owned providers. The kernel observes their data; it does not own it.
- **Orchestration** is entirely host territory. The kernel gives you `SubmitRun`, `ResumeRun`, `SessionGraph`, `CheckpointGraph` — you compose them into whatever paradigm your product needs.

### 5. Capabilities are tentacles, not kernel internals

Tools, memory, MCP servers, skills, file intelligence — these are **external data sources and capability providers** that the kernel observes through well-defined ports. They are not compiled into the kernel. This means:
- You choose your memory backend (vector DB, SQLite, flat files — your call).
- You choose your tool implementations (shell, file I/O, custom services).
- You choose your model provider (DashScope, OpenAI, Anthropic, local — your call).
- The kernel never holds business truth. It projects, it does not own.

## What the Kernel Owns

- **Runtime lifecycle** through `rtagent.Open` and `Runtime.Close`.
- **App-level command intake** through `SubmitRun`.
- **A runnable model/tool loop** with context packet assembly, model calls, tool calls, permission gates, convergence control, context budget management, checkpoints, and terminal run state.
- **Event-sourced journal** — every state change is an immutable event; runs are fully replayable.
- **WorldState projection** — a derived, cached, partition-aware read model of the agent's operational state.
- **Session lifecycle** — `InspectSession`, `SessionGraph`, `StopSession`, `InterruptRun`.
- **Checkpoint/resume** — `CheckpointGraph`, `ResumeRun`, `ResolveApproval` for suspending and continuing runs.
- **Permission boundary** — `PermissionCenter` gates tool execution, workspace writes, and governance proposals.
- **Host ports** for model, tools, memory, hypothesis, MCP inventory, skill inventory, and custom WorldState projection.
- **Model capability declaration** — providers declare their context window; the kernel derives a context-message budget automatically.
- **RuntimeHome** — a host-injected directory resolver for durable, zero-config storage.

Product shells (TUI, HTTP server, desktop app, `--resume <session_id>` command) live outside the SDK and call these APIs.

## Extending the Kernel: Tentacles

The kernel is useful on its own, but an agent needs capabilities. Tentacles are host-owned providers that plug into the kernel's ports. Here is how to connect each type:

### Model Provider

```go
provider, _ := rtagent.NewOpenAICompatibleProvider(rtagent.OpenAICompatibleProviderConfig{
    BaseURL:             "https://dashscope.aliyuncs.com/compatible-mode/v1",
    APIKey:              os.Getenv("DASHSCOPE_API_KEY"),
    Model:               "qwen3.6-plus",
    ContextWindowTokens: 131072,
})

rt, _ := rtagent.Open(ctx, rtagent.Config{
    Host: rtagent.HostPorts{Model: provider},
})
```

The provider can optionally declare `ModelCapabilities` (context window, streaming support). When it does, the kernel auto-derives a context-message budget so the loop does not overflow the model's window.

### Tool Providers (the agent's hands)

Tools are how the agent acts on the world — reading files, running commands, calling APIs. Implement `ToolProvider`:

```go
type ToolProvider interface {
    ToolSpecs(ctx context.Context, scope ExecutionScope) ([]ToolSpec, error)
    ExecuteTool(ctx context.Context, scope ExecutionScope, call ToolCall) (ToolObservation, error)
}
```

Tools write to **real external state** (the file system, a process, a remote API). The kernel observes the tool's result as a `ToolObservation`, appends it to the event journal, and the WorldState projection updates. The kernel never holds the file system as internal state — the file system is the truth source; the kernel projects.

### Memory Provider (the agent's long-term recall)

Memory is an **external data source**, not kernel-internal storage. Implement `MemoryProvider`:

```go
type MemoryProvider interface {
    MemoryFacts(ctx context.Context, scope ExecutionScope) ([]MemoryFact, error)
}
```

Your memory backend (vector DB, SQLite, flat files) lives outside the kernel. The kernel calls `MemoryFacts` during context packet assembly to pull relevant memories into the model's view. The kernel never writes memory — it only reads what your memory system provides. This means your memory truth lives where you decide, not trapped in the kernel's DB.

### MCP Provider (Model Context Protocol servers)

```go
type MCPProvider interface {
    MCPInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error)
}
```

MCP servers are external capability sources. The `MCPProvider` projects their inventory (what tools/services are available) into WorldState so the agent knows what it can reach. Execution still goes through `ToolProvider` and `PermissionCenter`.

### Skill Provider

```go
type SkillProvider interface {
    SkillInventory(ctx context.Context, scope ExecutionScope) ([]CapabilityInventoryItem, error)
}
```

Skills are declarative capability bundles (like profiles). The `SkillProvider` projects which skills are available into WorldState. The kernel does not execute skills — it makes them visible to the agent and the host.

### WorldState Providers (custom projections)

```go
type WorldStateProvider interface {
    Partition() string
    BuildWorldState(ctx context.Context, input WorldStateProviderInput) (WorldStatePartition, error)
}
```

If the built-in partitions (memory, capability, activity, task, context, governance, hypothesis) are not enough, you can add custom read-side projections. These feed the WorldState view without touching the write path.

### Wiring it together with RuntimeHome

Tentacles often need a shared directory convention (where do skills live? where is memory stored?). `RuntimeHome` is the host-injected resolver for this:

```go
rt, _ := rtagent.Open(ctx, rtagent.Config{
    RuntimeHome: rtagent.DefaultUserHome("myagent"),
    Host: rtagent.HostPorts{
        Model:  modelProvider,
        Tools:  []rtagent.ToolProvider{fileTools, shellTools},
        Memory: myMemoryProvider,
        Skill:  rtagent.SkillProviderFunc(func(ctx, scope) ([]CapabilityInventoryItem, error) {
            return loadSkills(rt.Home().SkillsDir)
        }),
    },
})
```

`DefaultUserHome` resolves `~/.myagent/` with `db/`, `workspace/`, `skills/`, `memory/`, `config/` subdirectories. Your tentacles read `rt.Home().SkillsDir`, `rt.Home().MemoryDir`, etc. to locate their data sources under a shared root. You can also implement `RuntimeHome` yourself for any convention.

## Extending the Kernel: Upper-Layer Orchestration

The kernel provides reliable single-run execution (`SubmitRun` → model loop → completion). Multi-run patterns are built **on top** of the kernel, using its primitives:

| Pattern | How to build it on the kernel |
|---|---|
| **Session continuity** | Submit multiple runs with the same `SessionID`. The kernel tracks the run graph; `InspectSession` and `SessionGraph` give you the full session trajectory. |
| **DAG / multi-step tasks** | Decompose a task into sub-runs. Each sub-run is a `SubmitRun` with a `ParentRunID` and `RootRunID`. Use `CheckpointGraph` to track where each branch is. |
| **Plan-and-execute** | Run 1 produces a plan (via `PlanArtifact`). Run 2+ execute each step. The kernel's checkpoint/resume lets you suspend and continue at any point. |
| **Human-in-the-loop** | The loop suspends on `ApprovalRequest`. The host UI calls `ResolveApproval` or `ResumeRun` to continue. `PermissionSnapshot` shows pending decisions. |
| **Multi-agent** | Each agent is a separate `Runtime` instance (or a run with a distinct profile). They communicate through shared external state (files, messages) — the kernel does not bake in inter-agent messaging. |
| **Reflection / self-critique** | Run 1 produces output. Run 2 (same session, new input) reviews and critiques it. The full event journal is available for the second run to inspect. |

The kernel deliberately does not freeze any of these patterns. You compose them from `SubmitRun`, `ResumeRun`, `SessionGraph`, `CheckpointGraph`, and `ListEvents`. Different products can use completely different orchestration on the same kernel.

## Quick Start

Run the SDK smoke command:

```bash
go run ./cmd/rtagent
```

Run the minimal host example:

```bash
go run ./examples/minimal_runtime
```

Expected output:

```text
completed: hello runtime sdk
```

## DashScope / OpenAI-Compatible Provider

The provider contract is a single `ModelProvider` interface:

```go
CompleteTurn(ctx context.Context, req rtagent.ModelRequest, stream rtagent.ModelStreamHandler) (rtagent.ModelResponse, error)
```

Use DashScope OpenAI-compatible mode:

```bash
export DASHSCOPE_API_KEY=...
go run ./examples/dashscope_qwen
```

Optional environment variables:

- `DASHSCOPE_MODEL`: defaults to `qwen3.7-plus`.
- `DASHSCOPE_BASE_URL`: defaults to DashScope compatible-mode endpoint.

## Validation

```bash
go test ./...
go vet ./...
```

Full local SDK validation:

```bash
bash scripts/validate_sdk.sh
```

## Important Boundaries

- Zero-config `Open(ctx, Config{})` uses ephemeral in-memory SQLite. For durable storage, inject `Config.RuntimeHome` (e.g. `DefaultUserHome("myagent")`) or pass `RuntimeConfig.DSN` explicitly.
- Run-scoped projections such as `PermissionSnapshot` and `WorldState` require an existing run id; use `SubmitRun` or `Run` to create run state first.
- WorldState is a source-watermarked read projection, not a truth store.
- The kernel never creates directories. `RuntimeHome` implementations (like `DefaultUserHome`) own directory creation.
- MCP and skill providers project inventory into WorldState; execution still goes through `ToolProvider` and `PermissionCenter`.
- The default model provider is deterministic and local. Real hosts should inject a real `ModelProvider`.
- Kernel/store injection is not public in v1; use `Config.Host` ports for host extension.
- Shared multi-process SQLite writers are not a committed v1 capability yet.

## Docs

- v1 readiness: `docs/release/v1-readiness.md`
- Release process: `docs/release/release-process.md`
- SDK architecture: `docs/architecture/sdk-core.md`
- Public compatibility: `docs/api/public-compatibility.md`
- Public API snapshot: `docs/api/public-api.snapshot.txt`
- Model providers (capabilities, budget, convergence, retry): `docs/api/model-providers.md`
- Tool providers: `docs/api/tool-providers.md`
- Permission center: `docs/api/permission-center.md`
- Session lifecycle: `docs/api/session-lifecycle.md`
- WorldState (determinism, read cache): `docs/api/world-state.md`
