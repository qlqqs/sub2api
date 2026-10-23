//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAffiliateRebateSettleService_DueRereadsInterval(t *testing.T) {
	settings := &twoTierSettingRepo{values: map[string]string{}}
	svc := NewAffiliateRebateSettleService(nil, NewSettingService(settings, nil))
	start := time.Now()

	// 未配置时用默认 60 分钟
	require.False(t, svc.due(start, start.Add(10*time.Minute)))
	require.True(t, svc.due(start, start.Add(60*time.Minute)))

	// 改成 5 分钟后下一次检查即按新间隔判断，不必等旧的 60 分钟跑完
	require.NoError(t, settings.Set(t.Context(), SettingKeyAffiliateSettleIntervalMinutes, "5"))
	require.False(t, svc.due(start, start.Add(4*time.Minute)))
	require.True(t, svc.due(start, start.Add(10*time.Minute)))
}

// 测试环境覆盖：结算周期取代后台设置（不受最少 5 分钟限制），每个节拍都执行；滞后窗口同步缩短。
func TestAffiliateRebateSettleService_SettleOverrides(t *testing.T) {
	settings := &twoTierSettingRepo{values: map[string]string{SettingKeyAffiliateSettleIntervalMinutes: "5"}}
	svc := NewAffiliateRebateSettleService(nil, NewSettingService(settings, nil))
	start := time.Now()

	require.Equal(t, affiliateSettleCheckInterval, svc.checkInterval())
	require.Equal(t, affiliateSettleLag, svc.settleLag())
	require.False(t, svc.due(start, start.Add(10*time.Second)))

	svc.SetSettleOverrides(10*time.Second, 10*time.Second)
	require.Equal(t, 10*time.Second, svc.checkInterval())
	require.Equal(t, 10*time.Second, svc.settleLag())
	require.True(t, svc.due(start, start.Add(10*time.Second)))

	// 非正数等于不覆盖
	svc.SetSettleOverrides(-time.Second, 0)
	require.Equal(t, affiliateSettleCheckInterval, svc.checkInterval())
	require.Equal(t, affiliateSettleLag, svc.settleLag())
	require.False(t, svc.due(start, start.Add(10*time.Second)))
}
