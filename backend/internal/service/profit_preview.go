package service

// 分组利润控制离线预演：使用生产只读导出的分组配置、账号倍率、探测状态、
// 用户覆盖倍率与主力模型清单，复用线上 U/D/epsilon 语义推演准入。
// 探测状态仅用于解释账号倍率来源是否健康，不影响准入结论。

import (
	"math"
	"sort"
	"time"
)

const (
	ProfitPreviewRateSourceManual        = "manual"
	ProfitPreviewRateSourceUpstreamProbe = "upstream_probe_sync"

	ProfitPreviewWarningProbeMissing     = "probe_snapshot_missing"
	ProfitPreviewWarningProbeStale       = "probe_snapshot_stale"
	ProfitPreviewWarningProbeFailed      = "probe_failed"
	ProfitPreviewWarningProbeUnsupported = "probe_unsupported"
	ProfitPreviewWarningManualRateOne    = "manual_rate_1_suspected_unmaintained"
)

// ProfitPreviewGroupInput 是一个分组的预演输入。
type ProfitPreviewGroupInput struct {
	Group *Group
	// Accounts 为该分组绑定且状态可调度的账号，允许 Anthropic/Gemini 分组中
	// 参与 mixed scheduling 的 Antigravity 账号。
	Accounts []*Account
	// UserOverrides 为该分组所有用户覆盖倍率（user_id → 倍率）。
	UserOverrides map[int64]float64
	// Models 为该分组近期主力模型清单。
	Models []string
	// AssumeEnabled 在分组当前关闭时仍按其保存配置启用利润门，供部署前执行
	// “先关组、预演、逐组重开”。
	AssumeEnabled bool
	// CUSTOM: ModelRates 为该分组的模型单独倍率；命中的主力模型按模型倍率单独算 D
	// （用户覆盖倍率不作用于这些模型）。
	ModelRates []GroupModelRateRule
}

// 预演账号分类与线上否决原因一一对应。
const (
	ProfitPreviewClassAdmitted            = "admitted"
	ProfitPreviewClassRejectedThreshold   = "rejected_threshold"
	ProfitPreviewClassRejectedInvalidRate = "rejected_invalid_account_rate"
)

// ProfitPreviewAccountVerdict 是单账号的预演结论。
type ProfitPreviewAccountVerdict struct {
	AccountID   int64    `json:"account_id"`
	Name        string   `json:"name"`
	Platform    string   `json:"platform"`
	Class       string   `json:"class"`
	AccountRate *float64 `json:"account_rate,omitempty"`
	RateSource  string   `json:"rate_source"`
	Warnings    []string `json:"warnings,omitempty"`
	// RejectedUnderMinD 表示账号在最低有效 D（用户覆盖最坏口径）下会被排除。
	RejectedUnderMinD bool `json:"rejected_under_min_d,omitempty"`
	// SupportedModels 为该账号支持的主力模型子集。
	SupportedModels []string `json:"supported_models,omitempty"`
}

// ProfitPreviewGroupReport 是单分组的预演报告。
type ProfitPreviewGroupReport struct {
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	Platform       string `json:"platform"`
	EffectiveGate  bool   `json:"effective_gate"`
	AssumedEnabled bool   `json:"assumed_enabled,omitempty"`
	// DefaultD / MinEffectiveD：分组默认 D 与最低有效 D（含 evalAt 高峰因子；
	// MinEffectiveD = min(分组默认, 全部非空用户覆盖) × 高峰因子）。
	DefaultD      float64 `json:"default_d"`
	MinEffectiveD float64 `json:"min_effective_d"`
	// ThresholdDefault / ThresholdMinD：两档 D 对应的准入阈值。
	ThresholdDefault float64 `json:"threshold_default"`
	ThresholdMinD    float64 `json:"threshold_min_d"`
	// RemainingByModel 仅表示利润门准入账号数，不模拟健康、冷却、限流或槽位。
	RemainingByModel     map[string]int                `json:"profit_admitted_by_model"`
	RemainingByModelMinD map[string]int                `json:"profit_admitted_by_model_min_d"`
	Verdicts             []ProfitPreviewAccountVerdict `json:"verdicts"`
	// CUSTOM: 命中模型单独倍率的主力模型各自的 D 与阈值（含 evalAt 高峰因子）；
	// 这些模型的 RemainingByModel / RemainingByModelMinD 按各自阈值计算。
	ModelD           map[string]float64 `json:"model_d,omitempty"`
	ThresholdByModel map[string]float64 `json:"threshold_by_model,omitempty"`
}

// PreviewProfitAdmission 对五大平台分组推演利润门准入结果。未启用的分组仅在
// AssumeEnabled=true 时按保存配置执行门；Composite 分组始终不直接安装利润门。
func PreviewProfitAdmission(inputs []ProfitPreviewGroupInput, evalAt time.Time) []ProfitPreviewGroupReport {
	reports := make([]ProfitPreviewGroupReport, 0, len(inputs))
	for _, in := range inputs {
		if in.Group == nil {
			continue
		}
		group := in.Group
		effectiveGate := (group.ProfitControlEnabled || in.AssumeEnabled) && profitControlPlatformSupported(group.Platform)
		report := ProfitPreviewGroupReport{
			GroupID:              group.ID,
			GroupName:            group.Name,
			Platform:             group.Platform,
			EffectiveGate:        effectiveGate,
			AssumedEnabled:       in.AssumeEnabled && !group.ProfitControlEnabled && effectiveGate,
			RemainingByModel:     make(map[string]int, len(in.Models)),
			RemainingByModelMinD: make(map[string]int, len(in.Models)),
		}
		for _, model := range in.Models {
			report.RemainingByModel[model] = 0
			report.RemainingByModelMinD[model] = 0
		}

		peak := group.PeakMultiplierAt(evalAt)
		defaultD := group.RateMultiplier * peak
		minRate := group.RateMultiplier
		for _, override := range in.UserOverrides {
			if math.IsNaN(override) || math.IsInf(override, 0) || override < 0 {
				continue
			}
			if override < minRate {
				minRate = override
			}
		}
		minD := minRate * peak
		deduction := group.ProfitMinMargin + group.ProfitSafetyBuffer
		thresholdDefault := clampProfitControlThreshold(defaultD * (1 - deduction))
		thresholdMinD := clampProfitControlThreshold(minD * (1 - deduction))
		report.DefaultD = defaultD
		report.MinEffectiveD = minD
		report.ThresholdDefault = thresholdDefault
		report.ThresholdMinD = thresholdMinD
		modelThresholds := applyProfitPreviewModelRates(&report, in, peak, deduction) // CUSTOM: 按模型分别算 D

		for _, account := range in.Accounts {
			if account == nil {
				continue
			}
			verdict := previewAccountProfitAdmission(account, effectiveGate, thresholdDefault, thresholdMinD, evalAt)
			admittedDefault := verdict.Class == ProfitPreviewClassAdmitted
			admittedMinD := admittedDefault && !verdict.RejectedUnderMinD
			for _, model := range in.Models {
				if !account.IsModelSupported(model) {
					continue
				}
				verdict.SupportedModels = append(verdict.SupportedModels, model)
				modelAdmittedDefault, modelAdmittedMinD := admittedDefault, admittedMinD
				if threshold, ok := modelThresholds[model]; ok { // CUSTOM: 模型单独倍率的阈值
					modelAdmittedDefault = profitPreviewAdmittedAt(account, effectiveGate, threshold)
					modelAdmittedMinD = modelAdmittedDefault
				}
				if modelAdmittedDefault {
					report.RemainingByModel[model]++
				}
				if modelAdmittedMinD {
					report.RemainingByModelMinD[model]++
				}
			}
			sort.Strings(verdict.SupportedModels)
			report.Verdicts = append(report.Verdicts, verdict)
		}
		sort.Slice(report.Verdicts, func(i, j int) bool { return report.Verdicts[i].AccountID < report.Verdicts[j].AccountID })
		reports = append(reports, report)
	}
	return reports
}

func previewAccountProfitAdmission(
	account *Account,
	effectiveGate bool,
	thresholdDefault float64,
	thresholdMinD float64,
	evalAt time.Time,
) ProfitPreviewAccountVerdict {
	verdict := ProfitPreviewAccountVerdict{
		AccountID:  account.ID,
		Name:       account.Name,
		Platform:   account.Platform,
		RateSource: ProfitPreviewRateSourceManual,
	}
	if enabled, _ := account.Extra[UpstreamBillingRateSyncEnabledExtraKey].(bool); enabled {
		verdict.RateSource = ProfitPreviewRateSourceUpstreamProbe
		verdict.Warnings = append(verdict.Warnings, profitPreviewProbeWarnings(account, evalAt)...)
	} else if account.RateMultiplier != nil && *account.RateMultiplier == 1 {
		verdict.Warnings = append(verdict.Warnings, ProfitPreviewWarningManualRateOne)
	}

	validRate := account.RateMultiplier != nil &&
		!math.IsNaN(*account.RateMultiplier) &&
		!math.IsInf(*account.RateMultiplier, 0) &&
		*account.RateMultiplier >= 0
	if validRate {
		rate := *account.RateMultiplier
		verdict.AccountRate = &rate
	}
	switch {
	case !effectiveGate:
		verdict.Class = ProfitPreviewClassAdmitted
	case !validRate:
		verdict.Class = ProfitPreviewClassRejectedInvalidRate
	case profitControlOverThreshold(*account.RateMultiplier, thresholdDefault):
		verdict.Class = ProfitPreviewClassRejectedThreshold
	default:
		verdict.Class = ProfitPreviewClassAdmitted
		verdict.RejectedUnderMinD = profitControlOverThreshold(*account.RateMultiplier, thresholdMinD)
	}
	return verdict
}

func profitPreviewProbeWarnings(account *Account, evalAt time.Time) []string {
	snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra)
	if snapshot == nil {
		return []string{ProfitPreviewWarningProbeMissing}
	}
	switch snapshot.Status {
	case UpstreamBillingProbeStatusFailed:
		return []string{ProfitPreviewWarningProbeFailed}
	case UpstreamBillingProbeStatusUnsupported:
		return []string{ProfitPreviewWarningProbeUnsupported}
	case UpstreamBillingProbeStatusOK:
		if snapshot.FreshUntil == nil || !evalAt.Before(*snapshot.FreshUntil) {
			return []string{ProfitPreviewWarningProbeStale}
		}
	}
	return nil
}

// applyProfitPreviewModelRates (CUSTOM) 为命中模型单独倍率的主力模型计算各自的 D 与阈值，
// 写入报告并返回 model → 阈值。用户覆盖倍率不作用于这些模型，因此最低有效 D 与默认 D 相同。
func applyProfitPreviewModelRates(report *ProfitPreviewGroupReport, in ProfitPreviewGroupInput, peak, deduction float64) map[string]float64 {
	if len(in.ModelRates) == 0 || len(in.Models) == 0 {
		return nil
	}
	idx := newGroupModelRateIndex(in.ModelRates)
	if idx.empty() {
		return nil
	}
	thresholds := make(map[string]float64)
	for _, model := range in.Models {
		rate, ok := idx.match(normalizeGroupModelRateName(model))
		if !ok {
			continue
		}
		if report.ModelD == nil {
			report.ModelD = make(map[string]float64)
			report.ThresholdByModel = make(map[string]float64)
		}
		d := rate * peak
		threshold := clampProfitControlThreshold(d * (1 - deduction))
		report.ModelD[model] = d
		report.ThresholdByModel[model] = threshold
		thresholds[model] = threshold
	}
	return thresholds
}

// profitPreviewAdmittedAt (CUSTOM) 按给定阈值判定账号能否通过利润门（与线上否决点同一语义）。
func profitPreviewAdmittedAt(account *Account, effectiveGate bool, threshold float64) bool {
	if !effectiveGate {
		return true
	}
	if account.RateMultiplier == nil ||
		math.IsNaN(*account.RateMultiplier) ||
		math.IsInf(*account.RateMultiplier, 0) ||
		*account.RateMultiplier < 0 {
		return false
	}
	return !profitControlOverThreshold(*account.RateMultiplier, threshold)
}
