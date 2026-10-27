package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestMergeBalanceHistoryCodesIncludesAffiliateTransfersByDefault(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	older := now.Add(-2 * time.Hour)
	newer := now.Add(time.Hour)

	usedBy := int64(10)
	redeemCodes := []RedeemCode{
		{
			ID:        1,
			Type:      RedeemTypeBalance,
			Value:     8,
			Status:    StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &now,
			CreatedAt: now,
		},
		{
			ID:        2,
			Type:      RedeemTypeConcurrency,
			Value:     1,
			Status:    StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &older,
			CreatedAt: older,
		},
	}
	affiliateCodes := []RedeemCode{
		{
			ID:        -20,
			Type:      RedeemTypeAffiliateBalance,
			Value:     3.5,
			Status:    StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &newer,
			CreatedAt: newer,
		},
	}

	got := mergeBalanceHistoryCodes(redeemCodes, affiliateCodes, pagination.PaginationParams{
		Page:     1,
		PageSize: 2,
	})

	require.Len(t, got, 2)
	require.Equal(t, RedeemTypeAffiliateBalance, got[0].Type)
	require.Equal(t, RedeemTypeBalance, got[1].Type)
}

func TestMergeBalanceHistoryCodesPaginatesAfterCombiningSources(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	usedBy := int64(10)
	at := func(hours int) *time.Time {
		v := base.Add(time.Duration(hours) * time.Hour)
		return &v
	}

	got := mergeBalanceHistoryCodes(
		[]RedeemCode{
			{ID: 1, Type: RedeemTypeBalance, UsedBy: &usedBy, UsedAt: at(4), CreatedAt: *at(4)},
			{ID: 2, Type: RedeemTypeConcurrency, UsedBy: &usedBy, UsedAt: at(2), CreatedAt: *at(2)},
		},
		[]RedeemCode{
			{ID: -3, Type: RedeemTypeAffiliateBalance, UsedBy: &usedBy, UsedAt: at(3), CreatedAt: *at(3)},
			{ID: -4, Type: RedeemTypeAffiliateBalance, UsedBy: &usedBy, UsedAt: at(1), CreatedAt: *at(1)},
		},
		pagination.PaginationParams{Page: 2, PageSize: 2},
	)

	require.Len(t, got, 2)
	require.Equal(t, RedeemTypeConcurrency, got[0].Type)
	require.Equal(t, int64(-4), got[1].ID)
}

// CUSTOM(fork): Totals are fetched once per request without the list filter/page.
type balanceHistoryTotalsRepo struct {
	RedeemCodeRepository
	calls int
	err   error
}

func (r *balanceHistoryTotalsRepo) GetBalanceHistoryTotals(_ context.Context, userID int64) (BalanceHistoryTotals, error) {
	r.calls++
	return BalanceHistoryTotals{TotalRecharged: 100, TotalGifted: 25}, r.err
}
func (r *balanceHistoryTotalsRepo) ListByUserPaginated(_ context.Context, _ int64, _ pagination.PaginationParams, _ string) ([]RedeemCode, *pagination.PaginationResult, error) {
	return []RedeemCode{}, &pagination.PaginationResult{Total: 0}, nil
}
func TestAdminBalanceHistoryTotalsIgnoreFilterAndPage(t *testing.T) {
	for _, filter := range []string{"", "admin_balance", "balance", "subscription", RedeemTypeAffiliateBalance} {
		for _, page := range []int{1, 2, 10} {
			repo := &balanceHistoryTotalsRepo{}
			svc := &adminServiceImpl{redeemCodeRepo: repo}
			_, _, totals, err := svc.GetUserBalanceHistory(context.Background(), 42, page, 1, filter)
			require.NoError(t, err)
			require.Equal(t, BalanceHistoryTotals{TotalRecharged: 100, TotalGifted: 25}, totals)
			require.Equal(t, 1, repo.calls)
			repo.err = errors.New("totals unavailable")
			_, _, _, err = svc.GetUserBalanceHistory(context.Background(), 42, page, 1, filter)
			require.ErrorIs(t, err, repo.err)
		}
	}
}
