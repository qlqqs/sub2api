package service

import (
	"context"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrAffiliateLevel2RateExceedsPool：二级比例不能超过利润池（一级比例）。
var ErrAffiliateLevel2RateExceedsPool = infraerrors.BadRequest("AFFILIATE_LEVEL2_RATE_EXCEEDS_POOL", "level-2 agent rate cannot exceed the profit pool rate")

// AffiliateRewardConfig 汇总两级代理分润、普通用户邀请奖励和被邀请人首充奖励的全部设置。
// 运行时（分润、首充）与设置页读写共用同一套解析和约束，保证两边看到的值一致。
type AffiliateRewardConfig struct {
	// PoolRate 是利润池 P，也是一级代理比例；Level2Rate 是二级比例 R2，始终 ≤ PoolRate。
	PoolRate           float64
	Level2Rate         float64
	CustomRateEnabled  bool
	AgentDurationDays  int
	AgentPerInviteeCap float64
	FreezeHours        int

	AppointEnabled bool
	AppointMax     int

	InviteMode          string
	InviteRate          float64
	InviteDurationDays  int
	InvitePerInviteeCap float64
	InviteFixedAmount   float64
	InviteMinRecharge   float64

	InviteeBonusMode        string
	InviteeBonusValue       float64
	InviteeBonusMinRecharge float64
	InviteeBonusMax         float64
}

var affiliateRewardConfigKeys = []string{
	SettingKeyAffiliateRebateRate,
	SettingKeyAffiliateAgentLevel2Rate,
	SettingKeyAffiliateAgentCustomRateEnabled,
	SettingKeyAffiliateRebateDurationDays,
	SettingKeyAffiliateRebatePerInviteeCap,
	SettingKeyAffiliateRebateFreezeHours,
	SettingKeyAffiliateAgentAppointEnabled,
	SettingKeyAffiliateAgentAppointMax,
	SettingKeyAffiliateInviteRewardMode,
	SettingKeyAffiliateInviteRewardRate,
	SettingKeyAffiliateInviteRewardDurationDays,
	SettingKeyAffiliateInviteRewardPerInviteeCap,
	SettingKeyAffiliateInviteRewardFixedAmount,
	SettingKeyAffiliateInviteRewardMinRecharge,
	SettingKeyAffiliateInviteeBonusMode,
	SettingKeyAffiliateInviteeBonusValue,
	SettingKeyAffiliateInviteeBonusMinRecharge,
	SettingKeyAffiliateInviteeBonusMax,
}

// defaultAffiliateRewardConfig 是没有任何设置时的配置。
func defaultAffiliateRewardConfig() AffiliateRewardConfig {
	return AffiliateRewardConfig{
		PoolRate:           AffiliateRebateRateDefault,
		Level2Rate:         AffiliateAgentLevel2RateDefault,
		CustomRateEnabled:  AffiliateAgentCustomRateEnabledDefault,
		AgentDurationDays:  AffiliateRebateDurationDaysDefault,
		AgentPerInviteeCap: AffiliateRebatePerInviteeCapDefault,
		FreezeHours:        AffiliateRebateFreezeHoursDefault,
		AppointEnabled:     AffiliateAgentAppointEnabledDefault,
		AppointMax:         AffiliateAgentAppointMaxDefault,
		InviteMode:         AffiliateInviteRewardModeOff,
		InviteeBonusMode:   AffiliateInviteeBonusModeOff,
	}.Normalized()
}

// parseAffiliateRewardConfig 从设置键值解析配置；缺失或非法的值回退到默认。
func parseAffiliateRewardConfig(m map[string]string) AffiliateRewardConfig {
	cfg := defaultAffiliateRewardConfig()
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateRebateRate]); ok {
		cfg.PoolRate = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateAgentLevel2Rate]); ok {
		cfg.Level2Rate = v
	}
	if raw := strings.TrimSpace(m[SettingKeyAffiliateAgentCustomRateEnabled]); raw != "" {
		cfg.CustomRateEnabled = raw == "true"
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m[SettingKeyAffiliateRebateDurationDays])); err == nil {
		cfg.AgentDurationDays = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateRebatePerInviteeCap]); ok {
		cfg.AgentPerInviteeCap = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m[SettingKeyAffiliateRebateFreezeHours])); err == nil {
		cfg.FreezeHours = v
	}
	if raw := strings.TrimSpace(m[SettingKeyAffiliateAgentAppointEnabled]); raw != "" {
		cfg.AppointEnabled = !isFalseSettingValue(raw)
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m[SettingKeyAffiliateAgentAppointMax])); err == nil {
		cfg.AppointMax = v
	}
	cfg.InviteMode = m[SettingKeyAffiliateInviteRewardMode]
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteRewardRate]); ok {
		cfg.InviteRate = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(m[SettingKeyAffiliateInviteRewardDurationDays])); err == nil {
		cfg.InviteDurationDays = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteRewardPerInviteeCap]); ok {
		cfg.InvitePerInviteeCap = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteRewardFixedAmount]); ok {
		cfg.InviteFixedAmount = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteRewardMinRecharge]); ok {
		cfg.InviteMinRecharge = v
	}
	cfg.InviteeBonusMode = m[SettingKeyAffiliateInviteeBonusMode]
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteeBonusValue]); ok {
		cfg.InviteeBonusValue = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteeBonusMinRecharge]); ok {
		cfg.InviteeBonusMinRecharge = v
	}
	if v, ok := parseAffiliateFloat(m[SettingKeyAffiliateInviteeBonusMax]); ok {
		cfg.InviteeBonusMax = v
	}
	return cfg.Normalized()
}

func parseAffiliateFloat(raw string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// Normalized 把每个字段收敛到合法范围，并保证 R2 ≤ P。
func (c AffiliateRewardConfig) Normalized() AffiliateRewardConfig {
	c.PoolRate = clampAffiliateRebateRate(c.PoolRate)
	c.Level2Rate = clampAffiliateRebateRate(c.Level2Rate)
	if c.Level2Rate > c.PoolRate {
		c.Level2Rate = c.PoolRate
	}
	c.AgentDurationDays = clampAffiliateIntRange(c.AgentDurationDays, 0, AffiliateRebateDurationDaysMax)
	c.AgentPerInviteeCap = clampAffiliateRewardAmount(c.AgentPerInviteeCap)
	c.FreezeHours = clampAffiliateIntRange(c.FreezeHours, 0, AffiliateRebateFreezeHoursMax)
	c.AppointMax = clampAffiliateIntRange(c.AppointMax, 0, AffiliateAgentAppointMaxMax)

	c.InviteMode = normalizeAffiliateInviteRewardMode(c.InviteMode)
	c.InviteRate = clampAffiliateRebateRate(c.InviteRate)
	c.InviteDurationDays = clampAffiliateIntRange(c.InviteDurationDays, 0, AffiliateRebateDurationDaysMax)
	c.InvitePerInviteeCap = clampAffiliateRewardAmount(c.InvitePerInviteeCap)
	c.InviteFixedAmount = clampAffiliateRewardAmount(c.InviteFixedAmount)
	c.InviteMinRecharge = clampAffiliateRewardAmount(c.InviteMinRecharge)

	c.InviteeBonusMode = normalizeAffiliateInviteeBonusMode(c.InviteeBonusMode)
	if c.InviteeBonusMode == AffiliateInviteeBonusModePercent {
		c.InviteeBonusValue = clampAffiliateRebateRate(c.InviteeBonusValue)
	} else {
		c.InviteeBonusValue = clampAffiliateRewardAmount(c.InviteeBonusValue)
	}
	c.InviteeBonusMinRecharge = clampAffiliateRewardAmount(c.InviteeBonusMinRecharge)
	c.InviteeBonusMax = clampAffiliateRewardAmount(c.InviteeBonusMax)
	return c
}

// settingUpdates 返回本配置新增的设置键值。利润池、有效期、单人上限、冻结期沿用的
// 旧键由 UpdateSettings 原有分支写入。
func (c AffiliateRewardConfig) settingUpdates() map[string]string {
	c = c.Normalized()
	return map[string]string{
		SettingKeyAffiliateAgentLevel2Rate:           formatAffiliateFloat(c.Level2Rate),
		SettingKeyAffiliateAgentCustomRateEnabled:    strconv.FormatBool(c.CustomRateEnabled),
		SettingKeyAffiliateAgentAppointEnabled:       strconv.FormatBool(c.AppointEnabled),
		SettingKeyAffiliateAgentAppointMax:           strconv.Itoa(c.AppointMax),
		SettingKeyAffiliateInviteRewardMode:          c.InviteMode,
		SettingKeyAffiliateInviteRewardRate:          formatAffiliateFloat(c.InviteRate),
		SettingKeyAffiliateInviteRewardDurationDays:  strconv.Itoa(c.InviteDurationDays),
		SettingKeyAffiliateInviteRewardPerInviteeCap: formatAffiliateFloat(c.InvitePerInviteeCap),
		SettingKeyAffiliateInviteRewardFixedAmount:   formatAffiliateFloat(c.InviteFixedAmount),
		SettingKeyAffiliateInviteRewardMinRecharge:   formatAffiliateFloat(c.InviteMinRecharge),
		SettingKeyAffiliateInviteeBonusMode:          c.InviteeBonusMode,
		SettingKeyAffiliateInviteeBonusValue:         formatAffiliateFloat(c.InviteeBonusValue),
		SettingKeyAffiliateInviteeBonusMinRecharge:   formatAffiliateFloat(c.InviteeBonusMinRecharge),
		SettingKeyAffiliateInviteeBonusMax:           formatAffiliateFloat(c.InviteeBonusMax),
	}
}

// affiliateRewardConfigFromSettings 从设置页提交的 SystemSettings 中取出奖励配置。
func affiliateRewardConfigFromSettings(s *SystemSettings) AffiliateRewardConfig {
	return AffiliateRewardConfig{
		PoolRate:                s.AffiliateRebateRate,
		Level2Rate:              s.AffiliateAgentLevel2Rate,
		CustomRateEnabled:       s.AffiliateAgentCustomRateEnabled,
		AgentDurationDays:       s.AffiliateRebateDurationDays,
		AgentPerInviteeCap:      s.AffiliateRebatePerInviteeCap,
		FreezeHours:             s.AffiliateRebateFreezeHours,
		AppointEnabled:          s.AffiliateAgentAppointEnabled,
		AppointMax:              s.AffiliateAgentAppointMax,
		InviteMode:              s.AffiliateInviteRewardMode,
		InviteRate:              s.AffiliateInviteRewardRate,
		InviteDurationDays:      s.AffiliateInviteRewardDurationDays,
		InvitePerInviteeCap:     s.AffiliateInviteRewardPerInviteeCap,
		InviteFixedAmount:       s.AffiliateInviteRewardFixedAmount,
		InviteMinRecharge:       s.AffiliateInviteRewardMinRecharge,
		InviteeBonusMode:        s.AffiliateInviteeBonusMode,
		InviteeBonusValue:       s.AffiliateInviteeBonusValue,
		InviteeBonusMinRecharge: s.AffiliateInviteeBonusMinRecharge,
		InviteeBonusMax:         s.AffiliateInviteeBonusMax,
	}
}

// applyToSettings 把（已归一化的）配置写回 SystemSettings 的对应字段。
func (c AffiliateRewardConfig) applyToSettings(s *SystemSettings) {
	s.AffiliateRebateRate = c.PoolRate
	s.AffiliateAgentLevel2Rate = c.Level2Rate
	s.AffiliateAgentCustomRateEnabled = c.CustomRateEnabled
	s.AffiliateRebateDurationDays = c.AgentDurationDays
	s.AffiliateRebatePerInviteeCap = c.AgentPerInviteeCap
	s.AffiliateRebateFreezeHours = c.FreezeHours
	s.AffiliateAgentAppointEnabled = c.AppointEnabled
	s.AffiliateAgentAppointMax = c.AppointMax
	s.AffiliateInviteRewardMode = c.InviteMode
	s.AffiliateInviteRewardRate = c.InviteRate
	s.AffiliateInviteRewardDurationDays = c.InviteDurationDays
	s.AffiliateInviteRewardPerInviteeCap = c.InvitePerInviteeCap
	s.AffiliateInviteRewardFixedAmount = c.InviteFixedAmount
	s.AffiliateInviteRewardMinRecharge = c.InviteMinRecharge
	s.AffiliateInviteeBonusMode = c.InviteeBonusMode
	s.AffiliateInviteeBonusValue = c.InviteeBonusValue
	s.AffiliateInviteeBonusMinRecharge = c.InviteeBonusMinRecharge
	s.AffiliateInviteeBonusMax = c.InviteeBonusMax
}

// GetAffiliateRewardConfig 一次读取全部奖励设置；读取失败时返回默认配置，
// 与其它返利设置「从不抛错」的约定一致。
func (s *SettingService) GetAffiliateRewardConfig(ctx context.Context) AffiliateRewardConfig {
	if s == nil || s.settingRepo == nil {
		return defaultAffiliateRewardConfig()
	}
	values, err := s.settingRepo.GetMultiple(ctx, affiliateRewardConfigKeys)
	if err != nil {
		return defaultAffiliateRewardConfig()
	}
	return parseAffiliateRewardConfig(values)
}

func normalizeAffiliateInviteRewardMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AffiliateInviteRewardModePercent:
		return AffiliateInviteRewardModePercent
	case AffiliateInviteRewardModeFirstRecharge:
		return AffiliateInviteRewardModeFirstRecharge
	default:
		return AffiliateInviteRewardModeOff
	}
}

func normalizeAffiliateInviteeBonusMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AffiliateInviteeBonusModePercent:
		return AffiliateInviteeBonusModePercent
	case AffiliateInviteeBonusModeFixed:
		return AffiliateInviteeBonusModeFixed
	default:
		return AffiliateInviteeBonusModeOff
	}
}

func clampAffiliateRewardAmount(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	if v > AffiliateRewardAmountMax {
		return AffiliateRewardAmountMax
	}
	return roundTo(v, 8)
}

func clampAffiliateIntRange(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func formatAffiliateFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', 8, 64)
}
