package repository

// CUSTOM(fork): 美元兑人民币汇率来源的 HTTP 客户端。代理配置与拉取价格表相同
// （update.proxy_url + security.proxy_fallback.allow_direct_on_error），见 pricing_service.go。

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// fxRateResponseMaxBytes 汇率接口响应体上限：正常响应只有几 KB（currency-api 全币种约 10 KB）。
const fxRateResponseMaxBytes = 1 << 20

type fxRateClient struct {
	httpClient *http.Client
}

// fxRateClientError 代理初始化失败且不允许直连时的占位客户端：所有请求直接返回初始化错误。
type fxRateClientError struct {
	err error
}

func (c *fxRateClientError) FetchFxRateJSON(_ context.Context, _ string) ([]byte, error) {
	return nil, c.err
}

// NewFxRateClient 创建汇率来源客户端。proxyURL 为空时直连；代理配置失败时：
//   - allowDirectOnProxyError=false（默认）：返回错误占位客户端，禁止回退直连；
//   - allowDirectOnProxyError=true：回退直连。
func NewFxRateClient(proxyURL string, allowDirectOnProxyError bool) service.FxRateFetcher {
	sharedClient, err := httpclient.GetClient(httpclient.Options{
		Timeout:  30 * time.Second,
		ProxyURL: proxyURL,
	})
	if err != nil {
		if strings.TrimSpace(proxyURL) != "" && !allowDirectOnProxyError {
			slog.Warn("proxy client init failed, all requests will fail", "service", "fx_rate", "error", err)
			return &fxRateClientError{err: fmt.Errorf("proxy client init failed and direct fallback is disabled; set security.proxy_fallback.allow_direct_on_error=true to allow fallback: %w", err)}
		}
		sharedClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &fxRateClient{httpClient: sharedClient}
}

// ProvideFxRateClient 从配置读取代理设置创建汇率来源客户端。
func ProvideFxRateClient(cfg *config.Config) service.FxRateFetcher {
	return NewFxRateClient(cfg.Update.ProxyURL, cfg.Security.ProxyFallback.AllowDirectOnError)
}

func (c *fxRateClient) FetchFxRateJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fxRateResponseMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > fxRateResponseMaxBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", fxRateResponseMaxBytes)
	}
	return body, nil
}
