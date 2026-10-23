package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// CUSTOM: 汇率来源客户端（本地 httptest，不访问外网）。
func TestFxRateClient_FetchFxRateJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			require.Equal(t, "application/json", r.Header.Get("Accept"))
			_, _ = w.Write([]byte(`{"rates":{"CNY":7.1}}`))
		case "/huge":
			_, _ = w.Write([]byte(strings.Repeat("x", fxRateResponseMaxBytes+10)))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	client := NewFxRateClient("", false)
	body, err := client.FetchFxRateJSON(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	require.JSONEq(t, `{"rates":{"CNY":7.1}}`, string(body))

	_, err = client.FetchFxRateJSON(context.Background(), srv.URL+"/down")
	require.ErrorContains(t, err, "HTTP 503")

	_, err = client.FetchFxRateJSON(context.Background(), srv.URL+"/huge")
	require.ErrorContains(t, err, "exceeds")
}

func TestFxRateClient_ProxyInitFailureDoesNotFallBackToDirect(t *testing.T) {
	client := NewFxRateClient("://bad-proxy", false)
	_, err := client.FetchFxRateJSON(context.Background(), "http://127.0.0.1:1/never")
	require.ErrorContains(t, err, "direct fallback is disabled")
}
