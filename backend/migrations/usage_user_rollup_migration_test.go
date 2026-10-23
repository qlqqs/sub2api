//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration251CreatesUsageUserDailyRollups(t *testing.T) {
	content, err := FS.ReadFile("251_usage_user_daily_rollups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS usage_user_daily_rollups")
	require.Contains(t, sql, "PRIMARY KEY (user_id, bucket_date, model, group_id)")
	require.Contains(t, sql, "actual_cost DECIMAL(20, 10)")
	require.Contains(t, sql, "(n.created_at AT TIME ZONE 'Asia/Shanghai')::date")
	require.Contains(t, sql, "COALESCE(NULLIF(BTRIM(n.requested_model), ''), n.model)")
	require.Contains(t, sql, "WHERE n.billing_type = 0")
	require.Contains(t, sql, "ORDER BY 1, 2, 3, 4")
	require.Contains(t, sql, "EXCEPTION WHEN OTHERS THEN")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION rebuild_usage_user_daily_rollups(from_date DATE, to_date DATE)")
}

func TestMigration251MaintainsRollupsWithStatementTriggers(t *testing.T) {
	content, err := FS.ReadFile("251_usage_user_daily_rollups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TRIGGER usage_logs_user_rollup_insert")
	require.Contains(t, sql, "REFERENCING NEW TABLE AS new_usage_logs")
	require.Contains(t, sql, "CREATE TRIGGER usage_logs_user_rollup_delete")
	require.Contains(t, sql, "REFERENCING OLD TABLE AS old_usage_logs")
	require.Contains(t, sql, "CREATE TRIGGER usage_logs_user_rollup_update")
	require.Contains(t, sql, "REFERENCING OLD TABLE AS old_usage_logs NEW TABLE AS new_usage_logs")
	require.Contains(t, sql, "FOR EACH STATEMENT")
	require.Contains(t, sql, "SELECT rebuild_usage_user_daily_rollups(DATE '1970-01-01', DATE '9999-12-30')")
}

func TestMigration251AddsLedgerUserCreatedIndex(t *testing.T) {
	content, err := FS.ReadFile("251_affiliate_ledger_user_created_index_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_user_created")
	require.Contains(t, sql, "ON user_affiliate_ledger (user_id, created_at) INCLUDE (source_user_id, reward_type, amount)")
}
