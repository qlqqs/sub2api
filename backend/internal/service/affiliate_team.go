package service

import (
	"context"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 代理团队消费页：按归属快照（agent_owner_id / agent_upline_id）查看客户与下级代理团队的
// 余额消费和代理分润。多客户聚合读 usage_user_daily_rollups（迁移 251），单客户明细读 usage_logs。

const (
	affiliateTeamDateLayout       = "2006-01-02"
	affiliateTeamDefaultRangeDays = 7
	affiliateTeamMaxRangeDays     = 90
	affiliateTeamDefaultPageSize  = 20
	affiliateTeamMaxPageSize      = 100
	affiliateTeamTopModels        = 5
)

// AffiliateTeamLocation 与迁移 251 的触发器一致：按北京时间自然日分桶。中国无夏令时，用固定时区，
// 不依赖运行环境的 tzdata。
var AffiliateTeamLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// UsageUserRollupDate 返回某时刻所在的北京时间日期（YYYY-MM-DD），即 usage_user_daily_rollups.bucket_date。
func UsageUserRollupDate(at time.Time) string {
	return at.In(AffiliateTeamLocation).Format(affiliateTeamDateLayout)
}

var (
	ErrAffiliateTeamForbidden    = infraerrors.Forbidden("AFFILIATE_TEAM_FORBIDDEN", "team consumption is only available to agents")
	ErrAffiliateTeamNotFound     = infraerrors.NotFound("AFFILIATE_TEAM_NOT_FOUND", "customer or agent not found")
	ErrAffiliateTeamInvalidRange = infraerrors.BadRequest("AFFILIATE_TEAM_INVALID_RANGE", "date range must cover 1 to 90 days")
	ErrAffiliateTeamUnavailable  = infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
)

// 列表排序白名单；repository 再映射为 SQL 表达式。
const (
	AffiliateTeamSortCost            = "cost"
	AffiliateTeamSortRequests        = "requests"
	AffiliateTeamSortEarnings        = "earnings"
	AffiliateTeamSortLastUsedAt      = "last_used_at"
	AffiliateTeamSortTeamCost        = "team_cost"
	AffiliateTeamSortActiveCustomers = "active_customers"
)

// AffiliateTeamRange 是闭区间日期 [StartDate, EndDate]；Start/End 为对应的半开时间区间 [Start, End)。
type AffiliateTeamRange struct {
	StartDate string
	EndDate   string
	Start     time.Time
	End       time.Time
}

// AffiliateTeamScope 描述查询范围：默认是整个团队；AgentID > 0 时为该二级代理在我下面的团队；
// CustomerID > 0 时为单个客户。
type AffiliateTeamScope struct {
	ViewerID   int64
	AgentID    int64
	CustomerID int64
}

type AffiliateTeamListQuery struct {
	AgentID  int64
	Search   string
	SortBy   string
	SortDesc bool
	Page     int
	PageSize int
	Model    string
	GroupID  *int64
	// Level 只用于管理端汇总页：AffiliateTeamLevel* 之一，空串为全部。
	Level string
	// AdminSearch 由 service 按受众设置：管理端搜索为昵称 / 邮箱模糊，纯数字时另按用户 ID 精确匹配。
	AdminSearch bool
}

// AffiliateTeamUser 是团队页上的用户：昵称 + 邮箱（代理端脱敏，管理端原文）；账号已删除时两者都为空。
type AffiliateTeamUser struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Deleted  bool   `json:"deleted"`
}

type AffiliateTeamViewer struct {
	AgentLevel            int  `json:"agent_level"`
	Downgraded            bool `json:"downgraded"`
	SettleIntervalMinutes int  `json:"settle_interval_minutes"`
}

type AffiliateTeamCounts struct {
	DirectCustomers int64 `json:"direct_customers"`
	SubAgents       int64 `json:"sub_agents"`
}

type AffiliateTeamAgentSubject struct {
	User       AffiliateTeamUser `json:"user"`
	Historical bool              `json:"historical"`
}

type AffiliateTeamCustomerSubject struct {
	User       AffiliateTeamUser `json:"user"`
	AgentLevel int               `json:"agent_level"`
	// Relation：owner 我的客户；upline 某二级代理（OwnerAgent）的客户。
	Relation         string             `json:"relation"`
	OwnerAgent       *AffiliateTeamUser `json:"owner_agent"`
	IsSubAgent       bool               `json:"is_sub_agent"`
	BoundAt          *time.Time         `json:"bound_at"`
	LifetimeEarnings float64            `json:"lifetime_earnings"`
}

type AffiliateTeamSummary struct {
	Cost            float64 `json:"cost"`
	DirectCost      float64 `json:"direct_cost"`
	SubCost         float64 `json:"sub_cost"`
	Earnings        float64 `json:"earnings"`
	ActiveCustomers int64   `json:"active_customers"`
	Customers       int64   `json:"customers"`
	Requests        int64   `json:"requests"`
	Tokens          int64   `json:"tokens"`
}

type AffiliateTeamTrendPoint struct {
	Date     string  `json:"date"`
	Cost     float64 `json:"cost"`
	Earnings float64 `json:"earnings"`
}

// AffiliateTeamModelShare 中 Model 为空串表示「其他」。
type AffiliateTeamModelShare struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
	Share float64 `json:"share"`
}

type AffiliateTeamGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AffiliateTeamFilters struct {
	Models []string             `json:"models"`
	Groups []AffiliateTeamGroup `json:"groups"`
}

type AffiliateTeamOverview struct {
	Viewer   AffiliateTeamViewer           `json:"viewer"`
	Counts   *AffiliateTeamCounts          `json:"counts"`
	Agent    *AffiliateTeamAgentSubject    `json:"agent"`
	Customer *AffiliateTeamCustomerSubject `json:"customer"`
	Summary  AffiliateTeamSummary          `json:"summary"`
	Trend    []AffiliateTeamTrendPoint     `json:"trend"`
	Models   []AffiliateTeamModelShare     `json:"models"`
	Filters  *AffiliateTeamFilters         `json:"filters"`
	// Subject 只在管理端返回：被查看的代理本人。
	Subject *AffiliateTeamViewerSubject `json:"subject,omitempty"`
}

type AffiliateTeamCustomerEntry struct {
	User       AffiliateTeamUser `json:"user"`
	AgentLevel int               `json:"agent_level"`
	Cost       float64           `json:"cost"`
	Requests   int64             `json:"requests"`
	Earnings   float64           `json:"earnings"`
	LastUsedAt *time.Time        `json:"last_used_at"`
	CostShare  float64           `json:"cost_share"`
}

type AffiliateTeamAgentEntry struct {
	User            AffiliateTeamUser `json:"user"`
	Historical      bool              `json:"historical"`
	Customers       int64             `json:"customers"`
	ActiveCustomers int64             `json:"active_customers"`
	TeamCost        float64           `json:"team_cost"`
	Earnings        float64           `json:"earnings"`
	CostShare       float64           `json:"cost_share"`
}

type AffiliateTeamRequest struct {
	CreatedAt           time.Time `json:"created_at"`
	Model               string    `json:"model"`
	GroupName           string    `json:"group_name"`
	BillingMode         string    `json:"billing_mode"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheCreationTokens int64     `json:"cache_creation_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	ImageCount          int64     `json:"image_count"`
	VideoCount          int64     `json:"video_count"`
	Cost                float64   `json:"cost"`
}

// AffiliateTeamAggregates 是一个范围在区间内的聚合结果。
type AffiliateTeamAggregates struct {
	Summary AffiliateTeamSummary
	Trend   []AffiliateTeamTrendPoint
	Models  []AffiliateTeamModelShare
}

// AffiliateTeamViewerState 是准入判断所需的原始数据。
type AffiliateTeamViewerState struct {
	AgentLevel        int
	HasOwnedCustomers bool
	HasUplineCustomer bool
}

// AffiliateTeamRepository 是团队消费页的数据端口；返回的邮箱均为原文，由 service 按受众脱敏。
type AffiliateTeamRepository interface {
	GetViewerState(ctx context.Context, userID int64) (AffiliateTeamViewerState, error)
	CountTeam(ctx context.Context, viewerID int64) (AffiliateTeamCounts, error)
	// GetSubAgent 返回下级代理集合中的 agentID；不在集合中返回 nil。
	GetSubAgent(ctx context.Context, viewerID, agentID int64) (*AffiliateTeamAgentSubject, error)
	// GetCustomer 返回团队中的客户；不在团队中返回 nil。
	GetCustomer(ctx context.Context, viewerID, customerID int64) (*AffiliateTeamCustomerSubject, error)
	// GetAggregates 返回摘要、趋势（只含有数据的日期，service 补齐）与全部模型（按消费降序）。
	GetAggregates(ctx context.Context, scope AffiliateTeamScope, rng AffiliateTeamRange) (AffiliateTeamAggregates, error)
	GetCustomerFilters(ctx context.Context, customerID int64, rng AffiliateTeamRange) (AffiliateTeamFilters, error)
	// ListCustomers：query.AgentID > 0 列该二级代理在我下面的团队，否则列「我的客户」。
	ListCustomers(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamCustomerEntry, int64, error)
	ListAgents(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamAgentEntry, int64, error)
	ListRequests(ctx context.Context, customerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error)

	// 以下只用于管理端。
	// GetViewerSubject 返回被查看代理的展示信息；累计分润读分润日汇总（迁移 252）。
	GetViewerSubject(ctx context.Context, userID int64) (AffiliateTeamViewerSubject, error)
	// GetPlatformAggregates 返回汇总页的计数、摘要、趋势（只含有数据的日期）与全部模型（按消费降序）。
	GetPlatformAggregates(ctx context.Context, rng AffiliateTeamRange) (AffiliateTeamPlatformAggregates, error)
	// ListTeamOwners 列出全部有团队的人（汇总页的行）。
	ListTeamOwners(ctx context.Context, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamOwnerEntry, int64, error)
}

// affiliateTeamAudience 区分同一套查询的两种查看者：代理本人，或在后台查看某代理视角的管理员。
type affiliateTeamAudience int

const (
	affiliateTeamAudienceAgent affiliateTeamAudience = iota
	affiliateTeamAudienceAdmin
)

type AffiliateTeamService struct {
	repo           AffiliateTeamRepository
	settingService *SettingService
}

func NewAffiliateTeamService(repo AffiliateTeamRepository, settingService *SettingService) *AffiliateTeamService {
	return &AffiliateTeamService{repo: repo, settingService: settingService}
}

// ParseAffiliateTeamRange 解析北京时间日期区间；两端为空时默认最近 7 天（含今天）。
func ParseAffiliateTeamRange(startDate, endDate string, now time.Time) (AffiliateTeamRange, error) {
	startDate, endDate = strings.TrimSpace(startDate), strings.TrimSpace(endDate)
	today := now.In(AffiliateTeamLocation)
	if startDate == "" && endDate == "" {
		endDate = today.Format(affiliateTeamDateLayout)
		startDate = today.AddDate(0, 0, -(affiliateTeamDefaultRangeDays - 1)).Format(affiliateTeamDateLayout)
	}
	start, err := time.ParseInLocation(affiliateTeamDateLayout, startDate, AffiliateTeamLocation)
	if err != nil {
		return AffiliateTeamRange{}, ErrAffiliateTeamInvalidRange
	}
	end, err := time.ParseInLocation(affiliateTeamDateLayout, endDate, AffiliateTeamLocation)
	if err != nil {
		return AffiliateTeamRange{}, ErrAffiliateTeamInvalidRange
	}
	days := int(end.Sub(start).Hours()/24) + 1
	if days < 1 || days > affiliateTeamMaxRangeDays {
		return AffiliateTeamRange{}, ErrAffiliateTeamInvalidRange
	}
	return AffiliateTeamRange{
		StartDate: start.Format(affiliateTeamDateLayout),
		EndDate:   end.Format(affiliateTeamDateLayout),
		Start:     start,
		End:       end.AddDate(0, 0, 1),
	}, nil
}

// Overview 返回摘要、趋势、模型分布；agentID / customerID 二选一，均为 0 时为团队总览。
func (s *AffiliateTeamService) Overview(ctx context.Context, viewerID, agentID, customerID int64, rng AffiliateTeamRange) (*AffiliateTeamOverview, error) {
	return s.overview(ctx, affiliateTeamAudienceAgent, viewerID, agentID, customerID, rng)
}

func (s *AffiliateTeamService) overview(ctx context.Context, audience affiliateTeamAudience, viewerID, agentID, customerID int64, rng AffiliateTeamRange) (*AffiliateTeamOverview, error) {
	viewer, err := s.resolveViewer(ctx, audience, viewerID)
	if err != nil {
		return nil, err
	}
	if agentID > 0 && customerID > 0 {
		return nil, infraerrors.BadRequest("AFFILIATE_TEAM_INVALID_SCOPE", "agent_id and user_id are mutually exclusive")
	}
	out := &AffiliateTeamOverview{Viewer: viewer}
	scope := AffiliateTeamScope{ViewerID: viewerID, AgentID: agentID, CustomerID: customerID}
	switch {
	case agentID > 0:
		agent, err := s.repo.GetSubAgent(ctx, viewerID, agentID)
		if err != nil {
			return nil, err
		}
		if agent == nil {
			return nil, ErrAffiliateTeamNotFound
		}
		agent.User = presentAffiliateTeamUser(audience, agent.User)
		out.Agent = agent
	case customerID > 0:
		customer, err := s.repo.GetCustomer(ctx, viewerID, customerID)
		if err != nil {
			return nil, err
		}
		if customer == nil {
			return nil, ErrAffiliateTeamNotFound
		}
		customer.User = presentAffiliateTeamUser(audience, customer.User)
		customer.AgentLevel = normalizeAgentLevel(customer.AgentLevel)
		if customer.OwnerAgent != nil {
			owner := presentAffiliateTeamUser(audience, *customer.OwnerAgent)
			customer.OwnerAgent = &owner
		}
		out.Customer = customer
		filters, err := s.repo.GetCustomerFilters(ctx, customerID, rng)
		if err != nil {
			return nil, err
		}
		out.Filters = &filters
	default:
		counts, err := s.repo.CountTeam(ctx, viewerID)
		if err != nil {
			return nil, err
		}
		out.Counts = &counts
	}

	agg, err := s.repo.GetAggregates(ctx, scope, rng)
	if err != nil {
		return nil, err
	}
	out.Summary = agg.Summary
	out.Trend = fillAffiliateTeamTrend(agg.Trend, rng)
	out.Models = topAffiliateTeamModels(agg.Models, affiliateTeamTopModels)
	if audience == affiliateTeamAudienceAdmin {
		subject, err := s.repo.GetViewerSubject(ctx, viewerID)
		if err != nil {
			return nil, err
		}
		subject.User = presentAffiliateTeamUser(audience, subject.User)
		if subject.Upline != nil {
			upline := presentAffiliateTeamUser(audience, *subject.Upline)
			subject.Upline = &upline
		}
		out.Subject = &subject
	}
	return out, nil
}

// ListCustomers 列「我的客户」，或 agentID 指定的二级代理在我下面的团队。
func (s *AffiliateTeamService) ListCustomers(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamCustomerEntry, int64, error) {
	return s.listCustomers(ctx, affiliateTeamAudienceAgent, viewerID, rng, query)
}

func (s *AffiliateTeamService) listCustomers(ctx context.Context, audience affiliateTeamAudience, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamCustomerEntry, int64, error) {
	if _, err := s.resolveViewer(ctx, audience, viewerID); err != nil {
		return nil, 0, err
	}
	if query.AgentID > 0 {
		agent, err := s.repo.GetSubAgent(ctx, viewerID, query.AgentID)
		if err != nil {
			return nil, 0, err
		}
		if agent == nil {
			return nil, 0, ErrAffiliateTeamNotFound
		}
	}
	query = normalizeAffiliateTeamListQuery(query, AffiliateTeamSortCost, AffiliateTeamSortCost, AffiliateTeamSortRequests, AffiliateTeamSortEarnings, AffiliateTeamSortLastUsedAt)
	query.AdminSearch = audience == affiliateTeamAudienceAdmin
	items, total, err := s.repo.ListCustomers(ctx, viewerID, rng, query)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].User = presentAffiliateTeamUser(audience, items[i].User)
		items[i].AgentLevel = normalizeAgentLevel(items[i].AgentLevel)
	}
	return items, total, nil
}

func (s *AffiliateTeamService) ListAgents(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamAgentEntry, int64, error) {
	return s.listAgents(ctx, affiliateTeamAudienceAgent, viewerID, rng, query)
}

func (s *AffiliateTeamService) listAgents(ctx context.Context, audience affiliateTeamAudience, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamAgentEntry, int64, error) {
	if _, err := s.resolveViewer(ctx, audience, viewerID); err != nil {
		return nil, 0, err
	}
	query = normalizeAffiliateTeamListQuery(query, AffiliateTeamSortTeamCost, AffiliateTeamSortTeamCost, AffiliateTeamSortEarnings, AffiliateTeamSortActiveCustomers)
	items, total, err := s.repo.ListAgents(ctx, viewerID, rng, query)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].User = presentAffiliateTeamUser(audience, items[i].User)
	}
	return items, total, nil
}

func (s *AffiliateTeamService) ListRequests(ctx context.Context, viewerID, customerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error) {
	return s.listRequests(ctx, affiliateTeamAudienceAgent, viewerID, customerID, rng, query)
}

func (s *AffiliateTeamService) listRequests(ctx context.Context, audience affiliateTeamAudience, viewerID, customerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error) {
	if _, err := s.resolveViewer(ctx, audience, viewerID); err != nil {
		return nil, 0, err
	}
	customer, err := s.repo.GetCustomer(ctx, viewerID, customerID)
	if err != nil {
		return nil, 0, err
	}
	if customer == nil {
		return nil, 0, ErrAffiliateTeamNotFound
	}
	query = normalizeAffiliateTeamListQuery(query, "", "")
	query.Model = strings.TrimSpace(query.Model)
	return s.repo.ListRequests(ctx, customerID, rng, query)
}

// HasTeamAccess 判断用户能否进入团队消费页：当前是代理，或名下仍有快照客户（被降级者看老客户）。
func (s *AffiliateTeamService) HasTeamAccess(ctx context.Context, userID int64) (bool, error) {
	_, err := s.resolveViewer(ctx, affiliateTeamAudienceAgent, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrAffiliateTeamForbidden) {
		return false, nil
	}
	return false, err
}

// resolveViewer 做准入：代理端受功能开关约束，没有团队时 403；管理端不看开关，没有团队时 404。
func (s *AffiliateTeamService) resolveViewer(ctx context.Context, audience affiliateTeamAudience, userID int64) (AffiliateTeamViewer, error) {
	if s == nil || s.repo == nil || (audience == affiliateTeamAudienceAdmin && s.settingService == nil) {
		return AffiliateTeamViewer{}, ErrAffiliateTeamUnavailable
	}
	if audience == affiliateTeamAudienceAgent && (s.settingService == nil || !s.settingService.IsAffiliateEnabled(ctx)) {
		return AffiliateTeamViewer{}, ErrAffiliateTeamForbidden
	}
	state, err := s.repo.GetViewerState(ctx, userID)
	if err != nil {
		return AffiliateTeamViewer{}, err
	}
	level := normalizeAgentLevel(state.AgentLevel)
	isAgent := isAgentLevel(level)
	if !isAgent && !state.HasOwnedCustomers && !state.HasUplineCustomer {
		if audience == affiliateTeamAudienceAdmin {
			return AffiliateTeamViewer{}, ErrAffiliateTeamNotFound
		}
		return AffiliateTeamViewer{}, ErrAffiliateTeamForbidden
	}
	return AffiliateTeamViewer{
		AgentLevel:            level,
		Downgraded:            !isAgent,
		SettleIntervalMinutes: s.settingService.GetAffiliateSettleIntervalMinutes(ctx),
	}, nil
}

// NormalizeAffiliatePaging 校正用户端邀请/团队列表的页码与每页条数（默认 20，最大 100）；handler 也用它保证分页响应与查询一致。
func NormalizeAffiliatePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = affiliateTeamDefaultPageSize
	}
	if pageSize > affiliateTeamMaxPageSize {
		pageSize = affiliateTeamMaxPageSize
	}
	return page, pageSize
}

// normalizeAffiliateTeamListQuery 校正分页与排序；排序键不在白名单时回落到 defaultSort。
func normalizeAffiliateTeamListQuery(query AffiliateTeamListQuery, defaultSort string, allowed ...string) AffiliateTeamListQuery {
	query.Page, query.PageSize = NormalizeAffiliatePaging(query.Page, query.PageSize)
	query.Search = strings.TrimSpace(query.Search)
	sortBy := defaultSort
	for _, key := range allowed {
		if query.SortBy == key {
			sortBy = key
			break
		}
	}
	query.SortBy = sortBy
	return query
}

// presentAffiliateTeamUser 按受众展示用户：代理端邮箱脱敏，管理端原文；已删除账号只留 ID。
func presentAffiliateTeamUser(audience affiliateTeamAudience, user AffiliateTeamUser) AffiliateTeamUser {
	if user.Deleted {
		return AffiliateTeamUser{UserID: user.UserID, Deleted: true}
	}
	user.Nickname = strings.TrimSpace(user.Nickname)
	if audience != affiliateTeamAudienceAdmin {
		user.Email = maskEmail(user.Email)
	}
	return user
}

func fillAffiliateTeamTrend(points []AffiliateTeamTrendPoint, rng AffiliateTeamRange) []AffiliateTeamTrendPoint {
	byDate := make(map[string]AffiliateTeamTrendPoint, len(points))
	for _, p := range points {
		byDate[p.Date] = p
	}
	out := make([]AffiliateTeamTrendPoint, 0, affiliateTeamMaxRangeDays)
	for day := rng.Start; day.Before(rng.End); day = day.AddDate(0, 0, 1) {
		date := day.Format(affiliateTeamDateLayout)
		p := byDate[date]
		p.Date = date
		out = append(out, p)
	}
	return out
}

// topAffiliateTeamModels 保留消费最高的 n 个模型，其余合并为 Model 为空串的「其他」，并计算占比。
func topAffiliateTeamModels(models []AffiliateTeamModelShare, n int) []AffiliateTeamModelShare {
	total := 0.0
	for _, m := range models {
		total += m.Cost
	}
	out := make([]AffiliateTeamModelShare, 0, n+1)
	other := 0.0
	for i, m := range models {
		if i < n {
			out = append(out, m)
			continue
		}
		other += m.Cost
	}
	if other > 0 {
		out = append(out, AffiliateTeamModelShare{Cost: other})
	}
	for i := range out {
		if total > 0 {
			out[i].Share = out[i].Cost / total
		}
	}
	return out
}
