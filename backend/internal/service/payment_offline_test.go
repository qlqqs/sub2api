//go:build unit

package service

// CUSTOM(fork): Receipt atomicity and accounting contracts, including rollback after a write.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

type offlineTestUserRepo struct {
	UserRepository
	client         *dbent.Client
	failAfterWrite bool
}

func (r *offlineTestUserRepo) clientFor(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}
func (r *offlineTestUserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	u, e := r.clientFor(ctx).User.Get(ctx, id)
	if e != nil {
		return nil, e
	}
	return &User{ID: u.ID, Email: u.Email, Username: u.Username, Balance: u.Balance}, nil
}
func (r *offlineTestUserRepo) AdjustBalance(ctx context.Context, id int64, delta float64) (BalanceChange, error) {
	u, e := r.clientFor(ctx).User.UpdateOneID(id).AddBalance(delta).Save(ctx)
	if e != nil {
		return BalanceChange{}, e
	}
	if r.failAfterWrite {
		return BalanceChange{}, errors.New("injected failure after balance write")
	}
	return BalanceChange{Old: u.Balance - delta, New: u.Balance}, nil
}
func offlineTestService(t *testing.T) (*PaymentService, AdminOfflineRechargeInput, *offlineTestUserRepo) {
	t.Helper()
	ctx := context.Background()
	c := newPaymentConfigServiceTestClient(t)
	u, e := c.User.Create().SetEmail("offline@test.local").SetPasswordHash("hash").SetBalance(5).Save(ctx)
	require.NoError(t, e)
	r := &offlineTestUserRepo{client: c}
	s := &PaymentService{entClient: c, userRepo: r}
	return s, AdminOfflineRechargeInput{UserID: u.ID, AdminID: 9, Amount: 110, PaidAmount: 100, OfflineChannel: "bank", TradeNo: " receipt-1 ", Notes: "cash received"}, r
}
func TestOfflineValidation(t *testing.T) {
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1), 1e9, 0.001, 10.999} {
		require.Equal(t, "OFFLINE_INVALID_AMOUNT", infraerrors.Reason(validateOfflineAmount(v)))
	}
	for _, v := range []float64{0.01, 100, 100.01, maxOfflineRechargeAmount} {
		require.NoError(t, validateOfflineAmount(v))
	}
	for _, c := range []struct {
		k, op string
		valid bool
	}{{"", "add", true}, {"gift", "set", true}, {"offline", "add", true}, {"offline", "set", false}, {"oops", "add", false}, {"offline", "subtract", false}} {
		require.Equal(t, c.valid, ValidateAdminBalanceKind(c.k, c.op) == nil)
	}
	in := AdminOfflineRechargeInput{PaidAmount: 1, OfflineChannel: "invalid"}
	require.Equal(t, "OFFLINE_INVALID_CHANNEL", infraerrors.Reason(validateOfflineReceipt(&in)))
	require.Equal(t, payment.TypeOffline, payment.GetBasePaymentType(payment.TypeOffline))
}
func TestOfflineCreateSplitDuplicateAndRefund(t *testing.T) {
	for _, tc := range []struct{ amount, paid, bonus float64 }{{110, 100, 10}, {90, 100, 0}, {100, 100, 0}} {
		t.Run(fmt.Sprint(tc.amount), func(t *testing.T) {
			s, in, _ := offlineTestService(t)
			ctx := context.Background()
			in.Amount = tc.amount
			in.PaidAmount = tc.paid
			u, e := s.AdminCreateOfflineRecharge(ctx, in)
			require.NoError(t, e)
			require.Equal(t, 5+tc.amount, u.Balance)
			o, e := s.entClient.PaymentOrder.Query().Only(ctx)
			require.NoError(t, e)
			require.Equal(t, tc.bonus, o.BonusAmount)
			require.Equal(t, tc.paid, o.PayAmount)
			require.Equal(t, "0", *o.ChannelFeeRate)
			require.Equal(t, payment.TypeOffline, o.PaymentType)
			require.Equal(t, OrderStatusCompleted, o.Status)
			require.Nil(t, o.ProviderInstanceID)
			require.Empty(t, o.ProviderSnapshot)
			require.WithinDuration(t, o.CreatedAt, *o.PaidAt, time.Millisecond)
			require.WithinDuration(t, o.CreatedAt, *o.CompletedAt, time.Millisecond)
			rc, e := s.entClient.RedeemCode.Query().Only(ctx)
			require.NoError(t, e)
			require.Equal(t, rc.Code, o.RechargeCode)
			require.Equal(t, AdjustmentTypeAdminBalance, rc.Type)
			_, e = s.AdminCreateOfflineRecharge(ctx, in)
			require.Equal(t, "OFFLINE_DUPLICATE_TRADE", infraerrors.Reason(e))
			require.Equal(t, 1, s.entClient.PaymentOrder.Query().CountX(ctx))
			_, e = s.validateRefundRequest(ctx, o.ID, u.ID)
			require.Equal(t, "OFFLINE_USER_REFUND_DISABLED", infraerrors.Reason(e))
			plan, _, e := s.PrepareRefund(ctx, o.ID, tc.amount/2, "offline refund", false, false)
			require.NoError(t, e)
			require.Equal(t, tc.paid/2, plan.GatewayAmount)
			result, e := s.gwRefund(ctx, plan)
			require.NoError(t, e)
			require.Equal(t, payment.ProviderStatusSuccess, result.Status)
			require.True(t, s.hasAuditLog(ctx, o.ID, "REFUND_OFFLINE"))
			e = s.HandlePaymentNotification(ctx, &payment.PaymentNotification{OrderID: o.OutTradeNo, Status: payment.NotificationStatusSuccess, Amount: tc.paid}, payment.TypeOffline)
			require.Equal(t, "OFFLINE_CALLBACK_REJECTED", infraerrors.Reason(e))
			_, e = s.AdminCancelOrder(ctx, o.ID)
			require.Error(t, e)
			require.Empty(t, s.reconcilePaid(ctx, o))
			st, e := s.GetDashboardStats(ctx, 1)
			require.NoError(t, e)
			require.Equal(t, tc.paid, st.TotalAmount["CNY"])
			require.Equal(t, "offline", st.PaymentMethods[0].Type)
		})
	}
}
func TestOfflineTransactionRollback(t *testing.T) {
	s, in, r := offlineTestService(t)
	ctx := context.Background()
	r.failAfterWrite = true
	_, e := s.AdminCreateOfflineRecharge(ctx, in)
	require.ErrorContains(t, e, "injected failure")
	u, e := r.GetByID(ctx, in.UserID)
	require.NoError(t, e)
	require.Equal(t, 5.0, u.Balance)
	require.Zero(t, s.entClient.RedeemCode.Query().CountX(ctx))
	require.Zero(t, s.entClient.PaymentOrder.Query().CountX(ctx))
	require.Zero(t, s.entClient.PaymentAuditLog.Query().CountX(ctx))
}
func TestOfflineBackfillConditionsAndNoSideEffects(t *testing.T) {
	s, in, r := offlineTestService(t)
	ctx := context.Background()
	at := time.Now().Add(-48 * time.Hour)
	rc, e := s.entClient.RedeemCode.Create().SetCode("old-admin-credit").SetType(AdjustmentTypeAdminBalance).SetValue(110).SetStatus(StatusUsed).SetUsedBy(in.UserID).SetUsedAt(at).Save(ctx)
	require.NoError(t, e)
	// Any accidental reward call panics; backfill must not use the affiliate service at all.
	s.affiliateService = &AffiliateService{}
	summary, e := s.AdminBackfillOfflineRecharge(ctx, rc.ID, in)
	require.NoError(t, e)
	require.Equal(t, OrderStatusCompleted, summary.Status)
	require.Equal(t, 110.0, summary.Amount)
	require.Zero(t, summary.RefundAmount)
	o, e := s.entClient.PaymentOrder.Get(ctx, summary.ID)
	require.NoError(t, e)
	require.WithinDuration(t, at, o.CreatedAt, time.Millisecond)
	require.Equal(t, 10.0, o.BonusAmount)
	u, e := r.GetByID(ctx, in.UserID)
	require.NoError(t, e)
	require.Equal(t, 5.0, u.Balance)
	require.True(t, s.hasAuditLog(ctx, o.ID, "OFFLINE_RECHARGE_BACKFILL"))
	require.False(t, s.hasAuditLog(ctx, o.ID, "AFFILIATE_REBATE_APPLIED"))
	_, e = s.AdminBackfillOfflineRecharge(ctx, rc.ID, in)
	require.Equal(t, "OFFLINE_ALREADY_LINKED", infraerrors.Reason(e))
	in.UserID++
	_, e = s.AdminBackfillOfflineRecharge(ctx, rc.ID, in)
	require.ErrorIs(t, e, ErrRedeemCodeNotFound)
	in.UserID--
	for _, typ := range []string{RedeemTypeBalance, AdjustmentTypeAdminConcurrency} {
		bad, e := s.entClient.RedeemCode.Create().SetCode("old-" + typ).SetType(typ).SetValue(10).SetStatus(StatusUsed).SetUsedBy(in.UserID).SetUsedAt(at).Save(ctx)
		require.NoError(t, e)
		_, e = s.AdminBackfillOfflineRecharge(ctx, bad.ID, in)
		require.Equal(t, "OFFLINE_INVALID_HISTORY", infraerrors.Reason(e))
	}
	for _, v := range []float64{0, -10} {
		bad, e := s.entClient.RedeemCode.Create().SetCode(fmt.Sprintf("admin-%g", v)).SetType(AdjustmentTypeAdminBalance).SetValue(v).SetStatus(StatusUsed).SetUsedBy(in.UserID).SetUsedAt(at).Save(ctx)
		require.NoError(t, e)
		_, e = s.AdminBackfillOfflineRecharge(ctx, bad.ID, in)
		require.Equal(t, "OFFLINE_INVALID_HISTORY", infraerrors.Reason(e))
	}
}

func TestOfflineRewardsUseOrderPrincipalAndAreIdempotent(t *testing.T) {
	s, in, _ := offlineTestService(t)
	ctx := context.Background()
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, s.entClient)
	repo := newFakeAffiliateRepo()
	repo.row(in.UserID)
	inviter := in.UserID + 100
	repo.row(inviter)
	repo.rows[in.UserID].InviterID = &inviter
	repo.inviteeCount = 1
	affiliate, _ := newTwoTierService(repo, map[string]string{
		SettingKeyAffiliateInviteRewardMode:     AffiliateInviteRewardModePercent,
		SettingKeyAffiliateInviteRewardRate:     "5",
		SettingKeyAffiliateInviteeBonusMode:     AffiliateInviteeBonusModePercent,
		SettingKeyAffiliateInviteeBonusValue:    "10",
		SettingKeyAffiliateAgentUpgradeMode:     AffiliateAgentUpgradeModeAuto,
		SettingKeyAffiliateAgentCountMode:       AffiliateAgentCountModeRecharged,
		SettingKeyAffiliateAgentLevel2Threshold: "1",
	})
	s.affiliateService = affiliate
	_, err := s.AdminCreateOfflineRecharge(ctx, in)
	require.NoError(t, err)
	o, err := s.entClient.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	require.InDelta(t, 10, repo.balances[in.UserID], 1e-9, "first recharge is 10%% of 100 principal, not 110 credit")
	require.Equal(t, AffiliateAgentLevel2, repo.rows[inviter].AgentLevel)
	require.Equal(t, 1, repo.countCalls)
	require.True(t, s.hasAuditLog(ctx, o.ID, "AFFILIATE_REBATE_APPLIED"))
	found := false
	for _, entry := range repo.ledger {
		if entry.rewardType == AffiliateRewardInvitePercent {
			found = true
			require.Equal(t, 5.0, entry.amount)
			require.Equal(t, o.ID, *entry.source.OrderID)
		}
	}
	require.True(t, found)
	before := len(repo.ledger)
	require.NoError(t, s.applyAffiliateRebateForOrder(ctx, o))
	s.applyFirstRechargeRewards(ctx, o)
	require.Len(t, repo.ledger, before)
	require.Equal(t, 10.0, repo.balances[in.UserID])
}

func TestOfflineFinanceRevenueOnlyRecognizesConsumedPrincipal(t *testing.T) {
	at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ principal, bonus, pay, consume, free, paid string }{{"100", "10", "100", "25", "10", "15"}, {"90", "0", "100", "45", "0", "50"}} {
		data := &FinanceData{
			Users:  []FinanceUser{{ID: 1, Balance: fd(tc.principal).Add(fd(tc.bonus)).Add(fd("5")).Sub(fd(tc.consume))}},
			Orders: []FinanceOrder{{ID: 2, UserID: 1, OrderType: "balance", PaymentType: "offline", CompletedAt: at, Principal: fd(tc.principal), Bonus: fd(tc.bonus), Pay: fd(tc.pay)}},
			Events: []FinanceUserEvent{{UserID: 1, Event: FinanceEvent{At: at, ID: 3, Kind: "free", Source: "admin_adjust_in", Amount: fd("5")}}, {UserID: 1, Event: FinanceEvent{At: at, Kind: "consume", Amount: fd(tc.consume).Add(fd("5"))}}},
		}
		// Separate gift remains free; receipt principal and gift portion come only from the order.
		data.Users[0].Balance = data.Users[0].Balance.Sub(fd("5"))
		r := financeReplays(data)[1]
		require.Equal(t, tc.principal, r.PaidIn.String())
		require.Equal(t, fd(tc.bonus).Add(fd("5")).String(), r.FreeIn.String())
		require.Equal(t, tc.paid, r.Days["2026-10-27"].RevenuePaid.String())
		require.Equal(t, fd(tc.free).Add(fd("5")).String(), r.Days["2026-10-27"].FreeConsumed.String())
		require.True(t, r.Difference.IsZero())
	}
}

// CUSTOM(fork): Both history and backfill expose the original balance refund, not cash.
func TestOfflineOrderSummaryRefundMetadata(t *testing.T) {
	for _, tc := range []struct {
		status string
		refund float64
	}{
		{OrderStatusCompleted, 0},
		{OrderStatusRefunded, 110},
		{OrderStatusPartiallyRefunded, 55.25},
		{OrderStatusRefundRequested, 0},
		{OrderStatusRefunding, 0},
		{OrderStatusRefundPending, 0},
		{OrderStatusRefundFailed, 0},
	} {
		t.Run(tc.status, func(t *testing.T) {
			order := &dbent.PaymentOrder{ID: 42, OutTradeNo: "offline-summary", PayAmount: 100, Amount: 110, Status: tc.status, RefundAmount: tc.refund}
			summary := offlineOrderSummary(order)
			require.Equal(t, &OfflineOrderSummary{ID: 42, OutTradeNo: "offline-summary", PaidAmount: 100, Amount: 110, Status: tc.status, RefundAmount: tc.refund}, summary)
			data, err := json.Marshal(summary)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(data, &fields))
			require.Equal(t, tc.status, fields["status"])
			require.Equal(t, 110.0, fields["amount"])
			require.Equal(t, tc.refund, fields["refund_amount"])
		})
	}
}
