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

func (s *AffiliateRebateSettleService) Start() {
	if s == nil || s.affiliateSvc == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 首轮延后一个周期，避免启动时与迁移、缓存预热争抢数据库。
		lastRun := time.Now()
		ticker := time.NewTicker(affiliateSettleCheckInterval)
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

// due 判断距上一轮是否已满结算间隔；间隔每次都从设置重新读取。
func (s *AffiliateRebateSettleService) due(lastRun, now time.Time) bool {
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
	result, err := s.affiliateSvc.SettleConsumptionRebates(ctx)
	if err != nil {
		slog.Error("[AffiliateSettle] consumption rebate settlement failed", "error", err, "users", result.Users, "rebate", result.Rebate)
		return
	}
	if result.Users > 0 {
		slog.Info("[AffiliateSettle] settled consumption rebates", "users", result.Users, "consumption", result.Consumption, "rebate", result.Rebate)
	}
}
