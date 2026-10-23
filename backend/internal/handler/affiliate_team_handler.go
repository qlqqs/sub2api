package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AffiliateTeamHandler 提供团队消费页的只读接口：代理端 /api/v1/user/aff/team/*，
// 管理端 /api/v1/admin/affiliates/team/*（affiliate_team_admin_handler.go）。
type AffiliateTeamHandler struct {
	teamService *service.AffiliateTeamService
}

func NewAffiliateTeamHandler(teamService *service.AffiliateTeamService) *AffiliateTeamHandler {
	return &AffiliateTeamHandler{teamService: teamService}
}

// Overview GET /api/v1/user/aff/team/overview?start_date&end_date[&agent_id|&user_id]
func (h *AffiliateTeamHandler) Overview(c *gin.Context) {
	viewerID, rng, ok := h.parseCommon(c)
	if !ok {
		return
	}
	serveTeamOverview(c, viewerID, rng, h.teamService.Overview)
}

// ListCustomers GET /api/v1/user/aff/team/customers
func (h *AffiliateTeamHandler) ListCustomers(c *gin.Context) {
	viewerID, rng, ok := h.parseCommon(c)
	if !ok {
		return
	}
	serveTeamCustomers(c, viewerID, rng, h.teamService.ListCustomers)
}

// ListAgents GET /api/v1/user/aff/team/agents
func (h *AffiliateTeamHandler) ListAgents(c *gin.Context) {
	viewerID, rng, ok := h.parseCommon(c)
	if !ok {
		return
	}
	serveTeamAgents(c, viewerID, rng, h.teamService.ListAgents)
}

// ListRequests GET /api/v1/user/aff/team/customers/:user_id/requests
func (h *AffiliateTeamHandler) ListRequests(c *gin.Context) {
	viewerID, rng, ok := h.parseCommon(c)
	if !ok {
		return
	}
	serveTeamRequests(c, viewerID, rng, h.teamService.ListRequests)
}

func (h *AffiliateTeamHandler) parseCommon(c *gin.Context) (int64, service.AffiliateTeamRange, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, service.AffiliateTeamRange{}, false
	}
	rng, ok := parseTeamRange(c)
	if !ok {
		return 0, service.AffiliateTeamRange{}, false
	}
	return subject.UserID, rng, true
}

func parseTeamRange(c *gin.Context) (service.AffiliateTeamRange, bool) {
	rng, err := service.ParseAffiliateTeamRange(c.Query("start_date"), c.Query("end_date"), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return service.AffiliateTeamRange{}, false
	}
	return rng, true
}

// 代理端与管理端代理视角共用的参数解析与响应；查看者由调用方解析（登录用户或路径中的 viewer_id）。

type (
	teamOverviewFunc  func(ctx context.Context, viewerID, agentID, customerID int64, rng service.AffiliateTeamRange) (*service.AffiliateTeamOverview, error)
	teamCustomersFunc func(ctx context.Context, viewerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamCustomerEntry, int64, error)
	teamAgentsFunc    func(ctx context.Context, viewerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamAgentEntry, int64, error)
	teamRequestsFunc  func(ctx context.Context, viewerID, customerID int64, rng service.AffiliateTeamRange, query service.AffiliateTeamListQuery) ([]service.AffiliateTeamRequest, int64, error)
)

func serveTeamOverview(c *gin.Context, viewerID int64, rng service.AffiliateTeamRange, overview teamOverviewFunc) {
	agentID, ok := parseOptionalID(c, "agent_id")
	if !ok {
		return
	}
	customerID, ok := parseOptionalID(c, "user_id")
	if !ok {
		return
	}
	out, err := overview(c.Request.Context(), viewerID, agentID, customerID, rng)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func serveTeamCustomers(c *gin.Context, viewerID int64, rng service.AffiliateTeamRange, list teamCustomersFunc) {
	agentID, ok := parseOptionalID(c, "agent_id")
	if !ok {
		return
	}
	query := parseTeamListQuery(c)
	query.AgentID = agentID
	query.Search = c.Query("search")
	items, total, err := list(c.Request.Context(), viewerID, rng, query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, query.Page, query.PageSize)
}

func serveTeamAgents(c *gin.Context, viewerID int64, rng service.AffiliateTeamRange, list teamAgentsFunc) {
	query := parseTeamListQuery(c)
	items, total, err := list(c.Request.Context(), viewerID, rng, query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, query.Page, query.PageSize)
}

func serveTeamRequests(c *gin.Context, viewerID int64, rng service.AffiliateTeamRange, list teamRequestsFunc) {
	customerID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || customerID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	query := parseTeamListQuery(c)
	query.Model = c.Query("model")
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID < 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		query.GroupID = &groupID
	}
	items, total, err := list(c.Request.Context(), viewerID, customerID, rng, query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, query.Page, query.PageSize)
}

func parseOptionalID(c *gin.Context, key string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid "+key)
		return 0, false
	}
	return id, true
}

func parseTeamListQuery(c *gin.Context) service.AffiliateTeamListQuery {
	page, pageSize := service.NormalizeAffiliatePaging(response.ParsePagination(c))
	return service.AffiliateTeamListQuery{
		SortBy:   c.Query("sort_by"),
		SortDesc: c.Query("sort_order") != "asc",
		Page:     page,
		PageSize: pageSize,
	}
}
