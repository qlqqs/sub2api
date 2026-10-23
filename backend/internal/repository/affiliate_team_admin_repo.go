package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 管理端团队消费页的汇总查询。汇总页的行是「有团队的人」，与代理端准入一致：当前一级 / 二级代理，
// 以及仍是某客户归属代理或上方一级的用户（已降级、已删除也算）。分润读分润日汇总（迁移 252）。

const teamOwnersCTE = `owners AS (
    SELECT user_id FROM user_affiliates WHERE agent_level IN (1, 2)
    UNION
    SELECT agent_owner_id FROM user_affiliates WHERE agent_owner_id IS NOT NULL
    UNION
    SELECT agent_upline_id FROM user_affiliates WHERE agent_upline_id IS NOT NULL
)`

// teamOwnerBaseSQL 是行的展示字段（别名 base 的列）；等级缺失按 3（已降级）处理。
const teamOwnerBaseSQL = `
    SELECT o.user_id,
           COALESCE(u.username, '') AS nickname,
           COALESCE(u.email, '') AS email,
           (u.id IS NULL OR u.deleted_at IS NOT NULL) AS deleted,
           COALESCE(ua.agent_level, 3) AS agent_level,
           CASE WHEN COALESCE(ua.agent_level, 3) = 2 THEN ua.agent_owner_id END AS upline_id
    FROM owners o
    LEFT JOIN users u ON u.id = o.user_id
    LEFT JOIN user_affiliates ua ON ua.user_id = o.user_id`

var affiliateTeamOwnerSorts = map[string]string{
	service.AffiliateTeamSortTeamCost:        "entries.team_cost",
	service.AffiliateTeamSortEarnings:        "entries.earnings",
	service.AffiliateTeamSortActiveCustomers: "entries.active_customers",
}

// teamAdminSearchSQL 是管理端搜索（作用于含 user_id、nickname、email、deleted 列的别名 entries）：
// 昵称 / 邮箱模糊，不含已删除账号；纯数字时另按用户 ID 精确匹配，已删除账号也能找到。
func teamAdminSearchSQL(args *teamArgs, search string) string {
	pattern := args.add("%" + escapeLike(search) + "%")
	cond := "(NOT entries.deleted AND (entries.nickname ILIKE " + pattern + ` ESCAPE '\' OR entries.email ILIKE ` + pattern + ` ESCAPE '\'))`
	if id, ok := parseTeamSearchID(search); ok {
		cond = "(" + cond + " OR entries.user_id = " + args.add(id) + ")"
	}
	return cond
}

func parseTeamSearchID(search string) (int64, bool) {
	for _, r := range search {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(search, 10, 64)
	return id, err == nil && id > 0
}

func teamOwnerLevelSQL(level string) string {
	switch level {
	case service.AffiliateTeamLevel1:
		return "entries.agent_level = 1"
	case service.AffiliateTeamLevel2:
		return "entries.agent_level = 2"
	case service.AffiliateTeamLevelDowngraded:
		return "entries.agent_level NOT IN (1, 2)"
	default:
		return "TRUE"
	}
}

func (r *affiliateTeamRepository) GetViewerSubject(ctx context.Context, userID int64) (service.AffiliateTeamViewerSubject, error) {
	var (
		out                     service.AffiliateTeamViewerSubject
		uplineID                sql.NullInt64
		uplineName, uplineEmail string
		uplineDeleted           bool
	)
	_, err := r.queryOne(ctx, `
SELECT v.id,
       COALESCE(u.username, ''),
       COALESCE(u.email, ''),
       (u.id IS NULL OR u.deleted_at IS NOT NULL),
       CASE WHEN COALESCE(ua.agent_level, 3) = 2 THEN ua.agent_owner_id END,
       COALESCE(up.username, ''),
       COALESCE(up.email, ''),
       (up.id IS NULL OR up.deleted_at IS NOT NULL),
       COALESCE((
           SELECT SUM(rw.amount) FROM affiliate_agent_reward_daily_rollups rw WHERE rw.user_id = v.id
       ), 0)::double precision
FROM (VALUES ($1::bigint)) AS v(id)
LEFT JOIN users u ON u.id = v.id
LEFT JOIN user_affiliates ua ON ua.user_id = v.id
LEFT JOIN users up ON up.id = ua.agent_owner_id AND COALESCE(ua.agent_level, 3) = 2`,
		[]any{userID},
		&out.User.UserID, &out.User.Nickname, &out.User.Email, &out.User.Deleted,
		&uplineID, &uplineName, &uplineEmail, &uplineDeleted, &out.LifetimeEarnings)
	if err != nil {
		return out, fmt.Errorf("get team viewer subject: %w", err)
	}
	if uplineID.Valid {
		out.Upline = &service.AffiliateTeamUser{UserID: uplineID.Int64, Nickname: uplineName, Email: uplineEmail, Deleted: uplineDeleted}
	}
	return out, nil
}

// GetPlatformAggregates 一次扫描全站用户日汇总：渠道客户（有归属代理）的行出趋势、模型分布与按客户汇总，
// 全部用户的按用户行出全站余额消费；分润按天读分润日汇总，不限来源。
func (r *affiliateTeamRepository) GetPlatformAggregates(ctx context.Context, rng service.AffiliateTeamRange) (service.AffiliateTeamPlatformAggregates, error) {
	out := service.AffiliateTeamPlatformAggregates{
		Trend:  make([]service.AffiliateTeamTrendPoint, 0),
		Models: make([]service.AffiliateTeamModelShare, 0),
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT GROUPING(r.bucket_date) = 0 AS by_day,
       GROUPING(r.model) = 0 AS by_model,
       ua.user_id IS NOT NULL AS channel,
       COALESCE(r.bucket_date::text, ''),
       COALESCE(r.model, ''),
       SUM(r.actual_cost)::double precision
FROM usage_user_daily_rollups r
LEFT JOIN user_affiliates ua ON ua.user_id = r.user_id AND ua.agent_owner_id IS NOT NULL
WHERE r.bucket_date BETWEEN $1::date AND $2::date
GROUP BY GROUPING SETS (
    (ua.user_id IS NOT NULL, r.bucket_date),
    (ua.user_id IS NOT NULL, r.model),
    (ua.user_id IS NOT NULL, r.user_id)
)`, rng.StartDate, rng.EndDate)
	if err != nil {
		return out, fmt.Errorf("aggregate platform team usage: %w", err)
	}
	for rows.Next() {
		var (
			byDay, byModel, channel bool
			date, model             string
			cost                    float64
		)
		if err := rows.Scan(&byDay, &byModel, &channel, &date, &model, &cost); err != nil {
			_ = rows.Close()
			return out, err
		}
		switch {
		case byDay:
			if channel {
				out.Trend = append(out.Trend, service.AffiliateTeamTrendPoint{Date: date, Cost: cost})
			}
		case byModel:
			if channel && cost > 0 {
				out.Models = append(out.Models, service.AffiliateTeamModelShare{Model: model, Cost: cost})
			}
		default: // 按用户
			out.Summary.PlatformCost += cost
			if channel {
				out.Summary.Cost += cost
				if cost > 0 {
					out.Summary.ActiveCustomers++
				}
			}
		}
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}
	sort.SliceStable(out.Models, func(i, j int) bool {
		if out.Models[i].Cost != out.Models[j].Cost {
			return out.Models[i].Cost > out.Models[j].Cost
		}
		return out.Models[i].Model < out.Models[j].Model
	})

	if _, err := r.queryOne(ctx, `
WITH `+teamOwnersCTE+`
SELECT (SELECT COUNT(*) FROM user_affiliates WHERE agent_owner_id IS NOT NULL),
       COUNT(*),
       COUNT(*) FILTER (WHERE COALESCE(ua.agent_level, 3) = 1),
       COUNT(*) FILTER (WHERE COALESCE(ua.agent_level, 3) = 2),
       COUNT(*) FILTER (WHERE COALESCE(ua.agent_level, 3) NOT IN (1, 2))
FROM owners o
LEFT JOIN user_affiliates ua ON ua.user_id = o.user_id`, nil,
		&out.Summary.Customers, &out.Counts.All, &out.Counts.Level1, &out.Counts.Level2, &out.Counts.Downgraded); err != nil {
		return out, fmt.Errorf("count team owners: %w", err)
	}

	rows, err = client.QueryContext(ctx, `
SELECT rw.bucket_date::text, SUM(rw.amount)::double precision
FROM affiliate_agent_reward_daily_rollups rw
WHERE rw.bucket_date BETWEEN $1::date AND $2::date
GROUP BY 1`, rng.StartDate, rng.EndDate)
	if err != nil {
		return out, fmt.Errorf("aggregate platform team earnings: %w", err)
	}
	earnings := make(map[string]float64)
	for rows.Next() {
		var (
			date   string
			amount float64
		)
		if err := rows.Scan(&date, &amount); err != nil {
			_ = rows.Close()
			return out, err
		}
		earnings[date] = amount
		out.Summary.Earnings += amount
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}
	for i := range out.Trend {
		if v, ok := earnings[out.Trend[i].Date]; ok {
			out.Trend[i].Earnings = v
			delete(earnings, out.Trend[i].Date)
		}
	}
	for date, v := range earnings { // 有分润但当天渠道客户无消费
		out.Trend = append(out.Trend, service.AffiliateTeamTrendPoint{Date: date, Earnings: v})
	}
	return out, nil
}

// ListTeamOwners 每行的团队消费、活跃 / 客户、分润与该代理 GetAggregates 的团队摘要相同：
// 团队 = agent_owner_id = X OR agent_upline_id = X；分润只计来源仍在其团队中的部分。
func (r *affiliateTeamRepository) ListTeamOwners(ctx context.Context, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamOwnerEntry, int64, error) {
	args := &teamArgs{}
	filters := []string{teamOwnerLevelSQL(query.Level)}
	if query.Search != "" {
		filters = append(filters, teamAdminSearchSQL(args, query.Search))
	}
	where := strings.Join(filters, " AND ")
	baseArgs := len(args.values)

	var total int64
	if _, err := r.queryOne(ctx, `
WITH `+teamOwnersCTE+`, entries AS (`+teamOwnerBaseSQL+`
)
SELECT COUNT(*) FROM entries WHERE `+where, args.values[:baseArgs], &total); err != nil {
		return nil, 0, fmt.Errorf("count team owners: %w", err)
	}

	d1, d2 := args.add(rng.StartDate), args.add(rng.EndDate)
	order := affiliateTeamOwnerSorts[query.SortBy]
	if order == "" {
		order = affiliateTeamOwnerSorts[service.AffiliateTeamSortTeamCost]
	}
	limit, offset := args.add(query.PageSize), args.add((query.Page-1)*query.PageSize)
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
WITH `+teamOwnersCTE+`, base AS (`+teamOwnerBaseSQL+`
), usage AS (
    SELECT r.user_id, SUM(r.actual_cost) AS cost
    FROM usage_user_daily_rollups r
    WHERE r.bucket_date BETWEEN `+d1+`::date AND `+d2+`::date
    GROUP BY r.user_id
), membership AS (
    -- 客户分别计入归属代理与上方一级的团队；UNION 去重，与代理端按客户计数一致。
    SELECT agent_owner_id AS agent_id, user_id FROM user_affiliates WHERE agent_owner_id IS NOT NULL
    UNION
    SELECT agent_upline_id, user_id FROM user_affiliates WHERE agent_upline_id IS NOT NULL
), per_agent AS (
    SELECT m.agent_id,
           COUNT(*) AS customers,
           COUNT(*) FILTER (WHERE usage.cost > 0) AS active_customers,
           COALESCE(SUM(usage.cost), 0) AS team_cost
    FROM membership m
    LEFT JOIN usage ON usage.user_id = m.user_id
    GROUP BY m.agent_id
), earn AS (
    SELECT rw.user_id AS agent_id, SUM(rw.amount) AS earnings
    FROM affiliate_agent_reward_daily_rollups rw
    JOIN user_affiliates s ON s.user_id = rw.source_user_id
    WHERE rw.bucket_date BETWEEN `+d1+`::date AND `+d2+`::date
      AND (s.agent_owner_id = rw.user_id OR s.agent_upline_id = rw.user_id)
    GROUP BY rw.user_id
), channel_total AS (
    SELECT COALESCE(SUM(usage.cost), 0) AS cost
    FROM user_affiliates ua
    JOIN usage ON usage.user_id = ua.user_id
    WHERE ua.agent_owner_id IS NOT NULL
), entries AS (
    SELECT b.*,
           COALESCE(p.customers, 0) AS customers,
           COALESCE(p.active_customers, 0) AS active_customers,
           COALESCE(p.team_cost, 0) AS team_cost,
           COALESCE(e.earnings, 0) AS earnings
    FROM base b
    LEFT JOIN per_agent p ON p.agent_id = b.user_id
    LEFT JOIN earn e ON e.agent_id = b.user_id
)
SELECT entries.user_id, entries.nickname, entries.email, entries.deleted, entries.agent_level,
       entries.upline_id, COALESCE(up.username, ''), COALESCE(up.email, ''), (up.id IS NULL OR up.deleted_at IS NOT NULL),
       entries.customers, entries.active_customers,
       entries.team_cost::double precision,
       entries.earnings::double precision,
       CASE WHEN channel_total.cost > 0 THEN (entries.team_cost / channel_total.cost)::double precision ELSE 0 END
FROM entries
CROSS JOIN channel_total
LEFT JOIN users up ON up.id = entries.upline_id
WHERE `+where+`
ORDER BY `+order+` `+sortDirection(query.SortDesc)+` NULLS LAST, entries.user_id DESC
LIMIT `+limit+` OFFSET `+offset, args.values...)
	if err != nil {
		return nil, 0, fmt.Errorf("list team owners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateTeamOwnerEntry, 0, query.PageSize)
	for rows.Next() {
		var (
			item                    service.AffiliateTeamOwnerEntry
			uplineID                sql.NullInt64
			uplineName, uplineEmail string
			uplineDeleted           bool
		)
		if err := rows.Scan(&item.User.UserID, &item.User.Nickname, &item.User.Email, &item.User.Deleted, &item.AgentLevel,
			&uplineID, &uplineName, &uplineEmail, &uplineDeleted,
			&item.Customers, &item.ActiveCustomers, &item.TeamCost, &item.Earnings, &item.CostShare); err != nil {
			return nil, 0, err
		}
		if uplineID.Valid {
			item.Upline = &service.AffiliateTeamUser{UserID: uplineID.Int64, Nickname: uplineName, Email: uplineEmail, Deleted: uplineDeleted}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
