package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type financeRecalculationService interface {
	Create(context.Context, service.FinanceRecalculationCreate) (*service.FinanceRecalculationBatch, error)
	List(context.Context, int, int) ([]service.FinanceRecalculationBatch, int64, error)
	Get(context.Context, int64) (*service.FinanceRecalculationBatch, error)
	Items(context.Context, int64, int, int) ([]service.FinanceRecalculationItem, int64, error)
	Confirm(context.Context, int64, int64, string, bool) (*service.FinanceRecalculationBatch, error)
	RetrySync(context.Context, int64, int64) (*service.FinanceRecalculationBatch, error)
}
type FinanceRecalculationHandler struct{ s financeRecalculationService }

func NewFinanceRecalculationHandler(s *service.FinanceRecalculationService) *FinanceRecalculationHandler {
	return &FinanceRecalculationHandler{s: s}
}

// Explicit public whitelist. Never serialize batch.Config, candidate evidence,
// lease tokens or request payloads even if their internal JSON tags change.
type financeRecalculationBatchResponse struct {
	service.FinanceRecalculationFilter
	ID                int64     `json:"id"`
	Status            string    `json:"status"`
	Scanned           int       `json:"scanned"`
	Changed           int       `json:"changed"`
	Unchanged         int       `json:"unchanged"`
	Skipped           int       `json:"skipped"`
	OldTotal          string    `json:"old_total"`
	NewTotal          string    `json:"new_total"`
	Delta             string    `json:"delta"`
	ActorID           int64     `json:"actor_id"`
	ConfigFingerprint string    `json:"config_fingerprint"`
	CutoffAt          time.Time `json:"cutoff_at"`
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
	ErrorCode         string    `json:"error_code"`
	ErrorMessage      string    `json:"error_message"`
	AmountUnit        string    `json:"amount_unit"`
	SnapshotSummary   []string  `json:"snapshot_summary"`
	FailedPhase       string    `json:"failed_phase"`
}
type financeRecalculationItemResponse struct {
	ID      int64   `json:"id"`
	Status  string  `json:"status"`
	Reason  string  `json:"reason"`
	Source  string  `json:"source"`
	OldCost *string `json:"old_cost"`
	NewCost *string `json:"new_cost"`
	Delta   *string `json:"delta"`
	Applied bool    `json:"applied"`
}

func financeRecalculationPublicFailure(code string) (string, string) {
	messages := map[string]string{
		"preview_failed": "Preview failed", "preview_too_large": "Preview exceeds 2000 source rows or the 31-day retained-baseline scope; use a smaller range",
		"apply_failed": "Apply failed; no changes committed", "sync_failed": "Cache synchronization failed; retry synchronization",
		"source_changed": "Source rows changed; create a new preview", "rule_changed": "Cost configuration changed; create a new preview",
		"preview_expired":  "Preview expired; create a new preview",
		"coverage_unknown": "Source coverage is unproven or retained-baseline reconciliation failed; no costs or aggregates were changed",
	}
	if code == "" {
		return "", ""
	}
	if message, ok := messages[code]; ok {
		return code, message
	}
	return "recalculation_failed", "Recalculation failed"
}
func financeRecalculationBatchToResponse(b *service.FinanceRecalculationBatch) financeRecalculationBatchResponse {
	code, message := financeRecalculationPublicFailure(b.ErrorCode)
	unit := "account_cost"
	if b.Kind == service.FinanceRecalculationPaymentFee {
		unit = "CNY"
	}
	summary := []string{}
	if b.ConfigFingerprint != "" {
		if b.Kind == service.FinanceRecalculationPaymentFee {
			summary = append(summary, fmt.Sprintf("Current fee rates: %d payment providers", len(b.Config.Payment)))
		} else {
			summary = append(summary, fmt.Sprintf("Current cost rules: %d account/group/model scopes", len(b.Config.Usage)))
			var earliest *time.Time
			for _, scope := range b.Config.Usage {
				if scope.CoverageFrom != nil && (earliest == nil || scope.CoverageFrom.Before(*earliest)) {
					earliest = scope.CoverageFrom
				}
			}
			if earliest != nil {
				summary = append(summary, "Earliest certified aggregate-coverage start: "+earliest.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format(time.RFC3339)+"; coverage is checked separately for each scope")
			}
			if b.AcceptRetainedBaseline && b.Config.RetainedBaseline != nil {
				summary = append(summary, fmt.Sprintf("Accepted retained-detail baseline: %d full days reconciled against hourly/daily aggregates; original aggregate values and source digests retained in audit", len(b.Config.RetainedBaseline.Days)))
				summary = append(summary, "Deleted detail cannot be recovered; baseline is an explicit accounting choice, not proof of historical completeness. Reconciliation is repeated before apply; no undo")
			} else {
				summary = append(summary, "Legacy historical aggregate coverage is not certified; requests without proven coverage are skipped, not repaired")
			}
			if b.Skipped > 0 {
				summary = append(summary, fmt.Sprintf("%d requests were skipped; their existing costs are preserved", b.Skipped))
			}
		}
	}
	phase := ""
	switch b.FailedPhase {
	case service.FinanceRecalculationPreviewing, service.FinanceRecalculationApplying, service.FinanceRecalculationSyncing:
		phase = b.FailedPhase
	}
	return financeRecalculationBatchResponse{FinanceRecalculationFilter: b.FinanceRecalculationFilter, ID: b.ID, Status: b.Status, Scanned: b.Scanned, Changed: b.Changed, Unchanged: b.Unchanged, Skipped: b.Skipped, OldTotal: b.OldTotal, NewTotal: b.NewTotal, Delta: b.Delta, ActorID: b.ActorID, ConfigFingerprint: b.ConfigFingerprint, CutoffAt: b.CutoffAt, ExpiresAt: b.ExpiresAt, CreatedAt: b.CreatedAt, ErrorCode: code, ErrorMessage: message, AmountUnit: unit, SnapshotSummary: summary, FailedPhase: phase}
}
func financeRecalculationPublicLabel(value string) string {
	if len(value) > 120 {
		return "unknown"
	}
	for _, c := range value {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' && c != '.' {
			return "unknown"
		}
	}
	return value
}
func financeRecalculationItemToResponse(i service.FinanceRecalculationItem) financeRecalculationItemResponse {
	return financeRecalculationItemResponse{ID: i.ID, Status: i.Status, Reason: financeRecalculationPublicLabel(i.Reason), Source: financeRecalculationPublicLabel(i.Source), OldCost: i.OldCost, NewCost: i.NewCost, Delta: i.Delta, Applied: i.Applied}
}
func financeRecalculationAdmin(c *gin.Context) bool {
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok {
		response.Unauthorized(c, "Administrator authentication required")
		return false
	}
	if role != service.RoleAdmin {
		response.Forbidden(c, "Administrator access required")
		return false
	}
	return true
}
func financeRecalculationActor(c *gin.Context) (int64, bool) {
	if !financeRecalculationAdmin(c) {
		return 0, false
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authenticated administrator identity required")
		return 0, false
	}
	return subject.UserID, true
}
func financeRecalculationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrFinanceRecalculationInvalid)
		return 0, false
	}
	return id, true
}
func financeRecalculationRespond(c *gin.Context, b *service.FinanceRecalculationBatch, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, financeRecalculationBatchToResponse(b))
}
func (h *FinanceRecalculationHandler) Create(c *gin.Context) {
	actor, ok := financeRecalculationActor(c)
	if !ok {
		return
	}
	var req struct {
		service.FinanceRecalculationFilter
		IdempotencyKey string `json:"idempotency_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrFinanceRecalculationInvalid)
		return
	}
	b, err := h.s.Create(c.Request.Context(), service.FinanceRecalculationCreate{FinanceRecalculationFilter: req.FinanceRecalculationFilter, ActorID: actor, IdempotencyKey: req.IdempotencyKey})
	financeRecalculationRespond(c, b, err)
}
func (h *FinanceRecalculationHandler) List(c *gin.Context) {
	if !financeRecalculationAdmin(c) {
		return
	}
	page, size, ok := financePage(c)
	if !ok {
		return
	}
	batches, total, err := h.s.List(c.Request.Context(), page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]financeRecalculationBatchResponse, 0, len(batches))
	for i := range batches {
		out = append(out, financeRecalculationBatchToResponse(&batches[i]))
	}
	response.Paginated(c, out, total, page, size)
}
func (h *FinanceRecalculationHandler) Get(c *gin.Context) {
	if !financeRecalculationAdmin(c) {
		return
	}
	id, ok := financeRecalculationID(c)
	if !ok {
		return
	}
	b, err := h.s.Get(c.Request.Context(), id)
	financeRecalculationRespond(c, b, err)
}
func (h *FinanceRecalculationHandler) Items(c *gin.Context) {
	if !financeRecalculationAdmin(c) {
		return
	}
	id, ok := financeRecalculationID(c)
	if !ok {
		return
	}
	page, size, ok := financePage(c)
	if !ok {
		return
	}
	items, total, err := h.s.Items(c.Request.Context(), id, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]financeRecalculationItemResponse, 0, len(items))
	for _, i := range items {
		out = append(out, financeRecalculationItemToResponse(i))
	}
	response.Paginated(c, out, total, page, size)
}
func (h *FinanceRecalculationHandler) Confirm(c *gin.Context) {
	actor, ok := financeRecalculationActor(c)
	if !ok {
		return
	}
	id, ok := financeRecalculationID(c)
	if !ok {
		return
	}
	var req struct {
		Fingerprint  string `json:"config_fingerprint" binding:"required"`
		Acknowledged bool   `json:"acknowledged"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, service.ErrFinanceRecalculationInvalid)
		return
	}
	if !req.Acknowledged {
		response.ErrorFrom(c, infraerrors.BadRequest("FINANCE_RECALCULATION_ACK_REQUIRED", "Explicit acknowledgement that this operation cannot be undone is required"))
		return
	}
	b, err := h.s.Confirm(c.Request.Context(), id, actor, req.Fingerprint, req.Acknowledged)
	financeRecalculationRespond(c, b, err)
}
func (h *FinanceRecalculationHandler) RetrySync(c *gin.Context) {
	actor, ok := financeRecalculationActor(c)
	if !ok {
		return
	}
	id, ok := financeRecalculationID(c)
	if !ok {
		return
	}
	b, err := h.s.RetrySync(c.Request.Context(), id, actor)
	financeRecalculationRespond(c, b, err)
}
