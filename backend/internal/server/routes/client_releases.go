package routes

// CUSTOM: Anonymous QxAPI updater endpoints.
import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterClientReleaseRoutes(v1 *gin.RouterGroup, h *handler.Handlers, limiter *middleware.PanelRateLimiter) {
	releases := v1.Group("/client-releases/qxapi", limiter.PublicIP())
	releases.GET("/latest", h.ClientRelease.Latest)
	releases.GET("/notes", h.ClientRelease.Notes)
	releases.GET("/files/:version/:name", h.ClientRelease.File)
	releases.HEAD("/files/:version/:name", h.ClientRelease.File)
}
