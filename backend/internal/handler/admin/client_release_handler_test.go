//go:build unit

package admin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminClientReleaseRepo struct {
	service.ClientReleaseRepository
	release *service.ClientRelease
}

func (r *adminClientReleaseRepo) GetByID(_ context.Context, id int64) (*service.ClientRelease, error) {
	if r.release == nil || r.release.ID != id {
		return nil, service.ErrClientReleaseNotFound
	}
	copy := *r.release
	return &copy, nil
}
func (r *adminClientReleaseRepo) List(context.Context) ([]*service.ClientRelease, error) {
	if r.release == nil {
		return []*service.ClientRelease{}, nil
	}
	return []*service.ClientRelease{r.release}, nil
}
func (r *adminClientReleaseRepo) Create(ctx context.Context, v string, m json.RawMessage) (*service.ClientRelease, error) {
	if r.release != nil {
		return nil, service.ErrClientReleaseVersionExists
	}
	now := time.Now().UTC()
	r.release = &service.ClientRelease{ID: 1, Product: "qxapi", Version: v, Manifest: m, Status: service.ClientReleaseDraft, CreatedAt: now, UpdatedAt: now}
	return r.GetByID(ctx, 1)
}
func (r *adminClientReleaseRepo) SaveFile(_ context.Context, _ int64, f service.ClientReleaseFile) error {
	r.release.Files = []service.ClientReleaseFile{f}
	return nil
}
func (r *adminClientReleaseRepo) UpdateNotes(_ context.Context, id int64, notes string) error {
	if r.release == nil || id != r.release.ID {
		return service.ErrClientReleaseNotFound
	}
	r.release.Notes = notes
	return nil
}
func (r *adminClientReleaseRepo) Publish(_ context.Context, _ int64, _ []int64) error {
	r.release.Status = service.ClientReleasePublished
	now := time.Now().UTC()
	r.release.PublishedAt = &now
	return nil
}
func (r *adminClientReleaseRepo) Withdraw(_ context.Context, _ int64) error {
	r.release.Status = service.ClientReleaseWithdrawn
	return nil
}
func (r *adminClientReleaseRepo) Delete(_ context.Context, _ int64) error {
	r.release = nil
	return nil
}
func TestClientReleaseAdminContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("DATA_DIR", root)
	repo := &adminClientReleaseRepo{}
	h := NewClientReleaseHandler(service.NewClientReleaseService(repo, nil))
	r := gin.New()
	r.GET("/releases", h.List)
	r.POST("/releases", h.Create)
	r.POST("/releases/:id/files/:name/check", h.CheckUpload)
	r.PUT("/releases/:id/files/:name", h.Upload)
	r.PUT("/releases/:id/notes", h.UpdateNotes)
	r.POST("/releases/:id/publish", h.Publish)
	r.POST("/releases/:id/withdraw", h.Withdraw)
	r.DELETE("/releases/:id", h.Delete)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if method == "PUT" && strings.Contains(path, "/files/") {
			req.Header.Set("Content-Type", "application/octet-stream")
		}
		r.ServeHTTP(w, req)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var m map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
		return m
	}
	checkErr := func(w *httptest.ResponseRecorder, status int, reason string) {
		t.Helper()
		require.Equal(t, status, w.Code, w.Body.String())
		require.Equal(t, reason, decode(w)["reason"])
	}
	require.JSONEq(t, `{"code":0,"message":"success","data":[]}`, request("GET", "/releases", "").Body.String())
	checkErr(request("POST", "/releases", "{"), 400, "CLIENT_RELEASE_INVALID_MANIFEST")
	manifest := fmt.Sprintf(`{"version":"v1.0.0","files":{"qxapi-windows-amd64.exe":{"size":5,"sha256":"%x"},"qxapi-linux-amd64":{"size":5,"sha256":"%x"}}}`, sha256.Sum256([]byte("hello")), sha256.Sum256([]byte("hello")))
	checkErr(request("POST", "/releases", manifest+"{}"), 400, "CLIENT_RELEASE_INVALID_MANIFEST")
	w := request("POST", "/releases", manifest)
	require.Equal(t, 200, w.Code)
	dto := decode(w)["data"].(map[string]any)
	require.Len(t, dto, 9)
	require.Equal(t, "1.0.0", dto["version"])
	require.Equal(t, "qxapi", dto["product"])
	require.Nil(t, dto["published_at"])
	require.Equal(t, "draft", dto["status"])
	files := dto["files"].([]any)
	require.Len(t, files, 2)
	require.Equal(t, "qxapi-linux-amd64", files[0].(map[string]any)["name"])
	require.Equal(t, false, files[0].(map[string]any)["uploaded"])
	checkErr(request("POST", "/releases", manifest), 409, "CLIENT_RELEASE_VERSION_EXISTS")
	for _, tc := range []struct {
		name, path, status string
		want               int
		reason             string
	}{
		{"draft", "/releases/1/files/qxapi-windows-amd64.exe/check", service.ClientReleaseDraft, 200, ""},
		{"withdrawn", "/releases/1/files/qxapi-windows-amd64.exe/check", service.ClientReleaseWithdrawn, 200, ""},
		{"published", "/releases/1/files/qxapi-windows-amd64.exe/check", service.ClientReleasePublished, 409, "CLIENT_RELEASE_STATUS_CONFLICT"},
		{"missing", "/releases/99/files/qxapi-windows-amd64.exe/check", service.ClientReleaseDraft, 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"invalid id", "/releases/no/files/qxapi-windows-amd64.exe/check", service.ClientReleaseDraft, 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"invalid file", "/releases/1/files/bad.exe/check", service.ClientReleaseDraft, 400, "CLIENT_RELEASE_INVALID_FILE_NAME"},
		{"undeclared", "/releases/1/files/qxapi-linux-arm64/check", service.ClientReleaseDraft, 400, "CLIENT_RELEASE_FILE_NOT_IN_MANIFEST"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo.release.Status = tc.status
			w := request("POST", tc.path, "")
			if tc.want != 200 {
				checkErr(w, tc.want, tc.reason)
			} else {
				require.Equal(t, 200, w.Code)
				require.JSONEq(t, `{"code":0,"message":"success","data":null}`, w.Body.String())
			}
		})
	}
	repo.release.Status = service.ClientReleaseDraft
	checkErr(request("POST", "/releases/1/publish", ""), 400, "CLIENT_RELEASE_FILES_MISSING")
	for _, content := range []string{"hell", "hello!", "HELLO"} {
		checkErr(request("PUT", "/releases/1/files/qxapi-windows-amd64.exe", content), 400, "CLIENT_RELEASE_FILE_MISMATCH")
	}
	dir := filepath.Join(root, "client-releases", "qxapi", "1.0.0")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
	w = request("PUT", "/releases/1/files/qxapi-windows-amd64.exe", "hello")
	require.Equal(t, 200, w.Code)
	files = decode(w)["data"].(map[string]any)["files"].([]any)
	require.Equal(t, true, files[1].(map[string]any)["uploaded"])
	w = request("PUT", "/releases/1/notes", `{"notes":"中文说明"}`)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "中文说明", decode(w)["data"].(map[string]any)["notes"])
	// Keep the lifecycle fixture to one declared file from here.
	repo.release.Manifest = bytes.ReplaceAll(repo.release.Manifest, []byte(fmt.Sprintf(`,"qxapi-linux-amd64":{"size":5,"sha256":"%x"}`, sha256.Sum256([]byte("hello")))), nil)
	w = request("POST", "/releases/1/publish", "")
	require.Equal(t, 200, w.Code)
	published := decode(w)["data"].(map[string]any)
	require.Equal(t, []any{}, published["removed_versions"])
	require.Equal(t, "published", published["release"].(map[string]any)["status"])
	checkErr(request("POST", "/releases/1/publish", ""), 409, "CLIENT_RELEASE_STATUS_CONFLICT")
	checkErr(request("PUT", "/releases/1/files/qxapi-windows-amd64.exe", "hello"), 409, "CLIENT_RELEASE_STATUS_CONFLICT")
	checkErr(request("DELETE", "/releases/1", ""), 409, "CLIENT_RELEASE_STATUS_CONFLICT")
	w = request("POST", "/releases/1/withdraw", "")
	require.Equal(t, 200, w.Code)
	require.Equal(t, "withdrawn", decode(w)["data"].(map[string]any)["status"])
	w = request("DELETE", "/releases/1", "")
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":null}`, w.Body.String())
	require.NoDirExists(t, dir)
}

type clientReleaseRepeatingReader struct{}

func (clientReleaseRepeatingReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestClientReleaseUpload200MiBStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("DATA_DIR", root)
	const size int64 = 200 << 20
	hash := sha256.New()
	_, err := io.Copy(hash, io.LimitReader(clientReleaseRepeatingReader{}, size))
	require.NoError(t, err)
	manifest := fmt.Sprintf(`{"version":"1.0.0","files":{"qxapi-linux-amd64":{"size":%d,"sha256":"%x"}}}`, size, hash.Sum(nil))
	repo := &adminClientReleaseRepo{release: &service.ClientRelease{ID: 1, Product: "qxapi", Version: "1.0.0", Status: service.ClientReleaseDraft, Manifest: []byte(manifest)}}
	h := NewClientReleaseHandler(service.NewClientReleaseService(repo, nil))
	router := gin.New()
	router.PUT("/releases/:id/files/:name", h.Upload)
	req := httptest.NewRequest("PUT", "/releases/1/files/qxapi-linux-amd64", io.LimitReader(clientReleaseRepeatingReader{}, size))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	http.MaxBytesHandler(router, 256<<20).ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Len(t, repo.release.Files, 1)
	path := filepath.Join(root, "client-releases", "qxapi", "1.0.0", "qxapi-linux-amd64")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, size, info.Size())
	require.NoFileExists(t, path+".part")
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	storedHash := sha256.New()
	_, err = io.Copy(storedHash, f)
	require.NoError(t, err)
	require.Equal(t, hash.Sum(nil), storedHash.Sum(nil))
}

type adminClientReleaseRetentionRepo struct {
	adminClientReleaseRepo
	retained []*service.ClientRelease
}

func (r *adminClientReleaseRetentionRepo) List(context.Context) ([]*service.ClientRelease, error) {
	return append([]*service.ClientRelease{r.release}, r.retained...), nil
}

func TestClientReleasePublishTooOldHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("DATA_DIR", root)
	repo := &adminClientReleaseRetentionRepo{}
	for i := 2; i <= 6; i++ {
		repo.retained = append(repo.retained, &service.ClientRelease{ID: int64(i), Version: fmt.Sprintf("0.9.%d", i), Status: service.ClientReleasePublished})
	}
	s := service.NewClientReleaseService(repo, nil)
	manifest := fmt.Sprintf(`{"version":"0.0.1","files":{"qxapi-linux-amd64":{"size":5,"sha256":"%x"}}}`, sha256.Sum256([]byte("hello")))
	release, err := s.Create(context.Background(), json.RawMessage(manifest))
	require.NoError(t, err)
	_, err = s.Upload(context.Background(), release.ID, "qxapi-linux-amd64", strings.NewReader("hello"))
	require.NoError(t, err)
	router := gin.New()
	router.POST("/releases/:id/publish", NewClientReleaseHandler(s).Publish)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/releases/1/publish", nil))
	require.Equal(t, http.StatusConflict, w.Code)
	require.JSONEq(t, `{"code":409,"reason":"CLIENT_RELEASE_TOO_OLD","message":"Release is older than the five retained versions"}`, w.Body.String())
	require.Equal(t, service.ClientReleaseDraft, repo.release.Status)
	require.Nil(t, repo.release.PublishedAt)
	data, err := os.ReadFile(filepath.Join(root, "client-releases", "qxapi", "0.0.1", "qxapi-linux-amd64"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
}
