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

func (r *adminBindAffiliateRepoStub) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r *adminBindAffiliateRepoStub) GetAffiliateNodes(_ context.Context, ids []int64) (map[int64]AffiliateNode, error) {
	out := map[int64]AffiliateNode{}
	for _, id := range ids {
		if summary, ok := r.summaries[id]; ok {
			out[id] = AffiliateNode{UserID: id, AgentLevel: summary.AgentLevel, AgentOwnerID: summary.AgentOwnerID, InviterID: summary.InviterID, Active: true}
		}
	}
	return out, nil
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
