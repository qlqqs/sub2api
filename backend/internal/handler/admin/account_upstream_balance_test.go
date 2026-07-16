package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type upstreamBalanceQuerierStub struct {
	result          *service.UpstreamBalanceResult
	err             error
	requestedID     int64
	requestCanceled bool
	callCount       int
}

func (stub *upstreamBalanceQuerierStub) QueryAccountBalance(
	ctx context.Context,
	accountID int64,
) (*service.UpstreamBalanceResult, error) {
	stub.callCount++
	stub.requestedID = accountID
	stub.requestCanceled = ctx.Err() != nil
	return stub.result, stub.err
}

func TestQueryUpstreamBalanceHandlerRejectsUnauthenticatedRequests(t *testing.T) {
	querier := &upstreamBalanceQuerierStub{}
	router := gin.New()
	handler := &AccountHandler{upstreamBalanceService: querier}
	adminAuth := servermiddleware.NewAdminAuthMiddleware(nil, nil, nil)
	router.POST(
		"/admin/accounts/:id/upstream-balance",
		gin.HandlerFunc(adminAuth),
		handler.QueryUpstreamBalance,
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/accounts/42/upstream-balance", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, 0, querier.callCount)
}

func setupUpstreamBalanceHandlerRouter(querier upstreamBalanceQuerier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &AccountHandler{upstreamBalanceService: querier}
	router.POST("/admin/accounts/:id/upstream-balance", handler.QueryUpstreamBalance)
	return router
}

func decodeUpstreamBalanceHandlerResponse(t *testing.T, recorder *httptest.ResponseRecorder) response.Response {
	t.Helper()

	var decoded response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &decoded))
	return decoded
}

func TestQueryUpstreamBalanceHandlerSuccess(t *testing.T) {
	queriedAt := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	querier := &upstreamBalanceQuerierStub{
		result: &service.UpstreamBalanceResult{
			Status:       service.UpstreamBalanceStatusAvailable,
			PlatformType: service.UpstreamBalancePlatformSub2API,
			Scope:        service.UpstreamBalanceScopeUser,
			Remaining:    "12.5",
			Unit:         "USD",
			QueriedAt:    queriedAt,
		},
	}
	router := setupUpstreamBalanceHandlerRouter(querier)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/accounts/42/upstream-balance", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), querier.requestedID)
	require.False(t, querier.requestCanceled)
	decoded := decodeUpstreamBalanceHandlerResponse(t, recorder)
	require.Equal(t, 0, decoded.Code)
	require.Equal(t, "success", decoded.Message)
	data, ok := decoded.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, service.UpstreamBalanceStatusAvailable, data["status"])
	require.Equal(t, "12.5", data["remaining"])
}

func TestQueryUpstreamBalanceHandlerValidationAndErrors(t *testing.T) {
	testCases := []struct {
		name           string
		requestPath    string
		querier        upstreamBalanceQuerier
		expectedStatus int
		expectedReason string
	}{
		{
			name:           "invalid account id",
			requestPath:    "/admin/accounts/not-a-number/upstream-balance",
			querier:        &upstreamBalanceQuerierStub{},
			expectedStatus: http.StatusBadRequest,
			expectedReason: "INVALID_ACCOUNT_ID",
		},
		{
			name:           "service unavailable",
			requestPath:    "/admin/accounts/42/upstream-balance",
			querier:        nil,
			expectedStatus: http.StatusServiceUnavailable,
			expectedReason: "UPSTREAM_BALANCE_UNAVAILABLE",
		},
		{
			name:        "credential required",
			requestPath: "/admin/accounts/42/upstream-balance",
			querier: &upstreamBalanceQuerierStub{
				err: infraerrors.BadRequest(
					"UPSTREAM_BALANCE_CREDENTIAL_REQUIRED",
					"configure balance query credentials",
				),
			},
			expectedStatus: http.StatusBadRequest,
			expectedReason: "UPSTREAM_BALANCE_CREDENTIAL_REQUIRED",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			router := setupUpstreamBalanceHandlerRouter(testCase.querier)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, testCase.requestPath, nil)

			router.ServeHTTP(recorder, request)

			require.Equal(t, testCase.expectedStatus, recorder.Code)
			decoded := decodeUpstreamBalanceHandlerResponse(t, recorder)
			require.Equal(t, testCase.expectedReason, decoded.Reason)
		})
	}
}
