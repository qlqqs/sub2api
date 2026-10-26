package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type financeChannelCostRepository struct{ db *sql.DB }

func NewFinanceChannelCostRepository(db *sql.DB) service.FinanceChannelCostRepository {
	return &financeChannelCostRepository{db: db}
}

func (r *financeChannelCostRepository) Get(ctx context.Context, id int64) (*service.FinanceChannelCostConfig, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin channel cost snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := loadFinanceChannelCostConfig(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit channel cost snapshot: %w", err)
	}
	return result, nil
}

func (r *financeChannelCostRepository) CompareAndSwap(ctx context.Context, id int64, input service.FinanceChannelCostUpdate) (*service.FinanceChannelCostConfig, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin channel cost update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Legacy channel Update also obtains this parent-row lock BEFORE replacing
	// rules, so a committed legacy edit is included in the version comparison.
	current, err := loadFinanceChannelCostConfig(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	if current.Version != input.ExpectedVersion {
		return nil, service.ErrFinanceCostConfigChanged
	}
	if input.ApplyPricingToAccountStats != nil {
		current.ApplyPricingToAccountStats = *input.ApplyPricingToAccountStats
	}
	if _, err = tx.ExecContext(ctx, `UPDATE channels SET apply_pricing_to_account_stats=$2, updated_at=NOW() WHERE id=$1`, id, current.ApplyPricingToAccountStats); err != nil {
		return nil, fmt.Errorf("update channel cost switch: %w", err)
	}
	if input.AccountStatsPricingRules != nil {
		if err = replaceAccountStatsPricingRulesTx(ctx, tx, id, *input.AccountStatsPricingRules); err != nil {
			return nil, err
		}
	}
	// Reload persisted precision and generated IDs inside the SAME transaction.
	result, err := loadFinanceChannelCostConfig(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit channel cost update: %w", err)
	}
	return result, nil
}

func loadFinanceChannelCostConfig(ctx context.Context, tx *sql.Tx, id int64, lock bool) (*service.FinanceChannelCostConfig, error) {
	result := &service.FinanceChannelCostConfig{ChannelID: id}
	query := `SELECT apply_pricing_to_account_stats FROM channels WHERE id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	err := tx.QueryRowContext(ctx, query, id).Scan(&result.ApplyPricingToAccountStats)
	if err == sql.ErrNoRows {
		return nil, service.ErrChannelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load channel cost switch: %w", err)
	}
	rules, err := batchLoadAccountStatsPricingRules(ctx, tx, []int64{id})
	if err != nil {
		return nil, err
	}
	result.AccountStatsPricingRules = rules[id]
	if result.AccountStatsPricingRules == nil {
		result.AccountStatsPricingRules = []service.AccountStatsPricingRule{}
	}
	result.Version, err = service.FinanceChannelCostConfigVersion(result.ApplyPricingToAccountStats, result.AccountStatsPricingRules)
	if err != nil {
		return nil, fmt.Errorf("fingerprint channel cost config: %w", err)
	}
	return result, nil
}

// LockFinanceChannelCostConfig lets recalculation guards share the exact editor
// fingerprint and parent-row lock in THEIR transaction. Lock multiple channel
// IDs in ascending order, before usage/order rows. The caller owns commit and
// rollback. Other configuration dependencies need their own locks/snapshots.
func LockFinanceChannelCostConfig(ctx context.Context, tx *sql.Tx, id int64) (*service.FinanceChannelCostConfig, error) {
	return loadFinanceChannelCostConfig(ctx, tx, id, true)
}
