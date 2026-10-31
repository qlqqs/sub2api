//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestClientReleaseAdminRoutesAuthAndStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{ClientRelease: &adminhandler.ClientReleaseHandler{}}}
	for _, tc := range []struct {
		method, path string
		stepUp       bool
	}{
		{"GET", "/client-releases", false}, {"POST", "/client-releases", true},
		{"POST", "/client-releases/1/files/qxapi-linux-amd64/check", true}, {"PUT", "/client-releases/1/files/qxapi-linux-amd64", true},
		{"PUT", "/client-releases/1/notes", false}, {"POST", "/client-releases/1/publish", true}, {"POST", "/client-releases/1/withdraw", true}, {"DELETE", "/client-releases/1", true},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			for token, want := range map[string]int{"": 401, "user": 403} {
				r := gin.New()
				auth := middleware.AdminAuthMiddleware(func(c *gin.Context) {
					if c.GetHeader("Authorization") == "" {
						c.AbortWithStatus(401)
					} else {
						c.AbortWithStatus(403)
					}
				})
				RegisterAdminRoutes(r.Group("/api/v1"), h, auth, middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), middleware.StepUpAuthMiddleware(func(c *gin.Context) { t.Fatal("step-up reached before admin auth") }), nil, nil)
				req := httptest.NewRequest(tc.method, "/api/v1/admin"+tc.path, nil)
				req.Header.Set("Authorization", token)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				require.Equal(t, want, w.Code)
			}
			if tc.stepUp {
				r := gin.New()
				registerClientReleaseRoutes(r.Group("/api/v1/admin"), h, middleware.StepUpAuthMiddleware(func(c *gin.Context) { middleware.AbortWithError(c, 403, "STEP_UP_REQUIRED", "Verify TOTP") }))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(tc.method, "/api/v1/admin"+tc.path, nil))
				require.Equal(t, 403, w.Code)
				require.Contains(t, w.Body.String(), "STEP_UP_REQUIRED")
			}
		})
	}
}

type routeClientReleaseRepo struct {
	service.ClientReleaseRepository
}

func (routeClientReleaseRepo) List(context.Context) ([]*service.ClientRelease, error) {
	return []*service.ClientRelease{}, nil
}
func (routeClientReleaseRepo) GetByID(context.Context, int64) (*service.ClientRelease, error) {
	return &service.ClientRelease{ID: 1, Status: service.ClientReleaseDraft, Manifest: []byte(`{"version":"1.0.0","files":{"qxapi-linux-amd64":{"size":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}`)}, nil
}
func TestClientReleaseCheckRouteGranted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := service.NewClientReleaseService(routeClientReleaseRepo{}, nil)
	h := &handler.Handlers{Admin: &handler.AdminHandlers{ClientRelease: adminhandler.NewClientReleaseHandler(s)}}
	r := gin.New()
	called := false
	registerClientReleaseRoutes(r.Group("/api/v1/admin"), h, middleware.StepUpAuthMiddleware(func(c *gin.Context) { called = true; c.Next() }))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/client-releases/1/files/qxapi-linux-amd64/check", nil))
	require.True(t, called)
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":null}`, w.Body.String())
}
func TestClientReleasePublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Handlers{ClientRelease: handler.NewClientReleaseHandler(service.NewClientReleaseService(routeClientReleaseRepo{}, nil))}
	r := gin.New()
	RegisterClientReleaseRoutes(r.Group("/api/v1"), h, middleware.NewPanelRateLimiter(nil, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/client-releases/qxapi/latest", nil))
	require.Equal(t, 404, w.Code)
	require.Contains(t, w.Body.String(), "CLIENT_RELEASE_NOT_FOUND")
	routes := map[string]bool{}
	for _, route := range r.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, key := range []string{"GET /api/v1/client-releases/qxapi/latest", "GET /api/v1/client-releases/qxapi/notes", "GET /api/v1/client-releases/qxapi/files/:version/:name", "HEAD /api/v1/client-releases/qxapi/files/:version/:name"} {
		require.True(t, routes[key], key)
	}
}

type clientReleaseRateSettings struct{ service.SettingRepository }

func (clientReleaseRateSettings) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"public_ip_rpm":1}`, nil
}
func TestClientReleasePublicRoutesShareIPLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	settings := service.NewSettingService(clientReleaseRateSettings{}, &config.Config{})
	h := &handler.Handlers{ClientRelease: handler.NewClientReleaseHandler(service.NewClientReleaseService(routeClientReleaseRepo{}, settings))}
	r := gin.New()
	RegisterClientReleaseRoutes(r.Group("/api/v1"), h, middleware.NewPanelRateLimiter(redisClient, settings))
	request := func(method, path, ip string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v1/client-releases/qxapi"+path, nil)
		req.RemoteAddr = ip + ":1234"
		r.ServeHTTP(w, req)
		return w.Code
	}
	require.Equal(t, 404, request("GET", "/latest", "203.0.113.9"))
	for _, tc := range []struct{ method, path string }{{"GET", "/latest"}, {"GET", "/notes"}, {"GET", "/files/1.0.0/qxapi-linux-amd64"}, {"HEAD", "/files/1.0.0/qxapi-linux-amd64"}} {
		require.Equal(t, 429, request(tc.method, tc.path, "203.0.113.9"), tc.path)
	}
	require.Equal(t, 404, request("GET", "/latest", "198.51.100.7"))
}
