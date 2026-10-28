package repository

// CUSTOM(fork): Run the same accounting contract against SQLite and PostgreSQL.
import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBalanceHistoryTotals(t *testing.T) {
	_, client := newUserEntRepo(t)
	assertBalanceHistoryTotals(t, context.Background(), client)
}

func assertBalanceHistoryTotals(t *testing.T, ctx context.Context, c *dbent.Client) {
	t.Helper()
	// SQLite 的 Ent 建表不含原生迁移表；PostgreSQL 已有完整表，保持不变。
	_, err := c.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS user_affiliate_ledger (user_id bigint, action text, amount numeric, created_at timestamp, updated_at timestamp)`)
	require.NoError(t, err)
	u, err := c.User.Create().SetEmail(fmt.Sprintf("totals-%d@test.local", time.Now().UnixNano())).SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	other, err := c.User.Create().SetEmail(fmt.Sprintf("other-totals-%d@test.local", u.ID)).SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	repo := NewRedeemCodeRepository(c)
	got, err := repo.GetBalanceHistoryTotals(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, service.BalanceHistoryTotals{}, got)
	at := time.Now()
	redeem := func(code, kind string, value float64, uid int64) {
		t.Helper()
		_, e := c.RedeemCode.Create().SetCode(code).SetType(kind).SetValue(value).SetStatus("used").SetUsedBy(uid).SetUsedAt(at).Save(ctx)
		require.NoError(t, e)
	}
	prefix := fmt.Sprintf("totals-%d-", u.ID)
	redeem(prefix+"gift", "admin_balance", 20, u.ID)
	redeem(prefix+"redeem", "balance", 30, u.ID)
	redeem(prefix+"negative", "admin_balance", -15, u.ID)
	redeem(prefix+"zero", "balance", 0, u.ID)
	redeem(prefix+"concurrency", "admin_concurrency", 99, u.ID)
	redeem(prefix+"other", "balance", 99, other.ID)
	cases := []struct {
		name, method, kind, status       string
		amount, cash, bonus, refund, net float64
	}{
		{"online", "alipay", "balance", "COMPLETED", 110, 100, 10, 0, 100},
		{"offline", "offline", "balance", "COMPLETED", 110, 100, 10, 0, 100},
		{"online-partial", "alipay", "balance", "PARTIALLY_REFUNDED", 85, 80, 5, 20, 60},
		{"offline-partial", "offline", "balance", "PARTIALLY_REFUNDED", 100, 80, 20, 25, 60},
		{"offline-full", "offline", "balance", "REFUNDED", 110, 100, 10, 110, 0},
		{"offline-rounded", "offline", "balance", "PARTIALLY_REFUNDED", 3, 10, 0, 1, 6.67},
		{"subscription", "alipay", "subscription", "COMPLETED", 500, 400, 100, 0, 0},
		{"pending", "alipay", "balance", "PENDING", 500, 400, 100, 0, 0},
		{"cancelled", "alipay", "balance", "CANCELLED", 500, 400, 100, 0, 0},
		{"expired", "alipay", "balance", "EXPIRED", 500, 400, 100, 0, 0},
		{"failed", "alipay", "balance", "FAILED", 500, 400, 100, 0, 0},
	}
	for _, status := range []string{"REFUND_REQUESTED", "REFUNDING", "REFUND_PENDING", "REFUND_FAILED"} {
		cases = append(cases, struct {
			name, method, kind, status       string
			amount, cash, bonus, refund, net float64
		}{status, "alipay", "balance", status, 3, 1, 2, 1, 1})
	}
	_, err = c.ExecContext(ctx, `INSERT INTO user_affiliate_ledger (user_id, action, amount, created_at, updated_at) VALUES
 ($1, 'invitee_bonus', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
 ($1, 'invitee_signup_bonus', 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
 ($1, 'transfer', 99, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
 ($2, 'invitee_signup_bonus', 999, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, u.ID, other.ID)
	require.NoError(t, err)
	want := service.BalanceHistoryTotals{TotalGifted: 58}
	for _, tc := range cases {
		code := prefix + tc.name
		// Both online internal codes and offline admin adjustments must be excluded,
		// even for orders excluded from the paid balance aggregate.
		codeType := "balance"
		if tc.method == "offline" {
			codeType = "admin_balance"
		}
		redeem(code, codeType, tc.amount, u.ID)
		_, err = c.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName("").
			SetAmount(tc.amount).SetPayAmount(tc.cash).SetBonusAmount(tc.bonus).SetRefundAmount(tc.refund).SetRefundAt(at).
			SetRechargeCode(code).SetOutTradeNo(code).SetPaymentType(tc.method).SetPaymentTradeNo("").
			SetOrderType(tc.kind).SetStatus(tc.status).SetExpiresAt(at).SetCompletedAt(at).SetClientIP("").SetSrcHost("").Save(ctx)
		require.NoError(t, err)
		want.TotalRecharged += tc.net
		switch tc.status {
		case "COMPLETED", "PARTIALLY_REFUNDED", "REFUNDED", "REFUND_REQUESTED", "REFUNDING", "REFUND_PENDING", "REFUND_FAILED":
			if tc.kind == "balance" {
				want.TotalGifted += tc.bonus
			}
		}
		got, err = repo.GetBalanceHistoryTotals(ctx, u.ID)
		require.NoError(t, err)
		require.InDelta(t, want.TotalRecharged, got.TotalRecharged, 0.000001, tc.name+" cash")
		require.InDelta(t, want.TotalGifted, got.TotalGifted, 0.000001, tc.name+" gifts")
	}
	// Another user's receipts cannot leak into this user's totals.
	_, err = c.PaymentOrder.Create().SetUserID(other.ID).SetUserEmail(other.Email).SetUserName("").
		SetAmount(999).SetPayAmount(999).SetRechargeCode(prefix + "other-order").SetOutTradeNo(prefix + "other-order").
		SetPaymentType("offline").SetPaymentTradeNo("").SetStatus("COMPLETED").SetExpiresAt(at).SetClientIP("").SetSrcHost("").Save(ctx)
	require.NoError(t, err)
	got, err = repo.GetBalanceHistoryTotals(ctx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, want.TotalRecharged, got.TotalRecharged, 0.000001)
	require.InDelta(t, want.TotalGifted, got.TotalGifted, 0.000001)
}
