package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type dashboardFenceCacheStub struct {
	dashboardCacheStub
	generation      string
	generationErr   error
	wroteGeneration string
}

func (c *dashboardFenceCacheStub) DashboardStatsGeneration(context.Context) (string, error) {
	return c.generation, c.generationErr
}
func (c *dashboardFenceCacheStub) SetDashboardStatsIfGeneration(_ context.Context, generation, data string, _ time.Duration) error {
	c.wroteGeneration = generation
	if generation == c.generation {
		c.lastSet = data
	}
	return nil
}
func (c *dashboardFenceCacheStub) InvalidateDashboardStatsGeneration(context.Context) error {
	c.generation = "after"
	c.lastSet = ""
	return nil
}

type dashboardFenceUsageStub struct {
	UsageLogRepository
	onFetch func()
}

func (r *dashboardFenceUsageStub) GetDashboardStats(context.Context) (*usagestats.DashboardStats, error) {
	r.onFetch()
	return &usagestats.DashboardStats{}, nil
}

func TestDashboardRefreshCapturesFenceBeforeDatabaseRead(t *testing.T) {
	cache := &dashboardFenceCacheStub{generation: "before"}
	repo := &dashboardFenceUsageStub{onFetch: func() { require.NoError(t, cache.InvalidateDashboardStatsGeneration(context.Background())) }}
	svc := &DashboardService{usageRepo: repo, cache: cache, aggEnabled: true, cacheTTL: time.Minute, refreshTimeout: time.Second}
	_, err := svc.refreshDashboardStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, "before", cache.wroteGeneration)
	require.Empty(t, cache.lastSet, "old query must not refill the corrected cache")
	require.EqualValues(t, 0, cache.setCalls, "fenced cache must never use unconditional SET")
}
func TestDashboardRefreshFenceFailureDoesNotWriteUnfenced(t *testing.T) {
	cache := &dashboardFenceCacheStub{generationErr: errors.New("redis unavailable")}
	repo := &dashboardFenceUsageStub{onFetch: func() {}}
	svc := &DashboardService{usageRepo: repo, cache: cache, aggEnabled: true, cacheTTL: time.Minute, refreshTimeout: time.Second}
	_, err := svc.refreshDashboardStats(context.Background())
	require.NoError(t, err)
	require.Empty(t, cache.wroteGeneration)
	require.Empty(t, cache.lastSet)
	require.EqualValues(t, 0, cache.setCalls)
}

type dashboardAsyncFenceCacheStub struct {
	dashboardFenceCacheStub
	finished chan struct{}
}

func (c *dashboardAsyncFenceCacheStub) SetDashboardStatsIfGeneration(ctx context.Context, generation, data string, ttl time.Duration) error {
	err := c.dashboardFenceCacheStub.SetDashboardStatsIfGeneration(ctx, generation, data, ttl)
	close(c.finished)
	return err
}
func TestDashboardAsyncRefreshCapturesFenceBeforeDatabaseRead(t *testing.T) {
	cache := &dashboardAsyncFenceCacheStub{dashboardFenceCacheStub: dashboardFenceCacheStub{generation: "before"}, finished: make(chan struct{})}
	repo := &dashboardFenceUsageStub{onFetch: func() { _ = cache.InvalidateDashboardStatsGeneration(context.Background()) }}
	svc := &DashboardService{usageRepo: repo, cache: cache, aggEnabled: true, cacheTTL: time.Minute, refreshTimeout: time.Second}
	svc.refreshDashboardStatsAsync()
	select {
	case <-cache.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("async refresh did not finish")
	}
	require.Equal(t, "before", cache.wroteGeneration)
	require.Empty(t, cache.lastSet)
	require.EqualValues(t, 0, cache.setCalls)
}
