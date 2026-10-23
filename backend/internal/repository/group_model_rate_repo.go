package repository

// CUSTOM(fork): 分组内单个模型倍率（表 group_model_rate_multipliers，迁移 253）。原生 SQL，参数化。

import (
	"context"
	"database/sql"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type groupModelRateRepository struct {
	db *sql.DB
}

// NewGroupModelRateRepository 创建分组模型倍率仓储。
func NewGroupModelRateRepository(db *sql.DB) service.GroupModelRateRepository {
	return &groupModelRateRepository{db: db}
}

func (r *groupModelRateRepository) ListAll(ctx context.Context) (map[int64][]service.GroupModelRateRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.group_id, m.model_pattern, m.rate_multiplier::double precision
		FROM group_model_rate_multipliers m
		JOIN groups g ON g.id = m.group_id
		WHERE g.deleted_at IS NULL
		ORDER BY m.group_id, m.model_pattern
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64][]service.GroupModelRateRule)
	for rows.Next() {
		var groupID int64
		var rule service.GroupModelRateRule
		if err := rows.Scan(&groupID, &rule.Model, &rule.RateMultiplier); err != nil {
			return nil, err
		}
		out[groupID] = append(out[groupID], rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *groupModelRateRepository) ListByGroup(ctx context.Context, groupID int64) ([]service.GroupModelRateRule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT model_pattern, rate_multiplier::double precision
		FROM group_model_rate_multipliers
		WHERE group_id = $1
		ORDER BY model_pattern
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.GroupModelRateRule, 0)
	for rows.Next() {
		var rule service.GroupModelRateRule
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

func (r *groupModelRateRepository) ReplaceForGroup(ctx context.Context, groupID int64, rules []service.GroupModelRateRule) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	exists, err := groupModelRateGroupExists(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if !exists {
		return service.ErrGroupNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_model_rate_multipliers WHERE group_id = $1`, groupID); err != nil {
		return fmt.Errorf("delete group model rates: %w", err)
	}
	if len(rules) > 0 {
		models := make([]string, 0, len(rules))
		rates := make([]float64, 0, len(rules))
		for _, rule := range rules {
			models = append(models, rule.Model)
			rates = append(rates, rule.RateMultiplier)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO group_model_rate_multipliers (group_id, model_pattern, rate_multiplier, created_at, updated_at)
			SELECT $1::bigint, data.model_pattern, data.rate_multiplier, NOW(), NOW()
			FROM unnest($2::text[], $3::double precision[]) AS data(model_pattern, rate_multiplier)
		`, groupID, pq.Array(models), pq.Array(rates)); err != nil {
			return translatePersistenceError(err, nil, infraerrors.Conflict("GROUP_MODEL_RATE_DUPLICATE", "duplicate model pattern in group"))
		}
	}
	return tx.Commit()
}

func (r *groupModelRateRepository) CopyGroup(ctx context.Context, fromID, toID int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO group_model_rate_multipliers (group_id, model_pattern, rate_multiplier, created_at, updated_at)
		SELECT $2::bigint, model_pattern, rate_multiplier, NOW(), NOW()
		FROM group_model_rate_multipliers
		WHERE group_id = $1
		ON CONFLICT (group_id, model_pattern)
		DO UPDATE SET rate_multiplier = EXCLUDED.rate_multiplier, updated_at = NOW()
	`, fromID, toID)
	return err
}

func (r *groupModelRateRepository) GroupExists(ctx context.Context, groupID int64) (bool, error) {
	return groupModelRateGroupExists(ctx, r.db, groupID)
}

type groupModelRateQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func groupModelRateGroupExists(ctx context.Context, q groupModelRateQueryer, groupID int64) (bool, error) {
	var exists bool
	if err := q.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1 AND deleted_at IS NULL)`, groupID,
	).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}
