//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type signupPreviewSettings struct{ *settingHandlerPublicRepoStub }

func (s *signupPreviewSettings) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

type signupPreviewRepo struct {
	service.AffiliateRepository // writes deliberately unimplemented: preview must be read-only
	err                         error
}

func (r signupPreviewRepo) GetAffiliateByCode(_ context.Context, code string) (*service.AffiliateSummary, error) {
	if r.err != nil {
		return nil, r.err
	}
	if code != "CODE1" {
		return nil, service.ErrAffiliateProfileNotFound
	}
	return &service.AffiliateSummary{UserID: 1}, nil
}
func (r signupPreviewRepo) GetAffiliateNodes(context.Context, []int64) (map[int64]service.AffiliateNode, error) {
	return map[int64]service.AffiliateNode{1: {UserID: 1, Active: true, AgentLevel: 1, Email: "private@example.com"}}, nil
}

func TestPreviewAffiliateSignupBonus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body, source string
		repoErr            error
		status             int
		amount             string
	}{
		{"matching", `{"aff_code":" code1 "}`, "agent", nil, 200, "5"},
		{"excluded", `{"aff_code":"CODE1"}`, "user", nil, 200, "0"},
		{"missing", `{}`, "all", nil, 200, "0"},
		{"unknown", `{"aff_code":"MISSING"}`, "all", nil, 200, "0"},
		{"malformed", `{`, "all", nil, 400, ""},
		{"too long", `{"aff_code":"` + strings.Repeat("A", 129) + `"}`, "all", nil, 400, ""},
		{"failure", `{"aff_code":"CODE1"}`, "all", errors.New("private database failure"), 500, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := &signupPreviewSettings{&settingHandlerPublicRepoStub{values: map[string]string{
				service.SettingKeyAffiliateEnabled: "true", service.SettingKeyAffiliateInviteeSignupBonus: "5", service.SettingKeyAffiliateInviteeSignupBonusSource: tc.source,
			}}}
			svc := service.NewAffiliateService(signupPreviewRepo{err: tc.repoErr}, service.NewSettingService(settings, &config.Config{}), nil, nil)
			h := &UserHandler{affiliateService: svc}
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest("POST", "/api/v1/auth/affiliate-signup-bonus", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			h.PreviewAffiliateSignupBonus(ctx)
			require.Equal(t, tc.status, rec.Code)
			if tc.status == 200 {
				require.JSONEq(t, `{"code":0,"message":"success","data":{"amount":`+tc.amount+`}}`, rec.Body.String())
				require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			}
			require.NotContains(t, rec.Body.String(), "private")
		})
	}
}
