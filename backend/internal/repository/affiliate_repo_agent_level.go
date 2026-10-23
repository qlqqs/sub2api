package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// paidPaymentOrderStatuses 是已付款并到账的订单状态；退款流程中的状态在未真正退款前
// 仍视为到账，只有 PARTIALLY_REFUNDED / REFUNDED 才扣除 refund_amount。
const paidPaymentOrderStatuses = `('COMPLETED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING', 'REFUND_FAILED', 'PARTIALLY_REFUNDED', 'REFUNDED')`

func (r *affiliateRepository) CountQualifiedInvitees(ctx context.Context, inviterID int64, rechargedOnly bool) (int, error) {
	client := clientFromContext(ctx, r.client)
	query := "SELECT COUNT(*) FROM user_affiliates ua WHERE ua.inviter_id = $1"
	if rechargedOnly {
		query += ` AND EXISTS (
    SELECT 1 FROM payment_orders po
    WHERE po.user_id = ua.user_id
      AND po.status IN ` + paidPaymentOrderStatuses + `
      AND po.amount - po.bonus_amount > 0)`
	}
	total, err := scanInt64(ctx, client, query, inviterID)
	if err != nil {
		return 0, fmt.Errorf("count qualified invitees: %w", err)
	}
	return int(total), nil
}

func (r *affiliateRepository) PromoteAgentLevelAuto(ctx context.Context, userID int64, level int) (bool, error) {
	client := clientFromContext(ctx, r.client)
	res, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET agent_level = $2,
    agent_level_source = $3,
    agent_level_updated_at = NOW(),
    agent_level_updated_by = NULL,
    updated_at = NOW()
WHERE user_id = $1
  AND agent_level_locked = FALSE
  AND agent_level > $2`, userID, level, service.AffiliateAgentLevelSourceAuto)
	if err != nil {
		return false, fmt.Errorf("promote agent level: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read promote agent level result: %w", err)
	}
	return affected > 0, nil
}

// SetAgentLevelManual 管理员设置等级。等级数字越大越低：目标等级数字更大是降级，
// 锁定后不再自动升级；目标更小是升级，解除锁定；相同等级只更新来源。
func (r *affiliateRepository) SetAgentLevelManual(ctx context.Context, userIDs []int64, level int, operatorID int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET agent_level_locked = CASE
        WHEN $1 > agent_level THEN TRUE
        WHEN $1 < agent_level THEN FALSE
        ELSE agent_level_locked
    END,
    agent_level = $1,
    agent_level_source = $2,
    agent_level_updated_at = NOW(),
    agent_level_updated_by = $4,
    updated_at = NOW()
WHERE user_id = ANY($3)`, level, service.AffiliateAgentLevelSourceManual, pq.Array(userIDs), nullableOperatorArg(operatorID))
		if err != nil {
			return fmt.Errorf("set agent level: %w", err)
		}
		return nil
	})
}

func (r *affiliateRepository) SetAgentLevelLocked(ctx context.Context, userIDs []int64, locked bool) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET agent_level_locked = $1,
    updated_at = NOW()
WHERE user_id = ANY($2)`, locked, pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("set agent level locked: %w", err)
		}
		return nil
	})
}

// ListInviterIDs 按 ID 升序分页列出仍有效（未删除）的邀请人，供重新评估等级使用。
func (r *affiliateRepository) ListInviterIDs(ctx context.Context, afterUserID int64, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 500
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT DISTINCT ua.inviter_id
FROM user_affiliates ua
JOIN users u ON u.id = ua.inviter_id AND u.deleted_at IS NULL
WHERE ua.inviter_id > $1
ORDER BY ua.inviter_id
LIMIT $2`, afterUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list inviter ids: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *affiliateRepository) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.withTx(ctx, func(txCtx context.Context, _ *dbent.Client) error {
		return fn(txCtx)
	})
}

// ListUsersWithPendingConsumption 列出有邀请人或归属代理、且在水位之后、滞后窗口之前产生了余额消费的用户。
func (r *affiliateRepository) ListUsersWithPendingConsumption(ctx context.Context, lag time.Duration, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 500
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id
FROM user_affiliates ua
WHERE (ua.inviter_id IS NOT NULL OR ua.agent_owner_id IS NOT NULL)
  AND EXISTS (
    SELECT 1 FROM usage_logs ul
    WHERE ul.user_id = ua.user_id
      AND ul.billing_type = 0
      AND ul.created_at > ua.rebate_settled_at
      AND ul.created_at <= NOW() - make_interval(secs => $1))
ORDER BY ua.user_id
LIMIT $2`, lag.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("list users with pending consumption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LockPendingConsumption 锁定用户的 user_affiliates 行，并在同一语句内读取水位、
// 窗口内余额消费合计与真实充值总额（到账金额 − 赠送 − 已退款）。
func (r *affiliateRepository) LockPendingConsumption(ctx context.Context, userID int64, lag time.Duration) (*service.AffiliatePendingConsumption, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
WITH locked AS (
    SELECT user_id, rebate_settled_at, rebate_settled_consumption
    FROM user_affiliates
    WHERE user_id = $1
    FOR UPDATE
), bounds AS (
    SELECT NOW() - make_interval(secs => $2) AS until_at
)
SELECT l.rebate_settled_at,
       b.until_at,
       l.rebate_settled_consumption::double precision,
       (SELECT COALESCE(SUM(ul.actual_cost), 0)
        FROM usage_logs ul
        WHERE ul.user_id = l.user_id
          AND ul.billing_type = 0
          AND ul.created_at > l.rebate_settled_at
          AND ul.created_at <= b.until_at)::double precision,
       (SELECT COALESCE(SUM(po.amount - po.bonus_amount
               - CASE WHEN po.status IN ('PARTIALLY_REFUNDED', 'REFUNDED') THEN po.refund_amount ELSE 0 END), 0)
        FROM payment_orders po
        WHERE po.user_id = l.user_id
          AND po.order_type = 'balance'
          AND po.status IN `+paidPaymentOrderStatuses+`)::double precision
FROM locked l
CROSS JOIN bounds b`, userID, lag.Seconds())
	if err != nil {
		return nil, fmt.Errorf("lock pending consumption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}
	pending := &service.AffiliatePendingConsumption{UserID: userID}
	if err := rows.Scan(&pending.SettledAt, &pending.UntilAt, &pending.SettledConsumption, &pending.DeltaCost, &pending.PaidTotal); err != nil {
		return nil, err
	}
	return pending, rows.Err()
}

// AdvanceConsumptionWatermark 推进结算水位。不更新 updated_at，避免结算把用户顶到
// 管理端列表前面。
func (r *affiliateRepository) AdvanceConsumptionWatermark(ctx context.Context, userID int64, until time.Time, settledConsumption float64) error {
	client := clientFromContext(ctx, r.client)
	res, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET rebate_settled_at = $2,
    rebate_settled_consumption = $3
WHERE user_id = $1`, userID, until, settledConsumption)
	if err != nil {
		return fmt.Errorf("advance consumption watermark: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read advance watermark result: %w", err)
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func nullableOperatorArg(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

// GetAffiliateForUpdate 在调用方事务内锁定并读取用户的 user_affiliates 行（不存在时先创建）。
func (r *affiliateRepository) GetAffiliateForUpdate(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	client := clientFromContext(ctx, r.client)
	if _, err := ensureUserAffiliateWithClient(ctx, client, userID); err != nil {
		return nil, err
	}
	return queryAffiliateSummary(ctx, client, "SELECT"+affiliateSummaryColumns+" FROM user_affiliates WHERE user_id = $1 FOR UPDATE", userID)
}

// SetAgentLevelByAgent 记录一级代理的任命或取消：来源为 agent，操作人为任命人。
func (r *affiliateRepository) SetAgentLevelByAgent(ctx context.Context, targetID int64, level int, appointerID int64, locked bool) error {
	client := clientFromContext(ctx, r.client)
	res, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET agent_level = $2,
    agent_level_source = $3,
    agent_level_locked = $4,
    agent_level_updated_at = NOW(),
    agent_level_updated_by = $5,
    updated_at = NOW()
WHERE user_id = $1`, targetID, level, service.AffiliateAgentLevelSourceAgent, locked, appointerID)
	if err != nil {
		return fmt.Errorf("set agent level by agent: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read set agent level by agent result: %w", err)
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

// SetAgentOwnerByAdmin 改写用户自己的归属快照：设为一级代理 ownerID（档位 1、上方一级清空）
// 或清除（ownerID 为 nil）。不改邀请关系。原本既没有邀请人也没有归属的用户，结算水位
// 移到当前时刻，避免把设置之前的消费补发给新上级；其余用户保留水位。
func (r *affiliateRepository) SetAgentOwnerByAdmin(ctx context.Context, userID int64, ownerID *int64, adminID int64) error {
	client := clientFromContext(ctx, r.client)
	res, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET agent_owner_id = $2,
    agent_owner_level = CASE WHEN $2::bigint IS NULL THEN NULL ELSE $3::smallint END,
    agent_upline_id = NULL,
    agent_owner_bound_at = CASE WHEN $2::bigint IS NULL THEN NULL ELSE NOW() END,
    agent_owner_set_by = $4,
    rebate_settled_at = CASE WHEN inviter_id IS NULL AND agent_owner_id IS NULL THEN NOW() ELSE rebate_settled_at END,
    updated_at = NOW()
WHERE user_id = $1`, userID, nullableInt64Arg(ownerID), service.AffiliateAgentLevelTop, adminID)
	if err != nil {
		return fmt.Errorf("set agent owner by admin: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read set agent owner result: %w", err)
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

// CountAgentAppointments 统计占用某一级代理任命名额的人数：由他任命、仍为二级、仍归属他且账号
// 未删除，即他还能取消的二级。被管理员改挂走或账号已删除的二级他取消不了，不再占名额。
func (r *affiliateRepository) CountAgentAppointments(ctx context.Context, appointerID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	n, err := scanInt64(ctx, client, `
SELECT COUNT(*)
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id AND u.deleted_at IS NULL
WHERE ua.agent_level = 2
  AND ua.agent_level_source = $2
  AND ua.agent_level_updated_by = $1
  AND ua.agent_owner_id = $1`, appointerID, service.AffiliateAgentLevelSourceAgent)
	return int(n), err
}
