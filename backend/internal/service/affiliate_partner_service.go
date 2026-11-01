package service

// CUSTOM(fork): 合伙人管理与自然月结算用例。
import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

func partnerMonthNow() string { return time.Now().In(AffiliateTeamLocation).Format("2006-01") }
func validatePartnerMonth(month string) error {
	if _, err := time.Parse("2006-01", month); err != nil {
		return ErrPartnerMonthRange
	}
	return nil
}
func validatePartnerName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return infraerrors.BadRequest("PARTNER_INVALID", "区域名称须为 1–64 个字")
	}
	return nil
}
func validatePartnerRate(rate decimal.Decimal) error {
	if rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(1)) || !rate.Equal(rate.Round(4)) {
		return infraerrors.BadRequest("PARTNER_INVALID", "分成比例须在 0–1 之间，最多四位小数")
	}
	return nil
}
func (s *PartnerService) RequirePartner(ctx context.Context, id int64) (*PartnerState, error) {
	if !s.affiliate.IsEnabled(ctx) {
		return nil, ErrPartnerForbidden
	}
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := state.Partners[id]; !ok {
		return nil, ErrPartnerForbidden
	}
	return state, nil
}
func partnerRegion(state *PartnerState, id int64) *PartnerCurrentRegion {
	m, ok := ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots)[id]
	if !ok {
		return nil
	}
	return &PartnerCurrentRegion{state.User(m.PartnerID), state.Partners[m.PartnerID].RegionName}
}
func (s *PartnerService) PreviewCreate(ctx context.Context, id int64) (*PartnerPreview, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	v, ok := state.Snapshots[id]
	if !ok || v.User.Deleted {
		return nil, ErrPartnerUserNotFound
	}
	_, partner := state.Partners[id]
	_, attached := state.Attachments[id]
	out := &PartnerPreview{User: v.User, AgentLevel: v.AgentLevel, IsPartner: partner, IsAttached: attached, Team: state.Team(id), CurrentRegion: partnerRegion(state, id), CanCreate: !partner && !attached}
	if partner {
		reason := "该用户已是合伙人"
		out.Reason = &reason
	} else if attached {
		reason := "该渠道经理已挂靠合伙人"
		out.Reason = &reason
	}
	return out, nil
}
func (s *PartnerService) PreviewAttach(ctx context.Context, id, agent int64) (*PartnerAttachPreview, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := state.Partners[id]; !ok {
		return nil, ErrPartnerNotFound
	}
	v, ok := state.Snapshots[agent]
	if !ok {
		return nil, ErrPartnerUserNotFound
	}
	out := &PartnerAttachPreview{Agent: v.User, AgentLevel: v.AgentLevel, Team: state.Team(agent), CurrentRegion: partnerRegion(state, agent), CanAttach: true}
	var reason string
	if _, ok := state.Partners[agent]; ok {
		reason = "合伙人不能挂靠其他合伙人"
	} else if _, ok := state.Attachments[agent]; ok {
		reason = "该渠道经理已挂靠合伙人"
	} else if v.User.Deleted || v.AgentLevel != 1 {
		reason = "只能挂靠现有渠道经理"
	}
	if reason != "" {
		out.Reason = &reason
		out.CanAttach = false
	}
	return out, nil
}
func (s *PartnerService) Create(ctx context.Context, id int64, name string, rate decimal.Decimal, actor int64) (*PartnerItem, error) {
	name = strings.TrimSpace(name)
	if err := validatePartnerName(name); err != nil {
		return nil, err
	}
	if err := validatePartnerRate(rate); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, id, name, rate, actor); err != nil {
		return nil, err
	}
	s.affiliate.invalidateAffiliateCaches(ctx, id)
	d, err := s.Detail(ctx, id)
	if err != nil {
		return nil, err
	}
	return &d.PartnerItem, nil
}
func (s *PartnerService) Update(ctx context.Context, id int64, name *string, rate *decimal.Decimal, actor int64) (*PartnerItem, error) {
	if name != nil {
		v := strings.TrimSpace(*name)
		if err := validatePartnerName(v); err != nil {
			return nil, err
		}
		name = &v
	}
	if rate != nil {
		if err := validatePartnerRate(*rate); err != nil {
			return nil, err
		}
	}
	if err := s.repo.Update(ctx, id, name, rate, actor, partnerNextMonth(partnerMonthNow())); err != nil {
		return nil, err
	}
	d, err := s.Detail(ctx, id)
	if err != nil {
		return nil, err
	}
	return &d.PartnerItem, nil
}
func (s *PartnerService) Attach(ctx context.Context, id, agent, actor int64) (*PartnerAgentItem, error) {
	if err := s.repo.Attach(ctx, id, agent, actor); err != nil {
		return nil, err
	}
	d, err := s.Detail(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, a := range d.Agents {
		if a.User.ID == agent {
			return &a, nil
		}
	}
	return nil, ErrPartnerAgentLevel
}

// CUSTOM(fork): 每次查询只取一次区间数据，已确认月直接接续冻结亏损。
func (s *PartnerService) months(ctx context.Context, state *PartnerState, ids []int64, to string) (*PartnerState, map[int64][]PartnerStatementDetail, error) {
	from := to
	for _, id := range ids {
		p, ok := state.Partners[id]
		if !ok {
			return nil, nil, ErrPartnerNotFound
		}
		for m := p.CreatedAt.In(AffiliateTeamLocation).Format("2006-01"); m <= to; m = partnerNextMonth(m) {
			if _, ok := state.Statements[id][m]; !ok {
				if m < from {
					from = m
				}
				break
			}
		}
	}
	data, err := s.repo.LoadMonths(ctx, ids, from, to)
	if err != nil {
		return nil, nil, err
	}
	state = data.State
	replays := financeReplays(data.Finance)
	out := map[int64][]PartnerStatementDetail{}
	for _, id := range ids {
		out[id] = []PartnerStatementDetail{}
		loss := decimal.Zero
		for m := state.Partners[id].CreatedAt.In(AffiliateTeamLocation).Format("2006-01"); m <= to; m = partnerNextMonth(m) {
			detail, ok := state.Statements[id][m]
			if !ok {
				detail = ComputePartnerMonth(PartnerMonthInput{PartnerID: id, Month: m, Rate: PartnerMonthRate(state.Rates, id, m), LossBefore: loss, Data: data, Replays: replays})
			}
			loss = partnerDecimal(detail.Totals.LossAfter)
			out[id] = append(out[id], detail)
		}
	}
	return state, out, nil
}
func partnerItem(state *PartnerState, id int64, months []PartnerStatementDetail) PartnerItem {
	now := partnerMonthNow()
	out := PartnerItem{User: state.User(id), PartnerInfo: *state.Info(id, now), LossCarried: "0"}
	for _, a := range state.Attachments {
		if a.PartnerID == id {
			out.AgentCount++
		}
	}
	for _, m := range ResolvePartnerMembers(state.Partners, state.Attachments, state.Snapshots) {
		if m.PartnerID == id {
			out.MemberCount++
		}
	}
	for _, d := range months {
		t := d.Totals
		if t.Month == now {
			out.CurrentMonth = t
		} else {
			out.LossCarried = t.LossAfter
			if t.Status == "draft" {
				out.UnconfirmedMonths++
			}
		}
	}
	return out
}
func (s *PartnerService) List(ctx context.Context) ([]PartnerItem, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(state.Partners))
	for id := range state.Partners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := []PartnerItem{}
	if len(ids) == 0 {
		return out, nil
	}
	state, months, err := s.months(ctx, state, ids, partnerMonthNow())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out = append(out, partnerItem(state, id, months[id]))
	}
	return out, nil
}
func (s *PartnerService) Detail(ctx context.Context, id int64) (*PartnerAdminDetail, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	state, months, err := s.months(ctx, state, []int64{id}, partnerMonthNow())
	if err != nil {
		return nil, err
	}
	out := &PartnerAdminDetail{PartnerItem: partnerItem(state, id, months[id]), Agents: []PartnerAgentItem{}}
	last := months[id][len(months[id])-1]
	for agent, a := range state.Attachments {
		if a.PartnerID != id {
			continue
		}
		line := partnerLineAmounts(financeAmounts{})
		for _, l := range last.Lines {
			if l.LineAgent != nil && l.LineAgent.ID == agent {
				line = l.PartnerLineAmounts
			}
		}
		out.Agents = append(out.Agents, PartnerAgentItem{User: state.User(agent), AttachedAt: a.AttachedAt, Source: a.Source, AttachedByEmail: a.AttachedByEmail, AgentLevel: state.Snapshots[agent].AgentLevel, Team: state.Team(agent), CurrentMonth: line})
	}
	sort.Slice(out.Agents, func(i, j int) bool { return out.Agents[i].User.ID < out.Agents[j].User.ID })
	return out, nil
}
func (s *PartnerService) Statements(ctx context.Context, id int64) ([]PartnerMonthTotals, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	_, months, err := s.months(ctx, state, []int64{id}, partnerMonthNow())
	if err != nil {
		return nil, err
	}
	out := []PartnerMonthTotals{}
	for i := len(months[id]) - 1; i >= 0; i-- {
		out = append(out, months[id][i].Totals)
	}
	return out, nil
}
func (s *PartnerService) Statement(ctx context.Context, id int64, month string) (*PartnerStatementDetail, error) {
	if err := validatePartnerMonth(month); err != nil {
		return nil, err
	}
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	p, ok := state.Partners[id]
	if !ok {
		return nil, ErrPartnerNotFound
	}
	if month < p.CreatedAt.In(AffiliateTeamLocation).Format("2006-01") || month > partnerMonthNow() {
		return nil, ErrPartnerMonthRange
	}
	if d, ok := state.Statements[id][month]; ok {
		return &d, nil
	}
	_, months, err := s.months(ctx, state, []int64{id}, partnerMonthNow())
	if err != nil {
		return nil, err
	}
	for _, d := range months[id] {
		if d.Totals.Month == month {
			return &d, nil
		}
	}
	return nil, ErrPartnerMonthRange
}
func (s *PartnerService) Overview(ctx context.Context, id int64) (*PartnerOverview, error) {
	state, err := s.RequirePartner(ctx, id)
	if err != nil {
		return nil, err
	}
	state, months, err := s.months(ctx, state, []int64{id}, partnerMonthNow())
	if err != nil {
		return nil, err
	}
	item := partnerItem(state, id, months[id])
	last := months[id][len(months[id])-1]
	last = PartnerPublicStatement(last)
	return &PartnerOverview{PartnerInfo: item.PartnerInfo, LossCarried: item.LossCarried, AgentCount: item.AgentCount, MemberCount: item.MemberCount, CurrentMonth: item.CurrentMonth, Lines: last.Lines}, nil
}
func (s *PartnerService) Confirm(ctx context.Context, id int64, month string, actor int64) (*PartnerStatementDetail, error) {
	if err := validatePartnerMonth(month); err != nil {
		return nil, err
	}
	if month >= partnerMonthNow() {
		return nil, infraerrors.BadRequest("PARTNER_MONTH_NOT_ENDED", "只能确认已经结束的月份")
	}
	err := s.repo.WithPartnerLock(ctx, id, func(ctx context.Context) error {
		state, err := s.repo.Read(ctx)
		if err != nil {
			return err
		}
		first := state.Partners[id].CreatedAt.In(AffiliateTeamLocation).Format("2006-01")
		if month < first {
			return ErrPartnerMonthRange
		}
		if _, ok := state.Statements[id][month]; ok {
			return ErrPartnerMonthConfirmed
		}
		at, _ := time.ParseInLocation("2006-01", month, AffiliateTeamLocation)
		prev := at.AddDate(0, -1, 0).Format("2006-01")
		if month != first {
			if _, ok := state.Statements[id][prev]; !ok {
				return infraerrors.BadRequest("PARTNER_PREVIOUS_UNCONFIRMED", "请先确认上一个月份")
			}
		}
		d, err := s.Statement(ctx, id, month)
		if err != nil {
			return err
		}
		d.Totals.Status = "confirmed"
		return s.repo.SaveStatement(ctx, id, actor, *d)
	})
	if err != nil {
		return nil, err
	}
	return s.Statement(ctx, id, month)
}
func (s *PartnerService) Pay(ctx context.Context, id int64, month string, actor int64, day, note string) (*PartnerStatementDetail, error) {
	if err := validatePartnerMonth(month); err != nil {
		return nil, err
	}
	if day == "" {
		day = time.Now().In(AffiliateTeamLocation).Format("2006-01-02")
	}
	_, err := time.Parse("2006-01-02", day)
	note = strings.TrimSpace(note)
	if err != nil || note == "" || utf8.RuneCountInString(note) > 500 {
		return nil, infraerrors.BadRequest("PARTNER_INVALID", "付款日期无效或备注不在 1–500 字以内")
	}
	if err = s.repo.PayStatement(ctx, id, month, actor, day, note); err != nil {
		return nil, err
	}
	return s.Statement(ctx, id, month)
}
func (s *PartnerService) AddAdjustment(ctx context.Context, id int64, month string, amount decimal.Decimal, note string, actor int64) (*PartnerAdjustment, error) {
	if err := validatePartnerMonth(month); err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if amount.IsZero() || !amount.Equal(amount.Round(2)) || amount.Abs().GreaterThanOrEqual(decimal.New(1, 18)) || note == "" || utf8.RuneCountInString(note) > 500 {
		return nil, infraerrors.BadRequest("PARTNER_ADJUSTMENT_INVALID", "调整金额须非零且最多两位小数，说明须为 1–500 字")
	}
	return s.repo.AddAdjustment(ctx, id, month, amount.StringFixed(2), note, actor)
}
func (s *PartnerService) DeleteAdjustment(ctx context.Context, id, adjustment int64) error {
	return s.repo.DeleteAdjustment(ctx, id, adjustment)
}

func PartnerPublicStatement(in PartnerStatementDetail) PartnerStatementDetail {
	in.Lines = append([]PartnerLine{}, in.Lines...)
	for i := range in.Lines {
		if in.Lines[i].LineAgent != nil {
			u := partnerMaskUser(*in.Lines[i].LineAgent)
			in.Lines[i].LineAgent = &u
		}
	}
	in.Adjustments = append([]PartnerAdjustment{}, in.Adjustments...)
	for i := range in.Adjustments {
		in.Adjustments[i].CreatedByEmail = ""
	}
	return in
}
func partnerMaskUser(u PartnerUserRef) PartnerUserRef {
	u.Email = maskEmail(u.Email)
	if u.Deleted {
		u.Email = ""
		u.Username = ""
	}
	return u
}
