package admin

// CUSTOM: Administrator release management, using the standard response envelope.
import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ClientReleaseHandler struct{ releases *service.ClientReleaseService }

func NewClientReleaseHandler(releases *service.ClientReleaseService) *ClientReleaseHandler {
	return &ClientReleaseHandler{releases: releases}
}

type clientReleaseFileDTO struct {
	Name string `json:"name"`
	service.ClientReleaseFileSpec
	Uploaded bool `json:"uploaded"`
}
type clientReleaseDTO struct {
	ID          int64                  `json:"id"`
	Product     string                 `json:"product"`
	Version     string                 `json:"version"`
	Notes       string                 `json:"notes"`
	Status      string                 `json:"status"`
	PublishedAt *time.Time             `json:"published_at"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Files       []clientReleaseFileDTO `json:"files"`
}

func toClientReleaseDTO(r *service.ClientRelease) (*clientReleaseDTO, error) {
	m, err := service.ParseClientReleaseManifest(r.Manifest)
	if err != nil {
		return nil, err
	}
	uploaded := make(map[string]bool, len(r.Files))
	for _, f := range r.Files {
		uploaded[f.Name] = true
	}
	dto := &clientReleaseDTO{ID: r.ID, Product: r.Product, Version: r.Version, Notes: r.Notes, Status: r.Status, PublishedAt: r.PublishedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Files: make([]clientReleaseFileDTO, 0, len(m.Files))}
	for name, f := range m.Files {
		dto.Files = append(dto.Files, clientReleaseFileDTO{Name: name, ClientReleaseFileSpec: f, Uploaded: uploaded[name]})
	}
	sort.Slice(dto.Files, func(i, j int) bool { return dto.Files[i].Name < dto.Files[j].Name })
	return dto, nil
}
func clientReleaseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrClientReleaseNotFound)
		return 0, false
	}
	return id, true
}
func writeClientRelease(c *gin.Context, r *service.ClientRelease, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	dto, err := toClientReleaseDTO(r)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto)
}
func (h *ClientReleaseHandler) List(c *gin.Context) {
	releases, err := h.releases.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]*clientReleaseDTO, 0, len(releases))
	for _, r := range releases {
		dto, err := toClientReleaseDTO(r)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		result = append(result, dto)
	}
	response.Success(c, result)
}
func (h *ClientReleaseHandler) Create(c *gin.Context) {
	raw, err := c.GetRawData()
	if err != nil {
		response.ErrorFrom(c, service.ErrClientReleaseInvalidManifest)
		return
	}
	r, err := h.releases.Create(c.Request.Context(), raw)
	writeClientRelease(c, r, err)
}
func (h *ClientReleaseHandler) CheckUpload(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	if _, err := h.releases.CheckUpload(c.Request.Context(), id, c.Param("name")); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// A typed nil preserves the contract's data:null through response.Success.
	response.Success(c, (*struct{})(nil))
}
func (h *ClientReleaseHandler) Upload(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	f, err := h.releases.CheckUpload(c.Request.Context(), id, c.Param("name"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, f.Size)
	r, err := h.releases.Upload(c.Request.Context(), id, c.Param("name"), c.Request.Body)
	writeClientRelease(c, r, err)
}
func (h *ClientReleaseHandler) UpdateNotes(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	var req struct {
		Notes *string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Notes == nil {
		response.BadRequest(c, "Notes are required")
		return
	}
	r, err := h.releases.UpdateNotes(c.Request.Context(), id, *req.Notes)
	writeClientRelease(c, r, err)
}
func (h *ClientReleaseHandler) Publish(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	r, removed, err := h.releases.Publish(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	dto, err := toClientReleaseDTO(r)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"release": dto, "removed_versions": removed})
}
func (h *ClientReleaseHandler) Withdraw(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	r, err := h.releases.Withdraw(c.Request.Context(), id)
	writeClientRelease(c, r, err)
}
func (h *ClientReleaseHandler) Delete(c *gin.Context) {
	id, ok := clientReleaseID(c)
	if !ok {
		return
	}
	if err := h.releases.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// A typed nil preserves the contract's data:null through response.Success.
	response.Success(c, (*struct{})(nil))
}
