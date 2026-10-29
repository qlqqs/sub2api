package service

import (
	"context"
	"errors"
	"strings"
)

const (
	AffiliateSignupSourceAll   = "all"
	AffiliateSignupSourceAgent = "agent"
	AffiliateSignupSourceUser  = "user"
)

func validAffiliateSignupSource(source string) bool {
	return source == AffiliateSignupSourceAll || source == AffiliateSignupSourceAgent || source == AffiliateSignupSourceUser
}

func normalizeAffiliateSignupSource(source string) string {
	if validAffiliateSignupSource(source) {
		return source
	}
	return AffiliateSignupSourceAll
}

// signupBonusFor is shared by registration, the inviter page and public preview.
func (c AffiliateRewardConfig) signupBonusFor(inviterWasAgent bool) float64 {
	if c.InviteeSignupBonusSource == AffiliateSignupSourceAgent && !inviterWasAgent ||
		c.InviteeSignupBonusSource == AffiliateSignupSourceUser && inviterWasAgent {
		return 0
	}
	return c.InviteeSignupBonus
}

// PreviewInviteeSignupBonus is read-only. It exposes only the applicable amount,
// never the inviter's identity or ownership, and does not reserve a reward.
func (s *AffiliateService) PreviewInviteeSignupBonus(ctx context.Context, rawCode string) (float64, error) {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if s == nil || s.repo == nil || !isValidAffiliateCodeFormat(code) || !s.IsEnabled(ctx) {
		return 0, nil
	}
	cfg := s.rewardConfig(ctx)
	if cfg.InviteeSignupBonus <= 0 {
		return 0, nil
	}
	inviter, err := s.repo.GetAffiliateByCode(ctx, code)
	if errors.Is(err, ErrAffiliateProfileNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if inviter == nil || inviter.UserID <= 0 {
		return 0, nil
	}
	node, err := s.loadNode(ctx, inviter.UserID)
	if err != nil {
		return 0, err
	}
	if node == nil || !node.Active {
		return 0, nil
	}
	return cfg.signupBonusFor(isAgentLevel(node.AgentLevel)), nil
}
