package service

import (
	"context"
	"math"
)

// AffiliateFirstRechargeResult 是一笔余额订单触发的首充奖励。
type AffiliateFirstRechargeResult struct {
	InviteeBonus  float64
	InviterReward float64
	InviterID     int64
}

// inviteeBonusAmount 按配置计算被邀请人首充奖励；未达门槛或模式关闭返回 0。
func inviteeBonusAmount(cfg AffiliateRewardConfig, realAmount float64) float64 {
	if realAmount <= 0 || realAmount < cfg.InviteeBonusMinRecharge || cfg.InviteeBonusValue <= 0 {
		return 0
	}
	switch cfg.InviteeBonusMode {
	case AffiliateInviteeBonusModePercent:
		amount := realAmount * cfg.InviteeBonusValue / 100
		if cfg.InviteeBonusMax > 0 {
			amount = math.Min(amount, cfg.InviteeBonusMax)
		}
		return roundTo(amount, 8)
	case AffiliateInviteeBonusModeFixed:
		return roundTo(cfg.InviteeBonusValue, 8)
	default:
		return 0
	}
}

// inviterFixedRewardAmount 计算普通用户邀请人的首充固定奖励；绑定时邀请人是代理时为 0。
func inviterFixedRewardAmount(cfg AffiliateRewardConfig, consumer *AffiliateSummary, realAmount float64) float64 {
	if cfg.InviteMode != AffiliateInviteRewardModeFirstRecharge || cfg.InviteFixedAmount <= 0 {
		return 0
	}
	if realAmount <= 0 || realAmount < cfg.InviteMinRecharge || !inviterWasNormalUserAtBind(consumer) {
		return 0
	}
	return roundTo(cfg.InviteFixedAmount, 8)
}

// ApplyFirstRechargeRewards 在真实支付的余额订单到账后调用（不含管理员充值、兑换码、订阅）。
// realAmount 是到账金额扣除充值赠送后的部分。「首充」指第一笔达到门槛的充值：未达门槛的
// 订单不消耗资格；是否已发以流水为准，并发由唯一索引兜底。被邀请人奖励直接进余额；
// 邀请人固定奖励只在绑定时邀请人是普通用户时发放，进返利额度（受冻结期影响）。
// 邀请人的自动升级不在这里评估，见 EvaluateInviterAfterRecharge。
func (s *AffiliateService) ApplyFirstRechargeRewards(ctx context.Context, userID int64, realAmount float64, sourceOrderID *int64) (*AffiliateFirstRechargeResult, error) {
	result := &AffiliateFirstRechargeResult{}
	if s == nil || s.repo == nil || userID <= 0 || realAmount <= 0 || math.IsNaN(realAmount) || math.IsInf(realAmount, 0) {
		return result, nil
	}
	if !s.IsEnabled(ctx) {
		return result, nil
	}
	consumer, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return result, err
	}
	if consumer.InviterID == nil || *consumer.InviterID <= 0 {
		return result, nil
	}
	inviterID := *consumer.InviterID
	result.InviterID = inviterID
	cfg := s.rewardConfig(ctx)

	if amount := inviteeBonusAmount(cfg, realAmount); amount > 0 {
		applied, err := s.repo.CreditInviteeBonus(ctx, userID, amount, sourceOrderID)
		if err != nil {
			return result, err
		}
		if applied {
			result.InviteeBonus = amount
			s.invalidateAffiliateCaches(ctx, userID)
		}
	}

	if amount := inviterFixedRewardAmount(cfg, consumer, realAmount); amount > 0 {
		nodes, err := s.repo.GetAffiliateNodes(ctx, []int64{inviterID})
		if err != nil {
			return result, err
		}
		if nodeActive(nodes, inviterID) {
			applied, err := s.repo.AccrueQuota(ctx, inviterID, userID, amount, cfg.FreezeHours, AffiliateAccrueSource{OrderID: sourceOrderID, BaseAmount: &realAmount}, AffiliateRewardInviteFixed)
			if err != nil {
				return result, err
			}
			if applied {
				result.InviterReward = amount
			}
		}
	}
	return result, nil
}

// EvaluateInviterAfterRecharge 在余额订单标记为 COMPLETED 之后调用：人数口径为「仅充值过」时
// 评估该用户邀请人的自动升级。必须在订单完成之后调用，否则本单还处于 RECHARGING，
// 不计入合格人数，最后一名达标的直属首充时邀请人不会升级。
func (s *AffiliateService) EvaluateInviterAfterRecharge(ctx context.Context, userID int64) {
	if s == nil || s.repo == nil || userID <= 0 || !s.IsEnabled(ctx) {
		return
	}
	if s.agentCountMode(ctx) != AffiliateAgentCountModeRecharged {
		return
	}
	summary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil || summary.InviterID == nil || *summary.InviterID <= 0 {
		return
	}
	s.tryAutoUpgradeAgentLevel(ctx, *summary.InviterID)
}
