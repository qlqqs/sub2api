//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Future-dated fixtures exercise prospective coverage without backdating the
// production migration. Restore their frontier effects after source cleanup so
// these tests do not alter unrelated integration fixtures' eligibility.
func financeCoverageFixture(t *testing.T) {
	t.Helper()
	var initial time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT complete_from FROM finance_aggregation_coverage_state WHERE id=1`).Scan(&initial))
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`UPDATE finance_aggregation_coverage_state SET complete_from=$1 WHERE id=1`, initial)
		require.NoError(t, err)
	})
}

func waitFinanceAggregationBlocked(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked bool
		err := integrationDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND NOT granted AND classid=$1::oid AND objid=$2::oid)`, uint32(financeAggregationLockID>>32), uint32(financeAggregationLockID&0xffffffff)).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "operation must wait on finance lock before reading source rows")
}

func TestFinanceAggregationSerializationDoesNotBlockGatewayInsert(t *testing.T) {
	financeCoverageFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f := financeRecalcData(t)
	at := time.Date(2090, 1, 3, 2, 0, 0, 0, time.UTC)
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,'aggregation-lock',5,5,5,1,$4) RETURNING id`, f.userID, f.keyID, f.accountID, at).Scan(&id))
	t.Cleanup(func() {
		for _, table := range []string{"usage_dashboard_hourly", "usage_dashboard_hourly_users", "finance_aggregation_repaired_hourly"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_start=$1`, at)
		}
		for _, table := range []string{"usage_dashboard_daily", "usage_dashboard_daily_users", "finance_aggregation_repaired_daily"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_date=$1::date`, at)
		}
	})
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	tx, err := integrationDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
	// The advisory lock does NOT participate in gateway INSERTs or take a table
	// SHARE lock. This must complete while the repair transaction is still open.
	insertCtx, insertCancel := context.WithTimeout(ctx, 2*time.Second)
	_, err = integrationDB.ExecContext(insertCtx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,'aggregation-lock',2,2,2,1,$4)`, f.userID, f.keyID, f.accountID, at)
	insertCancel()
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- repo.AggregateRange(ctx, at, at.Add(time.Hour)) }()
	waitFinanceAggregationBlocked(t)
	_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET account_stats_cost=9 WHERE id=$1 AND created_at=$2`, id, at)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	var cost string
	var count int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text,total_requests FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&cost, &count))
	require.Equal(t, "11.0000000000", cost)
	require.EqualValues(t, 2, count)
}

func TestFinanceAggregationRetentionAndRecomputeWaitForRepair(t *testing.T) {
	financeCoverageFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	for _, name := range []string{"retention", "recompute"} {
		t.Run(name, func(t *testing.T) {
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
			done := make(chan error, 1)
			go func() {
				if name == "retention" {
					_, err := cleanupUsageLogsBatchWithRollupInvalidation(ctx, integrationDB, time.Unix(0, 0))
					done <- err
				} else {
					done <- repo.RecomputeRange(ctx, time.Unix(0, 0), time.Unix(3600, 0))
				}
			}()
			waitFinanceAggregationBlocked(t)
			require.NoError(t, tx.Rollback())
			require.NoError(t, <-done)
		})
	}
}

// A historical bucket can outlive some of its sources. Recalculation must not
// rebuild it from survivors, and missing provenance must roll back source writes.
func TestFinanceAggregationUnprovenPartialBucketRollsBack(t *testing.T) {
	financeCoverageFixture(t)
	ctx := context.Background()
	f := financeRecalcData(t)
	at := time.Date(2090, 2, 3, 2, 0, 0, 0, time.UTC)
	retainedAt := at.Add(40 * time.Minute)
	var retainedID, deletedID int64
	insert := `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,'retained',$4,$4,$4,1,$5) RETURNING id`
	require.NoError(t, integrationDB.QueryRowContext(ctx, insert, f.userID, f.keyID, f.accountID, 7, retainedAt).Scan(&retainedID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, insert, f.userID, f.keyID, f.accountID, 5, at.Add(10*time.Minute)).Scan(&deletedID))
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	t.Cleanup(func() {
		for _, table := range []string{"usage_dashboard_hourly", "usage_dashboard_hourly_users", "finance_aggregation_repaired_hourly"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_start=$1`, at)
		}
		for _, table := range []string{"usage_dashboard_daily", "usage_dashboard_daily_users", "finance_aggregation_repaired_daily"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_date=$1::date`, at)
		}
	})
	// Opt this proven bucket into repair preservation before source retention.
	initial, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, AcquireFinanceAggregationLock(ctx, initial))
	initialBatch := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}}
	require.NoError(t, SynchronizeFinanceCostAggregates(ctx, initial, initialBatch, []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged, Candidate: service.FinanceRecalculationCandidate{TargetCreatedAt: retainedAt}}}))
	require.NoError(t, initial.Commit())

	// Actual deletion hook invalidates the bucket. No guessed computed_at/count
	// proof, and no manually fabricated metadata, is used in this test.
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE id=$1`, deletedID)
	require.NoError(t, err)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	require.NoError(t, repo.RecomputeRange(ctx, at, at.Add(time.Hour)))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
	_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET account_stats_cost=9 WHERE id=$1 AND created_at=$2`, retainedID, retainedAt)
	require.NoError(t, err)
	batch := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}}
	require.ErrorIs(t, SynchronizeFinanceCostAggregates(ctx, tx, batch, []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged, Candidate: service.FinanceRecalculationCandidate{TargetCreatedAt: retainedAt}}}), service.ErrFinanceRecalculationCoverageUnknown)
	require.NoError(t, tx.Rollback())
	var sourceCost, aggregateCost string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_stats_cost::text FROM usage_logs WHERE id=$1 AND created_at=$2`, retainedID, retainedAt).Scan(&sourceCost))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&aggregateCost))
	require.Equal(t, "7.0000000000", sourceCost)
	require.Equal(t, "12.0000000000", aggregateCost, "periodic/recompute must preserve retained historical contribution")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at).Scan(&aggregateCost))
	require.Equal(t, "12.0000000000", aggregateCost)
}

func TestFinanceAggregationCompleteCoverageSynchronizesNewAndExistingBuckets(t *testing.T) {
	financeCoverageFixture(t)
	ctx := context.Background()
	f := financeRecalcData(t)
	at := time.Date(2091, 5, 3, 2, 0, 0, 0, time.UTC)
	var id int64
	insert := `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) VALUES($1,$2,$3,'complete',$4,$4,$5) RETURNING id`
	require.NoError(t, integrationDB.QueryRowContext(ctx, insert, f.userID, f.keyID, f.accountID, 10, at).Scan(&id))
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	t.Cleanup(func() {
		for _, table := range []string{"usage_dashboard_hourly", "usage_dashboard_hourly_users", "finance_aggregation_repaired_hourly"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_start >= $1 AND bucket_start < $2`, at, at.Add(2*time.Hour))
		}
		for _, table := range []string{"usage_dashboard_daily", "usage_dashboard_daily_users", "finance_aggregation_repaired_daily"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_date=$1::date`, at)
		}
	})
	// New row committed AFTER the old aggregate; naive per-item delta is not a
	// membership proof. Complete-source replacement includes both exactly once.
	var lateID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, insert, f.userID, f.keyID, f.accountID, 3, at.Add(time.Minute)).Scan(&lateID))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
	_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET account_stats_cost=10,account_rate_multiplier=2 WHERE id=$1 AND created_at=$2`, id, at)
	require.NoError(t, err)
	batch := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}}
	items := []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged, Candidate: service.FinanceRecalculationCandidate{TargetID: id, TargetCreatedAt: at}}}
	require.NoError(t, SynchronizeFinanceCostAggregates(ctx, tx, batch, items))
	require.NoError(t, SynchronizeFinanceCostAggregates(ctx, tx, batch, items), "absolute replacement must be idempotent")
	require.NoError(t, tx.Commit())
	var cost, total string
	var requests int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text,total_cost::text,total_requests FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&cost, &total, &requests))
	require.Equal(t, "23.0000000000", cost)
	require.Equal(t, "10.0000000000", total, "repair only changes account_cost on existing aggregate")
	require.EqualValues(t, 1, requests)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at).Scan(&cost))
	require.Equal(t, "23.0000000000", cost)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text,total_requests FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&cost, &requests))
	require.Equal(t, "23.0000000000", cost)
	require.EqualValues(t, 2, requests)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_dashboard_hourly WHERE bucket_start=$1`, at)
	require.NoError(t, err)
	require.NoError(t, repo.AggregateRange(ctx, at.Add(time.Hour), at.Add(2*time.Hour)))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at).Scan(&cost))
	require.Equal(t, "23.0000000000", cost, "hourly expiry cannot erase repaired daily cost")

	// No daily/hourly rows: complete source evidence allows rebuilding, including
	// explicit zero rates. Daily must not depend on retained hourly rows.
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_dashboard_hourly WHERE bucket_start=$1`, at)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at)
	require.NoError(t, err)
	tx2, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx2.Rollback() }()
	require.NoError(t, AcquireFinanceAggregationLock(ctx, tx2))
	_, err = tx2.ExecContext(ctx, `UPDATE usage_logs SET account_rate_multiplier=0 WHERE id=$1 AND created_at=$2`, id, at)
	require.NoError(t, err)
	require.NoError(t, SynchronizeFinanceCostAggregates(ctx, tx2, batch, items))
	require.NoError(t, tx2.Commit())
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text,total_requests FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&cost, &requests))
	require.Equal(t, "3.0000000000", cost)
	require.EqualValues(t, 2, requests)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at).Scan(&cost))
	require.Equal(t, "3.0000000000", cost)
}

func TestFinanceAggregationLegacyBucketsKeepNormalRefresh(t *testing.T) {
	financeCoverageFixture(t)
	ctx := context.Background()
	f := financeRecalcData(t)
	at := time.Date(2024, 5, 3, 2, 0, 0, 0, time.UTC)
	eligible, err := FinanceAggregationEligibleFrom(ctx, integrationDB)
	require.NoError(t, err)
	require.True(t, at.Before(eligible), "fixture must be uncertified legacy history")
	insert := `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) VALUES($1,$2,$3,'legacy',$4,$4,$5)`
	_, err = integrationDB.ExecContext(ctx, insert, f.userID, f.keyID, f.accountID, 10, at)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	t.Cleanup(func() {
		for _, table := range []string{"usage_dashboard_hourly", "usage_dashboard_hourly_users"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_start=$1`, at)
		}
		for _, table := range []string{"usage_dashboard_daily", "usage_dashboard_daily_users"} {
			_, _ = integrationDB.Exec(`DELETE FROM `+table+` WHERE bucket_date=$1::date`, at)
		}
	})
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	_, err = integrationDB.ExecContext(ctx, insert, f.userID, f.keyID, f.accountID, 3, at.Add(time.Minute))
	require.NoError(t, err)
	require.NoError(t, repo.AggregateRange(ctx, at, at.Add(time.Hour)))
	require.NoError(t, repo.RecomputeRange(ctx, at, at.Add(time.Hour)))
	var cost string
	var requests int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text,total_requests FROM usage_dashboard_hourly WHERE bucket_start=$1`, at).Scan(&cost, &requests))
	require.Equal(t, "13.0000000000", cost)
	require.EqualValues(t, 2, requests)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT account_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, at).Scan(&cost))
	require.Equal(t, "13.0000000000", cost, "prospective repair frontier must NOT freeze ordinary dashboard updates")
}

func TestFinanceAggregationDirectDeleteTriggerLocksBeforeRows(t *testing.T) {
	financeCoverageFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	f := financeRecalcData(t)
	at := time.Date(2091, 6, 3, 2, 0, 0, 0, time.UTC)
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,created_at) VALUES($1,$2,$3,'delete-lock',10,10,$4) RETURNING id`, f.userID, f.keyID, f.accountID, at).Scan(&id))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, AcquireFinanceAggregationLock(ctx, tx))
	done := make(chan error, 1)
	go func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE id=$1`, id)
		done <- err
	}()
	waitFinanceAggregationBlocked(t)
	// If DELETE had locked the row before waiting on advisory, this UPDATE would
	// deadlock. It must complete while the DELETE is still waiting.
	_, err = tx.ExecContext(ctx, `UPDATE usage_logs SET account_rate_multiplier=2 WHERE id=$1`, id)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, <-done)
	eligible, err := FinanceAggregationEligibleFrom(ctx, integrationDB)
	require.NoError(t, err)
	require.True(t, eligible.After(at))
}

func TestFinanceAggregationTruncateRelationOrder(t *testing.T) {
	financeCoverageFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ddl, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = ddl.Rollback() }()
	_, err = ddl.ExecContext(ctx, `LOCK TABLE usage_logs IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		done <- newDashboardAggregationRepositoryWithSQL(integrationDB).AggregateRange(ctx, time.Unix(0, 0), time.Unix(3600, 0))
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='usage_logs'::regclass AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	// The worker must not hold advisory while waiting on this relation. TRUNCATE
	// can acquire advisory in its trigger, and rollback preserves other fixtures.
	_, err = ddl.ExecContext(ctx, `TRUNCATE usage_logs CASCADE`)
	require.NoError(t, err)
	require.NoError(t, ddl.Rollback())
	require.NoError(t, <-done)
}
