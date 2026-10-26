//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type financeBaselineFixture struct {
	financeRecalcFixture
	other      financeRecalcFixture
	batch      *service.FinanceRecalculationBatch
	candidates []service.FinanceRecalculationCandidate
	repo       *FinanceRecalculationConfigRepository
}

func financeBaselineData(t *testing.T) financeBaselineFixture {
	t.Helper()
	financeCoverageFixture(t)
	f := financeRecalcData(t)
	other := financeRecalcData(t)
	financeRecalcUsage(t, f)
	_, err := integrationDB.Exec(`UPDATE accounts SET rate_multiplier=2 WHERE id=$1`, f.accountID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,'other-model',3,2,3,1,$4)`, other.userID, other.keyID, other.accountID, f.at.Add(time.Hour))
	require.NoError(t, err)
	day := truncateToDay(f.at.In(timezone.Location()))
	t.Cleanup(func() {
		for _, table := range []string{"usage_dashboard_hourly", "usage_dashboard_hourly_users", "finance_aggregation_repaired_hourly"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_start >= $1 AND bucket_start < $2`, day, day.AddDate(0, 0, 1))
		}
		for _, table := range []string{"usage_dashboard_daily", "usage_dashboard_daily_users", "finance_aggregation_repaired_daily"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_date=$1::date`, day.Format("2006-01-02"))
		}
	})
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(integrationDB).AggregateRange(context.Background(), day, day.AddDate(0, 0, 1)))
	b := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost, Start: "2026-01-02", End: "2026-01-02", AccountID: &f.accountID, AcceptRetainedBaseline: true}, StartAt: day, EndAt: day.AddDate(0, 0, 1), CutoffAt: time.Now()}
	candidates, err := financeReadCandidates(context.Background(), integrationDB, b)
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	r := financeConfigRepository()
	b.Config, err = r.Snapshot(context.Background(), b, candidates)
	require.NoError(t, err)
	b.ConfigFingerprint = financeConfigFingerprint(t, b.Config)
	return financeBaselineFixture{f, other, b, candidates, r}
}

func financeBaselinePreview(t *testing.T, f financeBaselineFixture) service.FinanceRecalculationPreview {
	t.Helper()
	preview := service.FinanceRecalculationPreview{Config: f.batch.Config, ConfigFingerprint: f.batch.ConfigFingerprint}
	for _, candidate := range f.candidates {
		out, err := service.CalculateFinanceUsageCost(*candidate.Usage, f.batch.Config.Usage[0])
		require.NoError(t, err)
		require.Equal(t, service.FinanceRecalculationChanged, out.Status)
		preview.Items = append(preview.Items, service.FinanceRecalculationItem{Candidate: candidate, UsageResult: &out, Status: out.Status, Source: out.Source, Reason: out.Reason})
	}
	return preview
}

func financeBaselineQueued(t *testing.T, f financeBaselineFixture, guard FinanceRecalculationTransactionGuard) (service.FinanceRecalculationStore, *service.FinanceRecalculationBatch) {
	t.Helper()
	r := NewFinanceRecalculationRepository(integrationDB, guard)
	b, err := r.Create(context.Background(), service.FinanceRecalculationCreate{FinanceRecalculationFilter: f.batch.FinanceRecalculationFilter, ActorID: f.userID, IdempotencyKey: fmt.Sprintf("baseline-%d", time.Now().UnixNano())})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM finance_recalculation_batches WHERE id=$1`, b.ID) })
	b = financeRecalcClaim(t, r, b.ID)
	require.NoError(t, r.SavePreview(context.Background(), b.ID, b.LeaseToken, financeBaselinePreview(t, f)))
	b, err = r.QueueConfirm(context.Background(), b.ID, f.userID, f.batch.ConfigFingerprint, f.batch.ConfigFingerprint)
	require.NoError(t, err)
	return r, financeRecalcClaim(t, r, b.ID)
}

func TestFinanceRetainedBaselineHistoricalApplyAuditAndFullDay(t *testing.T) {
	f := financeBaselineData(t)
	baseline := f.batch.Config.RetainedBaseline
	require.Equal(t, 1, baseline.Version)
	require.Equal(t, timezone.Name(), baseline.Timezone)
	require.Len(t, baseline.Days, 1)
	require.EqualValues(t, 3, baseline.Days[0].SourceRows, "filtered-out account must be audited")
	require.Len(t, baseline.Days[0].Hourly, 2)
	require.Len(t, baseline.Days[0].SourceDigest, 64)
	require.Equal(t, "4.2000000002", baseline.Days[0].Daily.AccountCost, "aggregate comparison rounds SUM to stored scale")
	require.True(t, f.batch.Config.Usage[0].AcceptRetainedBaseline)
	require.True(t, f.at.Before(*f.batch.Config.Usage[0].CoverageFrom))
	before := financeRecalcProtected(t, f.financeRecalcFixture)
	otherBefore := financeRecalcProtected(t, f.other)
	var frontier time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT complete_from FROM finance_aggregation_coverage_state WHERE id=1`).Scan(&frontier))
	r, b := financeBaselineQueued(t, f, f.repo)
	require.NoError(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint))
	stored, err := r.Get(context.Background(), b.ID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationSyncing, stored.Status)
	require.Equal(t, baseline, stored.Config.RetainedBaseline, "audit keeps OLD aggregates after repair")
	var dayCost, hourCost string
	require.NoError(t, integrationDB.QueryRow(`SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date='2026-01-02'`).Scan(&dayCost))
	require.NoError(t, integrationDB.QueryRow(`SELECT account_cost::text FROM usage_dashboard_hourly WHERE bucket_start=$1`, f.at).Scan(&hourCost))
	require.Equal(t, "9.0000000002", dayCost)
	require.Equal(t, "6.0000000002", hourCost)
	var marks int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM finance_aggregation_repaired_daily WHERE bucket_date='2026-01-02'`).Scan(&marks))
	require.Equal(t, 1, marks)
	var after time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT complete_from FROM finance_aggregation_coverage_state WHERE id=1`).Scan(&after))
	require.True(t, frontier.Equal(after), "baseline never backdates or changes global coverage")
	require.Equal(t, before, financeRecalcProtected(t, f.financeRecalcFixture))
	require.Equal(t, otherBefore, financeRecalcProtected(t, f.other))
	require.NoError(t, r.Complete(context.Background(), b.ID, b.LeaseToken))
	require.ErrorIs(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint), service.ErrFinanceRecalculationState)
}

func TestFinanceRetainedBaselineUnacknowledgedAndProofCannotBypass(t *testing.T) {
	f := financeBaselineData(t)
	f.batch.AcceptRetainedBaseline = false
	plain, err := f.repo.Snapshot(context.Background(), f.batch, f.candidates)
	require.NoError(t, err)
	require.Nil(t, plain.RetainedBaseline)
	require.False(t, plain.Usage[0].AcceptRetainedBaseline)
	out, err := service.CalculateFinanceUsageCost(*f.candidates[0].Usage, plain.Usage[0])
	require.NoError(t, err)
	require.Equal(t, "unproven_aggregate_coverage", out.Reason)
	items := []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged, Candidate: f.candidates[0]}}
	for _, optin := range []bool{false, true} {
		f.batch.AcceptRetainedBaseline = optin
		tx, err := integrationDB.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		require.NoError(t, AcquireFinanceAggregationLock(context.Background(), tx))
		require.ErrorIs(t, SynchronizeFinanceCostAggregates(context.Background(), tx, f.batch, items), service.ErrFinanceRecalculationCoverageUnknown, "baseline data without guard proof must fail")
		require.NoError(t, tx.Rollback())
	}
	// A successful guard proof is tied to the exact snapshot, not only a day.
	tx, err := integrationDB.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, release, err := f.repo.Acquire(context.Background(), tx, f.batch)
	if release != nil {
		defer release()
	}
	require.NoError(t, err)
	f.batch.Config.RetainedBaseline.Days[0].SourceDigest = "forged"
	require.ErrorIs(t, SynchronizeFinanceCostAggregates(context.Background(), tx, f.batch, items), service.ErrFinanceRecalculationCoverageUnknown)
	require.NoError(t, tx.Rollback())
}

func TestFinanceRetainedBaselineRejectsReconciliationMismatch(t *testing.T) {
	for _, test := range []struct{ name, query string }{
		{"hour count", `UPDATE usage_dashboard_hourly SET total_requests=total_requests+1 WHERE bucket_start=$1`},
		{"hour cost", `UPDATE usage_dashboard_hourly SET account_cost=account_cost+1 WHERE bucket_start=$1`},
		{"daily count", `UPDATE usage_dashboard_daily SET total_requests=total_requests+1 WHERE bucket_date=$1::date`},
		{"daily customer cost", `UPDATE usage_dashboard_daily SET actual_cost=actual_cost+1 WHERE bucket_date=$1::date`},
		{"daily missing", `DELETE FROM usage_dashboard_daily WHERE bucket_date=$1::date`},
		{"hour missing", `DELETE FROM usage_dashboard_hourly WHERE bucket_start=$1`},
		{"aggregate only", `UPDATE usage_dashboard_hourly SET bucket_start=bucket_start+INTERVAL '10 hours' WHERE bucket_start=$1`},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := financeBaselineData(t)
			_, err := integrationDB.Exec(test.query, f.at)
			require.NoError(t, err)
			_, err = f.repo.Snapshot(context.Background(), f.batch, f.candidates)
			require.ErrorIs(t, err, service.ErrFinanceRecalculationCoverageUnknown)
			_, err = f.repo.Current(context.Background(), f.batch)
			require.ErrorIs(t, err, service.ErrFinanceRecalculationCoverageUnknown)
			tx, txErr := integrationDB.BeginTx(context.Background(), nil)
			require.NoError(t, txErr)
			_, release, guardErr := f.repo.Acquire(context.Background(), tx, f.batch)
			require.NoError(t, tx.Rollback())
			if release != nil {
				release()
			}
			require.ErrorIs(t, guardErr, service.ErrFinanceRecalculationCoverageUnknown, "apply guard must recheck after confirm")
		})
	}
}

func TestFinanceRetainedBaselineSourceAndConfigurationDrift(t *testing.T) {
	for _, test := range []struct{ name, query string }{
		{"filtered-out source", `UPDATE usage_logs SET model='changed-outside-filter' WHERE account_id=$1`},
		{"NULL versus zero source", `UPDATE usage_logs SET image_input_tokens=1 WHERE account_id=$1`},
		{"source added", `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) SELECT user_id,api_key_id,account_id,'added',0,0,created_at FROM usage_logs WHERE account_id=$1 LIMIT 1`},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := financeBaselineData(t)
			r, b := financeBaselineQueued(t, f, f.repo)
			_, err := integrationDB.Exec(test.query, f.other.accountID)
			require.NoError(t, err)
			day := truncateToDay(f.at.In(timezone.Location()))
			require.NoError(t, newDashboardAggregationRepositoryWithSQL(integrationDB).AggregateRange(context.Background(), day, day.AddDate(0, 0, 1)))
			fresh, err := f.repo.Current(context.Background(), f.batch)
			require.NoError(t, err)
			require.NotEqual(t, f.batch.ConfigFingerprint, financeConfigFingerprint(t, fresh))
			require.ErrorIs(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint), service.ErrFinanceRecalculationRuleChanged)
		})
	}
	t.Run("configuration", func(t *testing.T) {
		f := financeBaselineData(t)
		r, b := financeBaselineQueued(t, f, f.repo)
		_, err := integrationDB.Exec(`UPDATE accounts SET rate_multiplier=3 WHERE id=$1`, f.accountID)
		require.NoError(t, err)
		require.ErrorIs(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint), service.ErrFinanceRecalculationRuleChanged)
	})
}

func TestFinanceRetainedBaselineHoldsWholeSourceRelation(t *testing.T) {
	f := financeBaselineData(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, release, err := f.repo.Acquire(ctx, tx, f.batch)
	if release != nil {
		defer release()
	}
	require.NoError(t, err)
	for _, operation := range []string{
		`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) SELECT user_id,api_key_id,account_id,'blocked',0,0,created_at FROM usage_logs WHERE account_id=$1 LIMIT 1`,
		`UPDATE usage_logs SET model='blocked' WHERE account_id=$1`,
		`DELETE FROM usage_logs WHERE account_id=$1`,
	} {
		writeCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
		_, err = integrationDB.ExecContext(writeCtx, operation, f.other.accountID)
		stop()
		require.Error(t, err, "even writes to filtered-out source rows must block")
		require.ErrorIs(t, writeCtx.Err(), context.DeadlineExceeded)
	}
	require.NoError(t, tx.Rollback())
	_, err = integrationDB.ExecContext(ctx, `UPDATE usage_logs SET model=model WHERE account_id=$1`, f.other.accountID)
	require.NoError(t, err, "release restores source writes")
}

type financeBaselineFailSync struct {
	*FinanceRecalculationConfigRepository
}

func (g financeBaselineFailSync) Synchronize(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch, items []service.FinanceRecalculationItem) error {
	if err := g.FinanceRecalculationConfigRepository.Synchronize(ctx, tx, b, items); err != nil {
		return err
	}
	return service.ErrFinanceRecalculationSyncRequired
}
func TestFinanceRetainedBaselineAuditAndWritesRollbackTogether(t *testing.T) {
	f := financeBaselineData(t)
	r, b := financeBaselineQueued(t, f, financeBaselineFailSync{f.repo})
	var before string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_agg(to_jsonb(u) ORDER BY id)::text FROM usage_logs u WHERE account_id=$1`, f.accountID).Scan(&before))
	require.ErrorIs(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint), service.ErrFinanceRecalculationSyncRequired)
	var after, cost string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_agg(to_jsonb(u) ORDER BY id)::text FROM usage_logs u WHERE account_id=$1`, f.accountID).Scan(&after))
	require.Equal(t, before, after)
	require.NoError(t, integrationDB.QueryRow(`SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date='2026-01-02'`).Scan(&cost))
	require.Equal(t, "4.2000000002", cost)
	var applied, marked int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM finance_recalculation_items WHERE batch_id=$1 AND applied`, b.ID).Scan(&applied))
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM finance_aggregation_repaired_daily WHERE bucket_date='2026-01-02'`).Scan(&marked))
	require.Zero(t, applied)
	require.Zero(t, marked)
	stored, err := r.Get(context.Background(), b.ID)
	require.NoError(t, err)
	require.Equal(t, f.batch.Config.RetainedBaseline, stored.Config.RetainedBaseline)
	// Same durable batch can recover after transaction rollback; no second audit
	// or second cost application is fabricated.
	r = NewFinanceRecalculationRepository(integrationDB, f.repo)
	require.NoError(t, r.Apply(context.Background(), b.ID, b.LeaseToken, b.ConfigFingerprint))
}

func TestFinanceRetainedBaselineBoundsAndEmpty(t *testing.T) {
	f := financeBaselineData(t)
	empty, err := f.repo.Snapshot(context.Background(), f.batch, nil)
	require.NoError(t, err)
	require.NotNil(t, empty.RetainedBaseline)
	require.Empty(t, empty.RetainedBaseline.Days)
	copyBatch := *f.batch
	copyBatch.Config = empty
	fresh, err := f.repo.Current(context.Background(), &copyBatch)
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, empty), financeConfigFingerprint(t, fresh))
	var many []service.FinanceRecalculationCandidate
	for i := 0; i < 32; i++ {
		many = append(many, service.FinanceRecalculationCandidate{Usage: &service.FinanceUsageCostInput{AccountID: f.accountID}, TargetCreatedAt: f.at.AddDate(0, 0, i)})
	}
	_, err = f.repo.Snapshot(context.Background(), f.batch, many)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationTooLarge)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) SELECT $1,$2,$3,'outside-filter',0,0,$4 FROM generate_series(1,2000)`, f.other.userID, f.other.keyID, f.other.accountID, f.at)
	require.NoError(t, err)
	_, err = f.repo.Snapshot(context.Background(), f.batch, f.candidates)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationTooLarge, "limit includes unrelated account's rows, not only filtered candidates")
}

func TestFinanceRetainedBaselineWhitelistDeterminismAndNullEvidence(t *testing.T) {
	f := financeBaselineData(t)
	f.candidates[0], f.candidates[1] = f.candidates[1], f.candidates[0]
	fresh, err := f.repo.Snapshot(context.Background(), f.batch, f.candidates)
	require.NoError(t, err)
	require.Equal(t, f.batch.Config.RetainedBaseline, fresh.RetainedBaseline)
	oldDigest := fresh.RetainedBaseline.Days[0].SourceDigest
	_, err = integrationDB.Exec(`UPDATE usage_logs SET account_stats_cost=total_cost WHERE account_id=$1 AND account_stats_cost IS NULL`, f.accountID)
	require.NoError(t, err)
	fresh, err = f.repo.Current(context.Background(), f.batch)
	require.NoError(t, err)
	require.NotEqual(t, oldDigest, fresh.RetainedBaseline.Days[0].SourceDigest, "same cost but NULL fallback changed must invalidate digest")
	raw, err := json.Marshal(fresh.RetainedBaseline)
	require.NoError(t, err)
	for _, unsafe := range []string{"api_key", "email", "credentials", "user_id", "request_id"} {
		require.NotContains(t, string(raw), unsafe)
	}
}

func TestFinanceRetainedBaselineDailyRoundingMatchesKnownWriters(t *testing.T) {
	f := financeBaselineData(t)
	_, err := integrationDB.Exec(`UPDATE usage_logs SET account_stats_cost=0.0000000001,account_rate_multiplier=0.5 WHERE account_id=$1`, f.other.accountID)
	require.NoError(t, err)
	day := truncateToDay(f.at.In(timezone.Location()))
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(integrationDB).AggregateRange(context.Background(), day, day.AddDate(0, 0, 1)))
	fresh, err := f.repo.Current(context.Background(), f.batch)
	require.NoError(t, err)
	require.Equal(t, "1.2000000003", fresh.RetainedBaseline.Days[0].Daily.AccountCost, "normal writer sums rounded hourly amounts")
	_, err = integrationDB.Exec(`UPDATE usage_dashboard_daily SET account_cost=(SELECT SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)) FROM usage_logs WHERE created_at >= $1 AND created_at < $2) WHERE bucket_date=$3::date`, day, day.AddDate(0, 0, 1), day.Format("2006-01-02"))
	require.NoError(t, err)
	fresh, err = f.repo.Current(context.Background(), f.batch)
	require.NoError(t, err)
	require.Equal(t, "1.2000000002", fresh.RetainedBaseline.Days[0].Daily.AccountCost, "repair writer rounds exact daily source sum")
	_, err = integrationDB.Exec(`UPDATE usage_dashboard_daily SET account_cost=account_cost+0.0000000002 WHERE bucket_date=$1::date`, day.Format("2006-01-02"))
	require.NoError(t, err)
	_, err = f.repo.Current(context.Background(), f.batch)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationCoverageUnknown, "no epsilon tolerance: neither known exact result is rejected")
}

func financeBaselineWaitExclusive(t *testing.T, pid int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='usage_logs'::regclass AND mode='ExclusiveLock' AND NOT granted)`, pid).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "baseline must wait on relation before advisory")
	var advisory bool
	require.NoError(t, integrationDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted)`, pid).Scan(&advisory))
	require.False(t, advisory)
}

func TestFinanceRetainedBaselineCleanupAndCompetingBaselinesLockOrder(t *testing.T) {
	for _, cleanup := range []bool{true, false} {
		t.Run(fmt.Sprintf("cleanup_%t", cleanup), func(t *testing.T) {
			f := financeBaselineData(t)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			var release func()
			if cleanup {
				require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
			} else {
				_, release, err = f.repo.Acquire(ctx, tx, f.batch)
				require.NoError(t, err)
			}
			if release != nil {
				defer release()
			}
			pidReady := make(chan int, 1)
			done := make(chan error, 1)
			go func() {
				next, e := integrationDB.BeginTx(ctx, nil)
				if e != nil {
					done <- e
					return
				}
				var pid int
				e = next.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&pid)
				if e != nil {
					_ = next.Rollback()
					done <- e
					return
				}
				pidReady <- pid
				_, unlock, e := f.repo.Acquire(ctx, next, f.batch)
				_ = next.Rollback()
				if unlock != nil {
					unlock()
				}
				done <- e
			}()
			var pid int
			select {
			case pid = <-pidReady:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			financeBaselineWaitExclusive(t, pid)
			if cleanup {
				_, err = tx.ExecContext(ctx, `DELETE FROM usage_logs WHERE account_id=$1`, f.other.accountID)
			} else {
				_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET account_stats_cost=account_stats_cost WHERE account_id=$1`, f.accountID)
			}
			require.NoError(t, err, "first transaction must upgrade its write lock without cycle")
			require.NoError(t, tx.Commit())
			if cleanup {
				require.ErrorIs(t, <-done, service.ErrFinanceRecalculationCoverageUnknown)
			} else {
				require.NoError(t, <-done)
			}
		})
	}
}

func TestFinanceRetainedBaselineLaterInsertsAggregateThenDeletionPreserves(t *testing.T) {
	f := financeBaselineData(t)
	r, b := financeBaselineQueued(t, f, f.repo)
	ctx := context.Background()
	require.NoError(t, r.Apply(ctx, b.ID, b.LeaseToken, b.ConfigFingerprint))
	var added int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,'later-insert',5,5,5,1,$4) RETURNING id`, f.userID, f.keyID, f.accountID, f.at.Add(time.Minute)).Scan(&added))
	day := truncateToDay(f.at.In(timezone.Location()))
	aggregation := newDashboardAggregationRepositoryWithSQL(integrationDB)
	check := func() {
		t.Helper()
		var count int64
		var cost string
		require.NoError(t, integrationDB.QueryRow(`SELECT total_requests,account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, day.Format("2006-01-02")).Scan(&count, &cost))
		require.EqualValues(t, 4, count)
		require.Equal(t, "14.0000000002", cost)
		require.NoError(t, integrationDB.QueryRow(`SELECT total_requests,account_cost::text FROM usage_dashboard_hourly WHERE bucket_start=$1`, f.at).Scan(&count, &cost))
		require.EqualValues(t, 3, count)
		require.Equal(t, "11.0000000002", cost)
	}
	for _, operation := range []func(context.Context, time.Time, time.Time) error{aggregation.AggregateRange, aggregation.RecomputeRange} {
		require.NoError(t, operation(ctx, day, day.AddDate(0, 0, 1)))
		check()
	}
	_, err := integrationDB.Exec(`DELETE FROM usage_logs WHERE id=$1 AND created_at=$2`, added, f.at.Add(time.Minute))
	require.NoError(t, err)
	for _, operation := range []func(context.Context, time.Time, time.Time) error{aggregation.AggregateRange, aggregation.RecomputeRange} {
		require.NoError(t, operation(ctx, day, day.AddDate(0, 0, 1)))
		check()
	}
}
