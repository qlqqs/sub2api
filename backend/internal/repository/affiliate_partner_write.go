package repository

// CUSTOM(fork): 不可撤销的区域写入与按合伙人串行的结算操作。
import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

func (r *partnerRepository) Create(ctx context.Context, id int64, name string, rate decimal.Decimal, actor int64) error {
	a := &affiliateRepository{client: r.client}
	return a.withTx(ctx, func(ctx context.Context, c *dbent.Client) error {

		n, err := scanInt64(ctx, c, `SELECT COUNT(*) FROM users WHERE id=$1 AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		if n == 0 {
			return service.ErrPartnerUserNotFound
		}
		if _, err := ensureUserAffiliateWithClient(ctx, c, id); err != nil {
			return err
		}
		if _, err := scanInt64(ctx, c, `SELECT user_id FROM user_affiliates WHERE user_id=$1 FOR UPDATE`, id); err != nil {
			return err
		}
		state, err := partnerReadState(ctx, c)
		if err != nil {
			return err
		}
		u, ok := state.Snapshots[id]
		if !ok || u.User.Deleted {
			return service.ErrPartnerUserNotFound
		}
		if _, ok := state.Partners[id]; ok {
			return service.ErrPartnerExists
		}
		if _, ok := state.Attachments[id]; ok {
			return service.ErrPartnerAttached
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO affiliate_partners(user_id,region_name,created_by) VALUES($1,$2,$3)`, id, name, actor); err != nil {
			return err
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO affiliate_partner_rates(partner_user_id,effective_month,share_rate,created_by) VALUES($1,date_trunc('month',CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date,$2,$3)`, id, rate, actor); err != nil {
			return err
		}
		_, err = c.ExecContext(ctx, `UPDATE user_affiliates SET agent_level=1,agent_level_locked=true,agent_level_source='manual',agent_level_updated_by=$2,agent_level_updated_at=NOW(),updated_at=NOW() WHERE user_id=$1`, id, actor)
		return err
	})
}
func (r *partnerRepository) Attach(ctx context.Context, id, agent, actor int64) error {
	a := &affiliateRepository{client: r.client}
	return a.withTx(ctx, func(ctx context.Context, c *dbent.Client) error {
		if _, err := scanInt64(ctx, c, `SELECT COUNT(*) FROM (SELECT user_id FROM user_affiliates WHERE user_id=$1 FOR UPDATE) a`, agent); err != nil {
			return err
		}
		state, err := partnerReadState(ctx, c)
		if err != nil {
			return err
		}
		if _, ok := state.Partners[id]; !ok {
			return service.ErrPartnerNotFound
		}
		if _, ok := state.Partners[agent]; ok {
			return service.ErrPartnerAgentIsPartner
		}
		if _, ok := state.Attachments[agent]; ok {
			return service.ErrPartnerAttached
		}
		u, ok := state.Snapshots[agent]
		if !ok || u.User.Deleted || u.AgentLevel != 1 {
			return service.ErrPartnerAgentLevel
		}
		_, err = c.ExecContext(ctx, `INSERT INTO affiliate_partner_agents(agent_user_id,partner_user_id,attached_by,source) VALUES($1,$2,$3,'manual')`, agent, id, actor)
		return err
	})
}

func (r *partnerRepository) WithPartnerLock(ctx context.Context, id int64, fn func(context.Context) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var found int64
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM affiliate_partners WHERE user_id=$1 FOR UPDATE`, id).Scan(&found); err == sql.ErrNoRows {
		return service.ErrPartnerNotFound
	} else if err != nil {
		return err
	}
	if err = fn(context.WithValue(ctx, partnerTxKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *partnerRepository) Update(ctx context.Context, id int64, name *string, rate *decimal.Decimal, actor int64, month string) error {
	return r.WithPartnerLock(ctx, id, func(ctx context.Context) error {
		tx := partnerTx(ctx)
		if name != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE affiliate_partners SET region_name=$2,updated_at=NOW() WHERE user_id=$1`, id, *name); err != nil {
				return err
			}
		}
		if rate == nil {
			return nil
		}
		next, err := time.ParseInLocation("2006-01", month, service.AffiliateTeamLocation)
		if err != nil {
			return err
		}
		var current decimal.Decimal
		if err = tx.QueryRowContext(ctx, `SELECT share_rate FROM affiliate_partner_rates WHERE partner_user_id=$1 AND effective_month<$2::date ORDER BY effective_month DESC LIMIT 1`, id, next.Format("2006-01-02")).Scan(&current); err != nil {
			return err
		}
		if current.Equal(*rate) {
			_, err = tx.ExecContext(ctx, `DELETE FROM affiliate_partner_rates WHERE partner_user_id=$1 AND effective_month=$2::date`, id, next.Format("2006-01-02"))
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_partner_rates(partner_user_id,effective_month,share_rate,created_by) VALUES($1,$2::date,$3,$4) ON CONFLICT(partner_user_id,effective_month) DO UPDATE SET share_rate=EXCLUDED.share_rate,created_by=EXCLUDED.created_by,created_at=NOW()`, id, next.Format("2006-01-02"), *rate, actor)
		return err
	})
}
func (r *partnerRepository) SaveStatement(ctx context.Context, id, actor int64, d service.PartnerStatementDetail) error {
	tx := partnerTx(ctx)
	// CUSTOM(fork): 确认时间同时冻结到 DTO 和索引列。
	now := time.Now()
	d.Totals.ConfirmedAt = &now
	t := d.Totals
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO affiliate_partner_statements(partner_user_id,month,status,share_rate,consumption,revenue,account_cost,commission,region_expense,adjustment,profit,loss_before,loss_after,share_base,share_amount,detail,confirmed_by,confirmed_at) VALUES($1,$2::date,'confirmed',$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, id, t.Month+"-01", t.ShareRate, t.Consumption, t.Revenue, t.AccountCost, t.Commission, t.RegionExpense, t.Adjustment, t.Profit, t.LossBefore, t.LossAfter, t.ShareBase, t.ShareAmount, string(raw), actor, now)
	return err
}
func (r *partnerRepository) PayStatement(ctx context.Context, id int64, month string, actor int64, day, note string) error {
	return r.WithPartnerLock(ctx, id, func(ctx context.Context) error {
		tx := partnerTx(ctx)
		var status string
		err := tx.QueryRowContext(ctx, `SELECT status FROM affiliate_partner_statements WHERE partner_user_id=$1 AND month=$2::date`, id, month+"-01").Scan(&status)
		if err == sql.ErrNoRows {
			return infraerrors.BadRequest("PARTNER_MONTH_NOT_CONFIRMED", "请先确认该月份结算单")
		} else if err != nil {
			return err
		}
		if status == "paid" {
			return infraerrors.Conflict("PARTNER_MONTH_PAID", "该结算单已登记付款")
		}
		_, err = tx.ExecContext(ctx, `UPDATE affiliate_partner_statements SET status='paid',paid_by=$3,paid_on=$4::date,paid_note=$5 WHERE partner_user_id=$1 AND month=$2::date`, id, month+"-01", actor, day, note)
		return err
	})
}
func (r *partnerRepository) AddAdjustment(ctx context.Context, id int64, month, amount, note string, actor int64) (*service.PartnerAdjustment, error) {
	var out service.PartnerAdjustment
	err := r.WithPartnerLock(ctx, id, func(ctx context.Context) error {
		tx := partnerTx(ctx)
		if err := partnerAdjustmentMonth(ctx, tx, id, month); err != nil {
			return err
		}
		out = service.PartnerAdjustment{PartnerID: id, Month: month, Amount: amount, Note: note}
		return tx.QueryRowContext(ctx, `INSERT INTO affiliate_partner_adjustments(partner_user_id,month,amount,note,created_by) VALUES($1,$2::date,$3,$4,$5) RETURNING id,created_at,(SELECT email FROM users WHERE id=$5)`, id, month+"-01", amount, note, actor).Scan(&out.ID, &out.CreatedAt, &out.CreatedByEmail)
	})
	return &out, err
}
func partnerAdjustmentMonth(ctx context.Context, tx *sql.Tx, id int64, month string) error {
	var exists bool
	var first string
	if err := tx.QueryRowContext(ctx, `SELECT to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM'),EXISTS(SELECT 1 FROM affiliate_partner_statements WHERE partner_user_id=$1 AND month=$2::date) FROM affiliate_partners WHERE user_id=$1`, id, month+"-01").Scan(&first, &exists); err != nil {
		return err
	}
	if month < first || month > time.Now().In(service.AffiliateTeamLocation).Format("2006-01") {
		return service.ErrPartnerMonthRange
	}
	if exists {
		return service.ErrPartnerMonthConfirmed
	}
	return nil
}
func (r *partnerRepository) DeleteAdjustment(ctx context.Context, id, adjustment int64) error {
	return r.WithPartnerLock(ctx, id, func(ctx context.Context) error {
		tx := partnerTx(ctx)
		var month string
		err := tx.QueryRowContext(ctx, `SELECT to_char(month,'YYYY-MM') FROM affiliate_partner_adjustments WHERE partner_user_id=$1 AND id=$2`, id, adjustment).Scan(&month)
		if err == sql.ErrNoRows {
			return infraerrors.NotFound("PARTNER_ADJUSTMENT_NOT_FOUND", "调整项不存在")
		} else if err != nil {
			return err
		}
		if err = partnerAdjustmentMonth(ctx, tx, id, month); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM affiliate_partner_adjustments WHERE partner_user_id=$1 AND id=$2`, id, adjustment)
		return err
	})
}

// CUSTOM(fork): 与档位更新共用实体事务；读取升级前的归属快照，升级不改快照。
func autoAttachPartnerAgents(ctx context.Context, c *dbent.Client, ids []int64, actor int64) error {
	state, err := partnerReadState(ctx, c)
	if err != nil {
		return err
	}
	members := service.ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots)
	for _, id := range ids {
		if _, ok := state.Partners[id]; ok {
			continue
		}
		if _, ok := state.Attachments[id]; ok {
			continue
		}
		m, ok := members[id]
		if !ok {
			continue
		}
		if _, err = c.ExecContext(ctx, `INSERT INTO affiliate_partner_agents(agent_user_id,partner_user_id,attached_by,source) VALUES($1,$2,$3,'promote')`, id, m.PartnerID, nullableOperatorArg(actor)); err != nil {
			return err
		}
	}
	return nil
}
func lockPartnerLevels(ctx context.Context, c *dbent.Client, ids []int64) error {
	if err := financeRows(ctx, c, `SELECT user_id FROM user_affiliates WHERE user_id=ANY($1) ORDER BY user_id FOR UPDATE`, func(rows *sql.Rows) error { var id int64; return rows.Scan(&id) }, pq.Array(ids)); err != nil {
		return err
	}
	n, err := scanInt64(ctx, c, `SELECT COUNT(*) FROM affiliate_partners WHERE user_id=ANY($1)`, pq.Array(ids))
	if err != nil {
		return err
	}
	if n > 0 {
		return service.ErrPartnerLevelFixed
	}
	return nil
}
