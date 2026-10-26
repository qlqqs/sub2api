package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrFinanceCostConfigChanged = infraerrors.Conflict("FINANCE_COST_CONFIG_CHANGED", "Channel cost configuration changed; reload before saving")

type FinanceChannelCostConfig struct {
	ChannelID                  int64
	Version                    string
	ApplyPricingToAccountStats bool
	AccountStatsPricingRules   []AccountStatsPricingRule
}

type FinanceChannelCostUpdate struct {
	ExpectedVersion            string
	ApplyPricingToAccountStats *bool
	AccountStatsPricingRules   *[]AccountStatsPricingRule
}

// CompareAndSwap must lock the parent channel BEFORE reading the version and
// retain that lock through the cost-only write and commit. Get must return a
// coherent snapshot, not a parent/children mixture from separate transactions.
type FinanceChannelCostRepository interface {
	Get(context.Context, int64) (*FinanceChannelCostConfig, error)
	CompareAndSwap(context.Context, int64, FinanceChannelCostUpdate) (*FinanceChannelCostConfig, error)
}

type FinanceChannelCostService struct {
	repo     FinanceChannelCostRepository
	channels *ChannelService
}

func NewFinanceChannelCostService(repo FinanceChannelCostRepository, channels *ChannelService) *FinanceChannelCostService {
	return &FinanceChannelCostService{repo: repo, channels: channels}
}

func (s *FinanceChannelCostService) Get(ctx context.Context, id int64) (*FinanceChannelCostConfig, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CHANNEL_ID", "Invalid channel ID")
	}
	return s.repo.Get(ctx, id)
}

func (s *FinanceChannelCostService) Update(ctx context.Context, id int64, input FinanceChannelCostUpdate) (*FinanceChannelCostConfig, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_CHANNEL_ID", "Invalid channel ID")
	}
	if strings.TrimSpace(input.ExpectedVersion) == "" {
		return nil, infraerrors.BadRequest("FINANCE_COST_VERSION_REQUIRED", "expected_version is required")
	}
	if input.ApplyPricingToAccountStats == nil && input.AccountStatsPricingRules == nil {
		return nil, infraerrors.BadRequest("FINANCE_COST_CONFIG_INVALID", "At least one cost field is required")
	}
	if input.AccountStatsPricingRules != nil {
		// Validators normalize a few fields; never mutate the caller's draft.
		rules := (&Channel{AccountStatsPricingRules: *input.AccountStatsPricingRules}).Clone().AccountStatsPricingRules
		if rules == nil {
			rules = []AccountStatsPricingRule{}
		}
		for i := range rules {
			rules[i].SortOrder = i // Same first-match array ordering as the original channel API.
			if utf8.RuneCountInString(rules[i].Name) > 100 {
				return nil, infraerrors.BadRequest("FINANCE_COST_CONFIG_INVALID", "Rule name exceeds 100 characters")
			}
			for _, ids := range [][]int64{rules[i].GroupIDs, rules[i].AccountIDs} {
				for _, id := range ids {
					if id <= 0 {
						return nil, infraerrors.BadRequest("FINANCE_COST_CONFIG_INVALID", "Scope IDs must be positive")
					}
				}
			}
			if len(rules[i].GroupIDs) == 0 && len(rules[i].AccountIDs) == 0 {
				return nil, infraerrors.BadRequest("PRICING_RULE_EMPTY_SCOPE", fmt.Sprintf("pricing rule #%d must have at least one group or account", i+1))
			}
			if len(rules[i].Pricing) == 0 {
				return nil, infraerrors.BadRequest("PRICING_RULE_EMPTY_PRICING", fmt.Sprintf("pricing rule #%d must have at least one pricing entry", i+1))
			}
			if rules[i].GroupIDs == nil {
				rules[i].GroupIDs = []int64{}
			}
			if rules[i].AccountIDs == nil {
				rules[i].AccountIDs = []int64{}
			}
			for j := range rules[i].Pricing {
				p := &rules[i].Pricing[j]
				if !p.BillingMode.IsValid() || len(p.Models) == 0 || len(p.Models) > 100 || utf8.RuneCountInString(p.Platform) > 50 {
					return nil, infraerrors.BadRequest("FINANCE_COST_CONFIG_INVALID", "Invalid billing mode or model list")
				}
				if p.BillingMode == "" {
					p.BillingMode = BillingModeToken
				}
				// These fields exist in the shared DTO, but NOT in the cost rule tables.
				// Reject explicitly rather than using the old lossy request conversion.
				if p.FastMultiplier != nil || p.FlexMultiplier != nil || p.ImageInputPrice != nil {
					return nil, unsupportedFinanceCostField()
				}
				for _, iv := range p.Intervals {
					if iv.InputMultiplier != nil || iv.OutputMultiplier != nil || iv.CacheWriteMultiplier != nil || iv.CacheReadMultiplier != nil {
						return nil, unsupportedFinanceCostField()
					}
				}
			}
		}
		if err := validateAccountStatsPricingRules(rules); err != nil {
			return nil, err
		}
		// Also catches NaN/Inf in direct service callers before a database write.
		if _, err := FinanceChannelCostConfigVersion(false, rules); err != nil {
			return nil, infraerrors.BadRequest("FINANCE_COST_CONFIG_INVALID", "Cost prices must be finite numbers")
		}
		input.AccountStatsPricingRules = &rules
	}
	result, err := s.repo.CompareAndSwap(ctx, id, input)
	if err != nil {
		return nil, err
	}
	if s.channels != nil {
		s.channels.invalidateCache()
	}
	return result, nil
}

func unsupportedFinanceCostField() error {
	return infraerrors.BadRequest("FINANCE_COST_FIELD_UNSUPPORTED", "Account statistics rules do not support fast/flex, image input prices or interval multipliers")
}
