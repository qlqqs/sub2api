package service

// CUSTOM(fork): 美元兑人民币汇率。后台定时从公开接口获取，存设置表，默认值兜底。
// 汇率只用于用户端显示折扣（模型广场「相比官方价」、使用记录费用明细），不参与计费。
//
//   - 来源按顺序尝试（每个 10 秒超时），第一个成功即采用；配置项 fx.source_urls 非空时替换内置来源。
//   - 取到的值须在 [5, 9]，且与上次采用的值相差不超过 10%，否则丢弃并记 warn，继续试下一个来源。
//   - 防止卡死：只因「变化超过 10%」被丢弃的值，若同一轮有两个不同来源的值彼此相差 ≤ 1%，
//     视为真实变动，采用其中先尝试的来源的值（否则旧值偏离真实汇率超过 10% 后会永远更新不了）。
//     超出 [5, 9] 的值永远不参与。
//   - 生效值：开启自动更新且曾成功获取 → 最近一次成功获取的值；否则 → 默认汇率（初始 6.8）。
//   - 后台循环：启动约 1 分钟后取一次，之后每 6 小时一次，均带随机抖动；关闭自动更新时跳过。
//   - 多实例：每个实例各自拉取、各自写设置；值相同或相近，无需加锁。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 设置表 key。
const (
	SettingKeyFxUSDCNYAutoEnabled = "fx_usd_cny_auto_enabled"
	SettingKeyFxUSDCNYDefault     = "fx_usd_cny_default"
	SettingKeyFxUSDCNYFetched     = "fx_usd_cny_fetched"
	SettingKeyFxUSDCNYFetchedAt   = "fx_usd_cny_fetched_at"
	SettingKeyFxUSDCNYSource      = "fx_usd_cny_source"
)

const (
	// DefaultUSDCNYRate 默认汇率（从未成功获取或关闭自动更新时使用）。
	DefaultUSDCNYRate = 6.8

	FxRateEffectiveFromFetched = "fetched"
	FxRateEffectiveFromDefault = "default"

	fxRateMin       = 5.0
	fxRateMax       = 9.0
	fxRateMaxChange = 0.10
	// fxRateConsensusTolerance 两个来源「彼此一致」的相对差上限（防止卡死规则）。
	fxRateConsensusTolerance = 0.01
	fxRateStaleAge           = 72 * time.Hour

	fxRateSourceTimeout   = 10 * time.Second
	fxRateFirstRunDelay   = time.Minute
	fxRateFirstRunJitter  = 15 * time.Second
	fxRateRefreshInterval = 6 * time.Hour
	fxRateRefreshJitter   = 30 * time.Minute
	fxRateStopTimeout     = 5 * time.Second
)

// defaultFxRateSourceURLs 内置来源（2026-10-23 实测可用、免 key）：Frankfurter（ECB 参考价，
// 工作日更新）为首选，fawazahmed0 currency-api 的 jsDelivr 与 Cloudflare 镜像为备选。
var defaultFxRateSourceURLs = []string{
	"https://api.frankfurter.dev/v1/latest?base=USD&symbols=CNY",
	"https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.min.json",
	"https://latest.currency-api.pages.dev/v1/currencies/usd.min.json",
}

// fxRatePublicSettingKeys 公开设置计算生效汇率需要读取的 key。
var fxRatePublicSettingKeys = []string{
	SettingKeyFxUSDCNYAutoEnabled,
	SettingKeyFxUSDCNYDefault,
	SettingKeyFxUSDCNYFetched,
}

var fxRateAllSettingKeys = []string{
	SettingKeyFxUSDCNYAutoEnabled,
	SettingKeyFxUSDCNYDefault,
	SettingKeyFxUSDCNYFetched,
	SettingKeyFxUSDCNYFetchedAt,
	SettingKeyFxUSDCNYSource,
}

// FxRateFetcher 拉取汇率来源原始响应（repository 层实现，沿用价格表的代理配置）。
type FxRateFetcher interface {
	FetchFxRateJSON(ctx context.Context, url string) ([]byte, error)
}

// FxRateView 汇率当前状态（管理端展示）。
type FxRateView struct {
	AutoEnabled   bool
	DefaultRate   float64
	EffectiveRate float64
	EffectiveFrom string
	FetchedRate   *float64
	FetchedAt     *time.Time
	Source        string
	Stale         bool
	LastError     string
	LastAttemptAt *time.Time
}

type fxRateSource struct {
	url  string
	name string
}

// FxRateService 美元汇率服务。
type FxRateService struct {
	settingRepo SettingRepository
	fetcher     FxRateFetcher
	sources     []fxRateSource
	now         func() time.Time
	onChange    func()

	refreshMu sync.Mutex

	stateMu       sync.RWMutex
	lastError     string
	lastAttemptAt *time.Time

	firstDelay time.Duration
	interval   time.Duration
	jitter     func(max time.Duration) time.Duration

	startOnce sync.Once
	started   atomic.Bool
	stopOnce  sync.Once
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewFxRateService 创建汇率服务；来源取 cfg.FX.SourceURLs（非空时替换内置来源）。
func NewFxRateService(settingRepo SettingRepository, fetcher FxRateFetcher, cfg *config.Config) *FxRateService {
	var configured []string
	if cfg != nil {
		configured = cfg.FX.SourceURLs
	}
	return &FxRateService{
		settingRepo: settingRepo,
		fetcher:     fetcher,
		sources:     resolveFxRateSources(configured),
		now:         time.Now,
		firstDelay:  fxRateFirstRunDelay,
		interval:    fxRateRefreshInterval,
		jitter:      randomFxRateJitter,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

// ProvideFxRateService 创建并启动汇率后台循环；汇率变化时让注入 HTML 的公开设置缓存失效。
func ProvideFxRateService(settingRepo SettingRepository, fetcher FxRateFetcher, cfg *config.Config, settingService *SettingService) *FxRateService {
	svc := NewFxRateService(settingRepo, fetcher, cfg)
	svc.onChange = settingService.NotifyPublicSettingsChanged
	svc.Start()
	return svc
}

// NotifyPublicSettingsChanged 让依赖公开设置的缓存（注入了设置的 index.html）失效。
// 供不经 UpdateSettings 写设置表的 fork 功能（如汇率）调用。汇率后台循环早于 router
// 装配回调就已启动，因此这里经原子副本读取回调，不直接读 onUpdate 字段（否则有数据竞争）。
func (s *SettingService) NotifyPublicSettingsChanged() {
	if s == nil {
		return
	}
	if hook := s.publicSettingsChangedHook.Load(); hook != nil && *hook != nil {
		(*hook)()
	}
}

func randomFxRateJitter(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(max)))
}

// resolveFxRateSources 归一化来源列表：每项再按逗号 / 空白拆分（兼容 FX_SOURCE_URLS 的两种分隔），
// 去空去重；为空时使用内置来源。来源名取 URL 的 host。
func resolveFxRateSources(configured []string) []fxRateSource {
	var urls []string
	seen := map[string]struct{}{}
	for _, item := range configured {
		for _, part := range strings.FieldsFunc(item, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
		}) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, dup := seen[part]; dup {
				continue
			}
			seen[part] = struct{}{}
			urls = append(urls, part)
		}
	}
	if len(urls) == 0 {
		urls = defaultFxRateSourceURLs
	}
	out := make([]fxRateSource, 0, len(urls))
	for _, raw := range urls {
		out = append(out, fxRateSource{url: raw, name: fxRateSourceName(raw)})
	}
	return out
}

func fxRateSourceName(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	if len(raw) > 64 {
		return raw[:64]
	}
	return raw
}

// parseFxRateResponse 按内容识别响应格式：Frankfurter / ECB 风格 {"rates":{"CNY":x}}，
// 或 currency-api 风格 {"usd":{"cny":x}}。
func parseFxRateResponse(body []byte) (float64, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return 0, fmt.Errorf("invalid json: %w", err)
	}
	lookup := func(objKey string, keys ...string) (float64, bool, error) {
		raw, ok := doc[objKey]
		if !ok {
			return 0, false, nil
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return 0, false, nil
		}
		for _, k := range keys {
			v, ok := obj[k]
			if !ok {
				continue
			}
			var rate float64
			if err := json.Unmarshal(v, &rate); err != nil {
				return 0, true, fmt.Errorf("%s.%s is not a number", objKey, k)
			}
			return rate, true, nil
		}
		return 0, false, nil
	}
	if rate, ok, err := lookup("rates", "CNY", "cny"); ok || err != nil {
		return rate, err
	}
	if rate, ok, err := lookup("usd", "cny", "CNY"); ok || err != nil {
		return rate, err
	}
	return 0, errors.New("unrecognized response format (expected rates.CNY or usd.cny)")
}

// errFxRateChangeTooLarge 标记「值在 [5, 9] 内、只因相对上次变化超过 10% 被丢弃」，
// 这类值可参与同一轮的多来源一致性确认（见 fxRateConsensus）。
var errFxRateChangeTooLarge = errors.New("change exceeds limit")

// validateFetchedFxRate 校验取到的汇率：[5, 9]，且已有上次值时相对变化 ≤ 10%。
// 只因变化过大被拒时返回的错误 errors.Is(err, errFxRateChangeTooLarge)。
func validateFetchedFxRate(rate float64, previous *float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < fxRateMin || rate > fxRateMax {
		return fmt.Errorf("rate %v out of range [%v, %v]", rate, fxRateMin, fxRateMax)
	}
	if previous != nil && *previous > 0 {
		if change := math.Abs(rate-*previous) / *previous; change > fxRateMaxChange+1e-12 {
			return fmt.Errorf("rate %v changed %.1f%% from previous %v (max %.0f%%): %w", rate, change*100, *previous, fxRateMaxChange*100, errFxRateChangeTooLarge)
		}
	}
	return nil
}

// fxRateCandidate 是同一轮里只因变化过大被丢弃的来源值。
type fxRateCandidate struct {
	src  fxRateSource
	rate float64
}

// fxRatesAgree 判断两个来源的值是否彼此相差 ≤ 1%（相对较小者，对称）。
func fxRatesAgree(a, b float64) bool {
	lo := math.Min(a, b)
	if !(lo > 0) {
		return false
	}
	return math.Abs(a-b)/lo <= fxRateConsensusTolerance+1e-12
}

// fxRateConsensus 在本轮已被「变化过大」丢弃的值里找与 rate 一致的、来自不同来源的值，
// 找到时返回其中先尝试的那个（即调用方应采用的值）。
func fxRateConsensus(rejected []fxRateCandidate, src fxRateSource, rate float64) (fxRateCandidate, bool) {
	for _, prev := range rejected {
		if prev.src.url == src.url {
			continue
		}
		if fxRatesAgree(prev.rate, rate) {
			return prev, true
		}
	}
	return fxRateCandidate{}, false
}

func parsePositiveFloatSetting(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	return v, true
}

func fxRateAutoEnabled(settings map[string]string) bool {
	return !isFalseSettingValue(settings[SettingKeyFxUSDCNYAutoEnabled])
}

func fxRateDefault(settings map[string]string) float64 {
	if v, ok := parsePositiveFloatSetting(settings[SettingKeyFxUSDCNYDefault]); ok {
		return v
	}
	return DefaultUSDCNYRate
}

// resolveEffectiveUSDCNYRate 生效汇率：开启自动更新且曾成功获取 → 获取值；否则 → 默认值。
func resolveEffectiveUSDCNYRate(settings map[string]string) (float64, string) {
	if fxRateAutoEnabled(settings) {
		if v, ok := parsePositiveFloatSetting(settings[SettingKeyFxUSDCNYFetched]); ok {
			return v, FxRateEffectiveFromFetched
		}
	}
	return fxRateDefault(settings), FxRateEffectiveFromDefault
}

// applyPublicUSDCNYRate 填充公开设置里的生效汇率。
func applyPublicUSDCNYRate(public *PublicSettings, settings map[string]string) {
	if public == nil {
		return
	}
	public.USDCNYRate, _ = resolveEffectiveUSDCNYRate(settings)
}

// Get 返回汇率当前状态。
func (s *FxRateService) Get(ctx context.Context) (*FxRateView, error) {
	settings, err := s.settingRepo.GetMultiple(ctx, fxRateAllSettingKeys)
	if err != nil {
		return nil, fmt.Errorf("get fx rate settings: %w", err)
	}
	return s.buildView(settings), nil
}

func (s *FxRateService) buildView(settings map[string]string) *FxRateView {
	view := &FxRateView{
		AutoEnabled: fxRateAutoEnabled(settings),
		DefaultRate: fxRateDefault(settings),
		Source:      strings.TrimSpace(settings[SettingKeyFxUSDCNYSource]),
	}
	view.EffectiveRate, view.EffectiveFrom = resolveEffectiveUSDCNYRate(settings)
	if v, ok := parsePositiveFloatSetting(settings[SettingKeyFxUSDCNYFetched]); ok {
		view.FetchedRate = &v
	}
	if raw := strings.TrimSpace(settings[SettingKeyFxUSDCNYFetchedAt]); raw != "" {
		if at, err := time.Parse(time.RFC3339, raw); err == nil {
			view.FetchedAt = &at
		}
	}
	if view.AutoEnabled {
		view.Stale = view.FetchedRate == nil || view.FetchedAt == nil || s.now().Sub(*view.FetchedAt) > fxRateStaleAge
	}
	s.stateMu.RLock()
	view.LastError = s.lastError
	if s.lastAttemptAt != nil {
		at := *s.lastAttemptAt
		view.LastAttemptAt = &at
	}
	s.stateMu.RUnlock()
	return view
}

// Update 保存自动更新开关与默认汇率（默认汇率须在 5–9）。
func (s *FxRateService) Update(ctx context.Context, autoEnabled bool, defaultRate float64) (*FxRateView, error) {
	if math.IsNaN(defaultRate) || math.IsInf(defaultRate, 0) || defaultRate < fxRateMin || defaultRate > fxRateMax {
		return nil, infraerrors.BadRequest("FX_RATE_INVALID", fmt.Sprintf("默认汇率须在 %v–%v 之间", fxRateMin, fxRateMax))
	}
	defaultRate = math.Round(defaultRate*10000) / 10000
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyFxUSDCNYAutoEnabled: strconv.FormatBool(autoEnabled),
		SettingKeyFxUSDCNYDefault:     strconv.FormatFloat(defaultRate, 'f', -1, 64),
	}); err != nil {
		return nil, fmt.Errorf("save fx rate settings: %w", err)
	}
	s.notifyChange()
	return s.Get(ctx)
}

// RefreshNow 立即从来源获取一次（不受自动更新开关影响）。全部来源失败时 ok=false、
// errMsg 为失败原因；err 只用于读写设置表失败。
func (s *FxRateService) RefreshNow(ctx context.Context) (ok bool, errMsg string, view *FxRateView, err error) {
	refreshErr := s.refresh(ctx)
	view, err = s.Get(ctx)
	if err != nil {
		return false, "", nil, err
	}
	if refreshErr != nil {
		return false, refreshErr.Error(), view, nil
	}
	return true, "", view, nil
}

func (s *FxRateService) refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	attemptAt := s.now()
	s.stateMu.Lock()
	s.lastAttemptAt = &attemptAt
	s.stateMu.Unlock()

	err := s.fetchAndStore(ctx)

	s.stateMu.Lock()
	if err != nil {
		s.lastError = err.Error()
	} else {
		s.lastError = ""
	}
	s.stateMu.Unlock()
	return err
}

func (s *FxRateService) fetchAndStore(ctx context.Context) error {
	if s.fetcher == nil {
		return errors.New("fx rate fetcher is not configured")
	}
	settings, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyFxUSDCNYFetched})
	if err != nil {
		return fmt.Errorf("read previous fx rate: %w", err)
	}
	var previous *float64
	if v, ok := parsePositiveFloatSetting(settings[SettingKeyFxUSDCNYFetched]); ok {
		previous = &v
	}

	failures := make([]string, 0, len(s.sources))
	var jumped []fxRateCandidate // 本轮只因变化过大被丢弃的值（防止卡死规则）
	for _, src := range s.sources {
		rate, fetchErr := s.fetchOne(ctx, src, previous)
		if fetchErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(fetchErr, errFxRateChangeTooLarge) {
				if agreed, ok := fxRateConsensus(jumped, src, rate); ok {
					slog.Warn("fx_rate_change_confirmed_by_sources",
						"source", agreed.src.name, "rate", agreed.rate,
						"confirming_source", src.name, "confirming_rate", rate,
						"previous", *previous)
					return s.storeFetchedFxRate(ctx, agreed.src, agreed.rate)
				}
				jumped = append(jumped, fxRateCandidate{src: src, rate: rate})
			}
			failures = append(failures, src.name+": "+fetchErr.Error())
			continue
		}
		return s.storeFetchedFxRate(ctx, src, rate)
	}
	return fmt.Errorf("all fx rate sources failed: %s", strings.Join(failures, "; "))
}

func (s *FxRateService) storeFetchedFxRate(ctx context.Context, src fxRateSource, rate float64) error {
	rate = math.Round(rate*1e6) / 1e6
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyFxUSDCNYFetched:   strconv.FormatFloat(rate, 'f', -1, 64),
		SettingKeyFxUSDCNYFetchedAt: s.now().UTC().Format(time.RFC3339),
		SettingKeyFxUSDCNYSource:    src.name,
	}); err != nil {
		return fmt.Errorf("save fx rate: %w", err)
	}
	slog.Info("fx_rate_updated", "source", src.name, "rate", rate)
	s.notifyChange()
	return nil
}

func (s *FxRateService) fetchOne(ctx context.Context, src fxRateSource, previous *float64) (float64, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, fxRateSourceTimeout)
	defer cancel()
	body, err := s.fetcher.FetchFxRateJSON(fetchCtx, src.url)
	if err != nil {
		slog.Warn("fx_rate_source_failed", "source", src.name, "error", err)
		return 0, err
	}
	rate, err := parseFxRateResponse(body)
	if err != nil {
		slog.Warn("fx_rate_source_unparsable", "source", src.name, "error", err)
		return 0, err
	}
	if err := validateFetchedFxRate(rate, previous); err != nil {
		slog.Warn("fx_rate_source_rejected", "source", src.name, "rate", rate, "error", err)
		if errors.Is(err, errFxRateChangeTooLarge) {
			return rate, err // 带回取到的值，供同一轮的多来源一致性确认
		}
		return 0, err
	}
	return rate, nil
}

func (s *FxRateService) notifyChange() {
	if s.onChange != nil {
		s.onChange()
	}
}

// Start 启动后台循环（幂等）。
func (s *FxRateService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		s.started.Store(true)
		go s.loop()
	})
}

// Stop 停止后台循环（幂等）：取消在途请求并等待循环退出（最多 fxRateStopTimeout）。
func (s *FxRateService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	if !s.started.Load() {
		return
	}
	select {
	case <-s.doneCh:
	case <-time.After(fxRateStopTimeout):
	}
}

func (s *FxRateService) loop() {
	defer close(s.doneCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	timer := time.NewTimer(s.firstDelay + s.jitter(fxRateFirstRunJitter))
	defer timer.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
		}
		s.backgroundRefresh(ctx)
		timer.Reset(s.interval + s.jitter(fxRateRefreshJitter))
	}
}

func (s *FxRateService) backgroundRefresh(ctx context.Context) {
	settings, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyFxUSDCNYAutoEnabled})
	if err != nil {
		slog.Warn("fx_rate_settings_read_failed", "error", err)
		return
	}
	if !fxRateAutoEnabled(settings) {
		return
	}
	if err := s.refresh(ctx); err != nil && ctx.Err() == nil {
		slog.Warn("fx_rate_refresh_failed", "error", err)
	}
}
