package admin

// CUSTOM(fork): 仪表盘公告轮播的名单与顺序。
// Get 同时挂在管理端（/admin/settings/announcement-banner）和用户端（/announcement-banner）：
// 返回的只是公告 ID 名单，用户端据此筛选、排序自己已能看到的公告。

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AnnouncementBannerHandler 公告轮播配置接口。
type AnnouncementBannerHandler struct {
	banner *service.AnnouncementBannerService
}

// NewAnnouncementBannerHandler 创建公告轮播配置 handler。
func NewAnnouncementBannerHandler(banner *service.AnnouncementBannerService) *AnnouncementBannerHandler {
	return &AnnouncementBannerHandler{banner: banner}
}

type updateAnnouncementBannerRequest struct {
	Order  []int64 `json:"order"`
	Hidden []int64 `json:"hidden"`
}

// Get 返回轮播配置。
// GET /api/v1/admin/settings/announcement-banner
// GET /api/v1/announcement-banner
func (h *AnnouncementBannerHandler) Get(c *gin.Context) {
	cfg, err := h.banner.Get(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// Update 保存轮播配置。
// PUT /api/v1/admin/settings/announcement-banner
func (h *AnnouncementBannerHandler) Update(c *gin.Context) {
	var req updateAnnouncementBannerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.banner.Update(c.Request.Context(), service.AnnouncementBannerConfig{Order: req.Order, Hidden: req.Hidden})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}
