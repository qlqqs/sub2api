package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	affiliateSettleRunTimeout = 10 * time.Minute
	// affiliateSettleLeaderLockKey gates the periodic consumption settlement so only
	// one instance walks the invite chains per cycle.
	affiliateSettleLeaderLockKey = "affiliate:rebate:settle:leader"
	// affiliateSettleLeaderLockTTL must exceed affiliateSettleRunTimeout so the lock
	// never expires mid-run.
	affiliateSettleLeaderLockTTL = 15 * time.Minute
	// affiliateSettleCheckInterval 是检查「是否到期」的节拍；结算间隔每次检查都重新读取，
	// 修改设置后最迟一个节拍生效，不必等旧间隔跑完或重启。
	affiliateSettleCheckInterval = time.Minute
)

// AffiliateRebateSettleService periodically settles consumption-based affiliate
// rebates. The interval is re-read from settings every cycle so admins can tune
// it without restarting.
type AffiliateRebateSettleService struct {
	affiliateSvc *AffiliateService
	settingSvc   *SettingService
	stopCh       chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	// intervalOverride / lagOverride 仅供测试环境缩短结算等待（配置 affiliate.settle_*_override），
	// 为 0 时沿用后台结算周期与 affiliateSettleLag。
	intervalOverride time.Duration
	lagOverride      time.Duration
}

func NewAffiliateRebateSettleService(affiliateSvc *AffiliateService, settingSvc *SettingService) *AffiliateRebateSettleService {
	return &AffiliateRebateSettleService{
		affiliateSvc: affiliateSvc,
		settingSvc:   settingSvc,
		stopCh:       make(chan struct{}),
		instanceID:   uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// instance per cycle. When both are nil the job runs ungated.
func (s *AffiliateRebateSettleService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// SetSettleOverrides 设置测试环境的结算周期与滞后窗口覆盖；必须在 Start 之前调用，非正数表示不覆盖。
func (s *AffiliateRebateSettleService) SetSettleOverrides(interval, lag time.Duration) {
	if s == nil {
		return
	}
	s.intervalOverride, s.lagOverride = 0, 0
	if interval > 0 {
		s.intervalOverride = interval
	}
	if lag > 0 {
		s.lagOverride = lag
	}
}

func (s *AffiliateRebateSettleService) Start() {
	if s == nil || s.affiliateSvc == nil {
		return
	}
	if s.intervalOverride > 0 || s.lagOverride > 0 {
		slog.Warn("[AffiliateSettle] settle overrides active (test environments only)",
			"interval", s.checkInterval().String(), "lag", s.settleLag().String())
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 首轮延后一个周期，避免启动时与迁移、缓存预热争抢数据库。
		lastRun := time.Now()
		ticker := time.NewTicker(s.checkInterval())
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				if !s.due(lastRun, now) {
					continue
				}
				lastRun = now
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// checkInterval 是检查节拍；覆盖了结算周期时节拍就是该周期，每拍都执行。
func (s *AffiliateRebateSettleService) checkInterval() time.Duration {
	if s.intervalOverride > 0 {
		return s.intervalOverride
	}
	return affiliateSettleCheckInterval
}

// due 判断距上一轮是否已满结算间隔；间隔每次都从设置重新读取。
func (s *AffiliateRebateSettleService) due(lastRun, now time.Time) bool {
	if s.intervalOverride > 0 {
		return true
	}
	return now.Sub(lastRun) >= s.interval()
}

func (s *AffiliateRebateSettleService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *AffiliateRebateSettleService) interval() time.Duration {
	minutes := AffiliateSettleIntervalMinutesDefault
	if s.settingSvc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		minutes = s.settingSvc.GetAffiliateSettleIntervalMinutes(ctx)
		cancel()
	}
	return time.Duration(minutes) * time.Minute
}

func (s *AffiliateRebateSettleService) settleLag() time.Duration {
	if s.lagOverride > 0 {
		return s.lagOverride
	}
	return affiliateSettleLag
}

func (s *AffiliateRebateSettleService) runOnce() {
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, affiliateSettleLeaderLockKey, s.instanceID, affiliateSettleLeaderLockTTL)
	lockCancel()
	if !ok {
		return
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), affiliateSettleRunTimeout)
	defer cancel()
	result, err := s.affiliateSvc.SettleConsumptionRebates(ctx, s.settleLag())
	if err != nil {
		slog.Error("[AffiliateSettle] consumption rebate settlement failed", "error", err, "users", result.Users, "rebate", result.Rebate)
		return
	}
	if result.Users > 0 {
		slog.Info("[AffiliateSettle] settled consumption rebates", "users", result.Users, "consumption", result.Consumption, "rebate", result.Rebate)
	}
}
