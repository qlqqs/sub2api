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
