//go:build unit

package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// CUSTOM: 结算加速只经环境变量注入（本机测试环境的 docker-compose），默认不覆盖。
func TestLoadAffiliateSettleOverridesFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("AFFILIATE_SETTLE_INTERVAL_OVERRIDE", "10s")
	t.Setenv("AFFILIATE_SETTLE_LAG_OVERRIDE", "10s")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, cfg.Affiliate.SettleIntervalOverride)
	require.Equal(t, 10*time.Second, cfg.Affiliate.SettleLagOverride)
}

func TestLoadAffiliateSettleOverridesDefaultZero(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.Zero(t, cfg.Affiliate.SettleIntervalOverride)
	require.Zero(t, cfg.Affiliate.SettleLagOverride)
}
