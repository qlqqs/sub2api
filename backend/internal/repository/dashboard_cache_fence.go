package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.DashboardStatsCacheFence = (*dashboardCache)(nil)

func dashboardGenerationToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

// Random generations (rather than an INCR starting at zero) also prevent ABA
// after eviction/restart. Generation loss makes an in-flight write fail closed.
func (c *dashboardCache) DashboardStatsGeneration(ctx context.Context) (string, error) {
	token, err := dashboardGenerationToken()
	if err != nil {
		return "", err
	}
	return c.rdb.Eval(ctx, `local g = redis.call('GET', KEYS[1]); if not g then redis.call('SET', KEYS[1], ARGV[1]); return ARGV[1]; end; return g`, []string{c.buildKey() + ":generation"}, token).Text()
}

func (c *dashboardCache) SetDashboardStatsIfGeneration(ctx context.Context, generation, data string, ttl time.Duration) error {
	if generation == "" || ttl <= 0 {
		return service.ErrDashboardStatsCacheMiss
	}
	_, err := c.rdb.Eval(ctx, `if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0; end; redis.call('PSETEX', KEYS[2], ARGV[3], ARGV[2]); return 1`, []string{c.buildKey() + ":generation", c.buildKey()}, generation, data, ttl.Milliseconds()).Result()
	return err // A stale writer is intentionally discarded, not a cache outage.
}

// InvalidateDashboardStatsGeneration can be retried after a committed repair;
// each successful call invalidates all earlier readers on every application node.
func (c *dashboardCache) InvalidateDashboardStatsGeneration(ctx context.Context) error {
	token, err := dashboardGenerationToken()
	if err != nil {
		return err
	}
	return c.rdb.Eval(ctx, `redis.call('SET', KEYS[1], ARGV[1]); redis.call('DEL', KEYS[2]); return 1`, []string{c.buildKey() + ":generation", c.buildKey()}, token).Err()
}
