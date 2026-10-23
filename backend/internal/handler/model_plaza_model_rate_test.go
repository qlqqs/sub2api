//go:build unit

package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// CUSTOM: 模型 DTO 的 rate_multiplier / user_rate_multiplier（design §15.1）。
func TestToModelPlazaGroupDTO_ModelLevelRateMultipliers(t *testing.T) {
	g := service.PlazaGroup{
		ID: 5, Name: "g", Platform: "anthropic", SubscriptionType: "standard", RateMultiplier: 0.6,
		Models: []service.PlazaModel{
			{Name: "claude-opus-4-5", Platform: "anthropic", RateMultiplier: 1.0, HasModelRate: true},
			{Name: "claude-sonnet-4", Platform: "anthropic", RateMultiplier: 0.6},
			{Name: "legacy-unfilled", Platform: "anthropic"}, // 未注入模型倍率服务：回退分组倍率
		},
	}

	decode := func(userRates map[int64]float64) []map[string]any {
		raw, err := json.Marshal(toModelPlazaGroupDTO(&g, userRates))
		require.NoError(t, err)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(raw, &decoded))
		out := []map[string]any{}
		for _, m := range decoded["models"].([]any) {
			out = append(out, m.(map[string]any))
		}
		return out
	}

	anonymous := decode(nil)
	require.InDelta(t, 1.0, anonymous[0]["rate_multiplier"].(float64), 1e-12)
	require.InDelta(t, 0.6, anonymous[1]["rate_multiplier"].(float64), 1e-12)
	require.InDelta(t, 0.6, anonymous[2]["rate_multiplier"].(float64), 1e-12)
	for _, m := range anonymous {
		_, has := m["user_rate_multiplier"]
		require.False(t, has, "无专属倍率时省略")
	}

	withUserRate := decode(map[int64]float64{5: 0.4})
	_, has := withUserRate[0]["user_rate_multiplier"]
	require.False(t, has, "有单独倍率的模型不给专属倍率")
	require.InDelta(t, 0.4, withUserRate[1]["user_rate_multiplier"].(float64), 1e-12)
	require.InDelta(t, 0.4, withUserRate[2]["user_rate_multiplier"].(float64), 1e-12)
	require.InDelta(t, 0.6, withUserRate[1]["rate_multiplier"].(float64), 1e-12, "公开倍率不随用户变化")
}
