package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

// The audit covers whole local days, irrespective of the batch's account/model
// filters or creation cutoff. A filtered batch must not certify only its own
// subset while overwriting the dashboard total for every account that day.
func financeBaselineScope(candidates []service.FinanceRecalculationCandidate) (*service.FinanceRetainedBaseline, error) {
	seen := make(map[string]bool)
	for _, c := range candidates {
		if c.Usage == nil || c.TargetCreatedAt.IsZero() {
			return nil, service.ErrFinanceRecalculationInvalid
		}
		seen[c.TargetCreatedAt.In(timezone.Location()).Format("2006-01-02")] = true
	}
	if len(seen) > service.FinanceRetainedBaselineMaxDays {
		return nil, service.ErrFinanceRecalculationTooLarge
	}
	baseline := &service.FinanceRetainedBaseline{Version: 1, Timezone: timezone.Name(), Days: []service.FinanceRetainedBaselineDay{}}
	for day := range seen {
		baseline.Days = append(baseline.Days, service.FinanceRetainedBaselineDay{Day: day})
	}
	sort.Slice(baseline.Days, func(i, j int) bool { return baseline.Days[i].Day < baseline.Days[j].Day })
	return baseline, nil
}

type financeBaselineSum struct {
	requests               int64
	account, total, actual decimal.Decimal
}

func (s *financeBaselineSum) add(account, total, actual string) error {
	values := []*decimal.Decimal{&s.account, &s.total, &s.actual}
	for i, raw := range []string{account, total, actual} {
		v, err := decimal.NewFromString(raw)
		if err != nil {
			return service.ErrFinanceRecalculationCoverageUnknown
		}
		*values[i] = values[i].Add(v)
	}
	s.requests++
	return nil
}

func (s financeBaselineSum) matches(a service.FinanceRetainedBaselineAggregate) bool {
	if s.requests != a.Requests {
		return false
	}
	for i, raw := range []string{a.AccountCost, a.TotalCost, a.ActualCost} {
		v, err := decimal.NewFromString(raw)
		if err != nil || !v.Equal([]decimal.Decimal{s.account, s.total, s.actual}[i].Round(10)) {
			return false
		}
	}
	return true
}

// Read a deterministic audit using the caller's single SQL snapshot. This is a
// strict reconciliation, not an instruction to replace mismatching old totals.
// Existing empty hourly buckets are retained; nonempty source buckets require
// corresponding aggregates. Aggregate-only nonzero buckets are rejected.
func financeLoadRetainedBaseline(ctx context.Context, tx *sql.Tx, scope *service.FinanceRetainedBaseline) (*service.FinanceRetainedBaseline, error) {
	if scope == nil || scope.Version != 1 || scope.Timezone != timezone.Name() {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	if len(scope.Days) > service.FinanceRetainedBaselineMaxDays {
		return nil, service.ErrFinanceRecalculationTooLarge
	}
	out := &service.FinanceRetainedBaseline{Version: 1, Timezone: timezone.Name(), Days: []service.FinanceRetainedBaselineDay{}}
	if err := tx.QueryRowContext(ctx, `SELECT deletion_generation FROM finance_aggregation_coverage_state WHERE id=1`).Scan(&out.DeletionGeneration); err != nil {
		return nil, err
	}
	var used int64
	previous := ""
	for _, dayScope := range scope.Days {
		start, err := time.ParseInLocation("2006-01-02", dayScope.Day, timezone.Location())
		if err != nil || dayScope.Day <= previous {
			return nil, service.ErrFinanceRecalculationInvalid
		}
		previous = dayScope.Day
		end := start.AddDate(0, 0, 1)
		day := service.FinanceRetainedBaselineDay{Day: dayScope.Day, Hourly: []service.FinanceRetainedBaselineAggregate{}}
		hours := map[string]*financeBaselineSum{}
		total := financeBaselineSum{}
		digest := sha256.New()
		rows, err := tx.QueryContext(ctx, `SELECT u.created_at,`+financeUsageEvidence+`,
		 COALESCE(COALESCE(u.account_stats_cost,u.total_cost)*COALESCE(u.account_rate_multiplier,1),0)::text,
		 COALESCE(u.total_cost,0)::text,COALESCE(u.actual_cost,0)::text
		 FROM usage_logs u WHERE u.created_at >= $1 AND u.created_at < $2
		 ORDER BY u.created_at,u.id LIMIT $3`, start, end, int64(service.FinanceRecalculationMaxRows)+1-used)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var at time.Time
			var raw []byte
			var account, customerTotal, actual string
			if err = rows.Scan(&at, &raw, &account, &customerTotal, &actual); err != nil {
				_ = rows.Close()
				return nil, err
			}
			used++
			if used > service.FinanceRecalculationMaxRows {
				_ = rows.Close()
				return nil, service.ErrFinanceRecalculationTooLarge
			}
			// JSONB's canonical key ordering and newline framing make NULLs and
			// each individual row identity visible in the deterministic digest.
			_, _ = digest.Write(raw)
			_, _ = digest.Write([]byte{'\n'})
			key := at.In(timezone.Location()).Truncate(time.Hour).Format(time.RFC3339)
			if hours[key] == nil {
				hours[key] = &financeBaselineSum{}
			}
			if err = hours[key].add(account, customerTotal, actual); err == nil {
				err = total.add(account, customerTotal, actual)
			}
			if err != nil {
				_ = rows.Close()
				return nil, err
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		day.SourceRows = total.requests
		day.SourceDigest = hex.EncodeToString(digest.Sum(nil))
		rows, err = tx.QueryContext(ctx, `SELECT bucket_start,total_requests,account_cost::text,total_cost::text,actual_cost::text FROM usage_dashboard_hourly WHERE bucket_start >= $1 AND bucket_start < $2 ORDER BY bucket_start LIMIT 26`, start, end)
		if err != nil {
			return nil, err
		}
		hourlyTotal := financeBaselineSum{}
		for rows.Next() {
			var at time.Time
			var aggregate service.FinanceRetainedBaselineAggregate
			if err = rows.Scan(&at, &aggregate.Requests, &aggregate.AccountCost, &aggregate.TotalCost, &aggregate.ActualCost); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if len(day.Hourly) >= 25 || !at.Equal(at.Truncate(time.Hour)) {
				_ = rows.Close()
				return nil, service.ErrFinanceRecalculationCoverageUnknown
			}
			aggregate.Bucket = at.In(timezone.Location()).Format(time.RFC3339)
			sum := hours[aggregate.Bucket]
			if sum == nil {
				sum = &financeBaselineSum{}
			}
			if !sum.matches(aggregate) {
				_ = rows.Close()
				return nil, service.ErrFinanceRecalculationCoverageUnknown
			}
			if err = hourlyTotal.add(aggregate.AccountCost, aggregate.TotalCost, aggregate.ActualCost); err != nil {
				_ = rows.Close()
				return nil, err
			}
			// add counts one row; this sum represents underlying requests.
			hourlyTotal.requests += aggregate.Requests - 1
			delete(hours, aggregate.Bucket)
			day.Hourly = append(day.Hourly, aggregate)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
		if len(hours) != 0 {
			return nil, service.ErrFinanceRecalculationCoverageUnknown
		}
		day.Daily.Bucket = day.Day
		err = tx.QueryRowContext(ctx, `SELECT total_requests,account_cost::text,total_cost::text,actual_cost::text FROM usage_dashboard_daily WHERE bucket_date=$1::date`, day.Day).Scan(&day.Daily.Requests, &day.Daily.AccountCost, &day.Daily.TotalCost, &day.Daily.ActualCost)
		// The normal dashboard writes SUM(hourly rounded NUMERICs); repair
		// writes ROUND(SUM(raw source products),10). Both are exact known
		// producer paths, not a numeric tolerance. Hourly buckets have already
		// individually reconciled against retained sources above.
		if err == sql.ErrNoRows || (err == nil && !total.matches(day.Daily) && !hourlyTotal.matches(day.Daily)) {
			return nil, service.ErrFinanceRecalculationCoverageUnknown
		}
		if err != nil {
			return nil, err
		}
		out.Days = append(out.Days, day)
	}
	return out, nil
}

func financeRetainedBaselineDigest(b *service.FinanceRetainedBaseline) (string, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

// Only a successful guard revalidation can authorize legacy synchronization.
// The proof is transaction-local and disappears on commit/rollback; a forged
// config day or a call to Synchronize without Acquire cannot bypass coverage.
func financeAuthorizeRetainedBaseline(ctx context.Context, tx *sql.Tx, baseline *service.FinanceRetainedBaseline) error {
	digest, err := financeRetainedBaselineDigest(baseline)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.finance_retained_baseline', $1, true)`, digest)
	return err
}

func financeAuthorizedBaselineDays(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch) (map[string]bool, error) {
	days := map[string]bool{}
	if !b.AcceptRetainedBaseline {
		return days, nil
	}
	baseline := b.Config.RetainedBaseline
	if baseline == nil || baseline.Version != 1 || baseline.Timezone != timezone.Name() || len(baseline.Days) > service.FinanceRetainedBaselineMaxDays {
		return nil, service.ErrFinanceRecalculationCoverageUnknown
	}
	var proof string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(current_setting('sub2api.finance_retained_baseline', true),'')`).Scan(&proof); err != nil {
		return nil, err
	}
	digest, err := financeRetainedBaselineDigest(baseline)
	if err != nil {
		return nil, err
	}
	if digest != proof {
		return nil, service.ErrFinanceRecalculationCoverageUnknown
	}
	for _, day := range baseline.Days {
		days[day.Day] = true
	}
	return days, nil
}

// A calculator opt-in is emitted only after the entire saved day scope passed
// reconciliation. Never trust a persisted per-model boolean on its own.
func financeAttachRetainedBaseline(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch, scope service.FinanceRecalculationConfig, config *service.FinanceRecalculationConfig) error {
	if !b.AcceptRetainedBaseline {
		return nil
	}
	if b.Kind != service.FinanceRecalculationUsageCost {
		return service.ErrFinanceRecalculationInvalid
	}
	baseline, err := financeLoadRetainedBaseline(ctx, tx, scope.RetainedBaseline)
	if err != nil {
		return err
	}
	config.RetainedBaseline = baseline
	for i := range config.Usage {
		config.Usage[i].AcceptRetainedBaseline = true
	}
	return nil
}
