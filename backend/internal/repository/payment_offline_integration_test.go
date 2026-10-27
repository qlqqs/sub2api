//go:build integration

package repository

// CUSTOM(fork): Real PostgreSQL transaction propagation, locking and finance classification.
import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type offlineFailAfterBalanceRepo struct{ service.UserRepository }

func (r offlineFailAfterBalanceRepo) AdjustBalance(ctx context.Context, id int64, delta float64) (service.BalanceChange, error) {
	change, err := r.UserRepository.AdjustBalance(ctx, id, delta)
	if err != nil {
		return change, err
	}
	return change, errors.New("injected after real atomic SQL")
}
func TestOfflineRechargePostgres(t *testing.T) {
	ctx := context.Background()
	c := testEntClient(t)
	repo := NewUserRepository(c, integrationDB)
	u := mustCreateUser(t, c, &service.User{Email: fmt.Sprintf("offline-%d@test.local", time.Now().UnixNano())})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM payment_audit_logs WHERE order_id IN (SELECT id::text FROM payment_orders WHERE user_id=$1)`, u.ID)
		for _, q := range []string{`DELETE FROM redeem_codes WHERE used_by=$1`, `DELETE FROM payment_orders WHERE user_id=$1`, `DELETE FROM users WHERE id=$1`} {
			_, _ = integrationDB.ExecContext(ctx, q, u.ID)
		}
	})
	svc := service.NewPaymentService(c, nil, nil, nil, nil, nil, repo, nil, nil)
	input := service.AdminOfflineRechargeInput{UserID: u.ID, Amount: 110, PaidAmount: 100, OfflineChannel: "wechat", TradeNo: fmt.Sprintf("offline-%d", u.ID)}
	failSvc := service.NewPaymentService(c, nil, nil, nil, nil, nil, offlineFailAfterBalanceRepo{repo}, nil, nil)
	_, err := failSvc.AdminCreateOfflineRecharge(ctx, input)
	require.ErrorContains(t, err, "injected")
	actual, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Zero(t, actual.Balance)
	require.Zero(t, c.PaymentOrder.Query().Where(paymentorder.UserIDEQ(u.ID)).CountX(ctx))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM redeem_codes WHERE used_by=$1`, u.ID).Scan(&count))
	require.Zero(t, count)
	// Same receipt from simultaneous requests: exactly one commit, all others 409.
	runConcurrent := func(fn func() error, wantReason string) {
		t.Helper()
		var wg sync.WaitGroup
		results := make(chan error, 8)
		start := make(chan struct{})
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; results <- fn() }()
		}
		close(start)
		wg.Wait()
		close(results)
		successes := 0
		for e := range results {
			if e == nil {
				successes++
			} else {
				require.Equal(t, wantReason, infraerrors.Reason(e), "%v", e)
			}
		}
		require.Equal(t, 1, successes)
	}
	runConcurrent(func() error { _, e := svc.AdminCreateOfflineRecharge(ctx, input); return e }, "OFFLINE_DUPLICATE_TRADE")
	actual, err = repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 110.0, actual.Balance)
	at := time.Now().Add(-time.Hour)
	gift, err := c.RedeemCode.Create().SetCode(fmt.Sprintf("offline-gift-%d", u.ID)).SetType("admin_balance").SetValue(20).SetStatus("used").SetUsedBy(u.ID).SetUsedAt(at).Save(ctx)
	require.NoError(t, err)
	_, err = repo.AdjustBalance(ctx, u.ID, 20)
	require.NoError(t, err)
	now := time.Now()
	day := now.In(service.AffiliateTeamLocation).Format("2006-01-02")
	span, err := service.ParseFinanceRange(day, day, "day", now)
	require.NoError(t, err)
	finance := NewFinanceRepository(integrationDB)
	finSvc := service.NewFinanceService(finance)
	replay := func() map[string]any {
		t.Helper()
		rows, _, e := finSvc.Reconcile(ctx, 1, 10, false, u.Email)
		require.NoError(t, e)
		require.Len(t, rows, 1)
		return rows[0]
	}
	before := replay()
	require.Equal(t, "100", before["paid_in"])
	require.Equal(t, "30", before["free_in"])
	require.Equal(t, "100", before["paid_cash"])
	back := input
	back.PaidAmount = 15
	back.TradeNo = ""
	runConcurrent(func() error { _, e := svc.AdminBackfillOfflineRecharge(ctx, gift.ID, back); return e }, "OFFLINE_ALREADY_LINKED")
	actual, err = repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 130.0, actual.Balance)
	after := replay()
	require.Equal(t, "115", after["paid_in"])
	require.Equal(t, "15", after["free_in"])
	require.Equal(t, "115", after["paid_cash"])
	require.Equal(t, "0", after["difference"])
	// Replay real order + adjustment input through the production finance service.
	data, err := finance.Load(ctx, span)
	require.NoError(t, err)
	for _, ev := range data.Events {
		if ev.UserID == u.ID {
			require.NotEqual(t, gift.ID, ev.Event.ID, "linked backfill must not also appear as a free redeem event")
		}
	}
}

func TestOfflineRefundCashRatioPostgres(t *testing.T) {
	ctx := context.Background()
	c := testEntClient(t)
	u := mustCreateUser(t, c, &service.User{Email: fmt.Sprintf("offline-refund-%d@test.local", time.Now().UnixNano())})
	repo := NewUserRepository(c, integrationDB)
	svc := service.NewPaymentService(c, nil, nil, nil, nil, nil, repo, nil, nil)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM payment_audit_logs WHERE order_id IN (SELECT id::text FROM payment_orders WHERE user_id=$1)`, u.ID)
		for _, q := range []string{`DELETE FROM redeem_codes WHERE used_by=$1`, `DELETE FROM payment_orders WHERE user_id=$1`, `DELETE FROM users WHERE id=$1`} {
			_, _ = integrationDB.ExecContext(ctx, q, u.ID)
		}
	})
	day := time.Now().In(service.AffiliateTeamLocation).Format("2006-01-02")
	span, err := service.ParseFinanceRange(day, day, "day", time.Now())
	require.NoError(t, err)
	for _, tc := range []struct {
		amount, refund float64
		want           string
	}{{110, 110, "100"}, {90, 45, "50"}} {
		_, err := svc.AdminCreateOfflineRecharge(ctx, service.AdminOfflineRechargeInput{UserID: u.ID, Amount: tc.amount, PaidAmount: 100, OfflineChannel: "other"})
		require.NoError(t, err)
		o, err := c.PaymentOrder.Query().Where(paymentorder.UserIDEQ(u.ID)).Order(paymentorder.ByID()).All(ctx)
		require.NoError(t, err)
		last := o[len(o)-1]
		plan, result, err := svc.PrepareRefund(ctx, last.ID, tc.refund, "offline", false, true)
		require.NoError(t, err)
		require.Nil(t, result)
		result, err = svc.ExecuteRefund(ctx, plan)
		require.NoError(t, err)
		require.True(t, result.Success)
		data, err := NewFinanceRepository(integrationDB).Load(ctx, span)
		require.NoError(t, err)
		found := false
		for _, order := range data.Orders {
			if order.ID == last.ID {
				found = true
				require.Equal(t, tc.want, order.Refund.String())
				require.Equal(t, fmt.Sprint(tc.refund), order.RefundDeducted.String())
			}
		}
		require.True(t, found)
	}
}

func TestBalanceHistoryTotalsPostgres(t *testing.T) {
	tx := testEntTx(t)
	ctx := dbent.NewTxContext(context.Background(), tx)
	assertBalanceHistoryTotals(t, ctx, tx.Client())
}
