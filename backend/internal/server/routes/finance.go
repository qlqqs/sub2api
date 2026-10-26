package routes

// CUSTOM(fork): 财务明细只提供管理员访问，写入由外层审计中间件记录。
import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func registerFinanceRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	if h.Admin.Finance == nil {
		return
	}
	f := admin.Group("/finance")
	v := h.Admin.Finance
	f.GET("/summary", v.Summary)
	f.GET("/recharge", v.Recharge)
	f.GET("/breakdown", v.Breakdown)
	f.GET("/affiliate", v.Affiliate)
	f.GET("/reconcile", v.Reconcile)
	f.GET("/export", v.Export)
	f.GET("/settings", v.Settings)
	f.PUT("/settings", v.SaveSettings)
	f.GET("/expenses", v.Expenses)
	f.POST("/expenses", v.SaveExpense)
	f.PUT("/expenses/:id", v.SaveExpense)
	f.DELETE("/expenses/:id", v.DeleteExpense)
}
