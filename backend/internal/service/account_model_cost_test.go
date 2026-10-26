//go:build unit

package service

// CUSTOM(fork): 成本倍率必须与客户扣费解耦，空配置逐位保留原快照。
import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountCostRepoStub struct {
	rules map[int64][]AccountModelCostRule
	err   error
	calls int
}

func (r *accountCostRepoStub) ListAll(context.Context) (map[int64][]AccountModelCostRule, error) {
	r.calls++
	return r.rules, r.err
}
func (r *accountCostRepoStub) ListByAccount(_ context.Context, id int64) ([]AccountModelCostRule, error) {
	return r.rules[id], r.err
}
func (r *accountCostRepoStub) ReplaceForAccount(_ context.Context, id int64, rules []AccountModelCostRule) error {
	if r.err != nil {
		return r.err
	}
	r.rules[id] = rules
	return nil
}
func (r *accountCostRepoStub) AccountExists(context.Context, int64) (bool, error) { return true, r.err }
func TestAccountModelCostMatchAndFallback(t *testing.T) {
	repo := &accountCostRepoStub{rules: map[int64][]AccountModelCostRule{7: {{Model: "up-*", RateMultiplier: 2}, {Model: "up-long-*", RateMultiplier: 3}, {Model: "up-long-exact", RateMultiplier: 0}, {Model: "requested", RateMultiplier: 9}}}}
	svc := NewAccountModelCostService(repo)
	ctx := context.Background()
	rate := 1.23456789
	a := &Account{ID: 7, RateMultiplier: &rate}
	for _, tc := range []struct {
		up, req string
		want    float64
	}{{"UP-other", "requested", 2}, {"up-long-new", "requested", 3}, {"up-long-exact", "requested", 0}, {"unmatched", "requested", rate}, {"", "requested", 9}} {
		require.Equal(t, tc.want, resolveAccountCostMultiplier(ctx, svc, a, tc.up, tc.req))
	}
	require.Equal(t, 1, repo.calls, "热路径只读一次整表")
	_, err := svc.ReplaceForAccount(ctx, 7, []AccountModelCostRule{{Model: "up-*", RateMultiplier: 4}})
	require.NoError(t, err)
	require.Equal(t, 4.0, resolveAccountCostMultiplier(ctx, svc, a, "up-x", ""))
	repo.err = errors.New("数据库不可用")
	svc.Invalidate()
	require.Equal(t, 4.0, resolveAccountCostMultiplier(ctx, svc, a, "up-x", ""), "加载失败沿用旧快照")
	for _, configured := range []*AccountModelCostService{nil, NewAccountModelCostService(&accountCostRepoStub{rules: map[int64][]AccountModelCostRule{}}), NewAccountModelCostService(&accountCostRepoStub{err: errors.New("fail")})} {
		for _, v := range []float64{0, 0.01, 1, 1.23456789, 10000} {
			a.RateMultiplier = &v
			got := resolveAccountCostMultiplier(ctx, configured, a, "upstream", "requested")
			require.Equal(t, math.Float64bits(a.BillingRateMultiplier()), math.Float64bits(got))
		}
	}
}
func TestAccountModelCostValidation(t *testing.T) {
	for _, rules := range [][]AccountModelCostRule{{{Model: "", RateMultiplier: 1}}, {{Model: "*", RateMultiplier: 1}}, {{Model: "a*b", RateMultiplier: 1}}, {{Model: "A", RateMultiplier: 1}, {Model: "a", RateMultiplier: 2}}, {{Model: "a", RateMultiplier: -1}}, {{Model: "a", RateMultiplier: 10001}}, {{Model: "a", RateMultiplier: 0.00001}}, {{Model: "a", RateMultiplier: math.NaN()}}, make([]AccountModelCostRule, 201)} {
		_, err := ValidateAccountModelCostRules(rules)
		require.Error(t, err)
	}
	out, err := ValidateAccountModelCostRules([]AccountModelCostRule{{Model: " ZERO ", RateMultiplier: 0}})
	require.NoError(t, err)
	require.Equal(t, "zero", out[0].Model)
}
func TestAccountModelCostGatewaySnapshotAndBillingUnchanged(t *testing.T) {
	ctx := context.Background()
	group := &Group{ID: 71, Platform: PlatformAnthropic, RateMultiplier: 1}
	rate := 1.23456789
	for _, override := range []bool{false, true} {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		rules := map[int64][]AccountModelCostRule{}
		if override {
			rules[701] = []AccountModelCostRule{{Model: "upstream-cost", RateMultiplier: 0.4}}
		}
		svc.accountModelCosts = NewAccountModelCostService(&accountCostRepoStub{rules: rules})
		input := &RecordUsageInput{Result: &ForwardResult{RequestID: "amc", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 500}, Model: anthropicCheapFixtureModel, UpstreamModel: "upstream-cost", Duration: time.Second}, APIKey: &APIKey{ID: 501, GroupID: &group.ID, Group: group}, User: &User{ID: 601}, Account: &Account{ID: 701, RateMultiplier: &rate}}
		require.NoError(t, svc.RecordUsage(ctx, input))
		require.NotNil(t, repo.lastLog)
		require.Equal(t, repo.lastLog.TotalCost, repo.lastLog.ActualCost, "客户扣费不使用成本倍率")
		expected := rate
		if override {
			expected = 0.4
		}
		require.Equal(t, math.Float64bits(expected), math.Float64bits(*repo.lastLog.AccountRateMultiplier))
	}
}

func TestAccountModelCostOpenAISnapshotAndBillingUnchanged(t *testing.T) {
	ctx := context.Background()
	group := &Group{ID: 81, Platform: PlatformOpenAI, RateMultiplier: 1}
	rate := 1.23456789
	for _, override := range []bool{false, true} {
		repo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIUserGroupRateRepoStub{})
		rules := map[int64][]AccountModelCostRule{}
		if override {
			rules[701] = []AccountModelCostRule{{Model: "upstream-cost", RateMultiplier: 0.4}}
		}
		svc.accountModelCosts = NewAccountModelCostService(&accountCostRepoStub{rules: rules})
		input := &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "amc-openai", Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 500}, Model: openAIPriceyFixtureModel, UpstreamModel: "upstream-cost", Duration: time.Second}, APIKey: &APIKey{ID: 501, GroupID: &group.ID, Group: group}, User: &User{ID: 601}, Account: &Account{ID: 701, Type: AccountTypeAPIKey, RateMultiplier: &rate}}
		require.NoError(t, svc.RecordUsage(ctx, input))
		require.NotNil(t, repo.lastLog)
		require.Equal(t, repo.lastLog.TotalCost, repo.lastLog.ActualCost)
		expected := rate
		if override {
			expected = 0.4
		}
		require.Equal(t, math.Float64bits(expected), math.Float64bits(*repo.lastLog.AccountRateMultiplier))
	}
}
func TestAccountModelCostBatchPriceAndHoldUnchanged(t *testing.T) {
	unit, rate := 0.04, 1.23456789
	group := &Group{ID: 93, Platform: PlatformGemini, RateMultiplier: 1, AllowBatchImageGeneration: true, ImagePrice1K: &unit, BatchImageDiscountMultiplier: 0.5, BatchImageHoldMultiplier: 1}
	svc := &BatchImagePublicService{GroupRepo: batchImageGroupLiteRepoStub{group: group}}
	owner := BatchImageOwner{UserID: 5, GroupID: &group.ID}
	req := BatchImageSubmitRequest{Model: "gemini-image", ImageSize: "1K"}
	a := &Account{ID: 7, RateMultiplier: &rate}
	before, err := svc.resolvePricingSnapshot(context.Background(), owner, req, "vertex", a)
	require.NoError(t, err)
	svc.accountModelCosts = NewAccountModelCostService(&accountCostRepoStub{rules: map[int64][]AccountModelCostRule{}})
	empty, err := svc.resolvePricingSnapshot(context.Background(), owner, req, "vertex", a)
	require.NoError(t, err)
	require.Equal(t, before, empty, "无规则整个快照与旧行为完全一致")
	svc.accountModelCosts = NewAccountModelCostService(&accountCostRepoStub{rules: map[int64][]AccountModelCostRule{7: {{Model: "gemini-image", RateMultiplier: 0}}}})
	after, err := svc.resolvePricingSnapshot(context.Background(), owner, req, "vertex", a)
	require.NoError(t, err)
	require.Zero(t, after.AccountRateMultiplier)
	require.Equal(t, before.BillableUnitPrice, after.BillableUnitPrice)
	require.Equal(t, before.HoldUnitPrice, after.HoldUnitPrice)
}

func TestAccountModelCostMissingUpstreamUsesOriginalRequestModel(t *testing.T) {
	ctx := context.Background()
	rates := NewAccountModelCostService(&accountCostRepoStub{rules: map[int64][]AccountModelCostRule{701: {{Model: "original-request", RateMultiplier: 0.6}}}})
	group := &Group{ID: 71, Platform: PlatformAnthropic, RateMultiplier: 1}
	key := &APIKey{ID: 501, GroupID: &group.ID, Group: group}
	user := &User{ID: 601}
	account := &Account{ID: 701, Type: AccountTypeAPIKey}
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	gateway := newGatewayRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	gateway.accountModelCosts = rates
	err := gateway.RecordUsage(ctx, &RecordUsageInput{Result: &ForwardResult{RequestID: "original-gateway", Model: anthropicCheapFixtureModel, Usage: ClaudeUsage{InputTokens: 1000}}, APIKey: key, User: user, Account: account, ChannelUsageFields: ChannelUsageFields{OriginalModel: "original-request"}})
	require.NoError(t, err)
	require.Equal(t, 0.6, *repo.lastLog.AccountRateMultiplier)
	openai := newOpenAIRecordUsageServiceForTest(repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, &openAIUserGroupRateRepoStub{})
	openai.accountModelCosts = rates
	group.Platform = PlatformOpenAI
	err = openai.RecordUsage(ctx, &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "original-openai", Model: openAIPriceyFixtureModel, Usage: OpenAIUsage{InputTokens: 1000}}, APIKey: key, User: user, Account: account, ChannelUsageFields: ChannelUsageFields{OriginalModel: "original-request"}})
	require.NoError(t, err)
	require.Equal(t, 0.6, *repo.lastLog.AccountRateMultiplier)
}
