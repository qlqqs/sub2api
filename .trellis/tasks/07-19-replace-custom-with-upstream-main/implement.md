# Implementation plan

## 1. Preflight and evidence

- [x] Re-read Git status and stop if unexpected changes appeared after planning.
- [x] Verify available disk space before creating the external preservation snapshot.
- [x] Inspect running Docker state without changing it, so validation does not disturb production or Custom services.
- [x] Record current `custom`, `origin/custom`, local `main`, and cached `upstream/main` object IDs.
- [x] Inventory preserved secret/runtime paths by pathname and existence only; do not print secret contents.

## 2. Refresh upstream baseline

- [x] Fetch only the required official upstream refs/tags without changing the worktree.
- [x] Record the fetched `upstream/main` object ID and recent release/version information.
- [x] Recompare environment/build candidates with fetched upstream, especially `Dockerfile` and frontend package-manager files.
- [x] Finalize the exact preservation allowlist before touching the branch.

## 3. Create rollback protection

- [x] Create a unique local backup branch at the original committed `custom` tip.
- [x] Create a timestamped preservation directory outside `/opt/sub2api` after verifying its parent and available space.
- [x] Save `git diff --binary` for all dirty tracked files into the preservation directory.
- [x] Copy `.trellis/`, `.cursor/`, `.agents/`, `skills/`, `tools/`, and approved tracked environment files while preserving metadata; the snapshot retains the complete pre-cleanup Trellis state for rollback.
- [x] Record the path inventory for `deploy/migration-backups/` and runtime data; do not duplicate multi-gigabyte state unless a collision or filesystem risk requires it.
- [x] Verify the backup branch object ID, saved patch, and copied allowlist before proceeding.

### Rollback gate A

Stop unless both committed history and dirty tracked configuration are recoverable.

## 4. Build replacement branch

- [x] Restore only the snapshotted dirty tracked configuration files needed to permit a safe branch switch.
- [x] Create a uniquely named temporary replacement branch directly from fetched `upstream/main`.
- [x] If an untracked-path collision occurs, stop; copy the path externally and retry without forcing or cleaning.
- [x] Restore `.cursor/`, `.agents/`, `skills/`, and `tools/` in full, plus the reviewed environment allowlist, from the preservation snapshot.
- [x] Restore `.trellis/`, then apply the approved minimal business cleanup: delete the dedicated upstream-balance spec and archived task, and remove their feature-specific links/contracts from surviving specs.
- [x] Preserve `.trellis/workspace/qlqqs/`, the bootstrap archive, upstream-sync history, and isolated deployment/source-development history.
- [x] Keep ignored `.env*`, persistent data, and migration backups in place.
- [x] Retain fetched upstream versions of all non-allowlisted files.

### Rollback gate B

Before renaming branches, confirm the temporary branch can be abandoned and the original `custom` recovered from the backup branch plus dirty patch.

## 5. Validate preservation boundary

- [x] Diff the temporary replacement tree against the recorded fetched `upstream/main` commit.
- [x] Classify every changed/untracked path as workflow tooling, development/deployment configuration, secret/runtime state, or unexpected.
- [x] Stop on any unexpected application-source difference.
- [x] Search `CUSTOM:` markers and verify none remain in retired backend/frontend business code.
- [x] Verify the old upstream-account-balance implementation is absent unless independently present upstream.
- [x] Confirm `.cursor/`, `.agents/`, `skills/`, and `tools/` are complete relative to the snapshot.
- [x] Compare `.trellis/` with the snapshot and require every removal/edit to belong to the approved balance-feature cleanup or fetched-upstream revalidation.
- [x] Confirm no live Trellis specification or manifest links to `.trellis/spec/backend/upstream-balance.md` or the retired balance task.

## 6. Validate environments

- [x] Verify `deploy/.env`, `.env.custom`, and `.env.dev` remain ignored and unstaged when present.
- [x] Verify Custom, development, test, backend data, and migration-backup paths still exist.
- [x] Render Custom Compose configuration with `.env.custom`.
- [x] Render development Compose configuration with `.env.dev`.
- [x] Confirm loopback port bindings, isolated project/container/network names, and expected data mounts.
- [x] Do not run `down -v`, volume prune, system prune, or production deployment commands.

## 7. Quality checks

- [x] Read fetched upstream build/test entry points and select commands compatible with its toolchain.
- [x] Run frontend lint and production typecheck.
- [x] Run the frontend production build; no additional Vitest suite was required because application source is byte-for-byte upstream.
- [x] Run backend tests and embedded build checks as appropriate.
- [x] Record failures with command output and distinguish upstream/pre-existing failures from preservation regressions.

## 8. Make local branch identity final

- [x] Recheck rollback references and validation evidence.
- [x] Release the old local `custom` name without deleting its backup reference.
- [x] Rename the validated temporary replacement branch to `custom`.
- [x] Do not configure it to imply that a non-fast-forward push is safe.
- [x] Show final status, exact upstream baseline, backup branch, preservation allowlist, and remaining uncommitted files.

### Rollback gate C

If final validation fails, restore local `custom` from the backup branch and reapply the saved dirty patch. Leave `origin/custom` untouched.

## 9. Explicitly excluded

- [x] Do not push any branch or tag.
- [x] Do not force-update `origin/custom`.
- [x] Do not commit secrets, runtime data, or migration backups.
- [x] Do not port retired Custom business features.
- [x] Do not modify or restart the official production environment.

## Execution evidence

- Upstream baseline: `d4b9797ff72024960a035cf22fdd8f213e149169` (`v0.1.161`).
- Original local Custom tip: `91428fbc56008fb9bd8362678ba6571365ef35f5`.
- Recovery branches: `backup/pre-replace-custom-20260719-20260719T053714Z` and `legacy/custom-pre-replace-20260719-20260719T053714Z`.
- External snapshot: `/opt/sub2api-preservation-20260719T053714Z`.
- Additional dirty tracked recovery point: `stash@{0}` with message `pre-replace-custom-20260719T053714Z tracked environment config`.
- Frontend validation: pnpm 9 frozen install, lint, typecheck, and production build passed.
- Backend validation: `go -C backend test ./...` and embedded server build passed.
- Both isolated Compose files passed `docker compose ... config --quiet`; no containers were restarted or pruned.
