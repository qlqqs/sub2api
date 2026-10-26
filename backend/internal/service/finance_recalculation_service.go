package service

// CUSTOM(fork): one bounded durable job at a time. SQL owns leases/CAS and all
// cost writes; this service only calculates frozen previews and fences caches.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type FinanceRecalculationConfigLoader interface {
	Snapshot(context.Context, *FinanceRecalculationBatch, []FinanceRecalculationCandidate) (FinanceRecalculationConfig, error)
	Current(context.Context, *FinanceRecalculationBatch) (FinanceRecalculationConfig, error)
}

// Invalidate must fence in-flight stale cache writers, not merely delete keys.
// Repeated calls after a crash must be safe. Failure leaves the batch syncing.
type FinanceRecalculationCacheFence interface{ Invalidate(context.Context) error }

type FinanceRecalculationService struct {
	store                                                   FinanceRecalculationStore
	loader                                                  FinanceRecalculationConfigLoader
	fence                                                   FinanceRecalculationCacheFence
	workerID                                                string
	pollInterval, leaseDuration, renewInterval, workTimeout time.Duration
	mu                                                      sync.Mutex
	cancel                                                  context.CancelFunc
	done                                                    chan struct{}
}

func NewFinanceRecalculationService(store FinanceRecalculationStore, loader FinanceRecalculationConfigLoader, fence FinanceRecalculationCacheFence) *FinanceRecalculationService {
	return &FinanceRecalculationService{store: store, loader: loader, fence: fence, workerID: uuid.NewString(), pollInterval: time.Second, leaseDuration: 2 * time.Minute, renewInterval: 30 * time.Second, workTimeout: time.Minute}
}

// Start/Stop are idempotent and safe to call concurrently. Stop cancels database
// work and waits for the worker and its heartbeat, leaving durable leases for
// recovery rather than declaring partially interrupted work completed.
func (s *FinanceRecalculationService) Start() {
	if s == nil || s.store == nil || s.loader == nil || s.fence == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	go func() { defer close(done); s.run(ctx) }()
}
func (s *FinanceRecalculationService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	if cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	if done == nil {
		return
	}
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done == done {
		s.done = nil
		s.cancel = nil
	}
}
func (s *FinanceRecalculationService) run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := s.runOnce(ctx); err != nil && ctx.Err() == nil {
			// No raw SQL/provider error or configuration data in worker logs.
			logger.L().Warn("finance_recalculation.worker_failed", zap.String("error_code", financeRecalculationFailureCode("", err)))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *FinanceRecalculationService) Create(ctx context.Context, in FinanceRecalculationCreate) (*FinanceRecalculationBatch, error) {
	if err := validateFinanceRecalculationCreate(in); err != nil {
		return nil, err
	}
	return s.store.Create(ctx, in)
}
func validateFinanceRecalculationCreate(in FinanceRecalculationCreate) error {
	if in.ActorID <= 0 || strings.TrimSpace(in.IdempotencyKey) == "" || len(in.IdempotencyKey) > 128 || len(in.Model) > 200 {
		return ErrFinanceRecalculationInvalid
	}
	if in.AccountID != nil && *in.AccountID <= 0 || in.ProviderID != nil && *in.ProviderID <= 0 {
		return ErrFinanceRecalculationInvalid
	}
	switch in.Kind {
	case FinanceRecalculationUsageCost:
		if in.ProviderID != nil {
			return ErrFinanceRecalculationInvalid
		}
	case FinanceRecalculationPaymentFee:
		if in.AccountID != nil || in.Model != "" || in.AcceptRetainedBaseline {
			return ErrFinanceRecalculationInvalid
		}
	default:
		return ErrFinanceRecalculationInvalid
	}
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	start, e1 := time.ParseInLocation("2006-01-02", in.Start, loc)
	end, e2 := time.ParseInLocation("2006-01-02", in.End, loc)
	if e1 != nil || e2 != nil || end.Before(start) || end.AddDate(0, 0, 1).Sub(start) > 732*24*time.Hour {
		return ErrFinanceRecalculationInvalid
	}
	return nil
}
func financeRecalculationPage(page, size int) (int, error) {
	if page < 1 || page > 10000000 || size < 1 || size > 200 {
		return 0, ErrFinanceRecalculationInvalid
	}
	return (page - 1) * size, nil
}
func (s *FinanceRecalculationService) List(ctx context.Context, page, size int) ([]FinanceRecalculationBatch, int64, error) {
	offset, err := financeRecalculationPage(page, size)
	if err != nil {
		return nil, 0, err
	}
	return s.store.List(ctx, size, offset)
}
func (s *FinanceRecalculationService) Get(ctx context.Context, id int64) (*FinanceRecalculationBatch, error) {
	if id <= 0 {
		return nil, ErrFinanceRecalculationInvalid
	}
	return s.store.Get(ctx, id)
}
func (s *FinanceRecalculationService) Items(ctx context.Context, id int64, page, size int) ([]FinanceRecalculationItem, int64, error) {
	if id <= 0 {
		return nil, 0, ErrFinanceRecalculationInvalid
	}
	offset, err := financeRecalculationPage(page, size)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListItems(ctx, id, size, offset)
}
func (s *FinanceRecalculationService) Confirm(ctx context.Context, id, actor int64, expected string, acknowledged bool) (*FinanceRecalculationBatch, error) {
	if !acknowledged {
		return nil, infraerrors.BadRequest("FINANCE_RECALCULATION_ACK_REQUIRED", "Explicit acknowledgement that this operation cannot be undone is required")
	}
	if actor <= 0 || strings.TrimSpace(expected) == "" || len(expected) > 256 {
		return nil, ErrFinanceRecalculationInvalid
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.ActorID != actor {
		return nil, ErrFinanceRecalculationActor
	}
	// Preserve idempotent confirmation after execution/config changes. Repository
	// still validates expected fingerprint and actor under its batch-row lock.
	actual := b.ConfigFingerprint
	switch b.Status {
	case FinanceRecalculationQueued, FinanceRecalculationApplying, FinanceRecalculationSyncing, FinanceRecalculationCompleted:
	case FinanceRecalculationReady:
		if expected != b.ConfigFingerprint {
			return nil, ErrFinanceRecalculationRuleChanged
		}
		config, err := s.loader.Current(ctx, b)
		if err != nil {
			return nil, err
		}
		actual, err = FinanceRecalculationConfigFingerprint(config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrFinanceRecalculationState
	}
	return s.store.QueueConfirm(ctx, id, actor, expected, actual)
}
func (s *FinanceRecalculationService) RetrySync(ctx context.Context, id, actor int64) (*FinanceRecalculationBatch, error) {
	if actor <= 0 {
		return nil, ErrFinanceRecalculationInvalid
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.ActorID != actor {
		return nil, ErrFinanceRecalculationActor
	}
	if err = s.store.RetrySync(ctx, id, actor); err != nil {
		return nil, err
	}
	return s.store.Get(ctx, id)
}

// The store's Apply holds the batch row while writing; RenewLease can therefore
// wait behind it. Work is bounded below the lease duration, and canceling this
// context rolls back before expiry. A heartbeat error cancels work and never
// attempts a state write with a potentially stale lease.
func (s *FinanceRecalculationService) runOnce(parent context.Context) (bool, error) {
	b, err := s.store.Claim(parent, s.workerID, s.leaseDuration)
	if err != nil || b == nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(parent, s.workTimeout)
	heartbeatDone := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(s.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				heartbeatDone <- nil
				return
			case <-ticker.C:
				if err := s.store.RenewLease(ctx, b.ID, b.LeaseToken, s.leaseDuration); err != nil {
					if ctx.Err() == nil {
						cancel()
						heartbeatDone <- err
					} else {
						heartbeatDone <- nil
					}
					return
				}
			}
		}
	}()
	phase := b.Status
	workErr := s.processSafely(ctx, b, &phase)
	workContextErr := ctx.Err()
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return true, heartbeatErr
	}
	if parent.Err() != nil {
		return true, parent.Err()
	}
	if workErr == nil {
		return true, nil
	}
	// Ambiguous commits and lease loss are recovered by the next durable claim.
	if errors.Is(workErr, ErrFinanceRecalculationState) || errors.Is(workErr, errFinanceRecalculationUncertain) {
		return true, workErr
	}
	if workContextErr != nil {
		return true, workErr
	}
	code := financeRecalculationFailureCode(phase, workErr)
	failCtx, failCancel := context.WithTimeout(parent, 5*time.Second)
	defer failCancel()
	if err := s.store.Fail(failCtx, b.ID, b.LeaseToken, code, ""); err != nil {
		return true, fmt.Errorf("persist finance job failure: %w", err)
	}
	return true, workErr
}

var errFinanceRecalculationUncertain = errors.New("finance recalculation commit outcome unavailable")

func (s *FinanceRecalculationService) processSafely(ctx context.Context, b *FinanceRecalculationBatch, phase *string) (err error) {
	defer func() {
		if recover() != nil {
			if *phase == FinanceRecalculationApplying {
				// A panic at the commit boundary has the same ambiguity as a
				// disconnected client. Let the durable state drive recovery.
				err = errFinanceRecalculationUncertain
			} else {
				err = errors.New("finance recalculation worker panicked")
			}
		}
	}()
	switch b.Status {
	case FinanceRecalculationPreviewing:
		preview, err := s.preview(ctx, b)
		if err != nil {
			return err
		}
		return s.store.SavePreview(ctx, b.ID, b.LeaseToken, preview)
	case FinanceRecalculationQueued, FinanceRecalculationApplying:
		*phase = FinanceRecalculationApplying
		config, err := s.loader.Current(ctx, b)
		if err != nil {
			return err
		}
		actual, err := FinanceRecalculationConfigFingerprint(config)
		if err != nil {
			return err
		}
		if err = s.store.Apply(ctx, b.ID, b.LeaseToken, actual); err != nil {
			// A lost connection around COMMIT must never be mislabeled "no changes
			// committed". Read the durable state before failing or continuing.
			stored, readErr := s.store.Get(ctx, b.ID)
			if readErr != nil {
				return errFinanceRecalculationUncertain
			}
			if stored.LeaseToken != b.LeaseToken {
				return ErrFinanceRecalculationState
			}
			if stored.Status != FinanceRecalculationSyncing {
				return err
			}
		}
		*phase = FinanceRecalculationSyncing
	case FinanceRecalculationSyncing:
		*phase = FinanceRecalculationSyncing
	default:
		return ErrFinanceRecalculationState
	}
	if s.fence == nil {
		return ErrFinanceRecalculationSyncRequired
	}
	if err := s.fence.Invalidate(ctx); err != nil {
		return err
	}
	return s.store.Complete(ctx, b.ID, b.LeaseToken)
}
func financeRecalculationFailureCode(phase string, err error) string {
	switch {
	case errors.Is(err, ErrFinanceRecalculationCoverageUnknown):
		return "coverage_unknown"
	case errors.Is(err, ErrFinanceRecalculationTooLarge):
		return "preview_too_large"
	case errors.Is(err, ErrFinanceRecalculationExpired):
		return "preview_expired"
	case errors.Is(err, ErrFinanceRecalculationSourceChanged):
		return "source_changed"
	case errors.Is(err, ErrFinanceRecalculationRuleChanged):
		return "rule_changed"
	case phase == FinanceRecalculationSyncing:
		return "sync_failed"
	case phase == FinanceRecalculationApplying || phase == FinanceRecalculationQueued:
		return "apply_failed"
	default:
		return "preview_failed"
	}
}

type financeUsageSnapshotKey struct {
	account, group int64
	model          string
}

func (s *FinanceRecalculationService) preview(ctx context.Context, b *FinanceRecalculationBatch) (FinanceRecalculationPreview, error) {
	out := FinanceRecalculationPreview{}
	candidates, err := s.store.LoadCandidates(ctx, b)
	if err != nil {
		return out, err
	}
	if len(candidates) > FinanceRecalculationMaxRows {
		return out, ErrFinanceRecalculationTooLarge
	}
	out.Config, err = s.loader.Snapshot(ctx, b, candidates)
	if err != nil {
		return out, err
	}
	out.ConfigFingerprint, err = FinanceRecalculationConfigFingerprint(out.Config)
	if err != nil {
		return out, err
	}
	usage := make(map[financeUsageSnapshotKey]FinanceUsageCostSnapshot, len(out.Config.Usage))
	for _, v := range out.Config.Usage {
		key := financeUsageSnapshotKey{v.AccountID, v.GroupID, v.Model}
		if _, exists := usage[key]; exists {
			return out, ErrFinanceRecalculationInvalid
		}
		usage[key] = v
	}
	payment := make(map[int64]FinanceRecalculationPaymentRule, len(out.Config.Payment))
	for _, v := range out.Config.Payment {
		if _, exists := payment[v.ProviderID]; exists {
			return out, ErrFinanceRecalculationInvalid
		}
		payment[v.ProviderID] = v
	}
	out.Items = make([]FinanceRecalculationItem, 0, len(candidates))
	for _, c := range candidates {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		item := FinanceRecalculationItem{Candidate: c, Status: FinanceRecalculationSkipped, Reason: "missing_current_config"}
		if b.Kind == FinanceRecalculationUsageCost {
			if c.Usage == nil {
				return out, ErrFinanceRecalculationInvalid
			}
			item.OldCost = financeRecalculationOriginalUsageCost(*c.Usage)
			snap, ok := usage[financeUsageSnapshotKey{c.Usage.AccountID, c.Usage.GroupID, financeUsageCostModel(*c.Usage)}]
			if ok {
				input, reason := financeRecalculationUsageEvidence(c)
				if reason != "" {
					item.Reason = reason
				} else {
					result, err := CalculateFinanceUsageCost(input, snap)
					if err != nil {
						return out, err
					}
					item.Status, item.Reason, item.Source = result.Status, result.Reason, result.Source
					item.UsageResult = &result
					if result.OldCost != "" {
						item.OldCost = financeCopyString(&result.OldCost)
					}
					if result.Status != FinanceRecalculationSkipped {
						item.NewCost = financeCopyString(&result.NewCost)
						item.Delta = financeCopyString(&result.Delta)
					}
				}
			}
		} else if b.Kind == FinanceRecalculationPaymentFee {
			if c.Payment == nil {
				return out, ErrFinanceRecalculationInvalid
			}
			item.OldCost = financeRecalculationOriginalPaymentFee(*c.Payment)
			snap, ok := payment[c.Payment.ProviderID]
			if ok {
				if c.Payment.CurrentFeeRate != snap.FeeRate {
					return out, ErrFinanceRecalculationRuleChanged
				}
				if snap.FeeRate == "" {
					item.NewCost = financeCopyString(item.OldCost)
					out.Items = append(out.Items, item)
					continue
				}
				result, err := CalculateFinancePaymentFee(*c.Payment)
				if err != nil {
					item.Reason = "invalid_payment_evidence"
				} else {
					item.PaymentResult = &result
					item.Status, item.Reason, item.Source = result.Status, "", "payment_channel_fee"
					item.OldCost = financeCopyString(&result.OldFee)
					item.NewCost = financeCopyString(&result.NewFee)
					item.Delta = financeCopyString(&result.Delta)
				}
			}
		} else {
			return out, ErrFinanceRecalculationInvalid
		}
		if item.Status == FinanceRecalculationSkipped {
			item.NewCost = financeCopyString(item.OldCost)
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// Keep Candidate.Usage byte-for-byte equivalent to the repository snapshot.
// Any future evidence enrichment belongs only in this calculation copy and must
// have a proven persisted provenance; current account platform is not evidence
// of historical absence of cache storage charges or pricing timestamps.
func financeRecalculationUsageEvidence(c FinanceRecalculationCandidate) (FinanceUsageCostInput, string) {
	input := *c.Usage
	if len(c.Raw) == 0 {
		return input, ""
	}
	var raw struct {
		InboundEndpoint string `json:"inbound_endpoint"`
	}
	if err := json.Unmarshal(c.Raw, &raw); err != nil {
		return input, "invalid_usage_evidence"
	}
	// ImageCacheSplitKnown is populated by the repository. Only the pure
	// calculator knows whether this path needs token/image-cache evidence.
	if input.CacheStorageCost == nil {
		// These persisted gateway entry points do not charge Gemini
		// cachedContents storage. Never infer this from today's account platform.
		switch raw.InboundEndpoint {
		case "/v1/responses", "/v1/chat/completions", "/v1/messages", "/v1/images/generations", "/v1/images/edits":
			zero := "0"
			input.CacheStorageCost = &zero
		}
	}
	return input, ""
}
func financeRecalculationOriginalUsageCost(in FinanceUsageCostInput) *string {
	base := in.TotalCost
	if in.AccountStatsCost != nil {
		base = *in.AccountStatsCost
	}
	b, err := financeCostDecimal(base, 10, "10000000000")
	if err != nil {
		return nil
	}
	rate := decimal.NewFromInt(1)
	if in.AccountRateMultiplier != nil {
		rate, err = financeCostDecimal(*in.AccountRateMultiplier, 4, "1000000")
		if err != nil {
			return nil
		}
	}
	value := b.Mul(rate).String()
	return &value
}
func financeRecalculationOriginalPaymentFee(in FinancePaymentFeeInput) *string {
	if in.OriginalFeeRate == nil {
		return nil
	}
	amount, err := financeCostDecimal(in.PayAmount, 8, "1000000000000")
	if err != nil {
		return nil
	}
	rate, err := ValidateChannelFeeRate(*in.OriginalFeeRate)
	if err != nil {
		return nil
	}
	value := amount.Mul(decimal.RequireFromString(rate)).String()
	return &value
}
