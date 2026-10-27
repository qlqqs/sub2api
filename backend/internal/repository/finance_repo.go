package repository

// CUSTOM(fork): 财务只读快照及自有支出表；金额全程 numeric/decimal，查询值全部参数化。
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type financeRepository struct{ db *sql.DB }

func NewFinanceRepository(db *sql.DB) service.FinanceRepository { return &financeRepository{db: db} }

type financeQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func financeRows(ctx context.Context, q financeQueryer, query string, scan func(*sql.Rows) error, args ...any) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
func financeFixedIDs(ctx context.Context, q financeQueryer) ([]int64, error) {
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.FinanceFixedAccountsKey).Scan(&raw)
	if err == sql.ErrNoRows {
		return []int64{}, nil
	}
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	if err = json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("decode finance settings: %w", err)
	}
	if ids == nil {
		ids = []int64{}
	}
	return ids, nil
}
func (r *financeRepository) Load(ctx context.Context, span service.FinanceRange) (*service.FinanceData, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d := &service.FinanceData{}
	if err = tx.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&d.ComputedAt); err != nil {
		return nil, err
	}
	d.FixedIDs, err = financeFixedIDs(ctx, tx)
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, tx, `SELECT u.id,u.email,u.role,u.deleted_at IS NOT NULL,u.balance,u.frozen_balance,COALESCE(a.aff_quota,0),COALESCE(a.aff_frozen_quota,0) FROM users u LEFT JOIN user_affiliates a ON a.user_id=u.id ORDER BY u.id`, func(rows *sql.Rows) error {
		var u service.FinanceUser
		if e := rows.Scan(&u.ID, &u.Email, &u.Role, &u.Deleted, &u.Balance, &u.Frozen, &u.Available, &u.AffiliateFrozen); e != nil {
			return e
		}
		d.Users = append(d.Users, u)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finance users: %w", err)
	}
	err = financeRows(ctx, tx, `SELECT o.id,o.out_trade_no,o.user_id,o.user_email,o.payment_type,o.status,o.order_type,o.completed_at,
 CASE WHEN o.status IN ('REFUNDED','PARTIALLY_REFUNDED') THEN o.refund_at END,
 o.pay_amount,o.amount-o.bonus_amount,o.bonus_amount,COALESCE(o.channel_fee_rate,p.channel_fee_rate,0),
 `+paymentRefundCashSQL+`,
 CASE WHEN o.status IN ('REFUNDED','PARTIALLY_REFUNDED') AND o.refund_at IS NOT NULL THEN COALESCE(o.refund_balance_deducted,o.refund_amount,0) ELSE 0 END,
 o.channel_fee_rate IS NULL, o.refund_at IS NOT NULL AND o.refund_balance_deducted IS NULL
 FROM payment_orders o LEFT JOIN payment_provider_instances p ON p.id::text=o.provider_instance_id
 WHERE o.completed_at IS NOT NULL AND o.status IN `+paidPaymentOrderStatuses+` ORDER BY o.completed_at,o.id`, func(rows *sql.Rows) error {
		var o service.FinanceOrder
		if e := rows.Scan(&o.ID, &o.OutTradeNo, &o.UserID, &o.Email, &o.PaymentType, &o.Status, &o.OrderType, &o.CompletedAt, &o.RefundAt, &o.Pay, &o.Principal, &o.Bonus, &o.FeeRate, &o.Refund, &o.RefundDeducted, &o.LegacyFee, &o.LegacyRefund); e != nil {
			return e
		}
		d.Orders = append(d.Orders, o)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finance orders: %w", err)
	}
	err = financeRows(ctx, tx, `SELECT id,user_id,created_at,action,COALESCE(reward_type,'invite_percent'),amount FROM user_affiliate_ledger ORDER BY created_at,id`, func(rows *sql.Rows) error {
		var l service.FinanceLedger
		if e := rows.Scan(&l.ID, &l.UserID, &l.At, &l.Action, &l.RewardType, &l.Amount); e != nil {
			return e
		}
		d.Ledger = append(d.Ledger, l)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finance affiliate: %w", err)
	}
	err = financeRows(ctx, tx, `SELECT used_by,used_at,id,CASE WHEN value<0 THEN 'adjust_out' ELSE 'free' END,
 CASE WHEN type='admin_balance' THEN CASE WHEN value<0 THEN 'admin_adjust_out' ELSE 'admin_adjust_in' END ELSE 'redeem_code' END,ABS(value)
 FROM redeem_codes r WHERE used_by IS NOT NULL AND used_at IS NOT NULL AND type IN ('admin_balance','balance')
 AND NOT EXISTS(SELECT 1 FROM payment_orders o WHERE o.recharge_code=r.code)
 UNION ALL SELECT user_id,used_at,id,'free','promo',bonus_amount FROM promo_code_usages
 UNION ALL SELECT user_id,bucket_date::timestamp AT TIME ZONE 'Asia/Shanghai',0,'consume','usage',SUM(actual_cost)
 FROM usage_user_daily_rollups GROUP BY user_id,bucket_date`, func(rows *sql.Rows) error {
		var e service.FinanceUserEvent
		if err := rows.Scan(&e.UserID, &e.Event.At, &e.Event.ID, &e.Event.Kind, &e.Event.Source, &e.Event.Amount); err != nil {
			return err
		}
		d.Events = append(d.Events, e)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finance events: %w", err)
	}
	start, end := span.Bounds()
	err = financeRows(ctx, tx, `SELECT (l.created_at AT TIME ZONE 'Asia/Shanghai')::date::text,l.account_id,COALESCE(l.group_id,0),l.user_id,
 COALESCE(a.name,'未知账号'),COALESCE(g.name,'无分组'),COALESCE(u.email,'未知用户'),COALESCE(NULLIF(BTRIM(l.requested_model),''),l.model),
 a.id IS NULL OR a.deleted_at IS NOT NULL,(l.group_id IS NOT NULL AND g.id IS NULL) OR g.deleted_at IS NOT NULL,u.id IS NULL OR u.deleted_at IS NOT NULL,
 EXISTS(SELECT 1 FROM account_model_cost_multipliers m WHERE m.account_id=l.account_id),
 -- CUSTOM(fork): 方式只看当前配置；分组匹配查账号归属，不依赖这段用量的分组或图片数量。
 EXISTS(SELECT 1 FROM channel_account_stats_pricing_rules r
        JOIN channel_account_stats_model_pricing p ON p.rule_id=r.id
        WHERE p.billing_mode IN ('image','per_request')
          AND (l.account_id=ANY(r.account_ids) OR EXISTS(
              SELECT 1 FROM account_groups ag
              WHERE ag.account_id=l.account_id AND ag.group_id=ANY(r.group_ids)))),COUNT(*),COALESCE(SUM(l.image_count),0),
 COALESCE(SUM(l.actual_cost) FILTER(WHERE l.billing_type=0),0),COALESCE(SUM(l.actual_cost) FILTER(WHERE l.billing_type=1),0),
 SUM(COALESCE(l.account_stats_cost,l.total_cost)*COALESCE(l.account_rate_multiplier,1))
 FROM usage_logs l LEFT JOIN accounts a ON a.id=l.account_id LEFT JOIN groups g ON g.id=l.group_id LEFT JOIN users u ON u.id=l.user_id
 WHERE l.created_at >= $1 AND l.created_at < $2
 GROUP BY 1,2,3,4,5,6,7,8,9,10,11,12,13`, func(rows *sql.Rows) error {
		var c service.FinanceCost
		if e := rows.Scan(&c.Day, &c.AccountID, &c.GroupID, &c.UserID, &c.AccountName, &c.GroupName, &c.UserEmail, &c.Model, &c.AccountDeleted, &c.GroupDeleted, &c.UserDeleted, &c.ModelRate, &c.HasPerImagePricing, &c.Requests, &c.Images, &c.Consumption, &c.Subscription, &c.Cost); e != nil {
			return e
		}
		d.Costs = append(d.Costs, c)
		return nil
	}, start, end)
	if err != nil {
		return nil, fmt.Errorf("finance costs: %w", err)
	}
	d.Expenses, err = financeReadExpenses(ctx, tx, `e.deleted_at IS NULL AND e.occurred_on <= $2::date AND e.occurred_on + (e.amortize_days-1) >= $1::date`, []any{span.Start, span.End}, "")
	if err != nil {
		return nil, err
	}
	// CUSTOM(fork): 批量读取关联支出的账号，即使区间内无用量也保留完整成本方式与删除标记。
	err = financeRows(ctx, tx, `SELECT a.id,a.name,a.deleted_at IS NOT NULL,
 EXISTS(SELECT 1 FROM account_model_cost_multipliers m WHERE m.account_id=a.id),
 EXISTS(SELECT 1 FROM channel_account_stats_pricing_rules r
        JOIN channel_account_stats_model_pricing p ON p.rule_id=r.id
        WHERE p.billing_mode IN ('image','per_request')
          AND (a.id=ANY(r.account_ids) OR EXISTS(
              SELECT 1 FROM account_groups ag WHERE ag.account_id=a.id AND ag.group_id=ANY(r.group_ids))))
 FROM accounts a WHERE EXISTS(SELECT 1 FROM finance_expenses e WHERE e.account_id=a.id
 AND e.deleted_at IS NULL AND e.occurred_on <= $2::date AND e.occurred_on + (e.amortize_days-1) >= $1::date)`, func(rows *sql.Rows) error {
		var a service.FinanceExpenseAccount
		if e := rows.Scan(&a.ID, &a.Name, &a.Deleted, &a.ModelRate, &a.HasPerImagePricing); e != nil {
			return e
		}
		d.ExpenseAccounts = append(d.ExpenseAccounts, a)
		return nil
	}, span.Start, span.End)
	if err != nil {
		return nil, fmt.Errorf("finance expense accounts: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}
func (r *financeRepository) Settings(ctx context.Context) (service.FinanceSettings, error) {
	ids, err := financeFixedIDs(ctx, r.db)
	out := service.FinanceSettings{FixedCostAccountIDs: ids, Accounts: []service.FinanceAccount{}}
	if err != nil {
		return out, err
	}
	err = financeRows(ctx, r.db, `SELECT id,name,platform,deleted_at IS NOT NULL FROM accounts WHERE deleted_at IS NULL OR id=ANY($1) ORDER BY id`, func(rows *sql.Rows) error {
		var a service.FinanceAccount
		if e := rows.Scan(&a.ID, &a.Name, &a.Platform, &a.Deleted); e != nil {
			return e
		}
		out.Accounts = append(out.Accounts, a)
		return nil
	}, pq.Array(ids))
	return out, err
}
func (r *financeRepository) SaveSettings(ctx context.Context, ids []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE id=ANY($1)`, pq.Array(ids)).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return infraerrors.BadRequest("FINANCE_SETTINGS_INVALID", "包含不存在的账号")
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW()`, service.FinanceFixedAccountsKey, string(raw))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func financeReadExpenses(ctx context.Context, q financeQueryer, where string, args []any, tail string) ([]service.FinanceExpense, error) {
	out := []service.FinanceExpense{}
	err := financeRows(ctx, q, `SELECT e.id,e.occurred_on::text,e.category,e.amount,e.amortize_days,e.account_id,e.note,a.name,COALESCE(u.email,''),e.created_at,e.updated_at FROM finance_expenses e LEFT JOIN accounts a ON a.id=e.account_id LEFT JOIN users u ON u.id=e.created_by WHERE `+where+` ORDER BY e.occurred_on DESC,e.id DESC `+tail, func(rows *sql.Rows) error {
		var e service.FinanceExpense
		var amount decimal.Decimal
		if err := rows.Scan(&e.ID, &e.OccurredOn, &e.Category, &amount, &e.AmortizeDays, &e.AccountID, &e.Note, &e.AccountName, &e.CreatedByEmail, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return err
		}
		e.Amount = amount.StringFixed(2)
		e.DailyAmount = service.FinanceAmortization(amount, e.AmortizeDays, 0).String()
		out = append(out, e)
		return nil
	}, args...)
	return out, err
}
func (r *financeRepository) ListExpenses(ctx context.Context, span service.FinanceRange, category string, page, size int) ([]service.FinanceExpense, int64, error) {
	where := `e.deleted_at IS NULL AND e.occurred_on BETWEEN $1::date AND $2::date AND ($3='' OR e.category=$3)`
	args := []any{span.Start, span.End, category}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM finance_expenses e WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := financeReadExpenses(ctx, r.db, where, append(args, size, (page-1)*size), `LIMIT $4 OFFSET $5`)
	return rows, total, err
}
func (r *financeRepository) SaveExpense(ctx context.Context, id, actor int64, in service.FinanceExpenseInput) (*service.FinanceExpense, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if in.AccountID != nil {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE id=$1)`, *in.AccountID).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, infraerrors.BadRequest("FINANCE_EXPENSE_INVALID", "关联账号不存在")
		}
	}
	if id == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO finance_expenses(occurred_on,category,amount,amortize_days,account_id,note,created_by) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, in.OccurredOn, in.Category, in.Amount, in.AmortizeDays, in.AccountID, in.Note, actor).Scan(&id)
	} else {
		var res sql.Result
		res, err = tx.ExecContext(ctx, `UPDATE finance_expenses SET occurred_on=$2,category=$3,amount=$4,amortize_days=$5,account_id=$6,note=$7,updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id, in.OccurredOn, in.Category, in.Amount, in.AmortizeDays, in.AccountID, in.Note)
		if err == nil {
			n, e := res.RowsAffected()
			if e != nil {
				return nil, e
			}
			if n == 0 {
				return nil, service.ErrFinanceExpenseNotFound
			}
		}
	}
	if err != nil {
		return nil, err
	}
	items, err := financeReadExpenses(ctx, tx, `e.id=$1`, []any{id}, "")
	if err != nil {
		return nil, err
	}
	if len(items) != 1 {
		return nil, service.ErrFinanceExpenseNotFound
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &items[0], nil
}
func (r *financeRepository) DeleteExpense(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE finance_expenses SET deleted_at=NOW(),updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrFinanceExpenseNotFound
	}
	return nil
}
