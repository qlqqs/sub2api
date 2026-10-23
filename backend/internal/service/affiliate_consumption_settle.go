package service

import (
	"context"
	"errors"
	"math"
	"time"
)

const (
	// affiliateSettleLag 给 usage_logs 的异步写入留出时间，避免结算水位越过尚未落库的记录。
	affiliateSettleLag = 2 * time.Minute
	// affiliateSettleBatchSize 单轮结算最多处理的用户数。
	affiliateSettleBatchSize = 500
)

// AffiliateSettleResult 是一轮消费结算的统计。
type AffiliateSettleResult struct {
	Users       int
	Rebate      float64
	Consumption float64
}

// SettleConsumptionRebates 按余额消费结算返利：对每个有邀请人且产生了新消费的用户，
// 在行锁内汇总水位之后的余额消费，受「真实充值总额 − 已计入消费」上限约束后按
// 两级代理规则入账，并推进水位。基数不是「消费」或功能关闭时直接返回。
// lag 是滞后窗口，只结算这么久以前的消费；非正数时用 affiliateSettleLag。
func (s *AffiliateService) SettleConsumptionRebates(ctx context.Context, lag time.Duration) (*AffiliateSettleResult, error) {
	result := &AffiliateSettleResult{}
	if s == nil || s.repo == nil || s.settingService == nil {
		return result, nil
	}
	if !s.IsEnabled(ctx) || s.rebateBase(ctx) != AffiliateRebateBaseConsumption {
		return result, nil
	}
	if lag <= 0 {
		lag = affiliateSettleLag
	}

	userIDs, err := s.repo.ListUsersWithPendingConsumption(ctx, lag, affiliateSettleBatchSize)
	if err != nil {
		return result, err
	}
	for _, userID := range userIDs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		rebate, consumption, err := s.settleUserConsumption(ctx, userID, lag)
		if err != nil {
			// 单个用户失败不影响其余用户，下一轮会重试；用户被删除时直接跳过。
			if errors.Is(err, ErrUserNotFound) {
				continue
			}
			return result, err
		}
		result.Users++
		result.Rebate = roundTo(result.Rebate+rebate, 8)
		result.Consumption = roundTo(result.Consumption+consumption, 8)
	}
	return result, nil
}

// settleUserConsumption 在一个事务内结算单个用户，返回入账返利与计入返利的消费。
// 无论是否产生返利都推进水位，避免反复扫描同一批记录。
func (s *AffiliateService) settleUserConsumption(ctx context.Context, userID int64, lag time.Duration) (float64, float64, error) {
	var rebate, eligible float64
	err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		pending, err := s.repo.LockPendingConsumption(txCtx, userID, lag)
		if err != nil {
			return err
		}
		if pending == nil || !pending.UntilAt.After(pending.SettledAt) {
			return nil
		}
		eligible = eligibleConsumption(pending)
		if eligible > 0 {
			source, err := s.repo.EnsureUserAffiliate(txCtx, userID)
			if err != nil {
				return err
			}
			until := pending.UntilAt
			rebate, err = s.distributeRewards(txCtx, source, eligible, AffiliateAccrueSource{UsageUntil: &until})
			if err != nil {
				return err
			}
		}
		return s.repo.AdvanceConsumptionWatermark(txCtx, userID, pending.UntilAt, roundTo(pending.SettledConsumption+eligible, 8))
	})
	if err != nil {
		return 0, 0, err
	}
	return rebate, eligible, nil
}

// eligibleConsumption 返回本次可计入返利的消费：新增消费与剩余真实充值额度取小。
func eligibleConsumption(p *AffiliatePendingConsumption) float64 {
	if p == nil || p.DeltaCost <= 0 || math.IsNaN(p.DeltaCost) || math.IsInf(p.DeltaCost, 0) {
		return 0
	}
	remaining := p.PaidTotal - p.SettledConsumption
	if remaining <= 0 || math.IsNaN(remaining) {
		return 0
	}
	return roundTo(math.Min(p.DeltaCost, remaining), 8)
}
