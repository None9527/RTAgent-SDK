#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

failures=0
external_examples=(
  "openai_compatible"
)

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

pass() {
  echo "PASS: $*"
}

is_external_example() {
  local name="$1"
  local external
  for external in "${external_examples[@]}"; do
    if [[ "$name" == "$external" ]]; then
      return 0
    fi
  done
  return 1
}

audit_host_entrypoint() {
  local entry_dir="$1"

  if ! rg -q '^func run\(\) error \{' "$entry_dir/main.go"; then
    fail "$entry_dir must expose a run() error entrypoint for validation-friendly host integration"
  fi
  if ! rg -q 'log\.Fatal\(err\)' "$entry_dir/main.go"; then
    fail "$entry_dir main should report run errors with log.Fatal(err)"
  fi
  if rg -q '\bpanic\(' "$entry_dir/main.go"; then
    fail "$entry_dir must not use panic-based host integration"
  fi
  if ! rg -q 'Runtime:\s*rtagent\.RuntimeConfig\{' "$entry_dir/main.go"; then
    fail "$entry_dir must pass explicit RuntimeConfig instead of relying on zero-config runtime defaults"
  fi
  if ! rg -q '\bDSN:\s*' "$entry_dir/main.go"; then
    fail "$entry_dir must pass an explicit RuntimeConfig.DSN"
  fi
  if ! rg -q '\bWorkDir:\s*' "$entry_dir/main.go"; then
    fail "$entry_dir must pass an explicit RuntimeConfig.WorkDir"
  fi
}

if rg -n '"[^"]*/internal/' cmd examples --glob '*.go'; then
  fail "cmd and examples must not import internal packages; host-facing entrypoints should use pkg/rtagent"
fi

fixed_temp_db_refs="$(rg -n 'filepath\.Join\(os\.TempDir\(\),\s*"rtagent[^"]*\.db"\)' cmd examples --glob '*.go' | rg -v 'examples/host_resume_cli/' || true)"
if [[ -n "$fixed_temp_db_refs" ]]; then
  echo "$fixed_temp_db_refs" >&2
  fail "cmd and non-persistent examples must use temporary runtime directories or cleanup, not fixed os.TempDir rtagent*.db paths"
fi

while IFS= read -r main_file; do
  example_dir="$(dirname "$main_file")"
  example_name="$(basename "$example_dir")"

  audit_host_entrypoint "$example_dir"

  if is_external_example "$example_name"; then
    if ! rg -q 'OPENAI_API_KEY|OPENAI_BASE_URL|OPENAI_MODEL' "$main_file" README.md docs/sdk-handbook.md; then
      fail "$example_dir is marked external but lacks documented opt-in credentials/integration gate"
    fi
    continue
  fi

  if ! rg -q "go run ./examples/${example_name}\\b" scripts/validate_sdk.sh; then
    fail "$example_dir is a local example but is not run by scripts/validate_sdk.sh"
  fi
done < <(find examples -mindepth 2 -maxdepth 2 -type f -name main.go | sort)

while IFS= read -r main_file; do
  command_dir="$(dirname "$main_file")"
  command_name="$(basename "$command_dir")"

  audit_host_entrypoint "$command_dir"

  if ! rg -q "go run ./cmd/${command_name}\\b" scripts/validate_sdk.sh; then
    fail "$command_dir is a host-facing command but is not run by scripts/validate_sdk.sh"
  fi
done < <(find cmd -mindepth 2 -maxdepth 2 -type f -name main.go | sort)

if [[ "$failures" -gt 0 ]]; then
  echo "SDK examples audit failed with $failures issue(s)." >&2
  exit 1
fi

pass "local examples and host-facing commands are covered by validate_sdk.sh and entrypoints stay validation-friendly with explicit runtime config"
echo "SDK examples audit passed"
