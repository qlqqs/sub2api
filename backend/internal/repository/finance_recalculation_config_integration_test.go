//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type financeConfigFixture struct {
	financeRecalcFixture
	groupID, channelID, providerID int64
}

func financeConfigData(t *testing.T) financeConfigFixture {
	t.Helper()
	ctx := context.Background()
	base := financeRecalcData(t)
	c := testEntClient(t)
	_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET platform='openai',type='apikey',rate_multiplier=0.3750,credentials='{"api_key":"ACCOUNT_SECRET_PROBE"}',extra='{"openai_long_context_billing_enabled":true,"token":"EXTRA_SECRET_PROBE"}' WHERE id=$1`, base.accountID)
	require.NoError(t, err)
	group := mustCreateGroup(t, c, &service.Group{Name: fmt.Sprintf("finance-config-%d", time.Now().UnixNano()), Platform: service.PlatformOpenAI, RateMultiplier: 1})
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM groups WHERE id=$1`, group.ID) })
	require.NoError(t, NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, base.accountID, []service.AccountModelCostRule{{Model: "gpt-*", RateMultiplier: 0.1234}, {Model: "gpt-5.4", RateMultiplier: 0}}))
	requestPrice, inputPrice, outputPrice, cachePrice := 0.25, 0.000002, 0.000004, 0.000001
	channel := &service.Channel{Name: fmt.Sprintf("finance-config-%d", time.Now().UnixNano()), Status: service.StatusActive, BillingModelSource: "requested", GroupIDs: []int64{group.ID}, ApplyPricingToAccountStats: true, FeaturesConfig: map[string]any{"api_key": "CHANNEL_SECRET_PROBE"}, ModelMapping: map[string]map[string]string{"openai": {"private": "MAPPING_SECRET_PROBE"}}, AccountStatsPricingRules: []service.AccountStatsPricingRule{
		{Name: "later", AccountIDs: []int64{}, SortOrder: 20, GroupIDs: []int64{group.ID}, Pricing: []service.ChannelModelPricing{{Platform: "openai", Models: []string{"gpt-*"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &requestPrice}}},
		{Name: "first", GroupIDs: []int64{}, SortOrder: 10, AccountIDs: []int64{base.accountID}, Pricing: []service.ChannelModelPricing{{Platform: "openai", Models: []string{"gpt-5.4"}, BillingMode: service.BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice, CacheWrite1hPrice: &cachePrice, ReasoningEffortMultipliers: map[string]float64{"high": 1.5}, Intervals: []service.PricingInterval{{MinTokens: 1000, InputPrice: &outputPrice, SortOrder: 3}}}}},
	}}
	require.NoError(t, NewChannelRepository(integrationDB).Create(ctx, channel))
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM channels WHERE id=$1`, channel.ID) })
	provider, err := c.PaymentProviderInstance.Create().SetProviderKey("easypay").SetConfig(`{"key":"PROVIDER_SECRET_PROBE"}`).SetChannelFeeRate("0.0123").Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM payment_provider_instances WHERE id=$1`, provider.ID) })
	return financeConfigFixture{financeRecalcFixture: base, groupID: group.ID, channelID: channel.ID, providerID: provider.ID}
}
func financeConfigUsageBatch(f financeConfigFixture) *service.FinanceRecalculationBatch {
	return &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}, Config: service.FinanceRecalculationConfig{Usage: []service.FinanceUsageCostSnapshot{{AccountID: f.accountID, GroupID: f.groupID, Model: "gpt-5.4"}}}}
}
func financeConfigRepository() *FinanceRecalculationConfigRepository {
	return NewFinanceRecalculationConfigRepository(integrationDB, service.NewBillingService(nil, nil))
}
func financeConfigFingerprint(t *testing.T, c service.FinanceRecalculationConfig) string {
	t.Helper()
	v, err := service.FinanceRecalculationConfigFingerprint(c)
	require.NoError(t, err)
	return v
}

func TestFinanceRecalculationConfigCurrentSnapshotScopesAndSecrets(t *testing.T) {
	ctx := context.Background()
	f := financeConfigData(t)
	r := financeConfigRepository()
	b := financeConfigUsageBatch(f)
	candidates := []service.FinanceRecalculationCandidate{
		{Usage: &service.FinanceUsageCostInput{AccountID: f.accountID, GroupID: f.groupID, Model: "ignored", RequestedModel: "ignored-too", UpstreamModel: " gpt-5.4 "}},
		{Usage: &service.FinanceUsageCostInput{AccountID: f.accountID, GroupID: f.groupID, Model: "gpt-5.4"}},
		{Usage: &service.FinanceUsageCostInput{AccountID: f.accountID, GroupID: f.groupID, Model: "ignored", RequestedModel: " z-custom "}},
		{Usage: &service.FinanceUsageCostInput{AccountID: f.accountID, Model: "gpt-5.4"}},
	}
	snapshot, err := r.Snapshot(ctx, b, candidates)
	require.NoError(t, err)
	require.Len(t, snapshot.Usage, 3)
	require.Zero(t, snapshot.Usage[0].GroupID)
	require.Nil(t, snapshot.Usage[0].Channel)
	first := snapshot.Usage[1]
	eligibleFrom, err := FinanceAggregationEligibleFrom(ctx, integrationDB)
	require.NoError(t, err)
	for _, scope := range snapshot.Usage {
		require.NotNil(t, scope.CoverageFrom)
		require.True(t, eligibleFrom.Equal(*scope.CoverageFrom))
	}
	require.Equal(t, "gpt-5.4", first.Model)
	require.Equal(t, "z-custom", snapshot.Usage[2].Model)
	require.Equal(t, "0.3750", first.DefaultMultiplier)
	require.Equal(t, "openai", first.Platform)
	require.True(t, first.LongContextPricingEnabled)
	require.Equal(t, []service.AccountModelCostRule{{Model: "gpt-*", RateMultiplier: 0.1234}, {Model: "gpt-5.4", RateMultiplier: 0}}, first.ModelRates)
	require.NotNil(t, first.Channel)
	require.Equal(t, f.channelID, first.Channel.ID)
	require.True(t, first.Channel.ApplyPricingToAccountStats)
	require.Len(t, first.Channel.Rules, 2)
	require.Equal(t, "first", first.Channel.Rules[0].Name)
	require.Equal(t, "later", first.Channel.Rules[1].Name)
	pricing := first.Channel.Rules[0].Pricing[0]
	require.Equal(t, 1.5, pricing.ReasoningEffortMultipliers["high"])
	require.NotNil(t, pricing.CacheWrite1hPrice)
	require.Len(t, pricing.Intervals, 1)
	require.Equal(t, 1000, pricing.Intervals[0].MinTokens)
	require.NotNil(t, first.ModelPricing)
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	for _, secret := range []string{"ACCOUNT_SECRET_PROBE", "EXTRA_SECRET_PROBE", "CHANNEL_SECRET_PROBE", "MAPPING_SECRET_PROBE", "PROVIDER_SECRET_PROBE", "credentials", "api_key"} {
		require.NotContains(t, string(raw), secret)
	}
	// Candidate ordering and duplicated scope rows do not alter the frozen digest.
	for left, right := 0, len(candidates)-1; left < right; left, right = left+1, right-1 {
		candidates[left], candidates[right] = candidates[right], candidates[left]
	}
	reordered, err := r.Snapshot(ctx, b, candidates)
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, snapshot), financeConfigFingerprint(t, reordered))
	b.Config = snapshot
	current, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, snapshot), financeConfigFingerprint(t, current))
	// Current SQL, not gateway/account-rule caches, observes the original writer.
	require.NoError(t, NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, f.accountID, []service.AccountModelCostRule{{Model: "gpt-5.4", RateMultiplier: 0.4321}}))
	current, err = r.Current(ctx, b)
	require.NoError(t, err)
	require.NotEqual(t, financeConfigFingerprint(t, snapshot), financeConfigFingerprint(t, current))
	require.Equal(t, 0.4321, current.Usage[1].ModelRates[0].RateMultiplier)
}

func TestFinanceRecalculationConfigMissingAndDeletedSources(t *testing.T) {
	ctx := context.Background()
	f := financeConfigData(t)
	r := financeConfigRepository()
	b := financeConfigUsageBatch(f)
	initial, err := r.Current(ctx, b)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE groups SET deleted_at=clock_timestamp() WHERE id=$1`, f.groupID)
	require.NoError(t, err)
	missingGroup, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, "missing_current_group", missingGroup.Usage[0].MissingReason)
	require.NotEqual(t, financeConfigFingerprint(t, initial), financeConfigFingerprint(t, missingGroup))
	_, err = integrationDB.Exec(`UPDATE groups SET deleted_at=NULL WHERE id=$1`, f.groupID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`DELETE FROM channels WHERE id=$1`, f.channelID)
	require.NoError(t, err)
	noChannel, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Nil(t, noChannel.Usage[0].Channel)
	require.Empty(t, noChannel.Usage[0].MissingReason)
	require.NotEqual(t, financeConfigFingerprint(t, initial), financeConfigFingerprint(t, noChannel))
	_, err = integrationDB.Exec(`UPDATE accounts SET deleted_at=clock_timestamp() WHERE id=$1`, f.accountID)
	require.NoError(t, err)
	missingAccount, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, "missing_current_account", missingAccount.Usage[0].MissingReason)
	b.Config.Usage = []service.FinanceUsageCostSnapshot{{AccountID: 1 << 60, GroupID: f.groupID, Model: "gpt-5.4"}}
	absent, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, "missing_current_account", absent.Usage[0].MissingReason)
}

func TestFinanceRecalculationConfigPaymentDedupZeroMissingAndDrift(t *testing.T) {
	ctx := context.Background()
	f := financeConfigData(t)
	r := financeConfigRepository()
	b := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationPaymentFee}}
	candidates := []service.FinanceRecalculationCandidate{{Payment: &service.FinancePaymentFeeInput{ProviderID: 1 << 60}}, {Payment: &service.FinancePaymentFeeInput{ProviderID: f.providerID}}, {Payment: &service.FinancePaymentFeeInput{ProviderID: f.providerID}}, {Payment: &service.FinancePaymentFeeInput{ProviderID: 0}}}
	initial, err := r.Snapshot(ctx, b, candidates)
	require.NoError(t, err)
	require.Len(t, initial.Payment, 2)
	require.Equal(t, f.providerID, initial.Payment[0].ProviderID)
	require.Equal(t, "0.0123", initial.Payment[0].FeeRate)
	require.Empty(t, initial.Payment[1].FeeRate)
	raw, err := json.Marshal(initial)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "PROVIDER_SECRET_PROBE")
	b.Config = initial
	_, err = integrationDB.Exec(`UPDATE payment_provider_instances SET channel_fee_rate=0 WHERE id=$1`, f.providerID)
	require.NoError(t, err)
	zero, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, "0.0000", zero.Payment[0].FeeRate)
	require.NotEqual(t, financeConfigFingerprint(t, initial), financeConfigFingerprint(t, zero))
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	fingerprint, release, err := r.Acquire(ctx, tx, b)
	if release != nil {
		defer release()
	}
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, zero), fingerprint)
	require.NotEqual(t, financeConfigFingerprint(t, initial), fingerprint)
	require.NoError(t, tx.Rollback())
	_, err = integrationDB.Exec(`DELETE FROM payment_provider_instances WHERE id=$1`, f.providerID)
	require.NoError(t, err)
	deleted, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Empty(t, deleted.Payment[0].FeeRate)
	require.NotEqual(t, financeConfigFingerprint(t, zero), financeConfigFingerprint(t, deleted))
}

func financeConfigWaitTableWriter(t *testing.T, ctx context.Context, table string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var blocked bool
		err := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation=$1::regclass AND mode='RowExclusiveLock' AND NOT granted)`, table).Scan(&blocked)
		return err == nil && blocked
	}, 5*time.Second, 10*time.Millisecond, "original writer must wait on SHARE table lock")
}
func TestFinanceRecalculationConfigAcquireBlocksOriginalWriters(t *testing.T) {
	for _, kind := range []string{service.FinanceRecalculationUsageCost, service.FinanceRecalculationPaymentFee} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			f := financeConfigData(t)
			r := financeConfigRepository()
			b := financeConfigUsageBatch(f)
			tables := []string{"accounts", "account_model_cost_multipliers", "groups", "channels", "channel_groups", "channel_account_stats_pricing_rules", "channel_account_stats_model_pricing", "channel_account_stats_pricing_intervals"}
			if kind == service.FinanceRecalculationPaymentFee {
				b.Kind = kind
				b.Config = service.FinanceRecalculationConfig{Payment: []service.FinanceRecalculationPaymentRule{{ProviderID: f.providerID}}}
				tables = []string{"payment_provider_instances"}
			}
			initial, err := r.Current(ctx, b)
			require.NoError(t, err)
			b.Config = initial
			tx, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			var release func()
			defer func() {
				_ = tx.Rollback()
				if release != nil {
					release()
				}
			}()
			actual, unlock, err := r.Acquire(ctx, tx, b)
			release = unlock
			require.NoError(t, err)
			require.Equal(t, financeConfigFingerprint(t, initial), actual)
			for _, table := range tables {
				var held bool
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND relation=$1::regclass AND mode='ShareLock' AND granted)`, table).Scan(&held))
				require.True(t, held, table)
			}
			done := make(chan error, 1)
			writerTable := "account_model_cost_multipliers"
			if kind == service.FinanceRecalculationUsageCost {
				go func() {
					done <- NewAccountModelCostRepository(integrationDB).ReplaceForAccount(ctx, f.accountID, []service.AccountModelCostRule{{Model: "gpt-5.4", RateMultiplier: 0.75}})
				}()
			} else {
				writerTable = "payment_provider_instances"
				go func() {
					_, e := integrationDB.ExecContext(ctx, `UPDATE payment_provider_instances SET channel_fee_rate=0.0250 WHERE id=$1`, f.providerID)
					done <- e
				}()
			}
			financeConfigWaitTableWriter(t, ctx, writerTable)
			require.NoError(t, tx.Commit())
			release()
			release = nil
			require.NoError(t, <-done)
			changed, err := r.Current(ctx, b)
			require.NoError(t, err)
			require.NotEqual(t, actual, financeConfigFingerprint(t, changed))
		})
	}
}

func TestFinanceRecalculationConfigAcquireSerializesOriginalChannelEditor(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f := financeConfigData(t)
	r := financeConfigRepository()
	b := financeConfigUsageBatch(f)
	channelRepo := NewChannelRepository(integrationDB)
	channel, err := channelRepo.GetByID(ctx, f.channelID)
	require.NoError(t, err)
	channel.ApplyPricingToAccountStats = false
	before, err := r.Current(ctx, b)
	require.NoError(t, err)
	b.Config = before
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var release func()
	defer func() {
		_ = tx.Rollback()
		if release != nil {
			release()
		}
	}()
	_, release, err = r.Acquire(ctx, tx, b)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- channelRepo.Update(ctx, channel) }()
	financeConfigWaitTableWriter(t, ctx, "channels")
	require.NoError(t, tx.Commit())
	release()
	release = nil
	require.NoError(t, <-done)
	after, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.False(t, after.Usage[0].Channel.ApplyPricingToAccountStats)
	require.NotEqual(t, financeConfigFingerprint(t, before), financeConfigFingerprint(t, after))
}

func TestFinanceRecalculationConfigAcquireCleanupLockOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f := financeConfigData(t)
	financeRecalcUsage(t, f.financeRecalcFixture)
	r := financeConfigRepository()
	b := financeConfigUsageBatch(f)
	current, err := r.Current(ctx, b)
	require.NoError(t, err)
	b.Config = current
	tx, err := integrationDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	require.NoError(t, err)
	var release func()
	defer func() {
		_ = tx.Rollback()
		if release != nil {
			release()
		}
	}()
	_, release, err = r.Acquire(ctx, tx, b)
	require.NoError(t, err)
	cleanup := NewUsageCleanupRepository(testEntClient(t), integrationDB)
	done := make(chan error, 1)
	go func() {
		_, e := cleanup.DeleteUsageLogsBatch(ctx, service.UsageCleanupFilters{StartTime: f.at.Add(-time.Second), EndTime: f.at.Add(time.Second), AccountID: &f.accountID}, 10)
		done <- e
	}()
	waitFinanceAggregationBlocked(t)
	// The blocked cleanup has not locked source rows yet. The repair can still
	// obtain all source row locks before releasing its configuration protection.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM usage_logs WHERE account_id=$1 FOR UPDATE`, f.accountID)
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	require.NoError(t, tx.Commit())
	release()
	release = nil
	require.NoError(t, <-done)
	var remaining int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs WHERE account_id=$1`, f.accountID).Scan(&remaining))
	require.Zero(t, remaining)
}

func TestFinanceRecalculationConfigAcquireLegacyDeleteAndTruncateLockOrder(t *testing.T) {
	for _, operation := range []string{"account-cascade-delete", "usage-truncate"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			f := financeConfigData(t)
			financeRecalcUsage(t, f.financeRecalcFixture)
			r := financeConfigRepository()
			b := financeConfigUsageBatch(f)
			current, err := r.Current(ctx, b)
			require.NoError(t, err)
			b.Config = current
			legacy, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = legacy.Rollback() }()
			table, mode := "accounts", "RowExclusiveLock"
			if operation == "usage-truncate" {
				table, mode = "usage_logs", "AccessExclusiveLock"
			}
			statement := `LOCK TABLE accounts IN ROW EXCLUSIVE MODE`
			if operation == "usage-truncate" {
				statement = `LOCK TABLE usage_logs IN ACCESS EXCLUSIVE MODE`
			}
			_, err = legacy.ExecContext(ctx, statement)
			require.NoError(t, err)
			repair, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = repair.Rollback() }()
			var repairPID int
			require.NoError(t, repair.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&repairPID))
			type acquired struct {
				release func()
				err     error
			}
			done := make(chan acquired, 1)
			go func() { _, release, e := r.Acquire(ctx, repair, b); done <- acquired{release, e} }()
			require.Eventually(t, func() bool {
				var blocked bool
				e := integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND relation=$2::regclass AND NOT granted)`, repairPID, table).Scan(&blocked)
				return e == nil && blocked
			}, 5*time.Second, 10*time.Millisecond, "repair must wait on original writer relation lock: "+mode)
			// It must NOT own the advisory lock while waiting for the relation: the
			// legacy source DELETE/TRUNCATE trigger acquires advisory in this order.
			var advisoryHeld bool
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted)`, repairPID).Scan(&advisoryHeld))
			require.False(t, advisoryHeld)
			// Execute real FK cascade/statement trigger, then roll back all test data.
			// TRUNCATE is strictly confined to this testcontainers database transaction.
			if operation == "account-cascade-delete" {
				_, err = legacy.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, f.accountID)
			} else {
				_, err = legacy.ExecContext(ctx, `TRUNCATE TABLE usage_logs CASCADE`)
			}
			if err != nil {
				_ = legacy.Rollback()
				result := <-done
				_ = repair.Rollback()
				if result.release != nil {
					result.release()
				}
				require.NoError(t, err)
				return
			}
			require.NoError(t, legacy.Rollback())
			result := <-done
			require.NoError(t, result.err)
			require.NoError(t, repair.Rollback())
			if result.release != nil {
				result.release()
			}
			var count int
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs WHERE account_id=$1`, f.accountID).Scan(&count))
			require.Equal(t, 2, count)
		})
	}
}

// This test advances coverage only by deleting a real synthetic source through
// the migration-installed trigger. It never sets/backdates complete_from. All
// operations use the integration harness's isolated testcontainers database.
func TestFinanceRecalculationConfigCoverageExposedAndDeleteChangesFingerprint(t *testing.T) {
	ctx := context.Background()
	f := financeConfigData(t)
	r := financeConfigRepository()
	b := financeConfigUsageBatch(f)
	before, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Len(t, before.Usage, 1)
	require.NotNil(t, before.Usage[0].CoverageFrom)
	initial := *before.Usage[0].CoverageFrom
	eligible, err := FinanceAggregationEligibleFrom(ctx, integrationDB)
	require.NoError(t, err)
	require.True(t, initial.Equal(eligible))
	encoded, err := json.Marshal(before)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"coverage_from":`)
	b.Config = before
	payment := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationPaymentFee}, Config: service.FinanceRecalculationConfig{Payment: []service.FinanceRecalculationPaymentRule{{ProviderID: f.providerID}}}}
	paymentBefore, err := r.Current(ctx, payment)
	require.NoError(t, err)
	// Choose an instant in a later eligible day so safe-day rounding must change.
	at := initial.Add(26 * time.Hour)
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,model,total_cost,actual_cost,billing_type,created_at) VALUES($1,$2,$3,'coverage-test',1,0.1,0,$4) RETURNING id`, f.userID, f.keyID, f.accountID, at).Scan(&id))
	// Mere INSERT does not invalidate prospective completeness or the preview.
	inserted, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, before), financeConfigFingerprint(t, inserted))
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE id=$1 AND created_at=$2`, id, at)
	require.NoError(t, err)
	after, err := r.Current(ctx, b)
	require.NoError(t, err)
	require.NotNil(t, after.Usage[0].CoverageFrom)
	require.True(t, after.Usage[0].CoverageFrom.After(initial))
	require.True(t, after.Usage[0].CoverageFrom.After(at))
	require.NotEqual(t, financeConfigFingerprint(t, before), financeConfigFingerprint(t, after))
	eligible, err = FinanceAggregationEligibleFrom(ctx, integrationDB)
	require.NoError(t, err)
	require.True(t, eligible.Equal(*after.Usage[0].CoverageFrom))
	// Coverage affects usage repair, not payment fee rules.
	paymentAfter, err := r.Current(ctx, payment)
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, paymentBefore), financeConfigFingerprint(t, paymentAfter))
	// Acquire must compute the same fresh boundary while the source lock is held,
	// instead of trusting the obsolete CoverageFrom inside b.Config.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	var release func()
	defer func() {
		_ = tx.Rollback()
		if release != nil {
			release()
		}
	}()
	fingerprint, unlock, err := r.Acquire(ctx, tx, b)
	release = unlock
	require.NoError(t, err)
	require.Equal(t, financeConfigFingerprint(t, after), fingerprint)
	require.NotEqual(t, financeConfigFingerprint(t, before), fingerprint)
	require.NoError(t, tx.Rollback())
	release()
	release = nil
}
