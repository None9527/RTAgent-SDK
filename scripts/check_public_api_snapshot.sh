#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SNAPSHOT="docs/api/public-api.snapshot.txt"
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

if [[ ! -f "$SNAPSHOT" ]]; then
  echo "missing $SNAPSHOT; run scripts/update_public_api_snapshot.sh" >&2
  exit 1
fi

go doc -all ./pkg/rtagent | sed -E 's#// import "[^"]+/pkg/rtagent"#// import "<module>/pkg/rtagent"#' > "$TMP"

if ! diff -u "$SNAPSHOT" "$TMP"; then
  echo "public API snapshot drifted; review compatibility and run scripts/update_public_api_snapshot.sh if intentional" >&2
  exit 1
fi

echo "Public API snapshot matches"
