package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrAffiliateAgentOwnerTargetInvalid = infraerrors.BadRequest("AFFILIATE_AGENT_OWNER_TARGET_INVALID", "only level-2 agents can have a level-1 upline set")
	ErrAffiliateAgentOwnerInvalid       = infraerrors.BadRequest("AFFILIATE_AGENT_OWNER_INVALID", "upline must be an active level-1 agent")
)

// AdminSetAgentOwner 管理员为二级代理设置 / 更换 / 清除上级一级代理（ownerID 为 nil 表示清除）。
//
// 只改该二级自己的归属快照，不改邀请关系；只影响其之后新邀请的客户（新绑定时沿归属链解析出
// 上方一级）和其本人消费的分法，已有客户的快照不变。
func (s *AffiliateService) AdminSetAgentOwner(ctx context.Context, userID int64, ownerID *int64, adminID int64) error {
	if s == nil || s.repo == nil {
		return errAffiliateUnavailable()
	}
	if userID <= 0 || adminID <= 0 {
		return errInvalidUser()
	}
	if ownerID != nil && *ownerID == userID {
		return ErrAffiliateSelfInvite
	}
	return s.repo.WithTx(ctx, func(txCtx context.Context) error {
		target, err := s.repo.GetAffiliateForUpdate(txCtx, userID)
		if err != nil {
			return err
		}
		if target == nil {
			return ErrUserNotFound
		}
		if normalizeAgentLevel(target.AgentLevel) != AffiliateAgentLevel2 {
			return ErrAffiliateAgentOwnerTargetInvalid
		}
		if ownerID != nil {
			owner, err := s.loadNode(txCtx, *ownerID)
			if err != nil {
				return err
			}
			if owner == nil || !owner.Active || normalizeAgentLevel(owner.AgentLevel) != AffiliateAgentLevelTop {
				return ErrAffiliateAgentOwnerInvalid
			}
			// 上级的归属链上出现本人会成环。
			if _, err := s.walkOwnerChain(txCtx, userID, owner.AgentOwnerID, func(int) bool { return false }); err != nil {
				return err
			}
		}
		return s.repo.SetAgentOwnerByAdmin(txCtx, userID, ownerID, adminID)
	})
}
