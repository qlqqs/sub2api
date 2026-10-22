//go:build unit

package service

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 内存版 AffiliateRepository：按真实语义实现绑定快照、流水唯一约束、等级变更，
// 供两级代理分润的端到端单测使用。
// ---------------------------------------------------------------------------

type fakeLedgerEntry struct {
	userID, sourceUserID int64
	amount               float64
	action, rewardType   string
	freezeHours          int
	source               AffiliateAccrueSource
}

type fakeAffiliateRepo struct {
	rows     map[int64]*AffiliateSummary
	deleted  map[int64]bool
	ledger   []fakeLedgerEntry
	balances map[int64]float64

	inviteeCount      int
	countCalls        int
	lastRechargedOnly bool

	pendingUsers []int64
	pending      map[int64]*AffiliatePendingConsumption
	advances     map[int64]float64
}

func newFakeAffiliateRepo() *fakeAffiliateRepo {
	return &fakeAffiliateRepo{
		rows:     map[int64]*AffiliateSummary{},
		deleted:  map[int64]bool{},
		balances: map[int64]float64{},
		pending:  map[int64]*AffiliatePendingConsumption{},
		advances: map[int64]float64{},
	}
}

func (r *fakeAffiliateRepo) row(id int64) *AffiliateSummary {
	if row, ok := r.rows[id]; ok {
		return row
	}
	row := &AffiliateSummary{
		UserID: id, AffCode: fmt.Sprintf("CODE%d", id), AgentLevel: AffiliateAgentLevelDefault,
		AgentLevelSource: AffiliateAgentLevelSourceDefault, CreatedAt: time.Now().Add(-time.Hour),
	}
	r.rows[id] = row
	return row
}

func (r *fakeAffiliateRepo) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 || r.deleted[userID] {
		return nil, ErrUserNotFound
	}
	cp := *r.row(userID)
	return &cp, nil
}

func (r *fakeAffiliateRepo) GetAffiliateByCode(_ context.Context, code string) (*AffiliateSummary, error) {
	for _, row := range r.rows {
		if row.AffCode == code {
			cp := *row
			return &cp, nil
		}
	}
	return nil, ErrAffiliateProfileNotFound
}

func (r *fakeAffiliateRepo) BindInviter(_ context.Context, userID, inviterID int64, binding AffiliateInviterBinding) (*AffiliateInviterBindingResult, bool, error) {
	row := r.row(userID)
	if row.InviterID != nil {
		return nil, false, nil
	}
	now := time.Now()
	id := inviterID
	row.InviterID = &id
	row.InviterBoundAt = &now
	row.AgentOwnerID = binding.Owner.OwnerID
	row.AgentOwnerLevel = binding.Owner.OwnerLevel
	row.AgentUplineID = binding.Owner.UplineID
	row.AgentOwnerBoundAt = &now
	row.AgentOwnerSetBy = nil
	row.InviterWasAgent = binding.InviterWasAgent
	row.RebateSettledAt = now
	r.row(inviterID).AffCount++
	return &AffiliateInviterBindingResult{InviteeID: userID, InviterID: inviterID, BoundAt: now, Source: binding.Source}, true, nil
}

func isFirstRechargeType(rt string) bool {
	return rt == AffiliateRewardInviteFixed || rt == AffiliateRewardInviteeBonus
}

func (r *fakeAffiliateRepo) hasReward(source int64, rt string) bool {
	for _, e := range r.ledger {
		if e.sourceUserID == source && e.rewardType == rt {
			return true
		}
	}
	return false
}

func (r *fakeAffiliateRepo) AccrueQuota(_ context.Context, beneficiaryID, sourceUserID int64, amount float64, freezeHours int, source AffiliateAccrueSource, rewardType string) (bool, error) {
	if amount <= 0 {
		return false, nil
	}
	if isFirstRechargeType(rewardType) && r.hasReward(sourceUserID, rewardType) {
		return false, nil
	}
	row, ok := r.rows[beneficiaryID]
	if !ok {
		return false, nil
	}
	r.ledger = append(r.ledger, fakeLedgerEntry{beneficiaryID, sourceUserID, amount, "accrue", rewardType, freezeHours, source})
	if freezeHours > 0 {
		row.AffFrozenQuota += amount
	} else {
		row.AffQuota += amount
	}
	row.AffHistoryQuota += amount
	return true, nil
}

func (r *fakeAffiliateRepo) GetAccruedRebateFromInvitee(_ context.Context, beneficiaryID, sourceUserID int64, rewardTypes []string) (float64, error) {
	var total float64
	for _, e := range r.ledger {
		if e.userID != beneficiaryID || e.sourceUserID != sourceUserID || e.action != "accrue" {
			continue
		}
		for _, rt := range rewardTypes {
			if e.rewardType == rt {
				total += e.amount
			}
		}
	}
	return total, nil
}

func (r *fakeAffiliateRepo) GetAffiliateNodes(_ context.Context, userIDs []int64) (map[int64]AffiliateNode, error) {
	out := map[int64]AffiliateNode{}
	for _, id := range userIDs {
		row, ok := r.rows[id]
		if !ok {
			continue
		}
		out[id] = AffiliateNode{
			UserID: id, AgentLevel: row.AgentLevel, AgentOwnerID: row.AgentOwnerID, InviterID: row.InviterID,
			CustomRate: row.AffRebateRatePercent, Active: !r.deleted[id], Email: fmt.Sprintf("u%d@x.com", id),
		}
	}
	return out, nil
}

func (r *fakeAffiliateRepo) CreditInviteeBonus(_ context.Context, userID int64, amount float64, orderID *int64) (bool, error) {
	if r.hasReward(userID, AffiliateRewardInviteeBonus) {
		return false, nil
	}
	r.ledger = append(r.ledger, fakeLedgerEntry{userID, userID, amount, "invitee_bonus", AffiliateRewardInviteeBonus, 0, AffiliateAccrueSource{OrderID: orderID}})
	r.balances[userID] += amount
	return true, nil
}

func (r *fakeAffiliateRepo) HasRewardForSource(_ context.Context, sourceUserID int64, rewardType string) (bool, error) {
	return r.hasReward(sourceUserID, rewardType), nil
}

func (r *fakeAffiliateRepo) CountOwnedCustomers(_ context.Context, agentID int64) (int, error) {
	n := 0
	for _, row := range r.rows {
		if row.AgentOwnerID != nil && *row.AgentOwnerID == agentID {
			n++
		}
	}
	return n, nil
}

func (r *fakeAffiliateRepo) SumRewardTotals(_ context.Context, beneficiaryID int64) (AffiliateRewardTotals, error) {
	var t AffiliateRewardTotals
	for _, e := range r.ledger {
		if e.userID != beneficiaryID {
			continue
		}
		switch e.rewardType {
		case AffiliateRewardAgentDirect, AffiliateRewardAgentDiff:
			t.Agent += e.amount
		case AffiliateRewardInvitePercent, AffiliateRewardInviteFixed:
			t.Invite += e.amount
		case AffiliateRewardInviteeBonus:
			t.InviteeBonus += e.amount
		}
	}
	return t, nil
}

func (r *fakeAffiliateRepo) ThawFrozenQuota(context.Context, int64) (float64, error) { return 0, nil }
func (r *fakeAffiliateRepo) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	return 0, 0, nil
}
func (r *fakeAffiliateRepo) WithdrawQuota(context.Context, int64, float64, string) (*AffiliateWithdrawResult, error) {
	return nil, nil
}

func (r *fakeAffiliateRepo) ListInvitees(_ context.Context, inviterID int64, _ int) ([]AffiliateInvitee, error) {
	out := []AffiliateInvitee{}
	for _, row := range r.rows {
		if row.InviterID == nil || *row.InviterID != inviterID {
			continue
		}
		total, _ := r.GetAccruedRebateFromInvitee(context.Background(), inviterID, row.UserID, []string{
			AffiliateRewardAgentDirect, AffiliateRewardAgentDiff, AffiliateRewardInvitePercent, AffiliateRewardInviteFixed})
		out = append(out, AffiliateInvitee{
			UserID: row.UserID, Email: fmt.Sprintf("u%d@x.com", row.UserID), TotalRebate: total,
			AgentLevel: row.AgentLevel, AgentOwnerID: row.AgentOwnerID, AgentLevelSource: row.AgentLevelSource,
			AgentLevelLocked: row.AgentLevelLocked, AgentLevelUpdatedBy: row.AgentLevelUpdatedBy,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out, nil
}

func (r *fakeAffiliateRepo) CountQualifiedInvitees(_ context.Context, _ int64, rechargedOnly bool) (int, error) {
	r.countCalls++
	r.lastRechargedOnly = rechargedOnly
	return r.inviteeCount, nil
}

func (r *fakeAffiliateRepo) PromoteAgentLevelAuto(_ context.Context, userID int64, level int) (bool, error) {
	row := r.row(userID)
	if row.AgentLevelLocked || row.AgentLevel <= level {
		return false, nil
	}
	row.AgentLevel = level
	row.AgentLevelSource = AffiliateAgentLevelSourceAuto
	row.AgentLevelUpdatedBy = nil
	return true, nil
}

func (r *fakeAffiliateRepo) SetAgentLevelManual(_ context.Context, ids []int64, level int, operatorID int64) error {
	for _, id := range ids {
		row := r.row(id)
		if level > row.AgentLevel {
			row.AgentLevelLocked = true
		} else if level < row.AgentLevel {
			row.AgentLevelLocked = false
		}
		row.AgentLevel = level
		row.AgentLevelSource = AffiliateAgentLevelSourceManual
		op := operatorID
		row.AgentLevelUpdatedBy = &op
	}
	return nil
}

func (r *fakeAffiliateRepo) SetAgentLevelLocked(_ context.Context, ids []int64, locked bool) error {
	for _, id := range ids {
		r.row(id).AgentLevelLocked = locked
	}
	return nil
}

func (r *fakeAffiliateRepo) GetAffiliateForUpdate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	return r.EnsureUserAffiliate(ctx, userID)
}

func (r *fakeAffiliateRepo) SetAgentLevelByAgent(_ context.Context, targetID int64, level int, appointerID int64, locked bool) error {
	row := r.row(targetID)
	row.AgentLevel = level
	row.AgentLevelSource = AffiliateAgentLevelSourceAgent
	row.AgentLevelLocked = locked
	op := appointerID
	row.AgentLevelUpdatedBy = &op
	return nil
}

func (r *fakeAffiliateRepo) SetAgentOwnerByAdmin(_ context.Context, userID int64, ownerID *int64, adminID int64) error {
	row := r.row(userID)
	if row.InviterID == nil && row.AgentOwnerID == nil {
		row.RebateSettledAt = time.Now()
	}
	row.AgentOwnerID = ownerID
	row.AgentUplineID = nil
	row.AgentOwnerLevel = nil
	row.AgentOwnerBoundAt = nil
	if ownerID != nil {
		level := AffiliateAgentLevelTop
		now := time.Now()
		row.AgentOwnerLevel = &level
		row.AgentOwnerBoundAt = &now
	}
	op := adminID
	row.AgentOwnerSetBy = &op
	return nil
}

func (r *fakeAffiliateRepo) CountAgentAppointments(_ context.Context, appointerID int64) (int, error) {
	n := 0
	for _, row := range r.rows {
		if row.AgentLevel == AffiliateAgentLevel2 && row.AgentLevelSource == AffiliateAgentLevelSourceAgent &&
			row.AgentLevelUpdatedBy != nil && *row.AgentLevelUpdatedBy == appointerID {
			n++
		}
	}
	return n, nil
}

func (r *fakeAffiliateRepo) ListInviterIDs(context.Context, int64, int) ([]int64, error) { return nil, nil }
func (r *fakeAffiliateRepo) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *fakeAffiliateRepo) ListUsersWithPendingConsumption(context.Context, time.Duration, int) ([]int64, error) {
	return r.pendingUsers, nil
}
func (r *fakeAffiliateRepo) LockPendingConsumption(_ context.Context, userID int64, _ time.Duration) (*AffiliatePendingConsumption, error) {
	p, ok := r.pending[userID]
	if !ok {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}
func (r *fakeAffiliateRepo) AdvanceConsumptionWatermark(_ context.Context, userID int64, until time.Time, consumption float64) error {
	if p, ok := r.pending[userID]; ok {
		p.SettledAt = until
		p.SettledConsumption = consumption
	}
	r.advances[userID] = consumption
	return nil
}
func (r *fakeAffiliateRepo) UpdateUserAffCode(context.Context, int64, string) error { return nil }
func (r *fakeAffiliateRepo) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}
func (r *fakeAffiliateRepo) SetUserRebateRate(_ context.Context, userID int64, rate *float64) error {
	r.row(userID).AffRebateRatePercent = rate
	return nil
}
func (r *fakeAffiliateRepo) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	return nil
}
func (r *fakeAffiliateRepo) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}
func (r *fakeAffiliateRepo) ListAgentCustomers(context.Context, int64, int, int) ([]AffiliateAgentCustomer, int64, error) {
	return nil, 0, nil
}
func (r *fakeAffiliateRepo) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	return nil, 0, nil
}
func (r *fakeAffiliateRepo) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	return nil, 0, nil
}
func (r *fakeAffiliateRepo) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	return nil, 0, nil
}
func (r *fakeAffiliateRepo) GetAffiliateUserOverview(_ context.Context, userID int64) (*AffiliateUserOverview, error) {
	row := r.row(userID)
	return &AffiliateUserOverview{UserID: userID, AgentLevel: row.AgentLevel, CustomRate: row.AffRebateRatePercent}, nil
}

var _ AffiliateRepository = (*fakeAffiliateRepo)(nil)

// twoTierSettingRepo 支持 GetMultiple，奖励配置一次读取全部键。
type twoTierSettingRepo struct{ values map[string]string }

func (s *twoTierSettingRepo) Get(context.Context, string) (*Setting, error) { return nil, ErrSettingNotFound }
func (s *twoTierSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (s *twoTierSettingRepo) Set(_ context.Context, k, v string) error { s.values[k] = v; return nil }
func (s *twoTierSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := s.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}
func (s *twoTierSettingRepo) SetMultiple(_ context.Context, m map[string]string) error {
	for k, v := range m {
		s.values[k] = v
	}
	return nil
}
func (s *twoTierSettingRepo) GetAll(context.Context) (map[string]string, error) { return s.values, nil }
func (s *twoTierSettingRepo) Delete(_ context.Context, k string) error         { delete(s.values, k); return nil }

// newTwoTierService 默认：开启、充值基数、P=30、R2=20。
func newTwoTierService(repo *fakeAffiliateRepo, overrides map[string]string) (*AffiliateService, *twoTierSettingRepo) {
	values := map[string]string{
		SettingKeyAffiliateEnabled:         "true",
		SettingKeyAffiliateRebateBase:      AffiliateRebateBaseRecharge,
		SettingKeyAffiliateRebateRate:      "30",
		SettingKeyAffiliateAgentLevel2Rate: "20",
	}
	for k, v := range overrides {
		values[k] = v
	}
	settings := &twoTierSettingRepo{values: values}
	return NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil), settings
}

// rewardsBySource 汇总某来源用户在本次操作中产生的奖励：受益人 → 金额。
func rewardsBySource(repo *fakeAffiliateRepo, from int, source int64) map[int64]float64 {
	out := map[int64]float64{}
	for _, e := range repo.ledger[from:] {
		if e.sourceUserID == source && e.action == "accrue" {
			out[e.userID] = roundTo(out[e.userID]+e.amount, 8)
		}
	}
	return out
}

func consume(t *testing.T, svc *AffiliateService, repo *fakeAffiliateRepo, userID int64, amount float64) map[int64]float64 {
	t.Helper()
	from := len(repo.ledger)
	_, err := svc.AccrueInviteRebate(context.Background(), userID, amount)
	require.NoError(t, err)
	return rewardsBySource(repo, from, userID)
}

func bind(t *testing.T, svc *AffiliateService, invitee, inviter int64) {
	t.Helper()
	require.NoError(t, svc.BindInviterByCode(context.Background(), invitee, fmt.Sprintf("CODE%d", inviter)))
}

func setLevel(t *testing.T, svc *AffiliateService, userID int64, level int) {
	t.Helper()
	require.NoError(t, svc.AdminSetAgentLevel(context.Background(), []int64{userID}, level, 99))
}

const (
	userB  = int64(1) // 一级
	userA  = int64(2) // 二级
	userU1 = int64(3)
	userU2 = int64(4)
)

// newExampleChain 构造设计文档第 3 节示例：B(一级) → A(二级) → U1(普通) → U2(普通)，普通邀请比例 5%。
func newExampleChain(t *testing.T) (*AffiliateService, *fakeAffiliateRepo) {
	t.Helper()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{userB, userA, userU1, userU2, 5, 6, 7} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateInviteRewardMode: AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate: "5",
	})
	setLevel(t, svc, userB, 1)
	bind(t, svc, userA, userB)
	setLevel(t, svc, userA, 2)
	bind(t, svc, userU1, userA)
	bind(t, svc, userU2, userU1)
	return svc, repo
}

func TestTwoTier_BindSnapshotFollowsCurrentLevels(t *testing.T) {
	t.Parallel()
	_, repo := newExampleChain(t)
	a, u1, u2 := repo.rows[userA], repo.rows[userU1], repo.rows[userU2]
	// A 绑定时 B 已是一级：归属 B（一级），没有上方一级。
	require.Equal(t, userB, *a.AgentOwnerID)
	require.Equal(t, 1, *a.AgentOwnerLevel)
	require.Nil(t, a.AgentUplineID)
	// U1 邀请人 A 是二级：归属 A，上方一级 B。
	require.Equal(t, userA, *u1.AgentOwnerID)
	require.Equal(t, 2, *u1.AgentOwnerLevel)
	require.Equal(t, userB, *u1.AgentUplineID)
	// U2 邀请人 U1 是普通用户：继承 U1 的归属 A。
	require.Equal(t, userA, *u2.AgentOwnerID)
	require.Equal(t, userB, *u2.AgentUplineID)
}

func TestTwoTier_ExampleTable(t *testing.T) {
	t.Parallel()

	t.Run("initial", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		require.Equal(t, map[int64]float64{userA: 20, userB: 10, userU1: 5}, consume(t, svc, repo, userU2, 100))
		require.Equal(t, map[int64]float64{userB: 30}, consume(t, svc, repo, userA, 100))
		require.Empty(t, consume(t, svc, repo, userB, 100))
		// U1 由 A（二级）直接邀请：A 只拿分润，没有普通邀请奖励。
		require.Equal(t, map[int64]float64{userA: 20, userB: 10}, consume(t, svc, repo, userU1, 100))
	})

	t.Run("U1 promoted to level 2", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		setLevel(t, svc, userU1, 2)
		require.Equal(t, map[int64]float64{userA: 20, userB: 10, userU1: 5}, consume(t, svc, repo, userU2, 100))
		bind(t, svc, 5, userU1) // U4
		require.Equal(t, map[int64]float64{userU1: 20, userB: 10}, consume(t, svc, repo, 5, 100))
		require.Equal(t, map[int64]float64{userB: 30}, consume(t, svc, repo, userU1, 100))
	})

	t.Run("A promoted to level 1 keeps old split", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		setLevel(t, svc, userA, 1)
		require.Equal(t, map[int64]float64{userA: 20, userB: 10, userU1: 5}, consume(t, svc, repo, userU2, 100))
		bind(t, svc, 5, userA) // U5
		require.Equal(t, map[int64]float64{userA: 30}, consume(t, svc, repo, 5, 100))
	})

	t.Run("B demoted keeps old split, new binds skip B", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		setLevel(t, svc, userB, 3)
		require.Equal(t, map[int64]float64{userA: 20, userB: 10, userU1: 5}, consume(t, svc, repo, userU2, 100))
		bind(t, svc, 6, userA) // U6：A 上方已没有一级
		require.Nil(t, repo.rows[6].AgentUplineID)
		require.Equal(t, map[int64]float64{userA: 30}, consume(t, svc, repo, 6, 100))
	})

	t.Run("B deleted drops only B's share", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		repo.deleted[userB] = true
		require.Equal(t, map[int64]float64{userA: 20, userU1: 5}, consume(t, svc, repo, userU2, 100))
	})

	t.Run("level 2 without level 1 above gets the full pool", func(t *testing.T) {
		repo := newFakeAffiliateRepo()
		repo.row(10)
		repo.row(11)
		svc, _ := newTwoTierService(repo, nil)
		setLevel(t, svc, 10, 2)
		bind(t, svc, 11, 10)
		require.Equal(t, map[int64]float64{10: 30}, consume(t, svc, repo, 11, 100))
	})
}

func TestTwoTier_PoolIsFullyDistributedWhenLevel1Participates(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	got := consume(t, svc, repo, userU2, 100)
	require.InDelta(t, 30, got[userA]+got[userB], 1e-9)
}

func TestTwoTier_CustomRate(t *testing.T) {
	t.Parallel()
	rate := func(v float64) *float64 { return &v }

	t.Run("disabled ignores exclusive rates", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		repo.rows[userA].AffRebateRatePercent = rate(25)
		require.Equal(t, map[int64]float64{userA: 20, userB: 10, userU1: 5}, consume(t, svc, repo, userU2, 100))
	})

	t.Run("enabled overrides tier rate and upline takes the difference", func(t *testing.T) {
		svc, repo := newExampleChain(t)
		svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
			SettingKeyAffiliateEnabled:                "true",
			SettingKeyAffiliateRebateBase:             AffiliateRebateBaseRecharge,
			SettingKeyAffiliateRebateRate:             "30",
			SettingKeyAffiliateAgentLevel2Rate:        "20",
			SettingKeyAffiliateAgentCustomRateEnabled: "true",
		}}, nil)
		repo.rows[userA].AffRebateRatePercent = rate(25)
		require.Equal(t, map[int64]float64{userA: 25, userB: 5}, consume(t, svc, repo, userU2, 100))
		// 上级差额不为负：A 专属 25 高于 B 专属 22 时 B 拿 0。
		repo.rows[userB].AffRebateRatePercent = rate(22)
		require.Equal(t, map[int64]float64{userA: 25}, consume(t, svc, repo, userU2, 100))
	})
}

func TestTwoTier_NormalInviteRewardOnlyWhenInviterWasNormalAtBind(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateInviteRewardMode: AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate: "5",
	})
	bind(t, svc, 2, 1) // 1 是普通用户，无归属代理
	require.Equal(t, map[int64]float64{1: 5}, consume(t, svc, repo, 2, 100))

	// 邀请人之后升为代理：仍按绑定时快照拿普通邀请奖励，不拿分润。
	setLevel(t, svc, 1, 1)
	require.Equal(t, map[int64]float64{1: 5}, consume(t, svc, repo, 2, 100))

	// 邀请人绑定时已是代理：只拿分润。
	bind(t, svc, 3, 1)
	require.Equal(t, map[int64]float64{1: 30}, consume(t, svc, repo, 3, 100))
}

func TestTwoTier_PerInviteeCapsAreSeparatePerRewardKind(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateRebatePerInviteeCap:       "40",
		SettingKeyAffiliateInviteRewardMode:          AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate:          "10",
		SettingKeyAffiliateInviteRewardPerInviteeCap: "15",
	})
	setLevel(t, svc, 1, 1)
	bind(t, svc, 2, 3) // 2 由普通用户 3 邀请，归属无代理 → 只有普通邀请奖励
	require.Equal(t, map[int64]float64{3: 10}, consume(t, svc, repo, 2, 100))
	require.Equal(t, map[int64]float64{3: 5}, consume(t, svc, repo, 2, 100))
	require.Empty(t, consume(t, svc, repo, 2, 100))

	repo2 := newFakeAffiliateRepo()
	repo2.row(1)
	repo2.row(2)
	svc2, _ := newTwoTierService(repo2, map[string]string{SettingKeyAffiliateRebatePerInviteeCap: "40"})
	setLevel(t, svc2, 1, 1)
	bind(t, svc2, 2, 1)
	require.Equal(t, map[int64]float64{1: 30}, consume(t, svc2, repo2, 2, 100))
	require.Equal(t, map[int64]float64{1: 10}, consume(t, svc2, repo2, 2, 100))
	require.Empty(t, consume(t, svc2, repo2, 2, 100))
}

func TestTwoTier_DurationAndFreeze(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	old := time.Now().AddDate(0, 0, -10)
	repo.rows[userU2].InviterBoundAt = &old
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:                  "true",
		SettingKeyAffiliateRebateBase:               AffiliateRebateBaseRecharge,
		SettingKeyAffiliateRebateRate:               "30",
		SettingKeyAffiliateAgentLevel2Rate:          "20",
		SettingKeyAffiliateRebateDurationDays:       "5",
		SettingKeyAffiliateRebateFreezeHours:        "24",
		SettingKeyAffiliateInviteRewardMode:         AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate:         "5",
		SettingKeyAffiliateInviteRewardDurationDays: "30",
	}}, nil)
	// 代理利润已过期，普通邀请奖励仍在有效期内，且入账进入冻结。
	from := len(repo.ledger)
	require.Equal(t, map[int64]float64{userU1: 5}, consume(t, svc, repo, userU2, 100))
	require.Equal(t, 24, repo.ledger[from].freezeHours)
}

func TestTwoTier_ConsumptionBaseSkipsRechargeAccrual(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
	}}, nil)
	require.Empty(t, consume(t, svc, repo, userU2, 100))
}

func TestTwoTier_BindRejectsCycles(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, nil)
	setLevel(t, svc, 1, 2)
	bind(t, svc, 2, 1) // 2 归属 1
	// 邀请链成环：让 1 的邀请人变成 2。
	_, err := svc.AdminBindInviter(context.Background(), 1, 2, 99, "test")
	require.ErrorIs(t, err, ErrAffiliateCycle)
	_, err = svc.AdminPreviewInviterBinding(context.Background(), 1, 2)
	require.ErrorIs(t, err, ErrAffiliateCycle)

	// 归属链成环：3 由 2 邀请（继承归属 1），再让 1 绑定到 3。
	bind(t, svc, 3, 2)
	require.Equal(t, int64(1), *repo.rows[3].AgentOwnerID)
	_, err = svc.AdminBindInviter(context.Background(), 1, 3, 99, "test")
	require.ErrorIs(t, err, ErrAffiliateCycle)
}

func TestTwoTier_AdminPreviewReturnsSnapshot(t *testing.T) {
	t.Parallel()
	svc, _ := newExampleChain(t)
	preview, err := svc.AdminPreviewInviterBinding(context.Background(), 5, userU1)
	require.NoError(t, err)
	require.Equal(t, userA, *preview.AgentOwnerID)
	require.Equal(t, 2, *preview.AgentOwnerLevel)
	require.Equal(t, "u2@x.com", preview.AgentOwnerEmail)
	require.Equal(t, userB, *preview.AgentUplineID)
	require.Equal(t, "u1@x.com", preview.AgentUplineEmail)
}

// ---- first recharge ----

func TestFirstRecharge_InviteeBonusPercentWithThresholdAndCap(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:                 "true",
		SettingKeyAffiliateInviteeBonusMode:        AffiliateInviteeBonusModePercent,
		SettingKeyAffiliateInviteeBonusValue:       "10",
		SettingKeyAffiliateInviteeBonusMinRecharge: "10",
		SettingKeyAffiliateInviteeBonusMax:         "20",
	}}, nil)
	ctx := context.Background()

	// 未达门槛：不发，也不消耗资格。
	res, err := svc.ApplyFirstRechargeRewards(ctx, userU2, 5, nil)
	require.NoError(t, err)
	require.Zero(t, res.InviteeBonus)
	// 第一笔达到门槛：10% 封顶 20。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userU2, 500, nil)
	require.NoError(t, err)
	require.InDelta(t, 20, res.InviteeBonus, 1e-9)
	require.InDelta(t, 20, repo.balances[userU2], 1e-9)
	// 只发一次。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userU2, 500, nil)
	require.NoError(t, err)
	require.Zero(t, res.InviteeBonus)
	require.InDelta(t, 20, repo.balances[userU2], 1e-9)

	// 被代理直接邀请的人同样享有。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userU1, 50, nil)
	require.NoError(t, err)
	require.InDelta(t, 5, res.InviteeBonus, 1e-9)

	// 没有邀请人的用户不发。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userB, 50, nil)
	require.NoError(t, err)
	require.Zero(t, res.InviteeBonus)
}

func TestFirstRecharge_InviterFixedOnlyForNormalInviter(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:                 "true",
		SettingKeyAffiliateInviteRewardMode:        AffiliateInviteRewardModeFirstRecharge,
		SettingKeyAffiliateInviteRewardFixedAmount: "2",
		SettingKeyAffiliateInviteRewardMinRecharge: "10",
		SettingKeyAffiliateInviteeBonusMode:        AffiliateInviteeBonusModeFixed,
		SettingKeyAffiliateInviteeBonusValue:       "3",
		SettingKeyAffiliateInviteeBonusMinRecharge: "10",
	}}, nil)
	ctx := context.Background()

	// U2 由普通用户 U1 邀请：双方都拿。
	res, err := svc.ApplyFirstRechargeRewards(ctx, userU2, 10, nil)
	require.NoError(t, err)
	require.InDelta(t, 3, res.InviteeBonus, 1e-9)
	require.InDelta(t, 2, res.InviterReward, 1e-9)
	require.InDelta(t, 2, repo.rows[userU1].AffQuota, 1e-9)

	// U1 由二级代理 A 邀请：U1 拿首充奖励，A 不拿邀请人奖励。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userU1, 10, nil)
	require.NoError(t, err)
	require.InDelta(t, 3, res.InviteeBonus, 1e-9)
	require.Zero(t, res.InviterReward)

	// 邀请人奖励也只发一次。
	res, err = svc.ApplyFirstRechargeRewards(ctx, userU2, 100, nil)
	require.NoError(t, err)
	require.Zero(t, res.InviterReward)
	require.InDelta(t, 2, repo.rows[userU1].AffQuota, 1e-9)
}

func TestFirstRecharge_RechargedCountModeEvaluatesInviter(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	repo.row(1)
	repo.row(2)
	repo.inviteeCount = 1
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentCountMode:       AffiliateAgentCountModeRecharged,
		SettingKeyAffiliateAgentLevel2Threshold: "1",
	})
	bind(t, svc, 2, 1)
	repo.rows[1].AgentLevel = 3 // 绑定时 count=1 已触发升级，复位后验证首充同样触发评估
	_, err := svc.ApplyFirstRechargeRewards(context.Background(), 2, 10, nil)
	require.NoError(t, err)
	require.Equal(t, AffiliateAgentLevel2, repo.rows[1].AgentLevel)
	require.True(t, repo.lastRechargedOnly)
}

func TestFirstRecharge_AmountHelpers(t *testing.T) {
	t.Parallel()
	cfg := AffiliateRewardConfig{InviteeBonusMode: AffiliateInviteeBonusModePercent, InviteeBonusValue: 10, InviteeBonusMinRecharge: 10}
	require.Zero(t, inviteeBonusAmount(cfg, 9.99))
	require.InDelta(t, 100, inviteeBonusAmount(cfg, 1000), 1e-9)
	cfg.InviteeBonusMax = 20
	require.InDelta(t, 20, inviteeBonusAmount(cfg, 1000), 1e-9)
	cfg.InviteeBonusMode = AffiliateInviteeBonusModeOff
	require.Zero(t, inviteeBonusAmount(cfg, 1000))
}

// ---- appointment ----

func newAppointFixture(t *testing.T, overrides map[string]string) (*AffiliateService, *fakeAffiliateRepo) {
	t.Helper()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3, 4, 5} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, overrides)
	setLevel(t, svc, 1, 1)
	bind(t, svc, 2, 1)
	bind(t, svc, 3, 1)
	bind(t, svc, 5, 4) // 5 不是 1 的直属
	return svc, repo
}

func TestAppoint_SetAndRevoke(t *testing.T) {
	t.Parallel()
	svc, repo := newAppointFixture(t, nil)
	ctx := context.Background()
	require.NoError(t, svc.AppointAgentLevel(ctx, 1, 2, 2))
	require.Equal(t, AffiliateAgentLevel2, repo.rows[2].AgentLevel)
	require.Equal(t, AffiliateAgentLevelSourceAgent, repo.rows[2].AgentLevelSource)
	require.False(t, repo.rows[2].AgentLevelLocked)
	// 重复任命幂等。
	require.NoError(t, svc.AppointAgentLevel(ctx, 1, 2, 2))

	// 被任命的二级之后邀请的用户归他，差额回到任命人。
	bind(t, svc, 4, 2)
	require.Equal(t, int64(2), *repo.rows[4].AgentOwnerID)
	require.Equal(t, int64(1), *repo.rows[4].AgentUplineID)

	require.NoError(t, svc.AppointAgentLevel(ctx, 1, 2, 3))
	require.Equal(t, AffiliateAgentLevelDefault, repo.rows[2].AgentLevel)
	require.True(t, repo.rows[2].AgentLevelLocked)
	// 取消后再次任命仍允许（锁是任命人自己加的）。
	require.NoError(t, svc.AppointAgentLevel(ctx, 1, 2, 2))
}

func TestAppoint_Rejections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, repo := newAppointFixture(t, map[string]string{SettingKeyAffiliateAgentAppointMax: "1"})
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 1, 5, 2), ErrAffiliateAppointTargetInvalid)
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 2, 3, 2), ErrAffiliateAppointForbidden)
	require.NoError(t, svc.AppointAgentLevel(ctx, 1, 2, 2))
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 1, 3, 2), ErrAffiliateAppointLimitReached)

	// 管理员设置的二级不能由一级代理取消；管理员锁定的普通用户不能被任命。
	setLevel(t, svc, 3, 2)
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 1, 3, 3), ErrAffiliateAppointNotOwner)
	setLevel(t, svc, 3, 3)
	require.True(t, repo.rows[3].AgentLevelLocked)
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 1, 3, 2), ErrAffiliateAppointTargetInvalid)
	require.ErrorIs(t, svc.AppointAgentLevel(ctx, 1, 2, 1), ErrAffiliateAgentLevelInvalid)

	disabled, _ := newAppointFixture(t, map[string]string{SettingKeyAffiliateAgentAppointEnabled: "false"})
	require.ErrorIs(t, disabled.AppointAgentLevel(ctx, 1, 2, 2), ErrAffiliateAppointForbidden)

	// 任命人被降级后不能再任命 / 取消，已任命的二级保持不变。
	svc3, repo3 := newAppointFixture(t, nil)
	require.NoError(t, svc3.AppointAgentLevel(ctx, 1, 2, 2))
	setLevel(t, svc3, 1, 3)
	require.Equal(t, AffiliateAgentLevel2, repo3.rows[2].AgentLevel)
	require.ErrorIs(t, svc3.AppointAgentLevel(ctx, 1, 2, 3), ErrAffiliateAppointForbidden)
}

// ---- detail ----

func TestDetail_RoleSpecificFields(t *testing.T) {
	t.Parallel()
	svc, _ := newExampleChain(t)
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:           "true",
		SettingKeyAffiliateRebateBase:        AffiliateRebateBaseRecharge,
		SettingKeyAffiliateRebateRate:        "30",
		SettingKeyAffiliateAgentLevel2Rate:   "20",
		SettingKeyAffiliateInviteRewardMode:  AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate:  "5",
		SettingKeyAffiliateInviteeBonusMode:  AffiliateInviteeBonusModePercent,
		SettingKeyAffiliateInviteeBonusValue: "10",
	}}, nil)
	ctx := context.Background()

	normal, err := svc.GetAffiliateDetail(ctx, userU1)
	require.NoError(t, err)
	require.Nil(t, normal.AgentRatePercent)
	require.NotNil(t, normal.InviteReward)
	require.Equal(t, 5.0, normal.InviteReward.RatePercent)
	require.NotNil(t, normal.InviteeBonus)
	require.NotNil(t, normal.FirstRechargeBonus)

	agent, err := svc.GetAffiliateDetail(ctx, userA)
	require.NoError(t, err)
	require.Equal(t, 20.0, *agent.AgentRatePercent)
	require.Nil(t, agent.InviteReward)
	require.Equal(t, 2, agent.OwnedCustomerCount)
	require.Nil(t, agent.Appoint)

	top, err := svc.GetAffiliateDetail(ctx, userB)
	require.NoError(t, err)
	require.Equal(t, 30.0, *top.AgentRatePercent)
	require.NotNil(t, top.Appoint)
	require.Nil(t, top.FirstRechargeBonus) // 没有邀请人
	require.Len(t, top.Invitees, 1)
	require.True(t, top.Invitees[0].OwnedByMe)
	require.False(t, top.Invitees[0].Appointable) // A 已经是二级

	_, err = svc.ApplyFirstRechargeRewards(ctx, userU1, 100, nil)
	require.NoError(t, err)
	normal, err = svc.GetAffiliateDetail(ctx, userU1)
	require.NoError(t, err)
	require.Nil(t, normal.FirstRechargeBonus)
	require.InDelta(t, 10, normal.RewardTotals.InviteeBonus, 1e-9)
}

func TestDetail_AppointableFlags(t *testing.T) {
	t.Parallel()
	svc, _ := newAppointFixture(t, nil)
	require.NoError(t, svc.AppointAgentLevel(context.Background(), 1, 2, 2))
	detail, err := svc.GetAffiliateDetail(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, &AffiliateAppointInfo{Max: 0, Used: 1}, detail.Appoint)
	byID := map[int64]AffiliateInvitee{}
	for _, inv := range detail.Invitees {
		byID[inv.UserID] = inv
	}
	require.True(t, byID[2].AppointedByMe)
	require.False(t, byID[2].Appointable)
	require.True(t, byID[3].Appointable)
}

// ---- settings ----

func TestRewardConfig_NormalizesAndKeepsLevel2BelowPool(t *testing.T) {
	t.Parallel()
	cfg := parseAffiliateRewardConfig(map[string]string{
		SettingKeyAffiliateRebateRate:                "15",
		SettingKeyAffiliateAgentLevel2Rate:           "40",
		SettingKeyAffiliateInviteRewardMode:          "PERCENT",
		SettingKeyAffiliateInviteRewardRate:          "150",
		SettingKeyAffiliateInviteRewardPerInviteeCap: "-1",
		SettingKeyAffiliateInviteeBonusMode:          "bogus",
		SettingKeyAffiliateAgentAppointMax:           "-3",
	})
	require.Equal(t, 15.0, cfg.PoolRate)
	require.Equal(t, 15.0, cfg.Level2Rate)
	require.Equal(t, AffiliateInviteRewardModePercent, cfg.InviteMode)
	require.Equal(t, 100.0, cfg.InviteRate)
	require.Zero(t, cfg.InvitePerInviteeCap)
	require.Equal(t, AffiliateInviteeBonusModeOff, cfg.InviteeBonusMode)
	require.Zero(t, cfg.AppointMax)

	def := parseAffiliateRewardConfig(nil)
	require.Equal(t, AffiliateRebateRateDefault, def.PoolRate)
	require.Equal(t, AffiliateAgentLevel2RateDefault, def.Level2Rate)
	require.True(t, def.AppointEnabled)
	require.False(t, def.CustomRateEnabled)
}

// ---- agent level auto upgrade ----

func TestEvaluateAgentLevel_AutoPromotesByThresholdAndRespectsLock(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		level        int
		locked       bool
		mode         string
		count        int
		wantLevel    int
		wantPromoted bool
	}{
		{"manual mode never promotes", 3, false, AffiliateAgentUpgradeModeManual, 100, 3, false},
		{"below level2 threshold", 3, false, AffiliateAgentUpgradeModeAuto, 4, 3, false},
		{"reaches level2", 3, false, AffiliateAgentUpgradeModeAuto, 5, 2, true},
		{"jumps straight to level1", 3, false, AffiliateAgentUpgradeModeAuto, 20, 1, true},
		{"locked stays", 3, true, AffiliateAgentUpgradeModeAuto, 20, 3, false},
		{"already top", 1, false, AffiliateAgentUpgradeModeAuto, 20, 1, false},
		{"never demotes", 1, false, AffiliateAgentUpgradeModeAuto, 0, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeAffiliateRepo()
			row := repo.row(7)
			row.AgentLevel, row.AgentLevelLocked = tc.level, tc.locked
			repo.inviteeCount = tc.count
			svc, _ := newTwoTierService(repo, map[string]string{
				SettingKeyAffiliateAgentUpgradeMode:     tc.mode,
				SettingKeyAffiliateAgentLevel2Threshold: "5",
				SettingKeyAffiliateAgentLevel1Threshold: "20",
			})
			level, promoted, err := svc.EvaluateAgentLevel(context.Background(), 7)
			require.NoError(t, err)
			require.Equal(t, tc.wantLevel, level)
			require.Equal(t, tc.wantPromoted, promoted)
			require.Equal(t, tc.wantLevel, repo.rows[7].AgentLevel)
		})
	}
}

func TestEvaluateAgentLevel_ZeroThresholdLevelIsSkipped(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	repo.row(7)
	repo.inviteeCount = 30
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentLevel2Threshold: "0",
		SettingKeyAffiliateAgentLevel1Threshold: "20",
	})
	level, promoted, err := svc.EvaluateAgentLevel(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, promoted)
	require.Equal(t, 1, level)

	repo.inviteeCount = 10
	repo.rows[7].AgentLevel = 3
	_, promoted, err = svc.EvaluateAgentLevel(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, promoted)
}

func TestBindInviterByCode_TriggersAutoUpgradeOfInviter(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	repo.row(1)
	repo.row(2)
	repo.inviteeCount = 5
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentLevel2Threshold: "5",
	})
	bind(t, svc, 2, 1)
	require.Equal(t, AffiliateAgentLevel2, repo.rows[1].AgentLevel)
	// 升级只影响之后新绑定的关系：2 绑定时 1 还是普通用户。
	require.Nil(t, repo.rows[2].AgentOwnerID)
}

func TestAdminSetAgentLevel_ValidatesLevelAndRecordsOperator(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	svc, _ := newTwoTierService(repo, nil)
	ctx := context.Background()
	require.ErrorIs(t, svc.AdminSetAgentLevel(ctx, []int64{1}, 0, 9), ErrAffiliateAgentLevelInvalid)
	require.ErrorIs(t, svc.AdminSetAgentLevel(ctx, []int64{1}, 4, 9), ErrAffiliateAgentLevelInvalid)
	require.NoError(t, svc.AdminSetAgentLevel(ctx, []int64{0, -1}, 2, 9))
	require.Empty(t, repo.rows)
	require.NoError(t, svc.AdminSetAgentLevel(ctx, []int64{1, 0, 2}, 2, 9))
	require.Equal(t, int64(9), *repo.rows[1].AgentLevelUpdatedBy)
	require.Equal(t, AffiliateAgentLevelSourceManual, repo.rows[2].AgentLevelSource)
}

// ---- consumption settlement ----

func TestSettleConsumptionRebates_DistributesAndAdvancesWatermark(t *testing.T) {
	t.Parallel()
	svc, repo := newExampleChain(t)
	svc.settingService = NewSettingService(&twoTierSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:         "true",
		SettingKeyAffiliateRebateBase:      AffiliateRebateBaseConsumption,
		SettingKeyAffiliateRebateRate:      "30",
		SettingKeyAffiliateAgentLevel2Rate: "20",
	}}, nil)
	until := time.Now().Add(-affiliateSettleLag).Truncate(time.Second)
	repo.pendingUsers = []int64{userU2}
	repo.pending[userU2] = &AffiliatePendingConsumption{
		UserID: userU2, SettledAt: until.Add(-time.Hour), UntilAt: until, DeltaCost: 80, PaidTotal: 50,
	}
	from := len(repo.ledger)
	result, err := svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Users)
	// 计入返利的消费按真实充值封顶为 50。
	require.InDelta(t, 50, result.Consumption, 1e-9)
	require.Equal(t, map[int64]float64{userA: 10, userB: 5}, rewardsBySource(repo, from, userU2))
	require.InDelta(t, 50, repo.advances[userU2], 1e-9)
	require.NotNil(t, repo.ledger[from].source.UsageUntil)
}

func TestAdminSetAgentOwner_AffectsNewCustomersAndOwnConsumptionOnly(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3, 4, 5, 6} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, nil)
	ctx := context.Background()
	// 2 没有邀请人就被设为二级：没有上级一级。
	setLevel(t, svc, 1, 1)
	setLevel(t, svc, 2, 2)
	bind(t, svc, 3, 2)
	require.Nil(t, repo.rows[3].AgentUplineID)
	require.Equal(t, map[int64]float64{2: 30}, consume(t, svc, repo, 3, 100))
	require.Empty(t, consume(t, svc, repo, 2, 100))

	owner := int64(1)
	require.NoError(t, svc.AdminSetAgentOwner(ctx, 2, &owner, 99))
	a := repo.rows[2]
	require.Equal(t, int64(1), *a.AgentOwnerID)
	require.Equal(t, AffiliateAgentLevelTop, *a.AgentOwnerLevel)
	require.Equal(t, int64(99), *a.AgentOwnerSetBy)
	require.Nil(t, a.InviterID, "invite relation is untouched")

	// 已有客户 3 的快照不变；新客户 4 的上方一级为 1；2 本人消费归 1。
	require.Equal(t, map[int64]float64{2: 30}, consume(t, svc, repo, 3, 100))
	bind(t, svc, 4, 2)
	require.Equal(t, int64(1), *repo.rows[4].AgentUplineID)
	require.Equal(t, map[int64]float64{2: 20, 1: 10}, consume(t, svc, repo, 4, 100))
	require.Equal(t, map[int64]float64{1: 30}, consume(t, svc, repo, 2, 100))

	// 更换为 5，再清除。
	setLevel(t, svc, 5, 1)
	newOwner := int64(5)
	require.NoError(t, svc.AdminSetAgentOwner(ctx, 2, &newOwner, 99))
	bind(t, svc, 6, 2)
	require.Equal(t, int64(5), *repo.rows[6].AgentUplineID)
	require.Equal(t, int64(1), *repo.rows[4].AgentUplineID)
	require.NoError(t, svc.AdminSetAgentOwner(ctx, 2, nil, 99))
	require.Nil(t, repo.rows[2].AgentOwnerID)
	require.Nil(t, repo.rows[2].AgentOwnerLevel)
	require.Empty(t, consume(t, svc, repo, 2, 100))
}

func TestAdminSetAgentOwner_Rejections(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3, 4} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, nil)
	ctx := context.Background()
	setLevel(t, svc, 1, 1)
	setLevel(t, svc, 2, 2)
	ptr := func(v int64) *int64 { return &v }

	require.ErrorIs(t, svc.AdminSetAgentOwner(ctx, 3, ptr(1), 99), ErrAffiliateAgentOwnerTargetInvalid, "target must be level 2")
	require.ErrorIs(t, svc.AdminSetAgentOwner(ctx, 2, ptr(3), 99), ErrAffiliateAgentOwnerInvalid, "owner must be level 1")
	require.ErrorIs(t, svc.AdminSetAgentOwner(ctx, 2, ptr(2), 99), ErrAffiliateSelfInvite)
	repo.deleted[1] = true
	require.ErrorIs(t, svc.AdminSetAgentOwner(ctx, 2, ptr(1), 99), ErrAffiliateAgentOwnerInvalid, "deleted owner")
	delete(repo.deleted, 1)

	// 4 是一级，归属 2（二级）：把 2 挂到 4 下会成环。
	setLevel(t, svc, 4, 1)
	four := repo.rows[4]
	two := int64(2)
	four.AgentOwnerID = &two
	require.ErrorIs(t, svc.AdminSetAgentOwner(ctx, 2, ptr(4), 99), ErrAffiliateCycle)
	require.Nil(t, repo.rows[2].AgentOwnerID)
}

func TestAdminSetAgentOwner_KeepsInviteRewardEligibilityFromBind(t *testing.T) {
	t.Parallel()
	repo := newFakeAffiliateRepo()
	for _, id := range []int64{1, 2, 3, 4, 5} {
		repo.row(id)
	}
	svc, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateInviteRewardMode: AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate: "5",
	})
	ctx := context.Background()
	setLevel(t, svc, 1, 1)
	setLevel(t, svc, 5, 1)
	// 2 由一级 1 直接邀请后升为二级：邀请人是代理，不拿普通邀请奖励。
	bind(t, svc, 2, 1)
	setLevel(t, svc, 2, 2)
	owner := int64(5)
	require.NoError(t, svc.AdminSetAgentOwner(ctx, 2, &owner, 99))
	require.Equal(t, map[int64]float64{5: 30}, consume(t, svc, repo, 2, 100))

	// 4 由普通用户 3 邀请后被设为二级：改归属后 3 仍拿普通邀请奖励。
	bind(t, svc, 4, 3)
	setLevel(t, svc, 4, 2)
	require.NoError(t, svc.AdminSetAgentOwner(ctx, 4, &owner, 99))
	require.Equal(t, map[int64]float64{5: 30, 3: 5}, consume(t, svc, repo, 4, 100))
}
