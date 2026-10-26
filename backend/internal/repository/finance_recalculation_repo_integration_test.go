//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type financeTestGuard struct {
	acquireErr error
	releases   int
	releaseErr error
	fail       error
	actual     string
	calls      int
	expire     bool
}

func (g *financeTestGuard) Acquire(_ context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch) (string, func(), error) {
	release := func() {
		g.releases++
		var n int
		g.releaseErr = tx.QueryRowContext(context.Background(), `SELECT 1`).Scan(&n)
	}
	if g.acquireErr != nil {
		return "", release, g.acquireErr
	}
	if g.actual != "" {
		return g.actual, release, nil
	}
	return b.ConfigFingerprint, release, nil
}
func (g *financeTestGuard) Synchronize(ctx context.Context, tx *sql.Tx, b *service.FinanceRecalculationBatch, items []service.FinanceRecalculationItem) error {
	g.calls++
	if g.expire {
		_, err := tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET lease_until=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`, b.ID)
		if err != nil {
			return err
		}
	}
	// A transaction-local audit mutation must roll back with the business writes.
	_, err := tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET error_message='test-hook' WHERE id=$1`, b.ID)
	if err != nil {
		return err
	}
	return g.fail
}

type financeRecalcFixture struct {
	userID, accountID, keyID int64
	at                       time.Time
}

func financeRecalcData(t *testing.T) financeRecalcFixture {
	t.Helper()
	ctx := context.Background()
	c := testEntClient(t)
	suffix := time.Now().UnixNano()
	u := mustCreateUser(t, c, &service.User{Email: fmt.Sprintf("recalc-%d@test.local", suffix), Balance: 12})
	a := mustCreateAccount(t, c, &service.Account{Name: fmt.Sprintf("recalc-%d", suffix)})
	key := mustCreateApiKey(t, c, &service.APIKey{UserID: u.ID, Name: "recalc", Key: fmt.Sprintf("sk-recalc-%d", suffix)})
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM usage_logs WHERE user_id=$1`, `DELETE FROM payment_orders WHERE user_id=$1`, `DELETE FROM api_keys WHERE user_id=$1`, `DELETE FROM users WHERE id=$1`} {
			_, _ = integrationDB.ExecContext(ctx, q, u.ID)
		}
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, a.ID)
	})
	return financeRecalcFixture{u.ID, a.ID, key.ID, time.Date(2026, 1, 2, 2, 0, 0, 0, time.UTC)}
}
func financeRecalcCreate(t *testing.T, r service.FinanceRecalculationStore, f financeRecalcFixture, kind string) *service.FinanceRecalculationBatch {
	t.Helper()
	in := service.FinanceRecalculationCreate{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: kind, Start: "2026-01-02", End: "2026-01-02"}, ActorID: f.userID, IdempotencyKey: fmt.Sprintf("test-%d", time.Now().UnixNano())}
	if kind == service.FinanceRecalculationUsageCost {
		in.AccountID = &f.accountID
	}
	b, err := r.Create(context.Background(), in)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM finance_recalculation_items WHERE batch_id=$1`, b.ID)
		_, _ = integrationDB.Exec(`DELETE FROM finance_recalculation_batches WHERE id=$1`, b.ID)
	})
	duplicate, err := r.Create(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, b.ID, duplicate.ID)
	in.End = "2026-01-03"
	_, err = r.Create(context.Background(), in)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationState)
	require.Equal(t, time.Date(2026, 1, 1, 16, 0, 0, 0, time.UTC), b.StartAt.UTC())
	require.Equal(t, time.Date(2026, 1, 2, 16, 0, 0, 0, time.UTC), b.EndAt.UTC())
	return b
}
func financeRecalcClaim(t *testing.T, r service.FinanceRecalculationStore, id int64) *service.FinanceRecalculationBatch {
	t.Helper()
	b, err := r.Claim(context.Background(), "integration-worker", 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, id, b.ID)
	return b
}
func financeRecalcPreview(t *testing.T, r service.FinanceRecalculationStore, b *service.FinanceRecalculationBatch) service.FinanceRecalculationPreview {
	t.Helper()
	ctx := context.Background()
	candidates, err := r.LoadCandidates(ctx, b)
	require.NoError(t, err)
	preview := service.FinanceRecalculationPreview{ConfigFingerprint: "frozen-config-v1", Items: []service.FinanceRecalculationItem{}}
	for _, c := range candidates {
		i := service.FinanceRecalculationItem{Candidate: c}
		if c.Usage != nil {
			snapshot := service.FinanceUsageCostSnapshot{AccountID: c.Usage.AccountID, GroupID: c.Usage.GroupID, Model: c.Usage.UpstreamModel, DefaultMultiplier: "2"}
			if snapshot.Model == "" {
				snapshot.Model = c.Usage.Model
			}
			out, e := service.CalculateFinanceUsageCost(*c.Usage, snapshot)
			require.NoError(t, e)
			i.UsageResult = &out
			i.Status = out.Status
			i.Source = out.Source
			i.Reason = out.Reason
			preview.Config.Usage = append(preview.Config.Usage, snapshot)
		} else {
			out, e := service.CalculateFinancePaymentFee(*c.Payment)
			require.NoError(t, e)
			i.PaymentResult = &out
			i.Status = out.Status
			i.Source = "payment_provider"
			preview.Config.Payment = append(preview.Config.Payment, service.FinanceRecalculationPaymentRule{ProviderID: c.Payment.ProviderID, FeeRate: c.Payment.CurrentFeeRate})
		}
		preview.Items = append(preview.Items, i)
	}
	return preview
}
func financeRecalcUsage(t *testing.T, f financeRecalcFixture) {
	t.Helper()
	_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,input_tokens,output_tokens,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,billing_type,created_at) VALUES($1,$2,$3,'model',10,5,1.0000000001,0.1234567891,NULL,NULL,0,$4),($1,$2,$3,'model',10,5,2,0.2,0.4000000001,0.5,0,$4)`, f.userID, f.keyID, f.accountID, f.at)
	require.NoError(t, err)
}
func financeRecalcProtected(t *testing.T, f financeRecalcFixture) string {
	t.Helper()
	var raw string
	require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_build_object('user',(SELECT to_jsonb(u) FROM users u WHERE id=$1),'accounts',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM accounts a WHERE a.id IN (SELECT account_id FROM usage_logs WHERE user_id=$1)),'affiliates',(SELECT jsonb_agg(to_jsonb(a)) FROM user_affiliates a WHERE user_id=$1),'ledger',(SELECT jsonb_agg(to_jsonb(l) ORDER BY id) FROM user_affiliate_ledger l WHERE user_id=$1),'usage',(SELECT jsonb_agg(to_jsonb(u)-'account_stats_cost'-'account_rate_multiplier' ORDER BY id) FROM usage_logs u WHERE user_id=$1),'orders',(SELECT jsonb_agg(to_jsonb(o)-'channel_fee_rate' ORDER BY id) FROM payment_orders o WHERE user_id=$1))::text`, f.userID).Scan(&raw))
	return raw
}
func TestFinanceRecalculationUsageAtomicApplyAndSyncRecovery(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	before := financeRecalcProtected(t, f)
	guard := &financeTestGuard{}
	r := NewFinanceRecalculationRepository(integrationDB, guard)
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	preview := financeRecalcPreview(t, r, b)
	require.Len(t, preview.Items, 2)
	require.Equal(t, "1.0000000001", preview.Items[0].Candidate.Usage.TotalCost)
	require.Nil(t, preview.Items[0].Candidate.Usage.AccountStatsCost)
	require.Nil(t, preview.Items[0].Candidate.Usage.PricingAt)
	require.Nil(t, preview.Items[0].Candidate.Usage.CacheStorageCost)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview))
	require.Equal(t, before, financeRecalcProtected(t, f))
	ready, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, "1.20000000015", ready.OldTotal)
	require.Equal(t, "6.0000000002", ready.NewTotal)
	require.Equal(t, "4.80000000005", ready.Delta)
	_, err = r.QueueConfirm(ctx, b.ID, f.userID+1, preview.ConfigFingerprint, preview.ConfigFingerprint)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationActor)
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, "changed")
	require.ErrorIs(t, err, service.ErrFinanceRecalculationRuleChanged)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, preview.ConfigFingerprint)
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	b = financeRecalcClaim(t, r, b.ID)
	require.Equal(t, service.FinanceRecalculationApplying, b.Status)
	unsafe := NewFinanceRecalculationRepository(integrationDB)
	require.ErrorIs(t, unsafe.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationSyncRequired)
	guard.actual = "configuration-raced"
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationRuleChanged)
	require.Equal(t, 1, guard.releases)
	require.ErrorIs(t, guard.releaseErr, sql.ErrTxDone)
	guard.actual = ""
	guard.fail = errors.New("synthetic sync failure")
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), guard.fail)
	var rate *string
	require.NoError(t, integrationDB.QueryRow(`SELECT account_rate_multiplier::text FROM usage_logs WHERE user_id=$1 ORDER BY id LIMIT 1`, f.userID).Scan(&rate))
	require.Nil(t, rate)
	items, total, err := r.ListItems(ctx, b.ID, 50, 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.False(t, items[0].Applied)
	afterFailure, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Empty(t, afterFailure.ErrorMessage)
	require.Equal(t, 2, guard.releases)
	require.ErrorIs(t, guard.releaseErr, sql.ErrTxDone)
	guard.fail = nil
	guard.expire = true
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationState)
	guard.expire = false
	require.NoError(t, integrationDB.QueryRow(`SELECT account_rate_multiplier::text FROM usage_logs WHERE user_id=$1 ORDER BY id LIMIT 1`, f.userID).Scan(&rate))
	require.Nil(t, rate)
	require.NoError(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint))
	require.Equal(t, 4, guard.releases)
	require.ErrorIs(t, guard.releaseErr, sql.ErrTxDone)
	require.Equal(t, before, financeRecalcProtected(t, f))
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationState)
	items, _, err = r.ListItems(ctx, b.ID, 50, 0)
	require.NoError(t, err)
	require.True(t, items[0].Applied)
	require.Nil(t, items[0].UsageResult.OldAccountStatsCost)
	require.Nil(t, items[0].UsageResult.OldAccountRateMultiplier)
	require.NoError(t, r.Fail(ctx, b.ID, b.LeaseToken, "sync_failed", "secret exception must never be persisted"))
	failed, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, "Cache synchronization failed", failed.ErrorMessage)
	require.ErrorIs(t, r.RetrySync(ctx, b.ID, f.userID+1), service.ErrFinanceRecalculationActor)
	require.NoError(t, r.RetrySync(ctx, b.ID, f.userID))
	b = financeRecalcClaim(t, r, b.ID)
	require.Equal(t, service.FinanceRecalculationSyncing, b.Status)
	calls := guard.calls
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationState)
	require.Equal(t, calls, guard.calls)
	require.NoError(t, r.Complete(ctx, b.ID, b.LeaseToken))
	done, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationCompleted, done.Status)
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, "later config")
	require.NoError(t, err)
}
func TestFinanceRecalculationLeaseExpiryRecoveryAndBounds(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	r := NewFinanceRecalculationRepository(integrationDB)
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	oldToken := b.LeaseToken
	preview := financeRecalcPreview(t, r, b)
	_, err := integrationDB.Exec(`UPDATE finance_recalculation_batches SET lease_until=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`, b.ID)
	require.NoError(t, err)
	require.ErrorIs(t, r.RenewLease(ctx, b.ID, oldToken, time.Minute), service.ErrFinanceRecalculationState)
	b = financeRecalcClaim(t, r, b.ID)
	require.NotEqual(t, oldToken, b.LeaseToken)
	require.ErrorIs(t, r.SavePreview(ctx, b.ID, oldToken, preview), service.ErrFinanceRecalculationState)
	require.ErrorIs(t, r.Fail(ctx, b.ID, oldToken, "preview_failed", ""), service.ErrFinanceRecalculationState)
	incomplete := preview
	incomplete.Items = incomplete.Items[:1]
	require.ErrorIs(t, r.SavePreview(ctx, b.ID, b.LeaseToken, incomplete), service.ErrFinanceRecalculationSourceChanged)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview))
	_, err = integrationDB.Exec(`UPDATE finance_recalculation_batches SET expires_at=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`, b.ID)
	require.NoError(t, err)
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, preview.ConfigFingerprint)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationExpired)
	b = financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,billing_type,created_at) SELECT $1,$2,$3,'model',1,0.1,0,$4 FROM generate_series(1,2000)`, f.userID, f.keyID, f.accountID, f.at)
	require.NoError(t, err)
	_, err = r.LoadCandidates(ctx, b)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationTooLarge)
	require.NoError(t, r.Fail(ctx, b.ID, b.LeaseToken, "preview_too_large", ""))
	items, count, err := r.ListItems(ctx, b.ID, 50, 0)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, count)
}
func TestFinanceRecalculationSourceConflictsAndCutoff(t *testing.T) {
	for _, mutation := range []string{"change", "delete", "new-after-cutoff"} {
		t.Run(mutation, func(t *testing.T) {
			ctx := context.Background()
			f := financeRecalcData(t)
			financeRecalcUsage(t, f)
			guard := &financeTestGuard{}
			r := NewFinanceRecalculationRepository(integrationDB, guard)
			b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
			b = financeRecalcClaim(t, r, b.ID)
			p := financeRecalcPreview(t, r, b)
			require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, p))
			_, err := r.QueueConfirm(ctx, b.ID, f.userID, p.ConfigFingerprint, p.ConfigFingerprint)
			require.NoError(t, err)
			b = financeRecalcClaim(t, r, b.ID)
			switch mutation {
			case "change":
				_, err = integrationDB.Exec(`UPDATE usage_logs SET input_tokens=input_tokens+1 WHERE id=$1 AND created_at=$2`, p.Items[1].Candidate.TargetID, f.at)
			case "delete":
				_, err = integrationDB.Exec(`DELETE FROM usage_logs WHERE id=$1 AND created_at=$2`, p.Items[1].Candidate.TargetID, f.at)
			default:
				_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,billing_type,created_at) VALUES($1,$2,$3,'model',9,0.1,0,$4)`, f.userID, f.keyID, f.accountID, b.CutoffAt.Add(time.Second))
			}
			require.NoError(t, err)
			err = r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint)
			if mutation == "new-after-cutoff" {
				require.NoError(t, err)
				var rate *string
				require.NoError(t, integrationDB.QueryRow(`SELECT account_rate_multiplier::text FROM usage_logs WHERE user_id=$1 AND total_cost=9`, f.userID).Scan(&rate))
				require.Nil(t, rate)
			} else {
				require.ErrorIs(t, err, service.ErrFinanceRecalculationSourceChanged)
				var rate *string
				require.NoError(t, integrationDB.QueryRow(`SELECT account_rate_multiplier::text FROM usage_logs WHERE id=$1 AND created_at=$2`, p.Items[0].Candidate.TargetID, f.at).Scan(&rate))
				require.Nil(t, rate)
				require.Zero(t, guard.calls)
			}
		})
	}
}
func TestFinanceRecalculationPaymentRefundFingerprintAndNull(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	c := testEntClient(t)
	provider, err := c.PaymentProviderInstance.Create().SetProviderKey("easypay").SetConfig(`{"key":"never-copy-me"}`).SetChannelFeeRate("0.0320").Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM payment_provider_instances WHERE id=$1`, provider.ID) })
	code := fmt.Sprintf("REC-%d", time.Now().UnixNano())
	o, err := c.PaymentOrder.Create().SetUserID(f.userID).SetUserEmail("private@example.invalid").SetUserName("private user").SetAmount(10).SetPayAmount(10).SetRechargeCode(code).SetOutTradeNo(code).SetPaymentType("wxpay").SetPaymentTradeNo("private trade").SetClientIP("127.0.0.1").SetSrcHost("localhost").SetStatus("COMPLETED").SetCompletedAt(f.at).SetExpiresAt(f.at.Add(time.Hour)).SetProviderInstanceID(fmt.Sprint(provider.ID)).Save(ctx)
	require.NoError(t, err)
	guard := &financeTestGuard{}
	r := NewFinanceRecalculationRepository(integrationDB, guard)
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationPaymentFee)
	b = financeRecalcClaim(t, r, b.ID)
	p := financeRecalcPreview(t, r, b)
	require.Len(t, p.Items, 1)
	require.Nil(t, p.Items[0].Candidate.Payment.OriginalFeeRate)
	require.NotContains(t, string(p.Items[0].Candidate.Raw), "private")
	require.NotContains(t, string(p.Items[0].Candidate.Raw), "never-copy")
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, p))
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, p.ConfigFingerprint, p.ConfigFingerprint)
	require.NoError(t, err)
	b = financeRecalcClaim(t, r, b.ID)
	_, err = integrationDB.Exec(`UPDATE payment_orders SET status='PARTIALLY_REFUNDED',refund_amount=1,refund_at=clock_timestamp() WHERE id=$1`, o.ID)
	require.NoError(t, err)
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint), service.ErrFinanceRecalculationSourceChanged)
	require.NoError(t, r.Fail(ctx, b.ID, b.LeaseToken, "source_changed", ""))
	b = financeRecalcCreate(t, r, f, service.FinanceRecalculationPaymentFee)
	b = financeRecalcClaim(t, r, b.ID)
	p = financeRecalcPreview(t, r, b)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, p))
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, p.ConfigFingerprint, p.ConfigFingerprint)
	require.NoError(t, err)
	b = financeRecalcClaim(t, r, b.ID)
	before := financeRecalcProtected(t, f)
	require.NoError(t, r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint))
	require.Equal(t, before, financeRecalcProtected(t, f))
	var rate string
	require.NoError(t, integrationDB.QueryRow(`SELECT channel_fee_rate::text FROM payment_orders WHERE id=$1`, o.ID).Scan(&rate))
	require.Equal(t, "0.0320", rate)
	items, _, err := r.ListItems(ctx, b.ID, 50, 0)
	require.NoError(t, err)
	require.Nil(t, items[0].PaymentResult.OriginalFeeRate)
	require.Equal(t, "0", *items[0].Delta)
}

func TestFinanceRecalculationSourceImageEvidenceAndSyncLeaseRecovery(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	_, err := integrationDB.Exec(`UPDATE usage_logs SET input_tokens=100,image_input_tokens=60,cache_read_tokens=50,image_size_breakdown='{"image_cache_read_tokens":40,"unrelated_sensitive_key":"not-audit"}' WHERE user_id=$1`, f.userID)
	require.NoError(t, err)
	r := NewFinanceRecalculationRepository(integrationDB, &financeTestGuard{})
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	p := financeRecalcPreview(t, r, b)
	require.Equal(t, 20, p.Items[0].Candidate.Usage.Tokens.ImageInputTokens)
	require.Equal(t, 40, p.Items[0].Candidate.Usage.Tokens.ImageCacheReadTokens)
	require.True(t, p.Items[0].Candidate.Usage.ImageCacheSplitKnown)
	require.NotContains(t, string(p.Items[0].Candidate.Raw), "not-audit")
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, p))
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, p.ConfigFingerprint, p.ConfigFingerprint)
	require.NoError(t, err)
	b = financeRecalcClaim(t, r, b.ID)
	require.NoError(t, r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint))
	oldToken := b.LeaseToken
	_, err = integrationDB.Exec(`UPDATE finance_recalculation_batches SET lease_until=clock_timestamp()-INTERVAL '1 second' WHERE id=$1`, b.ID)
	require.NoError(t, err)
	require.ErrorIs(t, r.Complete(ctx, b.ID, oldToken), service.ErrFinanceRecalculationState)
	b = financeRecalcClaim(t, r, b.ID)
	require.Equal(t, service.FinanceRecalculationSyncing, b.Status)
	require.NotEqual(t, oldToken, b.LeaseToken)
	require.ErrorIs(t, r.Complete(ctx, b.ID, oldToken), service.ErrFinanceRecalculationState)
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint), service.ErrFinanceRecalculationState)
	require.NoError(t, r.Complete(ctx, b.ID, b.LeaseToken))
}

func TestFinanceRecalculationConcurrentClaimSingleWinner(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	r := NewFinanceRecalculationRepository(integrationDB)
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	var wg sync.WaitGroup
	claimed := make(chan *service.FinanceRecalculationBatch, 2)
	errs := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); v, e := r.Claim(ctx, "worker", time.Minute); claimed <- v; errs <- e }()
	}
	wg.Wait()
	close(claimed)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	winners := 0
	for v := range claimed {
		if v != nil {
			winners++
			require.Equal(t, b.ID, v.ID)
		}
	}
	require.Equal(t, 1, winners)
}

func TestFinanceRecalculationAcquireFailureReleasesAfterRollback(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	guard := &financeTestGuard{acquireErr: errors.New("synthetic acquire failure")}
	r := NewFinanceRecalculationRepository(integrationDB, guard)
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	p := financeRecalcPreview(t, r, b)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, p))
	_, err := r.QueueConfirm(ctx, b.ID, f.userID, p.ConfigFingerprint, p.ConfigFingerprint)
	require.NoError(t, err)
	b = financeRecalcClaim(t, r, b.ID)
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, p.ConfigFingerprint), guard.acquireErr)
	require.Equal(t, 1, guard.releases)
	require.ErrorIs(t, guard.releaseErr, sql.ErrTxDone)
	require.Zero(t, guard.calls)
	state, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationApplying, state.Status)
}

func TestFinanceRecalculationCoverageFailureIsDurable(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	r := NewFinanceRecalculationRepository(integrationDB, &financeTestGuard{fail: service.ErrFinanceRecalculationCoverageUnknown})
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	preview := financeRecalcPreview(t, r, b)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview))
	_, err := r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, preview.ConfigFingerprint)
	require.NoError(t, err)
	b = financeRecalcClaim(t, r, b.ID)
	require.ErrorIs(t, r.Apply(ctx, b.ID, b.LeaseToken, preview.ConfigFingerprint), service.ErrFinanceRecalculationCoverageUnknown)
	require.NoError(t, r.Fail(ctx, b.ID, b.LeaseToken, "coverage_unknown", "secret SQL must never persist"))
	failed, err := r.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationFailed, failed.Status)
	require.Equal(t, service.FinanceRecalculationApplying, failed.FailedPhase)
	require.Equal(t, "coverage_unknown", failed.ErrorCode)
	require.NotContains(t, failed.ErrorMessage, "secret")
	require.Empty(t, failed.LeaseToken)
	require.ErrorIs(t, r.RetrySync(ctx, b.ID, f.userID), service.ErrFinanceRecalculationState)
	items, _, err := r.ListItems(ctx, b.ID, 100, 0)
	require.NoError(t, err)
	for _, item := range items {
		require.False(t, item.Applied)
	}
	var unchanged int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs WHERE account_id=$1 AND account_rate_multiplier IS NULL`, f.accountID).Scan(&unchanged))
	require.Equal(t, 1, unchanged, "failed aggregate sync rolled back source cost changes")
}
func TestFinanceRecalculationCancelFencesPreviewAndRejectsConfirmed(t *testing.T) {
	ctx := context.Background()
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	r := NewFinanceRecalculationRepository(integrationDB)

	// Cancelling while the worker is still previewing fences its lease.
	b := financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	preview := financeRecalcPreview(t, r, b)
	_, err := r.Cancel(ctx, b.ID, f.userID+1)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationActor)
	cancelled, err := r.Cancel(ctx, b.ID, f.userID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationCancelled, cancelled.Status)
	require.ErrorIs(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview), service.ErrFinanceRecalculationState)
	require.ErrorIs(t, r.RenewLease(ctx, b.ID, b.LeaseToken, time.Minute), service.ErrFinanceRecalculationState)
	again, err := r.Cancel(ctx, b.ID, f.userID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationCancelled, again.Status)
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, preview.ConfigFingerprint)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationState)
	next, err := r.Claim(ctx, "integration-worker", time.Minute)
	require.NoError(t, err)
	require.Nil(t, next)

	// A ready preview can be cancelled; a confirmed one cannot.
	b = financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	preview = financeRecalcPreview(t, r, b)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview))
	cancelled, err = r.Cancel(ctx, b.ID, f.userID)
	require.NoError(t, err)
	require.Equal(t, service.FinanceRecalculationCancelled, cancelled.Status)

	b = financeRecalcCreate(t, r, f, service.FinanceRecalculationUsageCost)
	b = financeRecalcClaim(t, r, b.ID)
	preview = financeRecalcPreview(t, r, b)
	require.NoError(t, r.SavePreview(ctx, b.ID, b.LeaseToken, preview))
	_, err = r.QueueConfirm(ctx, b.ID, f.userID, preview.ConfigFingerprint, preview.ConfigFingerprint)
	require.NoError(t, err)
	_, err = r.Cancel(ctx, b.ID, f.userID)
	require.ErrorIs(t, err, service.ErrFinanceRecalculationState)
}
