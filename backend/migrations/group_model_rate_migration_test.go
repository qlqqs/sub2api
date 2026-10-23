//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// CUSTOM: fork 自有表 group_model_rate_multipliers（分组内单个模型倍率）。
func TestMigration253CreatesGroupModelRateMultipliers(t *testing.T) {
	content, err := FS.ReadFile("253_group_model_rate_multipliers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS group_model_rate_multipliers")
	require.Contains(t, sql, "group_id        BIGINT        NOT NULL REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, sql, "model_pattern   VARCHAR(255)  NOT NULL")
	require.Contains(t, sql, "rate_multiplier DECIMAL(10,4) NOT NULL CHECK (rate_multiplier > 0)")
	require.Contains(t, sql, "PRIMARY KEY (group_id, model_pattern)")
	// 只新增表：不得改动 groups 表或 Ent 管理的表结构。
	require.NotContains(t, strings.ToUpper(sql), "ALTER TABLE GROUPS")
	require.NotContains(t, strings.ToUpper(sql), "DROP ")
}
