# Aggregation synchronization status (2026-10-26)

## Current contract — explicitly accepted baseline, stopped traffic

User approved retained-detail baseline and explicitly requires stopping requests and draining in-flight work before execution. No online-repair feature is required. Accepted batches reconcile full-day sources/old aggregates and retain audit; default unaccepted legacy rows still skip. This is an accepted accounting baseline, not proof of no past deletion. Migration260 leaves the prospective frontier unchanged and lets accepted markers participate in normal post-resume refresh until source deletion invalidates their generation. Subsequent invalidation is conservative; no new online algorithm or all-database bootstrap is introduced. See baseline-verification.md for current tests when available.

## Previous limitation (superseded by the explicit baseline option)

The implementation now repairs **proven-complete prospective buckets**, rather than rejecting every change. However, no trustworthy pre-migration source-membership/deletion history exists. Migration 258 deliberately does not certify old history. This is a material product limitation requiring explicit approval; do not label the overall historical-recalculation requirement complete.

Bootstrap is conservative: `complete_from` is after migration time and existing source/hourly/date-only daily bounds (the daily timezone-independent upper bound is date +36h). Therefore the initial eligible day may be 1–2 calendar days later, or later if future-dated legacy data exists. This restricts **repair eligibility only**, not normal dashboard refresh.

Main-session local E2E observation: after October 26, 2026 deployment the frontier is October 27, 2026 20:00 and the eligible complete-day boundary is October 28, 2026 (configured local timezone). Consequently pre-existing historical usage is initially unavailable; payment/config E2E success does not change this limitation.

`usage_group_rollup_state.retained_from`, current retention settings, cleanup-task records, computed_at, row counts and MAX(id) are not reliable historical bootstrap evidence. Missing aggregates are not evidence of complete sources either. Never manually backdate the production frontier to enable a repair.

## APIs

```go
AcquireFinanceAggregationLock(context.Context, *sql.Tx) error
SynchronizeFinanceCostAggregates(context.Context, *sql.Tx,
    *service.FinanceRecalculationBatch, []service.FinanceRecalculationItem) error
FinanceAggregationEligibleFrom(context.Context, sqlQueryer) (time.Time, error)
```

The last helper accepts `*sql.DB` or `*sql.Tx`. It returns the earliest fully eligible day in the configured dashboard timezone. Preview marks earlier rows skipped (`unproven_aggregate_coverage`); main loader/calculator owns wiring. Execution rechecks the frontier because cleanup may advance it after preview, returning `service.ErrFinanceRecalculationCoverageUnknown` and rolling back source/audit/aggregate changes.

## Atomic synchronization and narrow preservation

- Shared transaction advisory key: `0x73326166696e6167` (`8300804155680645479`). Use READ COMMITTED.
- Relation locks precede advisory locks: common helper acquires `usage_logs ROW SHARE`, compatible with gateway INSERT/UPDATE/DELETE ROW EXCLUSIVE. Then advisory, group-rollup state, source rows. Main config guard takes config locks before these.
- Periodic AggregateRange/RecomputeRange, retention batches, manual cleanup and partition drops participate. Group-rollup synchronization acquires source ROW SHARE before group state to avoid DDL inversion.
- Partition DROP takes its unavoidable DDL relation locks before advisory, then advances the frontier to exclusive month end in the same transaction. DELETE hooks acquire advisory **before source row locks** and advance the frontier from deleted rows. TRUNCATE hook also advances it.
- Migration installs deletion hooks before initializing coverage, so trigger-DDL source locks close the initialization/install gap. Upgrading must not leave old aggregation/cleanup workers running outside the new lock protocol.
- Only changed usage items are considered. Payment and unchanged/skipped-only batches do nothing to dashboard aggregates.
- For eligible complete buckets, existing rows receive an absolute recomputation of **account_cost only** from current raw sources. Unrelated existing aggregate metrics are preserved. Missing buckets are safely created from complete sources. Repeated synchronization does not double-apply deltas; late/new inserts are summed at most once.
- Small hourly/day repair-marker tables are written in the same transaction. They are per-bucket metadata, not a per-usage ledger.
- **Unmarked legacy buckets keep existing periodic/recompute behavior. No global frontier clamp.** A marked bucket whose sources later become incomplete is preserved against periodic replacement/deletion. This intentionally does not fix unrelated pre-existing legacy retention behavior.
- For marked still-complete days, periodic account_cost is recomputed from raw sources, not potentially expired hourly aggregates. Existing unrelated legacy daily logic is retained.
- Direct source deletion is covered by the root-table trigger, including the existing `usage_log_repo_query.go` path without editing that file. Unsupported direct child-partition DML / out-of-band DROP or historical timestamp mutation must not bypass the protocol. Aggregate cleanup retains its configured expiry semantics.

## Cache fence

Optional `service.DashboardStatsCacheFence`:

```go
DashboardStatsGeneration(context.Context) (string, error)
SetDashboardStatsIfGeneration(context.Context, string, string, time.Duration) error
InvalidateDashboardStatsGeneration(context.Context) error
```

Redis random generation avoids ABA after key loss; Lua atomically compares/writes or rotates/deletes. Normal cache deletion also rotates. Synchronous/background refresh captures generation before source reads. Generation-read failure skips cache write rather than using unconditional SET.

After Apply commits, orchestration invalidates and only completes after success. Failure retains syncing/retry state; repeated invalidation is safe. This fences the dashboard stats cache; frontend cache refresh remains the orchestration/UI owner's work.

## Checks

Passed checks so far:
- Full repository unit suite (`go test -tags unit ./internal/repository -count=1`).
- Full service unit suite (215s), plus dashboard/cache focused tests.
- Final focused cache/service fence + aggregation-lock tests under `-race` passed (service 2.084s; repository 1.187s), followed by full repository unit suite passing in 10.268s.
- PostgreSQL tests for successful certified repair; late unaggregated rows; repeated synchronization; missing buckets; NULL fallback and zero rate; preserving marked partial buckets; expired hourly data not undoing daily repair; gateway INSERT while repair holds lock; aggregation/retention/recompute serialization.
- Latest full `^TestFinanceAggregation` PostgreSQL run passed in 5.8s, including uncertified legacy refresh continuing normally, direct DELETE ordering, and TRUNCATE relation ordering. Partition-drop frontier/rollback paths are covered by SQL-mock tests; no claim of a new real partition-conversion test.
- `git diff --check`.

No commit, dependency installation, deployment or production modification. No local full e2e claim.

## Migration handoff caution

Migration 258 was refined during development. If local E2E applied an earlier draft, check schema/checksum before rebuilding. Never silently rewrite an applied checksum. Any migration accepted outside disposable local test data requires a new forward migration for later changes.

Local migration verification: applied258 checksum equals SHA256 of trimmed current source (`136c67dd10cf13c9f8303f4b685b909e7f04a706ebdc81ffd63d1cddd019f650`); repaired-day table exists. No checksum rewriting or data reset was needed.
