//go:build unit

package migrations

// CUSTOM(fork): 迁移仅添加财务配置与快照，不修改已有业务数据。
import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinanceMigrations254To256(t *testing.T) {
	for file, checks := range map[string][]string{
		"254_account_model_cost_multipliers.sql": {"CREATE TABLE IF NOT EXISTS account_model_cost_multipliers", "REFERENCES accounts(id) ON DELETE CASCADE", "rate_multiplier >= 0", "PRIMARY KEY (account_id, model_pattern)"},
		"255_finance_expenses.sql":               {"CREATE TABLE IF NOT EXISTS finance_expenses", "amount > 0", "BETWEEN 1 AND 3660", "ON DELETE SET NULL", "WHERE deleted_at IS NULL"},
		"256_payment_channel_fee_rate.sql":       {"ADD COLUMN IF NOT EXISTS channel_fee_rate", "NUMERIC(6,4)", "refund_balance_deducted", "NUMERIC(20,8)"},
	} {
		t.Run(file, func(t *testing.T) {
			b, err := FS.ReadFile(file)
			require.NoError(t, err)
			s := string(b)
			for _, c := range checks {
				require.Contains(t, s, c)
			}
			require.NotContains(t, strings.ToUpper(s), "DROP TABLE")
			require.NotContains(t, strings.ToUpper(s), "UPDATE USAGE_LOGS")
		})
	}
}

func TestFinanceRetainedBaselineGenerationMigration(t *testing.T) {
	raw, err := FS.ReadFile("260_finance_retained_baseline_generation.sql")
	require.NoError(t, err)
	s := string(raw)
	require.Contains(t, s, "ADD COLUMN IF NOT EXISTS deletion_generation BIGINT NOT NULL DEFAULT 0")
	require.Equal(t, 2, strings.Count(s, "ADD COLUMN IF NOT EXISTS baseline_generation BIGINT"))
	require.Contains(t, s, "IF frontier IS NOT NULL THEN")
	require.Equal(t, 2, strings.Count(s, "deletion_generation = deletion_generation + 1"))
	require.NotContains(t, strings.ToUpper(s), "UPDATE USAGE_LOGS")
	require.NotContains(t, strings.ToUpper(s), "DELETE FROM")
	require.NotContains(t, s, "SET complete_from = frontier")
}
