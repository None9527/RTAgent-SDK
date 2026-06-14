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

if [[ ! -f docs/sdk-handbook.md ]]; then
  fail "docs/sdk-handbook.md is missing"
fi

required_headings=(
  "## Read When"
  "## Owner"
  "## Update Trigger"
  "## Validation"
)

extra_docs="$(find docs -type f ! -path 'docs/sdk-handbook.md' | sort)"
if [[ -n "$extra_docs" ]]; then
  echo "$extra_docs" >&2
  fail "docs/ must only contain docs/sdk-handbook.md"
fi

if [[ -f docs/sdk-handbook.md ]]; then
  for heading in "${required_headings[@]}"; do
    if ! rg -q "^${heading}$" docs/sdk-handbook.md; then
      fail "docs/sdk-handbook.md is missing required heading: $heading"
    fi
  done
fi

if [[ "$failures" -gt 0 ]]; then
  echo "SDK docs audit failed with $failures issue(s)." >&2
  exit 1
fi

pass "docs contains only the SDK handbook and required metadata is present"
echo "SDK docs audit passed"
