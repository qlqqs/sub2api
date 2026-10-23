//go:build unit

package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration252CreatesAgentRewardDailyRollups(t *testing.T) {
	content, err := FS.ReadFile("252_affiliate_agent_reward_daily_rollups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS affiliate_agent_reward_daily_rollups")
	require.Contains(t, sql, "PRIMARY KEY (user_id, source_user_id, bucket_date)")
	require.Contains(t, sql, "amount DECIMAL(20, 8)")
	require.Contains(t, sql, "(n.created_at AT TIME ZONE 'Asia/Shanghai')::date")
	require.Contains(t, sql, "COALESCE(n.source_user_id, 0)")
	require.Contains(t, sql, "WHERE n.reward_type IN ('agent_direct', 'agent_diff')")
	require.Contains(t, sql, "ORDER BY 1, 2, 3")
	require.Contains(t, sql, "HAVING SUM(d.amount) <> 0 OR SUM(d.entries) <> 0")
	require.Contains(t, sql, "EXCEPTION WHEN OTHERS THEN")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION rebuild_affiliate_agent_reward_daily_rollups(from_date DATE, to_date DATE)")
	require.Contains(t, sql, "LOCK TABLE user_affiliate_ledger IN SHARE MODE")
}

func TestMigration252MaintainsRollupsWithStatementTriggers(t *testing.T) {
	content, err := FS.ReadFile("252_affiliate_agent_reward_daily_rollups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TRIGGER user_affiliate_ledger_reward_rollup_insert")
	require.Contains(t, sql, "REFERENCING NEW TABLE AS new_ledger")
	require.Contains(t, sql, "CREATE TRIGGER user_affiliate_ledger_reward_rollup_delete")
	require.Contains(t, sql, "REFERENCING OLD TABLE AS old_ledger")
	require.Contains(t, sql, "CREATE TRIGGER user_affiliate_ledger_reward_rollup_update")
	require.Contains(t, sql, "REFERENCING OLD TABLE AS old_ledger NEW TABLE AS new_ledger")
	require.Contains(t, sql, "FOR EACH STATEMENT")
	require.Contains(t, sql, "SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '1970-01-01', DATE '9999-12-30')")
}
