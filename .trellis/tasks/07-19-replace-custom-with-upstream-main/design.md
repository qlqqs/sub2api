# Design: replace Custom application code with upstream main

## Overview

Rebuild the local `custom` branch from the fetched official `upstream/main` instead of merging or reverting the 23 Custom-only commits. Before changing branch identity, create a durable backup branch for committed history and an external preservation snapshot for dirty tracked configuration and workflow files. Ignored secrets and runtime data remain in place and are never passed through Git.

The resulting worktree has three clearly separated layers:

1. official application and upstream-owned build/deployment files from `upstream/main`;
2. an explicit development/deployment preservation allowlist;
3. the complete companion workflow toolchain plus a minimally cleaned `.trellis/` tree.

## Branch and rollback model

### Original state

- `custom` points to the existing Custom history.
- `origin/custom` remains untouched.
- Dirty tracked configuration and untracked/ignored data are not represented by the branch tip.

### Safety references

- Create `backup/pre-replace-custom-20260719-<timestamp>` at the original local `custom` commit.
- Record the original commit ID, fetched `upstream/main` commit ID, dirty-path inventory, and preservation allowlist in the task evidence.
- Store a binary Git patch for dirty tracked files and exact copies of preserved workflow/configuration files in a timestamped directory outside the repository.
- Leave ignored secrets, persistent data, and `deploy/migration-backups/` in their existing filesystem locations; verify their paths before and after replacement without printing file contents.

Rollback is possible by switching away from the rebuilt branch, recreating `custom` from the backup branch, applying the saved dirty patch, and restoring copied configuration if necessary. No remote operation is required for rollback.

## Upstream baseline

Fetch `upstream/main` immediately before replacement and record its exact object ID. The fetched remote-tracking ref is the only application-code baseline. Local `main` and `origin/main` are not assumed current.

The operation must not merge Custom history into the new baseline. A temporary local branch is created directly from fetched `upstream/main`, validated, and only then renamed to `custom` after the old local branch name is released.

## Preservation boundaries

### Preserve in full

- `.cursor/`
- `.agents/`
- `skills/`
- `tools/`

These paths are workflow/tooling state and are intentionally retained even when absent from upstream.

### Preserve selectively under `.trellis/`

Keep Trellis runtime and workflow infrastructure, the current replacement task, generic development specifications, upstream synchronization guidance, isolated deployment/source-development tasks, the bootstrap history, and `.trellis/workspace/qlqqs/`.

Apply only the minimum business cleanup:

- delete `.trellis/spec/backend/upstream-balance.md`;
- delete `.trellis/tasks/archive/2026-07/07-15-upstream-channel-balance/` in full;
- remove balance-specific links, API/state contracts, and examples from the surviving backend/frontend indexes and guidelines;
- retain unrelated content in mixed specification files;
- retain personal journals and bootstrap history even when they mention the retired feature as historical context.

After cleanup, revalidate retained specifications against fetched `upstream/main`. Stale generic facts should be corrected only when contradicted by the new upstream tree; this migration must not turn into a broad rewrite of Trellis history.

### Preserve local secrets and runtime state in place

- `deploy/.env`
- `deploy/.env.custom`
- `deploy/.env.dev`
- `deploy/custom_data/`
- `deploy/custom_postgres_data/`
- `deploy/custom_redis_data/`
- `deploy/dev_postgres_data/`
- `deploy/dev_redis_data/`
- `deploy/dev_source_data/`
- `deploy/test_data/`
- `backend/data/`
- `deploy/migration-backups/`

Presence checks are allowed; secret values must not be read, logged, staged, or committed. No Docker prune command or `down -v` operation is permitted.

### Carry forward as development/deployment configuration

- `deploy/.gitignore`
- `deploy/.env.custom.example`
- `deploy/.env.dev.example`
- `deploy/CUSTOM_ENVIRONMENTS.md`
- `deploy/docker-compose.custom.yml`
- `deploy/docker-compose.dev.yml`
- `frontend/tsconfig.node.json`
- the development-host configuration in `frontend/vite.config.ts`

The latest dirty versions of the four currently modified tracked files are authoritative for the environment snapshot. The Compose and ignore files preserve isolated names, ports, data directories, credentials, and networks.

### Re-evaluate against fetched upstream before carrying forward

- Root `Dockerfile`
- frontend package-manager files such as `package.json`, `pnpm-lock.yaml`, and `pnpm-workspace.yaml`

The existing Custom versions couple the container build to pnpm 11. They must not be copied wholesale over newer upstream files. After fetching, retain upstream versions unless the isolated Custom Compose build demonstrably requires a narrow compatibility adjustment; any such adjustment must be documented in the final allowlist.

### Replace from upstream

- `backend/` tracked application source and generated source
- `frontend/src/`, application assets, and tests
- `docs/` application documentation
- upstream-owned deployment files not listed above
- root build/configuration files not explicitly admitted after compatibility review

This removes the old account-balance feature and all other Custom business behavior unless upstream independently contains equivalent code.

## Worktree transition

Because the current worktree is dirty, do not use `git reset --hard` or bulk cleanup. The transition is staged:

1. create the backup branch reference;
2. create and verify the external preservation snapshot;
3. save the dirty binary patch;
4. restore only the four snapshotted tracked files to make branch switching possible;
5. create a temporary replacement branch directly from fetched `upstream/main`;
6. restore the workflow/configuration allowlist from the snapshot and apply the approved minimal Trellis business cleanup;
7. validate the source boundary and environment configuration;
8. rename branches locally so the replacement branch becomes `custom`.

Untracked and ignored runtime paths are never deleted. If Git reports an untracked-path collision while switching, stop and resolve it by copying that path into the preservation snapshot rather than forcing the checkout.

## Validation

### Boundary validation

- Compare the rebuilt tree with the recorded fetched `upstream/main` commit.
- Require every difference to belong to the reviewed allowlist.
- Search for `CUSTOM:` markers outside preserved workflow/deployment configuration; old business-code markers must be absent.
- Confirm the retired account-balance files/routes are absent unless present in upstream itself.

### Environment validation

- Verify preserved secret and runtime paths still exist without displaying contents.
- Render `deploy/docker-compose.dev.yml` and `deploy/docker-compose.custom.yml` with their corresponding env files using `docker compose ... config`.
- Confirm project names, loopback bindings, private database/Redis services, and data mounts remain isolated.
- Do not start, stop, recreate, or prune running production services as part of configuration rendering.

### Build and test validation

- Install dependencies only if the fetched lockfiles require it.
- Run backend and frontend checks selected from the fetched upstream build instructions.
- At minimum, run frontend lint/typecheck/build and backend test/build checks when resource availability permits.
- Record any pre-existing or upstream failure rather than changing unrelated upstream application code.

## Remote update boundary

The initial operation changes only local refs and files. `origin/custom` remains as an additional recovery point. Because replacing `custom` with an ancestor/new baseline can require a non-fast-forward remote update, no push or force-push is included. A later remote update requires the user's separate approval after reviewing exact local commits and validation results.
