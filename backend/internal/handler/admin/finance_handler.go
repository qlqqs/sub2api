package admin

// CUSTOM(fork): 财务管理员接口；沿用管理面认证、合规与审计中间件。
import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type FinanceHandler struct{ s *service.FinanceService }

func NewFinanceHandler(s *service.FinanceService) *FinanceHandler { return &FinanceHandler{s: s} }
func financeRange(c *gin.Context) (service.FinanceRange, bool) {
	r, err := service.ParseFinanceRange(c.Query("start"), c.Query("end"), c.Query("granularity"), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return r, false
	}
	return r, true
}
func financeResult(c *gin.Context, data any, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, data)
}
func (h *FinanceHandler) Summary(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	data, err := h.s.Summary(c.Request.Context(), r)
	financeResult(c, data, err)
}
func (h *FinanceHandler) Recharge(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	data, err := h.s.Recharge(c.Request.Context(), r)
	financeResult(c, data, err)
}
func (h *FinanceHandler) Affiliate(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	data, err := h.s.Affiliate(c.Request.Context(), r)
	financeResult(c, data, err)
}
func (h *FinanceHandler) Breakdown(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "200"))
	if err != nil {
		response.BadRequest(c, "条数无效")
		return
	}
	data, err := h.s.Breakdown(c.Request.Context(), r, c.DefaultQuery("dim", "account"), limit)
	financeResult(c, data, err)
}
func financePage(c *gin.Context) (int, int, bool) {
	page, e1 := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, e2 := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if e1 != nil || e2 != nil || page < 1 || page > 10000000 || size < 1 || size > 200 {
		response.BadRequest(c, "分页参数无效")
		return 0, 0, false
	}
	return page, size, true
}
func (h *FinanceHandler) Reconcile(c *gin.Context) {
	page, size, ok := financePage(c)
	if !ok {
		return
	}
	only, err := strconv.ParseBool(c.DefaultQuery("only_mismatch", "false"))
	if err != nil {
		response.BadRequest(c, "only_mismatch 必须为布尔值")
		return
	}
	items, total, err := h.s.Reconcile(c.Request.Context(), page, size, only, c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
func (h *FinanceHandler) Settings(c *gin.Context) {
	data, err := h.s.Settings(c.Request.Context())
	financeResult(c, data, err)
}
func (h *FinanceHandler) SaveSettings(c *gin.Context) {
	var req struct {
		IDs *[]int64 `json:"fixed_cost_account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.IDs == nil {
		response.BadRequest(c, "必须提供固定成本账号数组")
		return
	}
	data, err := h.s.SaveSettings(c.Request.Context(), *req.IDs)
	financeResult(c, data, err)
}
func (h *FinanceHandler) Expenses(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	page, size, ok := financePage(c)
	if !ok {
		return
	}
	items, total, err := h.s.ListExpenses(c.Request.Context(), r, c.Query("category"), page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
func financeID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "支出 ID 无效")
		return 0, false
	}
	return id, true
}
func (h *FinanceHandler) SaveExpense(c *gin.Context) {
	var id int64
	if c.Request.Method == "PUT" {
		var ok bool
		id, ok = financeID(c)
		if !ok {
			return
		}
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "请先登录")
		return
	}
	var in service.FinanceExpenseInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "支出格式无效")
		return
	}
	data, err := h.s.SaveExpense(c.Request.Context(), id, subject.UserID, in)
	financeResult(c, data, err)
}
func (h *FinanceHandler) DeleteExpense(c *gin.Context) {
	id, ok := financeID(c)
	if !ok {
		return
	}
	financeResult(c, map[string]bool{"deleted": true}, h.s.DeleteExpense(c.Request.Context(), id))
}
func (h *FinanceHandler) Export(c *gin.Context) {
	r, ok := financeRange(c)
	if !ok {
		return
	}
	data, err := h.s.Summary(c.Request.Context(), r)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 列顺序固定；内容仅为后端产生的日期和数字，不含可执行的用户公式。
	keys := []string{"period", "revenue", "revenue_paid", "revenue_commission", "free_consumed", "overdraft_consumed", "consumption", "account_cost", "account_cost_internal", "commission", "expenses", "profit", "profit_margin"}
	header := []string{"期间", "已确认收入", "付费收入", "分润转入收入", "赠送消耗", "透支消耗", "余额消耗", "账号成本", "内部测试成本", "分润支出", "其他支出", "利润", "利润率"}
	var buf bytes.Buffer
	if _, err = buf.WriteString("\xef\xbb\xbf"); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	w := csv.NewWriter(&buf)
	if err = w.Write(header); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	series, ok := data["series"].([]map[string]any)
	if !ok {
		response.ErrorFrom(c, fmt.Errorf("invalid finance summary series"))
		return
	}
	for _, row := range series {
		record := make([]string, len(keys))
		for i, k := range keys {
			if row[k] != nil {
				record[i] = fmt.Sprint(row[k])
			}
		}
		if err = w.Write(record); err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="finance-%s-%s.csv"`, r.Start, r.End))
	c.Data(200, "text/csv; charset=utf-8", buf.Bytes())
}
