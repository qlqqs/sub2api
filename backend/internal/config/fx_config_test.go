//go:build unit

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// CUSTOM: fx.source_urls 必须能经环境变量 FX_SOURCE_URLS 注入（本机 e2e 指向模拟汇率接口）。
// viper 的默认 decode hook 按逗号把字符串拆成切片；空白分隔由服务层再拆一次。
func TestLoadFXSourceURLsFromEnv(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("FX_SOURCE_URLS", "http://127.0.0.1:1/a,http://127.0.0.1:2/b")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, []string{"http://127.0.0.1:1/a", "http://127.0.0.1:2/b"}, cfg.FX.SourceURLs)
}

func TestLoadFXSourceURLsDefaultsEmpty(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.Empty(t, cfg.FX.SourceURLs)
}
