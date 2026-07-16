package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type usageSubscriptionProviderStub struct {
	subscription *service.UserSubscription
	err          error
	userID       int64
	groupID      int64
	calls        int
}

func (stub *usageSubscriptionProviderStub) GetActiveSubscriptionForUsage(
	ctx context.Context,
	userID int64,
	groupID int64,
) (*service.UserSubscription, error) {
	stub.calls++
	stub.userID = userID
	stub.groupID = groupID
	return stub.subscription, stub.err
}

func TestUsageUnrestrictedIncludesWeeklyWindowStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	weeklyWindowStart := time.Date(2026, time.July, 13, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{
		WeeklyWindowStart: &weeklyWindowStart,
	})

	handler := &GatewayHandler{}
	handler.usageUnrestricted(
		c,
		context.Background(),
		&service.APIKey{Group: &service.Group{
			Name:             "Weekly plan",
			SubscriptionType: service.SubscriptionTypeSubscription,
		}},
		middleware.AuthSubject{},
		nil,
		nil,
		nil,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Remaining    float64 `json:"remaining"`
		Subscription struct {
			WeeklyWindowStart *time.Time `json:"weekly_window_start"`
		} `json:"subscription"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotNil(t, response.Subscription.WeeklyWindowStart)
	require.True(t, weeklyWindowStart.Equal(*response.Subscription.WeeklyWindowStart))
	require.Equal(t, float64(-1), response.Remaining)
}

func TestUsageUnrestrictedLoadsActiveSubscriptionWhenContextIsMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	dailyLimit := 10.0
	provider := &usageSubscriptionProviderStub{
		subscription: &service.UserSubscription{DailyUsageUSD: 3.25},
	}
	handler := &GatewayHandler{usageSubscriptionProvider: provider}
	handler.usageUnrestricted(
		ginContext,
		context.Background(),
		&service.APIKey{Group: &service.Group{
			ID:               21,
			Name:             "Daily plan",
			SubscriptionType: service.SubscriptionTypeSubscription,
			DailyLimitUSD:    &dailyLimit,
		}},
		middleware.AuthSubject{UserID: 15},
		nil,
		nil,
		nil,
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Mode      string  `json:"mode"`
		Remaining float64 `json:"remaining"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "unrestricted", response.Mode)
	require.Equal(t, 6.75, response.Remaining)
	require.Equal(t, 1, provider.calls)
	require.Equal(t, int64(15), provider.userID)
	require.Equal(t, int64(21), provider.groupID)
}

func TestUsageUnrestrictedRejectsMissingSubscriptionInsteadOfOmittingBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	provider := &usageSubscriptionProviderStub{err: errors.New("subscription not found")}
	handler := &GatewayHandler{usageSubscriptionProvider: provider}
	handler.usageUnrestricted(
		ginContext,
		context.Background(),
		&service.APIKey{Group: &service.Group{
			ID:               21,
			SubscriptionType: service.SubscriptionTypeSubscription,
		}},
		middleware.AuthSubject{UserID: 15},
		nil,
		nil,
		nil,
	)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.NotContains(t, recorder.Body.String(), `"mode":"unrestricted"`)
	require.NotContains(t, recorder.Body.String(), `"remaining":0`)
}
