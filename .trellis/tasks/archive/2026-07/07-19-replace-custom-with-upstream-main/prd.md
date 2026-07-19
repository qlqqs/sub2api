# Replace custom code with latest upstream main

## Goal

Replace the application code on `custom` with the latest official `upstream/main` baseline while preserving a reversible copy of the current Custom state, reusable Trellis workflow knowledge, and the local development/deployment environment needed to continue operating the isolated Custom stack.

## Background

- `custom` currently points to `91428fbc5` and is 23 commits ahead of the locally cached `upstream/main` at `bc2244c83`; the available main commit is already an ancestor of `custom`.
- The requested outcome is therefore a deliberate retirement of the existing Custom application changes, not a normal merge-conflict resolution.
- `custom` has four modified tracked files: `deploy/CUSTOM_ENVIRONMENTS.md`, `deploy/docker-compose.custom.yml`, `frontend/tsconfig.node.json`, and `frontend/vite.config.ts`.
- `deploy/migration-backups/` contains untracked or ignored rollback data that is not protected by creating a Git branch alone.
- Local environment secrets and persistent data are stored in ignored paths under `deploy/`, including `.env*`, Custom PostgreSQL/Redis/application data, and development data.
- The existing Custom branch contains deployment isolation assets and development workflow tooling that do not exist on the cached upstream main.

## Requirements

- Create a uniquely named backup branch or equivalent immutable Git reference for the current committed `custom` tip before changing `custom`.
- Preserve the current dirty tracked configuration changes independently because a branch does not capture them.
- Preserve Trellis workflow/runtime infrastructure, this replacement task, reusable upstream-sync guidance, isolated development/deployment documentation, and generic specifications that remain valid against the fetched upstream baseline.
- Remove Trellis specifications, archived tasks, manifests, links, and embedded contracts dedicated to retired Custom business features, including the upstream-account-balance feature.
- Revalidate retained backend/frontend specifications against the fetched upstream baseline and remove or update stale Custom-only references before treating them as authoritative.
- Preserve the companion development workflow directories `.cursor/`, `.agents/`, `skills/`, and `tools/` in full; they are development tooling rather than retired Custom business code.
- Preserve local secret files, persistent runtime data, and migration backups without reading or exposing secret values.
- Base the replacement `custom` application code on the latest official `upstream/main` verified at execution time, not merely the currently cached remote-tracking ref.
- Remove the existing Custom business-code modifications, including the upstream account-balance feature and related frontend/backend changes.
- Carry forward only confirmed development/deployment configuration deltas; upstream-owned build and deployment files must otherwise use the new upstream versions.
- Keep the official production environment isolated from Custom and development environments; do not prune Docker volumes or reuse production data, credentials, containers, networks, or ports.
- Do not force-push or otherwise rewrite a remote branch unless the user separately approves the exact remote update after local validation.

## Acceptance Criteria

- [x] A backup reference identifies the original committed `custom` tip, and the dirty configuration plus `deploy/migration-backups/` have a documented recoverable copy.
- [x] The resulting local `custom` application source matches the fetched `upstream/main` except for an explicitly reviewed preservation allowlist.
- [x] `.cursor/`, `.agents/`, `skills/`, and `tools/` remain complete, while `.trellis/` retains workflow/runtime infrastructure, the active replacement task, reusable environment/sync knowledge, and upstream-valid generic specifications.
- [x] The retired upstream-account-balance specification, archived task, manifests, links, and embedded feature contracts are absent from the resulting `.trellis/` tree.
- [x] `deploy/.env`, `deploy/.env.custom`, and `deploy/.env.dev`, when present, remain intact and are not printed or committed.
- [x] Custom persistent data and migration backups remain intact; no destructive Docker volume operation is performed.
- [x] The isolated Custom deployment and source-development configuration required by the current environment remains usable.
- [x] Existing Custom business features are absent unless they are independently present in the new upstream baseline.
- [x] Git status and the final diff clearly separate upstream code from the preserved environment/workflow files.
- [x] Applicable dependency, type-check, build, and test commands pass, or every upstream/pre-existing failure is recorded with evidence.
- [x] No remote `custom` ref is rewritten without a separate explicit user approval.

## Out of Scope

- Porting the old upstream account-balance feature or other retired Custom business behavior onto the new baseline.
- Deleting local databases, Redis state, application data, migration backups, or secrets after migration.
- Updating production services or pushing the replacement branch during the initial local migration.
