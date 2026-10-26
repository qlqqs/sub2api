package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Cost snapshots are read directly from SQL, never from potentially stale
// gateway caches. Locks cover membership phantoms, child rule replacement and
// the original settings entrances, including old processes without new hooks.
// Batches are bounded; SHARE locks are held only for the short apply transaction.
type FinanceRecalculationConfigRepository struct {
	db      *sql.DB
	billing *service.BillingService
}

func NewFinanceRecalculationConfigRepository(db *sql.DB, billing *service.BillingService) *FinanceRecalculationConfigRepository {
	return &FinanceRecalculationConfigRepository{db: db, billing: billing}
}

func (r *FinanceRecalculationConfigRepository) Snapshot(ctx context.Context, b *service.FinanceRecalculationBatch, candidates []service.FinanceRecalculationCandidate) (service.FinanceRecalculationConfig, error) {
	scope := service.FinanceRecalculationConfig{}
	for _, candidate := range candidates {
		if in := candidate.Usage; in != nil {
			model := strings.TrimSpace(in.UpstreamModel)
			if model == "" {
				model = strings.TrimSpace(in.RequestedModel)
			}
			if model == "" {
				model = strings.TrimSpace(in.Model)
			}
			scope.Usage = append(scope.Usage, service.FinanceUsageCostSnapshot{AccountID: in.AccountID, GroupID: in.GroupID, Model: model})
		}
		if in := candidate.Payment; in != nil && in.ProviderID > 0 {
			scope.Payment = append(scope.Payment, service.FinanceRecalculationPaymentRule{ProviderID: in.ProviderID})
		}
	}
	if b.AcceptRetainedBaseline {
		if b.Kind != service.FinanceRecalculationUsageCost {
			return scope, service.ErrFinanceRecalculationInvalid
		}
		var err error
		scope.RetainedBaseline, err = financeBaselineScope(candidates)
		if err != nil {
			return scope, err
		}
	}
	return r.read(ctx, b, scope)
}
func (r *FinanceRecalculationConfigRepository) Current(ctx context.Context, b *service.FinanceRecalculationBatch) (service.FinanceRecalculationConfig, error) {
	return r.read(ctx, b, b.Config)
}
func (r *FinanceRecalculationConfigRepository) read(ctx context.Context, b *service.FinanceRecalculationBatch, scope service.FinanceRecalculationConfig) (service.FinanceRecalculationConfig, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return service.FinanceRecalculationConfig{}, err
	}
	defer func() { _ = tx.Rollback() }()
	price, release := r.billing.LockFinancePricing()
	defer release()
	result, err := financeLoadCostConfig(ctx, tx, b.Kind, scope, price)
	if err != nil {
		return result, err
	}
	if err = financeAttachRetainedBaseline(ctx, tx, b, scope, &result); err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func (r *FinanceRecalculationConfigRepository) Acquire(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch) (string, func(), error) {
	var tables string
	switch b.Kind {
	case service.FinanceRecalculationUsageCost:
		tables = "accounts, account_model_cost_multipliers, groups, channels, channel_groups, channel_account_stats_pricing_rules, channel_account_stats_model_pricing, channel_account_stats_pricing_intervals"
	case service.FinanceRecalculationPaymentFee:
		tables = "payment_provider_instances"
	default:
		return "", nil, service.ErrFinanceRecalculationInvalid
	}
	if _, err := tx.ExecContext(ctx, "LOCK TABLE "+tables+" IN SHARE MODE"); err != nil {
		return "", nil, fmt.Errorf("lock current cost configuration: %w", err)
	}
	// Acquire relation locks before the advisory lock: legacy FK deletion and
	// TRUNCATE acquire these first. Reversing them creates a lock-order cycle.
	if b.Kind == service.FinanceRecalculationUsageCost {
		sourceLock := "LOCK TABLE usage_logs IN ROW SHARE MODE"
		if b.AcceptRetainedBaseline {
			// Legacy whole-day evidence must remain stable through the cost
			// writes and aggregate synchronization, including gateway inserts.
			// EXCLUSIVE also conflicts with cleanup's pre-advisory ROW
			// SHARE lock, avoiding SHARE→ROW EXCLUSIVE lock upgrades while
			// cleanup owns the advisory lock. It serializes two baselines
			// before either can upgrade; ordinary SELECT remains compatible.
			sourceLock = "LOCK TABLE usage_logs IN EXCLUSIVE MODE"
		}
		if _, err := tx.ExecContext(ctx, sourceLock); err != nil {
			return "", nil, err
		}
	}
	if err := AcquireFinanceAggregationLock(ctx, tx); err != nil {
		return "", nil, err
	}
	price, release := r.billing.LockFinancePricing()
	config, err := financeLoadCostConfig(ctx, tx, b.Kind, b.Config, price)
	if err != nil {
		return "", release, err
	}
	if err = financeAttachRetainedBaseline(ctx, tx, b, b.Config, &config); err != nil {
		return "", release, err
	}
	if b.AcceptRetainedBaseline {
		if err = financeAuthorizeRetainedBaseline(ctx, tx, config.RetainedBaseline); err != nil {
			return "", release, err
		}
	}
	fingerprint, err := service.FinanceRecalculationConfigFingerprint(config)
	return fingerprint, release, err
}
func (r *FinanceRecalculationConfigRepository) Synchronize(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch, items []service.FinanceRecalculationItem) error {
	return SynchronizeFinanceCostAggregates(ctx, tx, b, items)
}

func financeLoadCostConfig(ctx context.Context, tx *sql.Tx, kind string, scope service.FinanceRecalculationConfig, price func(string) (*service.ModelPricing, error)) (service.FinanceRecalculationConfig, error) {
	result := service.FinanceRecalculationConfig{}
	if kind == service.FinanceRecalculationPaymentFee {
		seen := map[int64]bool{}
		for _, key := range scope.Payment {
			if seen[key.ProviderID] {
				continue
			}
			seen[key.ProviderID] = true
			rule := service.FinanceRecalculationPaymentRule{ProviderID: key.ProviderID}
			err := tx.QueryRowContext(ctx, `SELECT channel_fee_rate::text FROM payment_provider_instances WHERE id=$1`, key.ProviderID).Scan(&rule.FeeRate)
			if err != nil && err != sql.ErrNoRows {
				return result, err
			}
			// Empty rate represents a deleted provider, never zero.
			result.Payment = append(result.Payment, rule)
		}
		sort.Slice(result.Payment, func(i, j int) bool { return result.Payment[i].ProviderID < result.Payment[j].ProviderID })
		return result, nil
	}
	coverageFrom, err := FinanceAggregationEligibleFrom(ctx, tx)
	if err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for _, key := range scope.Usage {
		identity := fmt.Sprintf("%d/%d/%s", key.AccountID, key.GroupID, key.Model)
		if seen[identity] {
			continue
		}
		seen[identity] = true
		snapshot := service.FinanceUsageCostSnapshot{AccountID: key.AccountID, GroupID: key.GroupID, Model: key.Model, CoverageFrom: &coverageFrom}
		var platform, accountType string
		var enabled bool
		err := tx.QueryRowContext(ctx, `SELECT COALESCE(rate_multiplier,1)::text, platform, type, COALESCE(extra->'openai_long_context_billing_enabled'='true'::jsonb,FALSE) FROM accounts WHERE id=$1 AND deleted_at IS NULL`, key.AccountID).Scan(&snapshot.DefaultMultiplier, &platform, &accountType, &enabled)
		if err == sql.ErrNoRows {
			snapshot.MissingReason = "missing_current_account"
			result.Usage = append(result.Usage, snapshot)
			continue
		}
		if err != nil {
			return result, err
		}
		snapshot.LongContextPricingEnabled = service.FinanceAccountLongContextPolicy(platform, accountType, key.Model, enabled)
		rows, err := tx.QueryContext(ctx, `SELECT model_pattern,rate_multiplier::double precision FROM account_model_cost_multipliers WHERE account_id=$1 ORDER BY model_pattern`, key.AccountID)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var rule service.AccountModelCostRule
			if err = rows.Scan(&rule.Model, &rule.RateMultiplier); err != nil {
				_ = rows.Close()
				return result, err
			}
			snapshot.ModelRates = append(snapshot.ModelRates, rule)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return result, err
		}
		if key.GroupID > 0 {
			err = tx.QueryRowContext(ctx, `SELECT platform FROM groups WHERE id=$1 AND deleted_at IS NULL`, key.GroupID).Scan(&snapshot.Platform)
			if err == sql.ErrNoRows {
				snapshot.MissingReason = "missing_current_group"
				result.Usage = append(result.Usage, snapshot)
				continue
			}
			if err != nil {
				return result, err
			}
			var channelID int64
			err = tx.QueryRowContext(ctx, `SELECT channel_id FROM channel_groups WHERE group_id=$1`, key.GroupID).Scan(&channelID)
			if err != nil && err != sql.ErrNoRows {
				return result, err
			}
			if err == nil {
				channel, loadErr := loadFinanceChannelCostConfig(ctx, tx, channelID, false)
				if loadErr != nil {
					return result, loadErr
				}
				snapshot.Channel = &service.FinanceCostChannelSnapshot{ID: channelID, ApplyPricingToAccountStats: channel.ApplyPricingToAccountStats, Rules: channel.AccountStatsPricingRules}
				snapshot.ModelPricing, err = price(key.Model)
				if err != nil {
					return result, err
				}
			}
		}
		result.Usage = append(result.Usage, snapshot)
	}
	sort.Slice(result.Usage, func(i, j int) bool {
		a, b := result.Usage[i], result.Usage[j]
		if a.AccountID != b.AccountID {
			return a.AccountID < b.AccountID
		}
		if a.GroupID != b.GroupID {
			return a.GroupID < b.GroupID
		}
		return a.Model < b.Model
	})
	return result, nil
}
