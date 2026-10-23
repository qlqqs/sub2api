//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// --- 分组模型倍率 ---

type groupModelRateHandlerRepo struct {
	mu     sync.Mutex
	rules  map[int64][]service.GroupModelRateRule
	groups map[int64]bool
}

func (r *groupModelRateHandlerRepo) ListAll(context.Context) (map[int64][]service.GroupModelRateRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[int64][]service.GroupModelRateRule{}
	for id, rules := range r.rules {
		if len(rules) > 0 {
			out[id] = append([]service.GroupModelRateRule(nil), rules...)
		}
	}
	return out, nil
}

func (r *groupModelRateHandlerRepo) ListByGroup(_ context.Context, groupID int64) ([]service.GroupModelRateRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.GroupModelRateRule{}, r.rules[groupID]...), nil
}

func (r *groupModelRateHandlerRepo) ReplaceForGroup(_ context.Context, groupID int64, rules []service.GroupModelRateRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[groupID] = append([]service.GroupModelRateRule(nil), rules...)
	return nil
}

func (r *groupModelRateHandlerRepo) CopyGroup(context.Context, int64, int64) error { return nil }

func (r *groupModelRateHandlerRepo) GroupExists(_ context.Context, groupID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.groups[groupID], nil
}

func newGroupModelRateTestRouter(cfg *config.Config) (*gin.Engine, *groupModelRateHandlerRepo) {
	gin.SetMode(gin.TestMode)
	repo := &groupModelRateHandlerRepo{
		rules:  map[int64][]service.GroupModelRateRule{2: {{Model: "gpt-5.5", RateMultiplier: 0.8}}},
		groups: map[int64]bool{1: true, 2: true},
	}
	h := NewGroupModelRateHandler(service.NewGroupModelRateService(repo), cfg)
	r := gin.New()
	r.GET("/api/v1/admin/groups/model-rates", h.ListAll)
	r.GET("/api/v1/admin/groups/:id/model-rates", h.Get)
	r.PUT("/api/v1/admin/groups/:id/model-rates", h.Replace)
	return r, repo
}

type gmrEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Reason  string          `json:"reason"`
	Data    json.RawMessage `json:"data"`
}

func gmrDoJSON(t *testing.T, r http.Handler, method, path, body string) (*httptest.ResponseRecorder, gmrEnvelope) {
	t.Helper()
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var env gmrEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), rec.Body.String())
	return rec, env
}

func TestGroupModelRateHandler_ReplaceNormalizesAndLists(t *testing.T) {
	r, repo := newGroupModelRateTestRouter(&config.Config{})

	rec, env := gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/groups/1/model-rates",
		`{"rules":[{"model":"Claude-Opus-*","rate_multiplier":1},{"model":"claude-sonnet-4.5","rate_multiplier":0.75}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var saved struct {
		Rules []groupModelRateRuleDTO `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &saved))
	require.Equal(t, []groupModelRateRuleDTO{
		{Model: "claude-opus-*", RateMultiplier: 1},
		{Model: "claude-sonnet-4-5", RateMultiplier: 0.75},
	}, saved.Rules, "返回归一化后的模型名，按 model 排序")
	require.Len(t, repo.rules[1], 2)

	rec, env = gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/1/model-rates", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Rules []groupModelRateRuleDTO `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &got))
	require.Equal(t, saved.Rules, got.Rules)

	rec, env = gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/model-rates", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var all map[string][]groupModelRateRuleDTO
	require.NoError(t, json.Unmarshal(env.Data, &all))
	require.Len(t, all, 2, "只含有规则的分组，键为分组 id")
	require.Equal(t, saved.Rules, all["1"])
	require.Equal(t, []groupModelRateRuleDTO{{Model: "gpt-5.5", RateMultiplier: 0.8}}, all["2"])

	// 空数组 = 清空；空分组 GET 返回空数组而不是 null。
	rec, env = gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/groups/1/model-rates", `{"rules":[]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"rules":[]}`, string(env.Data))
}

func TestGroupModelRateHandler_ValidationAndNotFound(t *testing.T) {
	r, repo := newGroupModelRateTestRouter(&config.Config{})

	cases := []struct {
		name string
		body string
		msg  string
	}{
		{"wildcard_not_at_end", `{"rules":[{"model":"*-opus","rate_multiplier":1}]}`, "只能放在末尾"},
		{"duplicate", `{"rules":[{"model":"a","rate_multiplier":1},{"model":"A","rate_multiplier":2}]}`, "重复"},
		{"zero_rate", `{"rules":[{"model":"a","rate_multiplier":0}]}`, "须大于 0"},
		{"negative_rate", `{"rules":[{"model":"a","rate_multiplier":-0.5}]}`, "须大于 0"},
		{"missing_rules", `{}`, "rules is required"},
		{"bad_json", `{"rules":"x"}`, "Invalid request"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, env := gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/groups/1/model-rates", tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, http.StatusBadRequest, env.Code)
			require.Contains(t, env.Message, tc.msg)
		})
	}
	require.Empty(t, repo.rules[1], "校验失败不写库")

	rec, env := gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/groups/999/model-rates", `{"rules":[]}`)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "GROUP_NOT_FOUND", env.Reason)
	rec, _ = gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/999/model-rates", "")
	require.Equal(t, http.StatusNotFound, rec.Code)
	rec, _ = gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/abc/model-rates", "")
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGroupModelRateHandler_SimpleMode(t *testing.T) {
	r, _ := newGroupModelRateTestRouter(&config.Config{RunMode: config.RunModeSimple})

	rec, env := gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/model-rates", "")
	require.Equal(t, http.StatusOK, rec.Code, "列表页提示接口在简易模式返回空集合")
	require.JSONEq(t, `{}`, string(env.Data))

	rec, _ = gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/groups/1/model-rates", `{"rules":[]}`)
	require.Equal(t, http.StatusForbidden, rec.Code)
	rec, _ = gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/groups/1/model-rates", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// --- 美元汇率 ---

type fxHandlerSettingRepo struct {
	service.SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func (r *fxHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]string{}
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (r *fxHandlerSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, v := range settings {
		r.values[k] = v
	}
	return nil
}

type fxHandlerFetcher struct{ body string }

func (f fxHandlerFetcher) FetchFxRateJSON(context.Context, string) ([]byte, error) {
	if f.body == "" {
		return nil, errors.New("HTTP 503")
	}
	return []byte(f.body), nil
}

func newFxRateTestRouter(fetcher service.FxRateFetcher) (*gin.Engine, *fxHandlerSettingRepo) {
	gin.SetMode(gin.TestMode)
	repo := &fxHandlerSettingRepo{values: map[string]string{}}
	cfg := &config.Config{}
	cfg.FX.SourceURLs = []string{"http://fx.test/latest"}
	h := NewFxRateHandler(service.NewFxRateService(repo, fetcher, cfg))
	r := gin.New()
	r.GET("/api/v1/admin/settings/fx-rate", h.Get)
	r.PUT("/api/v1/admin/settings/fx-rate", h.Update)
	r.POST("/api/v1/admin/settings/fx-rate/refresh", h.Refresh)
	return r, repo
}

func TestFxRateHandler_GetUpdateRefresh(t *testing.T) {
	r, repo := newFxRateTestRouter(fxHandlerFetcher{body: `{"rates":{"CNY":7.1}}`})

	rec, env := gmrDoJSON(t, r, http.MethodGet, "/api/v1/admin/settings/fx-rate", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var view map[string]any
	require.NoError(t, json.Unmarshal(env.Data, &view))
	for _, key := range []string{"auto_enabled", "default_rate", "effective_rate", "effective_from", "fetched_rate", "fetched_at", "source", "stale", "last_error", "last_attempt_at"} {
		require.Contains(t, view, key)
	}
	require.Equal(t, true, view["auto_enabled"])
	require.InDelta(t, 6.8, view["effective_rate"].(float64), 1e-12)
	require.Equal(t, "default", view["effective_from"])
	require.Nil(t, view["fetched_rate"])
	require.Nil(t, view["fetched_at"])
	require.Nil(t, view["last_attempt_at"])

	rec, env = gmrDoJSON(t, r, http.MethodPost, "/api/v1/admin/settings/fx-rate/refresh", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var refreshed struct {
		OK    bool           `json:"ok"`
		Error string         `json:"error"`
		Rate  map[string]any `json:"rate"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &refreshed))
	require.True(t, refreshed.OK)
	require.Empty(t, refreshed.Error)
	require.InDelta(t, 7.1, refreshed.Rate["effective_rate"].(float64), 1e-12)
	require.Equal(t, "fetched", refreshed.Rate["effective_from"])
	require.Equal(t, "fx.test", refreshed.Rate["source"])
	require.NotNil(t, refreshed.Rate["fetched_at"])

	rec, _ = gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/settings/fx-rate", `{"auto_enabled":true,"default_rate":4.5}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec, _ = gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/settings/fx-rate", `{"default_rate":7}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec, env = gmrDoJSON(t, r, http.MethodPut, "/api/v1/admin/settings/fx-rate", `{"auto_enabled":false,"default_rate":7.3}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(env.Data, &view))
	require.Equal(t, false, view["auto_enabled"])
	require.InDelta(t, 7.3, view["effective_rate"].(float64), 1e-12)
	require.Equal(t, "false", repo.values[service.SettingKeyFxUSDCNYAutoEnabled])
}

func TestFxRateHandler_RefreshFailureIsStill200(t *testing.T) {
	r, _ := newFxRateTestRouter(fxHandlerFetcher{})
	rec, env := gmrDoJSON(t, r, http.MethodPost, "/api/v1/admin/settings/fx-rate/refresh", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var refreshed struct {
		OK    bool           `json:"ok"`
		Error string         `json:"error"`
		Rate  map[string]any `json:"rate"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &refreshed))
	require.False(t, refreshed.OK)
	require.Contains(t, refreshed.Error, "HTTP 503")
	require.InDelta(t, 6.8, refreshed.Rate["effective_rate"].(float64), 1e-12)
	require.Equal(t, true, refreshed.Rate["stale"])
	require.Contains(t, refreshed.Rate["last_error"].(string), "HTTP 503")
}
