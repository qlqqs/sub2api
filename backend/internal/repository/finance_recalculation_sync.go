package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A transaction-scoped lock shared by repair, aggregation and retention. Gateway
// INSERTs do not acquire it. Lock order: source relation, advisory lock,
// group-rollup state, source rows (config relations precede these in the guard).
const financeAggregationLockID int64 = 0x73326166696e6167

// AcquireFinanceAggregationLock must run in a READ COMMITTED transaction BEFORE
// source row locks or aggregate reads. Each subsequent SQL statement then sees
// the previous lock owner's committed writes; a pre-lock repeatable-read
// snapshot is not sufficient to prevent a stale aggregate overwrite.
func AcquireFinanceAggregationLock(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return service.ErrFinanceRecalculationSyncRequired
	}
	// ROW SHARE is compatible with gateway ROW EXCLUSIVE writes. Taking the
	// relation lock before the advisory lock avoids a TRUNCATE/DROP cycle.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE usage_logs IN ROW SHARE MODE`); err != nil {
		return fmt.Errorf("lock finance aggregation source relation: %w", err)
	}
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, financeAggregationLockID)
	if err != nil {
		return fmt.Errorf("acquire finance aggregation lock: %w", err)
	}
	return nil
}

// financeAggregationSafeStarts computes complete bucket boundaries, not a
// timestamp/ID membership guess. The frontier is bootstrapped prospectively and
// advanced transactionally whenever a source is deleted.
func financeAggregationSafeStarts(ctx context.Context, q sqlQueryer) (time.Time, time.Time, error) {
	var frontier time.Time
	if err := scanSingleRow(ctx, q, `SELECT complete_from FROM finance_aggregation_coverage_state WHERE id = 1`, nil, &frontier); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("read finance aggregation coverage: %w", err)
	}
	local := frontier.In(timezone.Location())
	hour := local.Truncate(time.Hour)
	if hour.Before(local) {
		hour = hour.Add(time.Hour)
	}
	day := truncateToDay(local)
	if day.Before(local) {
		day = day.AddDate(0, 0, 1)
	}
	return hour, day, nil
}

// SynchronizeFinanceCostAggregates recomputes account_cost only where complete
// source coverage is proven. It never applies a delta to an aggregate of unknown
// membership. Newly committed INSERTs are safe: the replacement sum includes
// them at most once, and later periodic replacement uses corrected source costs.
// Payment and unchanged/skipped items have no aggregate effects. Source/audit/
// aggregate changes share the caller's READ COMMITTED transaction and lock.
func SynchronizeFinanceCostAggregates(ctx context.Context, tx *sql.Tx, batch *service.FinanceRecalculationBatch, items []service.FinanceRecalculationItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tx == nil || batch == nil {
		return service.ErrFinanceRecalculationSyncRequired
	}
	if batch.Kind == service.FinanceRecalculationPaymentFee {
		return nil
	}
	if batch.Kind != service.FinanceRecalculationUsageCost || len(items) > service.FinanceRecalculationMaxRows {
		return service.ErrFinanceRecalculationInvalid
	}
	hours := map[time.Time]struct{}{}
	days := map[time.Time]struct{}{}
	for _, item := range items {
		if item.Status != service.FinanceRecalculationChanged {
			continue
		}
		at := item.Candidate.TargetCreatedAt.In(timezone.Location())
		if at.IsZero() {
			return service.ErrFinanceRecalculationInvalid
		}
		hours[at.Truncate(time.Hour)] = struct{}{}
		days[truncateToDay(at)] = struct{}{}
	}
	if len(hours) == 0 {
		return nil
	}
	safeHour, safeDay, err := financeAggregationSafeStarts(ctx, tx)
	if err != nil {
		return err
	}
	baselineDays, err := financeAuthorizedBaselineDays(ctx, tx, batch)
	if err != nil {
		return err
	}
	sortedHours := make([]time.Time, 0, len(hours))
	sortedDays := make([]time.Time, 0, len(days))
	for at := range hours {
		if at.Before(safeHour) && !baselineDays[at.Format("2006-01-02")] {
			return service.ErrFinanceRecalculationCoverageUnknown
		}
		sortedHours = append(sortedHours, at)
	}
	for at := range days {
		if at.Before(safeDay) && !baselineDays[at.Format("2006-01-02")] {
			return service.ErrFinanceRecalculationCoverageUnknown
		}
		sortedDays = append(sortedDays, at)
	}
	sort.Slice(sortedHours, func(i, j int) bool { return sortedHours[i].Before(sortedHours[j]) })
	sort.Slice(sortedDays, func(i, j int) bool { return sortedDays[i].Before(sortedDays[j]) })
	r := newDashboardAggregationRepositoryWithSQL(tx)
	for _, at := range sortedHours {
		marker := `INSERT INTO finance_aggregation_repaired_hourly(bucket_start) VALUES($1) ON CONFLICT DO NOTHING`
		if baselineDays[at.Format("2006-01-02")] {
			marker = `INSERT INTO finance_aggregation_repaired_hourly(bucket_start,baseline_generation)
			 SELECT $1,deletion_generation FROM finance_aggregation_coverage_state WHERE id=1
			 ON CONFLICT (bucket_start) DO UPDATE SET baseline_generation=EXCLUDED.baseline_generation`
		}
		if _, err := tx.ExecContext(ctx, marker, at); err != nil {
			return err
		}
		end := at.Add(time.Hour)
		res, err := tx.ExecContext(ctx, `UPDATE usage_dashboard_hourly SET account_cost=(SELECT COALESCE(SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)),0) FROM usage_logs WHERE created_at >= $1 AND created_at < $2) WHERE bucket_start=$1`, at, end)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			if err = r.insertHourlyActiveUsers(ctx, at, end); err != nil {
				return err
			}
			if err = r.upsertHourlyAggregates(ctx, at, end); err != nil {
				return err
			}
		}
	}
	for _, at := range sortedDays {
		marker := `INSERT INTO finance_aggregation_repaired_daily(bucket_date) VALUES($1::date) ON CONFLICT DO NOTHING`
		if baselineDays[at.Format("2006-01-02")] {
			marker = `INSERT INTO finance_aggregation_repaired_daily(bucket_date,baseline_generation)
			 SELECT $1::date,deletion_generation FROM finance_aggregation_coverage_state WHERE id=1
			 ON CONFLICT (bucket_date) DO UPDATE SET baseline_generation=EXCLUDED.baseline_generation`
		}
		if _, err := tx.ExecContext(ctx, marker, at.Format("2006-01-02")); err != nil {
			return err
		}
		end := at.AddDate(0, 0, 1)
		res, err := tx.ExecContext(ctx, `UPDATE usage_dashboard_daily SET account_cost=(SELECT COALESCE(SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)),0) FROM usage_logs WHERE created_at >= $1 AND created_at < $2) WHERE bucket_date=$3::date`, at, end, at.Format("2006-01-02"))
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			if err = r.upsertFinanceDailyFromSources(ctx, at, end); err != nil {
				return err
			}
		}
	}
	return nil
}

// FinanceAggregationEligibleFrom exposes the earliest proven complete dashboard
// day for preview eligibility. Earlier usage rows must be marked skipped with
// aggregate_coverage_unknown, not advertised as executable. This is only a
// preview hint: retention can advance it, so Synchronize checks again under the
// transaction lock. No pre-migration historical bucket is certified by this API.
func FinanceAggregationEligibleFrom(ctx context.Context, q sqlQueryer) (time.Time, error) {
	if q == nil {
		return time.Time{}, service.ErrFinanceRecalculationSyncRequired
	}
	_, day, err := financeAggregationSafeStarts(ctx, q)
	return day, err
}

func (r *dashboardAggregationRepository) upsertFinanceDailyFromSources(ctx context.Context, start, end time.Time) error {
	tzName := timezone.Name()
	query := `
		WITH daily AS (
			SELECT
				(created_at AT TIME ZONE $5)::date AS bucket_date,
				COUNT(*) AS total_requests,
				COALESCE(SUM(input_tokens), 0) AS input_tokens,
				COALESCE(SUM(output_tokens), 0) AS output_tokens,
				COALESCE(SUM(cache_creation_tokens), 0) AS cache_creation_tokens,
				COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
				COALESCE(SUM(total_cost), 0) AS total_cost,
				COALESCE(SUM(actual_cost), 0) AS actual_cost,
				COALESCE(SUM(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)), 0) AS account_cost,
				COALESCE(SUM(COALESCE(duration_ms,0)), 0) AS total_duration_ms
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY (created_at AT TIME ZONE $5)::date
		),
		user_counts AS (
			SELECT (created_at AT TIME ZONE $5)::date AS bucket_date, COUNT(DISTINCT user_id) AS active_users
			FROM usage_logs
			WHERE created_at >= $3 AND created_at < $4
			GROUP BY (created_at AT TIME ZONE $5)::date
		)
		INSERT INTO usage_dashboard_daily (
			bucket_date,
			total_requests,
			input_tokens,
			output_tokens,
			cache_creation_tokens,
			cache_read_tokens,
			total_cost,
			actual_cost,
			account_cost,
			total_duration_ms,
			active_users,
			computed_at
		)
		SELECT
			daily.bucket_date,
			daily.total_requests,
			daily.input_tokens,
			daily.output_tokens,
			daily.cache_creation_tokens,
			daily.cache_read_tokens,
			daily.total_cost,
			daily.actual_cost,
			daily.account_cost,
			daily.total_duration_ms,
			COALESCE(user_counts.active_users, 0) AS active_users,
			NOW()
		FROM daily
		LEFT JOIN user_counts ON user_counts.bucket_date = daily.bucket_date
		ON CONFLICT (bucket_date)
		DO UPDATE SET
			total_requests = EXCLUDED.total_requests,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			total_cost = EXCLUDED.total_cost,
			actual_cost = EXCLUDED.actual_cost,
			account_cost = EXCLUDED.account_cost,
			total_duration_ms = EXCLUDED.total_duration_ms,
			active_users = EXCLUDED.active_users,
			computed_at = EXCLUDED.computed_at
	`
	_, err := r.sql.ExecContext(ctx, query, start, end, start, end, tzName)
	return err
}
