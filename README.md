# RTAgent Runtime SDK

RTAgent is a Go runtime SDK extracted from ngoagent project experience. It provides an embeddable agent runtime loop, not a full product shell.

Current status: v0.2, usable for internal host integration and dogfooding. The v1 public boundary is still being stabilized.

## What It Owns

- Runtime lifecycle through `rtagent.Open` and `Runtime.Close`.
- App-level command intake through `SubmitRun`.
- A runnable model/tool loop with context packet assembly, model calls, tool calls, permission gates, checkpoints, and terminal run state.
- Read-side projections: `Inspect`, `InspectSession`, `SessionGraph`, `CheckpointGraph`, `PermissionSnapshot`, and `WorldState`.
- Host ports for model, tools, memory, hypothesis, MCP inventory, skill inventory, and custom WorldState projection.
- Small function adapters for host-provided model/tool/projection ports, so simple integrations do not need boilerplate structs.
- An OpenAI Chat Completions-compatible provider, including DashScope compatible mode for `qwen3.7-plus`.

Product shells such as a TUI, HTTP server, desktop app, frontend, or `--resume <session_id>` command live outside the SDK and call these APIs.

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

## Host Resume Example

The SDK keeps session and run state. A host CLI or frontend owns the user-facing resume command.

Create or continue a session:

```bash
go run ./examples/host_resume_cli \
  --db /tmp/rtagent-host-resume-demo.db \
  --session demo-session \
  --input "first turn" \
  --graph
```

Resume the same session from another process:

```bash
go run ./examples/host_resume_cli \
  --db /tmp/rtagent-host-resume-demo.db \
  --resume demo-session \
  --input "next turn" \
  --graph
```

The example calls `InspectSession`, verifies external resume readiness, submits a new run with the same `SessionID`, then prints the updated session graph.

Hosts can also call `ListEvents` with `EventQuery{SessionID: ...}` to read all run events in a session, or `Inspect` with `InspectQuery{SessionID: ...}` to inspect the latest run in that session.

## Host Provider Adapters

Hosts can implement SDK ports with regular Go types, or use lightweight adapters for small integrations and tests:

- `ModelProviderFunc`
- `ToolProviderAdapter`
- `MemoryProviderFunc`
- `HypothesisProviderFunc`
- `MCPProviderFunc`
- `SkillProviderFunc`
- `WorldStateProviderAdapter`

The examples under `examples/approval_resume` and `examples/mcp_skill_inventory` use these adapters to keep host wiring focused on behavior.

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

Check the public SDK API surface:

```bash
bash scripts/check_public_api_snapshot.sh
```

Audit the SDK boundary before release:

```bash
bash scripts/audit_sdk_boundary.sh
```

Audit SDK package shape and file-size budgets:

```bash
bash scripts/audit_sdk_shape.sh
```

Audit SDK docs index and required metadata:

```bash
bash scripts/audit_sdk_docs.sh
```

Audit SDK examples and validation coverage:

```bash
bash scripts/audit_sdk_examples.sh
```

Release preflight before tagging or publishing:

```bash
RTAGENT_RELEASE_MODULE_PATH=<final module path> bash scripts/release_preflight.sh
```

After the final repository path is chosen, migrate Go imports with:

```bash
bash scripts/set_module_path.sh --dry-run <final module path>
bash scripts/set_module_path.sh <final module path>
```

Optional real DashScope integration:

```bash
RTAGENT_RUN_DASHSCOPE_INTEGRATION=1 go test ./pkg/rtagent -run TestDashScopeQwen37PlusIntegration -count=1 -v
```

## Important Boundaries

- Zero-config `Open(ctx, Config{})` uses ephemeral in-memory SQLite. For durable storage, either pass `RuntimeConfig.DSN` explicitly or inject `Config.RuntimeHome` (e.g. `DefaultUserHome("myagent")`) to resolve a persistent `~/.myagent/` directory automatically.
- Run-scoped projections such as `PermissionSnapshot` and `WorldState` require an existing run id; use `SubmitRun` or `Run` to create run state first.
- WorldState is a source-watermarked read projection, not a truth store.
- Exported `EventKind*` constants cover SDK-owned runtime events; hosts can still emit custom journal event strings through `Runtime.Emit` for an existing run.
- MCP and skill providers project inventory into WorldState; execution still goes through `ToolProvider` and `PermissionCenter`.
- The default model provider is deterministic and local. Real hosts should inject a real `ModelProvider`.
- Kernel/store injection is not public in v1; use `Config.Host` ports for host extension.
- Shared multi-process SQLite writers are not a committed v1 capability yet.

## Runtime Home

`Config.RuntimeHome` is the customization seam for "where does this runtime live on disk." It is a host-injected resolver that owns the directory convention (location, structure, permissions, creation). The SDK never creates directories itself.

- **Durable zero-config:** inject `DefaultUserHome("myagent")` and `Open(ctx, Config{RuntimeHome: ...})` resolves `~/.myagent/` (or `$MYAGENT_HOME`) with `db/`, `workspace/`, `skills/`, `memory/`, `config/` subdirs and a durable SQLite at `db/myagent.db`.
- **Fully customizable:** implement the `RuntimeHome` interface (or use `RuntimeHomeFunc`) for any convention — custom location, permissions, layout, or config-file format.
- **Priority:** explicit `RuntimeConfig.DSN` wins; `RuntimeHome` is consulted only when DSN is empty.
- **Shared layout for tentacles:** `Runtime.Home()` returns the resolved `RuntimeHomeLayout`, so host providers (skill, memory, MCP) can read `SkillsDir`/`MemoryDir`/`ConfigDir` to locate their data sources under a common root.

```go
rt, _ := rtagent.Open(ctx, rtagent.Config{
    RuntimeHome: rtagent.DefaultUserHome("myagent"),
    Host: rtagent.HostPorts{
        Skill: rtagent.SkillProviderFunc(func(ctx context.Context, scope rtagent.ExecutionScope) ([]rtagent.CapabilityInventoryItem, error) {
            return loadSkills(rt.Home().SkillsDir) // host reads the shared layout
        }),
    },
})
```

## Docs

- v1 readiness: `docs/release/v1-readiness.md`
- Release process: `docs/release/release-process.md`
- SDK architecture: `docs/architecture/sdk-core.md`
- Public compatibility: `docs/api/public-compatibility.md`
- Public API snapshot: `docs/api/public-api.snapshot.txt`
- Model providers: `docs/api/model-providers.md`
- Tool providers: `docs/api/tool-providers.md`
- Permission center: `docs/api/permission-center.md`
- Session lifecycle: `docs/api/session-lifecycle.md`
- WorldState: `docs/api/world-state.md`
