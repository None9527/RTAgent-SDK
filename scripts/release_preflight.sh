#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/lib/module_path.sh"
cd "$ROOT"

failures=0

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

pass() {
  echo "PASS: $*"
}

module_path="$(go list -m)"
if [[ "$module_path" == "rtagent" ]]; then
  fail "go.mod still uses local module path 'rtagent'; set final repository import path before v1"
elif ! rtagent_is_final_module_path "$module_path"; then
  fail "go module path '$module_path' is not a final repository import path"
else
  pass "go module path is not local-only: $module_path"
fi

if [[ -n "${RTAGENT_RELEASE_MODULE_PATH:-}" ]] && ! rtagent_is_final_module_path "$RTAGENT_RELEASE_MODULE_PATH"; then
  fail "RTAGENT_RELEASE_MODULE_PATH '$RTAGENT_RELEASE_MODULE_PATH' is not a final repository import path"
fi

if [[ -n "${RTAGENT_RELEASE_MODULE_PATH:-}" && "$module_path" != "$RTAGENT_RELEASE_MODULE_PATH" ]]; then
  fail "go module path '$module_path' does not match RTAGENT_RELEASE_MODULE_PATH '$RTAGENT_RELEASE_MODULE_PATH'"
fi

if [[ -n "$(git status --porcelain)" ]]; then
  fail "worktree is dirty; prepare a reviewed clean release branch/tag before publishing"
  git status --short >&2
else
  pass "worktree is clean"
fi

tracked_generated="$(git ls-files | rg '(^rtagent$|^rtagent-console$|^rtagent\.db$|\.sqlite$|\.sqlite3$|\.test$)' || true)"
if [[ -n "$tracked_generated" ]]; then
  fail "generated binary/database artifacts are tracked: $tracked_generated"
else
  pass "no tracked root binary/database artifacts detected"
fi

root_generated="$(find . -maxdepth 1 -type f \( -name 'rtagent' -o -name 'rtagent-console' -o -name 'rtagent.db' -o -name '*.sqlite' -o -name '*.sqlite3' -o -name '*.test' \) -print | sort)"
if [[ -n "$root_generated" ]]; then
  echo "$root_generated" >&2
  fail "generated binary/database artifacts exist in the repository root; remove them before release packaging"
else
  pass "no root generated binary/database artifacts detected"
fi

if go doc ./pkg/rtagent | rg -q 'docs/sdk-handbook.md'; then
  pass "package docs reference the SDK handbook"
else
  fail "package docs do not reference docs/sdk-handbook.md"
fi

if api_snapshot_output="$(bash scripts/check_public_api_snapshot.sh 2>&1)"; then
  pass "public API snapshot matches"
else
  echo "$api_snapshot_output" >&2
  fail "public API snapshot drifted"
fi

if boundary_audit_output="$(bash scripts/audit_sdk_boundary.sh 2>&1)"; then
  pass "SDK boundary audit passed"
else
  echo "$boundary_audit_output" >&2
  fail "SDK boundary audit failed"
fi

if shape_audit_output="$(bash scripts/audit_sdk_shape.sh 2>&1)"; then
  pass "SDK shape audit passed"
else
  echo "$shape_audit_output" >&2
  fail "SDK shape audit failed"
fi

if docs_audit_output="$(bash scripts/audit_sdk_docs.sh 2>&1)"; then
  pass "SDK docs audit passed"
else
  echo "$docs_audit_output" >&2
  fail "SDK docs audit failed"
fi

if examples_audit_output="$(bash scripts/audit_sdk_examples.sh 2>&1)"; then
  pass "SDK examples audit passed"
else
  echo "$examples_audit_output" >&2
  fail "SDK examples audit failed"
fi

if [[ "$module_path" == "rtagent" ]]; then
  if rg -q 'Current status: v1\.0' README.md; then
    fail "README claims v1.0 while module path remains local-only"
  fi
  if rg -q 'Current status: v1\.0|\*\*Version:\*\* v1\.0' README.md docs/sdk-handbook.md; then
    fail "README or handbook claims v1.0 while module path remains local-only"
  fi
  pass "release identity docs do not contradict local module path"
else
  if rg -q 'Current status: v0\.2' README.md; then
    fail "README still claims v0.2 after module path was changed; update release status before tagging v1"
  fi
  if rg -q '\*\*Version:\*\* v0\.2|v1-candidate internal SDK|not a final `v1\.0` release' docs/sdk-handbook.md; then
    fail "handbook still claims v0.2/v1-candidate after module path was changed; update release identity before tagging v1"
  fi
  pass "release identity docs are compatible with finalized module path"
fi

if [[ "$failures" -gt 0 ]]; then
  echo "Release preflight failed with $failures blocker(s)." >&2
  exit 1
fi

echo "Release preflight passed"
