//go:build unit

package service

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockFinancePricingDeepDetachment(t *testing.T) {
	// The fallback path retains pointers/maps in its live ModelPricing. A shallow
	// struct copy would pass scalar checks but corrupt future previews here.
	live := &ModelPricing{
		InputPricePerToken: 0.001, OutputPricePerToken: 0.002,
		FastMultiplier: testPtrFloat64(3), FlexMultiplier: testPtrFloat64(0.25),
		ReasoningEffortMultipliers: map[string]float64{"high": 2, "low": 1},
		LongContextInputThreshold:  100, LongContextInputMultiplier: 2,
	}
	billing := &BillingService{fallbackPrices: map[string]*ModelPricing{"jev-latest": live}}
	read, release := billing.LockFinancePricing()
	defer release()
	first, err := read("jev-latest")
	require.NoError(t, err)
	require.Equal(t, live, first)
	require.NotSame(t, live, first)
	first.InputPricePerToken = 99
	*first.FastMultiplier = 99
	*first.FlexMultiplier = 99
	first.ReasoningEffortMultipliers["high"] = 99
	delete(first.ReasoningEffortMultipliers, "low")
	second, err := read("jev-latest")
	require.NoError(t, err)
	require.Equal(t, live, second, "each read must detach nested prices from the caller and live generation")
	require.Equal(t, 0.001, live.InputPricePerToken)
	require.Equal(t, 3.0, *live.FastMultiplier)
	require.Equal(t, 0.25, *live.FlexMultiplier)
	require.Equal(t, map[string]float64{"high": 2, "low": 1}, live.ReasoningEffortMultipliers)
	// Independence works in both directions once a DTO has been returned.
	live.ReasoningEffortMultipliers["high"] = 4
	*live.FastMultiplier = 5
	require.Equal(t, 2.0, second.ReasoningEffortMultipliers["high"])
	require.Equal(t, 3.0, *second.FastMultiplier)
}

func TestLockFinancePricingReloadWaitsForReleaseWithoutRecursiveReadDeadlock(t *testing.T) {
	const model = "finance-lock-test-model"
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		model: {InputCostPerToken: 0.001, OutputCostPerToken: 0.002},
	}}
	billing := &BillingService{pricingService: pricing}
	read, unlock := billing.LockFinancePricing()
	var once sync.Once
	release := func() { once.Do(unlock) }
	t.Cleanup(release)
	// Synchronous lock probe proves a read lock really exists, rather than
	// relying on the scheduler not having run the writer yet.
	if pricing.mu.TryLock() {
		pricing.mu.Unlock()
		t.Fatal("finance snapshot did not retain the live pricing read lock")
	}
	file := filepath.Join(t.TempDir(), "prices.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"finance-lock-test-model":{"input_cost_per_token":0.009,"output_cost_per_token":0.018,"litellm_provider":"test","mode":"chat"}}`), 0600))
	started := make(chan struct{})
	reloaded := make(chan error, 1)
	writerExited := make(chan struct{})
	go func() {
		defer close(writerExited)
		close(started)
		reloaded <- pricing.loadPricingData(file)
	}()
	t.Cleanup(func() {
		release()
		select {
		case <-writerExited:
		case <-time.After(5 * time.Second):
			t.Error("pricing reload goroutine did not exit")
		}
	})
	<-started
	// Wait for the real reload to queue on the RWMutex. TryRLock only fails
	// when a writer is waiting/holding; our original reader excludes holding.
	// Gosched is not a time-based assertion. The timeout is solely a deadlock guard.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for pricing.mu.TryRLock() {
		pricing.mu.RUnlock()
		select {
		case err := <-reloaded:
			t.Fatalf("reload finished while finance lock held: %v", err)
		case <-deadline.C:
			t.Fatal("reload did not reach the pricing write lock")
		default:
			runtime.Gosched()
		}
	}
	select {
	case err := <-reloaded:
		t.Fatalf("reload passed an unreleased finance read lock: %v", err)
	default:
	}
	type result struct {
		price *ModelPricing
		err   error
	}
	readDone := make(chan result, 1)
	go func() {
		price, err := read(model)
		readDone <- result{price, err}
	}()
	select {
	case got := <-readDone:
		require.NoError(t, got.err)
		require.NotNil(t, got.price)
		require.Equal(t, 0.001, got.price.InputPricePerToken, "queued reload cannot alter the locked generation")
		got.price.InputPricePerToken = 88
	case <-deadline.C:
		release() // unblock a regressed recursive RLock before failing the test
		select {
		case <-readDone:
		case <-time.After(5 * time.Second):
			t.Fatal("snapshot reader remained blocked after releasing the finance lock")
		}
		t.Fatal("snapshot read deadlocked behind the pending reload")
	}
	release()
	select {
	case err := <-reloaded:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("reload remained blocked after finance released its lock")
	}
	updated, err := billing.GetModelPricing(model)
	require.NoError(t, err)
	require.Equal(t, 0.009, updated.InputPricePerToken)
	readNext, releaseNext := billing.LockFinancePricing()
	defer releaseNext()
	next, err := readNext(model)
	require.NoError(t, err)
	require.Equal(t, 0.009, next.InputPricePerToken, "the next batch must capture the newly loaded generation")
}

func TestLockFinancePricingMissingAndInvalidPrices(t *testing.T) {
	billing := &BillingService{fallbackPrices: map[string]*ModelPricing{
		"jev-latest": {InputPricePerToken: math.NaN()},
	}}
	read, release := billing.LockFinancePricing()
	defer release()
	missing, err := read("finance-unknown-model")
	require.NoError(t, err, "an unavailable price is missing evidence, not a fabricated zero price")
	require.Nil(t, missing)
	invalid, err := read("jev-latest")
	require.Error(t, err, "non-finite prices cannot become durable snapshots")
	require.Nil(t, invalid)
}

func TestFinanceAccountLongContextPolicyOnlineParity(t *testing.T) {
	for _, tc := range []struct {
		name, platform, accountType, model string
		enabled, want                      bool
	}{
		{"api enabled", PlatformOpenAI, AccountTypeAPIKey, "gpt-6.1-sol", true, true},
		{"api disabled", PlatformOpenAI, AccountTypeAPIKey, "gpt-6.1-sol", false, false},
		{"subscription exempt", PlatformOpenAI, AccountTypeOAuth, "gpt-6.1-sol", true, false},
		{"setup token exempt", PlatformOpenAI, AccountTypeSetupToken, "gpt-6.1-sol", true, false},
		{"normalized subscription alias", PlatformOpenAI, AccountTypeOAuth, "openai/GPT_6.1_SOL-max", true, false},
		{"compact subscription alias", PlatformOpenAI, AccountTypeOAuth, "gpt-6.1-sol-openai-compact", true, false},
		{"other subscription model", PlatformOpenAI, AccountTypeOAuth, "gpt-6-astra", true, true},
		{"similar name not exempt", PlatformOpenAI, AccountTypeOAuth, "gpt-6.1-sol-preview", true, true},
		{"subscription switch off", PlatformOpenAI, AccountTypeOAuth, "gpt-6-astra", false, false},
		{"anthropic ignores openai switch", PlatformAnthropic, AccountTypeOAuth, "claude-sonnet-4", false, true},
		{"other platform not exempt", PlatformGrok, AccountTypeOAuth, "gpt-6.1-sol", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{Platform: tc.platform, Type: tc.accountType, Extra: map[string]any{openAILongContextBillingEnabledKey: tc.enabled}}
			online := accountStatsLongContextPricingEnabled(openAILongContextBillingGate(account)) && !openAISubscriptionLongContextExempt(account, &OpenAIForwardResult{UpstreamModel: tc.model})
			got := FinanceAccountLongContextPolicy(tc.platform, tc.accountType, tc.model, tc.enabled)
			require.Equal(t, tc.want, got, "explicit account policy expectation")
			require.Equal(t, online, got, "historical cost must match the online account policy, not the customer group policy")
		})
	}
}

func TestFinanceRecalculationConfigFingerprintStableAndSensitive(t *testing.T) {
	_, snapshot := financeCostFixture()
	snapshot.ModelRates = []AccountModelCostRule{{Model: "upstream-*", RateMultiplier: 2}}
	snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, ReasoningEffortMultipliers: map[string]float64{"high": 2, "low": 1}}
	snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.03)})
	snapshot.Channel.Rules = append(snapshot.Channel.Rules, AccountStatsPricingRule{AccountIDs: []int64{5}, Pricing: []ChannelModelPricing{{Models: []string{"upstream-*"}, BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.05)}}})
	base := FinanceRecalculationConfig{Usage: []FinanceUsageCostSnapshot{snapshot}, Payment: []FinanceRecalculationPaymentRule{{ProviderID: 7, FeeRate: "0.0320"}}}
	encoded, err := json.Marshal(base)
	require.NoError(t, err)
	fingerprint, err := FinanceRecalculationConfigFingerprint(base)
	require.NoError(t, err)
	require.Len(t, fingerprint, 64)
	clone := func() FinanceRecalculationConfig {
		var value FinanceRecalculationConfig
		require.NoError(t, json.Unmarshal(encoded, &value))
		return value
	}
	copy := clone()
	copy.Usage[0].ModelPricing.ReasoningEffortMultipliers = map[string]float64{}
	copy.Usage[0].ModelPricing.ReasoningEffortMultipliers["low"] = 1
	copy.Usage[0].ModelPricing.ReasoningEffortMultipliers["high"] = 2
	same, err := FinanceRecalculationConfigFingerprint(copy)
	require.NoError(t, err)
	require.Equal(t, fingerprint, same, "map insertion order and detached copies must not invalidate previews")
	for _, tc := range []struct {
		name   string
		change func(*FinanceRecalculationConfig)
	}{
		{"default multiplier", func(c *FinanceRecalculationConfig) { c.Usage[0].DefaultMultiplier = "0.5000" }},
		{"model multiplier", func(c *FinanceRecalculationConfig) { c.Usage[0].ModelRates[0].RateMultiplier = 3 }},
		{"model price", func(c *FinanceRecalculationConfig) { c.Usage[0].ModelPricing.InputPricePerToken = 0.002 }},
		{"reasoning price", func(c *FinanceRecalculationConfig) { c.Usage[0].ModelPricing.ReasoningEffortMultipliers["high"] = 3 }},
		{"long context policy", func(c *FinanceRecalculationConfig) { c.Usage[0].LongContextPricingEnabled = false }},
		{"channel price", func(c *FinanceRecalculationConfig) { *c.Usage[0].Channel.Rules[0].Pricing[0].PerRequestPrice = 0.04 }},
		{"rule account scope", func(c *FinanceRecalculationConfig) { c.Usage[0].Channel.Rules[0].AccountIDs[0] = 6 }},
		{"rule priority order", func(c *FinanceRecalculationConfig) {
			rules := c.Usage[0].Channel.Rules
			rules[0], rules[1] = rules[1], rules[0]
		}},
		{"payment fee", func(c *FinanceRecalculationConfig) { c.Payment[0].FeeRate = "0.0400" }},
		{"provider scope", func(c *FinanceRecalculationConfig) { c.Payment[0].ProviderID = 8 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := clone()
			tc.change(&changed)
			got, err := FinanceRecalculationConfigFingerprint(changed)
			require.NoError(t, err)
			require.NotEqual(t, fingerprint, got)
		})
	}
	after, err := json.Marshal(base)
	require.NoError(t, err)
	require.Equal(t, encoded, after, "fingerprinting and detached test edits must not mutate original rules")
}

func TestFinanceUsageCostMissingImageCacheSplitByCostBasis(t *testing.T) {
	for _, basis := range []string{"no channel", "per image", "per request", "custom token", "model token", "preserved customer base"} {
		t.Run(basis, func(t *testing.T) {
			in, snapshot := financeCostFixture()
			in.ImageCacheSplitKnown = false
			in.ImageCount, in.MediaType = 2, "image"
			in.Tokens = UsageTokens{InputTokens: 1000, OutputTokens: 100, ImageInputTokens: 300, CacheReadTokens: 200}
			switch basis {
			case "per image":
				snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: testPtrFloat64(0.03)})
			case "per request":
				snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModePerRequest, PerRequestPrice: testPtrFloat64(0.03)})
			case "custom token":
				snapshot.Channel = financeCostRule(ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(0.001), OutputPrice: testPtrFloat64(0.002)})
			case "model token":
				snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3}
				snapshot.ModelPricing = &ModelPricing{InputPricePerToken: 0.001, OutputPricePerToken: 0.002}
			case "preserved customer base":
				snapshot.Channel = &FinanceCostChannelSnapshot{ID: 3, ApplyPricingToAccountStats: true}
			}
			got, err := CalculateFinanceUsageCost(in, snapshot)
			require.NoError(t, err)
			if basis == "custom token" || basis == "model token" {
				require.Equal(t, FinanceRecalculationSkipped, got.Status)
				require.Equal(t, "missing_image_cache_split", got.Reason)
				require.Empty(t, got.NewCost)
				require.Nil(t, got.NewAccountStatsCost)
				require.Nil(t, got.NewAccountRateMultiplier)
				in.ImageCacheSplitKnown = true
				in.Tokens.ImageCacheReadTokens = 50
				known, err := CalculateFinanceUsageCost(in, snapshot)
				require.NoError(t, err)
				require.Equal(t, FinanceRecalculationChanged, known.Status)
				require.Empty(t, known.Reason)
				return
			}
			require.Equal(t, FinanceRecalculationChanged, got.Status)
			require.Empty(t, got.Reason)
			if basis == "per image" || basis == "per request" {
				require.Equal(t, "0.06", got.NewCost, "real image count is used, not a default count or missing token split")
			} else {
				require.Equal(t, "2.1", got.NewCost, "the preserved original total needs no reconstructed token split")
			}
		})
	}
}
