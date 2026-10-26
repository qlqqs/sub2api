package service

// CUSTOM(fork): durable, bounded finance repair jobs. These ports never authorize
// balance, quota, customer-price, refund-amount or ledger mutations.
import (
	"context"
	"encoding/json"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const FinanceRecalculationMaxRows = 2000
const (
	FinanceRecalculationUsageCost  = "usage_cost"
	FinanceRecalculationPaymentFee = "payment_fee"
	FinanceRecalculationPreviewing = "previewing"
	FinanceRecalculationReady      = "ready"
	FinanceRecalculationQueued     = "queued"
	FinanceRecalculationApplying   = "applying"
	FinanceRecalculationSyncing    = "syncing"
	FinanceRecalculationCompleted  = "completed"
	FinanceRecalculationFailed     = "failed"
	FinanceRecalculationConflict   = "conflict"
	FinanceRecalculationExpired    = "expired"
	FinanceRecalculationCancelled  = "cancelled"
)

var (
	ErrFinanceRecalculationNotFound      = infraerrors.NotFound("FINANCE_RECALCULATION_NOT_FOUND", "Recalculation not found")
	ErrFinanceRecalculationInvalid       = infraerrors.BadRequest("FINANCE_RECALCULATION_INVALID", "Invalid recalculation request")
	ErrFinanceRecalculationTooLarge      = infraerrors.BadRequest("FINANCE_RECALCULATION_TOO_LARGE", "Preview exceeds 2000 source rows or the 31-day retained-baseline scope; use a smaller range")
	ErrFinanceRecalculationState         = infraerrors.Conflict("FINANCE_RECALCULATION_STATE", "Recalculation state or lease changed")
	ErrFinanceRecalculationActor         = infraerrors.Forbidden("FINANCE_RECALCULATION_ACTOR", "Only the preview creator can confirm, cancel or retry")
	ErrFinanceRecalculationExpired       = infraerrors.Conflict("FINANCE_RECALCULATION_EXPIRED", "Preview expired; create a new preview")
	ErrFinanceRecalculationRuleChanged   = infraerrors.Conflict("FINANCE_RECALCULATION_RULE_CHANGED", "Cost configuration changed; create a new preview")
	ErrFinanceRecalculationSourceChanged = infraerrors.Conflict("FINANCE_RECALCULATION_SOURCE_CHANGED", "Source rows changed; create a new preview")
	ErrFinanceRecalculationSyncRequired  = infraerrors.ServiceUnavailable("FINANCE_RECALCULATION_SYNC_REQUIRED", "Transactional synchronization is not configured")
)

type FinanceRecalculationFilter struct {
	// Explicit acceptance of retained detail as the legacy baseline; never inferred.
	AcceptRetainedBaseline bool   `json:"accept_retained_baseline,omitempty"`
	Kind                   string `json:"kind"`
	Start                  string `json:"start"` // inclusive Beijing calendar day
	End                    string `json:"end"`   // inclusive Beijing calendar day
	AccountID              *int64 `json:"account_id"`
	ProviderID             *int64 `json:"provider_id"`
	Model                  string `json:"model"`
}
type FinanceRecalculationCreate struct {
	FinanceRecalculationFilter
	ActorID int64
	// Caller-generated request key, scoped to actor. Reuse with a different filter is rejected.
	IdempotencyKey string
}
type FinanceRecalculationBatch struct {
	ID int64 `json:"id"`
	FinanceRecalculationFilter
	Status            string                     `json:"status"`
	Scanned           int                        `json:"scanned"`
	Changed           int                        `json:"changed"`
	Unchanged         int                        `json:"unchanged"`
	Skipped           int                        `json:"skipped"`
	OldTotal          string                     `json:"old_total"`
	NewTotal          string                     `json:"new_total"`
	Delta             string                     `json:"delta"`
	ErrorCode         string                     `json:"error_code"`
	ErrorMessage      string                     `json:"error_message"`
	CreatedAt         time.Time                  `json:"created_at"`
	ExpiresAt         time.Time                  `json:"expires_at"`
	ActorID           int64                      `json:"actor_id"`
	CutoffAt          time.Time                  `json:"cutoff_at"`
	ConfigFingerprint string                     `json:"config_fingerprint"`
	Config            FinanceRecalculationConfig `json:"-"`
	StartAt           time.Time                  `json:"-"`
	EndAt             time.Time                  `json:"-"`
	LeaseToken        string                     `json:"-"`
	LeaseUntil        *time.Time                 `json:"-"`
	FailedPhase       string                     `json:"-"`
}

// Typed whitelist: never persist entire account/provider objects or credentials.
type FinanceRecalculationConfig struct {
	RetainedBaseline *FinanceRetainedBaseline          `json:"retained_baseline,omitempty"`
	Usage            []FinanceUsageCostSnapshot        `json:"usage,omitempty"`
	Payment          []FinanceRecalculationPaymentRule `json:"payment,omitempty"`
}
type FinanceRecalculationPaymentRule struct {
	ProviderID int64  `json:"provider_id"`
	FeeRate    string `json:"fee_rate"`
}
type FinanceRecalculationCandidate struct {
	TargetID          int64                   `json:"target_id"`
	TargetCreatedAt   time.Time               `json:"target_created_at"`
	SourceFingerprint string                  `json:"source_fingerprint"`
	Usage             *FinanceUsageCostInput  `json:"usage,omitempty"`
	Payment           *FinancePaymentFeeInput `json:"payment,omitempty"`
	// Whitelisted DB facts, including original NULLs, status and refund evidence.
	Raw json.RawMessage `json:"raw"`
}
type FinanceRecalculationItem struct {
	ID            int64                         `json:"id"`
	Status        string                        `json:"status"`
	Reason        string                        `json:"reason"`
	Source        string                        `json:"source"`
	OldCost       *string                       `json:"old_cost"`
	NewCost       *string                       `json:"new_cost"`
	Delta         *string                       `json:"delta"`
	Applied       bool                          `json:"applied"`
	Candidate     FinanceRecalculationCandidate `json:"-"`
	UsageResult   *FinanceUsageCostResult       `json:"-"`
	PaymentResult *FinancePaymentFeeResult      `json:"-"`
}
type FinanceRecalculationPreview struct {
	ConfigFingerprint string
	Config            FinanceRecalculationConfig
	Items             []FinanceRecalculationItem
}

// Claim returns nil,nil when idle. Lease tokens are unique per claim (not worker
// names); a stale worker cannot save/complete after another worker reclaims.
// Apply is atomic for the entire bounded preview, including aggregate changes.
// Apply enters syncing; Complete is called ONLY after idempotent cache fencing.
// Failed syncing is the only retryable failed phase; apply failures rolled back
// completely may be retried by lease recovery, conflicts require a new preview.
type FinanceRecalculationStore interface {
	Create(context.Context, FinanceRecalculationCreate) (*FinanceRecalculationBatch, error)
	List(context.Context, int, int) ([]FinanceRecalculationBatch, int64, error)
	Get(context.Context, int64) (*FinanceRecalculationBatch, error)
	ListItems(context.Context, int64, int, int) ([]FinanceRecalculationItem, int64, error)
	Claim(context.Context, string, time.Duration) (*FinanceRecalculationBatch, error)
	RenewLease(context.Context, int64, string, time.Duration) error
	LoadCandidates(context.Context, *FinanceRecalculationBatch) ([]FinanceRecalculationCandidate, error)
	SavePreview(context.Context, int64, string, FinanceRecalculationPreview) error
	QueueConfirm(context.Context, int64, int64, string, string) (*FinanceRecalculationBatch, error)
	Apply(context.Context, int64, string, string) error
	RetrySync(context.Context, int64, int64) error
	// Cancel discards an unconfirmed (previewing/ready) preview of the creator.
	// It clears the lease so an in-flight preview worker cannot save afterwards.
	Cancel(context.Context, int64, int64) (*FinanceRecalculationBatch, error)
	Complete(context.Context, int64, string) error
	Fail(context.Context, int64, string, string, string) error
}
