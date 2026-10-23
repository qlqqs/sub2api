//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type agentRewardRollupRow struct {
	UserID       int64
	SourceUserID int64
	BucketDate   string
	Amount       string
	Entries      int64
}

func TestAgentRewardRollupTriggerTracksInsertDeleteUpdate(t *testing.T) {
	ctx := context.Background()
	schema := createAgentRewardRollupTestSchema(t, ctx, true)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2), (3), (4);
		INSERT INTO user_affiliate_ledger (id, user_id, action, amount, source_user_id, reward_type, frozen_until, created_at)
		VALUES
			(1, 1, 'accrue', 0.6, 2, 'agent_direct', NOW() - INTERVAL '1 hour', TIMESTAMPTZ '2026-10-02 23:30:00+08'),
			(2, 1, 'accrue', 0.4, 2, 'agent_direct', NULL, TIMESTAMPTZ '2026-10-02 08:00:00+08'),
			(3, 1, 'accrue', 0.1, 2, 'agent_direct', NULL, TIMESTAMPTZ '2026-10-03 00:10:00+08'),
			(4, 1, 'accrue', 0.3, 3, 'agent_diff', NOW() - INTERVAL '1 hour', TIMESTAMPTZ '2026-10-02 12:00:00+08'),
			(5, 1, 'accrue', 9.9, 2, 'invite_percent', NULL, TIMESTAMPTZ '2026-10-02 12:00:00+08'),
			(6, 4, 'accrue', 0.2, 3, 'agent_direct', NULL, TIMESTAMPTZ '2026-10-02 12:00:00+08'),
			(7, 1, 'withdraw', -1.0, NULL, NULL, NULL, TIMESTAMPTZ '2026-10-02 12:00:00+08');
	`)
	require.NoError(t, err)

	rows := readAgentRewardRollupRows(t, ctx, tx)
	require.Equal(t, []agentRewardRollupRow{
		{UserID: 1, SourceUserID: 2, BucketDate: "2026-10-02", Amount: "1.00000000", Entries: 2},
		// 北京时间 00:10 归入次日。
		{UserID: 1, SourceUserID: 2, BucketDate: "2026-10-03", Amount: "0.10000000", Entries: 1},
		{UserID: 1, SourceUserID: 3, BucketDate: "2026-10-02", Amount: "0.30000000", Entries: 1},
		{UserID: 4, SourceUserID: 3, BucketDate: "2026-10-02", Amount: "0.20000000", Entries: 1},
	}, rows)

	// 解冻与提现回写快照只改其他列：汇总行一行都不写（ctid 不变）。
	before := readAgentRewardRollupCtids(t, ctx, tx)
	_, err = tx.ExecContext(ctx, `
		WITH matured AS (
			UPDATE user_affiliate_ledger SET frozen_until = NULL, updated_at = NOW()
			WHERE user_id = 1 AND frozen_until IS NOT NULL AND frozen_until <= NOW()
			RETURNING amount
		)
		SELECT COALESCE(SUM(amount), 0) FROM matured;
		UPDATE user_affiliate_ledger SET balance_after = 12.5, updated_at = NOW() WHERE id = 7;
	`)
	require.NoError(t, err)
	require.Equal(t, before, readAgentRewardRollupCtids(t, ctx, tx))
	require.Equal(t, rows, readAgentRewardRollupRows(t, ctx, tx))

	_, err = tx.ExecContext(ctx, `
		UPDATE user_affiliate_ledger SET amount = 0.5 WHERE id = 4;
		UPDATE user_affiliate_ledger SET reward_type = 'agent_diff' WHERE id = 5;
		DELETE FROM user_affiliate_ledger WHERE id IN (3, 6);
	`)
	require.NoError(t, err)

	require.Equal(t, []agentRewardRollupRow{
		{UserID: 1, SourceUserID: 2, BucketDate: "2026-10-02", Amount: "10.90000000", Entries: 3},
		{UserID: 1, SourceUserID: 3, BucketDate: "2026-10-02", Amount: "0.50000000", Entries: 1},
	}, readAgentRewardRollupRows(t, ctx, tx), "扣减到 0 的行被删除")
	requireAgentRewardRollupMatchesRaw(t, ctx, tx)
}

func TestAgentRewardRollupTriggerFollowsForeignKeys(t *testing.T) {
	ctx := context.Background()
	schema := createAgentRewardRollupTestSchema(t, ctx, true)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2), (3), (4);
		INSERT INTO user_affiliate_ledger (id, user_id, amount, source_user_id, reward_type, created_at)
		VALUES
			(1, 1, 0.6, 2, 'agent_direct', TIMESTAMPTZ '2026-10-02 12:00:00+08'),
			(2, 1, 0.3, 3, 'agent_diff', TIMESTAMPTZ '2026-10-02 13:00:00+08'),
			(3, 4, 0.2, 2, 'agent_direct', TIMESTAMPTZ '2026-10-02 14:00:00+08');
	`)
	require.NoError(t, err)

	// 来源被物理删除：外键 SET NULL，金额挪到来源 0。
	_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = 2`)
	require.NoError(t, err)
	require.Equal(t, []agentRewardRollupRow{
		{UserID: 1, SourceUserID: 0, BucketDate: "2026-10-02", Amount: "0.60000000", Entries: 1},
		{UserID: 1, SourceUserID: 3, BucketDate: "2026-10-02", Amount: "0.30000000", Entries: 1},
		{UserID: 4, SourceUserID: 0, BucketDate: "2026-10-02", Amount: "0.20000000", Entries: 1},
	}, readAgentRewardRollupRows(t, ctx, tx))
	requireAgentRewardRollupMatchesRaw(t, ctx, tx)

	// 受益人被物理删除：外键级联删除流水，汇总随之扣减。
	_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = 1`)
	require.NoError(t, err)
	require.Equal(t, []agentRewardRollupRow{
		{UserID: 4, SourceUserID: 0, BucketDate: "2026-10-02", Amount: "0.20000000", Entries: 1},
	}, readAgentRewardRollupRows(t, ctx, tx))
	requireAgentRewardRollupMatchesRaw(t, ctx, tx)
}

func TestAgentRewardRollupMigrationBackfillsExistingLedger(t *testing.T) {
	ctx := context.Background()
	schema := createAgentRewardRollupTestSchema(t, ctx, false)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2), (3), (4);
		INSERT INTO user_affiliate_ledger (id, user_id, amount, source_user_id, reward_type, created_at)
		SELECT g, 1 + g % 2, g * 0.01,
		       CASE WHEN g % 7 = 0 THEN NULL ELSE 3 + g % 2 END,
		       (ARRAY['agent_direct', 'agent_diff', 'invite_percent'])[1 + g % 3],
		       TIMESTAMPTZ '2026-10-01 00:00:00+08' + make_interval(hours => g)
		FROM generate_series(1, 200) AS g;
	`)
	require.NoError(t, err)
	applyAgentRewardRollupMigration(t, ctx, tx)
	applyAgentRewardRollupMigration(t, ctx, tx)

	requireAgentRewardRollupMatchesRaw(t, ctx, tx)
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM affiliate_agent_reward_daily_rollups WHERE source_user_id = 0`).Scan(&count))
	require.Greater(t, count, 0, "来源为空的流水回填到来源 0")
}

func TestAgentRewardRollupTriggerErrorDoesNotBlockLedgerInsert(t *testing.T) {
	ctx := context.Background()
	schema := createAgentRewardRollupTestSchema(t, ctx, true)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2);
		ALTER TABLE affiliate_agent_reward_daily_rollups RENAME TO affiliate_agent_reward_daily_rollups_moved;
		INSERT INTO user_affiliate_ledger (id, user_id, amount, source_user_id, reward_type, created_at)
		VALUES (1, 1, 0.6, 2, 'agent_direct', TIMESTAMPTZ '2026-10-02 12:00:00+08');
		UPDATE user_affiliate_ledger SET amount = 0.7 WHERE id = 1;
		DELETE FROM user_affiliate_ledger WHERE id = 1;
	`)
	require.NoError(t, err)
}

// 修复函数先锁住流水表：未提交的并发批次（含汇总里还没有的新键）提交后才删除并重建，结果不会重复累加。
func TestAgentRewardRollupRebuildWaitsForConcurrentInsert(t *testing.T) {
	ctx := context.Background()
	schema := createAgentRewardRollupTestSchema(t, ctx, true)

	setup := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	_, err := setup.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2), (3);
		INSERT INTO user_affiliate_ledger (id, user_id, amount, source_user_id, reward_type, created_at)
		VALUES (1, 1, 0.6, 2, 'agent_direct', TIMESTAMPTZ '2026-10-02 08:00:00+08');
		-- 模拟触发器曾吞掉错误导致的偏差。
		UPDATE affiliate_agent_reward_daily_rollups SET amount = 99, entries = 7;
	`)
	require.NoError(t, err)
	require.NoError(t, setup.Commit())

	writer := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(ctx, `
		INSERT INTO user_affiliate_ledger (id, user_id, amount, source_user_id, reward_type, created_at)
		VALUES (2, 1, 0.3, 3, 'agent_diff', TIMESTAMPTZ '2026-10-02 09:00:00+08');
	`)
	require.NoError(t, err)

	repair := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = repair.Rollback() }()
	var repairPID int
	require.NoError(t, repair.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&repairPID))

	repairResult := make(chan error, 1)
	go func() {
		_, execErr := repair.ExecContext(ctx, `SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '2026-10-02', DATE '2026-10-02')`)
		repairResult <- execErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		_ = integrationDB.QueryRowContext(ctx, `
			SELECT COALESCE(wait_event_type = 'Lock', false) FROM pg_stat_activity WHERE pid = $1
		`, repairPID).Scan(&waiting)
		return waiting
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, writer.Commit())
	require.NoError(t, <-repairResult)
	require.NoError(t, repair.Commit())

	check := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = check.Rollback() }()
	require.Equal(t, []agentRewardRollupRow{
		{UserID: 1, SourceUserID: 2, BucketDate: "2026-10-02", Amount: "0.60000000", Entries: 1},
		{UserID: 1, SourceUserID: 3, BucketDate: "2026-10-02", Amount: "0.30000000", Entries: 1},
	}, readAgentRewardRollupRows(t, ctx, check))
	requireAgentRewardRollupMatchesRaw(t, ctx, check)
}

// 经仓储真实的入账方法写两种代理分润（一条冻结），再解冻：汇总只随入账变化。
func TestAgentRewardRollupTracksAccrueQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)

	direct := mustCreateAffiliateWithdrawUser(t, client, "rollup-direct")
	upper := mustCreateAffiliateWithdrawUser(t, client, "rollup-upper")
	invitee := mustCreateAffiliateWithdrawUser(t, client, "rollup-invitee")
	mustSeedAffiliateQuota(t, txCtx, client, direct.ID, 0, 0, 0)
	mustSeedAffiliateQuota(t, txCtx, client, upper.ID, 0, 0, 0)

	for _, accrual := range []struct {
		beneficiary int64
		amount      float64
		freezeHours int
		rewardType  string
	}{
		{direct.ID, 0.8, 0, service.AffiliateRewardAgentDirect},
		{direct.ID, 0.2, 0, service.AffiliateRewardAgentDirect},
		{upper.ID, 0.3, 24, service.AffiliateRewardAgentDiff},
		{direct.ID, 5.0, 0, service.AffiliateRewardInvitePercent},
	} {
		applied, err := repo.AccrueQuota(txCtx, accrual.beneficiary, invitee.ID, accrual.amount, accrual.freezeHours, service.AffiliateAccrueSource{}, accrual.rewardType)
		require.NoError(t, err)
		require.True(t, applied)
	}

	type rollup struct {
		amount  float64
		entries int64
	}
	read := func(userID int64) rollup {
		t.Helper()
		var r rollup
		rows, err := client.QueryContext(txCtx, `
SELECT COALESCE(SUM(amount), 0)::double precision, COALESCE(SUM(entries), 0)
FROM affiliate_agent_reward_daily_rollups
WHERE user_id = $1 AND source_user_id = $2 AND bucket_date = (NOW() AT TIME ZONE 'Asia/Shanghai')::date`, userID, invitee.ID)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		require.True(t, rows.Next())
		require.NoError(t, rows.Scan(&r.amount, &r.entries))
		return r
	}
	require.Equal(t, rollup{amount: 1.0, entries: 2}, read(direct.ID), "邀请返利不计入")
	require.Equal(t, rollup{amount: 0.3, entries: 1}, read(upper.ID))

	_, err := client.ExecContext(txCtx, `UPDATE user_affiliate_ledger SET frozen_until = NOW() - INTERVAL '1 minute' WHERE user_id = $1`, upper.ID)
	require.NoError(t, err)
	thawed, err := repo.ThawFrozenQuota(txCtx, upper.ID)
	require.NoError(t, err)
	require.InDelta(t, 0.3, thawed, 1e-9)
	require.Equal(t, rollup{amount: 0.3, entries: 1}, read(upper.ID))
}

func createAgentRewardRollupTestSchema(t *testing.T, ctx context.Context, applyRollup bool) string {
	t.Helper()

	schema := fmt.Sprintf("agent_reward_rollup_trigger_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schema)
	_, err := integrationDB.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+quotedSchema+" CASCADE")
	})

	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, setGroupUsageRollupTriggerSearchPath(ctx, tx, quotedSchema))

	// 与生产表相同的外键动作：受益人级联删除，来源置空。
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE user_affiliate_ledger (
			id BIGINT PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			action VARCHAR(32) NOT NULL DEFAULT 'accrue',
			amount NUMERIC(20, 8) NOT NULL,
			source_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
			reward_type VARCHAR(24),
			frozen_until TIMESTAMPTZ,
			balance_after NUMERIC(20, 8),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)
	if applyRollup {
		applyAgentRewardRollupMigration(t, ctx, tx)
		applyAgentRewardRollupMigration(t, ctx, tx)
	}
	require.NoError(t, tx.Commit())
	return schema
}

func applyAgentRewardRollupMigration(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()
	migrationSQL, err := migrations.FS.ReadFile("252_affiliate_agent_reward_daily_rollups.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}

func readAgentRewardRollupRows(t *testing.T, ctx context.Context, tx *sql.Tx) []agentRewardRollupRow {
	t.Helper()
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id, source_user_id, bucket_date::text, amount::text, entries
		FROM affiliate_agent_reward_daily_rollups
		ORDER BY user_id, source_user_id, bucket_date
	`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []agentRewardRollupRow
	for rows.Next() {
		var r agentRewardRollupRow
		require.NoError(t, rows.Scan(&r.UserID, &r.SourceUserID, &r.BucketDate, &r.Amount, &r.Entries))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func readAgentRewardRollupCtids(t *testing.T, ctx context.Context, tx *sql.Tx) []string {
	t.Helper()
	rows, err := tx.QueryContext(ctx, `
		SELECT ctid::text FROM affiliate_agent_reward_daily_rollups ORDER BY user_id, source_user_id, bucket_date
	`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var ctid string
		require.NoError(t, rows.Scan(&ctid))
		out = append(out, ctid)
	}
	require.NoError(t, rows.Err())
	return out
}

// requireAgentRewardRollupMatchesRaw 对比汇总与流水直接聚合。
func requireAgentRewardRollupMatchesRaw(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()
	var diff int
	require.NoError(t, tx.QueryRowContext(ctx, `
		WITH raw AS (
			SELECT user_id,
			       COALESCE(source_user_id, 0) AS source_user_id,
			       (created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
			       SUM(amount)::numeric(20, 8) AS amount,
			       COUNT(*)::bigint AS entries
			FROM user_affiliate_ledger
			WHERE reward_type IN ('agent_direct', 'agent_diff')
			GROUP BY 1, 2, 3
		), rollup AS (
			SELECT user_id, source_user_id, bucket_date, amount::numeric(20, 8), entries
			FROM affiliate_agent_reward_daily_rollups
		)
		SELECT (SELECT COUNT(*) FROM (SELECT * FROM raw EXCEPT SELECT * FROM rollup) a)
		     + (SELECT COUNT(*) FROM (SELECT * FROM rollup EXCEPT SELECT * FROM raw) b)
	`).Scan(&diff))
	require.Zero(t, diff, "affiliate_agent_reward_daily_rollups must match user_affiliate_ledger aggregation")
}
