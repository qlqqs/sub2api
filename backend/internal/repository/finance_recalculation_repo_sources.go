package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Keep this whitelist explicit. to_jsonb(whole_row) would leak order credentials
// and PII. Every monetary input is cast to text before JSON encoding: no float
// conversion, and original nullable fields remain JSON null. Epoch timestamps
// in evidence are independent of the session timezone.
const financeUsageEvidence = `jsonb_build_object(
 'id',u.id,'created_epoch',extract(epoch from u.created_at)::text,
 'account_id',u.account_id,'group_id',u.group_id,'model',u.model,
 'requested_model',u.requested_model,'upstream_model',u.upstream_model,
 'upstream_response_model',u.upstream_response_model,'channel_id',u.channel_id,
 'service_tier',u.service_tier,'reasoning_effort',u.reasoning_effort,
 'billing_mode',u.billing_mode,'media_type',CASE WHEN u.video_count>0 OR u.billing_mode='video' THEN 'video' WHEN u.image_count>0 OR u.billing_mode='image' THEN 'image' ELSE NULL END,
 'video_count',u.video_count,'image_count',u.image_count,
 'image_cache_read_tokens',u.image_size_breakdown->'image_cache_read_tokens',
 'image_cache_split_present',COALESCE(u.image_size_breakdown ? 'image_cache_read_tokens',FALSE),
 'inbound_endpoint',u.inbound_endpoint,'upstream_endpoint',u.upstream_endpoint,
 'total_cost',u.total_cost::text,'actual_cost',u.actual_cost::text,
 'rate_multiplier',u.rate_multiplier::text,'billing_type',u.billing_type,
 'account_stats_cost',u.account_stats_cost::text,'account_rate_multiplier',u.account_rate_multiplier::text,
 'tokens',jsonb_build_object('InputTokens',u.input_tokens,'OutputTokens',u.output_tokens,
 'CacheCreationTokens',u.cache_creation_tokens,'CacheReadTokens',u.cache_read_tokens,
 'CacheCreation5mTokens',u.cache_creation_5m_tokens,'CacheCreation1hTokens',u.cache_creation_1h_tokens,
 'ImageInputTokens',u.image_input_tokens,'ImageOutputTokens',u.image_output_tokens))`
const financePaymentEvidence = `jsonb_build_object(
 'order_id',o.id,'created_epoch',extract(epoch from o.created_at)::text,
 'provider_instance_id',o.provider_instance_id,'pay_amount',o.pay_amount::text,
 'original_fee_rate',o.channel_fee_rate::text,'status',o.status,'order_type',o.order_type,
 'amount',o.amount::text,'bonus_amount',o.bonus_amount::text,'fee_rate',o.fee_rate::text,
 'paid_epoch',extract(epoch from o.paid_at)::text,'completed_epoch',extract(epoch from o.completed_at)::text,
 'refund_amount',o.refund_amount::text,'refund_balance_deducted',o.refund_balance_deducted::text,
 'refund_epoch',extract(epoch from o.refund_at)::text,
 'refund_requested_epoch',extract(epoch from o.refund_requested_at)::text,'force_refund',o.force_refund)`

func financeCandidate(kind string, id int64, created time.Time, raw []byte, currentFee string) (service.FinanceRecalculationCandidate, error) {
	hash := sha256.Sum256(raw)
	c := service.FinanceRecalculationCandidate{TargetID: id, TargetCreatedAt: created, SourceFingerprint: hex.EncodeToString(hash[:]), Raw: json.RawMessage(raw)}
	if kind == service.FinanceRecalculationUsageCost {
		in := new(service.FinanceUsageCostInput)
		if err := json.Unmarshal(raw, in); err != nil {
			return c, err
		}
		in.CreatedAt = created
		// OpenAI stores raw image input and the cached part in this explicit
		// JSONB key. Other writers do not persist image-input tokens. Preserve
		// unknown/malformed evidence in Raw for the orchestrator to skip safely.
		var split struct {
			Cached json.RawMessage `json:"image_cache_read_tokens"`
		}
		if err := json.Unmarshal(raw, &split); err != nil {
			return c, err
		}
		in.ImageCacheSplitKnown = in.Tokens.ImageInputTokens == 0 || in.Tokens.CacheReadTokens == 0
		var cached int
		if len(split.Cached) > 0 && string(split.Cached) != "null" && json.Unmarshal(split.Cached, &cached) == nil && cached >= 0 && cached <= in.Tokens.ImageInputTokens && cached <= in.Tokens.CacheReadTokens {
			in.ImageCacheSplitKnown = true
			in.Tokens.ImageCacheReadTokens = cached
			in.Tokens.ImageInputTokens -= cached
		}
		// There is no persisted request pricing_at, Gemini token-hours/storage
		// amount. Leave missing evidence unknown; never
		// infer pricing time from created_at/duration or invent a zero storage cost.
		c.Usage = in
	} else {
		in := new(service.FinancePaymentFeeInput)
		if err := json.Unmarshal(raw, in); err != nil {
			return c, err
		}
		var provider struct {
			ID *string `json:"provider_instance_id"`
		}
		if err := json.Unmarshal(raw, &provider); err != nil {
			return c, err
		}
		if provider.ID != nil {
			id, err := strconv.ParseInt(*provider.ID, 10, 64)
			if err == nil && id > 0 {
				in.ProviderID = id
			}
		}
		in.CurrentFeeRate = currentFee
		c.Payment = in
	}
	return c, nil
}
func financeReadCandidates(ctx context.Context, q financeQueryer, b *service.FinanceRecalculationBatch) ([]service.FinanceRecalculationCandidate, error) {
	candidates := []service.FinanceRecalculationCandidate{}
	var query string
	var args []any
	if b.Kind == service.FinanceRecalculationUsageCost {
		query = `SELECT u.id,u.created_at,` + financeUsageEvidence + `,''::text FROM usage_logs u WHERE u.created_at>=$1 AND u.created_at<$2 AND u.created_at<=$3 AND ($4::bigint IS NULL OR u.account_id=$4) AND ($5='' OR COALESCE(NULLIF(u.upstream_model,''),NULLIF(u.requested_model,''),u.model)=$5) ORDER BY u.created_at,u.id LIMIT $6`
		args = []any{b.StartAt, b.EndAt, b.CutoffAt, b.AccountID, b.Model, service.FinanceRecalculationMaxRows + 1}
	} else {
		query = `SELECT o.id,o.created_at,` + financePaymentEvidence + `,COALESCE(p.channel_fee_rate::text,'') FROM payment_orders o LEFT JOIN payment_provider_instances p ON p.id::text=o.provider_instance_id WHERE o.completed_at>=$1 AND o.completed_at<$2 AND o.created_at<=$3 AND o.completed_at<=$3 AND o.status IN ` + paidPaymentOrderStatuses + ` AND ($4::bigint IS NULL OR o.provider_instance_id=$4::text) ORDER BY o.created_at,o.id LIMIT $5`
		args = []any{b.StartAt, b.EndAt, b.CutoffAt, b.ProviderID, service.FinanceRecalculationMaxRows + 1}
	}
	err := financeRows(ctx, q, query, func(rows *sql.Rows) error {
		var id int64
		var created time.Time
		var raw []byte
		var current string
		if e := rows.Scan(&id, &created, &raw, &current); e != nil {
			return e
		}
		c, e := financeCandidate(b.Kind, id, created, raw, current)
		if e == nil {
			candidates = append(candidates, c)
		}
		return e
	}, args...)
	if err != nil {
		return nil, err
	}
	if len(candidates) > service.FinanceRecalculationMaxRows {
		return nil, service.ErrFinanceRecalculationTooLarge
	}
	return candidates, nil
}
func (r *financeRecalculationRepository) LoadCandidates(ctx context.Context, b *service.FinanceRecalculationBatch) ([]service.FinanceRecalculationCandidate, error) {
	if b == nil {
		return nil, service.ErrFinanceRecalculationInvalid
	}
	stored, err := r.Get(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	if stored.Status != service.FinanceRecalculationPreviewing {
		return nil, service.ErrFinanceRecalculationState
	}
	return financeReadCandidates(ctx, r.db, stored)
}
func financeLockCandidate(ctx context.Context, tx *sql.Tx, kind string, c service.FinanceRecalculationCandidate) (service.FinanceRecalculationCandidate, error) {
	var raw []byte
	var err error
	if kind == service.FinanceRecalculationUsageCost {
		err = tx.QueryRowContext(ctx, `SELECT `+financeUsageEvidence+` FROM usage_logs u WHERE u.id=$1 AND u.created_at=$2 FOR UPDATE OF u`, c.TargetID, c.TargetCreatedAt).Scan(&raw)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT `+financePaymentEvidence+` FROM payment_orders o WHERE o.id=$1 AND o.created_at=$2 FOR UPDATE OF o`, c.TargetID, c.TargetCreatedAt).Scan(&raw)
	}
	if err == sql.ErrNoRows {
		return service.FinanceRecalculationCandidate{}, service.ErrFinanceRecalculationSourceChanged
	}
	if err != nil {
		return service.FinanceRecalculationCandidate{}, err
	}
	return financeCandidate(kind, c.TargetID, c.TargetCreatedAt, raw, "")
}
