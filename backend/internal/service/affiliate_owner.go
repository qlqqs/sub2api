package service

import (
	"context"
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// affiliateChainMaxHops 限制沿邀请链 / 归属链向上查找的步数，防止异常数据导致长时间循环。
const affiliateChainMaxHops = 64

// AffiliateOwnerPreview 是管理员绑定邀请人前预览的分润结构快照。
type AffiliateOwnerPreview struct {
	AgentOwnerID     *int64 `json:"agent_owner_id"`
	AgentOwnerEmail  string `json:"agent_owner_email"`
	AgentOwnerLevel  *int   `json:"agent_owner_level"`
	AgentUplineID    *int64 `json:"agent_upline_id"`
	AgentUplineEmail string `json:"agent_upline_email"`
}

func (s *AffiliateService) loadNode(ctx context.Context, userID int64) (*AffiliateNode, error) {
	nodes, err := s.repo.GetAffiliateNodes(ctx, []int64{userID})
	if err != nil {
		return nil, err
	}
	node, ok := nodes[userID]
	if !ok {
		return nil, nil
	}
	return &node, nil
}

// resolveOwnerSnapshot 按当前各人的等级解析新绑定关系的分润结构（设计文档第 2 节）：
//   - 邀请人是代理 → 归属邀请人；否则沿邀请人的归属链向上找第一个当前仍是代理的人；
//   - 归属代理是二级 → 再沿其归属链向上找第一个当前等级为一级的人作为上方一级。
//
// 邀请链或归属链经过被绑定人本人时返回 ErrAffiliateCycle。
func (s *AffiliateService) resolveOwnerSnapshot(ctx context.Context, inviteeID, inviterID int64) (AffiliateOwnerSnapshot, *AffiliateNode, *AffiliateNode, error) {
	var snapshot AffiliateOwnerSnapshot
	inviter, err := s.loadNode(ctx, inviterID)
	if err != nil {
		return snapshot, nil, nil, err
	}
	if inviter == nil || !inviter.Active {
		return snapshot, nil, nil, ErrUserNotFound
	}

	// 邀请链成环：邀请人的祖先里出现被绑定人。
	visited := map[int64]struct{}{inviterID: {}}
	for next, hops := inviter.InviterID, 0; next != nil && *next > 0 && hops < affiliateChainMaxHops; hops++ {
		if *next == inviteeID {
			return snapshot, nil, nil, ErrAffiliateCycle
		}
		if _, seen := visited[*next]; seen {
			break
		}
		visited[*next] = struct{}{}
		node, err := s.loadNode(ctx, *next)
		if err != nil {
			return snapshot, nil, nil, err
		}
		if node == nil {
			break
		}
		next = node.InviterID
	}

	var owner *AffiliateNode
	if isAgentLevel(inviter.AgentLevel) {
		owner = inviter
	} else {
		owner, err = s.walkOwnerChain(ctx, inviteeID, inviter.AgentOwnerID, isAgentLevel)
		if err != nil {
			return snapshot, nil, nil, err
		}
	}
	if owner == nil {
		return snapshot, nil, nil, nil
	}
	if owner.UserID == inviteeID {
		return snapshot, nil, nil, ErrAffiliateCycle
	}
	ownerID := owner.UserID
	ownerLevel := normalizeAgentLevel(owner.AgentLevel)
	snapshot.OwnerID = &ownerID
	snapshot.OwnerLevel = &ownerLevel
	if ownerLevel != AffiliateAgentLevel2 {
		return snapshot, owner, nil, nil
	}
	upline, err := s.walkOwnerChain(ctx, inviteeID, owner.AgentOwnerID, func(level int) bool {
		return normalizeAgentLevel(level) == AffiliateAgentLevelTop
	})
	if err != nil {
		return snapshot, nil, nil, err
	}
	if upline != nil {
		uplineID := upline.UserID
		snapshot.UplineID = &uplineID
	}
	return snapshot, owner, upline, nil
}

// walkOwnerChain 从 start 沿 agent_owner_id 链向上，返回第一个账号仍存在且等级满足 match 的用户。
// 链上出现被绑定人时返回 ErrAffiliateCycle。
func (s *AffiliateService) walkOwnerChain(ctx context.Context, inviteeID int64, start *int64, match func(level int) bool) (*AffiliateNode, error) {
	visited := map[int64]struct{}{}
	for next, hops := start, 0; next != nil && *next > 0 && hops < affiliateChainMaxHops; hops++ {
		if *next == inviteeID {
			return nil, ErrAffiliateCycle
		}
		if _, seen := visited[*next]; seen {
			return nil, nil
		}
		visited[*next] = struct{}{}
		node, err := s.loadNode(ctx, *next)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, nil
		}
		if node.Active && match(node.AgentLevel) {
			return node, nil
		}
		next = node.AgentOwnerID
	}
	return nil, nil
}

func isAgentLevel(level int) bool {
	l := normalizeAgentLevel(level)
	return l == AffiliateAgentLevelTop || l == AffiliateAgentLevel2
}

// bindInviterWithOwner 在一个事务内解析归属快照并绑定邀请人。
func (s *AffiliateService) bindInviterWithOwner(ctx context.Context, inviteeID, inviterID int64, binding AffiliateInviterBinding) (*AffiliateInviterBindingResult, bool, error) {
	var result *AffiliateInviterBindingResult
	var bound bool
	err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		snapshot, _, _, err := s.resolveOwnerSnapshot(txCtx, inviteeID, inviterID)
		if err != nil {
			return err
		}
		binding.Owner = snapshot
		binding.InviterWasAgent = snapshot.OwnerID != nil && *snapshot.OwnerID == inviterID
		result, bound, err = s.repo.BindInviter(txCtx, inviteeID, inviterID, binding)
		if err == nil && bound && result != nil {
			result.InviterWasAgent = binding.InviterWasAgent
		}
		return err
	})
	if err != nil {
		return nil, false, err
	}
	return result, bound, nil
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return errAffiliateUnavailable()
	}
	// 总开关关闭时，注册阶段静默忽略 aff 参数（不报错，避免阻断注册流程）
	if !s.IsEnabled(ctx) {
		return nil
	}
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.repo.GetAffiliateByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	binding, bound, err := s.bindInviterWithOwner(ctx, userID, inviterSummary.UserID, AffiliateInviterBinding{Source: "registration"})
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	// 此入口只用于注册，且只在本次成功绑定时发放；补绑和重放不补发。
	if amount := s.rewardConfig(ctx).signupBonusFor(binding != nil && binding.InviterWasAgent); amount > 0 {
		if applied, err := s.repo.CreditInviteeSignupBonus(ctx, userID, amount); err != nil {
			logger.FromContext(ctx).Warn("affiliate.signup_bonus_credit_failed", zap.Int64("user_id", userID), zap.Error(err))
		} else if applied {
			s.invalidateAffiliateCaches(ctx, userID)
		}
	}
	s.tryAutoUpgradeAgentLevel(ctx, inviterSummary.UserID)
	return nil
}

// validateAdminBind 校验管理员补绑的参数与被绑定人状态。
func (s *AffiliateService) validateAdminBind(ctx context.Context, inviteeID, inviterID int64) error {
	if s == nil || s.repo == nil {
		return errAffiliateUnavailable()
	}
	if inviteeID <= 0 || inviterID <= 0 {
		return ErrUserNotFound
	}
	if inviteeID == inviterID {
		return ErrAffiliateSelfInvite
	}
	invitee, err := s.repo.EnsureUserAffiliate(ctx, inviteeID)
	if err != nil {
		return err
	}
	if invitee.InviterID != nil || invitee.InviterBoundAt != nil {
		return ErrAffiliateAlreadyBound
	}
	_, err = s.repo.EnsureUserAffiliate(ctx, inviterID)
	return err
}

func (s *AffiliateService) AdminBindInviter(ctx context.Context, inviteeID, inviterID, adminID int64, rawReason string) (*AffiliateInviterBindingResult, error) {
	if s == nil || s.repo == nil {
		return nil, errAffiliateUnavailable()
	}
	if inviteeID <= 0 || inviterID <= 0 || adminID <= 0 {
		return nil, errInvalidUser()
	}
	if inviteeID == inviterID {
		return nil, ErrAffiliateSelfInvite
	}
	reason := strings.TrimSpace(rawReason)
	if len([]rune(reason)) < 1 || len([]rune(reason)) > 500 {
		return nil, ErrAffiliateBindReason
	}
	if err := s.validateAdminBind(ctx, inviteeID, inviterID); err != nil {
		return nil, err
	}

	result, bound, err := s.bindInviterWithOwner(ctx, inviteeID, inviterID, AffiliateInviterBinding{
		Source: "admin", BoundBy: &adminID, Reason: &reason,
	})
	if err != nil {
		return nil, err
	}
	if !bound {
		return nil, ErrAffiliateAlreadyBound
	}
	s.tryAutoUpgradeAgentLevel(ctx, inviterID)
	return result, nil
}

// AdminPreviewInviterBinding 返回绑定后会写入的分润结构快照，校验规则与正式绑定一致。
func (s *AffiliateService) AdminPreviewInviterBinding(ctx context.Context, inviteeID, inviterID int64) (*AffiliateOwnerPreview, error) {
	if err := s.validateAdminBind(ctx, inviteeID, inviterID); err != nil {
		return nil, err
	}
	snapshot, owner, upline, err := s.resolveOwnerSnapshot(ctx, inviteeID, inviterID)
	if err != nil {
		return nil, err
	}
	preview := &AffiliateOwnerPreview{
		AgentOwnerID:    snapshot.OwnerID,
		AgentOwnerLevel: snapshot.OwnerLevel,
		AgentUplineID:   snapshot.UplineID,
	}
	if owner != nil {
		preview.AgentOwnerEmail = owner.Email
	}
	if upline != nil {
		preview.AgentUplineEmail = upline.Email
	}
	return preview, nil
}

func errAffiliateUnavailable() error {
	return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
}

func errInvalidUser() error {
	return infraerrors.BadRequest("INVALID_USER", "invalid user")
}
