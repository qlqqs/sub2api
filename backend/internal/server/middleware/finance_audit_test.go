package middleware

// CUSTOM(fork): 支出增改删由既有管理面审计链记录操作者、输入、结果。
import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFinanceExpenseAudit(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			repo := &auditCaptureRepository{}
			svc := service.NewAuditLogService(repo, nil)
			svc.Start()
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyUser), AuthSubject{UserID: 77})
				c.Set(string(ContextKeyUserRole), "admin")
				c.Next()
			})
			r.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
			path := "/api/v1/admin/finance/expenses"
			route := path
			if method != "POST" {
				path += "/11"
				route += "/:id"
			}
			r.Handle(method, route, func(c *gin.Context) { c.Status(200) })
			req := httptest.NewRequest(method, path, bytes.NewBufferString(`{"amount":"100.00","category":"server"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			svc.Stop()
			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Len(t, repo.logs, 1)
			entry := repo.logs[0]
			require.Equal(t, int64(77), *entry.ActorUserID)
			require.Equal(t, deriveAuditAction(method, route), entry.Action)
			require.Equal(t, 200, entry.StatusCode)
			require.Contains(t, entry.RequestBody, "100.00")
		})
	}
}
