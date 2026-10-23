package service

// CUSTOM(fork): 分组内单个模型倍率（表 group_model_rate_multipliers，迁移 253）。
//
// 一个分组可以给个别模型单独设倍率，计费时按计费模型（与 matchGroupModelPricing
// 同一口径，名字先 normalizeChannelPricingModelName）匹配：精确优先，其次前缀最长的
// 末尾 * 通配。命中时用户专属倍率不再作用于该模型；没命中时完全沿用原来的
// 「用户专属 ?? 分组倍率」。高峰倍率、渠道分时倍率、生图/视频独立倍率都在这一项
// 之外叠加，规则不变。
//
// 读路径：进程内整表快照（表很小），30 秒过期 + singleflight 加载；加载失败沿用旧快照
// 并记 warn，从未加载成功则视为无规则（退回原行为）。管理端保存后清空本进程快照。
// 未注入（nil）时 Match 恒为未命中，行为与引入本功能前完全相同。

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/singleflight"
)

const (
	// GroupModelRateMaxRulesPerGroup 单个分组最多的模型倍率规则数。
	GroupModelRateMaxRulesPerGroup = 200
	// GroupModelRateMaxMultiplier 单条规则倍率上限（与 DECIMAL(10,4) 相容）。
	GroupModelRateMaxMultiplier = 10000
	// groupModelRateMaxPatternLen 与 model_pattern VARCHAR(255) 对齐。
	groupModelRateMaxPatternLen = 255

	// groupModelRateSnapshotTTL 与用户专属倍率缓存同样的时效。
	groupModelRateSnapshotTTL = 30 * time.Second
	// groupModelRateLoadRetryInterval 加载失败后的重试间隔，避免故障期间每个请求都回源。
	groupModelRateLoadRetryInterval = 5 * time.Second
	// groupModelRateLoadTimeout 快照加载超时（与请求 ctx 解耦，避免请求取消导致整批等待者失败）。
	groupModelRateLoadTimeout = 5 * time.Second
)

// groupModelRateInvalidReason 模型倍率规则校验失败的 reason（message 为可读原因）。
const groupModelRateInvalidReason = "GROUP_MODEL_RATE_INVALID"

// GroupModelRateRule 是一条分组模型倍率规则。Model 为归一化后的模型名，可带末尾 *。
type GroupModelRateRule struct {
	Model          string  `json:"model"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// GroupModelRateRepository 是分组模型倍率的持久化端口（fork 自有表，原生 SQL）。
type GroupModelRateRepository interface {
	// ListAll 返回全部未删除分组的规则（groupID → 规则，按 model 排序），只含有规则的分组。
	ListAll(ctx context.Context) (map[int64][]GroupModelRateRule, error)
	// ListByGroup 返回单个分组的规则（按 model 排序）。
	ListByGroup(ctx context.Context, groupID int64) ([]GroupModelRateRule, error)
	// ReplaceForGroup 在一个事务内整组替换规则；分组不存在（或已软删除）返回 ErrGroupNotFound。
	ReplaceForGroup(ctx context.Context, groupID int64, rules []GroupModelRateRule) error
	// CopyGroup 把 fromID 的规则复制到 toID（toID 已有同名规则时覆盖倍率）。
	CopyGroup(ctx context.Context, fromID, toID int64) error
	// GroupExists 判断分组存在且未软删除。
	GroupExists(ctx context.Context, groupID int64) (bool, error)
}

type groupModelRateWildcard struct {
	prefix string
	rate   float64
}

// groupModelRateIndex 是单个分组的匹配索引：精确规则走 map，通配规则按前缀长度降序。
type groupModelRateIndex struct {
	exact     map[string]float64
	wildcards []groupModelRateWildcard
}

func newGroupModelRateIndex(rules []GroupModelRateRule) *groupModelRateIndex {
	idx := &groupModelRateIndex{exact: make(map[string]float64, len(rules))}
	for _, rule := range rules {
		pattern := normalizeGroupModelRateName(rule.Model)
		if pattern == "" || !(rule.RateMultiplier > 0) {
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if prefix == "" {
				continue
			}
			idx.wildcards = append(idx.wildcards, groupModelRateWildcard{prefix: prefix, rate: rule.RateMultiplier})
			continue
		}
		idx.exact[pattern] = rule.RateMultiplier
	}
	sort.SliceStable(idx.wildcards, func(i, j int) bool {
		if len(idx.wildcards[i].prefix) != len(idx.wildcards[j].prefix) {
			return len(idx.wildcards[i].prefix) > len(idx.wildcards[j].prefix)
		}
		return idx.wildcards[i].prefix < idx.wildcards[j].prefix
	})
	return idx
}

// match 返回模型命中的倍率：精确优先，其次前缀最长的通配。model 须已归一化。
func (idx *groupModelRateIndex) match(model string) (float64, bool) {
	if idx == nil || model == "" {
		return 0, false
	}
	if rate, ok := idx.exact[model]; ok {
		return rate, true
	}
	for _, w := range idx.wildcards {
		if strings.HasPrefix(model, w.prefix) {
			return w.rate, true
		}
	}
	return 0, false
}

func (idx *groupModelRateIndex) empty() bool {
	return idx == nil || (len(idx.exact) == 0 && len(idx.wildcards) == 0)
}

type groupModelRateSnapshot struct {
	groups map[int64]*groupModelRateIndex
}

// GroupModelRateService 分组模型倍率：整表快照缓存 + 管理端读写 + 校验。
type GroupModelRateService struct {
	repo GroupModelRateRepository
	ttl  time.Duration
	now  func() time.Time

	mu         sync.RWMutex
	snap       *groupModelRateSnapshot
	nextLoadAt time.Time
	generation uint64

	sf singleflight.Group
}

// NewGroupModelRateService 创建分组模型倍率服务。
func NewGroupModelRateService(repo GroupModelRateRepository) *GroupModelRateService {
	return &GroupModelRateService{
		repo: repo,
		ttl:  groupModelRateSnapshotTTL,
		now:  time.Now,
	}
}

// normalizeGroupModelRateName 与 matchGroupModelPricing 同一归一化口径。
func normalizeGroupModelRateName(model string) string {
	return normalizeChannelPricingModelName(model)
}

// Match 返回 groupID 内 model 命中的模型倍率；未注入、无规则或未命中时返回 (0, false)。
func (s *GroupModelRateService) Match(ctx context.Context, groupID int64, model string) (float64, bool) {
	if s == nil || s.repo == nil || groupID <= 0 {
		return 0, false
	}
	model = normalizeGroupModelRateName(model)
	if model == "" {
		return 0, false
	}
	snap := s.snapshot(ctx)
	if snap == nil {
		return 0, false
	}
	return snap.groups[groupID].match(model)
}

// HasRules 判断分组是否配置了任何模型倍率（规则倍率恒 > 0）。
func (s *GroupModelRateService) HasRules(ctx context.Context, groupID int64) bool {
	if s == nil || s.repo == nil || groupID <= 0 {
		return false
	}
	snap := s.snapshot(ctx)
	if snap == nil {
		return false
	}
	return !snap.groups[groupID].empty()
}

// Invalidate 让本进程快照立即失效（保留旧快照作为加载失败时的兜底）。
func (s *GroupModelRateService) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.generation++
	s.nextLoadAt = time.Time{}
	s.mu.Unlock()
}

func (s *GroupModelRateService) snapshot(ctx context.Context) *groupModelRateSnapshot {
	now := s.now()
	s.mu.RLock()
	snap, nextLoadAt, gen := s.snap, s.nextLoadAt, s.generation
	s.mu.RUnlock()
	if !nextLoadAt.IsZero() && now.Before(nextLoadAt) {
		return snap
	}

	// singleflight key 带上代数：失效后发起的加载不会复用失效前在途的旧加载。
	v, _, _ := s.sf.Do(strconv.FormatUint(gen, 10), func() (any, error) {
		s.mu.RLock()
		cur, curNext, curGen := s.snap, s.nextLoadAt, s.generation
		s.mu.RUnlock()
		if curGen == gen && !curNext.IsZero() && s.now().Before(curNext) {
			return cur, nil
		}
		return s.load(ctx, gen), nil
	})
	if loaded, ok := v.(*groupModelRateSnapshot); ok {
		return loaded
	}
	return snap
}

func (s *GroupModelRateService) load(ctx context.Context, gen uint64) *groupModelRateSnapshot {
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), groupModelRateLoadTimeout)
	defer cancel()
	all, err := s.repo.ListAll(loadCtx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		slog.Warn("group_model_rate_snapshot_load_failed", "error", err, "has_previous", s.snap != nil)
		if s.generation == gen {
			s.nextLoadAt = s.now().Add(groupModelRateLoadRetryInterval)
		}
		return s.snap
	}
	snap := &groupModelRateSnapshot{groups: make(map[int64]*groupModelRateIndex, len(all))}
	for groupID, rules := range all {
		if idx := newGroupModelRateIndex(rules); !idx.empty() {
			snap.groups[groupID] = idx
		}
	}
	if s.generation != gen {
		// 失效前发起的加载：数据可能早于本进程刚保存的规则，不得覆盖失效后加载到的新快照；
		// 只在从未加载成功时兜底保存（nextLoadAt 仍为零，下一次调用会重新加载）。
		if s.snap == nil {
			s.snap = snap
		}
		return snap
	}
	s.snap = snap
	s.nextLoadAt = s.now().Add(s.ttl)
	return snap
}

// ListAll 返回全部未删除分组的规则（管理端列表页提示用，直接读库）。
func (s *GroupModelRateService) ListAll(ctx context.Context) (map[int64][]GroupModelRateRule, error) {
	if s == nil || s.repo == nil {
		return map[int64][]GroupModelRateRule{}, nil
	}
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list group model rates: %w", err)
	}
	if all == nil {
		all = map[int64][]GroupModelRateRule{}
	}
	for groupID, rules := range all {
		all[groupID] = sortedGroupModelRateRules(rules)
	}
	return all, nil
}

// ListForGroup 返回单个分组的规则（按 model 排序）；分组不存在返回 ErrGroupNotFound。
func (s *GroupModelRateService) ListForGroup(ctx context.Context, groupID int64) ([]GroupModelRateRule, error) {
	if s == nil || s.repo == nil {
		return []GroupModelRateRule{}, nil
	}
	if err := s.ensureGroupExists(ctx, groupID); err != nil {
		return nil, err
	}
	rules, err := s.repo.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group model rates: %w", err)
	}
	return sortedGroupModelRateRules(rules), nil
}

// ReplaceForGroup 校验并整组替换规则（空切片 = 清空），返回落库后的规则；保存后本进程立即生效。
func (s *GroupModelRateService) ReplaceForGroup(ctx context.Context, groupID int64, rules []GroupModelRateRule) ([]GroupModelRateRule, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("GROUP_MODEL_RATE_UNAVAILABLE", "group model rate service is not configured")
	}
	normalized, err := ValidateGroupModelRateRules(rules)
	if err != nil {
		return nil, err
	}
	if err := s.ensureGroupExists(ctx, groupID); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceForGroup(ctx, groupID, normalized); err != nil {
		return nil, fmt.Errorf("replace group model rates: %w", err)
	}
	s.Invalidate()
	saved, err := s.repo.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list group model rates: %w", err)
	}
	return sortedGroupModelRateRules(saved), nil
}

// CopyGroup 把 fromID 的规则复制到 toID（复制分组后调用）。
func (s *GroupModelRateService) CopyGroup(ctx context.Context, fromID, toID int64) error {
	if s == nil || s.repo == nil || fromID <= 0 || toID <= 0 || fromID == toID {
		return nil
	}
	if err := s.repo.CopyGroup(ctx, fromID, toID); err != nil {
		return fmt.Errorf("copy group model rates: %w", err)
	}
	s.Invalidate()
	return nil
}

func (s *GroupModelRateService) ensureGroupExists(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return ErrGroupNotFound
	}
	exists, err := s.repo.GroupExists(ctx, groupID)
	if err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if !exists {
		return ErrGroupNotFound
	}
	return nil
}

// ValidateGroupModelRateRules 校验并归一化规则：模型名去空格、归一化后非空；* 只允许在末尾
// 且前面至少一个字符；同组不重复（按归一化后比较）；0 < 倍率 ≤ 10000（按 4 位小数落库后仍 > 0）；
// 单组最多 200 条。返回按 model 排序的归一化规则。
func ValidateGroupModelRateRules(rules []GroupModelRateRule) ([]GroupModelRateRule, error) {
	if len(rules) > GroupModelRateMaxRulesPerGroup {
		return nil, groupModelRateInvalid(fmt.Sprintf("每个分组最多 %d 条模型单独倍率，当前 %d 条", GroupModelRateMaxRulesPerGroup, len(rules)))
	}
	out := make([]GroupModelRateRule, 0, len(rules))
	seen := make(map[string]int, len(rules))
	for i, rule := range rules {
		pos := i + 1
		raw := strings.TrimSpace(rule.Model)
		model := normalizeGroupModelRateName(raw)
		if model == "" {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：模型名不能为空", pos))
		}
		if strings.ContainsAny(model, " \t\r\n") {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：模型名 %q 不能包含空白字符", pos, raw))
		}
		if len(model) > groupModelRateMaxPatternLen {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：模型名过长（最多 %d 个字符）", pos, groupModelRateMaxPatternLen))
		}
		if stars := strings.Count(model, "*"); stars > 0 {
			if stars > 1 || !strings.HasSuffix(model, "*") {
				return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：模型名 %q 的通配符 * 只能放在末尾", pos, raw))
			}
			if len(model) == 1 {
				return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：通配符 * 前至少要有一个字符", pos))
			}
		}
		if prev, dup := seen[model]; dup {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条：模型名 %q 与第 %d 条重复", pos, raw, prev))
		}
		rate := rule.RateMultiplier
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条（%s）：倍率须大于 0", pos, raw))
		}
		if rate > GroupModelRateMaxMultiplier {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条（%s）：倍率不能超过 %d", pos, raw, GroupModelRateMaxMultiplier))
		}
		rounded := math.Round(rate*10000) / 10000
		if rounded <= 0 {
			return nil, groupModelRateInvalid(fmt.Sprintf("第 %d 条（%s）：倍率须大于 0（最多 4 位小数）", pos, raw))
		}
		seen[model] = pos
		out = append(out, GroupModelRateRule{Model: model, RateMultiplier: rounded})
	}
	return sortedGroupModelRateRules(out), nil
}

func groupModelRateInvalid(message string) error {
	return infraerrors.BadRequest(groupModelRateInvalidReason, message)
}

func sortedGroupModelRateRules(rules []GroupModelRateRule) []GroupModelRateRule {
	out := make([]GroupModelRateRule, len(rules))
	copy(out, rules)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// resolveBaseRate 是所有计费路径共用的「基础倍率」（不含高峰、分时、生图/视频独立倍率）：
//   - 命中分组模型倍率 → 模型倍率，且**不调用** fallback：fallback 里的用户专属倍率解析会把
//     传入的默认值缓存在 user:group 键下，传模型倍率进去会污染同一用户的其他模型；
//   - 否则 → fallback()，即原来的「用户专属 ?? 分组倍率」逻辑。
func resolveBaseRate(ctx context.Context, rates *GroupModelRateService, groupID int64, model string, fallback func() float64) float64 {
	if rate, ok := rates.Match(ctx, groupID, model); ok {
		return rate
	}
	return fallback()
}

// ---- setter 注入（不改上游构造函数签名） ----

// SetGroupModelRateService 注入分组模型倍率（Anthropic / Gemini / Antigravity 网关计费、预扣与利润门）。
func (s *GatewayService) SetGroupModelRateService(rates *GroupModelRateService) {
	if s != nil {
		s.groupModelRates = rates
	}
}

// SetGroupModelRateService 注入分组模型倍率（OpenAI 兼容网关计费、预扣与利润门）。
func (s *OpenAIGatewayService) SetGroupModelRateService(rates *GroupModelRateService) {
	if s != nil {
		s.groupModelRates = rates
	}
}

// SetGroupModelRateService 注入分组模型倍率（批量生图定价快照）。
func (s *BatchImagePublicService) SetGroupModelRateService(rates *GroupModelRateService) {
	if s != nil {
		s.groupModelRates = rates
	}
}

// SetGroupModelRateService 注入分组模型倍率（模型广场展示倍率）。
func (s *ModelPlazaService) SetGroupModelRateService(rates *GroupModelRateService) {
	if s != nil {
		s.groupModelRates = rates
	}
}

// groupModelRateCopier 复制分组时复制模型倍率（窄接口，便于测试）。
type groupModelRateCopier interface {
	CopyGroup(ctx context.Context, fromID, toID int64) error
}

// SetAdminServiceGroupModelRateCopier 让复制分组时一并复制模型倍率。AdminService 不是
// *adminServiceImpl（测试替身）时忽略。
func SetAdminServiceGroupModelRateCopier(admin AdminService, copier groupModelRateCopier) {
	if impl, ok := admin.(*adminServiceImpl); ok && impl != nil {
		impl.groupModelRateCopier = copier
	}
}

// copyGroupModelRatesAfterDuplicate 在复制分组事务提交后复制模型倍率。失败只记 error 日志：
// 不回滚已复制的分组，复制接口的响应不变（管理员可在编辑弹窗里看到并补上规则）。
func (s *adminServiceImpl) copyGroupModelRatesAfterDuplicate(ctx context.Context, sourceID, duplicateID int64) {
	if s == nil || s.groupModelRateCopier == nil {
		return
	}
	if err := s.groupModelRateCopier.CopyGroup(ctx, sourceID, duplicateID); err != nil {
		slog.Error("group_model_rate_copy_on_duplicate_failed", "source_group_id", sourceID, "duplicate_group_id", duplicateID, "error", err)
	}
}

// ProvideGroupModelRateService 构造分组模型倍率服务，并经 setter 注入到各计费 / 展示 / 复制路径。
func ProvideGroupModelRateService(
	repo GroupModelRateRepository,
	gateway *GatewayService,
	openAIGateway *OpenAIGatewayService,
	batchImage *BatchImagePublicService,
	plaza *ModelPlazaService,
	admin AdminService,
) *GroupModelRateService {
	svc := NewGroupModelRateService(repo)
	gateway.SetGroupModelRateService(svc)
	openAIGateway.SetGroupModelRateService(svc)
	batchImage.SetGroupModelRateService(svc)
	plaza.SetGroupModelRateService(svc)
	SetAdminServiceGroupModelRateCopier(admin, svc)
	return svc
}
