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

if [[ ! -f docs/INDEX.md ]]; then
  fail "docs/INDEX.md is missing"
fi

required_headings=(
  "## Read When"
  "## Owner"
  "## Update Trigger"
  "## Validation"
)

while IFS= read -r file; do
  for heading in "${required_headings[@]}"; do
    if ! rg -q "^${heading}$" "$file"; then
      fail "$file is missing required heading: $heading"
    fi
  done

  if ! rg -q "\`${file}\`" docs/INDEX.md; then
    fail "$file is not listed in docs/INDEX.md"
  fi
done < <(find docs -type f -name '*.md' ! -path 'docs/INDEX.md' | sort)

if [[ -f docs/INDEX.md ]]; then
  while IFS='|' read -r _ raw_path _; do
    path="$(printf '%s' "$raw_path" | sed -E 's/^[[:space:]]*`?//; s/`?[[:space:]]*$//')"
    if [[ -z "$path" ]]; then
      continue
    fi
    if [[ "$path" =~ ^-+$ ]]; then
      continue
    fi
    if [[ ! -e "$path" ]]; then
      fail "docs/INDEX.md references missing path: $path"
    fi
  done < <(tail -n +4 docs/INDEX.md)
fi

if [[ "$failures" -gt 0 ]]; then
  echo "SDK docs audit failed with $failures issue(s)." >&2
  exit 1
fi

pass "docs index paths exist and markdown docs carry required metadata"
echo "SDK docs audit passed"
