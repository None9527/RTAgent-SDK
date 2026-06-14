#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "usage: $0 [--dry-run|--check] <final-go-module-path>" >&2
}

DRY_RUN=0
CHECK_ONLY=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  shift
elif [[ "${1:-}" == "--check" ]]; then
  CHECK_ONLY=1
  shift
fi

if [[ "$#" -ne 1 ]]; then
  usage
  exit 64
fi

NEW_MODULE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
source "$SCRIPT_DIR/lib/module_path.sh"
cd "$ROOT"

if ! rtagent_is_final_module_path "$NEW_MODULE"; then
  echo "final module path should be a repository import path, not a local name: $NEW_MODULE" >&2
  exit 64
fi

if [[ "$CHECK_ONLY" -eq 1 ]]; then
  echo "module path is valid for release: $NEW_MODULE"
  exit 0
fi

OLD_MODULE="$(go list -m)"
if [[ "$OLD_MODULE" == "$NEW_MODULE" ]]; then
  echo "module path already set to $NEW_MODULE"
  exit 0
fi

matched_files="$(rg -l -F "\"$OLD_MODULE/" --glob '*.go' . || true)"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry run: would update module path: $OLD_MODULE -> $NEW_MODULE"
  echo "Dry run: would run: go mod edit -module \"$NEW_MODULE\""
  if [[ -n "$matched_files" ]]; then
    echo "Dry run: would rewrite imports in:"
    printf '%s\n' "$matched_files"
  else
    echo "Dry run: no Go imports currently reference \"$OLD_MODULE/\""
  fi
  echo "Dry run: would run gofmt on Go files and go mod tidy"
  exit 0
fi

echo "Updating module path: $OLD_MODULE -> $NEW_MODULE"
go mod edit -module "$NEW_MODULE"

if [[ -n "$matched_files" ]]; then
  while IFS= read -r file; do
    OLD_MODULE="$OLD_MODULE" NEW_MODULE="$NEW_MODULE" perl -0pi -e 's/"\Q$ENV{OLD_MODULE}\E\//"$ENV{NEW_MODULE}\//g' "$file"
  done <<< "$matched_files"
fi

go_files=()
while IFS= read -r file; do
  go_files+=("$file")
done < <(rg --files --glob '*.go')
if [[ "${#go_files[@]}" -gt 0 ]]; then
  gofmt -w "${go_files[@]}"
fi
go mod tidy

echo "Module path updated. Run: bash scripts/validate_sdk.sh"
