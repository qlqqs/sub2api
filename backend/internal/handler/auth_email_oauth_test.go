package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmailOAuthCallbackRequiresPendingRegistrationWhenInvitationEnabled(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandler(t, true)
	ctx := context.Background()

	state := "github-oauth-state"
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github/callback?code=code-1&state="+url.QueryEscape(state), nil)
	req.AddCookie(&http.Cookie{Name: emailOAuthStateCookieName, Value: encodeCookieValue(state)})
	req.AddCookie(&http.Cookie{Name: emailOAuthRedirectCookie, Value: encodeCookieValue("/dashboard")})
	req.AddCookie(&http.Cookie{Name: emailOAuthProviderCookie, Value: encodeCookieValue("github")})
	c.Request = req

	profile := &emailOAuthProfile{
		Subject:       "github-123",
		Email:         "fresh@example.com",
		EmailVerified: true,
		Username:      "fresh",
		DisplayName:   "Fresh User",
		AvatarURL:     "https://cdn.example/fresh.png",
		Metadata: map[string]any{
			"login": "fresh",
		},
	}
	handler.emailOAuthCallbackWithProfile(c, "github", config.EmailOAuthProviderConfig{
		Enabled:             true,
		ClientID:            "github-client",
		ClientSecret:        "github-secret",
		RedirectURL:         "https://app.example/api/v1/auth/oauth/github/callback",
		FrontendRedirectURL: "/auth/oauth/callback",
	}, "/auth/oauth/callback", "/dashboard", profile)

	require.Equal(t, http.StatusFound, recorder.Code)
	location := recorder.Header().Get("Location")
	require.Contains(t, location, "/auth/oauth/callback")
	require.NotContains(t, location, "access_token=")

	userCount, err := client.User.Query().Where(dbuser.EmailEQ("fresh@example.com")).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, userCount)

	session, err := client.PendingAuthSession.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "github", session.ProviderType)
	require.Equal(t, "github", session.ProviderKey)
	require.Equal(t, "github-123", session.ProviderSubject)
	require.Equal(t, "fresh@example.com", session.ResolvedEmail)
	require.Equal(t, "/dashboard", session.RedirectTo)
	require.Nil(t, session.TargetUserID)

	completion, ok := readCompletionResponse(session.LocalFlowState)
	require.True(t, ok)
	require.Equal(t, oauthPendingChoiceStep, completion["step"])
	require.Equal(t, "invitation_required", completion["error"])
	require.Equal(t, true, completion["invitation_required"])
	require.Equal(t, "fresh@example.com", completion["email"])
	require.Equal(t, "fresh@example.com", completion["resolved_email"])
	require.Equal(t, true, completion["create_account_allowed"])

	require.NotEmpty(t, findSetCookieValue(recorder.Result().Cookies(), oauthPendingSessionCookieName))
	require.NotEmpty(t, findSetCookieValue(recorder.Result().Cookies(), oauthPendingBrowserCookieName))
}

func TestEmailOAuthCallbackExistingEmailLogsInWhenInvitationEnabled(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandler(t, true)
	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("existing@example.com").
		SetUsername("existing").
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/google/callback", nil)

	handler.emailOAuthCallbackWithProfile(c, "google", config.EmailOAuthProviderConfig{
		Enabled:             true,
		ClientID:            "google-client",
		ClientSecret:        "google-secret",
		RedirectURL:         "https://app.example/api/v1/auth/oauth/google/callback",
		FrontendRedirectURL: "/auth/oauth/callback",
	}, "/auth/oauth/callback", "/dashboard", &emailOAuthProfile{
		Subject:       "google-123",
		Email:         "existing@example.com",
		EmailVerified: true,
		Username:      "existing",
	})

	require.Equal(t, http.StatusFound, recorder.Code)
	location := recorder.Header().Get("Location")
	require.Contains(t, location, "access_token=")
	require.Contains(t, location, "redirect=%252Fdashboard")

	sessionCount, err := client.PendingAuthSession.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, sessionCount)

	identityCount, err := client.AuthIdentity.Query().Where(
		authidentity.ProviderTypeEQ("google"),
		authidentity.ProviderSubjectEQ("google-123"),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, identityCount)
	_ = user
}

func TestEmailOAuthCallbackCreatesPasswordRegistrationSessionForNewEmail(t *testing.T) {
	affiliateRepo := newOAuthEmailAffiliateRepoStub(map[string]int64{"AFF123": 1001})
	handler, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyAffiliateEnabled: "true",
		},
		affiliateFactory: func(_ *dbent.Client, settingSvc *service.SettingService) *service.AffiliateService {
			return service.NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
		},
	})
	ctx := context.Background()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github/callback", nil)
	req.AddCookie(&http.Cookie{Name: emailOAuthAffiliateCookie, Value: encodeCookieValue("AFF123")})
	c.Request = req

	handler.emailOAuthCallbackWithProfile(c, "github", config.EmailOAuthProviderConfig{
		Enabled:             true,
		ClientID:            "github-client",
		ClientSecret:        "github-secret",
		RedirectURL:         "https://app.example/api/v1/auth/oauth/github/callback",
		FrontendRedirectURL: "/auth/oauth/callback",
	}, "/auth/oauth/callback", "/dashboard", &emailOAuthProfile{
		Subject:       "github-aff-user",
		Email:         "aff-user@example.com",
		EmailVerified: true,
		Username:      "aff-user",
	})

	require.Equal(t, http.StatusFound, recorder.Code)
	require.NotContains(t, recorder.Header().Get("Location"), "access_token=")
	userCount, err := client.User.Query().Where(dbuser.EmailEQ("aff-user@example.com")).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, userCount)
	require.Empty(t, affiliateRepo.ensureUserIDs)
	require.Empty(t, affiliateRepo.bindCalls)

	session, err := client.PendingAuthSession.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "aff-user@example.com", session.ResolvedEmail)
	require.Equal(t, "AFF123", pendingSessionStringValue(session.UpstreamIdentityClaims, "aff_code"))

	completion, ok := readCompletionResponse(session.LocalFlowState)
	require.True(t, ok)
	require.Equal(t, oauthPendingChoiceStep, completion["step"])
	require.Equal(t, "registration_completion_required", completion["error"])
	require.Equal(t, false, completion["invitation_required"])
	require.Equal(t, true, completion["create_account_allowed"])
	require.Equal(t, true, completion["force_email_on_signup"])
	require.Equal(t, "aff-user@example.com", completion["resolved_email"])
}

func TestEmailOAuthStartPreservesPromoCodeInPendingSession(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyGitHubOAuthEnabled:      "true",
			service.SettingKeyGitHubOAuthClientID:     "github-client",
			service.SettingKeyGitHubOAuthClientSecret: "github-secret",
			service.SettingKeyGitHubOAuthRedirectURL:  "https://app.example/api/v1/auth/oauth/github/callback",
		},
	})
	ctx := context.Background()

	startRecorder := httptest.NewRecorder()
	startCtx, _ := gin.CreateTestContext(startRecorder)
	startCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github/start?promo_code=WELCOME2024", nil)

	handler.GitHubOAuthStart(startCtx)

	require.Equal(t, http.StatusFound, startRecorder.Code)
	promoCookie := findCookie(startRecorder.Result().Cookies(), oauthPromoCodeCookieName)
	require.NotNil(t, promoCookie)
	require.Equal(t, "WELCOME2024", decodeCookieValueForTest(t, promoCookie.Value))

	callbackRecorder := httptest.NewRecorder()
	callbackCtx, _ := gin.CreateTestContext(callbackRecorder)
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github/callback", nil)
	callbackReq.AddCookie(promoCookie)
	callbackCtx.Request = callbackReq

	handler.emailOAuthCallbackWithProfile(callbackCtx, "github", config.EmailOAuthProviderConfig{
		FrontendRedirectURL: "/auth/oauth/callback",
	}, "/auth/oauth/callback", "/dashboard", &emailOAuthProfile{
		Subject:       "github-promo-user",
		Email:         "promo-user@example.com",
		EmailVerified: true,
		Username:      "promo-user",
	})

	require.Equal(t, http.StatusFound, callbackRecorder.Code)
	session, err := client.PendingAuthSession.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "WELCOME2024", pendingOAuthPromoCode(session))
}

func TestCompleteEmailOAuthRegistrationUsesAffiliateCodeFromPendingSession(t *testing.T) {
	affiliateRepo := newOAuthEmailAffiliateRepoStub(map[string]int64{"AFF456": 2002})
	handler, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		invitationEnabled: true,
		settingValues: map[string]string{
			service.SettingKeyAffiliateEnabled:            "true",
			service.SettingKeyAffiliateInviteeSignupBonus: "5",
		},
		affiliateFactory: func(_ *dbent.Client, settingSvc *service.SettingService) *service.AffiliateService {
			return service.NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
		},
	})
	ctx := context.Background()
	invitation, err := client.RedeemCode.Create().
		SetCode("INVITE456").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		SetValue(0).
		Save(ctx)
	require.NoError(t, err)

	session, err := client.PendingAuthSession.Create().
		SetSessionToken("email-oauth-aff-session-token").
		SetIntent(oauthIntentLogin).
		SetProviderType("google").
		SetProviderKey("google").
		SetProviderSubject("google-aff-user").
		SetResolvedEmail("pending-aff@example.com").
		SetRedirectTo("/dashboard").
		SetBrowserSessionKey("browser-aff-key").
		SetUpstreamIdentityClaims(map[string]any{
			"email":            "pending-aff@example.com",
			"email_verified":   true,
			"username":         "pending-aff",
			"provider":         "google",
			"provider_key":     "google",
			"provider_subject": "google-aff-user",
			"aff_code":         "AFF456",
		}).
		SetLocalFlowState(map[string]any{
			"step":  oauthPendingChoiceStep,
			"error": "invitation_required",
		}).
		SetExpiresAt(time.Now().UTC().Add(10 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google/complete-registration", strings.NewReader(`{"password":"secret-123","invitation_code":"INVITE456","email":"tampered@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: oauthPendingSessionCookieName, Value: encodeCookieValue(session.SessionToken)})
	req.AddCookie(&http.Cookie{Name: oauthPendingBrowserCookieName, Value: encodeCookieValue("browser-aff-key")})
	c.Request = req

	handler.completeEmailOAuthRegistration(c, "google")

	require.Equal(t, http.StatusOK, recorder.Code)
	user, err := client.User.Query().Where(dbuser.EmailEQ("pending-aff@example.com")).Only(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, user.PasswordHash)
	require.NotEqual(t, "secret-123", user.PasswordHash)
	tamperedCount, err := client.User.Query().Where(dbuser.EmailEQ("tampered@example.com")).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, tamperedCount)
	require.Equal(t, []oauthEmailAffiliateBindCall{{userID: user.ID, inviterID: 2002}}, affiliateRepo.bindCalls)
	require.Equal(t, []int64{user.ID}, affiliateRepo.signupUserIDs)
	require.Equal(t, []float64{5}, affiliateRepo.signupAmounts)
	// 重放相同 pending 会话不得再次发放。
	replay := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(replay)
	c2.Request = req.Clone(ctx)
	c2.Request.Body = io.NopCloser(strings.NewReader(`{"password":"secret-123","invitation_code":"INVITE456"}`))
	handler.completeEmailOAuthRegistration(c2, "google")
	require.NotEqual(t, http.StatusOK, replay.Code)
	require.Len(t, affiliateRepo.signupUserIDs, 1)
	storedInvitation, err := client.RedeemCode.Query().Where(redeemcode.IDEQ(invitation.ID)).Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, storedInvitation.UsedBy)
	require.Equal(t, user.ID, *storedInvitation.UsedBy)
}

func TestCompleteEmailOAuthRegistrationRequiresPassword(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandler(t, false)
	ctx := context.Background()

	session, err := client.PendingAuthSession.Create().
		SetSessionToken("email-oauth-password-session-token").
		SetIntent(oauthIntentLogin).
		SetProviderType("github").
		SetProviderKey("github").
		SetProviderSubject("github-password-user").
		SetResolvedEmail("password-required@example.com").
		SetRedirectTo("/dashboard").
		SetBrowserSessionKey("browser-password-key").
		SetUpstreamIdentityClaims(map[string]any{
			"email":            "password-required@example.com",
			"email_verified":   true,
			"username":         "password-required",
			"provider":         "github",
			"provider_key":     "github",
			"provider_subject": "github-password-user",
		}).
		SetLocalFlowState(map[string]any{
			"step":  oauthPendingChoiceStep,
			"error": "registration_completion_required",
		}).
		SetExpiresAt(time.Now().UTC().Add(10 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/github/complete-registration", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: oauthPendingSessionCookieName, Value: encodeCookieValue(session.SessionToken)})
	req.AddCookie(&http.Cookie{Name: oauthPendingBrowserCookieName, Value: encodeCookieValue("browser-password-key")})
	c.Request = req

	handler.completeEmailOAuthRegistration(c, "github")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	userCount, err := client.User.Query().Where(dbuser.EmailEQ("password-required@example.com")).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, userCount)
}

func TestParseGitHubOAuthProfileRejectsPublicEmailWhenEmailsEndpointFails(t *testing.T) {
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing scope", http.StatusForbidden)
	}))
	t.Cleanup(emailServer.Close)

	profile, err := parseGitHubOAuthProfile(context.Background(), config.EmailOAuthProviderConfig{
		EmailsURL: emailServer.URL,
	}, &emailOAuthTokenResponse{AccessToken: "token"}, `{"id":123,"login":"octo","email":"public@example.com"}`)

	require.Error(t, err)
	require.Nil(t, profile)
	require.Contains(t, err.Error(), "github emails endpoint status 403")
}

type oauthEmailAffiliateBindCall struct {
	userID    int64
	inviterID int64
}

type oauthEmailAffiliateRepoStub struct {
	codeOwners    map[string]int64
	ensureUserIDs []int64
	bindCalls     []oauthEmailAffiliateBindCall
	signupUserIDs []int64
	signupAmounts []float64
}

func newOAuthEmailAffiliateRepoStub(codeOwners map[string]int64) *oauthEmailAffiliateRepoStub {
	return &oauthEmailAffiliateRepoStub{codeOwners: codeOwners}
}

func (r *oauthEmailAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*service.AffiliateSummary, error) {
	r.ensureUserIDs = append(r.ensureUserIDs, userID)
	return &service.AffiliateSummary{UserID: userID, AffCode: "SELF"}, nil
}

func (r *oauthEmailAffiliateRepoStub) GetAffiliateByCode(_ context.Context, code string) (*service.AffiliateSummary, error) {
	userID, ok := r.codeOwners[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return nil, service.ErrAffiliateProfileNotFound
	}
	return &service.AffiliateSummary{UserID: userID, AffCode: strings.ToUpper(strings.TrimSpace(code))}, nil
}

func (r *oauthEmailAffiliateRepoStub) BindInviter(_ context.Context, userID, inviterID int64, _ service.AffiliateInviterBinding) (*service.AffiliateInviterBindingResult, bool, error) {
	r.bindCalls = append(r.bindCalls, oauthEmailAffiliateBindCall{userID: userID, inviterID: inviterID})
	return &service.AffiliateInviterBindingResult{InviteeID: userID, InviterID: inviterID}, true, nil
}

func (r *oauthEmailAffiliateRepoStub) AccrueQuota(context.Context, int64, int64, float64, int, service.AffiliateAccrueSource, string) (bool, error) {
	panic("unexpected AccrueQuota call")
}

func (r *oauthEmailAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64, []string) (float64, error) {
	panic("unexpected GetAccruedRebateFromInvitee call")
}

func (r *oauthEmailAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (r *oauthEmailAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (r *oauthEmailAffiliateRepoStub) WithdrawQuota(context.Context, int64, float64, string) (*service.AffiliateWithdrawResult, error) {
	panic("unexpected WithdrawQuota call")
}

func (r *oauthEmailAffiliateRepoStub) ListInvitees(context.Context, int64, service.AffiliateInviteeQuery) ([]service.AffiliateInvitee, int64, error) {
	panic("unexpected ListInvitees call")
}

func (r *oauthEmailAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (r *oauthEmailAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (r *oauthEmailAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (r *oauthEmailAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (r *oauthEmailAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, service.AffiliateAdminFilter) ([]service.AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (r *oauthEmailAffiliateRepoStub) CountQualifiedInvitees(context.Context, int64, bool) (int, error) {
	panic("unexpected CountQualifiedInvitees call")
}

func (r *oauthEmailAffiliateRepoStub) PromoteAgentLevelAuto(context.Context, int64, int) (bool, error) {
	panic("unexpected PromoteAgentLevelAuto call")
}

func (r *oauthEmailAffiliateRepoStub) SetAgentLevelManual(context.Context, []int64, int, int64) error {
	panic("unexpected SetAgentLevelManual call")
}

func (r *oauthEmailAffiliateRepoStub) SetAgentLevelLocked(context.Context, []int64, bool) error {
	panic("unexpected SetAgentLevelLocked call")
}

func (r *oauthEmailAffiliateRepoStub) ListInviterIDs(context.Context, int64, int) ([]int64, error) {
	panic("unexpected ListInviterIDs call")
}

func (r *oauthEmailAffiliateRepoStub) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// GetAffiliateNodes 绑定时解析归属链会读取邀请人；桩里所有用户都是仍存在的普通用户。
func (r *oauthEmailAffiliateRepoStub) GetAffiliateNodes(_ context.Context, userIDs []int64) (map[int64]service.AffiliateNode, error) {
	out := make(map[int64]service.AffiliateNode, len(userIDs))
	for _, id := range userIDs {
		out[id] = service.AffiliateNode{UserID: id, AgentLevel: service.AffiliateAgentLevelDefault, Active: true}
	}
	return out, nil
}

func (r *oauthEmailAffiliateRepoStub) CreditInviteeBonus(context.Context, int64, float64, *int64) (bool, error) {
	panic("unexpected CreditInviteeBonus call")
}

func (r *oauthEmailAffiliateRepoStub) HasRewardForSource(context.Context, int64, string) (bool, error) {
	panic("unexpected HasRewardForSource call")
}

func (r *oauthEmailAffiliateRepoStub) CountOwnedCustomers(context.Context, int64) (int, error) {
	panic("unexpected CountOwnedCustomers call")
}

func (r *oauthEmailAffiliateRepoStub) SumRewardTotals(context.Context, int64) (service.AffiliateRewardTotals, error) {
	panic("unexpected SumRewardTotals call")
}

func (r *oauthEmailAffiliateRepoStub) GetAffiliateForUpdate(context.Context, int64) (*service.AffiliateSummary, error) {
	panic("unexpected GetAffiliateForUpdate call")
}

func (r *oauthEmailAffiliateRepoStub) SetAgentLevelByAgent(context.Context, int64, int, int64, bool) error {
	panic("unexpected SetAgentLevelByAgent call")
}

func (r *oauthEmailAffiliateRepoStub) SetAgentOwnerByAdmin(context.Context, int64, *int64, int64) error {
	panic("unexpected SetAgentOwnerByAdmin call")
}

func (r *oauthEmailAffiliateRepoStub) CountAgentAppointments(context.Context, int64) (int, error) {
	panic("unexpected CountAgentAppointments call")
}

func (r *oauthEmailAffiliateRepoStub) ListAgentCustomers(context.Context, int64, int, int) ([]service.AffiliateAgentCustomer, int64, error) {
	panic("unexpected ListAgentCustomers call")
}

func (r *oauthEmailAffiliateRepoStub) ListUsersWithPendingConsumption(context.Context, time.Duration, int) ([]int64, error) {
	panic("unexpected ListUsersWithPendingConsumption call")
}

func (r *oauthEmailAffiliateRepoStub) LockPendingConsumption(context.Context, int64, time.Duration) (*service.AffiliatePendingConsumption, error) {
	panic("unexpected LockPendingConsumption call")
}

func (r *oauthEmailAffiliateRepoStub) AdvanceConsumptionWatermark(context.Context, int64, time.Time, float64) error {
	panic("unexpected AdvanceConsumptionWatermark call")
}

func (r *oauthEmailAffiliateRepoStub) ListAffiliateInviteRecords(context.Context, service.AffiliateRecordFilter) ([]service.AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (r *oauthEmailAffiliateRepoStub) ListAffiliateRebateRecords(context.Context, service.AffiliateRecordFilter) ([]service.AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (r *oauthEmailAffiliateRepoStub) ListAffiliateTransferRecords(context.Context, service.AffiliateRecordFilter) ([]service.AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (r *oauthEmailAffiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*service.AffiliateUserOverview, error) {
	panic("unexpected GetAffiliateUserOverview call")
}

func findSetCookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie != nil && strings.EqualFold(cookie.Name, name) && cookie.MaxAge >= 0 {
			return cookie.Value
		}
	}
	return ""
}

func (r *oauthEmailAffiliateRepoStub) CreditInviteeSignupBonus(_ context.Context, userID int64, amount float64) (bool, error) {
	r.signupUserIDs = append(r.signupUserIDs, userID)
	r.signupAmounts = append(r.signupAmounts, amount)
	return true, nil
}
