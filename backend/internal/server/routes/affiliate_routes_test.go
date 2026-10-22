package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAffiliateRoutesBindingAndWithdrawalCoexist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Affiliate: adminhandler.NewAffiliateHandler(nil, nil),
	}}
	admin := router.Group("/api/v1/admin")
	var matchedPath, userID string
	admin.Use(func(c *gin.Context) {
		matchedPath = c.FullPath()
		userID = c.Param("user_id")
		c.AbortWithStatus(http.StatusNoContent)
	})

	// Gin requires the same wildcard name for sibling routes of a method.
	require.NotPanics(t, func() { registerAffiliateRoutes(admin, handlers) })
	for _, path := range []string{"/api/v1/admin/affiliates/users/batch-agent-level", "/api/v1/admin/affiliates/agent-levels/reevaluate"} {
		matchedPath = ""
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		require.Equal(t, http.StatusNoContent, response.Code, path)
		require.Equal(t, path, matchedPath)
	}
	for _, tc := range []struct {
		method string
		suffix string
	}{
		{http.MethodPost, "/inviter"},
		{http.MethodPost, "/withdraw"},
		{http.MethodGet, "/overview"},
		{http.MethodGet, "/customers"},
		{http.MethodGet, "/inviter/preview"},
		{http.MethodPut, "/agent-owner"},
		{http.MethodPut, ""},
		{http.MethodDelete, ""},
	} {
		t.Run(tc.method+tc.suffix, func(t *testing.T) {
			matchedPath, userID = "", ""
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, "/api/v1/admin/affiliates/users/11"+tc.suffix, nil)
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, "/api/v1/admin/affiliates/users/:user_id"+tc.suffix, matchedPath)
			require.Equal(t, "11", userID)
		})
	}
}
