package service

// CUSTOM(fork): 区域团队只展示客户用量，复用代理团队查询和脱敏。
import (
	"context"
	"sort"
	"strings"
	"time"
)

type PartnerTeamSummary struct {
	Cost            float64 `json:"cost"`
	DirectCost      float64 `json:"direct_cost"`
	SubCost         float64 `json:"sub_cost"`
	ActiveCustomers int64   `json:"active_customers"`
	Customers       int64   `json:"customers"`
	Requests        int64   `json:"requests"`
	Tokens          int64   `json:"tokens"`
}
type PartnerTeamTrend struct {
	Date string  `json:"date"`
	Cost float64 `json:"cost"`
}

// CUSTOM(fork): 区域客户主体沿用团队页字段，但不返回累计分润。
type PartnerTeamCustomerSubject struct {
	User       AffiliateTeamUser  `json:"user"`
	AgentLevel int                `json:"agent_level"`
	Relation   string             `json:"relation"`
	OwnerAgent *AffiliateTeamUser `json:"owner_agent"`
	IsSubAgent bool               `json:"is_sub_agent"`
	BoundAt    *time.Time         `json:"bound_at"`
}

type PartnerTeamOverview struct {
	Viewer   AffiliateTeamViewer         `json:"viewer"`
	Counts   *AffiliateTeamCounts        `json:"counts"`
	Agent    *AffiliateTeamAgentSubject  `json:"agent"`
	Customer *PartnerTeamCustomerSubject `json:"customer"`
	Summary  PartnerTeamSummary          `json:"summary"`
	Trend    []PartnerTeamTrend          `json:"trend"`
	Models   []AffiliateTeamModelShare   `json:"models"`
	Filters  *AffiliateTeamFilters       `json:"filters"`
}
type PartnerTeamCustomer struct {
	User       AffiliateTeamUser `json:"user"`
	AgentLevel int               `json:"agent_level"`
	Cost       float64           `json:"cost"`
	Requests   int64             `json:"requests"`
	LastUsedAt *time.Time        `json:"last_used_at"`
	CostShare  float64           `json:"cost_share"`
	Owner      *PartnerUserRef   `json:"owner"`
	LineAgent  *PartnerUserRef   `json:"line_agent"`
}
type PartnerTeamLine struct {
	LineAgent       *PartnerUserRef `json:"line_agent"`
	Customers       int64           `json:"customers"`
	ActiveCustomers int64           `json:"active_customers"`
	Requests        int64           `json:"requests"`
	TeamCost        float64         `json:"team_cost"`
	CostShare       float64         `json:"cost_share"`
}

func partnerMemberIDs(state *PartnerState, id int64) []int64 {
	ids := []int64{}
	for user, m := range ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots) {
		if m.PartnerID == id {
			ids = append(ids, user)
		}
	}
	return ids
}
func (s *PartnerService) TeamOverview(ctx context.Context, id, customerID int64, rng AffiliateTeamRange) (*PartnerTeamOverview, error) {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &PartnerTeamOverview{}
	scope := AffiliateTeamScope{ViewerID: id}
	if customerID > 0 {
		// CUSTOM(fork): 先按区域快照授权，再读取该客户的用量和筛选项。
		m, ok := ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots)[customerID]
		if !ok || m.PartnerID != id {
			return nil, ErrPartnerForbidden
		}
		snapshot := state.Snapshots[customerID]
		u := snapshot.User
		customer := &PartnerTeamCustomerSubject{
			User:       presentAffiliateTeamUser(affiliateTeamAudienceAgent, AffiliateTeamUser{UserID: u.ID, Nickname: u.Username, Email: u.Email, Deleted: u.Deleted}),
			AgentLevel: normalizeAgentLevel(snapshot.AgentLevel),
			Relation:   "owner",
			BoundAt:    snapshot.BoundAt,
		}
		if snapshot.OwnerID != nil {
			if *snapshot.OwnerID == id {
				customer.IsSubAgent = snapshot.AgentLevel == 2
			} else {
				customer.Relation = "upline"
				u := state.User(*snapshot.OwnerID)
				owner := presentAffiliateTeamUser(affiliateTeamAudienceAgent, AffiliateTeamUser{UserID: u.ID, Nickname: u.Username, Email: u.Email, Deleted: u.Deleted})
				customer.OwnerAgent = &owner
			}
		}
		// 与代理端一致，仍有快照客户的历史二级代理也保留下钻标记。
		for _, v := range state.Snapshots {
			if v.UplineID != nil && *v.UplineID == id && v.OwnerID != nil && *v.OwnerID == customerID {
				customer.IsSubAgent = true
				break
			}
		}
		out.Customer = customer
		filters, err := s.team.repo.GetCustomerFilters(ctx, customerID, rng)
		if err != nil {
			return nil, err
		}
		out.Filters = &filters
		scope.MemberIDs = []int64{customerID}
	} else {
		scope.MemberIDs = partnerMemberIDs(state, id)
	}
	agg, err := s.team.repo.GetAggregates(ctx, scope, rng)
	if err != nil {
		return nil, err
	}
	a := agg.Summary
	out.Viewer = AffiliateTeamViewer{AgentLevel: 1, SettleIntervalMinutes: s.team.settingService.GetAffiliateSettleIntervalMinutes(ctx)}
	out.Summary = PartnerTeamSummary{a.Cost, a.DirectCost, a.SubCost, a.ActiveCustomers, a.Customers, a.Requests, a.Tokens}
	out.Trend = []PartnerTeamTrend{}
	out.Models = topAffiliateTeamModels(agg.Models, affiliateTeamTopModels)
	for _, p := range fillAffiliateTeamTrend(agg.Trend, rng) {
		out.Trend = append(out.Trend, PartnerTeamTrend{p.Date, p.Cost})
	}
	return out, nil
}
func (s *PartnerService) TeamCustomers(ctx context.Context, id int64, rng AffiliateTeamRange, q AffiliateTeamListQuery) ([]PartnerTeamCustomer, int64, error) {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	members := ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots)
	q = normalizeAffiliateTeamListQuery(q, AffiliateTeamSortCost, AffiliateTeamSortCost, AffiliateTeamSortRequests, AffiliateTeamSortLastUsedAt)
	q.MemberIDs = partnerMemberIDs(state, id)
	q.AdminSearch = false
	rows, total, err := s.team.repo.ListCustomers(ctx, id, rng, q)
	if err != nil {
		return nil, 0, err
	}
	out := []PartnerTeamCustomer{}
	for _, row := range rows {
		v := PartnerTeamCustomer{User: presentAffiliateTeamUser(affiliateTeamAudienceAgent, row.User), AgentLevel: row.AgentLevel, Cost: row.Cost, Requests: row.Requests, LastUsedAt: row.LastUsedAt, CostShare: row.CostShare}
		if owner := state.Snapshots[row.User.UserID].OwnerID; owner != nil {
			u := partnerMaskUser(state.User(*owner))
			v.Owner = &u
		}
		if line := members[row.User.UserID].LineAgentID; line != 0 {
			u := partnerMaskUser(state.User(line))
			v.LineAgent = &u
		}
		out = append(out, v)
	}
	return out, total, nil
}
func (s *PartnerService) TeamLines(ctx context.Context, id int64, rng AffiliateTeamRange) ([]PartnerTeamLine, error) {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return nil, err
	}
	lines := map[int64][]int64{0: {}}
	for agent, a := range state.Attachments {
		if a.PartnerID == id {
			lines[agent] = []int64{}
		}
	}
	for user, m := range ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots) {
		if m.PartnerID == id {
			lines[m.LineAgentID] = append(lines[m.LineAgentID], user)
		}
	}
	out := []PartnerTeamLine{}
	total := 0.0
	for line, ids := range lines {
		a, err := s.team.repo.GetAggregates(ctx, AffiliateTeamScope{ViewerID: id, MemberIDs: ids}, rng)
		if err != nil {
			return nil, err
		}
		v := PartnerTeamLine{Customers: a.Summary.Customers, ActiveCustomers: a.Summary.ActiveCustomers, Requests: a.Summary.Requests, TeamCost: a.Summary.Cost}
		if line != 0 {
			u := partnerMaskUser(state.User(line))
			v.LineAgent = &u
		}
		out = append(out, v)
		total += v.TeamCost
	}
	for i := range out {
		if total > 0 {
			out[i].CostShare = out[i].TeamCost / total
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TeamCost != out[j].TeamCost {
			return out[i].TeamCost > out[j].TeamCost
		}
		if out[i].LineAgent == nil {
			return true
		}
		if out[j].LineAgent == nil {
			return false
		}
		return out[i].LineAgent.ID < out[j].LineAgent.ID
	})
	return out, nil
}
func (s *PartnerService) TeamRequests(ctx context.Context, id, customer int64, rng AffiliateTeamRange, q AffiliateTeamListQuery) ([]AffiliateTeamRequest, int64, error) {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	m, ok := ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots)[customer]
	if !ok || m.PartnerID != id {
		return nil, 0, ErrPartnerForbidden
	}
	q = normalizeAffiliateTeamListQuery(q, "", "")
	q.Model = strings.TrimSpace(q.Model)
	rows, total, err := s.team.repo.ListRequests(ctx, customer, rng, q)
	if err != nil {
		return nil, 0, err
	}
	fillAffiliateTeamRebateFactors(rows, s.team.settingService.GetAffiliateGroupRebateFactors(ctx))
	return rows, total, nil
}
func (s *PartnerService) RequireAttachedAgent(ctx context.Context, id, agent int64) error {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return err
	}
	a, ok := state.Attachments[agent]
	if !ok || a.PartnerID != id {
		return ErrPartnerForbidden
	}
	return nil
}
