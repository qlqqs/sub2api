package repository

// CUSTOM(fork): 合伙人原生 SQL 仓储；结算快照与财务重放共用事务。
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type partnerRepository struct {
	db     *sql.DB
	client *dbent.Client
}

func NewPartnerRepository(db *sql.DB, client *dbent.Client) service.PartnerRepository {
	return &partnerRepository{db: db, client: client}
}

type partnerTxKey struct{}

func (r *partnerRepository) query(ctx context.Context) financeQueryer {
	if tx, ok := ctx.Value(partnerTxKey{}).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// CUSTOM(fork): 写操作都在 WithPartnerLock 的事务内执行。
func partnerTx(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(partnerTxKey{}).(*sql.Tx)
	return tx
}

type partnerRowsQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func partnerReadState(ctx context.Context, q partnerRowsQueryer) (*service.PartnerState, error) {
	s := &service.PartnerState{Partners: map[int64]service.Partner{}, Attachments: map[int64]service.PartnerAttachment{}, Snapshots: map[int64]service.PartnerSnapshot{}, Statements: map[int64]map[string]service.PartnerStatementDetail{}, Rates: []service.PartnerRate{}, Adjustments: []service.PartnerAdjustment{}}
	err := financeRows(ctx, q, `SELECT user_id,region_name,created_at FROM affiliate_partners ORDER BY user_id`, func(rows *sql.Rows) error {
		var p service.Partner
		if e := rows.Scan(&p.UserID, &p.RegionName, &p.CreatedAt); e != nil {
			return e
		}
		s.Partners[p.UserID] = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `SELECT partner_user_id,to_char(effective_month,'YYYY-MM'),share_rate FROM affiliate_partner_rates ORDER BY effective_month`, func(rows *sql.Rows) error {
		var p service.PartnerRate
		if e := rows.Scan(&p.PartnerID, &p.Month, &p.Rate); e != nil {
			return e
		}
		s.Rates = append(s.Rates, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `SELECT a.agent_user_id,a.partner_user_id,a.attached_at,a.source,u.email FROM affiliate_partner_agents a LEFT JOIN users u ON u.id=a.attached_by ORDER BY a.agent_user_id`, func(rows *sql.Rows) error {
		var a service.PartnerAttachment
		if e := rows.Scan(&a.AgentID, &a.PartnerID, &a.AttachedAt, &a.Source, &a.AttachedByEmail); e != nil {
			return e
		}
		s.Attachments[a.AgentID] = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `SELECT COALESCE(u.id,a.user_id),COALESCE(u.email,''),COALESCE(u.username,''),u.id IS NULL OR u.deleted_at IS NOT NULL,COALESCE(u.created_at,a.created_at),COALESCE(a.agent_level,3),a.agent_owner_id,a.agent_upline_id,a.agent_owner_bound_at FROM users u FULL JOIN user_affiliates a ON a.user_id=u.id`, func(rows *sql.Rows) error {
		var v service.PartnerSnapshot
		if e := rows.Scan(&v.User.ID, &v.User.Email, &v.User.Username, &v.User.Deleted, &v.CreatedAt, &v.AgentLevel, &v.OwnerID, &v.UplineID, &v.BoundAt); e != nil {
			return e
		}
		s.Snapshots[v.User.ID] = v
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `SELECT a.id,a.partner_user_id,to_char(a.month,'YYYY-MM'),a.amount::text,a.note,a.created_at,COALESCE(u.email,'') FROM affiliate_partner_adjustments a LEFT JOIN users u ON u.id=a.created_by ORDER BY a.id`, func(rows *sql.Rows) error {
		var a service.PartnerAdjustment
		if e := rows.Scan(&a.ID, &a.PartnerID, &a.Month, &a.Amount, &a.Note, &a.CreatedAt, &a.CreatedByEmail); e != nil {
			return e
		}
		s.Adjustments = append(s.Adjustments, a)
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `SELECT partner_user_id,to_char(month,'YYYY-MM'),detail,status,confirmed_at,paid_on::text,paid_note FROM affiliate_partner_statements ORDER BY month`, func(rows *sql.Rows) error {
		var id int64
		var month, status string
		var raw []byte
		var confirmed time.Time
		var paid, note *string
		if e := rows.Scan(&id, &month, &raw, &status, &confirmed, &paid, &note); e != nil {
			return e
		}
		var d service.PartnerStatementDetail
		if e := json.Unmarshal(raw, &d); e != nil {
			return fmt.Errorf("decode partner statement: %w", e)
		}
		d.Totals.Status = status
		d.Totals.ConfirmedAt = &confirmed
		d.Totals.PaidOn = paid
		d.Totals.PaidNote = note
		if s.Statements[id] == nil {
			s.Statements[id] = map[string]service.PartnerStatementDetail{}
		}
		s.Statements[id][month] = d
		return nil
	})
	return s, err
}
func (r *partnerRepository) Read(ctx context.Context) (*service.PartnerState, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return partnerReadState(ctx, tx.Client())
	}
	return partnerReadState(ctx, r.query(ctx))
}
func (r *partnerRepository) LoadMonths(ctx context.Context, ids []int64, from, to string) (*service.PartnerMonthsData, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := partnerLoadMonths(ctx, tx, ids, from, to)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}
func partnerLoadMonths(ctx context.Context, q financeQueryer, ids []int64, from, to string) (*service.PartnerMonthsData, error) {
	state, err := partnerReadState(ctx, q)
	if err != nil {
		return nil, err
	}
	endMonth, err := time.ParseInLocation("2006-01", to, service.AffiliateTeamLocation)
	if err != nil {
		return nil, err
	}
	span := service.FinanceRange{Start: from + "-01", End: endMonth.AddDate(0, 1, -1).Format("2006-01-02"), Granularity: "month"}
	fd, err := financeLoad(ctx, q, span, true)
	if err != nil {
		return nil, err
	}
	d := &service.PartnerMonthsData{State: state, Finance: fd, Members: service.ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots), StartDays: map[int64]service.PartnerStartDay{}}
	selected := map[int64]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	members := []int64{}
	for id, m := range d.Members {
		if selected[m.PartnerID] {
			members = append(members, id)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i] < members[j] })
	starts := make([]time.Time, len(members))
	for i, id := range members {
		starts[i] = d.Members[id].StartAt
	}
	start, end := span.Bounds()
	err = financeRows(ctx, q, `WITH m(user_id,start_at) AS (SELECT * FROM unnest($1::bigint[],$2::timestamptz[]))
 SELECT l.user_id,(l.created_at AT TIME ZONE 'Asia/Shanghai')::date::text,COALESCE(l.group_id,0),COALESCE(g.name,'无分组'),COALESCE(NULLIF(BTRIM(l.requested_model),''),l.model),
 SUM(CASE WHEN l.account_id=ANY($3::bigint[]) THEN 0 ELSE COALESCE(l.account_stats_cost,l.total_cost)*COALESCE(l.account_rate_multiplier,1) END),
 SUM(CASE WHEN l.billing_type=0 THEN l.actual_cost ELSE 0 END),COUNT(*) FILTER(WHERE l.account_id=ANY($3::bigint[]))
 FROM usage_logs l JOIN m ON m.user_id=l.user_id LEFT JOIN groups g ON g.id=l.group_id
 WHERE l.created_at>=GREATEST(m.start_at,$4) AND l.created_at<$5 GROUP BY 1,2,3,4,5`, func(rows *sql.Rows) error {
		var u service.PartnerUsage
		if e := rows.Scan(&u.UserID, &u.Day, &u.GroupID, &u.GroupName, &u.Model, &u.Cost, &u.Consumption, &u.FixedRequests); e != nil {
			return e
		}
		d.Usage = append(d.Usage, u)
		return nil
	}, pq.Array(members), pq.Array(starts), pq.Array(fd.FixedIDs), start, end)
	if err != nil {
		return nil, err
	}
	err = financeRows(ctx, q, `WITH m(user_id,start_at) AS (SELECT * FROM unnest($1::bigint[],$2::timestamptz[]))
 SELECT m.user_id,COALESCE(SUM(l.actual_cost),0),COALESCE(SUM(l.actual_cost) FILTER(WHERE l.created_at>=m.start_at),0),COUNT(l.id)
 FROM m LEFT JOIN usage_logs l ON l.user_id=m.user_id AND l.billing_type=0
 AND l.created_at>=date_trunc('day',m.start_at AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'
 AND l.created_at<(date_trunc('day',m.start_at AT TIME ZONE 'Asia/Shanghai')+INTERVAL '1 day') AT TIME ZONE 'Asia/Shanghai'
 WHERE m.start_at>=$3 AND m.start_at<$4 GROUP BY m.user_id`, func(rows *sql.Rows) error {
		var id int64
		var v service.PartnerStartDay
		if e := rows.Scan(&id, &v.Total, &v.After, &v.Requests); e != nil {
			return e
		}
		d.StartDays[id] = v
		return nil
	}, pq.Array(members), pq.Array(starts), start, end)
	return d, err
}
