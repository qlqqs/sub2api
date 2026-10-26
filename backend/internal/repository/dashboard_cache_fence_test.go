package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDashboardCacheGenerationRejectsInFlightOldWrite(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache, ok := NewDashboardCache(client, nil).(*dashboardCache)
	require.True(t, ok)
	otherNode, ok := NewDashboardCache(client, nil).(*dashboardCache)
	require.True(t, ok)
	ctx := context.Background()
	before, err := cache.DashboardStatsGeneration(ctx)
	require.NoError(t, err)
	require.NoError(t, cache.SetDashboardStatsIfGeneration(ctx, before, "old", time.Minute))
	require.NoError(t, otherNode.InvalidateDashboardStatsGeneration(ctx))
	require.NoError(t, cache.SetDashboardStatsIfGeneration(ctx, before, "stale-in-flight", time.Minute))
	_, err = cache.GetDashboardStats(ctx)
	require.ErrorIs(t, err, service.ErrDashboardStatsCacheMiss)
	after, err := otherNode.DashboardStatsGeneration(ctx)
	require.NoError(t, err)
	require.NotEqual(t, before, after)
	require.NoError(t, otherNode.SetDashboardStatsIfGeneration(ctx, after, "corrected", time.Minute))
	got, err := cache.GetDashboardStats(ctx)
	require.NoError(t, err)
	require.Equal(t, "corrected", got)
	// Ordinary eviction must fence as well; repeated invalidation remains safe.
	require.NoError(t, cache.DeleteDashboardStats(ctx))
	require.NoError(t, cache.DeleteDashboardStats(ctx))
	require.NoError(t, cache.SetDashboardStatsIfGeneration(ctx, after, "stale", time.Minute))
	_, err = cache.GetDashboardStats(ctx)
	require.ErrorIs(t, err, service.ErrDashboardStatsCacheMiss)
}

func TestDashboardCacheGenerationLossNeverReusesOldToken(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	cache, ok := NewDashboardCache(client, nil).(*dashboardCache)
	require.True(t, ok)
	ctx := context.Background()
	old, err := cache.DashboardStatsGeneration(ctx)
	require.NoError(t, err)
	server.FlushAll()
	require.NoError(t, cache.SetDashboardStatsIfGeneration(ctx, old, "stale", time.Minute))
	_, err = cache.GetDashboardStats(ctx)
	require.ErrorIs(t, err, service.ErrDashboardStatsCacheMiss)
	next, err := cache.DashboardStatsGeneration(ctx)
	require.NoError(t, err)
	require.NotEqual(t, old, next)
}
