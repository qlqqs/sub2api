package repository

// CUSTOM(fork): 账号内单个模型倍率（表 account_model_cost_multipliers，迁移 254）。原生 SQL，参数化。

import (
	"context"
	"database/sql"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountModelCostRepository struct {
	db *sql.DB
}

// NewAccountModelCostRepository 创建账号模型倍率仓储。
func NewAccountModelCostRepository(db *sql.DB) service.AccountModelCostRepository {
	return &accountModelCostRepository{db: db}
}

func (r *accountModelCostRepository) ListAll(ctx context.Context) (map[int64][]service.AccountModelCostRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.account_id, m.model_pattern, m.rate_multiplier::double precision
		FROM account_model_cost_multipliers m
		JOIN accounts g ON g.id = m.account_id
		WHERE g.deleted_at IS NULL
		ORDER BY m.account_id, m.model_pattern
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64][]service.AccountModelCostRule)
	for rows.Next() {
		var accountID int64
		var rule service.AccountModelCostRule
		if err := rows.Scan(&accountID, &rule.Model, &rule.RateMultiplier); err != nil {
			return nil, err
		}
		out[accountID] = append(out[accountID], rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *accountModelCostRepository) ListByAccount(ctx context.Context, accountID int64) ([]service.AccountModelCostRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT model_pattern, rate_multiplier::double precision
		FROM account_model_cost_multipliers
		WHERE account_id = $1
		ORDER BY model_pattern
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.AccountModelCostRule, 0)
	for rows.Next() {
		var rule service.AccountModelCostRule
		if err := rows.Scan(&rule.Model, &rule.RateMultiplier); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *accountModelCostRepository) ReplaceForAccount(ctx context.Context, accountID int64, rules []service.AccountModelCostRule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var lockedID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, accountID).Scan(&lockedID)
	if err == sql.ErrNoRows {
		return service.ErrAccountNotFound
	}
	exists := lockedID > 0
	if err != nil {
		return err
	}
	if !exists {
		return service.ErrAccountNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM account_model_cost_multipliers WHERE account_id = $1`, accountID); err != nil {
		return fmt.Errorf("delete account model rates: %w", err)
	}
	if len(rules) > 0 {
		models := make([]string, 0, len(rules))
		rates := make([]float64, 0, len(rules))
		for _, rule := range rules {
			models = append(models, rule.Model)
			rates = append(rates, rule.RateMultiplier)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO account_model_cost_multipliers (account_id, model_pattern, rate_multiplier, created_at, updated_at)
			SELECT $1::bigint, data.model_pattern, data.rate_multiplier, NOW(), NOW()
			FROM unnest($2::text[], $3::double precision[]) AS data(model_pattern, rate_multiplier)
		`, accountID, pq.Array(models), pq.Array(rates)); err != nil {
			return translatePersistenceError(err, nil, infraerrors.Conflict("ACCOUNT_MODEL_COST_DUPLICATE", "duplicate model pattern in account"))
		}
	}
	return tx.Commit()
}

func (r *accountModelCostRepository) AccountExists(ctx context.Context, accountID int64) (bool, error) {
	return accountModelCostAccountExists(ctx, r.db, accountID)
}

type accountModelCostQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func accountModelCostAccountExists(ctx context.Context, q accountModelCostQueryer, accountID int64) (bool, error) {
	var exists bool
	if err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM accounts WHERE id = $1 AND deleted_at IS NULL)`, accountID,
	).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
