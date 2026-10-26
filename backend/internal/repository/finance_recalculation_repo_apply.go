package repository

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

// Normalize the UI amount fields from the calculation result, rather than
// trusting caller totals. SQL snapshots preserve NULL vs explicit zero.
func financeNormalizeItem(kind string, i *service.FinanceRecalculationItem) error {
	if !financeSafeLabel(i.Reason) || !financeSafeLabel(i.Source) {
		return service.ErrFinanceRecalculationInvalid
	}
	if i.Status != service.FinanceRecalculationChanged && i.Status != service.FinanceRecalculationUnchanged && i.Status != service.FinanceRecalculationSkipped {
		return service.ErrFinanceRecalculationInvalid
	}
	if i.Applied {
		return service.ErrFinanceRecalculationInvalid
	}
	if i.Status == service.FinanceRecalculationSkipped {
		i.NewCost = i.OldCost
		i.Delta = nil
		return nil
	}
	var old, next, delta string
	if kind == service.FinanceRecalculationUsageCost {
		in, out := i.Candidate.Usage, i.UsageResult
		if in == nil || out == nil || i.PaymentResult != nil || out.Status != i.Status || !financeOptionalEqual(in.AccountStatsCost, out.OldAccountStatsCost) || !financeOptionalEqual(in.AccountRateMultiplier, out.OldAccountRateMultiplier) {
			return service.ErrFinanceRecalculationInvalid
		}
		oldBase, err := financeDecimal(in.TotalCost, 10, "10000000000")
		if err != nil {
			return err
		}
		if in.AccountStatsCost != nil {
			oldBase, err = financeDecimal(*in.AccountStatsCost, 10, "10000000000")
			if err != nil {
				return err
			}
		}
		oldRate := decimal.NewFromInt(1)
		if in.AccountRateMultiplier != nil {
			oldRate, err = financeDecimal(*in.AccountRateMultiplier, 4, "1000000")
			if err != nil {
				return err
			}
		}
		newBase, err := financeDecimal(in.TotalCost, 10, "10000000000")
		if err != nil {
			return err
		}
		if out.NewAccountStatsCost != nil {
			newBase, err = financeDecimal(*out.NewAccountStatsCost, 10, "10000000000")
			if err != nil {
				return err
			}
		}
		newRate := decimal.NewFromInt(1)
		if out.NewAccountRateMultiplier != nil {
			newRate, err = financeDecimal(*out.NewAccountRateMultiplier, 4, "1000000")
			if err != nil {
				return err
			}
		}
		if oldBase.IsNegative() || oldRate.IsNegative() || newBase.IsNegative() || newRate.IsNegative() {
			return service.ErrFinanceRecalculationInvalid
		}
		old = oldBase.Mul(oldRate).String()
		next = newBase.Mul(newRate).String()
		delta = newBase.Mul(newRate).Sub(oldBase.Mul(oldRate)).String()
		same := financeOptionalEqual(in.AccountStatsCost, out.NewAccountStatsCost) && financeOptionalEqual(in.AccountRateMultiplier, out.NewAccountRateMultiplier)
		if same != (i.Status == service.FinanceRecalculationUnchanged) {
			return service.ErrFinanceRecalculationInvalid
		}
		if !financeOptionalEqual(&old, &out.OldCost) || !financeOptionalEqual(&next, &out.NewCost) || !financeOptionalEqual(&delta, &out.Delta) {
			return service.ErrFinanceRecalculationInvalid
		}
	} else {
		in, out := i.Candidate.Payment, i.PaymentResult
		if in == nil || out == nil || i.UsageResult != nil || out.Status != i.Status || !financeOptionalEqual(in.OriginalFeeRate, out.OriginalFeeRate) {
			return service.ErrFinanceRecalculationInvalid
		}
		calculated, err := service.CalculateFinancePaymentFee(*in)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(calculated, *out) {
			return service.ErrFinanceRecalculationInvalid
		}
		old, next, delta = out.OldFee, out.NewFee, out.Delta
	}
	i.OldCost = &old
	i.NewCost = &next
	i.Delta = &delta
	return nil
}
func (r *financeRecalculationRepository) SavePreview(ctx context.Context, id int64, token string, p service.FinanceRecalculationPreview) error {
	if len(p.Items) > service.FinanceRecalculationMaxRows {
		return service.ErrFinanceRecalculationTooLarge
	}
	if p.ConfigFingerprint == "" || len(p.ConfigFingerprint) > 256 {
		return service.ErrFinanceRecalculationInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	b, err := financeLockedBatch(ctx, tx, id, token, service.FinanceRecalculationPreviewing)
	if err != nil {
		return err
	}
	var expired bool
	if err = tx.QueryRowContext(ctx, `SELECT expires_at<=clock_timestamp() FROM finance_recalculation_batches WHERE id=$1`, id).Scan(&expired); err != nil {
		return err
	}
	if expired {
		return service.ErrFinanceRecalculationExpired
	}
	// Re-read the entire bounded scope: prevent silently truncated previews and
	// detect retention/input changes during calculation. New rows after CutoffAt
	// are deliberately excluded. Apply later validates exactly these identities.
	candidates, err := financeReadCandidates(ctx, tx, b)
	if err != nil {
		return err
	}
	if len(candidates) != len(p.Items) {
		return service.ErrFinanceRecalculationSourceChanged
	}
	sort.Slice(p.Items, func(i, j int) bool {
		a, c := p.Items[i].Candidate, p.Items[j].Candidate
		if a.TargetCreatedAt.Equal(c.TargetCreatedAt) {
			return a.TargetID < c.TargetID
		}
		return a.TargetCreatedAt.Before(c.TargetCreatedAt)
	})
	changed, unchanged, skipped := 0, 0, 0
	oldTotal, newTotal := decimal.Zero, decimal.Zero
	for n := range p.Items {
		i := &p.Items[n]
		expected := candidates[n]
		if i.Candidate.TargetID != expected.TargetID || !i.Candidate.TargetCreatedAt.Equal(expected.TargetCreatedAt) || i.Candidate.SourceFingerprint != expected.SourceFingerprint {
			return service.ErrFinanceRecalculationSourceChanged
		}
		// Inputs come from the DB, not from a mutable caller payload. Current payment
		// configuration may only differ if the orchestrator froze it explicitly.
		if !reflect.DeepEqual(i.Candidate.Usage, expected.Usage) {
			return service.ErrFinanceRecalculationSourceChanged
		}
		if expected.Payment != nil {
			if i.Candidate.Payment == nil {
				return service.ErrFinanceRecalculationInvalid
			}
			actual := *expected.Payment
			actual.CurrentFeeRate = i.Candidate.Payment.CurrentFeeRate
			if !reflect.DeepEqual(*i.Candidate.Payment, actual) {
				return service.ErrFinanceRecalculationSourceChanged
			}
			expected.Payment.CurrentFeeRate = actual.CurrentFeeRate
		}
		i.Candidate = expected
		if err = financeNormalizeItem(b.Kind, i); err != nil {
			return err
		}
		switch i.Status {
		case service.FinanceRecalculationChanged:
			changed++
		case service.FinanceRecalculationUnchanged:
			unchanged++
		case service.FinanceRecalculationSkipped:
			skipped++
		}
		if i.OldCost != nil {
			v, e := financeDecimal(*i.OldCost, 14, "10000000000000000")
			if e != nil {
				return e
			}
			oldTotal = oldTotal.Add(v)
		}
		if i.NewCost != nil {
			v, e := financeDecimal(*i.NewCost, 14, "10000000000000000")
			if e != nil {
				return e
			}
			newTotal = newTotal.Add(v)
		}
		candidate, e := financeSnapshotJSON(i.Candidate)
		if e != nil {
			return e
		}
		usage, e := financeSnapshotJSON(i.UsageResult)
		if e != nil {
			return e
		}
		payment, e := financeSnapshotJSON(i.PaymentResult)
		if e != nil {
			return e
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO finance_recalculation_items(batch_id,target_id,target_created_at,source_fingerprint,status,reason,source,old_cost,new_cost,delta,candidate,usage_result,payment_result) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, id, i.Candidate.TargetID, i.Candidate.TargetCreatedAt, i.Candidate.SourceFingerprint, i.Status, i.Reason, i.Source, i.OldCost, i.NewCost, i.Delta, candidate, usage, payment)
		if err != nil {
			return err
		}
	}
	config, err := json.Marshal(p.Config)
	if err != nil {
		return err
	}
	err = financeAffected(tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET scanned=$3,changed=$4,unchanged=$5,skipped=$6,old_total=$7,new_total=$8,delta=$9,config_fingerprint=$10,config_snapshot=$11,status='ready',lease_token='',lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND lease_until>clock_timestamp() AND expires_at>clock_timestamp()`, id, token, len(p.Items), changed, unchanged, skipped, oldTotal.String(), newTotal.String(), newTotal.Sub(oldTotal).String(), p.ConfigFingerprint, string(config)))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *financeRecalculationRepository) Apply(ctx context.Context, id int64, token, actualFingerprint string) error {
	if r.guard == nil {
		return service.ErrFinanceRecalculationSyncRequired
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var release func()
	defer func() {
		// Roll back before releasing the configuration lock on every failure
		// path. On success Commit has already completed; Rollback is a no-op.
		_ = tx.Rollback()
		if release != nil {
			release()
		}
	}()
	b, err := financeLockedBatch(ctx, tx, id, token, service.FinanceRecalculationApplying)
	if err != nil {
		return err
	}
	if b.ConfigFingerprint == "" || b.ConfigFingerprint != actualFingerprint {
		return service.ErrFinanceRecalculationRuleChanged
	}
	var guardedFingerprint string
	guardedFingerprint, release, err = r.guard.Acquire(ctx, tx, b)
	if err != nil {
		return err
	}
	if guardedFingerprint != b.ConfigFingerprint {
		return service.ErrFinanceRecalculationRuleChanged
	}
	items, err := financeLoadItems(ctx, tx, id, service.FinanceRecalculationMaxRows+1, 0)
	if err != nil {
		return err
	}
	if len(items) != b.Scanned || len(items) > service.FinanceRecalculationMaxRows {
		return service.ErrFinanceRecalculationInvalid
	}
	// Lock/validate ALL rows before any write. Missing or changed skipped/unchanged
	// sources also invalidate the preview. The bounded batch is all-or-nothing.
	for n := range items {
		i := &items[n]
		if i.Applied {
			return service.ErrFinanceRecalculationState
		}
		locked, e := financeLockCandidate(ctx, tx, b.Kind, i.Candidate)
		if e != nil {
			return e
		}
		if locked.SourceFingerprint != i.Candidate.SourceFingerprint {
			return service.ErrFinanceRecalculationSourceChanged
		}
		if err = financeNormalizeItem(b.Kind, i); err != nil {
			return err
		}
	}
	for _, i := range items {
		if i.Status != service.FinanceRecalculationChanged {
			continue
		}
		if b.Kind == service.FinanceRecalculationUsageCost {
			v := i.UsageResult
			err = financeAffected(tx.ExecContext(ctx, `UPDATE usage_logs SET account_stats_cost=$3,account_rate_multiplier=$4 WHERE id=$1 AND created_at=$2`, i.Candidate.TargetID, i.Candidate.TargetCreatedAt, v.NewAccountStatsCost, v.NewAccountRateMultiplier))
		} else {
			err = financeAffected(tx.ExecContext(ctx, `UPDATE payment_orders SET channel_fee_rate=$3 WHERE id=$1 AND created_at=$2`, i.Candidate.TargetID, i.Candidate.TargetCreatedAt, i.PaymentResult.NewFeeRate))
		}
		if err != nil {
			return err
		}
		err = financeAffected(tx.ExecContext(ctx, `UPDATE finance_recalculation_items SET applied=TRUE,applied_at=clock_timestamp() WHERE id=$1 AND NOT applied`, i.ID))
		if err != nil {
			return err
		}
	}
	if err = r.guard.Synchronize(ctx, tx, b, items); err != nil {
		return err
	}
	// Retain the lease for the worker's post-commit cache fence; crash recovery
	// claims syncing and must never invoke Apply again.
	if err = financeEndWorker(ctx, tx, id, token, service.FinanceRecalculationSyncing, false); err != nil {
		return err
	}
	return tx.Commit()
}
