package service

// CUSTOM(fork): 账号模型成本倍率；上游模型匹配，未配置时保留原倍率。
// 整表快照 30 秒，失败回退旧快照，保存后失效。

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
	"github.com/shopspring/decimal"
	"golang.org/x/sync/singleflight"
)

const (
	// AccountModelCostMaxRulesPerAccount 单个账号最多的模型倍率规则数。
	AccountModelCostMaxRulesPerAccount = 200
	// AccountModelCostMaxMultiplier 单条规则倍率上限（与 DECIMAL(10,4) 相容）。
	AccountModelCostMaxMultiplier = 10000
	// accountModelCostMaxPatternLen 与 model_pattern VARCHAR(255) 对齐。
	accountModelCostMaxPatternLen = 255

	// accountModelCostSnapshotTTL 与用户专属倍率缓存同样的时效。
	accountModelCostSnapshotTTL = 30 * time.Second
	// accountModelCostLoadRetryInterval 加载失败后的重试间隔，避免故障期间每个请求都回源。
	accountModelCostLoadRetryInterval = 5 * time.Second
	// accountModelCostLoadTimeout 快照加载超时（与请求 ctx 解耦，避免请求取消导致整批等待者失败）。
	accountModelCostLoadTimeout = 5 * time.Second
)

// accountModelCostInvalidReason 模型倍率规则校验失败的 reason（message 为可读原因）。
const accountModelCostInvalidReason = "ACCOUNT_MODEL_COST_INVALID"

// AccountModelCostRule 是一条账号模型倍率规则。Model 为归一化后的模型名，可带末尾 *。
type AccountModelCostRule struct {
	Model          string  `json:"model"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// AccountModelCostRepository 是账号模型倍率的持久化端口（fork 自有表，原生 SQL）。
type AccountModelCostRepository interface {
	// ListAll 返回全部未删除账号的规则（accountID → 规则，按 model 排序），只含有规则的账号。
	ListAll(ctx context.Context) (map[int64][]AccountModelCostRule, error)
	// ListByAccount 返回单个账号的规则（按 model 排序）。
	ListByAccount(ctx context.Context, accountID int64) ([]AccountModelCostRule, error)
	// ReplaceForAccount 在一个事务内整组替换规则；账号不存在（或已软删除）返回 ErrAccountNotFound。
	ReplaceForAccount(ctx context.Context, accountID int64, rules []AccountModelCostRule) error
	// AccountExists 判断账号存在且未软删除。
	AccountExists(ctx context.Context, accountID int64) (bool, error)
}

type accountModelCostWildcard struct {
	prefix string
	rate   float64
}

// accountModelCostIndex 是单个账号的匹配索引：精确规则走 map，通配规则按前缀长度降序。
type accountModelCostIndex struct {
	exact     map[string]float64
	wildcards []accountModelCostWildcard
}

func newAccountModelCostIndex(rules []AccountModelCostRule) *accountModelCostIndex {
	idx := &accountModelCostIndex{exact: make(map[string]float64, len(rules))}
	for _, rule := range rules {
		pattern := normalizeAccountModelCostName(rule.Model)
		if pattern == "" || rule.RateMultiplier < 0 {
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if prefix == "" {
				continue
			}
			idx.wildcards = append(idx.wildcards, accountModelCostWildcard{prefix: prefix, rate: rule.RateMultiplier})
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
func (idx *accountModelCostIndex) match(model string) (float64, bool) {
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

func (idx *accountModelCostIndex) empty() bool {
	return idx == nil || (len(idx.exact) == 0 && len(idx.wildcards) == 0)
}

type accountModelCostSnapshot struct {
	accounts map[int64]*accountModelCostIndex
}

// AccountModelCostService 账号模型倍率：整表快照缓存 + 管理端读写 + 校验。
type AccountModelCostService struct {
	repo AccountModelCostRepository
	ttl  time.Duration
	now  func() time.Time

	mu         sync.RWMutex
	snap       *accountModelCostSnapshot
	nextLoadAt time.Time
	generation uint64

	sf singleflight.Group
}

// NewAccountModelCostService 创建账号模型倍率服务。
func NewAccountModelCostService(repo AccountModelCostRepository) *AccountModelCostService {
	return &AccountModelCostService{
		repo: repo,
		ttl:  accountModelCostSnapshotTTL,
		now:  time.Now,
	}
}

// normalizeAccountModelCostName 与 matchAccountModelPricing 同一归一化口径。
func normalizeAccountModelCostName(model string) string {
	return normalizeChannelPricingModelName(model)
}

// Match 返回 accountID 内 model 命中的模型倍率；未注入、无规则或未命中时返回 (0, false)。
func (s *AccountModelCostService) Match(ctx context.Context, accountID int64, model string) (float64, bool) {
	if s == nil || s.repo == nil || accountID <= 0 {
		return 0, false
	}
	model = normalizeAccountModelCostName(model)
	if model == "" {
		return 0, false
	}
	snap := s.snapshot(ctx)
	if snap == nil {
		return 0, false
	}
	return snap.accounts[accountID].match(model)
}

// HasRules 判断账号是否配置了任何模型倍率（含 0 成本规则）。
func (s *AccountModelCostService) HasRules(ctx context.Context, accountID int64) bool {
	if s == nil || s.repo == nil || accountID <= 0 {
		return false
	}
	snap := s.snapshot(ctx)
	if snap == nil {
		return false
	}
	return !snap.accounts[accountID].empty()
}

// Invalidate 让本进程快照立即失效（保留旧快照作为加载失败时的兜底）。
func (s *AccountModelCostService) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.generation++
	s.nextLoadAt = time.Time{}
	s.mu.Unlock()
}

func (s *AccountModelCostService) snapshot(ctx context.Context) *accountModelCostSnapshot {
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
	if loaded, ok := v.(*accountModelCostSnapshot); ok {
		return loaded
	}
	return snap
}

func (s *AccountModelCostService) load(ctx context.Context, gen uint64) *accountModelCostSnapshot {
	loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), accountModelCostLoadTimeout)
	defer cancel()
	all, err := s.repo.ListAll(loadCtx)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		slog.Warn("account_model_cost_snapshot_load_failed", "error", err, "has_previous", s.snap != nil)
		if s.generation == gen {
			s.nextLoadAt = s.now().Add(accountModelCostLoadRetryInterval)
		}
		return s.snap
	}
	snap := &accountModelCostSnapshot{accounts: make(map[int64]*accountModelCostIndex, len(all))}
	for accountID, rules := range all {
		if idx := newAccountModelCostIndex(rules); !idx.empty() {
			snap.accounts[accountID] = idx
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

// ListAll 返回全部未删除账号的规则（管理端列表页提示用，直接读库）。
func (s *AccountModelCostService) ListAll(ctx context.Context) (map[int64][]AccountModelCostRule, error) {
	if s == nil || s.repo == nil {
		return map[int64][]AccountModelCostRule{}, nil
	}
	all, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account model rates: %w", err)
	}
	if all == nil {
		all = map[int64][]AccountModelCostRule{}
	}
	for accountID, rules := range all {
		all[accountID] = sortedAccountModelCostRules(rules)
	}
	return all, nil
}

// ListForAccount 返回单个账号的规则（按 model 排序）；账号不存在返回 ErrAccountNotFound。
func (s *AccountModelCostService) ListForAccount(ctx context.Context, accountID int64) ([]AccountModelCostRule, error) {
	if s == nil || s.repo == nil {
		return []AccountModelCostRule{}, nil
	}
	if err := s.ensureAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	rules, err := s.repo.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account model rates: %w", err)
	}
	return sortedAccountModelCostRules(rules), nil
}

// ReplaceForAccount 校验并整组替换规则（空切片 = 清空），返回落库后的规则；保存后本进程立即生效。
func (s *AccountModelCostService) ReplaceForAccount(ctx context.Context, accountID int64, rules []AccountModelCostRule) ([]AccountModelCostRule, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("ACCOUNT_MODEL_COST_UNAVAILABLE", "account model rate service is not configured")
	}
	normalized, err := ValidateAccountModelCostRules(rules)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceForAccount(ctx, accountID, normalized); err != nil {
		return nil, fmt.Errorf("replace account model rates: %w", err)
	}
	s.Invalidate()
	saved, err := s.repo.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account model rates: %w", err)
	}
	return sortedAccountModelCostRules(saved), nil
}
func (s *AccountModelCostService) ensureAccountExists(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return ErrAccountNotFound
	}
	exists, err := s.repo.AccountExists(ctx, accountID)
	if err != nil {
		return fmt.Errorf("check account exists: %w", err)
	}
	if !exists {
		return ErrAccountNotFound
	}
	return nil
}

// ValidateAccountModelCostRules 校验并归一化规则：模型名去空格、归一化后非空；* 只允许在末尾
// 且前面至少一个字符；同组不重复（按归一化后比较）；0 ≤ 倍率 ≤ 10000（最多 4 位小数）；
// 单组最多 200 条。返回按 model 排序的归一化规则。
func ValidateAccountModelCostRules(rules []AccountModelCostRule) ([]AccountModelCostRule, error) {
	if len(rules) > AccountModelCostMaxRulesPerAccount {
		return nil, accountModelCostInvalid(fmt.Sprintf("每个账号最多 %d 条模型单独倍率，当前 %d 条", AccountModelCostMaxRulesPerAccount, len(rules)))
	}
	out := make([]AccountModelCostRule, 0, len(rules))
	seen := make(map[string]int, len(rules))
	for i, rule := range rules {
		pos := i + 1
		raw := strings.TrimSpace(rule.Model)
		model := normalizeAccountModelCostName(raw)
		if model == "" {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：模型名不能为空", pos))
		}
		if strings.ContainsAny(model, " \t\r\n") {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：模型名 %q 不能包含空白字符", pos, raw))
		}
		if len(model) > accountModelCostMaxPatternLen {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：模型名过长（最多 %d 个字符）", pos, accountModelCostMaxPatternLen))
		}
		if stars := strings.Count(model, "*"); stars > 0 {
			if stars > 1 || !strings.HasSuffix(model, "*") {
				return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：模型名 %q 的通配符 * 只能放在末尾", pos, raw))
			}
			if len(model) == 1 {
				return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：通配符 * 前至少要有一个字符", pos))
			}
		}
		if prev, dup := seen[model]; dup {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条：模型名 %q 与第 %d 条重复", pos, raw, prev))
		}
		rate := rule.RateMultiplier
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条（%s）：倍率须大于等于 0", pos, raw))
		}
		if rate > AccountModelCostMaxMultiplier {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条（%s）：倍率不能超过 %d", pos, raw, AccountModelCostMaxMultiplier))
		}
		rounded := math.Round(rate*10000) / 10000
		if !decimal.NewFromFloat(rate).Equal(decimal.NewFromFloat(rate).Round(4)) {
			return nil, accountModelCostInvalid(fmt.Sprintf("第 %d 条（%s）：倍率最多 4 位小数", pos, raw))
		}
		seen[model] = pos
		out = append(out, AccountModelCostRule{Model: model, RateMultiplier: rounded})
	}
	return sortedAccountModelCostRules(out), nil
}

func accountModelCostInvalid(message string) error {
	return infraerrors.BadRequest(accountModelCostInvalidReason, message)
}

func sortedAccountModelCostRules(rules []AccountModelCostRule) []AccountModelCostRule {
	out := make([]AccountModelCostRule, len(rules))
	copy(out, rules)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// resolveAccountCostMultiplier 只覆盖成本快照，绝不用于客户扣费或调度。
func resolveAccountCostMultiplier(ctx context.Context, rates *AccountModelCostService, account *Account, upstreamModel, requestedModel string) float64 {
	if account == nil {
		return 1
	}
	model := upstreamModel
	if strings.TrimSpace(model) == "" {
		model = requestedModel
	}
	if rate, ok := rates.Match(ctx, account.ID, model); ok {
		return rate
	}
	return account.BillingRateMultiplier()
}

// ProvideAccountModelCostService 通过 setter 风格注入，不改变上游构造函数签名。
func ProvideAccountModelCostService(repo AccountModelCostRepository, gateway *GatewayService, openAI *OpenAIGatewayService, batch *BatchImagePublicService) *AccountModelCostService {
	s := NewAccountModelCostService(repo)
	gateway.accountModelCosts = s
	openAI.accountModelCosts = s
	batch.accountModelCosts = s
	return s
}
