//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type usageUserRollupRow struct {
	UserID       int64
	BucketDate   string
	Model        string
	GroupID      int64
	Requests     int64
	InputTokens  int64
	OutputTokens int64
	CacheCreate  int64
	CacheRead    int64
	ImageCount   int64
	VideoCount   int64
	ActualCost   string
	LastUsedAt   time.Time
}

func TestUsageUserRollupTriggerTracksInsertDeleteUpdate(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		name := "ordinary"
		if partitioned {
			name = "partitioned"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			schema := createUsageUserRollupTestSchema(t, ctx, partitioned, true)
			tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
			defer func() { _ = tx.Rollback() }()

			_, err := tx.ExecContext(ctx, `
				INSERT INTO groups (id) VALUES (10), (20);
				INSERT INTO users (id) VALUES (1), (2);
				INSERT INTO usage_logs (id, user_id, group_id, model, requested_model, billing_type,
					input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
					image_count, video_count, actual_cost, created_at)
				VALUES
					(1, 1, 10, 'claude-upstream', 'claude', 0, 100, 10, 1, 1000, NULL, 0, 1.5, TIMESTAMPTZ '2026-10-02 23:30:00+08'),
					(2, 1, 10, 'claude-upstream', 'claude', 0, 200, 20, 2, 2000, 0, 0, 2.5, TIMESTAMPTZ '2026-10-02 08:00:00+08'),
					(3, 1, 10, 'claude-upstream', 'claude', 0, 1, 1, 0, 0, 0, 0, 0.1, TIMESTAMPTZ '2026-10-03 00:10:00+08'),
					(4, 1, NULL, 'gpt-image', '  ', 0, 0, 0, 0, 0, 2, 0, 0.3, TIMESTAMPTZ '2026-10-02 12:00:00+08'),
					(5, 1, 10, 'claude-upstream', 'claude', 1, 999, 999, 0, 0, 0, 0, 9.9, TIMESTAMPTZ '2026-10-02 12:00:00+08'),
					(6, 2, 20, 'gpt', 'gpt', 0, 5, 5, 0, 0, 0, 1, 0.2, TIMESTAMPTZ '2026-10-02 12:00:00+08');
			`)
			require.NoError(t, err)

			rows := readUsageUserRollupRows(t, ctx, tx)
			require.Len(t, rows, 4)
			require.Equal(t, usageUserRollupKey(1, "2026-10-02", "claude", 10), usageUserRollupRowKey(rows[0]))
			require.Equal(t, int64(2), rows[0].Requests)
			require.Equal(t, int64(300), rows[0].InputTokens)
			require.Equal(t, int64(30), rows[0].OutputTokens)
			require.Equal(t, int64(3), rows[0].CacheCreate)
			require.Equal(t, int64(3000), rows[0].CacheRead)
			require.Equal(t, "4.0000000000", rows[0].ActualCost)
			require.True(t, rows[0].LastUsedAt.Equal(time.Date(2026, 10, 2, 15, 30, 0, 0, time.UTC)))
			// 空白 requested_model 回落到 model，NULL 分组记为 0。
			require.Equal(t, usageUserRollupKey(1, "2026-10-02", "gpt-image", 0), usageUserRollupRowKey(rows[1]))
			require.Equal(t, int64(2), rows[1].ImageCount)
			// 北京时间 00:10 归入次日。
			require.Equal(t, usageUserRollupKey(1, "2026-10-03", "claude", 10), usageUserRollupRowKey(rows[2]))
			require.Equal(t, usageUserRollupKey(2, "2026-10-02", "gpt", 20), usageUserRollupRowKey(rows[3]))
			require.Equal(t, int64(1), rows[3].VideoCount)

			_, err = tx.ExecContext(ctx, `
				DELETE FROM usage_logs WHERE id IN (2, 4);
				UPDATE usage_logs SET actual_cost = 0.5, group_id = 10 WHERE id = 6;
			`)
			require.NoError(t, err)

			rows = readUsageUserRollupRows(t, ctx, tx)
			require.Len(t, rows, 3)
			require.Equal(t, int64(1), rows[0].Requests)
			require.Equal(t, "1.5000000000", rows[0].ActualCost)
			require.Equal(t, usageUserRollupKey(1, "2026-10-03", "claude", 10), usageUserRollupRowKey(rows[1]))
			require.Equal(t, usageUserRollupKey(2, "2026-10-02", "gpt", 10), usageUserRollupRowKey(rows[2]))
			require.Equal(t, "0.5000000000", rows[2].ActualCost)
			requireUsageUserRollupMatchesRaw(t, ctx, tx)
		})
	}
}

func TestUsageUserRollupTriggerFollowsForeignKeySetNull(t *testing.T) {
	ctx := context.Background()
	schema := createUsageUserRollupTestSchema(t, ctx, false, true)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, model, actual_cost, created_at)
		VALUES (1, 1, 10, 'claude', 1.0, TIMESTAMPTZ '2026-10-02 12:00:00+08');
		DELETE FROM groups WHERE id = 10;
	`)
	require.NoError(t, err)

	rows := readUsageUserRollupRows(t, ctx, tx)
	require.Len(t, rows, 1)
	require.Equal(t, usageUserRollupKey(1, "2026-10-02", "claude", 0), usageUserRollupRowKey(rows[0]))
	requireUsageUserRollupMatchesRaw(t, ctx, tx)
}

func TestUsageUserRollupMigrationBackfillsExistingUsage(t *testing.T) {
	ctx := context.Background()
	schema := createUsageUserRollupTestSchema(t, ctx, false, false)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1), (2);
		INSERT INTO usage_logs (id, user_id, model, requested_model, billing_type, input_tokens, actual_cost, created_at)
		SELECT g, 1 + g % 2, 'm' || (g % 3), NULL, (g % 5 = 0)::int, g, g * 0.01,
		       TIMESTAMPTZ '2026-10-01 00:00:00+08' + make_interval(hours => g)
		FROM generate_series(1, 200) AS g;
	`)
	require.NoError(t, err)
	applyUsageUserRollupMigration(t, ctx, tx)
	applyUsageUserRollupMigration(t, ctx, tx)

	requireUsageUserRollupMatchesRaw(t, ctx, tx)
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_user_daily_rollups`).Scan(&count))
	require.Greater(t, count, 0)
}

func TestUsageUserRollupTriggerErrorDoesNotBlockUsageInsert(t *testing.T) {
	ctx := context.Background()
	schema := createUsageUserRollupTestSchema(t, ctx, false, true)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()

	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1);
		ALTER TABLE usage_user_daily_rollups RENAME TO usage_user_daily_rollups_moved;
		INSERT INTO usage_logs (id, user_id, model, actual_cost, created_at)
		VALUES (1, 1, 'claude', 1.0, TIMESTAMPTZ '2026-10-02 12:00:00+08');
	`)
	require.NoError(t, err)

	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs`).Scan(&count))
	require.Equal(t, 1, count)
}

// 修复函数与并发写入同时执行：并发批次先锁住汇总行，修复等待其提交后再删除并按新快照重建。
func TestUsageUserRollupRebuildWaitsForConcurrentInsert(t *testing.T) {
	ctx := context.Background()
	schema := createUsageUserRollupTestSchema(t, ctx, false, true)

	setup := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	_, err := setup.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, model, actual_cost, created_at)
		VALUES (1, 1, 'claude', 1.0, TIMESTAMPTZ '2026-10-02 08:00:00+08');
		-- 模拟触发器曾吞掉错误导致的偏差。
		UPDATE usage_user_daily_rollups SET actual_cost = 99, requests = 7;
	`)
	require.NoError(t, err)
	require.NoError(t, setup.Commit())

	writer := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(ctx, `
		INSERT INTO usage_logs (id, user_id, model, actual_cost, created_at)
		VALUES (2, 1, 'claude', 2.0, TIMESTAMPTZ '2026-10-02 09:00:00+08');
	`)
	require.NoError(t, err)

	repair := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = repair.Rollback() }()
	var repairPID int
	require.NoError(t, repair.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&repairPID))

	repairResult := make(chan error, 1)
	go func() {
		_, execErr := repair.ExecContext(ctx, `SELECT rebuild_usage_user_daily_rollups(DATE '2026-10-02', DATE '2026-10-02')`)
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
	rows := readUsageUserRollupRows(t, ctx, check)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Requests)
	require.Equal(t, "3.0000000000", rows[0].ActualCost)
	requireUsageUserRollupMatchesRaw(t, ctx, check)
}

func createUsageUserRollupTestSchema(t *testing.T, ctx context.Context, partitioned, applyRollup bool) string {
	t.Helper()

	schema := fmt.Sprintf("usage_user_rollup_trigger_%d", time.Now().UnixNano())
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

	columns := `
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
		model VARCHAR(100) NOT NULL,
		requested_model VARCHAR(100),
		billing_type SMALLINT NOT NULL DEFAULT 0,
		input_tokens INT NOT NULL DEFAULT 0,
		output_tokens INT NOT NULL DEFAULT 0,
		cache_creation_tokens INT NOT NULL DEFAULT 0,
		cache_read_tokens INT NOT NULL DEFAULT 0,
		image_count INT DEFAULT 0,
		video_count INTEGER NOT NULL DEFAULT 0,
		actual_cost NUMERIC(20, 10) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL`
	usageLogsDDL := `CREATE TABLE usage_logs (id BIGINT PRIMARY KEY,` + columns + `);`
	if partitioned {
		usageLogsDDL = `
			CREATE TABLE usage_logs (id BIGINT NOT NULL,` + columns + `) PARTITION BY RANGE (created_at);
			CREATE TABLE usage_logs_default PARTITION OF usage_logs DEFAULT;`
	}
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE groups (id BIGINT PRIMARY KEY);
	`+usageLogsDDL)
	require.NoError(t, err)

	// 与分组日汇总触发器共存，验证两套触发器同时生效。
	for _, migrationName := range []string{
		"222_group_usage_daily_rollups.sql",
		"223_group_usage_rollup_timezone.sql",
	} {
		migrationSQL, readErr := migrations.FS.ReadFile(migrationName)
		require.NoError(t, readErr)
		_, err = tx.ExecContext(ctx, string(migrationSQL))
		require.NoError(t, err)
	}
	if applyRollup {
		applyUsageUserRollupMigration(t, ctx, tx)
		applyUsageUserRollupMigration(t, ctx, tx)
	}
	require.NoError(t, tx.Commit())
	return schema
}

func applyUsageUserRollupMigration(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()
	migrationSQL, err := migrations.FS.ReadFile("251_usage_user_daily_rollups.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}

func readUsageUserRollupRows(t *testing.T, ctx context.Context, tx *sql.Tx) []usageUserRollupRow {
	t.Helper()
	rows, err := tx.QueryContext(ctx, `
		SELECT user_id, bucket_date::text, model, group_id, requests,
		       input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		       image_count, video_count, actual_cost::text, last_used_at
		FROM usage_user_daily_rollups
		ORDER BY user_id, bucket_date, model, group_id
	`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var out []usageUserRollupRow
	for rows.Next() {
		var r usageUserRollupRow
		require.NoError(t, rows.Scan(&r.UserID, &r.BucketDate, &r.Model, &r.GroupID, &r.Requests,
			&r.InputTokens, &r.OutputTokens, &r.CacheCreate, &r.CacheRead,
			&r.ImageCount, &r.VideoCount, &r.ActualCost, &r.LastUsedAt))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// requireUsageUserRollupMatchesRaw 对比汇总与明细直接聚合（不含 last_used_at：删除明细时不回退）。
func requireUsageUserRollupMatchesRaw(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()
	var diff int
	require.NoError(t, tx.QueryRowContext(ctx, `
		WITH raw AS (
			SELECT user_id,
			       (created_at AT TIME ZONE 'Asia/Shanghai')::date AS bucket_date,
			       COALESCE(NULLIF(BTRIM(requested_model), ''), model) AS model,
			       COALESCE(group_id, 0) AS group_id,
			       COUNT(*)::bigint AS requests,
			       SUM(input_tokens)::bigint AS input_tokens,
			       SUM(output_tokens)::bigint AS output_tokens,
			       SUM(cache_creation_tokens)::bigint AS cache_creation_tokens,
			       SUM(cache_read_tokens)::bigint AS cache_read_tokens,
			       SUM(COALESCE(image_count, 0))::bigint AS image_count,
			       SUM(video_count)::bigint AS video_count,
			       SUM(actual_cost)::numeric(20, 10) AS actual_cost
			FROM usage_logs
			WHERE billing_type = 0
			GROUP BY 1, 2, 3, 4
		), rollup AS (
			SELECT user_id, bucket_date, model::text, group_id, requests,
			       input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			       image_count, video_count, actual_cost::numeric(20, 10)
			FROM usage_user_daily_rollups
		)
		SELECT (SELECT COUNT(*) FROM (SELECT * FROM raw EXCEPT SELECT * FROM rollup) a)
		     + (SELECT COUNT(*) FROM (SELECT * FROM rollup EXCEPT SELECT * FROM raw) b)
	`).Scan(&diff))
	require.Zero(t, diff, "usage_user_daily_rollups must match usage_logs aggregation")
}

func usageUserRollupKey(userID int64, date, model string, groupID int64) string {
	return fmt.Sprintf("%d|%s|%s|%d", userID, date, model, groupID)
}

func usageUserRollupRowKey(r usageUserRollupRow) string {
	return usageUserRollupKey(r.UserID, r.BucketDate, r.Model, r.GroupID)
}
