package service

import "context"

// Invalidate implements the recalculation fence. A nil cache is genuinely
// disabled; an enabled legacy cache without generation fencing is not safe.
func (s *DashboardService) Invalidate(ctx context.Context) error {
	if s.cache == nil {
		return nil
	}
	fence, ok := s.cache.(DashboardStatsCacheFence)
	if !ok {
		return ErrFinanceRecalculationSyncRequired
	}
	return fence.InvalidateDashboardStatsGeneration(ctx)
}
