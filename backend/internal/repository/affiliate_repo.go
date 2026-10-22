package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const (
	affiliateCodeLength      = 12
	affiliateCodeMaxAttempts = 12
)

var affiliateCodeCharset = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

// affiliateAgentRefColumns / affiliateAgentRefJoins 读取 ua 行的归属代理与上方一级快照，
// 账号已删除（不存在或软删除）时 *_deleted 为 true。
const affiliateAgentRefColumns = `
       ua.agent_owner_id,
       COALESCE(owner_u.email, ''),
       ua.agent_owner_level,
       (ua.agent_owner_id IS NOT NULL AND (owner_u.id IS NULL OR owner_u.deleted_at IS NOT NULL)),
       ua.agent_upline_id,
       COALESCE(upline_u.email, ''),
       (ua.agent_upline_id IS NOT NULL AND (upline_u.id IS NULL OR upline_u.deleted_at IS NOT NULL))`

const affiliateAgentRefJoins = `
LEFT JOIN users owner_u ON owner_u.id = ua.agent_owner_id
LEFT JOIN users upline_u ON upline_u.id = ua.agent_upline_id`

// agentRefScan 承接 affiliateAgentRefColumns 的扫描结果。
type agentRefScan struct {
	ownerID, ownerLevel, uplineID sql.NullInt64
}

func (a *agentRefScan) targets(ref *service.AffiliateAgentRef) []any {
	return []any{&a.ownerID, &ref.AgentOwnerEmail, &a.ownerLevel, &ref.AgentOwnerDeleted, &a.uplineID, &ref.AgentUplineEmail, &ref.AgentUplineDeleted}
}

func (a *agentRefScan) apply(ref *service.AffiliateAgentRef) {
	ref.AgentOwnerID = nullableInt64Ptr(a.ownerID)
	ref.AgentUplineID = nullableInt64Ptr(a.uplineID)
	if a.ownerLevel.Valid {
		level := int(a.ownerLevel.Int64)
		ref.AgentOwnerLevel = &level
	}
}

const affiliateUserOverviewSQL = `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       ua.aff_rebate_rate_percent::double precision,
       ua.aff_count,
       COALESCE(rebated.rebated_invitee_count, 0),
       (ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0))::double precision,
       ua.aff_history_quota::double precision,
       ua.agent_level,
       ua.agent_level_locked,
       ua.agent_level_source,` + affiliateAgentRefColumns + `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id` + affiliateAgentRefJoins + `
LEFT JOIN (
    SELECT user_id, COUNT(DISTINCT source_user_id)::integer AS rebated_invitee_count
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND source_user_id IS NOT NULL
    GROUP BY user_id
) rebated ON rebated.user_id = ua.user_id
LEFT JOIN (
    SELECT user_id, COALESCE(SUM(amount), 0)::double precision AS matured_frozen_quota
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND frozen_until IS NOT NULL AND frozen_until <= NOW()
    GROUP BY user_id
) matured ON matured.user_id = ua.user_id
WHERE ua.user_id = $1
LIMIT 1`

type affiliateQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type affiliateRepository struct {
	client *dbent.Client
}

func NewAffiliateRepository(client *dbent.Client, _ *sql.DB) service.AffiliateRepository {
	return &affiliateRepository{client: client}
}

func (r *affiliateRepository) EnsureUserAffiliate(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	return ensureUserAffiliateWithClient(ctx, client, userID)
}

func (r *affiliateRepository) GetAffiliateByCode(ctx context.Context, code string) (*service.AffiliateSummary, error) {
	client := clientFromContext(ctx, r.client)
	return queryAffiliateByCode(ctx, client, code)
}

func (r *affiliateRepository) BindInviter(ctx context.Context, userID, inviterID int64, binding service.AffiliateInviterBinding) (*service.AffiliateInviterBindingResult, bool, error) {
	var result *service.AffiliateInviterBindingResult
	var bound bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, inviterID); err != nil {
			return err
		}

		rows, err := txClient.QueryContext(txCtx, `
UPDATE user_affiliates
SET inviter_id = $1,
    inviter_bound_at = NOW(),
    inviter_bound_source = $2,
    inviter_bound_by = $3,
    inviter_bind_reason = $4,
    agent_owner_id = $6,
    agent_owner_level = $7,
    agent_upline_id = $8,
    agent_owner_bound_at = CASE WHEN $6::bigint IS NULL THEN NULL ELSE NOW() END,
    rebate_settled_at = NOW(),
    updated_at = NOW()
WHERE user_id = $5
  AND inviter_id IS NULL
  AND inviter_bound_at IS NULL
RETURNING inviter_bound_at`, inviterID, binding.Source, binding.BoundBy, binding.Reason, userID,
			nullableInt64Arg(binding.Owner.OwnerID), nullableIntArg(binding.Owner.OwnerLevel), nullableInt64Arg(binding.Owner.UplineID))
		if err != nil {
			return fmt.Errorf("bind inviter: %w", err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return fmt.Errorf("bind inviter: %w", err)
			}
			return nil
		}
		var boundAt time.Time
		if err := rows.Scan(&boundAt); err != nil {
			return fmt.Errorf("scan inviter binding: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close inviter binding rows: %w", err)
		}

		res, err := txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET aff_count = aff_count + 1, updated_at = NOW() WHERE user_id = $1",
			inviterID,
		)
		if err != nil {
			return fmt.Errorf("increment inviter count: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("read inviter count result: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("increment inviter count affected %d rows", affected)
		}
		bound = true
		result = &service.AffiliateInviterBindingResult{InviteeID: userID, InviterID: inviterID, BoundAt: boundAt, Source: binding.Source}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, bound, nil
}

// AccrueQuota 先写流水再加额度：首充类奖励命中唯一部分索引（ON CONFLICT DO NOTHING）
// 时不加额度并返回 false；受益人没有 user_affiliates 行时同样返回 false。
func (r *affiliateRepository) AccrueQuota(ctx context.Context, beneficiaryID, sourceUserID int64, amount float64, freezeHours int, source service.AffiliateAccrueSource, rewardType string) (bool, error) {
	if amount <= 0 {
		return false, nil
	}

	var applied bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if freezeHours < 0 {
			freezeHours = 0
		}
		rows, err := txClient.QueryContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, source_usage_until, reward_type, frozen_until, created_at, updated_at)
SELECT $1, 'accrue', $2, $3, $4, $5, $6,
       CASE WHEN $7::int > 0 THEN NOW() + make_interval(hours => $7::int) ELSE NULL END,
       NOW(), NOW()
WHERE EXISTS (SELECT 1 FROM user_affiliates WHERE user_id = $1)
ON CONFLICT (source_user_id, reward_type) WHERE reward_type IN ('invite_fixed', 'invitee_bonus') DO NOTHING
RETURNING id`, beneficiaryID, amount, sourceUserID, nullableInt64Arg(source.OrderID), nullableTimeArg(source.UsageUntil), nullableStringArg(rewardType), freezeHours)
		if err != nil {
			return fmt.Errorf("insert affiliate accrue ledger: %w", err)
		}
		inserted := rows.Next()
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("insert affiliate accrue ledger: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close affiliate accrue ledger rows: %w", err)
		}
		if !inserted {
			return nil
		}

		// freezeHours > 0: add to frozen quota; == 0: add to available quota directly
		updateSQL := "UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		if freezeHours > 0 {
			updateSQL = "UPDATE user_affiliates SET aff_frozen_quota = aff_frozen_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		}
		if _, err := txClient.ExecContext(txCtx, updateSQL, amount, beneficiaryID); err != nil {
			return fmt.Errorf("accrue affiliate quota: %w", err)
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (r *affiliateRepository) GetAccruedRebateFromInvitee(ctx context.Context, beneficiaryID, sourceUserID int64, rewardTypes []string) (float64, error) {
	client := clientFromContext(ctx, r.client)
	return scanFloat64(ctx, client, `
SELECT COALESCE(SUM(amount), 0)::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND source_user_id = $2 AND action = 'accrue' AND reward_type = ANY($3)`,
		beneficiaryID, sourceUserID, pq.Array(rewardTypes))
}

// CreditInviteeBonus 写被邀请人首充奖励流水并加到用户余额；唯一索引冲突表示已发过。
func (r *affiliateRepository) CreditInviteeBonus(ctx context.Context, userID int64, amount float64, sourceOrderID *int64) (bool, error) {
	if amount <= 0 {
		return false, nil
	}
	var applied bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rows, err := txClient.QueryContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, reward_type, created_at, updated_at)
SELECT $1, 'invitee_bonus', $2, $1, $3, 'invitee_bonus', NOW(), NOW()
WHERE EXISTS (SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)
ON CONFLICT (source_user_id, reward_type) WHERE reward_type IN ('invite_fixed', 'invitee_bonus') DO NOTHING
RETURNING id`, userID, amount, nullableInt64Arg(sourceOrderID))
		if err != nil {
			return fmt.Errorf("insert invitee bonus ledger: %w", err)
		}
		inserted := rows.Next()
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("insert invitee bonus ledger: %w", err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close invitee bonus ledger rows: %w", err)
		}
		if !inserted {
			return nil
		}
		affected, err := txClient.User.Update().Where(user.IDEQ(userID)).AddBalance(amount).Save(txCtx)
		if err != nil {
			return fmt.Errorf("credit invitee bonus balance: %w", err)
		}
		if affected == 0 {
			return service.ErrUserNotFound
		}
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return applied, nil
}

func (r *affiliateRepository) HasRewardForSource(ctx context.Context, sourceUserID int64, rewardType string) (bool, error) {
	client := clientFromContext(ctx, r.client)
	n, err := scanInt64(ctx, client, `
SELECT COUNT(*) FROM user_affiliate_ledger WHERE source_user_id = $1 AND reward_type = $2`, sourceUserID, rewardType)
	return n > 0, err
}

func (r *affiliateRepository) CountOwnedCustomers(ctx context.Context, agentID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	n, err := scanInt64(ctx, client, `SELECT COUNT(*) FROM user_affiliates WHERE agent_owner_id = $1`, agentID)
	return int(n), err
}

func (r *affiliateRepository) SumRewardTotals(ctx context.Context, beneficiaryID int64) (service.AffiliateRewardTotals, error) {
	var totals service.AffiliateRewardTotals
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(amount) FILTER (WHERE reward_type IN ('agent_direct', 'agent_diff')), 0)::double precision,
       COALESCE(SUM(amount) FILTER (WHERE reward_type IN ('invite_percent', 'invite_fixed')), 0)::double precision,
       COALESCE(SUM(amount) FILTER (WHERE reward_type = 'invitee_bonus'), 0)::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action IN ('accrue', 'invitee_bonus')`, beneficiaryID)
	if err != nil {
		return totals, fmt.Errorf("sum affiliate reward totals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&totals.Agent, &totals.Invite, &totals.InviteeBonus); err != nil {
			return totals, err
		}
	}
	return totals, rows.Err()
}

// GetAffiliateNodes 批量读取用户的等级、快照、专属比例与账号状态。
func (r *affiliateRepository) GetAffiliateNodes(ctx context.Context, userIDs []int64) (map[int64]service.AffiliateNode, error) {
	out := make(map[int64]service.AffiliateNode, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       ua.agent_level,
       ua.agent_owner_id,
       ua.inviter_id,
       ua.aff_rebate_rate_percent::double precision,
       (u.id IS NOT NULL AND u.deleted_at IS NULL) AS active,
       COALESCE(u.email, '')
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
WHERE ua.user_id = ANY($1)`, pq.Array(userIDs))
	if err != nil {
		return nil, fmt.Errorf("query affiliate nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var node service.AffiliateNode
		var ownerID, inviterID sql.NullInt64
		var customRate sql.NullFloat64
		if err := rows.Scan(&node.UserID, &node.AgentLevel, &ownerID, &inviterID, &customRate, &node.Active, &node.Email); err != nil {
			return nil, err
		}
		node.AgentOwnerID = nullableInt64Ptr(ownerID)
		node.InviterID = nullableInt64Ptr(inviterID)
		node.CustomRate = nullableFloat64Ptr(customRate)
		out[node.UserID] = node
	}
	return out, rows.Err()
}

func (r *affiliateRepository) ThawFrozenQuota(ctx context.Context, userID int64) (float64, error) {
	var thawed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var err error
		thawed, err = thawFrozenQuotaTx(txCtx, txClient, userID)
		return err
	})
	return thawed, err
}

// thawFrozenQuotaTx moves matured frozen quota to available quota within an existing tx.
func thawFrozenQuotaTx(txCtx context.Context, txClient *dbent.Client, userID int64) (float64, error) {
	rows, err := txClient.QueryContext(txCtx, `
WITH matured AS (
    UPDATE user_affiliate_ledger
    SET frozen_until = NULL, updated_at = NOW()
    WHERE user_id = $1
      AND frozen_until IS NOT NULL
      AND frozen_until <= NOW()
    RETURNING amount
)
SELECT COALESCE(SUM(amount), 0) FROM matured`, userID)
	if err != nil {
		return 0, fmt.Errorf("thaw frozen quota: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var thawed float64
	if rows.Next() {
		if err := rows.Scan(&thawed); err != nil {
			return 0, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if thawed <= 0 {
		return 0, nil
	}

	_, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_frozen_quota = GREATEST(aff_frozen_quota - $1, 0),
    updated_at = NOW()
WHERE user_id = $2`, thawed, userID)
	if err != nil {
		return 0, fmt.Errorf("move thawed quota: %w", err)
	}
	return thawed, nil
}

func (r *affiliateRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var transferred float64
	var newBalance float64

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		// Thaw any matured frozen quota before transfer.
		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw before transfer: %w", err)
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH claimed AS (
	SELECT aff_quota::double precision AS amount
	FROM user_affiliates
	WHERE user_id = $1
	  AND aff_quota > 0
	FOR UPDATE
),
cleared AS (
	UPDATE user_affiliates ua
	SET aff_quota = 0,
	    updated_at = NOW()
	FROM claimed c
	WHERE ua.user_id = $1
	RETURNING c.amount
)
SELECT amount
FROM cleared`, userID)
		if err != nil {
			return fmt.Errorf("claim affiliate quota: %w", err)
		}

		if !rows.Next() {
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			return service.ErrAffiliateQuotaEmpty
		}
		if err := rows.Scan(&transferred); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if transferred <= 0 {
			return service.ErrAffiliateQuotaEmpty
		}

		affected, err := txClient.User.Update().
			Where(user.IDEQ(userID)).
			AddBalance(transferred).
			AddTotalRecharged(transferred).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("credit user balance by affiliate quota: %w", err)
		}
		if affected == 0 {
			return service.ErrUserNotFound
		}

		newBalance, err = queryUserBalance(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		snapshot, err := queryAffiliateTransferSnapshot(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (
    user_id,
    action,
    amount,
    source_user_id,
    balance_after,
    aff_quota_after,
    aff_frozen_quota_after,
    aff_history_quota_after,
    created_at,
    updated_at
)
VALUES ($1, 'transfer', $2, NULL, $3, $4, $5, $6, NOW(), NOW())`,
			userID,
			transferred,
			snapshot.BalanceAfter,
			snapshot.AvailableQuotaAfter,
			snapshot.FrozenQuotaAfter,
			snapshot.HistoryQuotaAfter,
		); err != nil {
			return fmt.Errorf("insert affiliate transfer ledger: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return transferred, newBalance, nil
}

// WithdrawQuota 登记一笔线下提现。operationID 标识一次登记：同一事务内先以
// operation_id 唯一约束写入占位流水，同标识的流水已存在时不重复扣减，用户与金额
// 一致则返回该流水记录的结果（Replayed=true），不一致返回
// ErrIdempotencyKeyConflict。占位成功后先解冻已到期的冻结额度，再以
// aff_quota >= amount 为条件原子扣减并补写额度快照。可提取额度不足或用户没有
// 返利档案时返回 ErrAffiliateQuotaInsufficient，占位随事务回滚，不占用该标识。
func (r *affiliateRepository) WithdrawQuota(ctx context.Context, userID int64, amount float64, operationID string) (*service.AffiliateWithdrawResult, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	if amount <= 0 {
		return nil, service.ErrAffiliateWithdrawAmountInvalid
	}
	if operationID == "" {
		return nil, service.ErrIdempotencyKeyRequired
	}

	var result *service.AffiliateWithdrawResult
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		ledgerID, claimed, err := claimAffiliateWithdrawLedger(txCtx, txClient, userID, amount, operationID)
		if err != nil {
			return err
		}
		if !claimed {
			result, err = findAffiliateWithdrawByOperation(txCtx, txClient, userID, amount, operationID)
			return err
		}

		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw before withdraw: %w", err)
		}

		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota - $1,
    updated_at = NOW()
WHERE user_id = $2
  AND aff_quota >= $1`, amount, userID)
		if err != nil {
			return fmt.Errorf("deduct affiliate quota: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("deduct affiliate quota: %w", err)
		}
		if affected == 0 {
			return service.ErrAffiliateQuotaInsufficient
		}

		snapshot, err := queryAffiliateTransferSnapshot(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliate_ledger
SET balance_after = $1,
    aff_quota_after = $2,
    aff_frozen_quota_after = $3,
    aff_history_quota_after = $4,
    updated_at = NOW()
WHERE id = $5`,
			snapshot.BalanceAfter,
			snapshot.AvailableQuotaAfter,
			snapshot.FrozenQuotaAfter,
			snapshot.HistoryQuotaAfter,
			ledgerID,
		); err != nil {
			return fmt.Errorf("record affiliate withdraw snapshot: %w", err)
		}

		result = &service.AffiliateWithdrawResult{
			LedgerID:            ledgerID,
			UserID:              userID,
			Amount:              amount,
			AvailableQuotaAfter: snapshot.AvailableQuotaAfter,
			FrozenQuotaAfter:    snapshot.FrozenQuotaAfter,
			HistoryQuotaAfter:   snapshot.HistoryQuotaAfter,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// claimAffiliateWithdrawLedger 为有返利档案的用户写入一条带 operation_id 的
// withdraw 占位流水。同标识的流水已存在或用户没有返利档案时 claimed 为 false。
// 同标识的并发事务在唯一索引上等待先写入者结束：先写入者提交后返回 claimed=false，
// 回滚后本事务继续写入。
func claimAffiliateWithdrawLedger(ctx context.Context, client affiliateQueryExecer, userID int64, amount float64, operationID string) (int64, bool, error) {
	rows, err := client.QueryContext(ctx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, operation_id, created_at, updated_at)
SELECT ua.user_id, 'withdraw', $2, $3, NOW(), NOW()
FROM user_affiliates ua
WHERE ua.user_id = $1
ON CONFLICT (operation_id) WHERE operation_id IS NOT NULL DO NOTHING
RETURNING id`, userID, amount, operationID)
	if err != nil {
		return 0, false, fmt.Errorf("claim affiliate withdraw ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, false, fmt.Errorf("claim affiliate withdraw ledger: %w", err)
		}
		return 0, false, nil
	}
	var ledgerID int64
	if err := rows.Scan(&ledgerID); err != nil {
		return 0, false, err
	}
	return ledgerID, true, rows.Close()
}

// findAffiliateWithdrawByOperation 读取 operation_id 已对应的线下提现流水，
// 用户与金额一致时返回该流水记录的登记结果（Replayed=true），不一致返回
// ErrIdempotencyKeyConflict；没有对应流水说明用户没有返利档案，返回
// ErrAffiliateQuotaInsufficient。
func findAffiliateWithdrawByOperation(ctx context.Context, client affiliateQueryExecer, userID int64, amount float64, operationID string) (*service.AffiliateWithdrawResult, error) {
	rows, err := client.QueryContext(ctx, `
SELECT id,
       user_id = $2 AND action = 'withdraw' AND amount = CAST($3 AS DECIMAL(20,8)),
       user_id,
       amount::double precision,
       COALESCE(aff_quota_after, 0)::double precision,
       COALESCE(aff_frozen_quota_after, 0)::double precision,
       COALESCE(aff_history_quota_after, 0)::double precision
FROM user_affiliate_ledger
WHERE operation_id = $1`, operationID, userID, amount)
	if err != nil {
		return nil, fmt.Errorf("query affiliate withdraw by operation: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query affiliate withdraw by operation: %w", err)
		}
		return nil, service.ErrAffiliateQuotaInsufficient
	}
	var sameRequest bool
	result := &service.AffiliateWithdrawResult{Replayed: true}
	if err := rows.Scan(
		&result.LedgerID,
		&sameRequest,
		&result.UserID,
		&result.Amount,
		&result.AvailableQuotaAfter,
		&result.FrozenQuotaAfter,
		&result.HistoryQuotaAfter,
	); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if !sameRequest {
		return nil, service.ErrIdempotencyKeyConflict
	}
	return result, nil
}

func (r *affiliateRepository) ListInvitees(ctx context.Context, inviterID int64, limit int) ([]service.AffiliateInvitee, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       COALESCE(ua.inviter_bound_at, ua.created_at),
       COALESCE((
           SELECT SUM(ual.amount)
           FROM user_affiliate_ledger ual
           WHERE ual.user_id = $1
             AND ual.source_user_id = ua.user_id
             AND ual.action = 'accrue'
       ), 0)::double precision AS total_rebate,
       ua.agent_level,
       ua.agent_owner_id,
       ua.agent_level_source,
       ua.agent_level_locked,
       ua.agent_level_updated_by
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
WHERE ua.inviter_id = $1
ORDER BY COALESCE(ua.inviter_bound_at, ua.created_at) DESC
LIMIT $2`, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	invitees := make([]service.AffiliateInvitee, 0)
	for rows.Next() {
		var item service.AffiliateInvitee
		var createdAt time.Time
		var ownerID, updatedBy sql.NullInt64
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &createdAt, &item.TotalRebate,
			&item.AgentLevel, &ownerID, &item.AgentLevelSource, &item.AgentLevelLocked, &updatedBy); err != nil {
			return nil, err
		}
		item.CreatedAt = &createdAt
		item.AgentOwnerID = nullableInt64Ptr(ownerID)
		item.AgentLevelUpdatedBy = nullableInt64Ptr(updatedBy)
		invitees = append(invitees, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invitees, nil
}

func (r *affiliateRepository) ListAffiliateInviteRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateInviteRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "COALESCE(ua.inviter_bound_at, ua.created_at)", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"ua.inviter_id::text", "ua.user_id::text", "inviter_aff.aff_code",
	})

	total, err := queryAffiliateRecordCount(ctx, client, `
SELECT COUNT(*)
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id
`+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"inviter":      "inviter.email",
		"invitee":      "invitee.email",
		"aff_code":     "inviter_aff.aff_code",
		"total_rebate": "total_rebate",
		"created_at":   "COALESCE(ua.inviter_bound_at, ua.created_at)",
	}, "COALESCE(ua.inviter_bound_at, ua.created_at)")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ua.inviter_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ua.user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       COALESCE(inviter_aff.aff_code, ''),
       COALESCE((
           SELECT SUM(ual.amount)
           FROM user_affiliate_ledger ual
           WHERE ual.user_id = ua.inviter_id
             AND ual.source_user_id = ua.user_id
             AND ual.action = 'accrue'
       ), 0)::double precision AS total_rebate,
       COALESCE(ua.inviter_bound_at, ua.created_at),`+affiliateAgentRefColumns+`
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id`+affiliateAgentRefJoins+`
`+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateInviteRecord, 0)
	for rows.Next() {
		var item service.AffiliateInviteRecord
		var ref agentRefScan
		targets := append([]any{
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.InviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&item.AffCode,
			&item.TotalRebate,
			&item.CreatedAt,
		}, ref.targets(&item.AffiliateAgentRef)...)
		if err := rows.Scan(targets...); err != nil {
			return nil, 0, err
		}
		ref.apply(&item.AffiliateAgentRef)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateRebateRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateRebateRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"po.id::text", "po.out_trade_no", "po.payment_type", "po.status",
	})
	// 返利记录列出全部奖励流水（accrue 与被邀请人首充奖励）：非订单来源的流水没有订单，
	// 被邀请人账号删除后 source_user_id 为 NULL，这两类行都保留。
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN users inviter ON inviter.id = ual.user_id
LEFT JOIN users invitee ON invitee.id = ual.source_user_id
LEFT JOIN payment_orders po ON po.id = ual.source_order_id
WHERE ual.action IN ('accrue', 'invitee_bonus')`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}
	if filter.RewardType != "" {
		args = append(args, filter.RewardType)
		where += fmt.Sprintf(" AND ual.reward_type = $%d", len(args))
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"order":         "po.id",
		"inviter":       "inviter.email",
		"invitee":       "invitee.email",
		"order_amount":  "po.amount",
		"pay_amount":    "po.pay_amount",
		"rebate_amount": "ual.amount",
		"reward_type":   "ual.reward_type",
		"payment_type":  "po.payment_type",
		"order_status":  "po.status",
		"created_at":    "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT po.id,
       COALESCE(po.out_trade_no, ''),
       ual.user_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ual.source_user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       po.amount::double precision,
       po.pay_amount::double precision,
       ual.amount::double precision,
       ual.reward_type,
       COALESCE(po.payment_type, ''),
       COALESCE(po.status, ''),
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateRebateRecord, 0)
	for rows.Next() {
		var item service.AffiliateRebateRecord
		var orderID sql.NullInt64
		var inviteeID sql.NullInt64
		var orderAmount sql.NullFloat64
		var payAmount sql.NullFloat64
		var rewardType sql.NullString
		if err := rows.Scan(
			&orderID,
			&item.OutTradeNo,
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&inviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&orderAmount,
			&payAmount,
			&item.RebateAmount,
			&rewardType,
			&item.PaymentType,
			&item.OrderStatus,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.OrderID = nullableInt64Ptr(orderID)
		item.InviteeID = nullableInt64Ptr(inviteeID)
		if rewardType.Valid {
			v := rewardType.String
			item.RewardType = &v
		}
		item.OrderAmount = nullableFloat64Ptr(orderAmount)
		item.PayAmount = nullableFloat64Ptr(payAmount)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateTransferRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateTransferRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"u.email", "u.username", "u.id::text",
	})
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN users u ON u.id = ual.user_id
WHERE ual.action IN ('transfer', 'withdraw')`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"user":                  "u.email",
		"action":                "ual.action",
		"amount":                "ual.amount",
		"balance_after":         "ual.balance_after",
		"available_quota_after": "ual.aff_quota_after",
		"frozen_quota_after":    "ual.aff_frozen_quota_after",
		"history_quota_after":   "ual.aff_history_quota_after",
		"created_at":            "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ual.id,
       ual.action,
       ual.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ual.amount::double precision,
       ual.balance_after::double precision,
       ual.aff_quota_after::double precision,
       ual.aff_frozen_quota_after::double precision,
       ual.aff_history_quota_after::double precision,
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateTransferRecord, 0)
	for rows.Next() {
		var item service.AffiliateTransferRecord
		var balanceAfter sql.NullFloat64
		var availableQuotaAfter sql.NullFloat64
		var frozenQuotaAfter sql.NullFloat64
		var historyQuotaAfter sql.NullFloat64
		if err := rows.Scan(
			&item.LedgerID,
			&item.Action,
			&item.UserID,
			&item.UserEmail,
			&item.Username,
			&item.Amount,
			&balanceAfter,
			&availableQuotaAfter,
			&frozenQuotaAfter,
			&historyQuotaAfter,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.BalanceAfter = nullableFloat64Ptr(balanceAfter)
		item.AvailableQuotaAfter = nullableFloat64Ptr(availableQuotaAfter)
		item.FrozenQuotaAfter = nullableFloat64Ptr(frozenQuotaAfter)
		item.HistoryQuotaAfter = nullableFloat64Ptr(historyQuotaAfter)
		item.SnapshotAvailable = balanceAfter.Valid &&
			availableQuotaAfter.Valid &&
			frozenQuotaAfter.Valid &&
			historyQuotaAfter.Valid
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) GetAffiliateUserOverview(ctx context.Context, userID int64) (*service.AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, affiliateUserOverviewSQL, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var overview service.AffiliateUserOverview
	var customRate sql.NullFloat64
	var ref agentRefScan
	targets := append([]any{
		&overview.UserID,
		&overview.Email,
		&overview.Username,
		&overview.AffCode,
		&customRate,
		&overview.InvitedCount,
		&overview.RebatedInviteeCount,
		&overview.AvailableQuota,
		&overview.HistoryQuota,
		&overview.AgentLevel,
		&overview.AgentLevelLocked,
		&overview.AgentLevelSource,
	}, ref.targets(&overview.AffiliateAgentRef)...)
	if err := rows.Scan(targets...); err != nil {
		return nil, err
	}
	ref.apply(&overview.AffiliateAgentRef)
	overview.CustomRate = nullableFloat64Ptr(customRate)
	return &overview, rows.Err()
}

func buildAffiliateRecordWhere(filter service.AffiliateRecordFilter, timeColumn string, searchColumns []string) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.StartAt != nil {
		args = append(args, *filter.StartAt)
		clauses = append(clauses, fmt.Sprintf("%s >= $%d", timeColumn, len(args)))
	}
	if filter.EndAt != nil {
		args = append(args, *filter.EndAt)
		clauses = append(clauses, fmt.Sprintf("%s <= $%d", timeColumn, len(args)))
	}
	search := strings.TrimSpace(filter.Search)
	if search != "" && len(searchColumns) > 0 {
		args = append(args, "%"+strings.ToLower(search)+"%")
		parts := make([]string, 0, len(searchColumns))
		for _, col := range searchColumns {
			parts = append(parts, fmt.Sprintf("LOWER(%s) LIKE $%d", col, len(args)))
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildAffiliateRecordOrderBy(filter service.AffiliateRecordFilter, sortColumns map[string]string, fallbackColumn string) string {
	column := sortColumns[filter.SortBy]
	if column == "" {
		column = fallbackColumn
	}
	direction := "DESC"
	if !filter.SortDesc {
		direction = "ASC"
	}
	return "ORDER BY " + column + " " + direction + " NULLS LAST"
}

func queryAffiliateRecordCount(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total int64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func (r *affiliateRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin affiliate transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit affiliate transaction: %w", err)
	}
	return nil
}

func ensureUserAffiliateWithClient(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, "SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL", userID)
	if err != nil {
		return nil, fmt.Errorf("check affiliate user existence: %w", err)
	}
	userExists := rows.Next()
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("check affiliate user existence: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close affiliate user existence rows: %w", err)
	}
	if !userExists {
		return nil, service.ErrUserNotFound
	}

	summary, err := queryAffiliateByUserID(ctx, client, userID)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, service.ErrAffiliateProfileNotFound) {
		return nil, err
	}

	for i := 0; i < affiliateCodeMaxAttempts; i++ {
		code, codeErr := generateAffiliateCode()
		if codeErr != nil {
			return nil, codeErr
		}
		_, insertErr := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING`, userID, code)
		if insertErr == nil {
			break
		}
		if isAffiliateUniqueViolation(insertErr) {
			continue
		}
		return nil, insertErr
	}

	return queryAffiliateByUserID(ctx, client, userID)
}

const affiliateSummaryColumns = `
       user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       inviter_bound_at,
       inviter_bound_source,
       inviter_bound_by,
       inviter_bind_reason,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       agent_level,
       agent_level_locked,
       agent_level_source,
       agent_level_updated_at,
       agent_level_updated_by,
       agent_owner_id,
       agent_owner_level,
       agent_upline_id,
       agent_owner_bound_at,
       rebate_settled_at,
       rebate_settled_consumption::double precision,
       created_at,
       updated_at`

func queryAffiliateByUserID(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	return queryAffiliateSummary(ctx, client, "SELECT"+affiliateSummaryColumns+" FROM user_affiliates WHERE user_id = $1 LIMIT 1", userID)
}

func queryAffiliateByCode(ctx context.Context, client affiliateQueryExecer, code string) (*service.AffiliateSummary, error) {
	return queryAffiliateSummary(ctx, client, "SELECT"+affiliateSummaryColumns+" FROM user_affiliates WHERE aff_code = $1 LIMIT 1", strings.ToUpper(strings.TrimSpace(code)))
}

func queryAffiliateSummary(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}
	return scanAffiliateSummary(rows)
}

func scanAffiliateSummary(rows *sql.Rows) (*service.AffiliateSummary, error) {
	var out service.AffiliateSummary
	var inviterID, inviterBoundBy sql.NullInt64
	var inviterBoundAt, agentLevelUpdatedAt sql.NullTime
	var agentLevelUpdatedBy, agentOwnerID, agentOwnerLevel, agentUplineID sql.NullInt64
	var agentOwnerBoundAt sql.NullTime
	var inviterBoundSource, inviterBindReason sql.NullString
	var rebateRate sql.NullFloat64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&inviterBoundAt,
		&inviterBoundSource,
		&inviterBoundBy,
		&inviterBindReason,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.AgentLevel,
		&out.AgentLevelLocked,
		&out.AgentLevelSource,
		&agentLevelUpdatedAt,
		&agentLevelUpdatedBy,
		&agentOwnerID,
		&agentOwnerLevel,
		&agentUplineID,
		&agentOwnerBoundAt,
		&out.RebateSettledAt,
		&out.RebateSettledConsumption,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if inviterBoundAt.Valid {
		out.InviterBoundAt = &inviterBoundAt.Time
	}
	if inviterBoundSource.Valid {
		out.InviterBoundSource = &inviterBoundSource.String
	}
	if inviterBoundBy.Valid {
		out.InviterBoundBy = &inviterBoundBy.Int64
	}
	if inviterBindReason.Valid {
		out.InviterBindReason = &inviterBindReason.String
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	if agentLevelUpdatedAt.Valid {
		out.AgentLevelUpdatedAt = &agentLevelUpdatedAt.Time
	}
	out.AgentLevelUpdatedBy = nullableInt64Ptr(agentLevelUpdatedBy)
	out.AgentOwnerID = nullableInt64Ptr(agentOwnerID)
	out.AgentUplineID = nullableInt64Ptr(agentUplineID)
	if agentOwnerLevel.Valid {
		level := int(agentOwnerLevel.Int64)
		out.AgentOwnerLevel = &level
	}
	if agentOwnerBoundAt.Valid {
		out.AgentOwnerBoundAt = &agentOwnerBoundAt.Time
	}
	return &out, nil
}

func queryUserBalance(ctx context.Context, client affiliateQueryExecer, userID int64) (float64, error) {
	rows, err := client.QueryContext(ctx,
		"SELECT balance::double precision FROM users WHERE id = $1 LIMIT 1",
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrUserNotFound
	}
	var balance float64
	if err := rows.Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

type affiliateTransferSnapshot struct {
	BalanceAfter        float64
	AvailableQuotaAfter float64
	FrozenQuotaAfter    float64
	HistoryQuotaAfter   float64
}

func queryAffiliateTransferSnapshot(ctx context.Context, client affiliateQueryExecer, userID int64) (*affiliateTransferSnapshot, error) {
	rows, err := client.QueryContext(ctx, `
SELECT u.balance::double precision,
       ua.aff_quota::double precision,
       ua.aff_frozen_quota::double precision,
       ua.aff_history_quota::double precision
FROM users u
JOIN user_affiliates ua ON ua.user_id = u.id
WHERE u.id = $1
LIMIT 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query affiliate transfer snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var snapshot affiliateTransferSnapshot
	if err := rows.Scan(
		&snapshot.BalanceAfter,
		&snapshot.AvailableQuotaAfter,
		&snapshot.FrozenQuotaAfter,
		&snapshot.HistoryQuotaAfter,
	); err != nil {
		return nil, err
	}
	return &snapshot, rows.Err()
}

func nullableFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullableInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func generateAffiliateCode() (string, error) {
	buf := make([]byte, affiliateCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate affiliate code: %w", err)
	}
	for i := range buf {
		buf[i] = affiliateCodeCharset[int(buf[i])%len(affiliateCodeCharset)]
	}
	return string(buf), nil
}

func isAffiliateUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}
	return false
}

// UpdateUserAffCode 改写用户的邀请码（自定义专属邀请码）。
// 唯一性冲突返回 ErrAffiliateCodeTaken。
func (r *affiliateRepository) UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	code := strings.ToUpper(strings.TrimSpace(newCode))
	if code == "" {
		return service.ErrAffiliateCodeInvalid
	}

	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = true,
    updated_at = NOW()
WHERE user_id = $2`, code, userID)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateCodeTaken
			}
			return fmt.Errorf("update aff_code: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// ResetUserAffCode 把 aff_code 还原为系统随机码，并清除 aff_code_custom 标记。
func (r *affiliateRepository) ResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", service.ErrUserNotFound
	}
	var newCode string
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		for i := 0; i < affiliateCodeMaxAttempts; i++ {
			candidate, codeErr := generateAffiliateCode()
			if codeErr != nil {
				return codeErr
			}
			res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = false,
    updated_at = NOW()
WHERE user_id = $2`, candidate, userID)
			if err != nil {
				if isAffiliateUniqueViolation(err) {
					continue
				}
				return fmt.Errorf("reset aff_code: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return service.ErrUserNotFound
			}
			newCode = candidate
			return nil
		}
		return fmt.Errorf("reset aff_code: exhausted attempts")
	})
	if err != nil {
		return "", err
	}
	return newCode, nil
}

// SetUserRebateRate 设置或清除用户专属返利比例。ratePercent==nil 表示清除（沿用全局）。
func (r *affiliateRepository) SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		// nullableArg lets us use a single UPDATE for both "set value" and
		// "clear" cases — database/sql converts nil interface{} to SQL NULL.
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = $2`, nullableArg(ratePercent), userID)
		if err != nil {
			return fmt.Errorf("set aff_rebate_rate_percent: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// BatchSetUserRebateRate 批量为多个用户设置专属比例（nil 清除）。
func (r *affiliateRepository) BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if uid <= 0 {
				continue
			}
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = ANY($2)`, nullableArg(ratePercent), pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("batch set aff_rebate_rate_percent: %w", err)
		}
		return nil
	})
}

// nullableArg unwraps a *float64 into an interface{} suitable for SQL parameter
// binding: nil pointer → SQL NULL, non-nil → the float value.
func nullableArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableIntArg(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableStringArg(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

// scanFloat64 runs a query expected to return a single float64 column (e.g. SUM).
func scanFloat64(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (float64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var v float64
	if rows.Next() {
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
	}
	return v, rows.Err()
}

func nullableInt64Arg(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTimeArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	return *v
}

// ListUsersWithCustomSettings 列出代理管理页的用户。Search、AgentLevel、OwnerID 都为空时
// 只列代理与有专属设置的用户；任一存在时按条件查全部用户。
func (r *affiliateRepository) ListUsersWithCustomSettings(ctx context.Context, filter service.AffiliateAdminFilter) ([]service.AffiliateAdminEntry, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	clauses := []string{"u.deleted_at IS NULL"}
	args := make([]any, 0, 5)
	search := strings.TrimSpace(filter.Search)
	if search == "" && filter.AgentLevel == 0 && filter.OwnerID == 0 {
		clauses = append(clauses, `(ua.aff_code_custom = true
       OR ua.aff_rebate_rate_percent IS NOT NULL
       OR ua.agent_level <> 3
       OR ua.agent_level_locked)`)
	}
	if search != "" {
		args = append(args, "%"+search+"%", search)
		clauses = append(clauses, fmt.Sprintf("(u.email ILIKE $%d OR u.username ILIKE $%d OR ua.user_id::text = $%d)", len(args)-1, len(args)-1, len(args)))
	}
	if filter.AgentLevel > 0 {
		args = append(args, filter.AgentLevel)
		clauses = append(clauses, fmt.Sprintf("ua.agent_level = $%d", len(args)))
	}
	if filter.OwnerID > 0 {
		args = append(args, filter.OwnerID)
		clauses = append(clauses, fmt.Sprintf("ua.agent_owner_id = $%d", len(args)))
	}
	baseFrom := `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
WHERE ` + strings.Join(clauses, " AND ")

	client := clientFromContext(ctx, r.client)
	total, err := scanInt64(ctx, client, "SELECT COUNT(*)"+baseFrom, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate admin entries: %w", err)
	}

	args = append(args, pageSize, offset)
	listQuery := `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       ua.aff_code_custom,
       ua.aff_rebate_rate_percent::double precision,
       ua.aff_count,
       ua.agent_level,
       ua.agent_level_locked,
       ua.agent_level_source,
       ua.agent_level_updated_by,
       COALESCE(updater.email, ''),
       (SELECT COUNT(*) FROM user_affiliates owned WHERE owned.agent_owner_id = ua.user_id)::integer,
       COALESCE((
           SELECT SUM(l.amount)
           FROM user_affiliate_ledger l
           WHERE l.user_id = ua.user_id AND l.reward_type IN ('agent_direct', 'agent_diff')
       ), 0)::double precision,` + affiliateAgentRefColumns + `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
LEFT JOIN users updater ON updater.id = ua.agent_level_updated_by` + affiliateAgentRefJoins + `
WHERE ` + strings.Join(clauses, " AND ") + `
ORDER BY ua.agent_level ASC, ua.updated_at DESC, ua.user_id DESC
LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := client.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate admin entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]service.AffiliateAdminEntry, 0)
	for rows.Next() {
		var e service.AffiliateAdminEntry
		var rebate sql.NullFloat64
		var updatedBy sql.NullInt64
		var ref agentRefScan
		targets := append([]any{&e.UserID, &e.Email, &e.Username, &e.AffCode,
			&e.AffCodeCustom, &rebate, &e.AffCount, &e.AgentLevel, &e.AgentLevelLocked, &e.AgentLevelSource,
			&updatedBy, &e.AgentLevelUpdatedByEmail, &e.OwnedCustomerCount, &e.AgentProfitTotal,
		}, ref.targets(&e.AffiliateAgentRef)...)
		if err := rows.Scan(targets...); err != nil {
			return nil, 0, err
		}
		ref.apply(&e.AffiliateAgentRef)
		e.AffRebateRatePercent = nullableFloat64Ptr(rebate)
		e.AgentLevelUpdatedBy = nullableInt64Ptr(updatedBy)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// ListAgentCustomers 列出归属该代理（owner）或以其为上方一级（upline）的客户及其贡献的代理分润。
func (r *affiliateRepository) ListAgentCustomers(ctx context.Context, agentID int64, page, pageSize int) ([]service.AffiliateAgentCustomer, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	client := clientFromContext(ctx, r.client)
	total, err := scanInt64(ctx, client, `
SELECT COUNT(*) FROM user_affiliates ua WHERE ua.agent_owner_id = $1 OR ua.agent_upline_id = $1`, agentID)
	if err != nil {
		return nil, 0, fmt.Errorf("count agent customers: %w", err)
	}
	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.agent_level,
       CASE WHEN ua.agent_owner_id = $1 THEN 'owner' ELSE 'upline' END,
       ua.agent_owner_bound_at,
       COALESCE((
           SELECT SUM(l.amount)
           FROM user_affiliate_ledger l
           WHERE l.user_id = $1
             AND l.source_user_id = ua.user_id
             AND l.reward_type IN ('agent_direct', 'agent_diff')
       ), 0)::double precision
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
WHERE ua.agent_owner_id = $1 OR ua.agent_upline_id = $1
ORDER BY ua.agent_owner_bound_at DESC NULLS LAST, ua.user_id DESC
LIMIT $2 OFFSET $3`, agentID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list agent customers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateAgentCustomer, 0)
	for rows.Next() {
		var item service.AffiliateAgentCustomer
		var boundAt sql.NullTime
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.AgentLevel, &item.Relation, &boundAt, &item.Contribution); err != nil {
			return nil, 0, err
		}
		if boundAt.Valid {
			item.BoundAt = &boundAt.Time
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// scanInt64 runs a query expected to return a single int64 column (e.g. COUNT).
func scanInt64(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var v int64
	if err := rows.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
