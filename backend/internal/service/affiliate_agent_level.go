package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var ErrAffiliateAgentLevelInvalid = infraerrors.BadRequest("INVALID_AGENT_LEVEL", "invalid agent level")

// normalizeAgentLevel 把持久化前/后的等级值收敛到 [Top, Max]，0 或越界按默认等级处理。
func normalizeAgentLevel(level int) int {
	if level < AffiliateAgentLevelTop || level > AffiliateAgentLevelMax {
		return AffiliateAgentLevelDefault
	}
	return level
}

// agentLevelAllowedLayers 返回该等级能获得的返利层数：3 级 0 层，2 级 1 层，1 级 2 层。
func agentLevelAllowedLayers(level int) int {
	return AffiliateAgentLevelMax - normalizeAgentLevel(level)
}

// agentLevelAllowsLayer 判断 level 等级的受益人能否获得第 layer 层返利。
func agentLevelAllowsLayer(level, layer int) bool {
	return layer >= 1 && layer <= agentLevelAllowedLayers(level)
}

func validateAgentLevel(level int) error {
	if level < AffiliateAgentLevelTop || level > AffiliateAgentLevelMax {
		return ErrAffiliateAgentLevelInvalid
	}
	return nil
}

func (s *AffiliateService) agentUpgradeMode(ctx context.Context) string {
	if s == nil || s.settingService == nil {
		return AffiliateAgentUpgradeModeDefault
	}
	return s.settingService.GetAffiliateAgentUpgradeMode(ctx)
}

func (s *AffiliateService) agentCountMode(ctx context.Context) string {
	if s == nil || s.settingService == nil {
		return AffiliateAgentCountModeDefault
	}
	return s.settingService.GetAffiliateAgentCountMode(ctx)
}

// nextAutoAgentLevel 返回比 current 高一级且配置了门槛的目标等级及其门槛。
// 门槛为 0 的等级视为不可自动升级，会继续向上找。
func (s *AffiliateService) nextAutoAgentLevel(ctx context.Context, current int) (level int, threshold int, ok bool) {
	if s == nil || s.settingService == nil {
		return 0, 0, false
	}
	for level = normalizeAgentLevel(current) - 1; level >= AffiliateAgentLevelTop; level-- {
		if threshold = s.settingService.GetAffiliateAgentLevelThreshold(ctx, level); threshold > 0 {
			return level, threshold, true
		}
	}
	return 0, 0, false
}

// targetAutoAgentLevel 按邀请人数求出能自动达到的最高等级（门槛为 0 的等级跳过）。
func (s *AffiliateService) targetAutoAgentLevel(ctx context.Context, current, inviteCount int) int {
	target := normalizeAgentLevel(current)
	for level := target - 1; level >= AffiliateAgentLevelTop; level-- {
		threshold := s.settingService.GetAffiliateAgentLevelThreshold(ctx, level)
		if threshold > 0 && inviteCount >= threshold {
			target = level
		}
	}
	return target
}

// EvaluateAgentLevel 在自动升级模式下按邀请人数重新评估一个用户的等级，只升不降；
// 手动锁定的用户不受影响。返回升级后的等级（未升级时返回当前等级）。
func (s *AffiliateService) EvaluateAgentLevel(ctx context.Context, userID int64) (int, bool, error) {
	if s == nil || s.repo == nil || s.settingService == nil {
		return 0, false, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	summary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return 0, false, err
	}
	current := normalizeAgentLevel(summary.AgentLevel)
	if s.agentUpgradeMode(ctx) != AffiliateAgentUpgradeModeAuto || summary.AgentLevelLocked || current == AffiliateAgentLevelTop {
		return current, false, nil
	}
	count, err := s.repo.CountQualifiedInvitees(ctx, userID, s.agentCountMode(ctx) == AffiliateAgentCountModeRecharged)
	if err != nil {
		return current, false, err
	}
	target := s.targetAutoAgentLevel(ctx, current, count)
	if target >= current {
		return current, false, nil
	}
	promoted, err := s.repo.PromoteAgentLevelAuto(ctx, userID, target)
	if err != nil {
		return current, false, err
	}
	if !promoted {
		return current, false, nil
	}
	return target, true, nil
}

// tryAutoUpgradeAgentLevel 在绑定邀请关系成功后尽力评估邀请人等级，失败只记日志。
func (s *AffiliateService) tryAutoUpgradeAgentLevel(ctx context.Context, inviterID int64) {
	if s == nil || s.settingService == nil || s.agentUpgradeMode(ctx) != AffiliateAgentUpgradeModeAuto {
		return
	}
	if _, _, err := s.EvaluateAgentLevel(ctx, inviterID); err != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate] auto agent level evaluation failed for user %d: %v", inviterID, err)
	}
}

// ReevaluateAllAgentLevels 对所有有直属下级的用户按当前门槛重新评估等级（仅升级），
// 返回评估人数与升级人数。
func (s *AffiliateService) ReevaluateAllAgentLevels(ctx context.Context) (evaluated int, promoted int, err error) {
	if s == nil || s.repo == nil || s.settingService == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if s.agentUpgradeMode(ctx) != AffiliateAgentUpgradeModeAuto {
		return 0, 0, nil
	}
	const batch = 500
	var after int64
	for {
		ids, err := s.repo.ListInviterIDs(ctx, after, batch)
		if err != nil {
			return evaluated, promoted, err
		}
		if len(ids) == 0 {
			return evaluated, promoted, nil
		}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return evaluated, promoted, err
			}
			_, up, err := s.EvaluateAgentLevel(ctx, id)
			if err != nil {
				return evaluated, promoted, err
			}
			evaluated++
			if up {
				promoted++
			}
			after = id
		}
		if len(ids) < batch {
			return evaluated, promoted, nil
		}
	}
}

// AdminSetAgentLevel 管理员为一批用户设置代理等级。降级会锁定（不再自动升级），
// 升级不锁定。
func (s *AffiliateService) AdminSetAgentLevel(ctx context.Context, userIDs []int64, level int) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateAgentLevel(level); err != nil {
		return err
	}
	cleaned := cleanUserIDs(userIDs)
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.SetAgentLevelManual(ctx, cleaned, level)
}

// AdminSetAgentLevelLocked 解除或恢复手动锁定，锁定的用户不参与自动升级。
func (s *AffiliateService) AdminSetAgentLevelLocked(ctx context.Context, userIDs []int64, locked bool) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	cleaned := cleanUserIDs(userIDs)
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.SetAgentLevelLocked(ctx, cleaned, locked)
}

func cleanUserIDs(userIDs []int64) []int64 {
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	return cleaned
}
