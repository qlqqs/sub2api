package service

import (
	"context"
	"math"
	"strings"
	"time"
)

// affiliateShare 是一笔基数上某个受益人应得的比例。
type affiliateShare struct {
	BeneficiaryID int64
	RatePercent   float64
	RewardType    string
}

// agentRewardTypes / inviteRewardTypes 分别是代理利润与普通邀请比例奖励的流水类型，
// 两类各自有单人上限，汇总时互不挤占。
var (
	agentRewardTypes  = []string{AffiliateRewardAgentDirect, AffiliateRewardAgentDiff}
	inviteRewardTypes = []string{AffiliateRewardInvitePercent}
)

// agentTierRate 返回某档代理的比例：专属开关开启且有专属比例时用专属比例，否则用本档默认
// （一级为利润池 P，二级为 R2）。
func agentTierRate(cfg AffiliateRewardConfig, level int, customRate *float64) float64 {
	if cfg.CustomRateEnabled && customRate != nil && !math.IsNaN(*customRate) && !math.IsInf(*customRate, 0) {
		return clampAffiliateRebateRate(*customRate)
	}
	if normalizeAgentLevel(level) == AffiliateAgentLevelTop {
		return cfg.PoolRate
	}
	if normalizeAgentLevel(level) == AffiliateAgentLevel2 {
		return cfg.Level2Rate
	}
	return 0
}

// nodeCustomRate 返回节点的专属比例；节点不存在时为 nil（按本档默认比例）。
func nodeCustomRate(nodes map[int64]AffiliateNode, id int64) *float64 {
	if node, ok := nodes[id]; ok {
		return node.CustomRate
	}
	return nil
}

func nodeActive(nodes map[int64]AffiliateNode, id int64) bool {
	node, ok := nodes[id]
	return ok && node.Active
}

// planAgentShares 按消费者的绑定时快照计算代理利润的分配比例（设计文档第 3 节）。
// 结构只看快照里的档位，不看归属代理 / 上方一级现在的等级；只有账号删除会去掉
// 被删者自己的份额，其他人的份额不变。
func planAgentShares(consumer *AffiliateSummary, nodes map[int64]AffiliateNode, cfg AffiliateRewardConfig) []affiliateShare {
	if consumer == nil || consumer.AgentOwnerID == nil || consumer.AgentOwnerLevel == nil {
		return nil
	}
	ownerID := *consumer.AgentOwnerID
	ownerLevel := *consumer.AgentOwnerLevel
	rate1 := func(id int64) float64 { return agentTierRate(cfg, AffiliateAgentLevelTop, nodeCustomRate(nodes, id)) }
	rate2 := func(id int64) float64 { return agentTierRate(cfg, AffiliateAgentLevel2, nodeCustomRate(nodes, id)) }

	var shares []affiliateShare
	switch normalizeAgentLevel(consumer.AgentLevel) {
	case AffiliateAgentLevelTop:
		// 一级代理本人消费不产生代理利润。
		return nil
	case AffiliateAgentLevel2:
		// 二级代理本人消费视为一级的直属客户，二级本人不拿。
		switch ownerLevel {
		case AffiliateAgentLevelTop:
			shares = append(shares, affiliateShare{ownerID, rate1(ownerID), AffiliateRewardAgentDirect})
		case AffiliateAgentLevel2:
			if consumer.AgentUplineID != nil {
				upline := *consumer.AgentUplineID
				shares = append(shares, affiliateShare{upline, rate1(upline), AffiliateRewardAgentDirect})
			}
		}
	default:
		switch ownerLevel {
		case AffiliateAgentLevelTop:
			shares = append(shares, affiliateShare{ownerID, rate1(ownerID), AffiliateRewardAgentDirect})
		case AffiliateAgentLevel2:
			r2 := rate2(ownerID)
			if consumer.AgentUplineID != nil {
				upline := *consumer.AgentUplineID
				shares = append(shares,
					affiliateShare{ownerID, r2, AffiliateRewardAgentDirect},
					affiliateShare{upline, math.Max(0, rate1(upline)-r2), AffiliateRewardAgentDiff},
				)
			} else {
				// 绑定时上方就没有一级：二级拿满利润池（专属比例更高时按专属）。
				shares = append(shares, affiliateShare{ownerID, math.Max(r2, cfg.PoolRate), AffiliateRewardAgentDirect})
			}
		}
	}

	out := shares[:0]
	for _, share := range shares {
		if share.RatePercent > 0 && share.BeneficiaryID != consumer.UserID && nodeActive(nodes, share.BeneficiaryID) {
			out = append(out, share)
		}
	}
	return out
}

// planInviteShare 计算普通用户邀请比例奖励：仅 percent 模式、且绑定时邀请人是普通用户
// （inviter_was_agent 为 false，绑定时写死）时发放。
func planInviteShare(consumer *AffiliateSummary, nodes map[int64]AffiliateNode, cfg AffiliateRewardConfig) *affiliateShare {
	if !inviterWasNormalUserAtBind(consumer) || cfg.InviteMode != AffiliateInviteRewardModePercent || cfg.InviteRate <= 0 {
		return nil
	}
	inviterID := *consumer.InviterID
	if !nodeActive(nodes, inviterID) {
		return nil
	}
	return &affiliateShare{inviterID, cfg.InviteRate, AffiliateRewardInvitePercent}
}

// inviterWasNormalUserAtBind 判断绑定时邀请人是普通用户（读绑定时写死的 inviter_was_agent，
// 不随管理员改归属而变化）。
func inviterWasNormalUserAtBind(consumer *AffiliateSummary) bool {
	if consumer == nil || consumer.InviterID == nil || *consumer.InviterID <= 0 || *consumer.InviterID == consumer.UserID {
		return false
	}
	return !consumer.InviterWasAgent
}

// bindingStart 返回邀请关系的起算时间（有效期从绑定时起算）；没有邀请人时按管理员设置归属的时间。
func bindingStart(consumer *AffiliateSummary) time.Time {
	if consumer.InviterBoundAt != nil {
		return *consumer.InviterBoundAt
	}
	if consumer.AgentOwnerBoundAt != nil {
		return *consumer.AgentOwnerBoundAt
	}
	return consumer.CreatedAt
}

func withinDuration(consumer *AffiliateSummary, days int, now time.Time) bool {
	return days <= 0 || !now.After(bindingStart(consumer).AddDate(0, 0, days))
}

// distributeRewards 以 base 为基数给消费者 consumer 的代理与普通邀请人入账：
// 代理利润按绑定时快照级差分配，普通邀请比例奖励另算。返回入账合计。
func (s *AffiliateService) distributeRewards(ctx context.Context, consumer *AffiliateSummary, base float64, source AffiliateAccrueSource) (float64, error) {
	// 没有邀请人也可能有管理员设置的归属代理（二级代理挂到一级下）。
	hasInviter := consumer != nil && consumer.InviterID != nil && *consumer.InviterID > 0
	if consumer == nil || (!hasInviter && consumer.AgentOwnerID == nil) {
		return 0, nil
	}
	if base <= 0 || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, nil
	}
	if source.BaseAmount == nil {
		source.BaseAmount = &base
	}
	cfg := s.rewardConfig(ctx)
	now := time.Now()
	agentInDuration := withinDuration(consumer, cfg.AgentDurationDays, now)
	inviteInDuration := cfg.InviteMode == AffiliateInviteRewardModePercent && withinDuration(consumer, cfg.InviteDurationDays, now)
	if !agentInDuration && !inviteInDuration {
		return 0, nil
	}

	ids := make([]int64, 0, 3)
	for _, id := range []*int64{consumer.AgentOwnerID, consumer.AgentUplineID, consumer.InviterID} {
		if id != nil && *id > 0 {
			ids = append(ids, *id)
		}
	}
	nodes, err := s.repo.GetAffiliateNodes(ctx, ids)
	if err != nil {
		return 0, err
	}

	type plannedShare struct {
		affiliateShare
		cap      float64
		capTypes []string
	}
	var planned []plannedShare
	if agentInDuration {
		for _, share := range planAgentShares(consumer, nodes, cfg) {
			planned = append(planned, plannedShare{affiliateShare: share, cap: cfg.AgentPerInviteeCap, capTypes: agentRewardTypes})
		}
	}
	if inviteInDuration {
		if share := planInviteShare(consumer, nodes, cfg); share != nil {
			planned = append(planned, plannedShare{affiliateShare: *share, cap: cfg.InvitePerInviteeCap, capTypes: inviteRewardTypes})
		}
	}

	var total float64
	for _, share := range planned {
		amount := roundTo(base*share.RatePercent/100, 8)
		if amount <= 0 {
			continue
		}
		if share.cap > 0 {
			// 单人上限按「受益人 × 来源用户 × 奖励类别」计算，精确截断到剩余额度。
			existing, err := s.repo.GetAccruedRebateFromInvitee(ctx, share.BeneficiaryID, consumer.UserID, share.capTypes)
			if err != nil {
				return total, err
			}
			if existing >= share.cap {
				continue
			}
			if remaining := roundTo(share.cap-existing, 8); amount > remaining {
				amount = remaining
			}
		}
		applied, err := s.repo.AccrueQuota(ctx, share.BeneficiaryID, consumer.UserID, amount, cfg.FreezeHours, source, share.RewardType)
		if err != nil {
			return total, err
		}
		if applied {
			total = roundTo(total+amount, 8)
		}
	}
	return total, nil
}

// normalizeAffiliateRewardTypeFilter 只接受已知的奖励类型，其它值视为不过滤。
func normalizeAffiliateRewardTypeFilter(value string) string {
	switch v := strings.TrimSpace(value); v {
	case AffiliateRewardAgentDirect, AffiliateRewardAgentDiff, AffiliateRewardInvitePercent,
		AffiliateRewardInviteFixed, AffiliateRewardInviteeBonus:
		return v
	default:
		return ""
	}
}
