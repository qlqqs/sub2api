package service

import (
	"context"
	"math"
)

// GetAffiliateDetail 返回用户「我的邀请」页数据。普通用户不下发任何代理比例；代理不下发
// 邀请人奖励（代理邀请的人只产生分润）。
func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	// Lazy thaw: move any matured frozen quota to available before reading.
	if s != nil && s.repo != nil {
		// best-effort: thaw failure is non-fatal
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}

	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	cfg := s.rewardConfig(ctx)
	level := normalizeAgentLevel(summary.AgentLevel)
	detail := &AffiliateDetail{
		UserID:           summary.UserID,
		AffCode:          summary.AffCode,
		InviterID:        summary.InviterID,
		AffCount:         summary.AffCount,
		AffQuota:         summary.AffQuota,
		AffFrozenQuota:   summary.AffFrozenQuota,
		AffHistoryQuota:  summary.AffHistoryQuota,
		RebateBase:       s.rebateBase(ctx),
		AgentLevel:       level,
		AgentLevelLocked: summary.AgentLevelLocked,
		AgentUpgradeMode: s.agentUpgradeMode(ctx),
	}
	if detail.AgentUpgradeMode == AffiliateAgentUpgradeModeAuto && !summary.AgentLevelLocked {
		if next, threshold, ok := s.nextAutoAgentLevel(ctx, level); ok {
			count, err := s.repo.CountQualifiedInvitees(ctx, userID, s.agentCountMode(ctx) == AffiliateAgentCountModeRecharged)
			if err != nil {
				return nil, err
			}
			detail.NextAgentLevel = &next
			detail.NextLevelThreshold = &threshold
			detail.CurrentInviteCount = count
		}
	}

	if isAgentLevel(level) {
		rate := agentTierRate(cfg, level, summary.AffRebateRatePercent)
		detail.AgentRatePercent = &rate
		if level == AffiliateAgentLevelTop {
			team := math.Max(0, rate-cfg.Level2Rate)
			detail.TeamRatePercent = &team
		}
	} else if cfg.InviteMode != AffiliateInviteRewardModeOff {
		detail.InviteReward = &AffiliateInviteRewardInfo{
			Mode:          cfg.InviteMode,
			RatePercent:   cfg.InviteRate,
			DurationDays:  cfg.InviteDurationDays,
			PerInviteeCap: cfg.InvitePerInviteeCap,
			FixedAmount:   cfg.InviteFixedAmount,
			MinRecharge:   cfg.InviteMinRecharge,
		}
	}
	if s.IsEnabled(ctx) {
		detail.InviteeSignupBonus = cfg.InviteeSignupBonus
	}
	if bonus := inviteeBonusInfo(cfg); bonus != nil {
		detail.InviteeBonus = bonus
		if summary.InviterID != nil {
			granted, err := s.repo.HasRewardForSource(ctx, userID, AffiliateRewardInviteeBonus)
			if err != nil {
				return nil, err
			}
			if !granted {
				own := *bonus
				detail.FirstRechargeBonus = &own
			}
		}
	}

	if detail.OwnedCustomerCount, err = s.repo.CountOwnedCustomers(ctx, userID); err != nil {
		return nil, err
	}
	if detail.RewardTotals, err = s.repo.SumRewardTotals(ctx, userID); err != nil {
		return nil, err
	}
	if level == AffiliateAgentLevelTop && cfg.AppointEnabled {
		used, err := s.repo.CountAgentAppointments(ctx, userID)
		if err != nil {
			return nil, err
		}
		detail.Appoint = &AffiliateAppointInfo{Max: cfg.AppointMax, Used: used}
	}
	return detail, nil
}

// inviteeBonusInfo 返回被邀请人首充奖励的展示信息；模式为 off 或数值为 0 时返回 nil。
func inviteeBonusInfo(cfg AffiliateRewardConfig) *AffiliateInviteeBonusInfo {
	if cfg.InviteeBonusMode == AffiliateInviteeBonusModeOff || cfg.InviteeBonusValue <= 0 {
		return nil
	}
	return &AffiliateInviteeBonusInfo{
		Mode:        cfg.InviteeBonusMode,
		Value:       cfg.InviteeBonusValue,
		MinRecharge: cfg.InviteeBonusMinRecharge,
		Max:         cfg.InviteeBonusMax,
	}
}
