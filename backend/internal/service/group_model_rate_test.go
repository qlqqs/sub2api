//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// groupModelRateRepoStub 是 GroupModelRateRepository 的内存实现。
type groupModelRateRepoStub struct {
	mu           sync.Mutex
	rules        map[int64][]GroupModelRateRule
	groups       map[int64]bool
	listAllCalls int
	listAllErr   error
	replaceErr   error
	copyErr      error
	copied       [][2]int64
}

func newGroupModelRateRepoStub(rules map[int64][]GroupModelRateRule) *groupModelRateRepoStub {
	repo := &groupModelRateRepoStub{rules: map[int64][]GroupModelRateRule{}, groups: map[int64]bool{}}
	for groupID, rs := range rules {
		repo.rules[groupID] = append([]GroupModelRateRule(nil), rs...)
		repo.groups[groupID] = true
	}
	return repo
}

func (r *groupModelRateRepoStub) ListAll(context.Context) (map[int64][]GroupModelRateRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listAllCalls++
	if r.listAllErr != nil {
		return nil, r.listAllErr
	}
	out := make(map[int64][]GroupModelRateRule, len(r.rules))
	for groupID, rs := range r.rules {
		if len(rs) > 0 {
			out[groupID] = append([]GroupModelRateRule(nil), rs...)
		}
	}
	return out, nil
}

func (r *groupModelRateRepoStub) ListByGroup(_ context.Context, groupID int64) ([]GroupModelRateRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]GroupModelRateRule{}, r.rules[groupID]...), nil
}

func (r *groupModelRateRepoStub) ReplaceForGroup(_ context.Context, groupID int64, rules []GroupModelRateRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.replaceErr != nil {
		return r.replaceErr
	}
	if !r.groups[groupID] {
		return ErrGroupNotFound
	}
	r.rules[groupID] = append([]GroupModelRateRule(nil), rules...)
	return nil
}

func (r *groupModelRateRepoStub) CopyGroup(_ context.Context, fromID, toID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.copied = append(r.copied, [2]int64{fromID, toID})
	if r.copyErr != nil {
		return r.copyErr
	}
	r.rules[toID] = append([]GroupModelRateRule(nil), r.rules[fromID]...)
	r.groups[toID] = true
	return nil
}

func (r *groupModelRateRepoStub) GroupExists(_ context.Context, groupID int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.groups[groupID], nil
}

func newTestGroupModelRateService(rules map[int64][]GroupModelRateRule) (*GroupModelRateService, *groupModelRateRepoStub) {
	repo := newGroupModelRateRepoStub(rules)
	return NewGroupModelRateService(repo), repo
}

// --- 匹配 ---

func TestGroupModelRateMatch_ExactLongestPrefixAndNormalization(t *testing.T) {
	svc, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{
		7: {
			{Model: "claude-*", RateMultiplier: 1.5},
			{Model: "claude-opus-*", RateMultiplier: 2.0},
			{Model: "claude-opus-4-5", RateMultiplier: 3.0},
			{Model: "gpt-5.5", RateMultiplier: 0.8},
		},
		8: {{Model: "claude-opus-*", RateMultiplier: 9.0}},
	})
	ctx := context.Background()

	cases := []struct {
		model string
		want  float64
		ok    bool
	}{
		{"claude-opus-4-5", 3.0, true},   // 精确优先于通配
		{"Claude-Opus-4.5", 3.0, true},   // 大小写 + claude- 前缀的 . 归一化为 -
		{" claude-opus-4-5 ", 3.0, true}, // 去空格
		{"claude-opus-4-1", 2.0, true},   // 前缀最长的通配
		{"claude-sonnet-4", 1.5, true},   // 较短的通配
		{"GPT-5.5", 0.8, true},           // 非 claude 只做小写
		{"gpt-5-5", 0, false},            // 非 claude 的 . 不替换
		{"gemini-3.7-flash", 0, false},   // 未命中
		{"", 0, false},                   // 空模型
		{"claude", 0, false},             // 通配要求前缀完全匹配
	}
	for _, tc := range cases {
		rate, ok := svc.Match(ctx, 7, tc.model)
		require.Equalf(t, tc.ok, ok, "model %q", tc.model)
		require.InDeltaf(t, tc.want, rate, 1e-12, "model %q", tc.model)
	}

	rate, ok := svc.Match(ctx, 8, "claude-opus-4-5")
	require.True(t, ok, "规则按分组隔离")
	require.InDelta(t, 9.0, rate, 1e-12)
	_, ok = svc.Match(ctx, 9, "claude-opus-4-5")
	require.False(t, ok)
	require.True(t, svc.HasRules(ctx, 7))
	require.False(t, svc.HasRules(ctx, 9))
}

func TestGroupModelRateMatch_NilServiceIsNoop(t *testing.T) {
	var svc *GroupModelRateService
	_, ok := svc.Match(context.Background(), 1, "claude-opus-4-5")
	require.False(t, ok)
	require.False(t, svc.HasRules(context.Background(), 1))
	svc.Invalidate()
	require.NoError(t, svc.CopyGroup(context.Background(), 1, 2))

	unconfigured := NewGroupModelRateService(nil)
	_, ok = unconfigured.Match(context.Background(), 1, "claude-opus-4-5")
	require.False(t, ok)
}

// --- 快照缓存 ---

func TestGroupModelRateSnapshot_CachesWithinTTLAndReloadsAfterInvalidate(t *testing.T) {
	svc, repo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{1: {{Model: "a-*", RateMultiplier: 2}}})
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		rate, ok := svc.Match(ctx, 1, "a-1")
		require.True(t, ok)
		require.InDelta(t, 2.0, rate, 1e-12)
	}
	require.Equal(t, 1, repo.listAllCalls, "TTL 内只加载一次")

	// 直接改库（模拟其他实例写入）：TTL 内仍用旧快照，过期后重新加载。
	repo.rules[1] = []GroupModelRateRule{{Model: "a-*", RateMultiplier: 3}}
	rate, _ := svc.Match(ctx, 1, "a-1")
	require.InDelta(t, 2.0, rate, 1e-12)
	now = now.Add(groupModelRateSnapshotTTL + time.Second)
	rate, _ = svc.Match(ctx, 1, "a-1")
	require.InDelta(t, 3.0, rate, 1e-12)
	require.Equal(t, 2, repo.listAllCalls)

	// 本进程保存后立即生效。
	_, err := svc.ReplaceForGroup(ctx, 1, []GroupModelRateRule{{Model: "a-*", RateMultiplier: 4}})
	require.NoError(t, err)
	rate, _ = svc.Match(ctx, 1, "a-1")
	require.InDelta(t, 4.0, rate, 1e-12)
}

// blockingFirstListAllRepo 让第一次 ListAll 先读出当时的数据，再阻塞到 release 关闭后才返回，
// 用于模拟「失效前发起的慢加载」在失效后的新加载之后才完成。
type blockingFirstListAllRepo struct {
	*groupModelRateRepoStub
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (r *blockingFirstListAllRepo) ListAll(ctx context.Context) (map[int64][]GroupModelRateRule, error) {
	first := false
	r.once.Do(func() { first = true })
	out, err := r.groupModelRateRepoStub.ListAll(ctx)
	if first {
		close(r.started)
		<-r.release
	}
	return out, err
}

// 失效前发起、失效后才完成的旧加载不得覆盖失效后加载到的新快照（否则保存后最长 30 秒仍按旧规则计费）。
func TestGroupModelRateSnapshot_StaleInFlightLoadDoesNotOverwriteNewerSnapshot(t *testing.T) {
	stub := newGroupModelRateRepoStub(map[int64][]GroupModelRateRule{1: {{Model: "a-*", RateMultiplier: 2}}})
	repo := &blockingFirstListAllRepo{groupModelRateRepoStub: stub, started: make(chan struct{}), release: make(chan struct{})}
	svc := NewGroupModelRateService(repo)
	ctx := context.Background()

	staleDone := make(chan float64, 1)
	go func() {
		rate, _ := svc.Match(ctx, 1, "a-1") // 读到旧规则后阻塞
		staleDone <- rate
	}()
	<-repo.started

	_, err := svc.ReplaceForGroup(ctx, 1, []GroupModelRateRule{{Model: "a-*", RateMultiplier: 4}})
	require.NoError(t, err)
	rate, ok := svc.Match(ctx, 1, "a-1")
	require.True(t, ok)
	require.InDelta(t, 4.0, rate, 1e-12, "保存后本进程立即生效")

	close(repo.release)
	require.InDelta(t, 2.0, <-staleDone, 1e-12, "在途的旧请求按它读到的旧规则返回")

	rate, ok = svc.Match(ctx, 1, "a-1")
	require.True(t, ok)
	require.InDelta(t, 4.0, rate, 1e-12, "旧加载完成后不得把快照回退成旧规则")
}

func TestGroupModelRateSnapshot_LoadFailureKeepsPreviousSnapshot(t *testing.T) {
	svc, repo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{1: {{Model: "a-*", RateMultiplier: 2}}})
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	_, ok := svc.Match(ctx, 1, "a-1")
	require.True(t, ok)

	repo.listAllErr = errors.New("db down")
	now = now.Add(groupModelRateSnapshotTTL + time.Second)
	rate, ok := svc.Match(ctx, 1, "a-1")
	require.True(t, ok, "加载失败沿用旧快照")
	require.InDelta(t, 2.0, rate, 1e-12)
	calls := repo.listAllCalls
	_, _ = svc.Match(ctx, 1, "a-1")
	require.Equal(t, calls, repo.listAllCalls, "失败后在重试间隔内不再回源")
}

func TestGroupModelRateSnapshot_NeverLoadedTreatsAsNoRules(t *testing.T) {
	svc, repo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{1: {{Model: "a-*", RateMultiplier: 2}}})
	repo.listAllErr = errors.New("db down")
	_, ok := svc.Match(context.Background(), 1, "a-1")
	require.False(t, ok, "从未加载成功视为无规则（退回原行为）")
}

// --- 校验 ---

func TestValidateGroupModelRateRules(t *testing.T) {
	valid, err := ValidateGroupModelRateRules([]GroupModelRateRule{
		{Model: " Claude-Opus-4.5 ", RateMultiplier: 1.23456},
		{Model: "claude-opus-*", RateMultiplier: 2},
		{Model: "GPT-5.5", RateMultiplier: 10000},
	})
	require.NoError(t, err)
	require.Equal(t, []GroupModelRateRule{
		{Model: "claude-opus-*", RateMultiplier: 2},
		{Model: "claude-opus-4-5", RateMultiplier: 1.2346},
		{Model: "gpt-5.5", RateMultiplier: 10000},
	}, valid, "归一化 + 4 位小数 + 按 model 排序")

	empty, err := ValidateGroupModelRateRules(nil)
	require.NoError(t, err)
	require.Empty(t, empty)

	tooMany := make([]GroupModelRateRule, GroupModelRateMaxRulesPerGroup+1)
	for i := range tooMany {
		tooMany[i] = GroupModelRateRule{Model: "m-" + strings.Repeat("x", i+1), RateMultiplier: 1}
	}

	invalid := []struct {
		name  string
		rules []GroupModelRateRule
		msg   string
	}{
		{"empty model", []GroupModelRateRule{{Model: "  ", RateMultiplier: 1}}, "模型名不能为空"},
		{"leading star", []GroupModelRateRule{{Model: "*opus", RateMultiplier: 1}}, "只能放在末尾"},
		{"middle star", []GroupModelRateRule{{Model: "claude-*-4", RateMultiplier: 1}}, "只能放在末尾"},
		{"double star", []GroupModelRateRule{{Model: "claude**", RateMultiplier: 1}}, "只能放在末尾"},
		{"bare star", []GroupModelRateRule{{Model: "*", RateMultiplier: 1}}, "至少要有一个字符"},
		{"inner whitespace", []GroupModelRateRule{{Model: "claude opus", RateMultiplier: 1}}, "空白字符"},
		{"too long", []GroupModelRateRule{{Model: strings.Repeat("a", 256), RateMultiplier: 1}}, "过长"},
		{"duplicate after normalization", []GroupModelRateRule{
			{Model: "Claude-Opus-4.5", RateMultiplier: 1},
			{Model: "claude-opus-4-5", RateMultiplier: 2},
		}, "重复"},
		{"zero rate", []GroupModelRateRule{{Model: "a", RateMultiplier: 0}}, "须大于 0"},
		{"negative rate", []GroupModelRateRule{{Model: "a", RateMultiplier: -1}}, "须大于 0"},
		{"nan rate", []GroupModelRateRule{{Model: "a", RateMultiplier: math.NaN()}}, "须大于 0"},
		{"inf rate", []GroupModelRateRule{{Model: "a", RateMultiplier: math.Inf(1)}}, "须大于 0"},
		{"too large rate", []GroupModelRateRule{{Model: "a", RateMultiplier: 10000.5}}, "不能超过"},
		{"rounds to zero", []GroupModelRateRule{{Model: "a", RateMultiplier: 0.00001}}, "最多 4 位小数"},
		{"too many rules", tooMany, "最多 200 条"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateGroupModelRateRules(tc.rules)
			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, groupModelRateInvalidReason, infraerrors.Reason(err))
			require.Contains(t, infraerrors.Message(err), tc.msg)
		})
	}
}

func TestGroupModelRateService_ReplaceListAndNotFound(t *testing.T) {
	svc, repo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{1: nil})
	ctx := context.Background()

	saved, err := svc.ReplaceForGroup(ctx, 1, []GroupModelRateRule{
		{Model: "Claude-Opus-*", RateMultiplier: 2},
		{Model: "a", RateMultiplier: 1.5},
	})
	require.NoError(t, err)
	require.Equal(t, []GroupModelRateRule{{Model: "a", RateMultiplier: 1.5}, {Model: "claude-opus-*", RateMultiplier: 2}}, saved)

	listed, err := svc.ListForGroup(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, saved, listed)

	all, err := svc.ListAll(ctx)
	require.NoError(t, err)
	require.Equal(t, map[int64][]GroupModelRateRule{1: saved}, all)

	cleared, err := svc.ReplaceForGroup(ctx, 1, []GroupModelRateRule{})
	require.NoError(t, err)
	require.Empty(t, cleared)
	_, ok := svc.Match(ctx, 1, "a")
	require.False(t, ok, "清空后立即生效")

	_, err = svc.ReplaceForGroup(ctx, 404, []GroupModelRateRule{{Model: "a", RateMultiplier: 1}})
	require.ErrorIs(t, err, ErrGroupNotFound)
	_, err = svc.ListForGroup(ctx, 404)
	require.ErrorIs(t, err, ErrGroupNotFound)

	// 校验先于查分组：非法规则不写库。
	_, err = svc.ReplaceForGroup(ctx, 1, []GroupModelRateRule{{Model: "*", RateMultiplier: 1}})
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Empty(t, repo.rules[1])
}

// --- resolveBaseRate：命中时不调用专属倍率解析 ---

func TestResolveBaseRate_SkipsFallbackWhenModelRateMatches(t *testing.T) {
	svc, _ := newTestGroupModelRateService(map[int64][]GroupModelRateRule{5: {{Model: "x-model", RateMultiplier: 2}}})
	ctx := context.Background()

	calls := 0
	fallback := func() float64 { calls++; return 0.5 }
	require.InDelta(t, 2.0, resolveBaseRate(ctx, svc, 5, "x-model", fallback), 1e-12)
	require.Equal(t, 0, calls, "命中模型倍率时不得调用专属倍率解析")
	require.InDelta(t, 0.5, resolveBaseRate(ctx, svc, 5, "y-model", fallback), 1e-12)
	require.Equal(t, 1, calls)
	require.InDelta(t, 0.5, resolveBaseRate(ctx, nil, 5, "x-model", fallback), 1e-12, "未注入时沿用原逻辑")
}

// --- 复制分组 ---

func TestDuplicateGroupCopiesGroupModelRates(t *testing.T) {
	source := &Group{ID: 31, Name: "rated", Platform: PlatformAnthropic, Status: StatusActive}
	repo := newDuplicateGroupRepoStub(source)
	rates, rateRepo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{31: {{Model: "claude-opus-*", RateMultiplier: 2}}})
	svc := &adminServiceImpl{groupRepo: repo, groupDuplicateRepo: repo}
	SetAdminServiceGroupModelRateCopier(svc, rates)

	duplicate, err := svc.DuplicateGroup(context.Background(), source.ID, "admin:1", "copy-rates")
	require.NoError(t, err)
	require.Equal(t, [][2]int64{{31, duplicate.ID}}, rateRepo.copied)
	rate, ok := rates.Match(context.Background(), duplicate.ID, "claude-opus-4-5")
	require.True(t, ok)
	require.InDelta(t, 2.0, rate, 1e-12)
}

func TestDuplicateGroupModelRateCopyFailureKeepsDuplicate(t *testing.T) {
	source := &Group{ID: 32, Name: "rated", Platform: PlatformAnthropic, Status: StatusActive}
	repo := newDuplicateGroupRepoStub(source)
	rates, rateRepo := newTestGroupModelRateService(map[int64][]GroupModelRateRule{32: {{Model: "a", RateMultiplier: 2}}})
	rateRepo.copyErr = errors.New("copy failed")
	svc := &adminServiceImpl{groupRepo: repo, groupDuplicateRepo: repo}
	SetAdminServiceGroupModelRateCopier(svc, rates)

	duplicate, err := svc.DuplicateGroup(context.Background(), source.ID, "admin:1", "copy-fail")
	require.NoError(t, err, "复制规则失败不影响复制分组的响应")
	require.NotNil(t, duplicate)
	require.Len(t, rateRepo.copied, 1)
}
