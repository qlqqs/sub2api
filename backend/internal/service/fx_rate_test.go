//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// fxSettingRepoStub 是内存设置表。
type fxSettingRepoStub struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
	err    error
}

func newFxSettingRepoStub(values map[string]string) *fxSettingRepoStub {
	repo := &fxSettingRepoStub{values: map[string]string{}}
	for k, v := range values {
		repo.values[k] = v
	}
	return repo
}

func (r *fxSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (r *fxSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	for k, v := range settings {
		r.values[k] = v
	}
	return nil
}

func (r *fxSettingRepoStub) get(key string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.values[key]
}

// fxFetcherStub 按 URL 返回预设响应。
type fxFetcherStub struct {
	mu        sync.Mutex
	responses map[string]string
	errs      map[string]error
	calls     []string
	called    chan string
}

func (f *fxFetcherStub) FetchFxRateJSON(_ context.Context, url string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, url)
	called := f.called
	err := f.errs[url]
	body, ok := f.responses[url]
	f.mu.Unlock()
	if called != nil {
		select {
		case called <- url:
		default:
		}
	}
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("HTTP 404")
	}
	return []byte(body), nil
}

const (
	fxTestSourceA = "http://fx-a.test/latest"
	fxTestSourceB = "http://fx-b.test/usd.json"
)

func newFxTestService(settings map[string]string, fetcher FxRateFetcher) (*FxRateService, *fxSettingRepoStub) {
	repo := newFxSettingRepoStub(settings)
	cfg := &config.Config{}
	cfg.FX.SourceURLs = []string{fxTestSourceA + ", " + fxTestSourceB}
	svc := NewFxRateService(repo, fetcher, cfg)
	return svc, repo
}

func TestParseFxRateResponse(t *testing.T) {
	rate, err := parseFxRateResponse([]byte(`{"amount":1.0,"base":"USD","date":"2026-10-23","rates":{"CNY":7.1234}}`))
	require.NoError(t, err)
	require.InDelta(t, 7.1234, rate, 1e-12)

	rate, err = parseFxRateResponse([]byte(`{"date":"2026-10-23","usd":{"cny":7.05,"eur":0.92}}`))
	require.NoError(t, err)
	require.InDelta(t, 7.05, rate, 1e-12)

	_, err = parseFxRateResponse([]byte(`{"rates":{"EUR":0.9}}`))
	require.ErrorContains(t, err, "unrecognized response format")
	_, err = parseFxRateResponse([]byte(`{"usd":{"cny":"seven"}}`))
	require.ErrorContains(t, err, "not a number")
	_, err = parseFxRateResponse([]byte(`not json`))
	require.ErrorContains(t, err, "invalid json")
}

func TestValidateFetchedFxRate(t *testing.T) {
	require.NoError(t, validateFetchedFxRate(7.1, nil))
	require.NoError(t, validateFetchedFxRate(5, nil))
	require.NoError(t, validateFetchedFxRate(9, nil))
	require.Error(t, validateFetchedFxRate(4.99, nil))
	require.Error(t, validateFetchedFxRate(9.01, nil))
	prev := 7.0
	require.NoError(t, validateFetchedFxRate(7.7, &prev), "恰好 10% 允许")
	require.Error(t, validateFetchedFxRate(7.71, &prev), "超过 10% 丢弃")
	require.Error(t, validateFetchedFxRate(6.29, &prev))
}

func TestResolveFxRateSources(t *testing.T) {
	sources := resolveFxRateSources(nil)
	require.Len(t, sources, 3)
	require.Equal(t, "api.frankfurter.dev", sources[0].name)
	require.Equal(t, "cdn.jsdelivr.net", sources[1].name)
	require.Equal(t, "latest.currency-api.pages.dev", sources[2].name)

	sources = resolveFxRateSources([]string{"http://upstream-mock:8998/fx/a http://upstream-mock:8998/fx/b,http://upstream-mock:8998/fx/a", " "})
	require.Len(t, sources, 2, "逗号 / 空白分隔，去重")
	require.Equal(t, "http://upstream-mock:8998/fx/a", sources[0].url)
	require.Equal(t, "upstream-mock:8998", sources[0].name)
	require.Equal(t, "http://upstream-mock:8998/fx/b", sources[1].url)
}

func TestResolveEffectiveUSDCNYRate(t *testing.T) {
	rate, from := resolveEffectiveUSDCNYRate(map[string]string{})
	require.InDelta(t, DefaultUSDCNYRate, rate, 1e-12, "初始默认 6.8")
	require.Equal(t, FxRateEffectiveFromDefault, from)

	rate, from = resolveEffectiveUSDCNYRate(map[string]string{SettingKeyFxUSDCNYFetched: "7.12"})
	require.InDelta(t, 7.12, rate, 1e-12, "自动更新默认开启，取获取值")
	require.Equal(t, FxRateEffectiveFromFetched, from)

	rate, from = resolveEffectiveUSDCNYRate(map[string]string{
		SettingKeyFxUSDCNYAutoEnabled: "false", SettingKeyFxUSDCNYFetched: "7.12", SettingKeyFxUSDCNYDefault: "7.3",
	})
	require.InDelta(t, 7.3, rate, 1e-12, "关闭自动更新用默认值")
	require.Equal(t, FxRateEffectiveFromDefault, from)

	rate, _ = resolveEffectiveUSDCNYRate(map[string]string{SettingKeyFxUSDCNYDefault: "garbage"})
	require.InDelta(t, DefaultUSDCNYRate, rate, 1e-12)
}

func TestFxRateRefresh_FirstSuccessfulSourceWins(t *testing.T) {
	fetcher := &fxFetcherStub{
		errs:      map[string]error{fxTestSourceA: errors.New("connection refused")},
		responses: map[string]string{fxTestSourceB: `{"usd":{"cny":7.0512}}`},
	}
	svc, repo := newFxTestService(nil, fetcher)
	fixedNow := time.Date(2026, 10, 23, 4, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }
	changed := 0
	svc.onChange = func() { changed++ }

	ok, errMsg, view, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, errMsg)
	require.Equal(t, []string{fxTestSourceA, fxTestSourceB}, fetcher.calls)
	require.Equal(t, "7.0512", repo.get(SettingKeyFxUSDCNYFetched))
	require.Equal(t, "fx-b.test", repo.get(SettingKeyFxUSDCNYSource))
	require.Equal(t, "2026-10-23T04:00:00Z", repo.get(SettingKeyFxUSDCNYFetchedAt))
	require.Equal(t, 1, changed, "汇率变化后让公开设置缓存失效")

	require.InDelta(t, 7.0512, view.EffectiveRate, 1e-12)
	require.Equal(t, FxRateEffectiveFromFetched, view.EffectiveFrom)
	require.NotNil(t, view.FetchedRate)
	require.Equal(t, "fx-b.test", view.Source)
	require.False(t, view.Stale)
	require.Empty(t, view.LastError)
	require.NotNil(t, view.LastAttemptAt)
}

func TestFxRateRefresh_DiscardsAbnormalValues(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":12.5}}`, // 超出 [5, 9]
		fxTestSourceB: `{"usd":{"cny":8.2}}`,    // 与上次 7.0 相差 > 10%
	}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"}, fetcher)

	ok, errMsg, view, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, errMsg, "fx-a.test")
	require.Contains(t, errMsg, "out of range")
	require.Contains(t, errMsg, "fx-b.test")
	require.Contains(t, errMsg, "changed")
	require.Equal(t, "7.0", repo.get(SettingKeyFxUSDCNYFetched), "异常值不得写入")
	require.InDelta(t, 7.0, view.EffectiveRate, 1e-12)
	require.Equal(t, errMsg, view.LastError)
}

// 防止卡死（design §12.1）：只有一个来源的值相对上次跳变 > 10% 时仍丢弃。
func TestFxRateRefresh_SingleSourceJumpRejected(t *testing.T) {
	fetcher := &fxFetcherStub{
		responses: map[string]string{fxTestSourceA: `{"rates":{"CNY":7.9}}`},
		errs:      map[string]error{fxTestSourceB: errors.New("HTTP 503")},
	}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0", SettingKeyFxUSDCNYSource: "old.test"}, fetcher)

	ok, errMsg, _, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, errMsg, "changed")
	require.Equal(t, "7.0", repo.get(SettingKeyFxUSDCNYFetched))
	require.Equal(t, "old.test", repo.get(SettingKeyFxUSDCNYSource))
}

// 防止卡死：同一轮两个不同来源的值都只因跳变 > 10% 被丢弃、但彼此相差 ≤ 1% 时，视为真实变动，
// 采用先尝试的来源的值。
func TestFxRateRefresh_TwoAgreeingSourcesConfirmJump(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":7.9}}`,
		fxTestSourceB: `{"usd":{"cny":7.95}}`, // 与 A 相差约 0.63%
	}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"}, fetcher)
	changed := 0
	svc.onChange = func() { changed++ }

	ok, errMsg, view, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.True(t, ok, errMsg)
	require.Equal(t, "7.9", repo.get(SettingKeyFxUSDCNYFetched), "采用首个来源的值")
	require.Equal(t, "fx-a.test", repo.get(SettingKeyFxUSDCNYSource))
	require.InDelta(t, 7.9, view.EffectiveRate, 1e-12)
	require.Empty(t, view.LastError)
	require.Equal(t, 1, changed)
}

// 防止卡死：两个来源都跳变但彼此相差 > 1% 时不能互相确认，仍丢弃。
func TestFxRateRefresh_TwoDisagreeingSourcesRejected(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":7.9}}`,
		fxTestSourceB: `{"usd":{"cny":8.1}}`, // 与 A 相差约 2.5%
	}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"}, fetcher)

	ok, errMsg, _, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, errMsg, "fx-a.test")
	require.Contains(t, errMsg, "fx-b.test")
	require.Equal(t, "7.0", repo.get(SettingKeyFxUSDCNYFetched))
}

// 防止卡死：超出 [5, 9] 的值永远不参与确认，即使与另一个跳变值相差 ≤ 1%。
func TestFxRateRefresh_OutOfRangeNeverConfirms(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":9.02}}`, // 超出范围
		fxTestSourceB: `{"usd":{"cny":8.99}}`,   // 跳变（相对 7.0 约 28%），与 A 相差 < 1%
	}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"}, fetcher)

	ok, _, _, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "7.0", repo.get(SettingKeyFxUSDCNYFetched))
}

// 防止卡死：三个来源时，跳变值可由后面任一不同来源确认；采用一致的两者中先尝试的那个。
func TestFxRateRefresh_ConsensusAcrossNonAdjacentSources(t *testing.T) {
	const sourceC = "http://fx-c.test/usd.json"
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":7.9}}`,
		fxTestSourceB: `{"usd":{"cny":8.5}}`,
		sourceC:       `{"usd":{"cny":7.92}}`,
	}}
	repo := newFxSettingRepoStub(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"})
	cfg := &config.Config{}
	cfg.FX.SourceURLs = []string{fxTestSourceA, fxTestSourceB, sourceC}
	svc := NewFxRateService(repo, fetcher, cfg)

	ok, errMsg, _, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.True(t, ok, errMsg)
	require.Equal(t, "7.9", repo.get(SettingKeyFxUSDCNYFetched))
	require.Equal(t, "fx-a.test", repo.get(SettingKeyFxUSDCNYSource))
}

// 同一轮里正常通过校验的来源仍按顺序优先（行为与引入防止卡死规则前相同）。
func TestFxRateRefresh_PassingSourceStillWinsInOrder(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{
		fxTestSourceA: `{"rates":{"CNY":7.9}}`, // 跳变
		fxTestSourceB: `{"usd":{"cny":7.2}}`,   // 正常
	}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYFetched: "7.0"}, fetcher)

	ok, _, _, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "7.2", repo.get(SettingKeyFxUSDCNYFetched))
	require.Equal(t, "fx-b.test", repo.get(SettingKeyFxUSDCNYSource))
}

func TestFxRatesAgree(t *testing.T) {
	require.True(t, fxRatesAgree(7.0, 7.07), "恰好 1%")
	require.True(t, fxRatesAgree(7.07, 7.0), "对称")
	require.False(t, fxRatesAgree(7.0, 7.08))
	require.False(t, fxRatesAgree(0, 0))
	require.ErrorIs(t, validateFetchedFxRate(7.8, ptrFloat(7.0)), errFxRateChangeTooLarge)
	require.NotErrorIs(t, validateFetchedFxRate(9.5, ptrFloat(7.0)), errFxRateChangeTooLarge, "超出范围不算「只因跳变」")
}

func TestFxRateRefresh_AllSourcesFailUsesDefault(t *testing.T) {
	fetcher := &fxFetcherStub{errs: map[string]error{
		fxTestSourceA: errors.New("dial tcp: i/o timeout"),
		fxTestSourceB: errors.New("HTTP 503"),
	}}
	svc, _ := newFxTestService(nil, fetcher)

	ok, errMsg, view, err := svc.RefreshNow(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
	require.Contains(t, errMsg, "all fx rate sources failed")
	require.InDelta(t, DefaultUSDCNYRate, view.EffectiveRate, 1e-12)
	require.Equal(t, FxRateEffectiveFromDefault, view.EffectiveFrom)
	require.Nil(t, view.FetchedRate)
	require.True(t, view.Stale, "开启自动更新且从未成功：提醒")
}

func TestFxRateView_StaleAndDisabled(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	settings := map[string]string{
		SettingKeyFxUSDCNYFetched:   "7.1",
		SettingKeyFxUSDCNYFetchedAt: now.Add(-73 * time.Hour).Format(time.RFC3339),
		SettingKeyFxUSDCNYSource:    "api.frankfurter.dev",
	}
	svc, repo := newFxTestService(settings, &fxFetcherStub{})
	svc.now = func() time.Time { return now }

	view, err := svc.Get(context.Background())
	require.NoError(t, err)
	require.True(t, view.Stale, "超过 3 天未更新")
	require.InDelta(t, 7.1, view.EffectiveRate, 1e-12, "过期的获取值仍优先于默认值")

	repo.values[SettingKeyFxUSDCNYFetchedAt] = now.Add(-71 * time.Hour).Format(time.RFC3339)
	view, err = svc.Get(context.Background())
	require.NoError(t, err)
	require.False(t, view.Stale)

	repo.values[SettingKeyFxUSDCNYAutoEnabled] = "false"
	repo.values[SettingKeyFxUSDCNYFetchedAt] = now.Add(-200 * time.Hour).Format(time.RFC3339)
	view, err = svc.Get(context.Background())
	require.NoError(t, err)
	require.False(t, view.Stale, "关闭自动更新不提醒")
	require.InDelta(t, DefaultUSDCNYRate, view.EffectiveRate, 1e-12)
	require.Equal(t, FxRateEffectiveFromDefault, view.EffectiveFrom)
}

func TestFxRateUpdate_ValidatesDefaultRate(t *testing.T) {
	svc, repo := newFxTestService(nil, &fxFetcherStub{})
	changed := 0
	svc.onChange = func() { changed++ }

	for _, bad := range []float64{4.99, 9.01, 0, -1} {
		_, err := svc.Update(context.Background(), true, bad)
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err), strconv.FormatFloat(bad, 'f', -1, 64))
	}
	require.Zero(t, changed)

	view, err := svc.Update(context.Background(), false, 7.25)
	require.NoError(t, err)
	require.Equal(t, "false", repo.get(SettingKeyFxUSDCNYAutoEnabled))
	require.Equal(t, "7.25", repo.get(SettingKeyFxUSDCNYDefault))
	require.False(t, view.AutoEnabled)
	require.InDelta(t, 7.25, view.DefaultRate, 1e-12)
	require.InDelta(t, 7.25, view.EffectiveRate, 1e-12)
	require.Equal(t, 1, changed)
}

func TestFxRateBackgroundLoop_RefreshesWhenEnabledAndStops(t *testing.T) {
	fetcher := &fxFetcherStub{
		responses: map[string]string{fxTestSourceA: `{"rates":{"CNY":7.2}}`},
		called:    make(chan string, 4),
	}
	svc, repo := newFxTestService(nil, fetcher)
	svc.firstDelay = time.Millisecond
	svc.interval = time.Hour
	svc.jitter = func(time.Duration) time.Duration { return 0 }
	svc.Start()
	select {
	case <-fetcher.called:
	case <-time.After(5 * time.Second):
		t.Fatal("background loop did not refresh")
	}
	svc.Stop()
	svc.Stop() // 幂等
	require.Eventually(t, func() bool { return repo.get(SettingKeyFxUSDCNYFetched) == "7.2" }, time.Second, 5*time.Millisecond)
}

func TestFxRateBackgroundLoop_SkipsWhenAutoDisabled(t *testing.T) {
	fetcher := &fxFetcherStub{responses: map[string]string{fxTestSourceA: `{"rates":{"CNY":7.2}}`}}
	svc, repo := newFxTestService(map[string]string{SettingKeyFxUSDCNYAutoEnabled: "false"}, fetcher)
	svc.backgroundRefresh(context.Background())
	require.Empty(t, fetcher.calls)
	require.Empty(t, repo.get(SettingKeyFxUSDCNYFetched))

	var stopped *FxRateService
	stopped.Stop() // nil 安全
	notStarted, _ := newFxTestService(nil, fetcher)
	notStarted.Stop() // 未启动时立即返回
}

func TestSettingService_GetPublicSettings_ExposesUSDCNYRate(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.InDelta(t, DefaultUSDCNYRate, settings.USDCNYRate, 1e-12)

	repo.values[SettingKeyFxUSDCNYFetched] = "7.0123"
	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.InDelta(t, 7.0123, settings.USDCNYRate, 1e-12)

	raw, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	payload, ok := raw.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.InDelta(t, 7.0123, payload.USDCNYRate, 1e-12)

	repo.values[SettingKeyFxUSDCNYAutoEnabled] = "false"
	repo.values[SettingKeyFxUSDCNYDefault] = "6.9"
	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.InDelta(t, 6.9, settings.USDCNYRate, 1e-12)
	require.False(t, strings.Contains(strings.Join(fxRatePublicSettingKeys, ","), SettingKeyFxUSDCNYSource), "公开设置不暴露来源")
}

// 汇率后台循环在路由装配（SetOnUpdateCallback）之前就已启动：两者并发时不得有数据竞争（go test -race）。
// 测试里除「循环 → 测试」的完成信号外，不让两个 goroutine 共享任何锁，否则 happens-before 会掩盖竞争。
func TestFxRateBackgroundLoop_PublicSettingsCallbackIsRaceSafe(t *testing.T) {
	settingSvc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})
	fetcher := &fxFetcherStub{responses: map[string]string{fxTestSourceA: `{"rates":{"CNY":7.2}}`}}
	svc, repo := newFxTestService(nil, fetcher)
	refreshed := make(chan struct{})
	var refreshedOnce sync.Once
	svc.onChange = func() {
		settingSvc.NotifyPublicSettingsChanged()
		refreshedOnce.Do(func() { close(refreshed) })
	}
	svc.firstDelay = time.Millisecond
	svc.interval = time.Hour
	svc.jitter = func(time.Duration) time.Duration { return 0 }

	var notified atomic.Int32
	svc.Start()
	// 与后台循环并发地装配回调（生产中由 router 在 wire 初始化之后调用）。
	settingSvc.SetOnUpdateCallback(func() { notified.Add(1) })

	select {
	case <-refreshed:
	case <-time.After(5 * time.Second):
		t.Fatal("background loop did not refresh")
	}
	svc.Stop()
	require.Equal(t, "7.2", repo.get(SettingKeyFxUSDCNYFetched))

	before := notified.Load()
	svc.notifyChange()
	require.Equal(t, before+1, notified.Load(), "装配后的回调必须被调用")

	var nilSetting *SettingService
	nilSetting.NotifyPublicSettingsChanged()                                                                               // nil 安全
	NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).NotifyPublicSettingsChanged() // 未装配回调时为空操作
}
