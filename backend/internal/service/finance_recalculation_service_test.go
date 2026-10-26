//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type financeOrchestrationStore struct {
	FinanceRecalculationStore
	batch                 *FinanceRecalculationBatch
	candidates            []FinanceRecalculationCandidate
	saved                 *FinanceRecalculationPreview
	claim                 func(context.Context) (*FinanceRecalculationBatch, error)
	renew                 func(context.Context) error
	apply                 func(context.Context) error
	get                   func(context.Context) (*FinanceRecalculationBatch, error)
	load                  func(context.Context) ([]FinanceRecalculationCandidate, error)
	save                  func(context.Context) error
	calls                 []string
	failureCode           string
	created               FinanceRecalculationCreate
	listLimit, listOffset int
}

func (s *financeOrchestrationStore) Claim(ctx context.Context, _ string, _ time.Duration) (*FinanceRecalculationBatch, error) {
	if s.claim != nil {
		return s.claim(ctx)
	}
	if s.batch == nil {
		return nil, nil
	}
	b := *s.batch
	return &b, nil
}
func (s *financeOrchestrationStore) RenewLease(ctx context.Context, _ int64, _ string, _ time.Duration) error {
	if s.renew != nil {
		return s.renew(ctx)
	}
	return nil
}
func (s *financeOrchestrationStore) LoadCandidates(ctx context.Context, _ *FinanceRecalculationBatch) ([]FinanceRecalculationCandidate, error) {
	if s.load != nil {
		return s.load(ctx)
	}
	return s.candidates, nil
}
func (s *financeOrchestrationStore) SavePreview(ctx context.Context, _ int64, _ string, p FinanceRecalculationPreview) error {
	if s.save != nil {
		if err := s.save(ctx); err != nil {
			return err
		}
	}
	s.saved = &p
	s.calls = append(s.calls, "save")
	s.batch.Status = FinanceRecalculationReady
	return nil
}
func (s *financeOrchestrationStore) Apply(ctx context.Context, _ int64, _ string, _ string) error {
	s.calls = append(s.calls, "apply")
	if s.apply != nil {
		return s.apply(ctx)
	}
	s.batch.Status = FinanceRecalculationSyncing
	return nil
}
func (s *financeOrchestrationStore) Complete(context.Context, int64, string) error {
	s.calls = append(s.calls, "complete")
	s.batch.Status = FinanceRecalculationCompleted
	return nil
}
func (s *financeOrchestrationStore) Fail(_ context.Context, _ int64, _ string, code, message string) error {
	s.calls = append(s.calls, "fail")
	s.failureCode = code
	s.batch.FailedPhase = s.batch.Status
	s.batch.Status = FinanceRecalculationFailed
	return nil
}
func (s *financeOrchestrationStore) Get(ctx context.Context, _ int64) (*FinanceRecalculationBatch, error) {
	if s.get != nil {
		return s.get(ctx)
	}
	b := *s.batch
	return &b, nil
}
func (s *financeOrchestrationStore) QueueConfirm(_ context.Context, _ int64, _ int64, expected, actual string) (*FinanceRecalculationBatch, error) {
	s.calls = append(s.calls, "confirm")
	if expected != s.batch.ConfigFingerprint || actual != expected {
		return nil, ErrFinanceRecalculationRuleChanged
	}
	if s.batch.Status == FinanceRecalculationReady {
		s.batch.Status = FinanceRecalculationQueued
	}
	b := *s.batch
	return &b, nil
}
func (s *financeOrchestrationStore) RetrySync(context.Context, int64, int64) error {
	s.calls = append(s.calls, "retry")
	if s.batch.Status == FinanceRecalculationFailed && s.batch.FailedPhase == FinanceRecalculationSyncing {
		s.batch.Status = FinanceRecalculationSyncing
		return nil
	}
	return ErrFinanceRecalculationState
}
func (s *financeOrchestrationStore) Cancel(_ context.Context, _ int64, actor int64) (*FinanceRecalculationBatch, error) {
	s.calls = append(s.calls, "cancel")
	if actor != s.batch.ActorID {
		return nil, ErrFinanceRecalculationActor
	}
	s.batch.Status = FinanceRecalculationCancelled
	b := *s.batch
	return &b, nil
}
func (s *financeOrchestrationStore) Create(_ context.Context, in FinanceRecalculationCreate) (*FinanceRecalculationBatch, error) {
	s.created = in
	return s.batch, nil
}
func (s *financeOrchestrationStore) List(_ context.Context, limit, offset int) ([]FinanceRecalculationBatch, int64, error) {
	s.listLimit = limit
	s.listOffset = offset
	return nil, 0, nil
}
func (s *financeOrchestrationStore) ListItems(_ context.Context, _ int64, limit, offset int) ([]FinanceRecalculationItem, int64, error) {
	s.listLimit = limit
	s.listOffset = offset
	return nil, 0, nil
}

type financeOrchestrationLoader struct {
	config       FinanceRecalculationConfig
	err          error
	snapshot     func(context.Context) error
	currentCalls int
}

func (l *financeOrchestrationLoader) Snapshot(ctx context.Context, _ *FinanceRecalculationBatch, _ []FinanceRecalculationCandidate) (FinanceRecalculationConfig, error) {
	if l.snapshot != nil {
		if err := l.snapshot(ctx); err != nil {
			return l.config, err
		}
	}
	return l.config, l.err
}
func (l *financeOrchestrationLoader) Current(context.Context, *FinanceRecalculationBatch) (FinanceRecalculationConfig, error) {
	l.currentCalls++
	return l.config, l.err
}

type financeOrchestrationFence struct {
	fn    func(context.Context) error
	calls int
}

func (f *financeOrchestrationFence) Invalidate(ctx context.Context) error {
	f.calls++
	if f.fn != nil {
		return f.fn(ctx)
	}
	return nil
}
func financeOrchestrationFixture(status, kind string) (*FinanceRecalculationService, *financeOrchestrationStore, *financeOrchestrationLoader, *financeOrchestrationFence) {
	store := &financeOrchestrationStore{batch: &FinanceRecalculationBatch{ID: 1, FinanceRecalculationFilter: FinanceRecalculationFilter{Kind: kind}, Status: status, ActorID: 9, LeaseToken: "lease", ExpiresAt: time.Now().Add(time.Hour)}}
	loader := &financeOrchestrationLoader{}
	fence := &financeOrchestrationFence{}
	return NewFinanceRecalculationService(store, loader, fence), store, loader, fence
}
func financeOrchestrationUsage() FinanceRecalculationCandidate {
	old, rate := "0.5", "1"
	in := &FinanceUsageCostInput{ID: 1, CreatedAt: time.Now().UTC(), AccountID: 1, GroupID: 2, Model: "model", TotalCost: "10", AccountStatsCost: &old, AccountRateMultiplier: &rate, ImageCount: 2, BillingMode: BillingModeImage, MediaType: "image"}
	return FinanceRecalculationCandidate{TargetID: 1, TargetCreatedAt: in.CreatedAt, SourceFingerprint: "original", Usage: in, Raw: json.RawMessage(`{"inbound_endpoint":"/v1/images/generations"}`)}
}
func TestFinanceRecalculationServicePreviewPreservesCandidateEvidence(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	candidate := financeOrchestrationUsage()
	store.candidates = []FinanceRecalculationCandidate{candidate}
	price := 0.1
	loader.config.Usage = []FinanceUsageCostSnapshot{{AccountID: 1, GroupID: 2, Model: "model", Platform: PlatformOpenAI, DefaultMultiplier: "2", Channel: &FinanceCostChannelSnapshot{ID: 4, Rules: []AccountStatsPricingRule{{AccountIDs: []int64{1}, Pricing: []ChannelModelPricing{{Models: []string{"model"}, BillingMode: BillingModeImage, PerRequestPrice: &price}}}}}}}
	original, err := json.Marshal(candidate)
	require.NoError(t, err)
	worked, err := s.runOnce(context.Background())
	require.True(t, worked)
	require.NoError(t, err)
	require.Equal(t, []string{"save"}, store.calls)
	require.NotNil(t, store.saved)
	item := store.saved.Items[0]
	require.Equal(t, FinanceRecalculationChanged, item.Status)
	require.Equal(t, "0.4", *item.NewCost)
	persisted, err := json.Marshal(item.Candidate)
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(persisted))
	require.Nil(t, candidate.Usage.CacheStorageCost)
	require.Nil(t, item.Candidate.Usage.CacheStorageCost)
	fingerprint, err := FinanceRecalculationConfigFingerprint(loader.config)
	require.NoError(t, err)
	require.Equal(t, fingerprint, store.saved.ConfigFingerprint)
}
func TestFinanceRecalculationServiceMissingConfigAndUnknownEvidenceSkip(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, reason string
		missing                bool
		marker                 string
	}{
		{name: "absent configuration", missing: true, reason: "missing_current_config"},
		{name: "deleted account", marker: "missing_current_account", reason: "missing_current_account"},
		{name: "unknown endpoint", endpoint: "", reason: "unknown_cache_storage_cost"},
		{name: "Gemini", endpoint: "/v1beta/models/gemini:generateContent", reason: "unknown_cache_storage_cost"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
			c := financeOrchestrationUsage()
			c.Raw = json.RawMessage(`{"inbound_endpoint":"` + tc.endpoint + `"}`)
			store.candidates = []FinanceRecalculationCandidate{c}
			price := 0.1
			if !tc.missing {
				loader.config.Usage = []FinanceUsageCostSnapshot{{AccountID: 1, GroupID: 2, Model: "model", DefaultMultiplier: "1", MissingReason: tc.marker, Channel: &FinanceCostChannelSnapshot{Rules: []AccountStatsPricingRule{{AccountIDs: []int64{1}, Pricing: []ChannelModelPricing{{Models: []string{"model"}, BillingMode: BillingModeImage, PerRequestPrice: &price}}}}}}}
			}
			_, err := s.runOnce(context.Background())
			require.NoError(t, err)
			item := store.saved.Items[0]
			require.Equal(t, FinanceRecalculationSkipped, item.Status)
			require.Equal(t, tc.reason, item.Reason)
			require.Equal(t, "0.5", *item.OldCost)
			require.Equal(t, item.OldCost, item.NewCost)
			require.Nil(t, item.Delta)
		})
	}
}
func TestFinanceRecalculationServicePaymentFrozenRate(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationPaymentFee)
	old := "0.01"
	candidate := FinanceRecalculationCandidate{TargetID: 1, Payment: &FinancePaymentFeeInput{OrderID: 1, ProviderID: 8, PayAmount: "100", OriginalFeeRate: &old, CurrentFeeRate: "0.02"}}
	store.candidates = []FinanceRecalculationCandidate{candidate}
	loader.config.Payment = []FinanceRecalculationPaymentRule{{ProviderID: 8, FeeRate: "0.02"}}
	p, err := s.preview(context.Background(), store.batch)
	require.NoError(t, err)
	require.Equal(t, "1", *p.Items[0].Delta)
	require.True(t, reflect.DeepEqual(candidate, p.Items[0].Candidate))
	loader.config.Payment[0].FeeRate = "0.03"
	_, err = s.preview(context.Background(), store.batch)
	require.ErrorIs(t, err, ErrFinanceRecalculationRuleChanged)
	require.Equal(t, "0.02", candidate.Payment.CurrentFeeRate)
	candidate.Payment.CurrentFeeRate = ""
	candidate.Payment.OriginalFeeRate = nil
	loader.config.Payment[0].FeeRate = ""
	p, err = s.preview(context.Background(), store.batch)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationSkipped, p.Items[0].Status)
	require.Nil(t, p.Items[0].OldCost)
}
func TestFinanceRecalculationServiceSyncRetryNeverReapplies(t *testing.T) {
	s, store, loader, fence := financeOrchestrationFixture(FinanceRecalculationApplying, FinanceRecalculationUsageCost)
	fence.fn = func(context.Context) error { return errors.New("secret cache address") }
	_, err := s.runOnce(context.Background())
	require.Error(t, err)
	require.Equal(t, []string{"apply", "fail"}, store.calls)
	require.Equal(t, "sync_failed", store.failureCode)
	require.Equal(t, FinanceRecalculationSyncing, store.batch.FailedPhase)
	_, err = s.RetrySync(context.Background(), 1, 9)
	require.NoError(t, err)
	fence.fn = nil
	_, err = s.runOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"apply", "fail", "retry", "complete"}, store.calls)
	require.Equal(t, 1, loader.currentCalls)
	require.Equal(t, 2, fence.calls)
	require.Equal(t, FinanceRecalculationCompleted, store.batch.Status)
}
func TestFinanceRecalculationServiceRestartInSyncing(t *testing.T) {
	s, store, loader, fence := financeOrchestrationFixture(FinanceRecalculationSyncing, FinanceRecalculationUsageCost)
	loader.err = errors.New("configuration unavailable after commit")
	_, err := s.runOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"complete"}, store.calls)
	require.Zero(t, loader.currentCalls)
	require.Equal(t, 1, fence.calls)
}
func TestFinanceRecalculationServiceApplyFailureAndAmbiguousCommit(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		err                  error
		readFails, committed bool
		code                 string
	}{
		{name: "config conflict", err: ErrFinanceRecalculationRuleChanged, code: "rule_changed"},
		{name: "source conflict", err: ErrFinanceRecalculationSourceChanged, code: "source_changed"},
		{name: "coverage incomplete", err: ErrFinanceRecalculationCoverageUnknown, code: "coverage_unknown"},
		{name: "apply failure", err: errors.New("driver password"), code: "apply_failed"},
		{name: "unknown outcome", err: errors.New("connection lost"), readFails: true},
		{name: "committed but reply lost", err: errors.New("connection lost"), committed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, _, fence := financeOrchestrationFixture(FinanceRecalculationApplying, FinanceRecalculationUsageCost)
			store.apply = func(context.Context) error {
				if tc.committed {
					store.batch.Status = FinanceRecalculationSyncing
				}
				return tc.err
			}
			if tc.readFails {
				store.get = func(context.Context) (*FinanceRecalculationBatch, error) { return nil, errors.New("db unavailable") }
			}
			_, err := s.runOnce(context.Background())
			if tc.committed {
				require.NoError(t, err)
				require.Equal(t, []string{"apply", "complete"}, store.calls)
				require.Equal(t, 1, fence.calls)
			} else {
				require.Error(t, err)
				require.Equal(t, tc.code, store.failureCode)
				require.Zero(t, fence.calls)
			}
		})
	}
}
func TestFinanceRecalculationServiceHeartbeatLossCancelsWithoutFailureWrite(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	s.renewInterval = time.Millisecond
	s.workTimeout = time.Second
	lost := errors.New("lease lost")
	store.renew = func(context.Context) error { return lost }
	loader.snapshot = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	_, err := s.runOnce(context.Background())
	require.ErrorIs(t, err, lost)
	require.Empty(t, store.calls)
	require.Nil(t, store.saved)
}
func TestFinanceRecalculationServiceRenewsDuringPreview(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	s.renewInterval = time.Millisecond
	s.workTimeout = time.Second
	renewed := make(chan struct{}, 1)
	store.renew = func(context.Context) error {
		select {
		case renewed <- struct{}{}:
		default:
		}
		return nil
	}
	loader.snapshot = func(ctx context.Context) error {
		select {
		case <-renewed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	_, err := s.runOnce(context.Background())
	require.NoError(t, err)
	require.NotNil(t, store.saved)
}
func TestFinanceRecalculationServiceStartStopIdempotentAndRecovery(t *testing.T) {
	s, store, _, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	entered := make(chan struct{}, 2)
	var calls atomic.Int32
	store.claim = func(ctx context.Context) (*FinanceRecalculationBatch, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	s.Start()
	s.Start()
	<-entered
	s.Stop()
	s.Stop()
	require.Equal(t, int32(1), calls.Load())
	s.Start()
	<-entered
	s.Stop()
	require.Equal(t, int32(2), calls.Load())
	require.Empty(t, store.calls)
}
func TestFinanceRecalculationServiceCreatorAndConfirmation(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationReady, FinanceRecalculationUsageCost)
	fingerprint, err := FinanceRecalculationConfigFingerprint(loader.config)
	require.NoError(t, err)
	store.batch.ConfigFingerprint = fingerprint
	_, err = s.Confirm(context.Background(), 1, 9, fingerprint, false)
	require.Error(t, err)
	_, err = s.Confirm(context.Background(), 1, 8, fingerprint, true)
	require.ErrorIs(t, err, ErrFinanceRecalculationActor)
	_, err = s.RetrySync(context.Background(), 1, 8)
	require.ErrorIs(t, err, ErrFinanceRecalculationActor)
	require.Zero(t, loader.currentCalls)
	require.Empty(t, store.calls)
	_, err = s.Confirm(context.Background(), 1, 9, "old", true)
	require.ErrorIs(t, err, ErrFinanceRecalculationRuleChanged)
	_, err = s.Confirm(context.Background(), 1, 9, fingerprint, true)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationQueued, store.batch.Status)
	loader.err = errors.New("current unavailable")
	_, err = s.Confirm(context.Background(), 1, 9, fingerprint, true)
	require.NoError(t, err)
	require.Equal(t, 1, loader.currentCalls)
	store.batch.Status = FinanceRecalculationReady
	loader.err = nil
	loader.config.Payment = []FinanceRecalculationPaymentRule{{ProviderID: 1, FeeRate: "0.1"}}
	_, err = s.Confirm(context.Background(), 1, 9, fingerprint, true)
	require.ErrorIs(t, err, ErrFinanceRecalculationRuleChanged)
}
func TestFinanceRecalculationServiceBoundedValidationAndFailures(t *testing.T) {
	s, store, _, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	valid := FinanceRecalculationCreate{ActorID: 9, IdempotencyKey: "request", FinanceRecalculationFilter: FinanceRecalculationFilter{Kind: FinanceRecalculationUsageCost, Start: "2026-10-25", End: "2026-10-26"}}
	_, err := s.Create(context.Background(), valid)
	require.NoError(t, err)
	require.Equal(t, valid, store.created)
	for _, change := range []func(*FinanceRecalculationCreate){func(v *FinanceRecalculationCreate) { v.ActorID = 0 }, func(v *FinanceRecalculationCreate) { v.Start = "2020-01-01" }, func(v *FinanceRecalculationCreate) { v.End = "bad" }, func(v *FinanceRecalculationCreate) { v.IdempotencyKey = "" }, func(v *FinanceRecalculationCreate) { v.Kind = "undo" }} {
		v := valid
		change(&v)
		_, err = s.Create(context.Background(), v)
		require.ErrorIs(t, err, ErrFinanceRecalculationInvalid)
	}
	_, _, err = s.List(context.Background(), 3, 20)
	require.NoError(t, err)
	require.Equal(t, 20, store.listLimit)
	require.Equal(t, 40, store.listOffset)
	_, _, err = s.Items(context.Background(), 1, 0, 20)
	require.Error(t, err)
	store.load = func(context.Context) ([]FinanceRecalculationCandidate, error) {
		return nil, ErrFinanceRecalculationTooLarge
	}
	_, err = s.runOnce(context.Background())
	require.ErrorIs(t, err, ErrFinanceRecalculationTooLarge)
	require.Equal(t, "preview_too_large", store.failureCode)
}
func TestFinanceRecalculationUsageEvidenceDefersImageSplitToCalculator(t *testing.T) {
	c := financeOrchestrationUsage()
	c.Raw = json.RawMessage(`{"tokens":{"ImageInputTokens":5,"CacheReadTokens":2},"inbound_endpoint":"/v1/responses"}`)
	c.Usage.Tokens.ImageInputTokens = 5
	c.Usage.Tokens.CacheReadTokens = 2
	in, reason := financeRecalculationUsageEvidence(c)
	require.Empty(t, reason)
	require.False(t, in.ImageCacheSplitKnown)
	require.Equal(t, "0", *in.CacheStorageCost)
	require.Nil(t, c.Usage.CacheStorageCost)
	// Rate-only cost does not need reconstructed image-token evidence.
	snapshot := FinanceUsageCostSnapshot{AccountID: 1, GroupID: 2, Model: "model", DefaultMultiplier: "2"}
	result, err := CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationChanged, result.Status)
	// Nor does a known image-count rule.
	price := 0.1
	snapshot.Channel = &FinanceCostChannelSnapshot{Rules: []AccountStatsPricingRule{{AccountIDs: []int64{1}, Pricing: []ChannelModelPricing{{Models: []string{"model"}, BillingMode: BillingModeImage, PerRequestPrice: &price}}}}}
	result, err = CalculateFinanceUsageCost(in, snapshot)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationChanged, result.Status)
}

func TestFinanceRecalculationServicePanicRecoveryKeepsCommitAmbiguitySafe(t *testing.T) {
	t.Run("preview failure is persisted", func(t *testing.T) {
		s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
		loader.snapshot = func(context.Context) error { panic("do not persist this private payload") }
		_, err := s.runOnce(context.Background())
		require.Error(t, err)
		require.Equal(t, "preview_failed", store.failureCode)
		require.Equal(t, []string{"fail"}, store.calls)
	})
	t.Run("apply panic is recovered from durable state", func(t *testing.T) {
		s, store, _, _ := financeOrchestrationFixture(FinanceRecalculationApplying, FinanceRecalculationUsageCost)
		store.apply = func(context.Context) error {
			store.batch.Status = FinanceRecalculationSyncing
			panic("commit reply lost")
		}
		_, err := s.runOnce(context.Background())
		require.ErrorIs(t, err, errFinanceRecalculationUncertain)
		require.Equal(t, []string{"apply"}, store.calls)
		// After lease expiry a restarted worker claims syncing, not applying.
		_, err = s.runOnce(context.Background())
		require.NoError(t, err)
		require.Equal(t, []string{"apply", "complete"}, store.calls)
	})
}

func TestFinanceRecalculationServiceUnprovenCoverageIsSkipped(t *testing.T) {
	s, store, loader, _ := financeOrchestrationFixture(FinanceRecalculationPreviewing, FinanceRecalculationUsageCost)
	candidate := financeOrchestrationUsage()
	from := candidate.Usage.CreatedAt.Add(time.Hour)
	loader.config.Usage = []FinanceUsageCostSnapshot{{AccountID: 1, GroupID: 2, Model: "model", DefaultMultiplier: "2", CoverageFrom: &from}}
	store.candidates = []FinanceRecalculationCandidate{candidate}
	_, err := s.runOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationSkipped, store.saved.Items[0].Status)
	require.Equal(t, "unproven_aggregate_coverage", store.saved.Items[0].Reason)
	require.Equal(t, store.saved.Items[0].OldCost, store.saved.Items[0].NewCost)
	require.Nil(t, store.saved.Items[0].Delta)
}

func TestFinanceRecalculationRetainedBaselineUsageOnly(t *testing.T) {
	in := FinanceRecalculationCreate{FinanceRecalculationFilter: FinanceRecalculationFilter{Kind: FinanceRecalculationUsageCost, Start: "2026-10-25", End: "2026-10-26", AcceptRetainedBaseline: true}, ActorID: 1, IdempotencyKey: "baseline"}
	require.NoError(t, validateFinanceRecalculationCreate(in))
	in.Kind = FinanceRecalculationPaymentFee
	require.ErrorIs(t, validateFinanceRecalculationCreate(in), ErrFinanceRecalculationInvalid)
	in.AcceptRetainedBaseline = false
	require.NoError(t, validateFinanceRecalculationCreate(in))
}

func TestFinanceRecalculationServiceCancelValidatesInput(t *testing.T) {
	s, store, _, _ := financeOrchestrationFixture(FinanceRecalculationReady, FinanceRecalculationUsageCost)
	_, err := s.Cancel(context.Background(), 0, 9)
	require.ErrorIs(t, err, ErrFinanceRecalculationInvalid)
	_, err = s.Cancel(context.Background(), 1, 0)
	require.ErrorIs(t, err, ErrFinanceRecalculationInvalid)
	require.Empty(t, store.calls)
	b, err := s.Cancel(context.Background(), 1, 9)
	require.NoError(t, err)
	require.Equal(t, FinanceRecalculationCancelled, b.Status)
}
