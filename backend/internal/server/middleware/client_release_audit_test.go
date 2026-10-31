//go:build unit

package middleware

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientReleaseReadProbe struct {
	io.Reader
	reads int
}

func (r *clientReleaseReadProbe) Read(p []byte) (int, error) { r.reads++; return r.Reader.Read(p) }
func (r *clientReleaseReadProbe) Close() error               { return nil }
func TestClientReleaseAuditDoesNotReadUploadBeforeStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &auditCaptureRepository{}
	audit := service.NewAuditLogService(repo, nil)
	audit.Start()
	r := gin.New()
	r.Use(gin.HandlerFunc(NewAuditLogMiddleware(audit)))
	probe := &clientReleaseReadProbe{Reader: strings.NewReader("binary artifact")}
	r.PUT("/api/v1/admin/client-releases/:id/files/:name", func(c *gin.Context) { require.Zero(t, probe.reads); c.AbortWithStatus(403) })
	req := httptest.NewRequest("PUT", "/api/v1/admin/client-releases/1/files/qxapi-linux-amd64", nil)
	req.Body = probe
	req.ContentLength = 200 << 20
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.Zero(t, probe.reads)
	audit.Stop()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.logs, 1)
	require.Equal(t, "<binary body omitted>", repo.logs[0].RequestBody)
}
