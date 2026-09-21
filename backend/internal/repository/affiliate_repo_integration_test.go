//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func querySingleFloat(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value float64
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func querySingleInt(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) int {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value int
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func TestAffiliateRepository_TransferQuotaToBalance_UsesClaimedQuotaBeforeClear(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 12.34)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.34, transferred, 1e-9)
	require.InDelta(t, 17.84, balance, 1e-9)

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 17.84, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'transfer'", u.ID)
	require.Equal(t, 1, ledgerCount)

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       balance_after::double precision,
       aff_quota_after::double precision,
       aff_frozen_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action = 'transfer'
LIMIT 1`, u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected transfer ledger")
	var amount, balanceAfter, quotaAfter, frozenAfter, historyAfter float64
	require.NoError(t, rows.Scan(&amount, &balanceAfter, &quotaAfter, &frozenAfter, &historyAfter))
	require.InDelta(t, 12.34, amount, 1e-9)
	require.InDelta(t, 17.84, balanceAfter, 1e-9)
	require.InDelta(t, 0.0, quotaAfter, 1e-9)
	require.InDelta(t, 0.0, frozenAfter, 1e-9)
	require.InDelta(t, 12.34, historyAfter, 1e-9)
}

// TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction guards the
// cross-layer tx propagation invariant: when AccrueQuota is called with a ctx
// that already carries a transaction (via dbent.NewTxContext), repo.withTx
// must reuse that tx rather than opening a nested one. If this invariant
// breaks, AccrueQuota would commit independently and survive a rollback of
// the outer tx, which would violate payment_fulfillment's all-or-nothing
// semantics.
func TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	// Defensive cleanup: if any require.* below fires before the explicit
	// Rollback, this prevents the tx from leaking until container teardown.
	// Rollback is idempotent at the driver level (extra rollback returns an
	// error we ignore).
	t.Cleanup(func() { _ = outerTx.Rollback() })
	client := outerTx.Client()
	txCtx := dbent.NewTxContext(ctx, outerTx)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	repo := NewAffiliateRepository(client, integrationDB)
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	_, bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID, service.AffiliateInviterBinding{Source: "registration"})
	require.NoError(t, err)
	require.True(t, bound, "invitee must bind to inviter")

	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 3.5, 0, nil)
	require.NoError(t, err)
	require.True(t, applied, "AccrueQuota must report applied=true")

	// Visible inside the outer tx.
	innerQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 3.5, innerQuota, 1e-9)

	// Roll back the outer tx; if AccrueQuota had opened its own inner tx and
	// committed it, the rows would still be visible to the global client.
	require.NoError(t, outerTx.Rollback())

	rows, err := integrationEntClient.QueryContext(ctx,
		"SELECT COUNT(*) FROM user_affiliates WHERE user_id IN ($1, $2)",
		inviter.ID, invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var postRollbackCount int
	require.NoError(t, rows.Scan(&postRollbackCount))
	require.Equal(t, 0, postRollbackCount,
		"AccrueQuota must propagate the outer tx — found persisted rows after rollback")
}

func TestAffiliateRepository_TransferQuotaToBalance_EmptyQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-empty-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      3.21,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, 0, 0, NOW(), NOW())`, u.ID, affCode)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaEmpty)
	require.InDelta(t, 0.0, transferred, 1e-9)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 3.21, persistedBalance, 1e-9)
}

// TestAffiliateRepository_AdminCustomCode covers the success path of admin
// invite-code rewrite + reset within a shared test transaction:
// - UpdateUserAffCode replaces aff_code, sets aff_code_custom=true, lookup works
// - the old code can no longer be found
// - ResetUserAffCode reverts aff_code_custom and assigns a new system-format code
//
// The conflict path (duplicate code → ErrAffiliateCodeTaken) lives in its own
// test because a unique-violation aborts the surrounding Postgres tx, which
// would poison subsequent assertions in the same transaction.
func TestAffiliateRepository_AdminCustomCode(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-custom-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	original, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.False(t, original.AffCodeCustom, "system-generated codes start as non-custom")
	originalCode := original.AffCode

	// Rewrite to a custom code
	customCode := fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, u.ID, customCode))

	updated, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, customCode, updated.AffCode)
	require.True(t, updated.AffCodeCustom)

	// Lookup by new custom code finds the user
	byCode, err := repo.GetAffiliateByCode(txCtx, customCode)
	require.NoError(t, err)
	require.Equal(t, u.ID, byCode.UserID)

	// Old system code should no longer match
	_, err = repo.GetAffiliateByCode(txCtx, originalCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)

	// Reset back to a fresh system code, clears custom flag
	newSysCode, err := repo.ResetUserAffCode(txCtx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, customCode, newSysCode)

	reset, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, newSysCode, reset.AffCode)
	require.False(t, reset.AffCodeCustom)

	// The old custom code is now free again
	_, err = repo.GetAffiliateByCode(txCtx, customCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)
}

// TestAffiliateRepository_AdminCustomCode_Conflict isolates the unique-violation
// path. PostgreSQL aborts the enclosing tx when a unique constraint fires, so
// this test must be the only assertion and run in its own tx — production
// callers each have their own outer tx, so this matches real behavior.
func TestAffiliateRepository_AdminCustomCode_Conflict(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	taker := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-taker-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	requester := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-req-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})

	takenCode := fmt.Sprintf("HOT%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, taker.ID, takenCode))

	// Now requester tries to grab the same code → conflict.
	err := repo.UpdateUserAffCode(txCtx, requester.ID, takenCode)
	require.ErrorIs(t, err, service.ErrAffiliateCodeTaken)
}

// TestAffiliateRepository_AdminRebateRate covers per-user exclusive rate
// set/clear and the Batch variant including NULL semantics.
func TestAffiliateRepository_AdminRebateRate(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u1 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-a@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	u2 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-b@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	// Set exclusive rate for u1
	rate := 42.5
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, &rate))

	got, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRatePercent)
	require.InDelta(t, 42.5, *got.AffRebateRatePercent, 1e-9)

	// Clear exclusive rate
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, nil))
	cleared, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.AffRebateRatePercent)

	// Batch set both users
	batchRate := 15.0
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, &batchRate))

	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.NotNil(t, v.AffRebateRatePercent)
		require.InDelta(t, 15.0, *v.AffRebateRatePercent, 1e-9)
	}

	// Batch clear
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, nil))
	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.Nil(t, v.AffRebateRatePercent)
	}
}

// TestAffiliateRepository_ListUsersWithCustomSettings verifies the admin list
// only includes users with at least one override applied.
func TestAffiliateRepository_ListUsersWithCustomSettings(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	// User without any custom config — should NOT appear in the list.
	plainEmail := fmt.Sprintf("affiliate-plain-%d@example.com", time.Now().UnixNano())
	uPlain := mustCreateUser(t, client, &service.User{
		Email: plainEmail, PasswordHash: "hash",
		Role: service.RoleUser, Status: service.StatusActive,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, uPlain.ID)
	require.NoError(t, err)

	// User with a custom code — should appear.
	uCode := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-codeonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	require.NoError(t, repo.UpdateUserAffCode(txCtx, uCode.ID, fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)))

	// User with only an exclusive rate — should appear.
	uRate := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rateonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	r := 33.3
	require.NoError(t, repo.SetUserRebateRate(txCtx, uRate.ID, &r))

	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{
		Page: 1, PageSize: 100,
	})
	require.NoError(t, err)

	// Build a quick lookup to assert per-user attributes (other tests may have
	// inserted custom rows in the same DB; we only care about our 3).
	byUserID := make(map[int64]service.AffiliateAdminEntry, len(entries))
	for _, e := range entries {
		byUserID[e.UserID] = e
	}

	require.NotContains(t, byUserID, uPlain.ID, "users without overrides must not appear")

	codeEntry, ok := byUserID[uCode.ID]
	require.True(t, ok, "custom-code user missing from list")
	require.True(t, codeEntry.AffCodeCustom)
	require.Nil(t, codeEntry.AffRebateRatePercent)

	rateEntry, ok := byUserID[uRate.ID]
	require.True(t, ok, "custom-rate user missing from list")
	require.False(t, rateEntry.AffCodeCustom)
	require.NotNil(t, rateEntry.AffRebateRatePercent)
	require.InDelta(t, 33.3, *rateEntry.AffRebateRatePercent, 1e-9)

	require.GreaterOrEqual(t, total, int64(2), "total must include at least our 2 custom rows")
}

func TestAffiliateRepository_BindInviterPersistsMetadataAndOnlyIncrementsOnce(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	admin := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-admin-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleAdmin, Status: service.StatusActive, Concurrency: 5})
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-bind-inviter-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-bind-invitee-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5})
	reason := "registration omission"

	result, bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID, service.AffiliateInviterBinding{Source: "admin", BoundBy: &admin.ID, Reason: &reason})
	require.NoError(t, err)
	require.True(t, bound)
	require.Equal(t, invitee.ID, result.InviteeID)

	_, bound, err = repo.BindInviter(txCtx, invitee.ID, inviter.ID, service.AffiliateInviterBinding{Source: "admin", BoundBy: &admin.ID, Reason: &reason})
	require.NoError(t, err)
	require.False(t, bound)
	require.Equal(t, 1, querySingleInt(t, txCtx, client, "SELECT aff_count FROM user_affiliates WHERE user_id = $1", inviter.ID))

	rows, err := client.QueryContext(txCtx, "SELECT inviter_id, inviter_bound_source, inviter_bound_by, inviter_bind_reason, inviter_bound_at FROM user_affiliates WHERE user_id = $1", invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var gotInviter, gotAdmin int64
	var gotSource, gotReason string
	var gotBoundAt time.Time
	require.NoError(t, rows.Scan(&gotInviter, &gotSource, &gotAdmin, &gotReason, &gotBoundAt))
	require.Equal(t, inviter.ID, gotInviter)
	require.Equal(t, "admin", gotSource)
	require.Equal(t, admin.ID, gotAdmin)
	require.Equal(t, reason, gotReason)
	require.False(t, gotBoundAt.IsZero())
}

func TestAffiliateRepository_BindInviterConcurrentRequestsOnlyIncrementOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	suffix := time.Now().UnixNano()
	admin := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-race-admin-%d@example.com", suffix), Role: service.RoleAdmin})
	inviterA := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-race-a-%d@example.com", suffix)})
	inviterB := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-race-b-%d@example.com", suffix)})
	invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-race-invitee-%d@example.com", suffix)})
	userIDs := []int64{admin.ID, inviterA.ID, inviterB.ID, invitee.ID}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_affiliates WHERE user_id = ANY($1)", pq.Array(userIDs))
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = ANY($1)", pq.Array(userIDs))
	})

	for _, userID := range userIDs {
		_, err := repo.EnsureUserAffiliate(ctx, userID)
		require.NoError(t, err)
	}

	type bindResult struct {
		bound bool
		err   error
	}
	start := make(chan struct{})
	results := make(chan bindResult, 2)
	var wg sync.WaitGroup
	for _, inviterID := range []int64{inviterA.ID, inviterB.ID} {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			<-start
			reason := "concurrent support correction"
			_, bound, err := repo.BindInviter(ctx, invitee.ID, id, service.AffiliateInviterBinding{Source: "admin", BoundBy: &admin.ID, Reason: &reason})
			results <- bindResult{bound: bound, err: err}
		}(inviterID)
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for result := range results {
		require.NoError(t, result.err)
		if result.bound {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	countA := querySingleInt(t, ctx, client, "SELECT aff_count FROM user_affiliates WHERE user_id = $1", inviterA.ID)
	countB := querySingleInt(t, ctx, client, "SELECT aff_count FROM user_affiliates WHERE user_id = $1", inviterB.ID)
	require.Equal(t, 1, countA+countB)
}

func TestAffiliateRepository_BindInviterRollsBackRelationshipWhenCountIncrementFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	suffix := time.Now().UnixNano()
	admin := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-rollback-admin-%d@example.com", suffix), Role: service.RoleAdmin})
	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-rollback-inviter-%d@example.com", suffix)})
	invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-rollback-invitee-%d@example.com", suffix)})
	userIDs := []int64{admin.ID, inviter.ID, invitee.ID}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_affiliates WHERE user_id = ANY($1)", pq.Array(userIDs))
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = ANY($1)", pq.Array(userIDs))
	})

	for _, userID := range userIDs {
		_, err := repo.EnsureUserAffiliate(ctx, userID)
		require.NoError(t, err)
	}
	_, err := integrationDB.ExecContext(ctx, "UPDATE user_affiliates SET aff_count = 2147483647 WHERE user_id = $1", inviter.ID)
	require.NoError(t, err)

	reason := "force rollback coverage"
	_, bound, err := repo.BindInviter(ctx, invitee.ID, inviter.ID, service.AffiliateInviterBinding{Source: "admin", BoundBy: &admin.ID, Reason: &reason})
	require.Error(t, err)
	require.False(t, bound)

	var inviterID, boundAt any
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT inviter_id, inviter_bound_at FROM user_affiliates WHERE user_id = $1", invitee.ID,
	).Scan(&inviterID, &boundAt))
	require.Nil(t, inviterID)
	require.Nil(t, boundAt)
	require.Equal(t, 2147483647, querySingleInt(t, ctx, client, "SELECT aff_count FROM user_affiliates WHERE user_id = $1", inviter.ID))
}

func TestAffiliateInviterBindingTriggerSupportsLegacyWritesAndPermanentMarker(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRepository(client, integrationDB)
	suffix := time.Now().UnixNano()
	legacyInviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-trigger-legacy-%d@example.com", suffix)})
	replacementInviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-trigger-replacement-%d@example.com", suffix)})
	invitee := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-trigger-invitee-%d@example.com", suffix)})
	userIDs := []int64{legacyInviter.ID, replacementInviter.ID, invitee.ID}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_affiliates WHERE user_id = ANY($1)", pq.Array(userIDs))
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = ANY($1)", pq.Array(userIDs))
	})

	for _, userID := range userIDs {
		_, err := repo.EnsureUserAffiliate(ctx, userID)
		require.NoError(t, err)
	}

	// Simulate an old binary that only knows about inviter_id.
	_, err := integrationDB.ExecContext(ctx,
		"UPDATE user_affiliates SET inviter_id = $1, updated_at = NOW() WHERE user_id = $2",
		legacyInviter.ID, invitee.ID)
	require.NoError(t, err)

	var boundAt time.Time
	var source string
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT inviter_bound_at, inviter_bound_source FROM user_affiliates WHERE user_id = $1", invitee.ID,
	).Scan(&boundAt, &source))
	require.False(t, boundAt.IsZero())
	require.Equal(t, "registration", source)

	// A direct attempt to clear the permanent marker is normalized back to the
	// original value rather than reopening the row for another binding.
	_, err = integrationDB.ExecContext(ctx,
		"UPDATE user_affiliates SET inviter_bound_at = NULL WHERE user_id = $1", invitee.ID)
	require.NoError(t, err)
	var preservedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT inviter_bound_at FROM user_affiliates WHERE user_id = $1", invitee.ID,
	).Scan(&preservedAt))
	require.True(t, preservedAt.Equal(boundAt))

	// Exercise the real FK action: deleting the inviter may clear inviter_id,
	// but the trigger must preserve the one-time marker.
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", legacyInviter.ID)
	require.NoError(t, err)
	var inviterID any
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT inviter_id, inviter_bound_at FROM user_affiliates WHERE user_id = $1", invitee.ID,
	).Scan(&inviterID, &preservedAt))
	require.Nil(t, inviterID)
	require.True(t, preservedAt.Equal(boundAt))

	// Once the marker exists, assigning a replacement inviter is forbidden.
	_, err = integrationDB.ExecContext(ctx,
		"UPDATE user_affiliates SET inviter_id = $1 WHERE user_id = $2",
		replacementInviter.ID, invitee.ID)
	require.Error(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT inviter_id, inviter_bound_at FROM user_affiliates WHERE user_id = $1", invitee.ID,
	).Scan(&inviterID, &preservedAt))
	require.Nil(t, inviterID)
	require.True(t, preservedAt.Equal(boundAt))
}

func TestAffiliateInviterBindingMigrationCanBeReapplied(t *testing.T) {
	content, err := migrations.FS.ReadFile("239_affiliate_inviter_binding_metadata.sql")
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		_, err = integrationDB.ExecContext(context.Background(), string(content))
		require.NoErrorf(t, err, "migration 239 reapply %d", i+1)
	}
}
