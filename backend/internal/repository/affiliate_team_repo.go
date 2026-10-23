package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 代理团队消费页的查询。范围全部按归属快照：
//   - 我的客户：agent_owner_id = 我
//   - 下级团队：agent_upline_id = 我；二级代理 X 的团队：agent_owner_id = X AND agent_upline_id = 我
//   - 下级代理集合：当前 agent_level = 2 且归属我的用户 ∪ 下级团队成员的归属代理
// 多客户聚合读 usage_user_daily_rollups（迁移 251），收益读 user_affiliate_ledger 的代理分润。

const affiliateTeamAgentRewardTypes = `('agent_direct', 'agent_diff')`

type affiliateTeamRepository struct {
	client *dbent.Client
}

func NewAffiliateTeamRepository(client *dbent.Client) service.AffiliateTeamRepository {
	return &affiliateTeamRepository{client: client}
}

// teamArgs 按出现顺序生成 $n 占位符。
type teamArgs struct{ values []any }

func (a *teamArgs) add(v any) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}

// subAgentSetSQL 返回下级代理集合（列 agent_id）；viewer 为 viewerID 的占位符。
func subAgentSetSQL(viewer string) string {
	return `
SELECT sa.user_id AS agent_id FROM user_affiliates sa WHERE sa.agent_level = 2 AND sa.agent_owner_id = ` + viewer + `
UNION
SELECT sm.agent_owner_id FROM user_affiliates sm WHERE sm.agent_upline_id = ` + viewer + ` AND sm.agent_owner_id IS NOT NULL`
}

// scopeSQL 返回范围 CTE 的 WHERE 条件（作用于别名 ua）。
func scopeSQL(args *teamArgs, viewer string, scope service.AffiliateTeamScope) string {
	switch {
	case scope.AgentID > 0:
		return "ua.agent_owner_id = " + args.add(scope.AgentID) + " AND ua.agent_upline_id = " + viewer
	case scope.CustomerID > 0:
		return "ua.user_id = " + args.add(scope.CustomerID) + " AND (ua.agent_owner_id = " + viewer + " OR ua.agent_upline_id = " + viewer + ")"
	default:
		return "(ua.agent_owner_id = " + viewer + " OR ua.agent_upline_id = " + viewer + ")"
	}
}

func (r *affiliateTeamRepository) queryOne(ctx context.Context, query string, args []any, dest ...any) (bool, error) {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return false, rows.Err()
	}
	if err := rows.Scan(dest...); err != nil {
		return false, err
	}
	return true, rows.Err()
}

func (r *affiliateTeamRepository) GetViewerState(ctx context.Context, userID int64) (service.AffiliateTeamViewerState, error) {
	var state service.AffiliateTeamViewerState
	_, err := r.queryOne(ctx, `
SELECT COALESCE((SELECT agent_level FROM user_affiliates WHERE user_id = $1), 3),
       EXISTS (SELECT 1 FROM user_affiliates WHERE agent_owner_id = $1),
       EXISTS (SELECT 1 FROM user_affiliates WHERE agent_upline_id = $1)`,
		[]any{userID}, &state.AgentLevel, &state.HasOwnedCustomers, &state.HasUplineCustomer)
	if err != nil {
		return state, fmt.Errorf("get team viewer state: %w", err)
	}
	return state, nil
}

func (r *affiliateTeamRepository) CountTeam(ctx context.Context, viewerID int64) (service.AffiliateTeamCounts, error) {
	var counts service.AffiliateTeamCounts
	_, err := r.queryOne(ctx, `
SELECT (SELECT COUNT(*) FROM user_affiliates WHERE agent_owner_id = $1),
       (SELECT COUNT(*) FROM (`+subAgentSetSQL("$1")+`) s)`,
		[]any{viewerID}, &counts.DirectCustomers, &counts.SubAgents)
	if err != nil {
		return counts, fmt.Errorf("count team: %w", err)
	}
	return counts, nil
}

func (r *affiliateTeamRepository) GetSubAgent(ctx context.Context, viewerID, agentID int64) (*service.AffiliateTeamAgentSubject, error) {
	var out service.AffiliateTeamAgentSubject
	found, err := r.queryOne(ctx, `
SELECT s.agent_id,
       COALESCE(u.username, ''),
       COALESCE(u.email, ''),
       (u.id IS NULL OR u.deleted_at IS NOT NULL),
       NOT (COALESCE(ua.agent_level, 3) = 2 AND ua.agent_owner_id IS NOT DISTINCT FROM $1)
FROM (`+subAgentSetSQL("$1")+`) s
LEFT JOIN users u ON u.id = s.agent_id
LEFT JOIN user_affiliates ua ON ua.user_id = s.agent_id
WHERE s.agent_id = $2`,
		[]any{viewerID, agentID},
		&out.User.UserID, &out.User.Nickname, &out.User.Email, &out.User.Deleted, &out.Historical)
	if err != nil || !found {
		return nil, wrapTeamErr("get sub agent", err)
	}
	return &out, nil
}

func (r *affiliateTeamRepository) GetCustomer(ctx context.Context, viewerID, customerID int64) (*service.AffiliateTeamCustomerSubject, error) {
	var (
		out                   service.AffiliateTeamCustomerSubject
		ownerID               sql.NullInt64
		ownerName, ownerEmail string
		ownerDeleted          bool
		boundAt               sql.NullTime
	)
	found, err := r.queryOne(ctx, `
SELECT ua.user_id,
       COALESCE(u.username, ''),
       COALESCE(u.email, ''),
       (u.id IS NULL OR u.deleted_at IS NOT NULL),
       ua.agent_level,
       CASE WHEN ua.agent_owner_id = $1 THEN 'owner' ELSE 'upline' END,
       ua.agent_owner_id,
       COALESCE(ou.username, ''),
       COALESCE(ou.email, ''),
       (ou.id IS NULL OR ou.deleted_at IS NOT NULL),
       ua.agent_owner_bound_at,
       EXISTS (SELECT 1 FROM (`+subAgentSetSQL("$1")+`) s WHERE s.agent_id = ua.user_id),
       COALESCE((
           SELECT SUM(l.amount)
           FROM user_affiliate_ledger l
           WHERE l.user_id = $1
             AND l.source_user_id = ua.user_id
             AND l.reward_type IN `+affiliateTeamAgentRewardTypes+`
       ), 0)::double precision
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
LEFT JOIN users ou ON ou.id = ua.agent_owner_id
WHERE ua.user_id = $2 AND (ua.agent_owner_id = $1 OR ua.agent_upline_id = $1)`,
		[]any{viewerID, customerID},
		&out.User.UserID, &out.User.Nickname, &out.User.Email, &out.User.Deleted,
		&out.AgentLevel, &out.Relation,
		&ownerID, &ownerName, &ownerEmail, &ownerDeleted,
		&boundAt, &out.IsSubAgent, &out.LifetimeEarnings)
	if err != nil || !found {
		return nil, wrapTeamErr("get team customer", err)
	}
	if out.Relation == "upline" && ownerID.Valid {
		out.OwnerAgent = &service.AffiliateTeamUser{UserID: ownerID.Int64, Nickname: ownerName, Email: ownerEmail, Deleted: ownerDeleted}
	}
	if boundAt.Valid {
		out.BoundAt = &boundAt.Time
	}
	return &out, nil
}

// GetAggregates 一次扫描汇总表得出趋势、模型分布与按客户汇总（GROUPING SETS），
// 一次扫描分润流水得出按天收益；摘要由两者在 Go 里合计，避免对大团队重复扫描。
func (r *affiliateTeamRepository) GetAggregates(ctx context.Context, scope service.AffiliateTeamScope, rng service.AffiliateTeamRange) (service.AffiliateTeamAggregates, error) {
	out := service.AffiliateTeamAggregates{
		Trend:  make([]service.AffiliateTeamTrendPoint, 0),
		Models: make([]service.AffiliateTeamModelShare, 0),
	}
	args := &teamArgs{}
	viewer := args.add(scope.ViewerID)
	where := scopeSQL(args, viewer, scope)
	scopeCTE := `
WITH scope AS (
    SELECT ua.user_id, COALESCE(ua.agent_owner_id = ` + viewer + `, false) AS direct
    FROM user_affiliates ua WHERE ` + where + `
)`
	scopeArgs := len(args.values)
	d1, d2 := args.add(rng.StartDate), args.add(rng.EndDate)

	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, scopeCTE+`
SELECT GROUPING(r.bucket_date) = 0 AS by_day,
       GROUPING(r.model) = 0 AS by_model,
       COALESCE(r.bucket_date::text, ''),
       COALESCE(r.model, ''),
       BOOL_OR(s.direct),
       SUM(r.actual_cost)::double precision,
       SUM(r.requests)::bigint,
       SUM(r.input_tokens + r.output_tokens + r.cache_creation_tokens + r.cache_read_tokens)::bigint,
       (SELECT COUNT(*) FROM scope)
FROM usage_user_daily_rollups r
JOIN scope s ON s.user_id = r.user_id
WHERE r.bucket_date BETWEEN `+d1+`::date AND `+d2+`::date
GROUP BY GROUPING SETS ((r.bucket_date), (r.model), (r.user_id))`, args.values...)
	if err != nil {
		return out, fmt.Errorf("aggregate team usage: %w", err)
	}
	for rows.Next() {
		var (
			byDay, byModel, direct bool
			date, model            string
			cost                   float64
			requests, tokens       int64
		)
		if err := rows.Scan(&byDay, &byModel, &date, &model, &direct, &cost, &requests, &tokens, &out.Summary.Customers); err != nil {
			_ = rows.Close()
			return out, err
		}
		switch {
		case byDay:
			out.Trend = append(out.Trend, service.AffiliateTeamTrendPoint{Date: date, Cost: cost})
		case byModel:
			if cost > 0 {
				out.Models = append(out.Models, service.AffiliateTeamModelShare{Model: model, Cost: cost})
			}
		default: // 按客户
			out.Summary.Cost += cost
			if direct {
				out.Summary.DirectCost += cost
			} else {
				out.Summary.SubCost += cost
			}
			if cost > 0 {
				out.Summary.ActiveCustomers++
			}
			out.Summary.Requests += requests
			out.Summary.Tokens += tokens
		}
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}
	if out.Summary.Customers == 0 {
		// 区间内无消费时上面没有任何行，客户数单独取。
		if _, err := r.queryOne(ctx, scopeCTE+` SELECT COUNT(*) FROM scope`, args.values[:scopeArgs], &out.Summary.Customers); err != nil {
			return out, fmt.Errorf("count team scope: %w", err)
		}
	}
	sort.SliceStable(out.Models, func(i, j int) bool {
		if out.Models[i].Cost != out.Models[j].Cost {
			return out.Models[i].Cost > out.Models[j].Cost
		}
		return out.Models[i].Model < out.Models[j].Model
	})

	earnArgs := &teamArgs{values: append([]any{}, args.values[:scopeArgs]...)}
	t1, t2 := earnArgs.add(rng.Start), earnArgs.add(rng.End)
	rows, err = client.QueryContext(ctx, scopeCTE+`
SELECT (l.created_at AT TIME ZONE 'Asia/Shanghai')::date::text, SUM(l.amount)::double precision
FROM user_affiliate_ledger l
WHERE l.user_id = `+viewer+`
  AND l.reward_type IN `+affiliateTeamAgentRewardTypes+`
  AND l.created_at >= `+t1+` AND l.created_at < `+t2+`
  AND l.source_user_id IN (SELECT user_id FROM scope)
GROUP BY 1`, earnArgs.values...)
	if err != nil {
		return out, fmt.Errorf("aggregate team earnings: %w", err)
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
	for date, v := range earnings { // 有收益但当天无消费
		out.Trend = append(out.Trend, service.AffiliateTeamTrendPoint{Date: date, Earnings: v})
	}
	return out, nil
}

func (r *affiliateTeamRepository) GetCustomerFilters(ctx context.Context, customerID int64, rng service.AffiliateTeamRange) (service.AffiliateTeamFilters, error) {
	out := service.AffiliateTeamFilters{Models: []string{}, Groups: []service.AffiliateTeamGroup{}}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT DISTINCT r.model
FROM usage_user_daily_rollups r
WHERE r.user_id = $1 AND r.bucket_date BETWEEN $2::date AND $3::date
ORDER BY 1`, customerID, rng.StartDate, rng.EndDate)
	if err != nil {
		return out, fmt.Errorf("list customer models: %w", err)
	}
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			_ = rows.Close()
			return out, err
		}
		out.Models = append(out.Models, model)
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}

	rows, err = client.QueryContext(ctx, `
SELECT g.group_id, COALESCE(gr.name, '')
FROM (
    SELECT DISTINCT r.group_id
    FROM usage_user_daily_rollups r
    WHERE r.user_id = $1 AND r.bucket_date BETWEEN $2::date AND $3::date
) g
LEFT JOIN groups gr ON gr.id = g.group_id
ORDER BY g.group_id`, customerID, rng.StartDate, rng.EndDate)
	if err != nil {
		return out, fmt.Errorf("list customer groups: %w", err)
	}
	for rows.Next() {
		var group service.AffiliateTeamGroup
		if err := rows.Scan(&group.ID, &group.Name); err != nil {
			_ = rows.Close()
			return out, err
		}
		out.Groups = append(out.Groups, group)
	}
	return out, closeRows(rows)
}

var affiliateTeamCustomerSorts = map[string]string{
	service.AffiliateTeamSortCost:       "entries.cost",
	service.AffiliateTeamSortRequests:   "entries.requests",
	service.AffiliateTeamSortEarnings:   "entries.earnings",
	service.AffiliateTeamSortLastUsedAt: "entries.last_used_at",
}

func (r *affiliateTeamRepository) ListCustomers(ctx context.Context, viewerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamCustomerEntry, int64, error) {
	args := &teamArgs{}
	viewer := args.add(viewerID)
	where := "ua.agent_owner_id = " + viewer
	if query.AgentID > 0 {
		where = "ua.agent_owner_id = " + args.add(query.AgentID) + " AND ua.agent_upline_id = " + viewer
	}
	searchSQL := "TRUE"
	switch {
	case query.Search != "" && query.AdminSearch:
		searchSQL = teamAdminSearchSQL(args, query.Search)
	case query.Search != "":
		searchSQL = "NOT entries.deleted AND (entries.nickname ILIKE " + args.add("%"+escapeLike(query.Search)+"%") +
			` ESCAPE '\' OR LOWER(entries.email) = LOWER(` + args.add(query.Search) + "))"
	}
	baseArgs := len(args.values)
	d1, d2 := args.add(rng.StartDate), args.add(rng.EndDate)
	t1, t2 := args.add(rng.Start), args.add(rng.End)

	rowsCTE := `
WITH scope AS (
    SELECT ua.user_id, ua.agent_level FROM user_affiliates ua WHERE ` + where + `
), usage AS (
    SELECT r.user_id,
           SUM(r.actual_cost) FILTER (WHERE r.bucket_date BETWEEN ` + d1 + `::date AND ` + d2 + `::date) AS cost,
           SUM(r.requests) FILTER (WHERE r.bucket_date BETWEEN ` + d1 + `::date AND ` + d2 + `::date) AS requests,
           MAX(r.last_used_at) AS last_used_at
    FROM usage_user_daily_rollups r
    JOIN scope s ON s.user_id = r.user_id
    GROUP BY r.user_id
), earn AS (
    SELECT l.source_user_id AS user_id, SUM(l.amount) AS earnings
    FROM user_affiliate_ledger l
    WHERE l.user_id = ` + viewer + `
      AND l.reward_type IN ` + affiliateTeamAgentRewardTypes + `
      AND l.created_at >= ` + t1 + ` AND l.created_at < ` + t2 + `
      AND l.source_user_id IN (SELECT user_id FROM scope)
    GROUP BY l.source_user_id
), entries AS (
    SELECT s.user_id,
           COALESCE(u.username, '') AS nickname,
           COALESCE(u.email, '') AS email,
           (u.id IS NULL OR u.deleted_at IS NOT NULL) AS deleted,
           s.agent_level,
           COALESCE(usage.cost, 0) AS cost,
           COALESCE(usage.requests, 0) AS requests,
           COALESCE(earn.earnings, 0) AS earnings,
           usage.last_used_at
    FROM scope s
    LEFT JOIN users u ON u.id = s.user_id
    LEFT JOIN usage ON usage.user_id = s.user_id
    LEFT JOIN earn ON earn.user_id = s.user_id
), total AS (
    SELECT COALESCE(SUM(cost), 0) AS cost FROM entries
)`

	var total int64
	if _, err := r.queryOne(ctx, `
WITH scope AS (
    SELECT ua.user_id FROM user_affiliates ua WHERE `+where+`
), entries AS (
    SELECT s.user_id,
           COALESCE(u.username, '') AS nickname,
           COALESCE(u.email, '') AS email,
           (u.id IS NULL OR u.deleted_at IS NOT NULL) AS deleted
    FROM scope s
    LEFT JOIN users u ON u.id = s.user_id
)
SELECT COUNT(*) FROM entries WHERE `+searchSQL, args.values[:baseArgs], &total); err != nil {
		return nil, 0, fmt.Errorf("count team customers: %w", err)
	}

	order := affiliateTeamCustomerSorts[query.SortBy]
	if order == "" {
		order = affiliateTeamCustomerSorts[service.AffiliateTeamSortCost]
	}
	limit, offset := args.add(query.PageSize), args.add((query.Page-1)*query.PageSize)
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, rowsCTE+`
SELECT entries.user_id, entries.nickname, entries.email, entries.deleted, entries.agent_level,
       entries.cost::double precision,
       entries.requests::bigint,
       entries.earnings::double precision,
       entries.last_used_at,
       CASE WHEN total.cost > 0 THEN (entries.cost / total.cost)::double precision ELSE 0 END
FROM entries, total
WHERE `+searchSQL+`
ORDER BY `+order+` `+sortDirection(query.SortDesc)+` NULLS LAST, entries.user_id DESC
LIMIT `+limit+` OFFSET `+offset, args.values...)
	if err != nil {
		return nil, 0, fmt.Errorf("list team customers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateTeamCustomerEntry, 0, query.PageSize)
	for rows.Next() {
		var (
			item     service.AffiliateTeamCustomerEntry
			lastUsed sql.NullTime
		)
		if err := rows.Scan(&item.User.UserID, &item.User.Nickname, &item.User.Email, &item.User.Deleted,
			&item.AgentLevel, &item.Cost, &item.Requests, &item.Earnings, &lastUsed, &item.CostShare); err != nil {
			return nil, 0, err
		}
		if lastUsed.Valid {
			item.LastUsedAt = &lastUsed.Time
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

var affiliateTeamAgentSorts = map[string]string{
	service.AffiliateTeamSortTeamCost:        "entries.team_cost",
	service.AffiliateTeamSortEarnings:        "entries.earnings",
	service.AffiliateTeamSortActiveCustomers: "entries.active_customers",
}

func (r *affiliateTeamRepository) ListAgents(ctx context.Context, viewerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamAgentEntry, int64, error) {
	var total int64
	if _, err := r.queryOne(ctx, `SELECT COUNT(*) FROM (`+subAgentSetSQL("$1")+`) s`, []any{viewerID}, &total); err != nil {
		return nil, 0, fmt.Errorf("count sub agents: %w", err)
	}

	args := &teamArgs{}
	viewer := args.add(viewerID)
	d1, d2 := args.add(rng.StartDate), args.add(rng.EndDate)
	t1, t2 := args.add(rng.Start), args.add(rng.End)
	order := affiliateTeamAgentSorts[query.SortBy]
	if order == "" {
		order = affiliateTeamAgentSorts[service.AffiliateTeamSortTeamCost]
	}
	limit, offset := args.add(query.PageSize), args.add((query.Page-1)*query.PageSize)
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
WITH agents AS (`+subAgentSetSQL(viewer)+`
), members AS (
    SELECT ua.user_id, ua.agent_owner_id AS agent_id
    FROM user_affiliates ua
    WHERE ua.agent_upline_id = `+viewer+`
), usage AS (
    SELECT r.user_id, SUM(r.actual_cost) AS cost
    FROM usage_user_daily_rollups r
    JOIN members m ON m.user_id = r.user_id
    WHERE r.bucket_date BETWEEN `+d1+`::date AND `+d2+`::date
    GROUP BY r.user_id
), earn AS (
    SELECT l.source_user_id AS user_id, SUM(l.amount) AS earnings
    FROM user_affiliate_ledger l
    WHERE l.user_id = `+viewer+`
      AND l.reward_type IN `+affiliateTeamAgentRewardTypes+`
      AND l.created_at >= `+t1+` AND l.created_at < `+t2+`
      AND l.source_user_id IN (SELECT user_id FROM members)
    GROUP BY l.source_user_id
), per_agent AS (
    SELECT m.agent_id,
           COUNT(*) AS customers,
           COUNT(*) FILTER (WHERE usage.cost > 0) AS active_customers,
           COALESCE(SUM(usage.cost), 0) AS team_cost,
           COALESCE(SUM(earn.earnings), 0) AS earnings
    FROM members m
    LEFT JOIN usage ON usage.user_id = m.user_id
    LEFT JOIN earn ON earn.user_id = m.user_id
    GROUP BY m.agent_id
), entries AS (
    SELECT a.agent_id,
           COALESCE(u.username, '') AS nickname,
           COALESCE(u.email, '') AS email,
           (u.id IS NULL OR u.deleted_at IS NOT NULL) AS deleted,
           NOT (COALESCE(aua.agent_level, 3) = 2 AND aua.agent_owner_id IS NOT DISTINCT FROM `+viewer+`) AS historical,
           COALESCE(p.customers, 0) AS customers,
           COALESCE(p.active_customers, 0) AS active_customers,
           COALESCE(p.team_cost, 0) AS team_cost,
           COALESCE(p.earnings, 0) AS earnings
    FROM agents a
    LEFT JOIN per_agent p ON p.agent_id = a.agent_id
    LEFT JOIN users u ON u.id = a.agent_id
    LEFT JOIN user_affiliates aua ON aua.user_id = a.agent_id
), total AS (
    SELECT COALESCE(SUM(team_cost), 0) AS cost FROM entries
)
SELECT entries.agent_id, entries.nickname, entries.email, entries.deleted, entries.historical,
       entries.customers, entries.active_customers,
       entries.team_cost::double precision,
       entries.earnings::double precision,
       CASE WHEN total.cost > 0 THEN (entries.team_cost / total.cost)::double precision ELSE 0 END
FROM entries, total
ORDER BY `+order+` `+sortDirection(query.SortDesc)+` NULLS LAST, entries.agent_id DESC
LIMIT `+limit+` OFFSET `+offset, args.values...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sub agents: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateTeamAgentEntry, 0, query.PageSize)
	for rows.Next() {
		var item service.AffiliateTeamAgentEntry
		if err := rows.Scan(&item.User.UserID, &item.User.Nickname, &item.User.Email, &item.User.Deleted, &item.Historical,
			&item.Customers, &item.ActiveCustomers, &item.TeamCost, &item.Earnings, &item.CostShare); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

const affiliateTeamRequestModelExpr = `COALESCE(NULLIF(BTRIM(ul.requested_model), ''), ul.model)`

func (r *affiliateTeamRepository) ListRequests(ctx context.Context, customerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamRequest, int64, error) {
	args := &teamArgs{}
	conds := []string{
		"ul.user_id = " + args.add(customerID),
		"ul.billing_type = 0",
		"ul.created_at >= " + args.add(rng.Start),
		"ul.created_at < " + args.add(rng.End),
	}
	if query.Model != "" {
		conds = append(conds, affiliateTeamRequestModelExpr+" = "+args.add(query.Model))
	}
	if query.GroupID != nil {
		conds = append(conds, "COALESCE(ul.group_id, 0) = "+args.add(*query.GroupID))
	}
	where := strings.Join(conds, " AND ")

	var total int64
	if _, err := r.queryOne(ctx, `SELECT COUNT(*) FROM usage_logs ul WHERE `+where, args.values, &total); err != nil {
		return nil, 0, fmt.Errorf("count customer requests: %w", err)
	}
	limit, offset := args.add(query.PageSize), args.add((query.Page-1)*query.PageSize)
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `
SELECT ul.created_at,
       `+affiliateTeamRequestModelExpr+`,
       COALESCE(g.name, ''),
       COALESCE(ul.billing_mode, ''),
       ul.input_tokens, ul.output_tokens, ul.cache_creation_tokens, ul.cache_read_tokens,
       COALESCE(ul.image_count, 0), ul.video_count,
       ul.actual_cost::double precision
FROM usage_logs ul
LEFT JOIN groups g ON g.id = ul.group_id
WHERE `+where+`
ORDER BY ul.created_at DESC, ul.id DESC
LIMIT `+limit+` OFFSET `+offset, args.values...)
	if err != nil {
		return nil, 0, fmt.Errorf("list customer requests: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AffiliateTeamRequest, 0, query.PageSize)
	for rows.Next() {
		var item service.AffiliateTeamRequest
		if err := rows.Scan(&item.CreatedAt, &item.Model, &item.GroupName, &item.BillingMode,
			&item.InputTokens, &item.OutputTokens, &item.CacheCreationTokens, &item.CacheReadTokens,
			&item.ImageCount, &item.VideoCount, &item.Cost); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func sortDirection(desc bool) string {
	if desc {
		return "DESC"
	}
	return "ASC"
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	return err
}

func wrapTeamErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}
