package service

// CUSTOM: compact, actual-charge display prices for the embedded chat picker.
// Mirrors modelPlaza/PlazaModelPricingTable.vue: balance units are ¥, not USD;
// FX only affects the plaza's official-price discount column, never this price.
import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type ChatModelPrice struct {
	ID        string `json:"id"`
	PriceText string `json:"price_text"`
}

// ChatModelReasoning lists the efforts a chat request may send as reasoning_effort,
// lowest first. Default is what the gateway uses when the request omits it.
type ChatModelReasoning struct {
	Levels      []string           `json:"levels"`
	Default     string             `json:"default,omitempty"`
	Multipliers map[string]float64 `json:"multipliers,omitempty"`
}

func ChatModelPrices(g PlazaGroup, userRates map[int64]float64) []ChatModelPrice {
	out := make([]ChatModelPrice, 0, len(g.Models))
	for _, m := range g.Models {
		if m.Pricing == nil {
			continue
		}
		rate := g.RateMultiplier
		if r, ok := userRates[g.ID]; ok {
			rate = r
		}
		if m.HasModelRate {
			rate = m.RateMultiplier
		}
		p := m.Pricing
		if p.BillingMode == BillingModeImage && g.ImageRateIndependent {
			rate = math.Max(0, g.ImageRateMultiplier)
		}
		if p.BillingMode == BillingModeVideo && g.VideoRateIndependent {
			rate = math.Max(0, g.VideoRateMultiplier)
		}
		text := chatPriceText(p, rate)
		if text == "" {
			continue
		}
		if m.TimePricing != nil && len(m.TimePricing.Periods) > 0 {
			text = "标准时段：" + text
			for _, period := range m.TimePricing.Periods {
				prefix := period.StartTime + "–" + period.EndTime + " (" + m.TimePricing.Timezone + ")"
				if m.TimePricing.WeekdaysOnly {
					prefix = "工作日 " + prefix
				}
				text += "；" + prefix + "：" + chatPriceText(p, math.Round(rate*period.Multiplier*1000)/1000)
			}
		}
		out = append(out, ChatModelPrice{ID: m.Name, PriceText: text})
	}
	return out
}

// ChatModelReasoningFor reuses the Codex manifest's per-model effort catalog
// (the same levels the gateway converters accept) and drops levels above the
// group ceiling: they would be downgraded or denied anyway. "none" and
// Codex-only "ultra" are not offered. Fewer than two levels leaves nothing to
// choose. pricing (optional) supplies per-level billing multipliers.
func ChatModelReasoningFor(modelID, maxEffort string, pricing *ChannelModelPricing) *ChatModelReasoning {
	descriptor := newConfiguredCodexModelDescriptor(modelID)
	maxRank, capped := reasoningEffortRank(maxEffort)
	levels := make([]string, 0, len(descriptor.SupportedReasoningLevels))
	for _, level := range descriptor.SupportedReasoningLevels {
		rank, ok := reasoningEffortRank(level.Effort)
		if !ok || (capped && rank > maxRank) {
			continue
		}
		levels = append(levels, level.Effort)
	}
	if len(levels) < 2 {
		return nil
	}
	out := &ChatModelReasoning{Levels: levels}
	if descriptor.DefaultReasoningLevel != nil {
		def := *descriptor.DefaultReasoningLevel
		if rank, ok := reasoningEffortRank(def); ok && capped && rank > maxRank {
			def = levels[len(levels)-1]
		}
		if slices.Contains(levels, def) {
			out.Default = def
		}
	}
	if pricing != nil {
		for _, level := range levels {
			if v, ok := pricing.ReasoningEffortMultipliers[level]; ok && v != 1 {
				if out.Multipliers == nil {
					out.Multipliers = map[string]float64{}
				}
				out.Multipliers[level] = v
			}
		}
	}
	return out
}
func chatPriceText(p *ChannelModelPricing, rate float64) string {
	token := p.BillingMode == "" || p.BillingMode == BillingModeToken
	unit := " / 次"
	if p.BillingMode == BillingModeImage {
		unit = " / 张"
	}
	format := func(input, output, request *float64) string {
		if !token {
			if request == nil {
				return ""
			}
			return chatFormatPrice(*request*rate) + unit
		}
		parts := []string{}
		if input != nil {
			parts = append(parts, "输入 "+chatFormatPrice(*input*rate*1e6)+" / 百万 tokens")
		}
		if output != nil {
			parts = append(parts, "输出 "+chatFormatPrice(*output*rate*1e6)+" / 百万 tokens")
		}
		return strings.Join(parts, " · ")
	}
	if len(p.Intervals) == 0 {
		return format(p.InputPrice, p.OutputPrice, p.PerRequestPrice)
	}
	parts := []string{}
	for _, iv := range p.Intervals {
		text := format(chatIntervalPrice(iv.InputPrice, iv.InputMultiplier, p.InputPrice), chatIntervalPrice(iv.OutputPrice, iv.OutputMultiplier, p.OutputPrice), iv.PerRequestPrice)
		if text == "" {
			continue
		}
		label := iv.TierLabel
		if token && len(p.Intervals) > 1 {
			if iv.MaxTokens != nil {
				label = fmt.Sprintf("%d–%d tokens", iv.MinTokens, *iv.MaxTokens)
			} else {
				label = fmt.Sprintf("≥%d tokens", iv.MinTokens)
			}
		}
		if label != "" {
			text = label + "：" + text
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "；")
}
func chatIntervalPrice(absolute, multiplier, base *float64) *float64 {
	if absolute != nil {
		return absolute
	}
	if base == nil {
		return nil
	}
	v := *base
	if multiplier != nil {
		v *= *multiplier
	}
	return &v
}
func chatFormatPrice(v float64) string {
	// Same 10 significant digits and Number.toString exponent thresholds as the
	// plaza's formatScaled; avoid Go's different %g scientific-notation cutoffs.
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 10, 64), 64)
	if err != nil || math.IsNaN(rounded) || math.IsInf(rounded, 0) {
		return "-"
	}
	abs := math.Abs(rounded)
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		s := strconv.FormatFloat(rounded, 'e', -1, 64)
		s = strings.ReplaceAll(strings.ReplaceAll(s, "e-0", "e-"), "e+0", "e+")
		return "¥" + s
	}
	s := strconv.FormatFloat(rounded, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".00"
	} else if len(s)-strings.IndexByte(s, '.') == 2 {
		s += "0"
	}
	return "¥" + s
}
