#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

OUT="docs/api/public-api.snapshot.txt"

go doc -all ./pkg/rtagent | sed -E 's#// import "[^"]+/pkg/rtagent"#// import "<module>/pkg/rtagent"#' > "$OUT"
echo "updated $OUT"
