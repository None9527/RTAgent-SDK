#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

pass() {
  echo "PASS: $*"
}

DOC_TMP="$(mktemp)"
trap 'rm -f "$DOC_TMP"' EXIT

go doc -all ./pkg/rtagent > "$DOC_TMP"

model_doc="$(go doc ./pkg/rtagent.ModelProvider)"
if [[ "$model_doc" == *"CompleteTurn(ctx context.Context, req ModelRequest, stream ModelStreamHandler) (ModelResponse, error)"* ]]; then
  pass "ModelProvider uses the single CompleteTurn contract with stream handler"
else
  fail "ModelProvider contract drifted from CompleteTurn(ctx, req, stream)"
fi

if rg -q '\bStreamingModelProvider\b' pkg/rtagent docs/api/public-api.snapshot.txt; then
  fail "legacy StreamingModelProvider surface is present"
else
  pass "no legacy StreamingModelProvider surface"
fi

if rg -q '\b(DatasetStore|DatasetExportRecord|PutDatasetExport|GetDatasetExport)\b' internal pkg docs README.md; then
  fail "unused dataset export persistence surface is present; do not keep no-op store contracts in the SDK core"
else
  pass "no unused dataset export persistence surface"
fi

if rg -q '\b(AuditStore|AuditRecord|PutAuditLog|AuditModel)\b' internal pkg docs README.md; then
  fail "unused audit persistence surface is present; runtime events/artifacts are the current SDK audit trail"
else
  pass "no unused audit persistence surface"
fi

if rg -q '\b(MessageStore|MessageRecord|AppendMessage|MessageModel)\b' internal pkg docs README.md; then
  fail "unused message persistence surface is present; model history belongs in checkpoints/runtime events, not a no-op store"
else
  pass "no unused message persistence surface"
fi

if rg -q '\b(EvidenceStore|EvidenceRecord|AppendEvidence|ListEvidence|EvidenceModel)\b' internal pkg docs README.md; then
  fail "unused evidence persistence surface is present; evidence references should stay projected until a real provider exists"
else
  pass "no unused evidence persistence surface"
fi

if rg -q '\b(TaskStore|TaskRecord|PutTask|GetTask|ListTasksByRunID|TaskModel)\b' internal pkg docs README.md; then
  fail "unused task persistence surface is present; task WorldState is derived from runtime events"
else
  pass "no unused task persistence surface"
fi

if rg -q '\b(DeleteRun|DeleteThread|GetMemory|GetCapability|ListActivitiesByRunID)\b' internal pkg docs README.md; then
  fail "unused persistence method returned; keep internal Bundle limited to runtime consumers"
else
  pass "no unused internal persistence methods"
fi

if rg -q '\bpersistence\.WorldStateEntry\b|\bWorldStateModel\b|\bv2_world_state_entries\b|records_world_state' internal pkg docs README.md; then
  fail "WorldState persistence surface returned; WorldState must remain a derived read projection"
else
  pass "no WorldState persistence surface"
fi

if rg -q '\b(ProjectionQuery|GetWorldStateSnapshot|GetWorldStatePartition|type WorldStateBuilder interface)\b' internal pkg docs README.md || rg -q '\btype WorldStateQuery struct\b' internal/domain/worldstate; then
  fail "unused internal WorldState provider/query interfaces returned; keep only concrete runtime projection contracts"
else
  pass "no unused internal WorldState interfaces"
fi

if rg -q '\b(PartitionMemory|PartitionCapability|PartitionContext|PartitionHypothesis)\b' internal/domain/worldstate; then
  fail "internal WorldState domain duplicated public typed projection partitions"
else
  pass "internal WorldState domain only keeps runtime flat-builder partitions"
fi

if rg -q '\b(HandleMemory|HandleActivity|type Materializer interface)\b' internal pkg docs README.md; then
  fail "unsupported context materializer handle surface returned"
else
  pass "no unsupported context materializer handle surface"
fi

if rg -q 'persistence\.Bundle' internal/runtime; then
  fail "internal runtime component depends on aggregate persistence.Bundle; use a narrow local store interface"
else
  pass "internal runtime components avoid aggregate persistence.Bundle"
fi

if rg -q 'persistence\.Bundle' pkg/rtagent; then
  fail "SDK facade kernel depends on aggregate persistence.Bundle; use a narrow SDK-local store interface"
else
  pass "SDK facade kernel avoids aggregate persistence.Bundle"
fi

startup_imports="$(rg -n '"rtagent/internal/startup"' --glob '*.go' | rg -v '^pkg/rtagent/kernel\.go:' || true)"
if [[ -n "$startup_imports" ]]; then
  echo "$startup_imports" >&2
  fail "internal startup container imports must stay confined to pkg/rtagent/kernel.go"
else
  pass "internal startup container import is confined to SDK kernel bootstrap"
fi

bundle_usages="$(rg -n 'persistence\.Bundle' internal pkg --glob '*.go' | rg -v '^internal/domain/persistence/stores\.go:' | rg -v '^internal/startup/bootstrap\.go:' || true)"
if [[ -n "$bundle_usages" ]]; then
  echo "$bundle_usages" >&2
  fail "aggregate persistence.Bundle must stay confined to the persistence contract and startup composition"
else
  pass "aggregate persistence.Bundle is confined to persistence contract and startup composition"
fi

runtime_doc="$(go doc ./pkg/rtagent.Runtime)"
if [[ "$runtime_doc" == *"func (r *Runtime) Execute("* ]]; then
  fail "Runtime exposes legacy Execute entry"
else
  pass "Runtime does not expose legacy Execute entry"
fi

if rg -q '\bRuntimeContainer\b|gorm\.DB|gorm\.io/|sqlite/adapters|internal/startup' "$DOC_TMP"; then
  fail "public package docs expose internal startup or persistence implementation details"
else
  pass "public package docs do not expose startup/persistence implementation types"
fi

legacy_failures=0
for legacy_path in \
  internal/runtime/api/server.go \
  internal/runtime/execution/governed_executor.go \
  internal/runtime/execution/sandbox.go \
  internal/domain/governance/governance.go
do
  if [[ -e "$legacy_path" ]]; then
    fail "legacy product-shell file still exists: $legacy_path"
    legacy_failures=$((legacy_failures + 1))
  fi
done

if [[ "$legacy_failures" -eq 0 ]]; then
  pass "legacy product-shell files are absent"
fi

if go doc ./pkg/rtagent | rg -q 'docs/api/public-compatibility.md'; then
  pass "package docs reference public compatibility policy"
else
  fail "package docs do not reference docs/api/public-compatibility.md"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "SDK boundary audit failed with $failures issue(s)." >&2
  exit 1
fi

echo "SDK boundary audit passed"
