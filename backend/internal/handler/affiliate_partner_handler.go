package handler

// CUSTOM(fork): 合伙人管理、月结及区域团队接口。
import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type AffiliatePartnerHandler struct {
	s    *service.PartnerService
	team *service.AffiliateTeamService
}

func NewAffiliatePartnerHandler(s *service.PartnerService, team *service.AffiliateTeamService) *AffiliatePartnerHandler {
	return &AffiliatePartnerHandler{s: s, team: team}
}
func partnerResult(c *gin.Context, v any, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, v)
}
func partnerID(c *gin.Context, key string) (int64, bool) {
	raw := c.Param(key)
	if raw == "" {
		raw = c.Query(key)
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_INVALID", "用户或记录 ID 无效"))
		return 0, false
	}
	return id, true
}
func partnerActor(c *gin.Context) (int64, bool) {
	a, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return a.UserID, true
}
func (h *AffiliatePartnerHandler) publicID(c *gin.Context) (int64, bool) {
	id, ok := partnerActor(c)
	if !ok {
		return 0, false
	}
	if _, err := h.s.RequirePartner(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return 0, false
	}
	return id, true
}
func (h *AffiliatePartnerHandler) List(c *gin.Context) {
	v, err := h.s.List(c.Request.Context())
	partnerResult(c, gin.H{"items": v}, err)
}
func (h *AffiliatePartnerHandler) Preview(c *gin.Context) {
	id, ok := partnerID(c, "user_id")
	if !ok {
		return
	}
	v, err := h.s.PreviewCreate(c.Request.Context(), id)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Create(c *gin.Context) {
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	var in struct {
		UserID     int64            `json:"user_id"`
		RegionName string           `json:"region_name"`
		ShareRate  *decimal.Decimal `json:"share_rate"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.UserID <= 0 || in.ShareRate == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_INVALID", "请提供用户、区域名称和分成比例"))
		return
	}
	v, err := h.s.Create(c.Request.Context(), in.UserID, in.RegionName, *in.ShareRate, actor)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Detail(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	v, err := h.s.Detail(c.Request.Context(), id)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Update(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	var in struct {
		RegionName *string          `json:"region_name"`
		ShareRate  *decimal.Decimal `json:"share_rate"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_INVALID", "区域名称或分成比例格式无效"))
		return
	}
	v, err := h.s.Update(c.Request.Context(), id, in.RegionName, in.ShareRate, actor)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) PreviewAttach(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	agent, ok := partnerID(c, "agent_user_id")
	if !ok {
		return
	}
	v, err := h.s.PreviewAttach(c.Request.Context(), id, agent)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Attach(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	var in struct {
		AgentUserID int64 `json:"agent_user_id"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.AgentUserID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_INVALID", "渠道经理 ID 无效"))
		return
	}
	v, err := h.s.Attach(c.Request.Context(), id, in.AgentUserID, actor)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Statements(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	v, err := h.s.Statements(c.Request.Context(), id)
	partnerResult(c, gin.H{"items": v}, err)
}
func (h *AffiliatePartnerHandler) Statement(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	v, err := h.s.Statement(c.Request.Context(), id, c.Param("month"))
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Confirm(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	v, err := h.s.Confirm(c.Request.Context(), id, c.Param("month"), actor)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) Pay(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	var in struct {
		PaidOn string `json:"paid_on"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_INVALID", "付款信息格式无效"))
		return
	}
	v, err := h.s.Pay(c.Request.Context(), id, c.Param("month"), actor, in.PaidOn, in.Note)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) AddAdjustment(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	actor, ok := partnerActor(c)
	if !ok {
		return
	}
	var in struct {
		Month  string           `json:"month"`
		Amount *decimal.Decimal `json:"amount"`
		Note   string           `json:"note"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Amount == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PARTNER_ADJUSTMENT_INVALID", "调整项格式无效"))
		return
	}
	v, err := h.s.AddAdjustment(c.Request.Context(), id, in.Month, *in.Amount, in.Note, actor)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) DeleteAdjustment(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	a, ok := partnerID(c, "id")
	if !ok {
		return
	}
	partnerResult(c, gin.H{"deleted": true}, h.s.DeleteAdjustment(c.Request.Context(), id, a))
}
func (h *AffiliatePartnerHandler) Overview(c *gin.Context) {
	id, ok := h.publicID(c)
	if !ok {
		return
	}
	v, err := h.s.Overview(c.Request.Context(), id)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) UserStatements(c *gin.Context) {
	id, ok := h.publicID(c)
	if !ok {
		return
	}
	v, err := h.s.Statements(c.Request.Context(), id)
	partnerResult(c, gin.H{"items": v}, err)
}
func (h *AffiliatePartnerHandler) UserStatement(c *gin.Context) {
	id, ok := h.publicID(c)
	if !ok {
		return
	}
	v, err := h.s.Statement(c.Request.Context(), id, c.Param("month"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	partnerResult(c, service.PartnerPublicStatement(*v), nil)
}

func partnerTeamRange(c *gin.Context) (service.AffiliateTeamRange, bool) {
	start, end := c.Query("start_date"), c.Query("end_date")
	if start == "" {
		start = c.Query("start")
	}
	if end == "" {
		end = c.Query("end")
	}
	if start == "" && end == "" && c.Query("days") != "" {
		days, err := strconv.Atoi(c.Query("days"))
		if err != nil || days < 1 || days > 90 {
			response.ErrorFrom(c, service.ErrAffiliateTeamInvalidRange)
			return service.AffiliateTeamRange{}, false
		}
		now := time.Now().In(service.AffiliateTeamLocation)
		start = now.AddDate(0, 0, 1-days).Format("2006-01-02")
		end = now.Format("2006-01-02")
	}
	rng, err := service.ParseAffiliateTeamRange(start, end, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return rng, false
	}
	return rng, true
}
func (h *AffiliatePartnerHandler) teamCommon(c *gin.Context) (int64, service.AffiliateTeamRange, bool) {
	id, ok := h.publicID(c)
	if !ok {
		return 0, service.AffiliateTeamRange{}, false
	}
	rng, ok := partnerTeamRange(c)
	return id, rng, ok
}
func (h *AffiliatePartnerHandler) TeamOverview(c *gin.Context) {
	id, rng, ok := h.teamCommon(c)
	if !ok {
		return
	}
	// CUSTOM(fork): 区域客户页沿用代理端的 user_id 参数。
	customerID, ok := parseOptionalID(c, "user_id")
	if !ok {
		return
	}
	v, err := h.s.TeamOverview(c.Request.Context(), id, customerID, rng)
	partnerResult(c, v, err)
}
func (h *AffiliatePartnerHandler) TeamCustomers(c *gin.Context) {
	id, rng, ok := h.teamCommon(c)
	if !ok {
		return
	}
	q := parseTeamListQuery(c)
	q.Search = c.Query("search")
	v, n, err := h.s.TeamCustomers(c.Request.Context(), id, rng, q)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, v, n, q.Page, q.PageSize)
}
func (h *AffiliatePartnerHandler) TeamLines(c *gin.Context) {
	id, rng, ok := h.teamCommon(c)
	if !ok {
		return
	}
	v, err := h.s.TeamLines(c.Request.Context(), id, rng)
	partnerResult(c, gin.H{"items": v}, err)
}
func (h *AffiliatePartnerHandler) TeamRequests(c *gin.Context) {
	id, rng, ok := h.teamCommon(c)
	if !ok {
		return
	}
	serveTeamRequests(c, id, rng, h.s.TeamRequests)
}
func (h *AffiliatePartnerHandler) agentCommon(c *gin.Context) (int64, service.AffiliateTeamRange, bool) {
	id, rng, ok := h.teamCommon(c)
	if !ok {
		return 0, rng, false
	}
	agent, ok := partnerID(c, "agent_id")
	if !ok {
		return 0, rng, false
	}
	if err := h.s.RequireAttachedAgent(c.Request.Context(), id, agent); err != nil {
		response.ErrorFrom(c, err)
		return 0, rng, false
	}
	return agent, rng, true
}
func (h *AffiliatePartnerHandler) AgentOverview(c *gin.Context) {
	id, rng, ok := h.agentCommon(c)
	if ok {
		serveTeamOverview(c, id, rng, h.team.Overview)
	}
}
func (h *AffiliatePartnerHandler) AgentCustomers(c *gin.Context) {
	id, rng, ok := h.agentCommon(c)
	if ok {
		serveTeamCustomers(c, id, rng, h.team.ListCustomers)
	}
}
func (h *AffiliatePartnerHandler) AgentAgents(c *gin.Context) {
	id, rng, ok := h.agentCommon(c)
	if ok {
		serveTeamAgents(c, id, rng, h.team.ListAgents)
	}
}
func (h *AffiliatePartnerHandler) AgentRequests(c *gin.Context) {
	id, rng, ok := h.agentCommon(c)
	if ok {
		serveTeamRequests(c, id, rng, h.team.ListRequests)
	}
}

func partnerCSVText(s string) string {
	first := strings.TrimSpace(s)
	if first != "" && strings.ContainsAny(first[:1], "=+-@") {
		return "'" + s
	}
	return s
}
func partnerStatementCSV(d service.PartnerStatementDetail) ([]byte, error) {
	b := bytes.NewBuffer([]byte{239, 187, 191})
	w := csv.NewWriter(b)
	t := d.Totals
	records := [][]string{
		{"月份", "状态", "比例", "余额消耗", "收入", "账号成本", "分润", "专项支出", "调整", "利润", "期初亏损", "期末亏损", "分成基数", "分成金额"},
		{t.Month, t.Status, t.ShareRate, t.Consumption, t.Revenue, t.AccountCost, t.Commission, t.RegionExpense, t.Adjustment, t.Profit, t.LossBefore, t.LossAfter, t.ShareBase, t.ShareAmount}, {},
		{"团队线", "成员", "余额消耗", "收入", "账号成本", "分润", "利润"},
	}
	for _, l := range d.Lines {
		name := "合伙人直属团队"
		if l.LineAgent != nil {
			name = fmt.Sprintf("%d %s %s", l.LineAgent.ID, l.LineAgent.Username, l.LineAgent.Email)
		}
		records = append(records, []string{partnerCSVText(name), strconv.Itoa(l.Members), l.Consumption, l.Revenue, l.AccountCost, l.Commission, l.Profit})
	}
	records = append(records, []string{}, []string{"分组 ID", "分组", "余额消耗", "账号成本"})
	for _, g := range d.CostByGroup {
		records = append(records, []string{strconv.FormatInt(g.GroupID, 10), partnerCSVText(g.GroupName), g.Consumption, g.AccountCost})
	}
	records = append(records, []string{}, []string{"模型", "余额消耗", "账号成本"})
	for _, m := range d.CostByModel {
		records = append(records, []string{partnerCSVText(m.Model), m.Consumption, m.AccountCost})
	}
	records = append(records, []string{}, []string{"调整 ID", "金额", "说明", "创建时间", "操作人"})
	for _, a := range d.Adjustments {
		records = append(records, []string{strconv.FormatInt(a.ID, 10), a.Amount, partnerCSVText(a.Note), a.CreatedAt.Format(time.RFC3339), partnerCSVText(a.CreatedByEmail)})
	}
	if err := w.WriteAll(records); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (h *AffiliatePartnerHandler) Export(c *gin.Context) {
	id, ok := partnerID(c, "partner_id")
	if !ok {
		return
	}
	month := c.Param("month")
	d, err := h.s.Statement(c.Request.Context(), id, month)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	raw, err := partnerStatementCSV(*d)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="partner-%d-%s.csv"`, id, month))
	c.Data(200, "text/csv; charset=utf-8", raw)
}
