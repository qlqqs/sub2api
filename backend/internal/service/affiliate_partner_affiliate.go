package service

// CUSTOM(fork): 现有代理接口使用合伙人仓储，避免与利润服务形成循环依赖。
import "context"

func ProvideAffiliateService(repo AffiliateRepository, settings *SettingService, auth APIKeyAuthCacheInvalidator, billing *BillingCacheService, partners PartnerRepository) *AffiliateService {
	s := NewAffiliateService(repo, settings, auth, billing)
	s.partnerRepo = partners
	return s
}
func (s *AffiliateService) partnerInfo(ctx context.Context, id int64) (*PartnerInfo, error) {
	if s.partnerRepo == nil {
		return nil, nil
	}
	state, err := s.partnerRepo.Read(ctx)
	if err != nil {
		return nil, err
	}
	return state.Info(id, partnerMonthNow()), nil
}
func (s *AffiliateService) CheckPartnerLevels(ctx context.Context, ids []int64) error {
	if s.partnerRepo == nil {
		return nil
	}
	state, err := s.partnerRepo.Read(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := state.Partners[id]; ok {
			return ErrPartnerLevelFixed
		}
	}
	return nil
}
func (s *AffiliateService) AdminSetAgentLevelWithAttachments(ctx context.Context, ids []int64, level int, actor int64) ([]PartnerAutoAttachment, error) {
	out := []PartnerAutoAttachment{}
	if s.partnerRepo == nil || level != 1 {
		return out, s.AdminSetAgentLevel(ctx, ids, level, actor)
	}
	err := s.repo.WithTx(ctx, func(ctx context.Context) error {
		before, err := s.partnerRepo.Read(ctx)
		if err != nil {
			return err
		}
		if err := s.AdminSetAgentLevel(ctx, ids, level, actor); err != nil {
			return err
		}
		after, err := s.partnerRepo.Read(ctx)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, existed := before.Attachments[id]; existed {
				continue
			}
			if a, ok := after.Attachments[id]; ok {
				out = append(out, PartnerAutoAttachment{id, after.User(a.PartnerID), after.Partners[a.PartnerID].RegionName})
			}
		}
		return nil
	})
	return out, err
}
