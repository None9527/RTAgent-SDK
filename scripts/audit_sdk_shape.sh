#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

failures=0
max_source_lines="${RTAGENT_MAX_SOURCE_LINES:-350}"
max_test_lines="${RTAGENT_MAX_TEST_LINES:-1200}"
max_adapter_lines="${RTAGENT_MAX_ADAPTER_SOURCE_LINES:-350}"
max_domain_lines="${RTAGENT_MAX_DOMAIN_SOURCE_LINES:-350}"
max_runtime_lines="${RTAGENT_MAX_RUNTIME_SOURCE_LINES:-350}"

fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

pass() {
  echo "PASS: $*"
}

if [[ -e pkg/rtagent/types.go ]]; then
  fail "pkg/rtagent/types.go exists; keep public contracts split by responsibility"
else
  pass "public contract types remain split out of types.go"
fi

while IFS= read -r file; do
  lines="$(wc -l < "$file" | tr -d ' ')"
  if [[ "$lines" -gt "$max_source_lines" ]]; then
    fail "$file has $lines lines; split responsibilities or raise RTAGENT_MAX_SOURCE_LINES intentionally"
  fi
done < <(find pkg/rtagent -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' | sort)

while IFS= read -r file; do
  lines="$(wc -l < "$file" | tr -d ' ')"
  if [[ "$lines" -gt "$max_adapter_lines" ]]; then
    fail "$file has $lines lines; split SQLite adapter responsibilities or raise RTAGENT_MAX_ADAPTER_SOURCE_LINES intentionally"
  fi
done < <(find internal/infrastructure/persistence/sqlite/adapters -maxdepth 1 -type f -name '*.go' | sort)

while IFS= read -r file; do
  lines="$(wc -l < "$file" | tr -d ' ')"
  if [[ "$lines" -gt "$max_domain_lines" ]]; then
    fail "$file has $lines lines; split internal domain contract responsibilities or raise RTAGENT_MAX_DOMAIN_SOURCE_LINES intentionally"
  fi
done < <(find internal/domain/persistence -maxdepth 1 -type f -name '*.go' | sort)

while IFS= read -r file; do
  lines="$(wc -l < "$file" | tr -d ' ')"
  if [[ "$lines" -gt "$max_runtime_lines" ]]; then
    fail "$file has $lines lines; split runtime responsibilities or raise RTAGENT_MAX_RUNTIME_SOURCE_LINES intentionally"
  fi
done < <(find internal/runtime -type f -name '*.go' ! -name '*_test.go' | sort)

while IFS= read -r file; do
  lines="$(wc -l < "$file" | tr -d ' ')"
  if [[ "$lines" -gt "$max_test_lines" ]]; then
    fail "$file has $lines lines; split test scenarios or raise RTAGENT_MAX_TEST_LINES intentionally"
  fi
done < <(find pkg/rtagent -maxdepth 1 -type f -name '*_test.go' | sort)

if [[ "$failures" -gt 0 ]]; then
  echo "SDK shape audit failed with $failures issue(s)." >&2
  exit 1
fi

pass "pkg/rtagent source files are within ${max_source_lines}-line source and ${max_test_lines}-line test budgets"
pass "SQLite adapter files are within ${max_adapter_lines}-line source budget"
pass "internal persistence domain files are within ${max_domain_lines}-line source budget"
pass "internal runtime source files are within ${max_runtime_lines}-line source budget"
echo "SDK shape audit passed"
