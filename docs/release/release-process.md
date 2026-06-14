# Release Process

## Read When

- Preparing a v1 release branch, tag, or external repository publication.
- Closing the remaining gates in `docs/release/v1-readiness.md`.
- Changing module path, release naming, README status, validation scripts, or packaging rules.

## Owner

Runtime/SDK owner.

## Update Trigger

- Final repository import path or release name is chosen.
- Release validation commands change.
- Packaging policy changes, including generated artifacts, clean branch requirements, or DashScope integration status.

## Validation

- `bash scripts/validate_sdk.sh`
- `bash scripts/release_preflight.sh`
- `while IFS= read -r script; do bash -n "$script"; done < <(find scripts -type f -name '*.sh' | sort)`
- `bash scripts/set_module_path.sh --check <final module path>`
- `bash scripts/set_module_path.sh --dry-run <final module path>`
- `bash scripts/check_public_api_snapshot.sh`
- `bash scripts/audit_sdk_boundary.sh`
- `bash scripts/audit_sdk_shape.sh`
- `bash scripts/audit_sdk_docs.sh`
- `bash scripts/audit_sdk_examples.sh`

## v1 Release Steps

1. Choose the final external identity:
   - Product name: `RTAgent Runtime SDK` unless owner decides otherwise.
   - Go module path: final repository import path, not local `rtagent`.
   - Package import path: expected host import path for `pkg/rtagent`.
2. Update `go.mod` and all Go imports to the final module path:
   - `bash scripts/set_module_path.sh --check <final module path>`
   - `bash scripts/set_module_path.sh --dry-run <final module path>`
   - `bash scripts/set_module_path.sh <final module path>`
3. Update README status from `v0.2 / v1-candidate` to `v1.0` only after the remaining release gates close.
4. Run local validation:
   - `bash scripts/validate_sdk.sh`
5. Run release preflight:
   - `RTAGENT_RELEASE_MODULE_PATH=<final module path> bash scripts/release_preflight.sh`
6. Decide DashScope live gate:
   - Run `RTAGENT_RUN_DASHSCOPE_INTEGRATION=1 go test ./pkg/rtagent -run TestDashScopeQwen37PlusIntegration -count=1 -v` when credentials and network are available, or record it as a non-blocking external-provider gate.
7. Prepare a reviewed clean release branch or tag. Do not publish from a dirty worktree.
8. Record the final release decision in `docs/release/v1-readiness.md`.

## Release Preflight Rules

`scripts/release_preflight.sh` is expected to fail before v1 while blockers remain. It checks:

- `go.mod` uses a final repository import path, not a local name or invalid path shape.
- Final module path validation rejects local names, paths without a domain-like first segment, URL forms, host:port forms, credential-bearing paths, whitespace, double slashes, and trailing slashes.
- Optional `RTAGENT_RELEASE_MODULE_PATH` matches `go list -m`.
- Worktree is clean.
- Generated binary/database artifacts are not tracked and do not exist in the repository root.
- Package docs reference `docs/api/public-compatibility.md`.
- Public API snapshot matches `docs/api/public-api.snapshot.txt`.
- SDK boundary audit passes.
- SDK shape audit passes.
- SDK docs audit passes.
- SDK examples audit passes.
- README and `docs/release/v1-readiness.md` status match the module path state: no premature v1.0 on a local module path, and no stale v0.2/v1-candidate status after the final module path is applied.

## Current Blockers

- Final Go module path has not been selected.
- Worktree is intentionally dirty while SDK extraction work is in progress.
- README and `docs/release/v1-readiness.md` must remain v0.2 / v1-candidate until release identity and packaging are finalized.
- Final release-candidate validation must be rerun after module path and README status change.

## Evidence

- `go.mod`
- `README.md`
- `scripts/validate_sdk.sh`
- `scripts/check_public_api_snapshot.sh`
- `scripts/audit_sdk_boundary.sh`
- `scripts/audit_sdk_shape.sh`
- `scripts/audit_sdk_docs.sh`
- `scripts/audit_sdk_examples.sh`
- `scripts/update_public_api_snapshot.sh`
- `scripts/set_module_path.sh`
- `scripts/lib/module_path.sh`
- `scripts/release_preflight.sh`
- `docs/release/v1-readiness.md`
- `docs/api/public-compatibility.md`
