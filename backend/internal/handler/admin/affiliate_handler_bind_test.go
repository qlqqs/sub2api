//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type affiliateBindHandlerRepo struct {
	service.AffiliateRepository
	boundAt time.Time
}

func (r *affiliateBindHandlerRepo) EnsureUserAffiliate(_ context.Context, userID int64) (*service.AffiliateSummary, error) {
	return &service.AffiliateSummary{UserID: userID}, nil
}

func (r *affiliateBindHandlerRepo) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r *affiliateBindHandlerRepo) GetAffiliateNodes(_ context.Context, ids []int64) (map[int64]service.AffiliateNode, error) {
	out := map[int64]service.AffiliateNode{}
	for _, id := range ids {
		out[id] = service.AffiliateNode{UserID: id, AgentLevel: service.AffiliateAgentLevelDefault, Active: true}
	}
	return out, nil
}

func (r *affiliateBindHandlerRepo) BindInviter(_ context.Context, inviteeID, inviterID int64, binding service.AffiliateInviterBinding) (*service.AffiliateInviterBindingResult, bool, error) {
	return &service.AffiliateInviterBindingResult{InviteeID: inviteeID, InviterID: inviterID, BoundAt: r.boundAt, Source: binding.Source}, true, nil
}

func TestAffiliateHandlerBindInviterUsesAuthenticatedAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &affiliateBindHandlerRepo{boundAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)}
	handler := NewAffiliateHandler(service.NewAffiliateService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
	})
	router.POST("/api/v1/admin/affiliates/users/:user_id/inviter", handler.BindInviter)

	body, err := json.Marshal(gin.H{"inviter_id": 22, "reason": " verified by support "})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/affiliates/users/11/inviter", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"invitee_id":11,"inviter_id":22,"bound_at":"2026-09-21T12:00:00Z","source":"admin"}}`, resp.Body.String())
}
