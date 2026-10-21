//go:build unit

package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 20%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (20%)
	require.InDelta(t, AffiliateRebateRateDefault,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}

type affiliateWithdrawCall struct {
	userID      int64
	amount      float64
	operationID string
}

type affiliateWithdrawRepoStub struct {
	AffiliateRepository
	calls  []affiliateWithdrawCall
	result *AffiliateWithdrawResult
	err    error
}

func (s *affiliateWithdrawRepoStub) WithdrawQuota(_ context.Context, userID int64, amount float64, operationID string) (*AffiliateWithdrawResult, error) {
	s.calls = append(s.calls, affiliateWithdrawCall{userID: userID, amount: amount, operationID: operationID})
	return s.result, s.err
}

const testAffiliateWithdrawKey = "affiliate-withdraw-7-3f2b8c1e"

// TestAdminWithdrawQuota_RejectsInvalidAmount 覆盖线下提现金额校验：非正数、
// NaN/Inf、舍入到 8 位小数后为 0 或溢出的金额都在服务层拒绝，不进入仓储。
func TestAdminWithdrawQuota_RejectsInvalidAmount(t *testing.T) {
	t.Parallel()
	for _, amount := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1), 1e-9, math.MaxFloat64} {
		repo := &affiliateWithdrawRepoStub{}
		svc := &AffiliateService{repo: repo}
		_, err := svc.AdminWithdrawQuota(context.Background(), 1, amount, testAffiliateWithdrawKey)
		require.ErrorIs(t, err, ErrAffiliateWithdrawAmountInvalid, "amount %v", amount)
		require.Empty(t, repo.calls, "amount %v must not reach repository", amount)
	}
}

// TestAdminWithdrawQuota_RejectsInvalidUser 验证缺少目标用户时直接拒绝。
func TestAdminWithdrawQuota_RejectsInvalidUser(t *testing.T) {
	t.Parallel()
	repo := &affiliateWithdrawRepoStub{}
	svc := &AffiliateService{repo: repo}
	_, err := svc.AdminWithdrawQuota(context.Background(), 0, 1, testAffiliateWithdrawKey)
	require.Error(t, err)
	require.Empty(t, repo.calls)
}

// TestAdminWithdrawQuota_RequiresValidIdempotencyKey 验证幂等键必填且须为
// 可见 ASCII、不超过 128 字符；不合格的键不进入仓储。
func TestAdminWithdrawQuota_RequiresValidIdempotencyKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key  string
		want error
	}{
		{key: "", want: ErrIdempotencyKeyRequired},
		{key: "   ", want: ErrIdempotencyKeyRequired},
		{key: "has space", want: ErrIdempotencyKeyInvalid},
		{key: "登记", want: ErrIdempotencyKeyInvalid},
		{key: strings.Repeat("k", 129), want: ErrIdempotencyKeyInvalid},
	}
	for _, tc := range cases {
		repo := &affiliateWithdrawRepoStub{}
		svc := &AffiliateService{repo: repo}
		_, err := svc.AdminWithdrawQuota(context.Background(), 1, 1, tc.key)
		require.ErrorIs(t, err, tc.want, "key %q", tc.key)
		require.Empty(t, repo.calls, "key %q must not reach repository", tc.key)
	}
}

// TestAdminWithdrawQuota_DerivesStableOperationID 验证仓储收到的 operation_id
// 由幂等键确定性派生：同一个键（含首尾空白）得到同一个值，不同的键得到不同的值，
// 且不是原始键本身。
func TestAdminWithdrawQuota_DerivesStableOperationID(t *testing.T) {
	t.Parallel()
	repo := &affiliateWithdrawRepoStub{result: &AffiliateWithdrawResult{LedgerID: 1}}
	svc := &AffiliateService{repo: repo}

	for _, key := range []string{testAffiliateWithdrawKey, "  " + testAffiliateWithdrawKey + " ", testAffiliateWithdrawKey + "-2"} {
		_, err := svc.AdminWithdrawQuota(context.Background(), 42, 10, key)
		require.NoError(t, err)
	}

	require.Len(t, repo.calls, 3)
	first := repo.calls[0].operationID
	require.Regexp(t, `^[0-9a-f]{64}$`, first)
	require.NotEqual(t, testAffiliateWithdrawKey, first)
	require.Equal(t, first, repo.calls[1].operationID)
	require.NotEqual(t, first, repo.calls[2].operationID)
}

// TestAdminWithdrawQuota_RoundsAmountToLedgerPrecision 验证金额按 8 位小数舍入后
// 交给仓储，仓储结果原样返回。
func TestAdminWithdrawQuota_RoundsAmountToLedgerPrecision(t *testing.T) {
	t.Parallel()
	want := &AffiliateWithdrawResult{LedgerID: 9, UserID: 42, Amount: 12.34567891}
	repo := &affiliateWithdrawRepoStub{result: want}
	svc := &AffiliateService{repo: repo}

	got, err := svc.AdminWithdrawQuota(context.Background(), 42, 12.3456789149, testAffiliateWithdrawKey)
	require.NoError(t, err)
	require.Same(t, want, got)
	require.Len(t, repo.calls, 1)
	require.Equal(t, int64(42), repo.calls[0].userID)
	require.Equal(t, 12.34567891, repo.calls[0].amount)
}

// TestAdminWithdrawQuota_PropagatesRepositoryErrors 验证额度不足与同键不同参数
// 由仓储判定，服务层原样返回，前端据错误码提示。
func TestAdminWithdrawQuota_PropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()
	for _, want := range []error{ErrAffiliateQuotaInsufficient, ErrIdempotencyKeyConflict} {
		repo := &affiliateWithdrawRepoStub{err: want}
		svc := &AffiliateService{repo: repo}
		_, err := svc.AdminWithdrawQuota(context.Background(), 1, 1, testAffiliateWithdrawKey)
		require.ErrorIs(t, err, want)
	}
}

type adminBindAffiliateRepoStub struct {
	AffiliateRepository
	summaries map[int64]*AffiliateSummary
	result    *AffiliateInviterBindingResult
	bound     bool
	binding   AffiliateInviterBinding
}

func (r *adminBindAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	summary, ok := r.summaries[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return summary, nil
}

func (r *adminBindAffiliateRepoStub) BindInviter(_ context.Context, inviteeID, inviterID int64, binding AffiliateInviterBinding) (*AffiliateInviterBindingResult, bool, error) {
	r.binding = binding
	return r.result, r.bound, nil
}

func TestAffiliateService_AdminBindInviter(t *testing.T) {
	t.Parallel()
	now := time.Now()
	repo := &adminBindAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{1: {UserID: 1}, 2: {UserID: 2}},
		result:    &AffiliateInviterBindingResult{InviteeID: 1, InviterID: 2, BoundAt: now, Source: "admin"},
		bound:     true,
	}
	svc := NewAffiliateService(repo, nil, nil, nil)
	result, err := svc.AdminBindInviter(context.Background(), 1, 2, 9, "  support verified  ")
	require.NoError(t, err)
	require.Equal(t, repo.result, result)
	require.Equal(t, "admin", repo.binding.Source)
	require.Equal(t, int64(9), *repo.binding.BoundBy)
	require.Equal(t, "support verified", *repo.binding.Reason)
}

func TestAffiliateService_AdminBindInviterRejectsInvalidAndPreviouslyBound(t *testing.T) {
	t.Parallel()
	svc := NewAffiliateService(&adminBindAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{1: {UserID: 1}, 2: {UserID: 2}},
	}, nil, nil, nil)

	_, err := svc.AdminBindInviter(context.Background(), 1, 1, 9, "reason")
	require.ErrorIs(t, err, ErrAffiliateSelfInvite)
	_, err = svc.AdminBindInviter(context.Background(), 1, 2, 9, "   ")
	require.ErrorIs(t, err, ErrAffiliateBindReason)
	_, err = svc.AdminBindInviter(context.Background(), 1, 2, 9, strings.Repeat("界", 501))
	require.ErrorIs(t, err, ErrAffiliateBindReason)

	boundAt := time.Now()
	svc.repo.(*adminBindAffiliateRepoStub).summaries[1].InviterBoundAt = &boundAt
	_, err = svc.AdminBindInviter(context.Background(), 1, 2, 9, "reason")
	require.ErrorIs(t, err, ErrAffiliateAlreadyBound)

	repo := svc.repo.(*adminBindAffiliateRepoStub)
	repo.summaries[1].InviterBoundAt = nil
	repo.bound = false
	_, err = svc.AdminBindInviter(context.Background(), 1, 2, 9, "reason")
	require.ErrorIs(t, err, ErrAffiliateAlreadyBound)
}

type multiLevelAffiliateRepoStub struct {
	AffiliateRepository
	summaries map[int64]*AffiliateSummary
	accrued   map[int64]float64
	calls     []multiLevelAccrueCall

	inviteeCount      int
	countCalls        int
	lastRechargedOnly bool
	promoteLevels     []int
	manualSets        []manualLevelSet
	bindOK            bool

	pendingUsers     []int64
	pending          map[int64]*AffiliatePendingConsumption
	advances         []watermarkAdvance
	listPendingCalls int
	txCount          int
}

type multiLevelAccrueCall struct {
	beneficiaryID int64
	inviteeID     int64
	amount        float64
	freezeHours   int
	level         int
	source        AffiliateAccrueSource
}

type manualLevelSet struct {
	ids   []int64
	level int
}

type watermarkAdvance struct {
	userID      int64
	until       time.Time
	consumption float64
}

func (r *multiLevelAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	summary, ok := r.summaries[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	cp := *summary
	return &cp, nil
}

func (r *multiLevelAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *multiLevelAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return nil, nil
}

func (r *multiLevelAffiliateRepoStub) GetAccruedRebateFromInvitee(_ context.Context, beneficiaryID, _ int64) (float64, error) {
	return r.accrued[beneficiaryID], nil
}

func (r *multiLevelAffiliateRepoStub) AccrueQuota(_ context.Context, beneficiaryID, inviteeID int64, amount float64, freezeHours int, source AffiliateAccrueSource, level int) (bool, error) {
	r.calls = append(r.calls, multiLevelAccrueCall{beneficiaryID: beneficiaryID, inviteeID: inviteeID, amount: amount, freezeHours: freezeHours, level: level, source: source})
	return true, nil
}

func (r *multiLevelAffiliateRepoStub) GetAffiliateByCode(_ context.Context, code string) (*AffiliateSummary, error) {
	for _, summary := range r.summaries {
		if summary.AffCode == code {
			cp := *summary
			return &cp, nil
		}
	}
	return nil, ErrAffiliateProfileNotFound
}

func (r *multiLevelAffiliateRepoStub) BindInviter(_ context.Context, userID, inviterID int64, _ AffiliateInviterBinding) (*AffiliateInviterBindingResult, bool, error) {
	if !r.bindOK {
		return nil, false, nil
	}
	r.summaries[userID].InviterID = &inviterID
	return &AffiliateInviterBindingResult{InviteeID: userID, InviterID: inviterID}, true, nil
}

func (r *multiLevelAffiliateRepoStub) CountQualifiedInvitees(_ context.Context, _ int64, rechargedOnly bool) (int, error) {
	r.countCalls++
	r.lastRechargedOnly = rechargedOnly
	return r.inviteeCount, nil
}

func (r *multiLevelAffiliateRepoStub) PromoteAgentLevelAuto(_ context.Context, userID int64, level int) (bool, error) {
	summary := r.summaries[userID]
	if summary.AgentLevelLocked || summary.AgentLevel <= level {
		return false, nil
	}
	summary.AgentLevel = level
	summary.AgentLevelSource = AffiliateAgentLevelSourceAuto
	r.promoteLevels = append(r.promoteLevels, level)
	return true, nil
}

func (r *multiLevelAffiliateRepoStub) SetAgentLevelManual(_ context.Context, ids []int64, level int) error {
	r.manualSets = append(r.manualSets, manualLevelSet{ids: ids, level: level})
	return nil
}

func (r *multiLevelAffiliateRepoStub) WithTx(ctx context.Context, fn func(context.Context) error) error {
	r.txCount++
	return fn(ctx)
}

func (r *multiLevelAffiliateRepoStub) ListUsersWithPendingConsumption(context.Context, time.Duration, int) ([]int64, error) {
	r.listPendingCalls++
	return r.pendingUsers, nil
}

func (r *multiLevelAffiliateRepoStub) LockPendingConsumption(_ context.Context, userID int64, _ time.Duration) (*AffiliatePendingConsumption, error) {
	pending, ok := r.pending[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	cp := *pending
	return &cp, nil
}

func (r *multiLevelAffiliateRepoStub) AdvanceConsumptionWatermark(_ context.Context, userID int64, until time.Time, consumption float64) error {
	r.advances = append(r.advances, watermarkAdvance{userID: userID, until: until, consumption: consumption})
	if p, ok := r.pending[userID]; ok {
		p.SettledAt = until
		p.SettledConsumption = consumption
	}
	return nil
}

type multiLevelSettingRepoStub struct {
	values map[string]string
}

func (s *multiLevelSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *multiLevelSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *multiLevelSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *multiLevelSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (s *multiLevelSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *multiLevelSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (s *multiLevelSettingRepoStub) Delete(context.Context, string) error { return nil }

// chain: 4 (source user) -> 3 -> 2 -> 1 -> none. Agents 3/2/1 default to the
// top level so chain-shape tests exercise the layer logic rather than gating.
func newMultiLevelChainRepo() *multiLevelAffiliateRepoStub {
	return &multiLevelAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			1: {UserID: 1, AgentLevel: 1},
			2: {UserID: 2, InviterID: ptrInt64(1), AgentLevel: 1},
			3: {UserID: 3, InviterID: ptrInt64(2), AgentLevel: 1},
			4: {UserID: 4, InviterID: ptrInt64(3), AgentLevel: 3},
		},
		accrued: map[int64]float64{},
	}
}

func newMultiLevelService(repo AffiliateRepository, values map[string]string) *AffiliateService {
	base := map[string]string{
		SettingKeyAffiliateEnabled:    "true",
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseRecharge,
	}
	for k, v := range values {
		base[k] = v
	}
	return NewAffiliateService(repo, NewSettingService(&multiLevelSettingRepoStub{values: base}, nil), nil, nil)
}

func TestAccrueInviteRebate_TwoLayerChainPaysEachLayerFromSourceAmount(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 11, total, 1e-9)
	require.Len(t, repo.calls, 2)
	require.Equal(t, multiLevelAccrueCall{beneficiaryID: 3, inviteeID: 4, amount: 8, level: 1}, repo.calls[0])
	require.Equal(t, multiLevelAccrueCall{beneficiaryID: 2, inviteeID: 4, amount: 3, level: 2}, repo.calls[1])
}

func TestAccrueInviteRebate_LevelsClampToTwoLayers(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "3",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 11, total, 1e-9)
	require.Len(t, repo.calls, 2)
}

func TestAccrueInviteRebate_DefaultLevelsKeepsSingleLevelBehavior(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "50",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 8, total, 1e-9)
	require.Len(t, repo.calls, 1)
	require.Equal(t, int64(3), repo.calls[0].beneficiaryID)
	require.Equal(t, 1, repo.calls[0].level)
}

func TestAccrueInviteRebate_ConsumptionBaseSkipsRechargeAccrual(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
		SettingKeyAffiliateRebateRate: "8",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, repo.calls)
}

func TestAccrueInviteRebate_AgentLevelGatesLayers(t *testing.T) {
	t.Parallel()
	settings := map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	}
	cases := []struct {
		name            string
		directLevel     int
		upperLevel      int
		wantTotal       float64
		wantBeneficiary []int64
	}{
		{"default level 3 earns nothing", 3, 3, 0, nil},
		{"level 2 direct earns layer 1 only", 2, 3, 8, []int64{3}},
		{"level 2 upper cannot take layer 2", 2, 2, 8, []int64{3}},
		{"level 1 upper takes layer 2 even when direct is level 3", 3, 1, 3, []int64{2}},
		{"level 1 both", 1, 1, 11, []int64{3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMultiLevelChainRepo()
			repo.summaries[3].AgentLevel = tc.directLevel
			repo.summaries[2].AgentLevel = tc.upperLevel
			svc := newMultiLevelService(repo, settings)

			total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
			require.NoError(t, err)
			require.InDelta(t, tc.wantTotal, total, 1e-9)
			got := make([]int64, 0, len(repo.calls))
			for _, call := range repo.calls {
				got = append(got, call.beneficiaryID)
			}
			require.Equal(t, tc.wantBeneficiary, nilIfEmpty(got))
		})
	}
}

func nilIfEmpty(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func TestAccrueInviteRebate_LevelsOpenedButUpperRateZeroSkipsUpperLedger(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 8, total, 1e-9)
	require.Len(t, repo.calls, 1)
}

func TestAccrueInviteRebate_ExclusiveRateOnlyAppliesToDirectLevel(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	exclusive := 50.0
	repo.summaries[3].AffRebateRatePercent = &exclusive
	repo.summaries[2].AffRebateRatePercent = &exclusive
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 53, total, 1e-9)
	require.Len(t, repo.calls, 2)
	require.InDelta(t, 50, repo.calls[0].amount, 1e-9)
	require.InDelta(t, 3, repo.calls[1].amount, 1e-9)
}

func TestAccrueInviteRebate_StopsWhenChainEndsEarly(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	repo.summaries[3].InviterID = nil
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 8, total, 1e-9)
	require.Len(t, repo.calls, 1)
}

func TestAccrueInviteRebate_DeletedUplineStopsChainWithoutError(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	delete(repo.summaries, 2)
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 8, total, 1e-9)
	require.Len(t, repo.calls, 1)

	delete(repo.summaries, 3)
	repo.calls = nil
	_, err = svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.ErrorIs(t, err, ErrUserNotFound)
	require.Empty(t, repo.calls)
}

func TestAccrueInviteRebate_CycleInChainTerminates(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	repo.summaries[3].InviterID = ptrInt64(4)
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 8, total, 1e-9)
	require.Len(t, repo.calls, 1)
}

func TestAccrueInviteRebate_PerInviteeCapAppliesPerBeneficiary(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	repo.accrued[3] = 5
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels:        "2",
		SettingKeyAffiliateRebateRate:          "8",
		SettingKeyAffiliateRebateRateL2:        "3",
		SettingKeyAffiliateRebatePerInviteeCap: "6",
	})

	total, err := svc.AccrueInviteRebate(context.Background(), 4, 100)
	require.NoError(t, err)
	require.InDelta(t, 4, total, 1e-9)
	require.Len(t, repo.calls, 2)
	require.InDelta(t, 1, repo.calls[0].amount, 1e-9)
	require.InDelta(t, 3, repo.calls[1].amount, 1e-9)
}

func TestGetAffiliateDetail_ExposesLevelRatesAndAgentLevel(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	exclusive := 12.5
	repo.summaries[2].AffRebateRatePercent = &exclusive
	repo.summaries[2].AgentLevel = 2
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateLevels:         "2",
		SettingKeyAffiliateRebateRate:           "8",
		SettingKeyAffiliateRebateRateL2:         "3",
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentLevel1Threshold: "10",
	})

	detail, err := svc.GetAffiliateDetail(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, 2, detail.RebateLevels)
	require.Equal(t, []float64{12.5, 3}, detail.LevelRebateRatePercents)
	require.InDelta(t, 12.5, detail.EffectiveRebateRatePercent, 1e-9)
	require.Equal(t, 2, detail.AgentLevel)
	require.Equal(t, 1, detail.UnlockedLayers)
	require.Equal(t, AffiliateRebateBaseRecharge, detail.RebateBase)
	require.Equal(t, AffiliateAgentUpgradeModeAuto, detail.AgentUpgradeMode)
	require.NotNil(t, detail.NextAgentLevel)
	require.Equal(t, 1, *detail.NextAgentLevel)
	require.Equal(t, 10, *detail.NextLevelThreshold)
	require.Equal(t, 4, detail.CurrentInviteCount)
}

func TestGetAffiliateDetail_ManualModeHasNoUpgradeProgress(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	svc := newMultiLevelService(repo, map[string]string{SettingKeyAffiliateRebateLevels: "2"})

	detail, err := svc.GetAffiliateDetail(context.Background(), 4)
	require.NoError(t, err)
	require.Equal(t, 3, detail.AgentLevel)
	require.Zero(t, detail.UnlockedLayers)
	require.Nil(t, detail.NextAgentLevel)
	require.Zero(t, repo.countCalls)
}

func TestSettingService_AffiliateRebateLevelsAndLevelRatesClamp(t *testing.T) {
	t.Parallel()
	svc := NewSettingService(&multiLevelSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateRebateLevels:          "9",
		SettingKeyAffiliateRebateRate:            "8",
		SettingKeyAffiliateRebateRateL2:          "150",
		SettingKeyAffiliateRebateBase:            "RECHARGE",
		SettingKeyAffiliateAgentUpgradeMode:      "weird",
		SettingKeyAffiliateAgentCountMode:        "Recharged",
		SettingKeyAffiliateAgentLevel2Threshold:  "-5",
		SettingKeyAffiliateAgentLevel1Threshold:  "99999999",
		SettingKeyAffiliateSettleIntervalMinutes: "1",
	}}, nil)
	ctx := context.Background()
	require.Equal(t, AffiliateRebateLevelsMax, svc.GetAffiliateRebateLevels(ctx))
	require.InDelta(t, 8, svc.GetAffiliateRebateRatePercentForLevel(ctx, 1), 1e-9)
	require.InDelta(t, AffiliateRebateRateMax, svc.GetAffiliateRebateRatePercentForLevel(ctx, 2), 1e-9)
	require.InDelta(t, 0, svc.GetAffiliateRebateRatePercentForLevel(ctx, 3), 1e-9)
	require.Equal(t, AffiliateRebateBaseRecharge, svc.GetAffiliateRebateBase(ctx))
	require.Equal(t, AffiliateAgentUpgradeModeManual, svc.GetAffiliateAgentUpgradeMode(ctx))
	require.Equal(t, AffiliateAgentCountModeRecharged, svc.GetAffiliateAgentCountMode(ctx))
	require.Equal(t, AffiliateAgentThresholdDefault, svc.GetAffiliateAgentLevelThreshold(ctx, 2))
	require.Equal(t, AffiliateAgentThresholdMax, svc.GetAffiliateAgentLevelThreshold(ctx, 1))
	require.Equal(t, AffiliateSettleIntervalMinutesMin, svc.GetAffiliateSettleIntervalMinutes(ctx))

	empty := NewSettingService(&multiLevelSettingRepoStub{values: map[string]string{}}, nil)
	require.Equal(t, AffiliateRebateLevelsDefault, empty.GetAffiliateRebateLevels(ctx))
	require.Equal(t, AffiliateRebateBaseConsumption, empty.GetAffiliateRebateBase(ctx))
	require.Equal(t, AffiliateSettleIntervalMinutesDefault, empty.GetAffiliateSettleIntervalMinutes(ctx))
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
			repo := &multiLevelAffiliateRepoStub{
				summaries:    map[int64]*AffiliateSummary{7: {UserID: 7, AgentLevel: tc.level, AgentLevelLocked: tc.locked}},
				inviteeCount: tc.count,
			}
			svc := newMultiLevelService(repo, map[string]string{
				SettingKeyAffiliateAgentUpgradeMode:     tc.mode,
				SettingKeyAffiliateAgentLevel2Threshold: "5",
				SettingKeyAffiliateAgentLevel1Threshold: "20",
			})
			level, promoted, err := svc.EvaluateAgentLevel(context.Background(), 7)
			require.NoError(t, err)
			require.Equal(t, tc.wantLevel, level)
			require.Equal(t, tc.wantPromoted, promoted)
			if promoted {
				require.Equal(t, []int{tc.wantLevel}, repo.promoteLevels)
			} else {
				require.Empty(t, repo.promoteLevels)
			}
		})
	}
}

func TestEvaluateAgentLevel_ZeroThresholdLevelIsSkipped(t *testing.T) {
	t.Parallel()
	repo := &multiLevelAffiliateRepoStub{
		summaries:    map[int64]*AffiliateSummary{7: {UserID: 7, AgentLevel: 3}},
		inviteeCount: 30,
	}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentLevel2Threshold: "0",
		SettingKeyAffiliateAgentLevel1Threshold: "20",
	})
	level, promoted, err := svc.EvaluateAgentLevel(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, promoted)
	require.Equal(t, 1, level)

	repo.inviteeCount = 10
	repo.summaries[7].AgentLevel = 3
	repo.promoteLevels = nil
	_, promoted, err = svc.EvaluateAgentLevel(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, promoted)
}

func TestEvaluateAgentLevel_UsesRechargedCountModeFlag(t *testing.T) {
	t.Parallel()
	repo := &multiLevelAffiliateRepoStub{summaries: map[int64]*AffiliateSummary{7: {UserID: 7, AgentLevel: 3}}}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentCountMode:       AffiliateAgentCountModeRecharged,
		SettingKeyAffiliateAgentLevel2Threshold: "1",
	})
	_, _, err := svc.EvaluateAgentLevel(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, 1, repo.countCalls)
	require.True(t, repo.lastRechargedOnly)
}

func TestBindInviterByCode_TriggersAutoUpgradeOfInviter(t *testing.T) {
	t.Parallel()
	repo := &multiLevelAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			1: {UserID: 1, AffCode: "INVITER1", AgentLevel: 3},
			2: {UserID: 2, AgentLevel: 3},
		},
		inviteeCount: 5,
		bindOK:       true,
	}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentLevel2Threshold: "5",
	})
	require.NoError(t, svc.BindInviterByCode(context.Background(), 2, "INVITER1"))
	require.Equal(t, []int{2}, repo.promoteLevels)
}

func TestAdminSetAgentLevel_ValidatesLevel(t *testing.T) {
	t.Parallel()
	repo := &multiLevelAffiliateRepoStub{}
	svc := newMultiLevelService(repo, nil)
	require.ErrorIs(t, svc.AdminSetAgentLevel(context.Background(), []int64{1}, 0), ErrAffiliateAgentLevelInvalid)
	require.ErrorIs(t, svc.AdminSetAgentLevel(context.Background(), []int64{1}, 4), ErrAffiliateAgentLevelInvalid)
	require.NoError(t, svc.AdminSetAgentLevel(context.Background(), []int64{0, -1}, 2))
	require.Empty(t, repo.manualSets)
	require.NoError(t, svc.AdminSetAgentLevel(context.Background(), []int64{1, 0, 2}, 2))
	require.Equal(t, []manualLevelSet{{ids: []int64{1, 2}, level: 2}}, repo.manualSets)
}

// ---- consumption settlement ----

func TestSettleConsumptionRebates_PaysChainAndAdvancesWatermark(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	until := time.Now().Add(-affiliateSettleLag).Truncate(time.Second)
	repo.pendingUsers = []int64{4}
	repo.pending = map[int64]*AffiliatePendingConsumption{
		4: {UserID: 4, SettledAt: until.Add(-time.Hour), UntilAt: until, DeltaCost: 100, PaidTotal: 500, SettledConsumption: 0},
	}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateBase:   AffiliateRebateBaseConsumption,
		SettingKeyAffiliateRebateLevels: "2",
		SettingKeyAffiliateRebateRate:   "8",
		SettingKeyAffiliateRebateRateL2: "3",
	})

	result, err := svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Users)
	require.InDelta(t, 100, result.Consumption, 1e-9)
	require.InDelta(t, 11, result.Rebate, 1e-9)
	require.Len(t, repo.calls, 2)
	require.Equal(t, 8.0, repo.calls[0].amount)
	require.NotNil(t, repo.calls[0].source.UsageUntil)
	require.True(t, repo.calls[0].source.UsageUntil.Equal(until))
	require.Nil(t, repo.calls[0].source.OrderID)
	require.Equal(t, []watermarkAdvance{{userID: 4, until: until, consumption: 100}}, repo.advances)
	require.Equal(t, 1, repo.txCount)
}

func TestSettleConsumptionRebates_CapsByRealRechargeAndStillAdvances(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	until := time.Now()
	repo.pendingUsers = []int64{4}
	repo.pending = map[int64]*AffiliatePendingConsumption{
		4: {UserID: 4, SettledAt: until.Add(-time.Hour), UntilAt: until, DeltaCost: 100, PaidTotal: 70, SettledConsumption: 50},
	}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
		SettingKeyAffiliateRebateRate: "10",
	})

	result, err := svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.InDelta(t, 20, result.Consumption, 1e-9)
	require.InDelta(t, 2, result.Rebate, 1e-9)
	require.Equal(t, []watermarkAdvance{{userID: 4, until: until, consumption: 70}}, repo.advances)

	// Fully consumed cap: no rebate, but the watermark still moves.
	repo.calls, repo.advances = nil, nil
	repo.pending[4] = &AffiliatePendingConsumption{UserID: 4, SettledAt: until, UntilAt: until.Add(time.Hour), DeltaCost: 30, PaidTotal: 70, SettledConsumption: 70}
	result, err = svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.Rebate)
	require.Empty(t, repo.calls)
	require.Len(t, repo.advances, 1)
	require.InDelta(t, 70, repo.advances[0].consumption, 1e-9)
}

func TestSettleConsumptionRebates_SkipsWhenRechargeBaseOrDisabled(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	repo.pendingUsers = []int64{4}
	svc := newMultiLevelService(repo, map[string]string{SettingKeyAffiliateRebateBase: AffiliateRebateBaseRecharge})
	result, err := svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.Users)
	require.Zero(t, repo.listPendingCalls)

	svc = newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateEnabled:    "false",
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
	})
	result, err = svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.Users)
	require.Zero(t, repo.listPendingCalls)
}

func TestSettleConsumptionRebates_DeletedUserIsSkipped(t *testing.T) {
	t.Parallel()
	repo := newMultiLevelChainRepo()
	until := time.Now()
	repo.pendingUsers = []int64{99, 4}
	repo.pending = map[int64]*AffiliatePendingConsumption{
		4: {UserID: 4, SettledAt: until.Add(-time.Hour), UntilAt: until, DeltaCost: 10, PaidTotal: 100},
	}
	svc := newMultiLevelService(repo, map[string]string{
		SettingKeyAffiliateRebateBase: AffiliateRebateBaseConsumption,
		SettingKeyAffiliateRebateRate: "10",
	})
	result, err := svc.SettleConsumptionRebates(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Users)
	require.InDelta(t, 1, result.Rebate, 1e-9)
}

func TestEligibleConsumption(t *testing.T) {
	t.Parallel()
	require.Zero(t, eligibleConsumption(nil))
	require.Zero(t, eligibleConsumption(&AffiliatePendingConsumption{DeltaCost: 0, PaidTotal: 10}))
	require.Zero(t, eligibleConsumption(&AffiliatePendingConsumption{DeltaCost: 5, PaidTotal: 0}))
	require.InDelta(t, 5, eligibleConsumption(&AffiliatePendingConsumption{DeltaCost: 5, PaidTotal: 10}), 1e-9)
	require.InDelta(t, 3, eligibleConsumption(&AffiliatePendingConsumption{DeltaCost: 5, PaidTotal: 10, SettledConsumption: 7}), 1e-9)
}

func TestAgentLevelLayerMatrix(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, agentLevelAllowedLayers(3))
	require.Equal(t, 1, agentLevelAllowedLayers(2))
	require.Equal(t, 2, agentLevelAllowedLayers(1))
	require.Equal(t, 0, agentLevelAllowedLayers(0))
	require.Equal(t, 0, agentLevelAllowedLayers(9))
	require.False(t, agentLevelAllowsLayer(2, 2))
	require.True(t, agentLevelAllowsLayer(1, 2))
	require.False(t, agentLevelAllowsLayer(1, 3))
}
