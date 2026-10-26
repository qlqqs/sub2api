package repository

// CUSTOM(fork): all updates are confined to the two cost snapshot columns on
// usage_logs and channel_fee_rate on orders. Audit and synchronization commit
// together; this repository does not provide an undo operation.
import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

// FinanceRecalculationTransactionGuard MUST participate in the same lock/version
// protocol as configuration writers, aggregation workers and retention cleanup.
// Acquire runs before source row locks and must hold protection until tx ends;
// its returned fingerprint is the current canonical configuration fingerprint.
// The optional release closure relinquishes in-memory protection only AFTER
// commit or rollback. A nonnil release is also called when Acquire returns error.
// Synchronize runs AFTER cost writes in the SAME transaction and must correct
// aggregates without changing revenue/counts or reconstructing partial history.
// A caller-supplied fingerprint alone is NOT atomic configuration protection.
// Production must not install a no-op guard. Without a guard Apply fails closed.
type FinanceRecalculationTransactionGuard interface {
	Acquire(context.Context, *sql.Tx, *service.FinanceRecalculationBatch) (fingerprint string, release func(), err error)
	Synchronize(context.Context, *sql.Tx, *service.FinanceRecalculationBatch, []service.FinanceRecalculationItem) error
}
type financeRecalculationRepository struct {
	db    *sql.DB
	guard FinanceRecalculationTransactionGuard
}

var _ service.FinanceRecalculationStore = (*financeRecalculationRepository)(nil)

func NewFinanceRecalculationRepository(db *sql.DB, guards ...FinanceRecalculationTransactionGuard) service.FinanceRecalculationStore {
	r := &financeRecalculationRepository{db: db}
	if len(guards) == 1 {
		r.guard = guards[0]
	}
	return r
}

const financeBatchColumns = `id,kind,status,actor_id,filter,start_at,end_at,cutoff_at,config_fingerprint,config_snapshot,scanned,changed,unchanged,skipped,old_total::text,new_total::text,delta::text,lease_token,lease_until,failed_phase,error_code,error_message,created_at,expires_at`

type financeScanner interface{ Scan(...any) error }

func scanFinanceBatch(row financeScanner) (*service.FinanceRecalculationBatch, error) {
	b := new(service.FinanceRecalculationBatch)
	var filter, config []byte
	var kind string
	err := row.Scan(&b.ID, &kind, &b.Status, &b.ActorID, &filter, &b.StartAt, &b.EndAt, &b.CutoffAt, &b.ConfigFingerprint, &config, &b.Scanned, &b.Changed, &b.Unchanged, &b.Skipped, &b.OldTotal, &b.NewTotal, &b.Delta, &b.LeaseToken, &b.LeaseUntil, &b.FailedPhase, &b.ErrorCode, &b.ErrorMessage, &b.CreatedAt, &b.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrFinanceRecalculationNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(filter, &b.FinanceRecalculationFilter); err != nil {
		return nil, err
	}
	if b.Kind != kind {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	if err = json.Unmarshal(config, &b.Config); err != nil {
		return nil, err
	}
	return b, nil
}
func financePage(limit, offset int) (int, int) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
func (r *financeRecalculationRepository) Create(ctx context.Context, in service.FinanceRecalculationCreate) (*service.FinanceRecalculationBatch, error) {
	if in.ActorID <= 0 || len(in.IdempotencyKey) == 0 || len(in.IdempotencyKey) > 128 || len(in.Model) > 200 {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	if in.Kind != service.FinanceRecalculationUsageCost && in.Kind != service.FinanceRecalculationPaymentFee {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	if in.AccountID != nil && *in.AccountID <= 0 || in.ProviderID != nil && *in.ProviderID <= 0 {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	if in.Kind == service.FinanceRecalculationUsageCost && in.ProviderID != nil || in.Kind == service.FinanceRecalculationPaymentFee && (in.AccountID != nil || in.Model != "" || in.AcceptRetainedBaseline) {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	start, err := time.ParseInLocation("2006-01-02", in.Start, loc)
	if err != nil {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	end, err := time.ParseInLocation("2006-01-02", in.End, loc)
	if err != nil {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	end = end.AddDate(0, 0, 1)
	if !start.Before(end) || end.Sub(start) > 732*24*time.Hour {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	filter, err := json.Marshal(in.FinanceRecalculationFilter)
	if err != nil {
		return nil, err
	}
	// A harmless no-op update makes concurrent duplicate creation return the same
	// row without a SELECT snapshot race. Compare the immutable filter afterwards.
	b, err := scanFinanceBatch(r.db.QueryRowContext(ctx, `INSERT INTO finance_recalculation_batches(kind,actor_id,idempotency_key,filter,start_at,end_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(actor_id,idempotency_key) DO UPDATE SET idempotency_key=EXCLUDED.idempotency_key RETURNING `+financeBatchColumns, in.Kind, in.ActorID, in.IdempotencyKey, string(filter), start, end))
	if err == nil && !reflect.DeepEqual(b.FinanceRecalculationFilter, in.FinanceRecalculationFilter) {
		return nil, service.ErrFinanceRecalculationState
	}
	return b, err
}
func (r *financeRecalculationRepository) Get(ctx context.Context, id int64) (*service.FinanceRecalculationBatch, error) {
	return scanFinanceBatch(r.db.QueryRowContext(ctx, `SELECT `+financeBatchColumns+` FROM finance_recalculation_batches WHERE id=$1`, id))
}
func (r *financeRecalculationRepository) List(ctx context.Context, limit, offset int) ([]service.FinanceRecalculationBatch, int64, error) {
	limit, offset = financePage(limit, offset)
	out := []service.FinanceRecalculationBatch{}
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM finance_recalculation_batches`).Scan(&count); err != nil {
		return nil, 0, err
	}
	err := financeRows(ctx, r.db, `SELECT `+financeBatchColumns+` FROM finance_recalculation_batches ORDER BY id DESC LIMIT $1 OFFSET $2`, func(rows *sql.Rows) error {
		b, e := scanFinanceBatch(rows)
		if e == nil {
			out = append(out, *b)
		}
		return e
	}, limit, offset)
	return out, count, err
}

const financeItemColumns = `id,status,reason,source,old_cost::text,new_cost::text,delta::text,applied,candidate,usage_result,payment_result`

func scanFinanceItem(row financeScanner) (service.FinanceRecalculationItem, error) {
	var i service.FinanceRecalculationItem
	var c, u, p []byte
	err := row.Scan(&i.ID, &i.Status, &i.Reason, &i.Source, &i.OldCost, &i.NewCost, &i.Delta, &i.Applied, &c, &u, &p)
	if err != nil {
		return i, err
	}
	if err = json.Unmarshal(c, &i.Candidate); err != nil {
		return i, err
	}
	if len(u) > 0 {
		if err = json.Unmarshal(u, &i.UsageResult); err != nil {
			return i, err
		}
	}
	if len(p) > 0 {
		err = json.Unmarshal(p, &i.PaymentResult)
	}
	return i, err
}
func financeLoadItems(ctx context.Context, q financeQueryer, id int64, limit, offset int) ([]service.FinanceRecalculationItem, error) {
	out := []service.FinanceRecalculationItem{}
	err := financeRows(ctx, q, `SELECT `+financeItemColumns+` FROM finance_recalculation_items WHERE batch_id=$1 ORDER BY target_created_at,target_id LIMIT $2 OFFSET $3`, func(rows *sql.Rows) error {
		i, e := scanFinanceItem(rows)
		if e == nil {
			out = append(out, i)
		}
		return e
	}, id, limit, offset)
	return out, err
}
func (r *financeRecalculationRepository) ListItems(ctx context.Context, id int64, limit, offset int) ([]service.FinanceRecalculationItem, int64, error) {
	b, err := r.Get(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	limit, offset = financePage(limit, offset)
	out, err := financeLoadItems(ctx, r.db, id, limit, offset)
	return out, int64(b.Scanned), err
}
func financeLeaseDuration(d time.Duration) (int64, error) {
	if d < time.Second || d > 30*time.Minute {
		return 0, service.ErrFinanceRecalculationInvalid
	}
	return int64(d / time.Second), nil
}
func (r *financeRecalculationRepository) Claim(ctx context.Context, worker string, d time.Duration) (*service.FinanceRecalculationBatch, error) {
	seconds, err := financeLeaseDuration(d)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(worker) == "" {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	token := make([]byte, 24)
	if _, err = rand.Read(token); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET status='expired',lease_token='',lease_until=NULL,error_code='preview_expired',error_message='Preview expired',updated_at=clock_timestamp() WHERE status IN ('previewing','ready') AND expires_at<=clock_timestamp() AND (lease_until IS NULL OR lease_until<=clock_timestamp())`); err != nil {
		return nil, err
	}
	b, err := scanFinanceBatch(tx.QueryRowContext(ctx, `UPDATE finance_recalculation_batches SET status=CASE WHEN status='queued' THEN 'applying' ELSE status END,lease_token=$1,lease_until=clock_timestamp()+$2*INTERVAL '1 second',updated_at=clock_timestamp() WHERE id=(SELECT id FROM finance_recalculation_batches WHERE status IN ('previewing','queued','applying','syncing') AND (lease_until IS NULL OR lease_until<=clock_timestamp()) ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+financeBatchColumns, hex.EncodeToString(token), seconds))
	if errors.Is(err, service.ErrFinanceRecalculationNotFound) {
		return nil, tx.Commit()
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return b, nil
}
func financeAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrFinanceRecalculationState
	}
	return nil
}
func (r *financeRecalculationRepository) RenewLease(ctx context.Context, id int64, token string, d time.Duration) error {
	seconds, err := financeLeaseDuration(d)
	if err != nil {
		return err
	}
	return financeAffected(r.db.ExecContext(ctx, `UPDATE finance_recalculation_batches SET lease_until=clock_timestamp()+$3*INTERVAL '1 second',updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND lease_token<>'' AND lease_until>clock_timestamp() AND status IN ('previewing','applying','syncing')`, id, token, seconds))
}
func financeLockedBatch(ctx context.Context, tx *sql.Tx, id int64, token, phase string) (*service.FinanceRecalculationBatch, error) {
	b, err := scanFinanceBatch(tx.QueryRowContext(ctx, `SELECT `+financeBatchColumns+` FROM finance_recalculation_batches WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	var valid bool
	if err = tx.QueryRowContext(ctx, `SELECT lease_token=$2 AND lease_token<>'' AND lease_until>clock_timestamp() FROM finance_recalculation_batches WHERE id=$1`, id, token).Scan(&valid); err != nil {
		return nil, err
	}
	if !valid || b.Status != phase {
		return nil, service.ErrFinanceRecalculationState
	}
	return b, nil
}
func financeEndWorker(ctx context.Context, tx *sql.Tx, id int64, token, status string, release bool) error {
	return financeAffected(tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET status=$3,lease_token=CASE WHEN $4 THEN '' ELSE lease_token END,lease_until=CASE WHEN $4 THEN NULL ELSE lease_until END,updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND lease_token<>'' AND lease_until>clock_timestamp()`, id, token, status, release))
}
func (r *financeRecalculationRepository) QueueConfirm(ctx context.Context, id, actor int64, expected, actual string) (*service.FinanceRecalculationBatch, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	b, err := scanFinanceBatch(tx.QueryRowContext(ctx, `SELECT `+financeBatchColumns+` FROM finance_recalculation_batches WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if b.ActorID != actor {
		return nil, service.ErrFinanceRecalculationActor
	}
	// Replayed confirmation is idempotent even after expiry or later config edits.
	if b.Status == service.FinanceRecalculationQueued || b.Status == service.FinanceRecalculationApplying || b.Status == service.FinanceRecalculationSyncing || b.Status == service.FinanceRecalculationCompleted {
		if expected != b.ConfigFingerprint {
			return nil, service.ErrFinanceRecalculationRuleChanged
		}
		return b, tx.Commit()
	}
	if b.Status != service.FinanceRecalculationReady {
		return nil, service.ErrFinanceRecalculationState
	}
	var expired bool
	if err = tx.QueryRowContext(ctx, `SELECT expires_at<=clock_timestamp() FROM finance_recalculation_batches WHERE id=$1`, id).Scan(&expired); err != nil {
		return nil, err
	}
	if expired {
		if _, err = tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET status='expired',error_code='preview_expired',error_message='Preview expired' WHERE id=$1`, id); err != nil {
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return nil, service.ErrFinanceRecalculationExpired
	}
	if expected == "" || expected != b.ConfigFingerprint || actual != expected {
		return nil, service.ErrFinanceRecalculationRuleChanged
	}
	b, err = scanFinanceBatch(tx.QueryRowContext(ctx, `UPDATE finance_recalculation_batches SET status='queued',confirmed_at=clock_timestamp(),updated_at=clock_timestamp() WHERE id=$1 RETURNING `+financeBatchColumns, id))
	if err != nil {
		return nil, err
	}
	return b, tx.Commit()
}
func (r *financeRecalculationRepository) RetrySync(ctx context.Context, id, actor int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	b, err := scanFinanceBatch(tx.QueryRowContext(ctx, `SELECT `+financeBatchColumns+` FROM finance_recalculation_batches WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if b.ActorID != actor {
		return service.ErrFinanceRecalculationActor
	}
	if b.Status == service.FinanceRecalculationSyncing || b.Status == service.FinanceRecalculationCompleted {
		return tx.Commit()
	}
	if b.Status != service.FinanceRecalculationFailed || b.FailedPhase != service.FinanceRecalculationSyncing {
		return service.ErrFinanceRecalculationState
	}
	if _, err = tx.ExecContext(ctx, `UPDATE finance_recalculation_batches SET status='syncing',error_code='',error_message='',lease_token='',lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *financeRecalculationRepository) Complete(ctx context.Context, id int64, token string) error {
	return financeAffected(r.db.ExecContext(ctx, `UPDATE finance_recalculation_batches SET status='completed',completed_at=clock_timestamp(),updated_at=clock_timestamp(),lease_token='',lease_until=NULL,error_code='',error_message='' WHERE id=$1 AND status='syncing' AND lease_token=$2 AND lease_token<>'' AND lease_until>clock_timestamp()`, id, token))
}
func (r *financeRecalculationRepository) Fail(ctx context.Context, id int64, token, code, message string) error {
	// The worker supplies a stable public classification, never a driver/upstream
	// error string. Do not persist the untrusted message argument.
	_ = message
	messages := map[string]string{"coverage_unknown": "Historical source coverage is incomplete; no costs or aggregates were changed", "preview_failed": "Preview failed", "preview_too_large": "Preview exceeds 2000 rows", "apply_failed": "Apply failed; no changes committed", "sync_failed": "Cache synchronization failed", "source_changed": "Source rows changed", "rule_changed": "Cost configuration changed", "preview_expired": "Preview expired"}
	safe, ok := messages[code]
	if !ok {
		return service.ErrFinanceRecalculationInvalid
	}
	status := "failed"
	if code == "source_changed" || code == "rule_changed" {
		status = "conflict"
	}
	if code == "preview_expired" {
		status = "expired"
	}
	return financeAffected(r.db.ExecContext(ctx, `UPDATE finance_recalculation_batches SET failed_phase=status,status=$3,error_code=$4,error_message=$5,lease_token='',lease_until=NULL,updated_at=clock_timestamp() WHERE id=$1 AND lease_token=$2 AND lease_token<>'' AND lease_until>clock_timestamp() AND status IN ('previewing','applying','syncing')`, id, token, status, code, safe))
}

func financeDecimal(s string, scale int32, limit string) (decimal.Decimal, error) {
	if len(s) == 0 || len(s) > 80 || strings.ContainsAny(s, "eE") {
		return decimal.Zero, service.ErrFinanceRecalculationInvalid
	}
	d, err := decimal.NewFromString(s)
	if err != nil || !d.Equal(d.Round(scale)) || d.Abs().GreaterThanOrEqual(decimal.RequireFromString(limit)) {
		return decimal.Zero, service.ErrFinanceRecalculationInvalid
	}
	return d, nil
}
func financeOptionalEqual(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	x, e := decimal.NewFromString(*a)
	y, f := decimal.NewFromString(*b)
	return e == nil && f == nil && x.Equal(y)
}
func financeSnapshotJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return nil, nil
	}
	return string(raw), nil
}
func financeSafeLabel(s string) bool {
	if len(s) > 120 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
