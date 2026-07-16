// CUSTOM: Provides an isolated, no-redirect HTTP client for balance queries.
package repository

import (
	"io"
	"net"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const upstreamBalanceDialTimeout = 10 * time.Second

type upstreamBalanceHTTPClient struct{}

func NewUpstreamBalanceHTTPClient() service.UpstreamBalanceHTTPClient {
	return &upstreamBalanceHTTPClient{}
}

func (client *upstreamBalanceHTTPClient) Do(request *http.Request, rawProxyURL string) (*http.Response, error) {
	_, parsedProxyURL, err := proxyurl.Parse(rawProxyURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: upstreamBalanceDialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   upstreamBalanceDialTimeout,
		ResponseHeaderTimeout: upstreamBalanceDialTimeout,
	}
	if err := proxyutil.ConfigureTransportProxy(transport, parsedProxyURL); err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}

	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := httpClient.Do(request)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	response.Body = &closeTransportResponseBody{
		ReadCloser: response.Body,
		transport:  transport,
	}
	return response, nil
}

type closeTransportResponseBody struct {
	io.ReadCloser
	transport *http.Transport
}

func (body *closeTransportResponseBody) Close() error {
	err := body.ReadCloser.Close()
	body.transport.CloseIdleConnections()
	return err
}
