//go:build unit

package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChatModelPrices(t *testing.T) {
	ptr := func(v float64) *float64 { return &v }
	group := PlazaGroup{ID: 3, RateMultiplier: 2, ImageRateIndependent: true, ImageRateMultiplier: 0.5, Models: []PlazaModel{
		{Name: "normal", Pricing: &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: ptr(3e-6), OutputPrice: ptr(15e-6)}},
		{Name: "model-rate", HasModelRate: true, RateMultiplier: 0.25, Pricing: &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: ptr(3e-6)}},
		{Name: "image", Pricing: &ChannelModelPricing{BillingMode: BillingModeImage, PerRequestPrice: ptr(0.125)}},
		{Name: "missing"},
	}}
	result := ChatModelPrices(group, map[int64]float64{3: 0.1})
	require.Equal(t, []ChatModelPrice{{"normal", "输入 ¥0.30 / 百万 tokens · 输出 ¥1.50 / 百万 tokens"}, {"model-rate", "输入 ¥0.75 / 百万 tokens"}, {"image", "¥0.0625 / 张"}}, result)
	result = ChatModelPrices(group, nil)
	require.Equal(t, "输入 ¥6.00 / 百万 tokens · 输出 ¥30.00 / 百万 tokens", result[0].PriceText)
	group.RateMultiplier = 0
	require.Contains(t, ChatModelPrices(group, nil)[0].PriceText, "¥0.00")
}
func TestChatModelPricesIntervalsAndTime(t *testing.T) {
	input, output, twice, request := 3e-6, 15e-6, 2.0, 0.25
	maxTokens := 200000
	g := PlazaGroup{RateMultiplier: 1, Models: []PlazaModel{{Name: "tiered", Pricing: &ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: &input, OutputPrice: &output, Intervals: []PricingInterval{{MaxTokens: &maxTokens}, {MinTokens: 200000, InputMultiplier: &twice}}}, TimePricing: &TimePricingSchedule{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []TimePricingPeriod{{StartTime: "08:00", EndTime: "10:00", Multiplier: 2}}}}, {Name: "image", Pricing: &ChannelModelPricing{BillingMode: BillingModeImage, Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: &request}}}}}}
	out := ChatModelPrices(g, nil)
	require.Len(t, out, 2)
	require.Contains(t, out[0].PriceText, "0–200000 tokens：输入 ¥3.00")
	require.Contains(t, out[0].PriceText, "≥200000 tokens：输入 ¥6.00")
	require.Contains(t, out[0].PriceText, "工作日 08:00–10:00 (Asia/Shanghai)")
	require.NotContains(t, out[0].PriceText, "倍率")
	require.Equal(t, "1K：¥0.25 / 张", out[1].PriceText)
}

func TestChatFormatPriceMatchesPlaza(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{{0, "¥0.00"}, {3, "¥3.00"}, {1.25, "¥1.25"}, {0.00001, "¥0.00001"}, {0.0000001, "¥1e-7"}, {1e12, "¥1000000000000.00"}, {0.1 + 0.2, "¥0.30"}} {
		require.Equal(t, tc.want, chatFormatPrice(tc.value))
	}
}
