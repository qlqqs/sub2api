//go:build unit

package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeStudioJWT 模拟 JWT 中间件：带 "Bearer good-<role>" 视为登录，否则 401。
func fakeStudioJWT(c *gin.Context) {
	switch c.GetHeader("Authorization") {
	case "Bearer good-user":
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Set(string(middleware2.ContextKeyUserRole), "user")
	case "Bearer good-admin":
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 1})
		c.Set(string(middleware2.ContextKeyUserRole), "admin")
	default:
		middleware2.AbortWithError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid token")
		return
	}
	c.Next()
}

type studioUpstreamHit struct {
	path    string
	headers http.Header
}

func newStudioTestEngine(t *testing.T, novaURL string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil)) // 与未配置 trusted_proxies 的生产一致：不信任客户端转发头
	r.Use(func(c *gin.Context) {
		// 与 SecurityHeaders 相同的全站默认值
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'")
		c.Next()
	})
	installStudioProxy(r, &config.Config{Studio: config.StudioConfig{NovaURL: novaURL}}, fakeStudioJWT)
	r.Use(func(c *gin.Context) { // 代替 SPA 中间件
		c.String(http.StatusOK, "spa")
		c.Abort()
	})
	return r
}

func newStudioUpstream(t *testing.T) (*httptest.Server, *[]studioUpstreamHit) {
	t.Helper()
	hits := &[]studioUpstreamHit{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits = append(*hits, studioUpstreamHit{path: r.URL.RequestURI(), headers: r.Header.Clone()})
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		_, _ = w.Write([]byte("nova"))
	}))
	t.Cleanup(srv.Close)
	return srv, hits
}

type studioResponse struct {
	Code   int
	Header http.Header
	Body   string
}

// doStudio 经真实 HTTP 服务器发请求（ResponseRecorder 不支持 ReverseProxy 需要的 CloseNotify）。
func doStudio(t *testing.T, r *gin.Engine, method, path string, headers map[string]string) studioResponse {
	t.Helper()
	srv := httptest.NewServer(r)
	defer srv.Close()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultTransport.RoundTrip(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return studioResponse{Code: res.StatusCode, Header: res.Header, Body: string(body)}
}

func TestStudioProxy_DisabledReturns503(t *testing.T) {
	r := newStudioTestEngine(t, "")

	w := doStudio(t, r, http.MethodGet, "/_nova/index.html", nil)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body, "STUDIO_UNAVAILABLE")

	w = doStudio(t, r, http.MethodPost, "/api/nova/tasks", map[string]string{"Authorization": "Bearer good-user"})
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestStudioProxy_InvalidURLDisables(t *testing.T) {
	r := newStudioTestEngine(t, "ftp://nova")
	w := doStudio(t, r, http.MethodGet, "/_nova/", nil)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestStudioProxy_StaticNoAuthAndEmbeddableHeaders(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	w := doStudio(t, r, http.MethodGet, "/_nova/index.html?embedded=1", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "nova", w.Body)
	require.Len(t, *hits, 1)
	require.Equal(t, "/_nova/index.html?embedded=1", (*hits)[0].path)

	require.Equal(t, []string{"SAMEORIGIN"}, w.Header.Values("X-Frame-Options"))
	require.Equal(t, []string{studioContentSecurityPolicy}, w.Header.Values("Content-Security-Policy"))
}

func TestStudioProxy_APIRequiresLogin(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	w := doStudio(t, r, http.MethodGet, "/api/nova/tasks/abc", nil)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, *hits)
}

func TestStudioProxy_APIInjectsIdentityAndDropsForgedHeaders(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	w := doStudio(t, r, http.MethodPost, "/api/nova/tasks", map[string]string{
		"Authorization":       "Bearer good-user",
		"Cookie":              "a=b",
		"X-Sub2API-User-Id":   "1",
		"X-Sub2API-User-Role": "admin",
		"X-Sub2API-Other":     "x",
		"X-Forwarded-For":     "1.2.3.4",
		"X-Real-IP":           "1.2.3.4",
	})
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, *hits, 1)
	got := (*hits)[0].headers
	require.Equal(t, "42", got.Get("X-Sub2API-User-Id"))
	require.Equal(t, []string{"42"}, got.Values("X-Sub2API-User-Id"))
	require.Equal(t, "user", got.Get("X-Sub2API-User-Role"))
	require.Empty(t, got.Get("X-Sub2API-Other"))
	require.Equal(t, "127.0.0.1", got.Get("X-Forwarded-For"))
	require.Equal(t, "127.0.0.1", got.Get("X-Real-IP"))
	require.Empty(t, got.Get("Authorization"))
	require.Empty(t, got.Get("Cookie"))
}

func TestStudioProxy_AdminRole(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	w := doStudio(t, r, http.MethodGet, "/api/nova/plugins", map[string]string{"Authorization": "Bearer good-admin"})
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "admin", (*hits)[0].headers.Get("X-Sub2API-User-Role"))
	require.Equal(t, "1", (*hits)[0].headers.Get("X-Sub2API-User-Id"))
}

func TestStudioProxy_WebSocketPathSkipsJWTButNoIdentity(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	w := doStudio(t, r, http.MethodGet, "/api/nova/ws?ticket=t1", map[string]string{"X-Sub2API-User-Id": "1"})
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, *hits, 1)
	require.Equal(t, "/api/nova/ws?ticket=t1", (*hits)[0].path)
	require.Empty(t, (*hits)[0].headers.Get("X-Sub2API-User-Id"))
	require.Equal(t, "127.0.0.1", (*hits)[0].headers.Get("X-Forwarded-For"))
}

func TestStudioProxy_OtherPathsUntouched(t *testing.T) {
	up, hits := newStudioUpstream(t)
	r := newStudioTestEngine(t, up.URL)

	for _, p := range []string{"/", "/studio", "/_novax", "/api/novax"} {
		w := doStudio(t, r, http.MethodGet, p, nil)
		require.Equal(t, "spa", w.Body, p)
		require.Equal(t, "DENY", w.Header.Get("X-Frame-Options"), p)
	}
	require.Empty(t, *hits)
}

func TestStudioProxy_StatusEndpoint(t *testing.T) {
	w := doStudio(t, newStudioTestEngine(t, ""), http.MethodGet, "/api/v1/studio/status", nil)
	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"enabled":false}}`, w.Body)

	up, _ := newStudioUpstream(t)
	w = doStudio(t, newStudioTestEngine(t, up.URL), http.MethodGet, "/api/v1/studio/status", nil)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"enabled":true}}`, w.Body)
}
