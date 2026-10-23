package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// 管理端团队消费页。路由挂在管理员路由组下（认证、管理员校验与全局限流由路由组负责）；
// 放在 handler 包而不是 handler/admin，是为了复用代理端的参数解析（handler 包引用了 handler/admin，反向会循环引用）。

// AdminOverview GET /api/v1/admin/affiliates/team/overview?start_date&end_date
func (h *AffiliateTeamHandler) AdminOverview(c *gin.Context) {
	rng, ok := parseTeamRange(c)
	if !ok {
		return
	}
	out, err := h.teamService.AdminOverview(c.Request.Context(), rng)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

// AdminListOwners GET /api/v1/admin/affiliates/team/agents?start_date&end_date&level&search&sort_by&sort_order&page&page_size
func (h *AffiliateTeamHandler) AdminListOwners(c *gin.Context) {
	rng, ok := parseTeamRange(c)
	if !ok {
		return
	}
	query := parseTeamListQuery(c)
	query.Level = c.Query("level")
	query.Search = c.Query("search")
	items, total, err := h.teamService.AdminListOwners(c.Request.Context(), rng, query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, query.Page, query.PageSize)
}

// AdminViewerOverview GET /api/v1/admin/affiliates/team/:viewer_id/overview（参数同代理端 overview）
func (h *AffiliateTeamHandler) AdminViewerOverview(c *gin.Context) {
	viewerID, rng, ok := parseAdminTeamViewer(c)
	if !ok {
		return
	}
	serveTeamOverview(c, viewerID, rng, h.teamService.AdminViewerOverview)
}

// AdminViewerCustomers GET /api/v1/admin/affiliates/team/:viewer_id/customers
func (h *AffiliateTeamHandler) AdminViewerCustomers(c *gin.Context) {
	viewerID, rng, ok := parseAdminTeamViewer(c)
	if !ok {
		return
	}
	serveTeamCustomers(c, viewerID, rng, h.teamService.AdminViewerCustomers)
}

// AdminViewerAgents GET /api/v1/admin/affiliates/team/:viewer_id/agents
func (h *AffiliateTeamHandler) AdminViewerAgents(c *gin.Context) {
	viewerID, rng, ok := parseAdminTeamViewer(c)
	if !ok {
		return
	}
	serveTeamAgents(c, viewerID, rng, h.teamService.AdminViewerAgents)
}

// AdminViewerRequests GET /api/v1/admin/affiliates/team/:viewer_id/customers/:user_id/requests
func (h *AffiliateTeamHandler) AdminViewerRequests(c *gin.Context) {
	viewerID, rng, ok := parseAdminTeamViewer(c)
	if !ok {
		return
	}
	serveTeamRequests(c, viewerID, rng, h.teamService.AdminViewerRequests)
}

func parseAdminTeamViewer(c *gin.Context) (int64, service.AffiliateTeamRange, bool) {
	viewerID, err := strconv.ParseInt(c.Param("viewer_id"), 10, 64)
	if err != nil || viewerID <= 0 {
		response.BadRequest(c, "Invalid viewer_id")
		return 0, service.AffiliateTeamRange{}, false
	}
	rng, ok := parseTeamRange(c)
	if !ok {
		return 0, service.AffiliateTeamRange{}, false
	}
	return viewerID, rng, true
}
