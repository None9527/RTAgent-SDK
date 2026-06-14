#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export GOCACHE="${GOCACHE:-/private/tmp/rtagent-go-cache}"
HOST_RESUME_DB_USER_SET=0
if [[ -n "${RTAGENT_HOST_RESUME_DB+x}" ]]; then
  HOST_RESUME_DB_USER_SET=1
fi
HOST_RESUME_DB="${RTAGENT_HOST_RESUME_DB:-/tmp/rtagent-sdk-validate-$$.db}"
HOST_RESUME_SESSION="${RTAGENT_HOST_RESUME_SESSION:-sdk-validation-session}"

cleanup() {
  if [[ "$HOST_RESUME_DB_USER_SET" -eq 0 ]]; then
    rm -f "$HOST_RESUME_DB" "$HOST_RESUME_DB-shm" "$HOST_RESUME_DB-wal"
  fi
}
trap cleanup EXIT

echo "==> go doc ./pkg/rtagent"
go doc ./pkg/rtagent >/dev/null

echo "==> bash -n scripts/**/*.sh"
while IFS= read -r script; do
  bash -n "$script"
done < <(find scripts -type f -name '*.sh' | sort)

echo "==> module path validation cases"
source scripts/lib/module_path.sh
valid_module_paths=(
  "example.com/rtagent/sdk"
  "github.com/example/rtagent"
)
for module_path in "${valid_module_paths[@]}"; do
  if ! rtagent_is_final_module_path "$module_path"; then
    echo "expected final module path to be accepted: $module_path" >&2
    exit 1
  fi
  bash scripts/set_module_path.sh --check "$module_path" >/dev/null
done
invalid_module_paths=(
  "rtagent"
  "none/rtagent"
  "localhost/rtagent"
  "http://example.com/rtagent"
  "github.com/example/rtagent/"
  "/github.com/example/rtagent"
  ".github.com/example/rtagent"
  "github.com/example//rtagent"
  "github.com:3000/example/rtagent"
  "user@github.com/example/rtagent"
  "github.com/example/rt agent"
)
for module_path in "${invalid_module_paths[@]}"; do
  if rtagent_is_final_module_path "$module_path"; then
    echo "expected invalid module path to be rejected: $module_path" >&2
    exit 1
  fi
  if bash scripts/set_module_path.sh --check "$module_path" >/dev/null 2>&1; then
    echo "expected module path check to reject: $module_path" >&2
    exit 1
  fi
done

echo "==> module path migration dry run"
bash scripts/set_module_path.sh --dry-run example.com/rtagent/sdk >/dev/null

echo "==> public API snapshot"
bash scripts/check_public_api_snapshot.sh

echo "==> SDK boundary audit"
bash scripts/audit_sdk_boundary.sh

echo "==> SDK shape audit"
bash scripts/audit_sdk_shape.sh

echo "==> SDK docs audit"
bash scripts/audit_sdk_docs.sh

echo "==> SDK examples audit"
bash scripts/audit_sdk_examples.sh

echo "==> go test ./... -count=1"
go test ./... -count=1

echo "==> go test ./... -race -count=1"
go test ./... -race -count=1

echo "==> go vet ./..."
go vet ./...

echo "==> go run ./cmd/rtagent"
go run ./cmd/rtagent >/dev/null

echo "==> go run ./examples/minimal_runtime"
go run ./examples/minimal_runtime >/dev/null

echo "==> go run ./examples/approval_resume"
go run ./examples/approval_resume >/dev/null

echo "==> go run ./examples/mcp_skill_inventory"
go run ./examples/mcp_skill_inventory >/dev/null

echo "==> host resume first run"
go run ./examples/host_resume_cli \
  --db "$HOST_RESUME_DB" \
  --session "$HOST_RESUME_SESSION" \
  --input "first turn" \
  --graph >/dev/null

echo "==> host resume second run"
go run ./examples/host_resume_cli \
  --db "$HOST_RESUME_DB" \
  --resume "$HOST_RESUME_SESSION" \
  --input "next turn" \
  --graph >/dev/null

echo "SDK validation passed"
