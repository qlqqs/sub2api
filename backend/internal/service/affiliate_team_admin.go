package service

import "context"

// 管理端团队消费页：汇总页列出全部有团队的人；代理视角复用代理端的全部查询，以被查看的代理为根
// （AdminViewer*），只在受众上不同：不看功能开关、没有团队时 404、邮箱不脱敏、搜索支持邮箱与 ID、overview 附 subject。

// 汇总页的等级筛选（AffiliateTeamListQuery.Level）。
const (
	AffiliateTeamLevelAll        = ""
	AffiliateTeamLevel1          = "1"
	AffiliateTeamLevel2          = "2"
	AffiliateTeamLevelDowngraded = "downgraded"
)

// AffiliateTeamViewerSubject 是管理端代理视角中被查看的代理本人（各层 overview 都返回，用于面包屑与元信息行）。
type AffiliateTeamViewerSubject struct {
	User AffiliateTeamUser `json:"user"`
	// Upline 只对当前二级代理有值：其归属的一级代理。
	Upline *AffiliateTeamUser `json:"upline"`
	// LifetimeEarnings 是全部历史的代理分润，与代理管理列表的「累计分润」一致。
	LifetimeEarnings float64 `json:"lifetime_earnings"`
}

type AffiliateTeamOwnerCounts struct {
	All        int64 `json:"all"`
	Level1     int64 `json:"level1"`
	Level2     int64 `json:"level2"`
	Downgraded int64 `json:"downgraded"`
}

// AffiliateTeamPlatformSummary 是汇总条：渠道客户（有归属代理）的余额消费，每个客户只算一次；
// 分润为区间内入账的全部代理分润，不限来源。
type AffiliateTeamPlatformSummary struct {
	Cost            float64 `json:"cost"`
	PlatformCost    float64 `json:"platform_cost"`
	Earnings        float64 `json:"earnings"`
	ActiveCustomers int64   `json:"active_customers"`
	Customers       int64   `json:"customers"`
}

type AffiliateTeamPlatformOverview struct {
	SettleIntervalMinutes int                          `json:"settle_interval_minutes"`
	Counts                AffiliateTeamOwnerCounts     `json:"counts"`
	Summary               AffiliateTeamPlatformSummary `json:"summary"`
	Trend                 []AffiliateTeamTrendPoint    `json:"trend"`
	Models                []AffiliateTeamModelShare    `json:"models"`
}

// AffiliateTeamPlatformAggregates 是汇总页的原始聚合：趋势只含有数据的日期，模型按消费降序。
type AffiliateTeamPlatformAggregates struct {
	Counts  AffiliateTeamOwnerCounts
	Summary AffiliateTeamPlatformSummary
	Trend   []AffiliateTeamTrendPoint
	Models  []AffiliateTeamModelShare
}

// AffiliateTeamOwnerEntry 是汇总页的一行：团队消费、活跃 / 客户、分润与该代理自己页面的摘要相同
// （一级包含其二级团队，各行有重叠）；CostShare 以汇总条的消费为分母。
type AffiliateTeamOwnerEntry struct {
	User            AffiliateTeamUser  `json:"user"`
	AgentLevel      int                `json:"agent_level"`
	Upline          *AffiliateTeamUser `json:"upline"`
	Customers       int64              `json:"customers"`
	ActiveCustomers int64              `json:"active_customers"`
	TeamCost        float64            `json:"team_cost"`
	Earnings        float64            `json:"earnings"`
	CostShare       float64            `json:"cost_share"`
}

// AdminOverview 返回汇总页的汇总条、等级计数、趋势与模型分布。
func (s *AffiliateTeamService) AdminOverview(ctx context.Context, rng AffiliateTeamRange) (*AffiliateTeamPlatformOverview, error) {
	if err := s.requireAdminRepo(); err != nil {
		return nil, err
	}
	agg, err := s.repo.GetPlatformAggregates(ctx, rng)
	if err != nil {
		return nil, err
	}
	return &AffiliateTeamPlatformOverview{
		SettleIntervalMinutes: s.settingService.GetAffiliateSettleIntervalMinutes(ctx),
		Counts:                agg.Counts,
		Summary:               agg.Summary,
		Trend:                 fillAffiliateTeamTrend(agg.Trend, rng),
		Models:                topAffiliateTeamModels(agg.Models, affiliateTeamTopModels),
	}, nil
}

// AdminListOwners 列出汇总页的行；等级不在白名单时按全部处理。
func (s *AffiliateTeamService) AdminListOwners(ctx context.Context, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamOwnerEntry, int64, error) {
	if err := s.requireAdminRepo(); err != nil {
		return nil, 0, err
	}
	query = normalizeAffiliateTeamListQuery(query, AffiliateTeamSortTeamCost, AffiliateTeamSortTeamCost, AffiliateTeamSortEarnings, AffiliateTeamSortActiveCustomers)
	switch query.Level {
	case AffiliateTeamLevel1, AffiliateTeamLevel2, AffiliateTeamLevelDowngraded:
	default:
		query.Level = AffiliateTeamLevelAll
	}
	query.AdminSearch = true
	items, total, err := s.repo.ListTeamOwners(ctx, rng, query)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].User = presentAffiliateTeamUser(affiliateTeamAudienceAdmin, items[i].User)
		items[i].AgentLevel = normalizeAgentLevel(items[i].AgentLevel)
		if items[i].Upline != nil {
			upline := presentAffiliateTeamUser(affiliateTeamAudienceAdmin, *items[i].Upline)
			items[i].Upline = &upline
		}
	}
	return items, total, nil
}

// AdminViewerOverview 等同于 viewerID 自己调用 Overview，另附 subject。
func (s *AffiliateTeamService) AdminViewerOverview(ctx context.Context, viewerID, agentID, customerID int64, rng AffiliateTeamRange) (*AffiliateTeamOverview, error) {
	return s.overview(ctx, affiliateTeamAudienceAdmin, viewerID, agentID, customerID, rng)
}

func (s *AffiliateTeamService) AdminViewerCustomers(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamCustomerEntry, int64, error) {
	return s.listCustomers(ctx, affiliateTeamAudienceAdmin, viewerID, rng, query)
}

func (s *AffiliateTeamService) AdminViewerAgents(ctx context.Context, viewerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamAgentEntry, int64, error) {
	return s.listAgents(ctx, affiliateTeamAudienceAdmin, viewerID, rng, query)
}

func (s *AffiliateTeamService) AdminViewerRequests(ctx context.Context, viewerID, customerID int64, rng AffiliateTeamRange, query AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error) {
	return s.listRequests(ctx, affiliateTeamAudienceAdmin, viewerID, customerID, rng, query)
}

func (s *AffiliateTeamService) requireAdminRepo() error {
	if s == nil || s.repo == nil || s.settingService == nil {
		return ErrAffiliateTeamUnavailable
	}
	return nil
}
