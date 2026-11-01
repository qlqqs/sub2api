package service

// CUSTOM(fork): 全站财务使用整月分成，已确认月份沿用冻结值。
import "context"

func (s *PartnerService) PartnerShares(ctx context.Context, from, to string) (map[string]PartnerShare, error) {
	state, err := s.repo.Read(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]PartnerShare{}
	ids := []int64{}
	for id, p := range state.Partners {
		if p.CreatedAt.In(AffiliateTeamLocation).Format("2006-01") <= to {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	if to > partnerMonthNow() {
		to = partnerMonthNow()
	}
	_, months, err := s.months(ctx, state, ids, to)
	if err != nil {
		return nil, err
	}
	for _, list := range months {
		for _, detail := range list {
			t := detail.Totals
			if t.Month < from || t.Month > to {
				continue
			}
			v := out[t.Month]
			v.Amount = v.Amount.Add(partnerDecimal(t.ShareAmount))
			v.Estimated = v.Estimated || t.Status == "draft" || t.Status == "current"
			out[t.Month] = v
		}
	}
	return out, nil
}
