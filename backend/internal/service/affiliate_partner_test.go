//go:build unit

package service

// CUSTOM(fork): 归属、利润、角色保护和用户端隐私回归。
import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func partnerTestTime(day string) time.Time {
	v, _ := time.ParseInLocation("2006-01-02 15:04", day, AffiliateTeamLocation)
	return v
}
func TestPartnerMembersTopology(t *testing.T) {
	start := partnerTestTime("2026-02-02 12:00")
	early := start.Add(-time.Hour)
	late := start.Add(time.Hour)
	p, a, b := int64(1), int64(10), int64(20)
	second := int64(2)
	partners := map[int64]Partner{1: {UserID: 1, CreatedAt: start}, 2: {UserID: 2, CreatedAt: late}}
	attached := map[int64]PartnerAttachment{10: {AgentID: 10, PartnerID: 1, AttachedAt: late}}
	snaps := map[int64]PartnerSnapshot{
		1: {OwnerID: &a, BoundAt: &late}, 2: {OwnerID: &p},
		3: {OwnerID: &p, BoundAt: &early}, 4: {OwnerID: &p, BoundAt: &early},
		20: {OwnerID: &p, BoundAt: &early}, 21: {OwnerID: &b, UplineID: &p, BoundAt: &early},
		10: {OwnerID: &p, BoundAt: &early}, 11: {OwnerID: &a, BoundAt: &early},
		30: {OwnerID: &a, BoundAt: &early}, 31: {OwnerID: ptrPartnerID(30), UplineID: &a, BoundAt: &early},
		40: {}, 41: {OwnerID: &a, BoundAt: ptrPartnerTime(late.Add(time.Hour))},
		42: {OwnerID: &p, CreatedAt: late}, 43: {OwnerID: &second, UplineID: &p, BoundAt: &early},
		44: {User: PartnerUserRef{Deleted: true}, OwnerID: &p, BoundAt: &early},
	}
	got := ResolvePartnerMembers(partners, attached, snaps)
	for _, id := range []int64{1, 3, 4, 20, 21, 44} {
		require.Equal(t, int64(1), got[id].PartnerID)
		require.Zero(t, got[id].LineAgentID)
		require.Equal(t, start, got[id].StartAt)
	}
	for _, id := range []int64{10, 11, 30, 31} {
		require.Equal(t, int64(1), got[id].PartnerID)
		require.Equal(t, int64(10), got[id].LineAgentID)
		require.Equal(t, late, got[id].StartAt)
	}
	require.NotContains(t, got, int64(40))
	require.Equal(t, late.Add(time.Hour), got[41].StartAt)
	require.Equal(t, late, got[42].StartAt)
	require.Equal(t, int64(2), got[2].PartnerID)
	require.Equal(t, int64(2), got[43].PartnerID)
}
func ptrPartnerID(id int64) *int64           { return &id }
func ptrPartnerTime(at time.Time) *time.Time { return &at }
func partnerProfitFixture() PartnerMonthInput {
	at := partnerTestTime("2026-02-02 12:00")
	state := &PartnerState{Partners: map[int64]Partner{1: {UserID: 1, CreatedAt: at}}, Attachments: map[int64]PartnerAttachment{}, Snapshots: map[int64]PartnerSnapshot{}, Statements: map[int64]map[string]PartnerStatementDetail{}}
	return PartnerMonthInput{PartnerID: 1, Month: "2026-02", Rate: decimal.RequireFromString("0.4"), Data: &PartnerMonthsData{State: state, Finance: &FinanceData{ComputedAt: partnerTestTime("2026-03-02 00:00")}, Members: map[int64]PartnerMember{2: {PartnerID: 1, StartAt: at}}, StartDays: map[int64]PartnerStartDay{2: {Total: decimal.NewFromInt(10), After: decimal.NewFromInt(6)}}}, Replays: map[int64]FinanceRevenue{2: {Days: map[string]*FinanceRevenueDay{"2026-02-02": {RevenuePaid: decimal.NewFromInt(10), Consumption: decimal.NewFromInt(10)}}}}}
}
func TestPartnerProfitProrationRefundCostsCommissionAndAdjustments(t *testing.T) {
	in := partnerProfitFixture()
	in.Replays[2].Days["2026-02-03"] = &FinanceRevenueDay{RevenuePaid: decimal.NewFromInt(-2)}
	at := in.Data.Members[2].StartAt
	in.Data.Usage = []PartnerUsage{{UserID: 2, Day: "2026-02-02", GroupID: 9, GroupName: "测试分组", Model: "model", Cost: decimal.NewFromInt(1), Consumption: decimal.NewFromInt(6)}}
	in.Data.Finance.Ledger = []FinanceLedger{
		{SourceUserID: 2, At: at, Action: "accrue", RewardType: "agent_direct", Amount: decimal.RequireFromString("0.2")},
		{SourceUserID: 2, At: at.Add(-time.Second), Action: "accrue", RewardType: "agent_direct", Amount: decimal.NewFromInt(500)},
		{SourceUserID: 2, At: at, Action: "transfer", Amount: decimal.NewFromInt(500)},
		{SourceUserID: 2, At: at, Action: "accrue", RewardType: "invitee_bonus", Amount: decimal.NewFromInt(500)},
		{SourceUserID: 99, At: at, Action: "accrue", RewardType: "agent_direct", Amount: decimal.NewFromInt(500)},
	}
	in.Data.Finance.Expenses = []FinanceExpense{{FinanceExpenseInput: FinanceExpenseInput{PartnerUserID: ptrPartnerID(1), OccurredOn: "2026-02-28", Amount: "2.00", AmortizeDays: 2}}, {FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-28", Amount: "100.00", AmortizeDays: 1}}}
	in.Data.State.Adjustments = []PartnerAdjustment{{ID: 1, PartnerID: 1, Month: "2026-02", Amount: "0.20", Note: "补差"}}
	out := ComputePartnerMonth(in)
	require.Equal(t, "4", out.Totals.Revenue)
	require.Equal(t, "6", out.Totals.Consumption)
	require.Equal(t, "1", out.Totals.AccountCost)
	require.Equal(t, "0.2", out.Totals.Commission, "账本金额已含分组系数，不再折算")
	require.Equal(t, "1", out.Totals.RegionExpense)
	require.Equal(t, "2", out.Totals.Profit)
	require.Equal(t, "0.80", out.Totals.ShareAmount)
	require.Equal(t, "2.8", out.Lines[0].Profit)
	require.Len(t, out.CostByGroup, 1)
	require.Len(t, out.CostByModel, 1)
	require.Equal(t, "1", out.CostByModel[0].AccountCost)
}
func TestPartnerProfitLossChainRatesAndRounding(t *testing.T) {
	for _, tc := range []struct{ profit, loss, base, after, share string }{{"-3", "5", "0", "8", "0.00"}, {"3", "5", "0", "2", "0.00"}, {"5", "5", "0", "0", "0.00"}, {"5.0125", "5", "0.0125", "0", "0.01"}} {
		t.Run(tc.profit, func(t *testing.T) {
			in := partnerProfitFixture()
			in.Replays = map[int64]FinanceRevenue{}
			in.Data.State.Adjustments = []PartnerAdjustment{{PartnerID: 1, Month: in.Month, Amount: tc.profit}}
			in.LossBefore = decimal.RequireFromString(tc.loss)
			out := ComputePartnerMonth(in)
			require.Equal(t, tc.base, out.Totals.ShareBase)
			require.Equal(t, tc.after, out.Totals.LossAfter)
			require.Equal(t, tc.share, out.Totals.ShareAmount)
		})
	}
	rates := []PartnerRate{{PartnerID: 1, Month: "2026-02", Rate: decimal.RequireFromString("0.4")}, {PartnerID: 1, Month: "2026-03", Rate: decimal.RequireFromString("0.35")}}
	require.Equal(t, "0.4", PartnerMonthRate(rates, 1, "2026-02").String())
	require.Equal(t, "0.35", PartnerMonthRate(rates, 1, "2026-04").String())
	in := partnerProfitFixture()
	in.Data.Members = map[int64]PartnerMember{}
	out := ComputePartnerMonth(in)
	require.Equal(t, "0", out.Totals.Profit)
	require.Equal(t, "0.00", out.Totals.ShareAmount)
	in = partnerProfitFixture()
	in.Data.StartDays[2] = PartnerStartDay{}
	out = ComputePartnerMonth(in)
	require.Equal(t, "0", out.Totals.Revenue)
	require.Contains(t, out.Warnings, "成员计入起点当天的部分明细已被清理，收入按现存明细折算")
}

type partnerNodesRepo struct {
	*fakeAffiliateRepo
	partner int64
}

func (r *partnerNodesRepo) GetAffiliateNodes(ctx context.Context, ids []int64) (map[int64]AffiliateNode, error) {
	nodes, err := r.fakeAffiliateRepo.GetAffiliateNodes(ctx, ids)
	if n, ok := nodes[r.partner]; ok {
		n.Partner = true
		nodes[r.partner] = n
	}
	return nodes, err
}
func TestPartnerRewardsKeepActiveAndLevel2Share(t *testing.T) {
	p, l := int64(1), int64(2)
	level := 2
	nodes := map[int64]AffiliateNode{1: {Active: true, Partner: true}, 2: {Active: true}}
	consumer := &AffiliateSummary{UserID: 3, AgentLevel: 3, AgentOwnerID: &l, AgentOwnerLevel: &level, AgentUplineID: &p, InviterID: &p}
	shares := planAgentShares(consumer, nodes, AffiliateRewardConfig{PoolRate: 20, Level2Rate: 8})
	require.Equal(t, []affiliateShare{{BeneficiaryID: 2, RatePercent: 8, RewardType: AffiliateRewardAgentDirect}}, shares)
	require.True(t, nodeActive(nodes, 1))
	require.False(t, nodeRewardable(nodes, 1))
	require.Nil(t, planInviteShare(consumer, nodes, AffiliateRewardConfig{InviteMode: AffiliateInviteRewardModePercent, InviteRate: 2}))
	svc, repo := newExampleChain(t)
	svc.repo = &partnerNodesRepo{repo, userU1}
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{SettingKeyAffiliateEnabled: "true", SettingKeyAffiliateInviteRewardMode: AffiliateInviteRewardModeFirstRecharge, SettingKeyAffiliateInviteRewardFixedAmount: "2", SettingKeyAffiliateInviteRewardMinRecharge: "10"}}, nil)
	reward, err := svc.ApplyFirstRechargeRewards(context.Background(), userU2, 10, nil)
	require.NoError(t, err)
	require.Zero(t, reward.InviterReward)
	svc, repo = newAppointFixture(t, nil)
	svc.repo = &partnerNodesRepo{repo, userB}
	require.ErrorIs(t, svc.AppointAgentLevel(context.Background(), userB, userU1, 2), ErrPartnerCannotAppoint)
}

type partnerStateStub struct {
	PartnerRepository
	state *PartnerState
	loads int
	data  *PartnerMonthsData
}

func (s *partnerStateStub) Read(context.Context) (*PartnerState, error) { return s.state, nil }
func (s *partnerStateStub) LoadMonths(context.Context, []int64, string, string) (*PartnerMonthsData, error) {
	s.loads++
	return s.data, nil
}
func TestPartnerPermissionsAndPublicPrivacy(t *testing.T) {
	svc, _ := newTwoTierService(newFakeAffiliateRepo(), nil)
	state := &PartnerState{Partners: map[int64]Partner{1: {UserID: 1}}, Attachments: map[int64]PartnerAttachment{10: {PartnerID: 1}}}
	repo := &partnerStateStub{state: state}
	svc.partnerRepo = repo
	p := NewPartnerService(repo, svc, nil)
	require.ErrorIs(t, svc.CheckPartnerLevels(context.Background(), []int64{2, 1}), ErrPartnerLevelFixed)
	require.ErrorIs(t, p.RequireAttachedAgent(context.Background(), 1, 11), ErrPartnerForbidden)
	require.NoError(t, p.RequireAttachedAgent(context.Background(), 1, 10))
	_, err := p.RequirePartner(context.Background(), 2)
	require.ErrorIs(t, err, ErrPartnerForbidden)
	_, _, err = p.TeamRequests(context.Background(), 1, 99, AffiliateTeamRange{}, AffiliateTeamListQuery{})
	require.ErrorIs(t, err, ErrPartnerForbidden)
	d := PartnerStatementDetail{Lines: []PartnerLine{{LineAgent: &PartnerUserRef{ID: 10, Email: "private@example.com"}}}, Adjustments: []PartnerAdjustment{{CreatedByEmail: "admin@example.com"}}}
	pub := PartnerPublicStatement(d)
	raw, err := json.Marshal(pub)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private@example.com")
	require.NotContains(t, string(raw), "created_by_email")
	require.Equal(t, "private@example.com", d.Lines[0].LineAgent.Email)
	raw, err = json.Marshal(PartnerTeamCustomer{})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "earnings")
	raw, err = json.Marshal(PartnerTeamOverview{})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "earnings")
}
func TestPartnerMonthsUseFrozenLossAndSingleLoad(t *testing.T) {
	in := partnerProfitFixture()
	state := in.Data.State
	state.Statements[1] = map[string]PartnerStatementDetail{"2026-02": {Totals: PartnerMonthTotals{Month: "2026-02", Status: "confirmed", LossAfter: "5"}}}
	state.Rates = []PartnerRate{{PartnerID: 1, Month: "2026-02", Rate: in.Rate}}
	repo := &partnerStateStub{state: state, data: in.Data}
	p := NewPartnerService(repo, nil, nil)
	_, months, err := p.months(context.Background(), state, []int64{1}, "2026-03")
	require.NoError(t, err)
	require.Equal(t, 1, repo.loads)
	require.Len(t, months[1], 2)
	require.Equal(t, "5", months[1][1].Totals.LossBefore)
	require.Equal(t, "confirmed", months[1][0].Totals.Status)
}

// CUSTOM(fork): 使用既有 USER_NOT_FOUND reason，仅在新接口返回中文。
func TestPartnerMissingUserChineseMessage(t *testing.T) {
	repo := &partnerStateStub{state: &PartnerState{Snapshots: map[int64]PartnerSnapshot{}}}
	svc := NewPartnerService(repo, nil, nil)
	_, err := svc.PreviewCreate(context.Background(), 99)
	require.ErrorIs(t, err, ErrUserNotFound)
	require.ErrorContains(t, err, "用户不存在")
}

// CUSTOM(fork): 各线按北京时间挂靠月出现，直属行和当月零用量线保留。
func TestPartnerLinesStartAtAttachmentMonth(t *testing.T) {
	in := partnerProfitFixture()
	in.Data.Members = map[int64]PartnerMember{}
	in.Data.State.Attachments = map[int64]PartnerAttachment{
		10: {AgentID: 10, PartnerID: 1, AttachedAt: partnerTestTime("2026-09-30 23:59")},
		20: {AgentID: 20, PartnerID: 1, AttachedAt: time.Date(2026, 10, 31, 15, 59, 59, 0, time.UTC)},
		30: {AgentID: 30, PartnerID: 1, AttachedAt: time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)},
		40: {AgentID: 40, PartnerID: 2, AttachedAt: partnerTestTime("2026-08-01 00:00")},
	}
	for _, tc := range []struct {
		month string
		ids   []int64
	}{
		{"2026-08", []int64{0}},
		{"2026-09", []int64{0, 10}},
		{"2026-10", []int64{0, 10, 20}},
		{"2026-11", []int64{0, 10, 20, 30}},
	} {
		t.Run(tc.month, func(t *testing.T) {
			in.Month = tc.month
			out := ComputePartnerMonth(in)
			ids := make([]int64, 0, len(out.Lines))
			for _, line := range out.Lines {
				id := int64(0)
				if line.LineAgent != nil {
					id = line.LineAgent.ID
				}
				ids = append(ids, id)
				require.Zero(t, line.Members)
				require.Equal(t, partnerLineAmounts(financeAmounts{}), line.PartnerLineAmounts)
			}
			require.Equal(t, tc.ids, ids)
			require.Nil(t, out.Lines[0].LineAgent)
		})
	}
}
