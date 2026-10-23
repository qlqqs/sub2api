package admin

// CUSTOM(fork): 美元汇率管理接口（design §15.4）。

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// FxRateHandler 美元汇率管理端接口。
type FxRateHandler struct {
	fx *service.FxRateService
}

// NewFxRateHandler 创建美元汇率 handler。
func NewFxRateHandler(fx *service.FxRateService) *FxRateHandler {
	return &FxRateHandler{fx: fx}
}

// fxRateViewDTO 汇率状态（API 合约见 design §15.4）。
type fxRateViewDTO struct {
	AutoEnabled   bool     `json:"auto_enabled"`
	DefaultRate   float64  `json:"default_rate"`
	EffectiveRate float64  `json:"effective_rate"`
	EffectiveFrom string   `json:"effective_from"`
	FetchedRate   *float64 `json:"fetched_rate"`
	FetchedAt     *string  `json:"fetched_at"`
	Source        string   `json:"source"`
	Stale         bool     `json:"stale"`
	LastError     string   `json:"last_error"`
	LastAttemptAt *string  `json:"last_attempt_at"`
}

type updateFxRateRequest struct {
	AutoEnabled *bool    `json:"auto_enabled"`
	DefaultRate *float64 `json:"default_rate"`
}

type refreshFxRateResponse struct {
	OK    bool          `json:"ok"`
	Error string        `json:"error"`
	Rate  fxRateViewDTO `json:"rate"`
}

func formatFxRateTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func toFxRateViewDTO(view *service.FxRateView) fxRateViewDTO {
	if view == nil {
		return fxRateViewDTO{}
	}
	return fxRateViewDTO{
		AutoEnabled:   view.AutoEnabled,
		DefaultRate:   view.DefaultRate,
		EffectiveRate: view.EffectiveRate,
		EffectiveFrom: view.EffectiveFrom,
		FetchedRate:   view.FetchedRate,
		FetchedAt:     formatFxRateTime(view.FetchedAt),
		Source:        view.Source,
		Stale:         view.Stale,
		LastError:     view.LastError,
		LastAttemptAt: formatFxRateTime(view.LastAttemptAt),
	}
}

// Get 返回汇率当前状态。
// GET /api/v1/admin/settings/fx-rate
func (h *FxRateHandler) Get(c *gin.Context) {
	view, err := h.fx.Get(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFxRateViewDTO(view))
}

// Update 保存自动更新开关与默认汇率（默认汇率须在 5–9）。
// PUT /api/v1/admin/settings/fx-rate
func (h *FxRateHandler) Update(c *gin.Context) {
	var req updateFxRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.AutoEnabled == nil || req.DefaultRate == nil {
		response.BadRequest(c, "auto_enabled and default_rate are required")
		return
	}
	view, err := h.fx.Update(c.Request.Context(), *req.AutoEnabled, *req.DefaultRate)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toFxRateViewDTO(view))
}

// Refresh 立即从来源获取一次；全部来源失败时 ok=false，HTTP 仍为 200。
// POST /api/v1/admin/settings/fx-rate/refresh
func (h *FxRateHandler) Refresh(c *gin.Context) {
	ok, errMsg, view, err := h.fx.RefreshNow(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, refreshFxRateResponse{OK: ok, Error: errMsg, Rate: toFxRateViewDTO(view)})
}
