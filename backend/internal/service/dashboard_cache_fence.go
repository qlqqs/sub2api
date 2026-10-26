package service

import (
	"context"
	"time"
)

// DashboardStatsCacheFence is an optional extension for caches shared by multiple
// processes. The token MUST be captured before database reads, not before SET.
// Invalidating rotates the token atomically with eviction, rejecting any query
// that started before the invalidation even if it finishes afterwards.
type DashboardStatsCacheFence interface {
	DashboardStatsGeneration(context.Context) (string, error)
	SetDashboardStatsIfGeneration(context.Context, string, string, time.Duration) error
	InvalidateDashboardStatsGeneration(context.Context) error
}

func (s *DashboardService) dashboardCacheGeneration(ctx context.Context) (string, bool) {
	if fence, ok := s.cache.(DashboardStatsCacheFence); ok {
		generation, err := fence.DashboardStatsGeneration(ctx)
		return generation, err == nil
	}
	return "", true
}
