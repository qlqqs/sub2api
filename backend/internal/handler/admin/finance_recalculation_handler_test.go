//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type financeRecalculationHTTPStub struct {
	batch        service.FinanceRecalculationBatch
	item         service.FinanceRecalculationItem
	err          error
	calls        int
	actor        int64
	acknowledged bool
	fingerprint  string
	create       service.FinanceRecalculationCreate
}

func (s *financeRecalculationHTTPStub) Create(_ context.Context, in service.FinanceRecalculationCreate) (*service.FinanceRecalculationBatch, error) {
	s.calls++
	s.create = in
	return &s.batch, s.err
}
func (s *financeRecalculationHTTPStub) List(context.Context, int, int) ([]service.FinanceRecalculationBatch, int64, error) {
	s.calls++
	return []service.FinanceRecalculationBatch{s.batch}, 1, s.err
}
func (s *financeRecalculationHTTPStub) Get(context.Context, int64) (*service.FinanceRecalculationBatch, error) {
	s.calls++
	return &s.batch, s.err
}
func (s *financeRecalculationHTTPStub) Items(context.Context, int64, int, int) ([]service.FinanceRecalculationItem, int64, error) {
	s.calls++
	return []service.FinanceRecalculationItem{s.item}, 1, s.err
}
func (s *financeRecalculationHTTPStub) Confirm(_ context.Context, _ int64, actor int64, fingerprint string, ack bool) (*service.FinanceRecalculationBatch, error) {
	s.calls++
	s.actor = actor
	s.fingerprint = fingerprint
	s.acknowledged = ack
	return &s.batch, s.err
}
func (s *financeRecalculationHTTPStub) RetrySync(_ context.Context, _ int64, actor int64) (*service.FinanceRecalculationBatch, error) {
	s.calls++
	s.actor = actor
	return &s.batch, s.err
}
func (s *financeRecalculationHTTPStub) Cancel(_ context.Context, _ int64, actor int64) (*service.FinanceRecalculationBatch, error) {
	s.calls++
	s.actor = actor
	return &s.batch, s.err
}
func financeRecalculationHTTPRouter(s *financeRecalculationHTTPStub, role string, actor int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if role != "" {
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
		if actor > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor})
		}
		c.Next()
	})
	h := &FinanceRecalculationHandler{s: s}
	root := router.Group("/api/v1/admin/finance/recalculations")
	root.POST("", h.Create)
	root.GET("", h.List)
	root.GET("/:id", h.Get)
	root.GET("/:id/items", h.Items)
	root.POST("/:id/confirm", h.Confirm)
	root.POST("/:id/retry-sync", h.RetrySync)
	root.POST("/:id/cancel", h.Cancel)
	return router
}
func financeRecalculationHTTPRequest(router *gin.Engine, method, suffix, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/v1/admin/finance/recalculations"+suffix, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	return rec
}
func TestFinanceRecalculationHTTPPublicDTO(t *testing.T) {
	s := &financeRecalculationHTTPStub{batch: service.FinanceRecalculationBatch{ID: 4, FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}, Status: service.FinanceRecalculationFailed, ActorID: 8, ConfigFingerprint: "fp", LeaseToken: "secret-lease-token", ErrorCode: "sync_failed", ErrorMessage: "secret database credentials", FailedPhase: service.FinanceRecalculationSyncing, OldTotal: "1.0000000001", NewTotal: "2.0000000001", Delta: "1", Config: service.FinanceRecalculationConfig{Usage: []service.FinanceUsageCostSnapshot{{AccountID: 1, Model: "secret internal model", Channel: &service.FinanceCostChannelSnapshot{Rules: []service.AccountStatsPricingRule{{Name: "secret-config"}}}}}}}, item: service.FinanceRecalculationItem{ID: 9, Status: service.FinanceRecalculationSkipped, Reason: "missing_current_config", Candidate: service.FinanceRecalculationCandidate{Raw: json.RawMessage(`{"secret":"candidate"}`)}, UsageResult: &service.FinanceUsageCostResult{RuleFingerprint: "secret-rule"}}}
	router := financeRecalculationHTTPRouter(s, service.RoleAdmin, 8)
	for _, path := range []string{"/4", ""} {
		rec := financeRecalculationHTTPRequest(router, http.MethodGet, path, "")
		require.Equal(t, 200, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"amount_unit":"account_cost"`)
		require.Contains(t, rec.Body.String(), `"failed_phase":"syncing"`)
		require.Contains(t, rec.Body.String(), `"snapshot_summary":[`)
		for _, forbidden := range []string{"secret", "lease_token", "candidate", "usage_result", "config_snapshot", "model_rates"} {
			require.NotContains(t, rec.Body.String(), forbidden)
		}
	}
	rec := financeRecalculationHTTPRequest(router, http.MethodGet, "/4/items", "")
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"items":[`)
	require.Contains(t, rec.Body.String(), `"old_cost":null`)
	require.Contains(t, rec.Body.String(), `"new_cost":null`)
	require.Contains(t, rec.Body.String(), `"delta":null`)
	require.NotContains(t, rec.Body.String(), "secret")
	s.batch.Kind = service.FinanceRecalculationPaymentFee
	rec = financeRecalculationHTTPRequest(router, http.MethodGet, "/4", "")
	require.Contains(t, rec.Body.String(), `"amount_unit":"CNY"`)
	s.batch.ErrorCode = "secret-error"
	s.batch.FailedPhase = "secret-phase"
	s.item.Reason = "secret = db://password"
	rec = financeRecalculationHTTPRequest(router, http.MethodGet, "/4", "")
	require.NotContains(t, rec.Body.String(), "secret")
	require.Contains(t, rec.Body.String(), `"error_code":"recalculation_failed"`)
	rec = financeRecalculationHTTPRequest(router, http.MethodGet, "/4/items", "")
	require.NotContains(t, rec.Body.String(), "secret")
	require.Contains(t, rec.Body.String(), `"reason":"unknown"`)
}
func TestFinanceRecalculationHTTPAuthenticatedActorAndAck(t *testing.T) {
	s := &financeRecalculationHTTPStub{}
	router := financeRecalculationHTTPRouter(s, service.RoleAdmin, 88)
	rec := financeRecalculationHTTPRequest(router, http.MethodPost, "", `{"kind":"usage_cost","start":"2026-10-25","end":"2026-10-26","idempotency_key":"request","actor_id":999}`)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, int64(88), s.create.ActorID)
	require.Equal(t, "request", s.create.IdempotencyKey)
	for _, body := range []string{`{"config_fingerprint":"fp"}`, `{"config_fingerprint":"fp","acknowledged":false}`, `{"acknowledged":true}`} {
		before := s.calls
		rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/confirm", body)
		require.Equal(t, 400, rec.Code)
		require.Equal(t, before, s.calls)
	}
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/confirm", `{"config_fingerprint":"fp","acknowledged":true,"actor_id":999}`)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, int64(88), s.actor)
	require.True(t, s.acknowledged)
	require.Equal(t, "fp", s.fingerprint)
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/retry-sync", "")
	require.Equal(t, 200, rec.Code)
	require.Equal(t, int64(88), s.actor)
	s.actor = 0
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/cancel", `{"actor_id":999}`)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, int64(88), s.actor)
	s.err = service.ErrFinanceRecalculationActor
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/confirm", `{"config_fingerprint":"fp","acknowledged":true}`)
	require.Equal(t, 403, rec.Code)
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/retry-sync", "")
	require.Equal(t, 403, rec.Code)
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/cancel", "")
	require.Equal(t, 403, rec.Code)
}
func TestFinanceRecalculationHTTPPermissionsValidationAndSanitization(t *testing.T) {
	routes := []struct{ method, suffix, body string }{{"GET", "", ""}, {"GET", "/4", ""}, {"GET", "/4/items", ""}, {"POST", "", `{"idempotency_key":"r"}`}, {"POST", "/4/confirm", `{"config_fingerprint":"fp","acknowledged":true}`}, {"POST", "/4/retry-sync", ""}, {"POST", "/4/cancel", ""}}
	for _, role := range []string{"", service.RoleUser} {
		for _, route := range routes {
			s := &financeRecalculationHTTPStub{}
			router := financeRecalculationHTTPRouter(s, role, 9)
			rec := financeRecalculationHTTPRequest(router, route.method, route.suffix, route.body)
			want := 403
			if role == "" {
				want = 401
			}
			require.Equal(t, want, rec.Code)
			require.Zero(t, s.calls)
		}
	}
	s := &financeRecalculationHTTPStub{}
	router := financeRecalculationHTTPRouter(s, service.RoleAdmin, 9)
	for _, path := range []string{"/0", "/bad", "?page=0", "?page_size=201", "/4/items?page=-1"} {
		rec := financeRecalculationHTTPRequest(router, http.MethodGet, path, "")
		require.Equal(t, 400, rec.Code)
	}
	require.Zero(t, s.calls)
	s.err = errors.New("secret postgres password")
	rec := financeRecalculationHTTPRequest(router, http.MethodGet, "/4", "")
	require.Equal(t, 500, rec.Code)
	require.NotContains(t, rec.Body.String(), "password")
	s.err = service.ErrFinanceRecalculationRuleChanged
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/confirm", `{"config_fingerprint":"fp","acknowledged":true}`)
	require.Equal(t, 409, rec.Code)
	// No undo route is registered.
	rec = financeRecalculationHTTPRequest(router, http.MethodPost, "/4/undo", "")
	require.Equal(t, 404, rec.Code)
}

func TestFinanceRecalculationHTTPSnapshotCoverageWarning(t *testing.T) {
	early := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	late := early.Add(24 * time.Hour)
	b := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}, ConfigFingerprint: "fp", Skipped: 3, Config: service.FinanceRecalculationConfig{Usage: []service.FinanceUsageCostSnapshot{{CoverageFrom: &late}, {CoverageFrom: &early}}}}
	public := financeRecalculationBatchToResponse(b)
	summary := strings.Join(public.SnapshotSummary, "\n")
	require.Contains(t, summary, "2026-10-26T08:00:00+08:00")
	require.NotContains(t, summary, "2026-10-27")
	require.Contains(t, summary, "Legacy historical aggregate coverage is not certified")
	require.Contains(t, summary, "skipped, not repaired")
	require.Contains(t, summary, "3 requests were skipped")
	b.Config.Usage = nil
	summary = strings.Join(financeRecalculationBatchToResponse(b).SnapshotSummary, "\n")
	require.NotContains(t, summary, "Earliest certified")
	require.Contains(t, summary, "not certified")
}

func TestFinanceRecalculationHTTPRetainedBaselineExplicitAndSanitized(t *testing.T) {
	s := &financeRecalculationHTTPStub{}
	router := financeRecalculationHTTPRouter(s, service.RoleAdmin, 88)
	rec := financeRecalculationHTTPRequest(router, http.MethodPost, "", `{"kind":"usage_cost","start":"2026-10-25","end":"2026-10-26","idempotency_key":"baseline","accept_retained_baseline":true}`)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.True(t, s.create.AcceptRetainedBaseline)
	s.batch.FinanceRecalculationFilter = s.create.FinanceRecalculationFilter
	s.batch.ConfigFingerprint = "baseline-fingerprint"
	s.batch.Config.RetainedBaseline = &service.FinanceRetainedBaseline{Version: 1, Timezone: "Asia/Shanghai", Days: []service.FinanceRetainedBaselineDay{{Day: "2026-10-25", SourceDigest: "internal-source-digest"}}}
	rec = financeRecalculationHTTPRequest(router, http.MethodGet, "/4", "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"accept_retained_baseline":true`)
	require.Contains(t, rec.Body.String(), "Accepted retained-detail baseline: 1 full days")
	require.Contains(t, rec.Body.String(), "not proof of historical completeness")
	require.NotContains(t, rec.Body.String(), "internal-source-digest")
	require.NotContains(t, rec.Body.String(), "requests without proven coverage are skipped")
}
