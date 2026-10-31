package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AffiliateGroupRebateFactor 是分组消费的分润百分比；未设置的分组按 100% 计算。
type AffiliateGroupRebateFactor struct {
	GroupID int64   `json:"group_id"`
	Factor  float64 `json:"factor"`
}

type AffiliateGroupRebateFactorView struct {
	AffiliateGroupRebateFactor
	GroupName string `json:"group_name"`
}

func parseAffiliateGroupRebateFactors(raw string) []AffiliateGroupRebateFactor {
	items := make([]AffiliateGroupRebateFactor, 0)
	if raw == "" || json.Unmarshal([]byte(raw), &items) != nil || items == nil {
		return []AffiliateGroupRebateFactor{}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GroupID < items[j].GroupID })
	return items
}

func (s *SettingService) validateAffiliateGroupRebateFactors(ctx context.Context, items []AffiliateGroupRebateFactor) ([]AffiliateGroupRebateFactor, error) {
	out := make([]AffiliateGroupRebateFactor, 0, len(items))
	seen := make(map[int64]bool, len(items))
	for _, item := range items {
		if math.IsNaN(item.Factor) || math.IsInf(item.Factor, 0) || item.Factor < 0 || item.Factor > 100 {
			return nil, infraerrors.BadRequest("AFFILIATE_GROUP_FACTOR_INVALID", "分组分润系数必须在 0–100 之间")
		}
		if seen[item.GroupID] {
			return nil, infraerrors.BadRequest("AFFILIATE_GROUP_FACTOR_DUPLICATE", "分组分润系数的分组不能重复")
		}
		seen[item.GroupID] = true
		group, err := s.defaultSubGroupReader.GetByID(ctx, item.GroupID)
		if errors.Is(err, ErrGroupNotFound) || (err == nil && group == nil) {
			return nil, infraerrors.BadRequest("AFFILIATE_GROUP_FACTOR_GROUP_INVALID", "分组分润系数的分组不存在")
		}
		if err != nil {
			return nil, fmt.Errorf("get affiliate rebate factor group: %w", err)
		}
		item.Factor = roundTo(item.Factor, 2)
		if item.Factor != 100 {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupID < out[j].GroupID })
	return out, nil
}

func (s *SettingService) GetAffiliateGroupRebateFactors(ctx context.Context) []AffiliateGroupRebateFactor {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateGroupRebateFactors)
	if err != nil {
		return []AffiliateGroupRebateFactor{}
	}
	return parseAffiliateGroupRebateFactors(raw)
}

// AffiliateGroupRebateFactorViews 只在设置回显时补充分组名；已删除分组保留条目。
func (s *SettingService) AffiliateGroupRebateFactorViews(ctx context.Context, items []AffiliateGroupRebateFactor) []AffiliateGroupRebateFactorView {
	out := make([]AffiliateGroupRebateFactorView, 0, len(items))
	for _, item := range items {
		view := AffiliateGroupRebateFactorView{AffiliateGroupRebateFactor: item}
		if group, err := s.defaultSubGroupReader.GetByID(ctx, item.GroupID); err == nil && group != nil {
			view.GroupName = group.Name
		}
		out = append(out, view)
	}
	return out
}

func fillAffiliateTeamRebateFactors(items []AffiliateTeamRequest, factors []AffiliateGroupRebateFactor) {
	byGroup := make(map[int64]float64, len(factors))
	for _, factor := range factors {
		byGroup[factor.GroupID] = factor.Factor
	}
	for i := range items {
		items[i].RebateFactor = 100
		if items[i].GroupID != nil {
			if factor, ok := byGroup[*items[i].GroupID]; ok {
				items[i].RebateFactor = factor
			}
		}
	}
}
