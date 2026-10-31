//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicClientReleaseRepo struct {
	service.ClientReleaseRepository
	releases []*service.ClientRelease
	err      error
}

func (r *publicClientReleaseRepo) List(context.Context) ([]*service.ClientRelease, error) {
	return append([]*service.ClientRelease{}, r.releases...), r.err
}
func (r *publicClientReleaseRepo) GetByVersion(_ context.Context, version string) (*service.ClientRelease, error) {
	for _, v := range r.releases {
		if v.Version == version {
			return v, nil
		}
	}
	return nil, service.ErrClientReleaseNotFound
}

type publicClientReleaseSettings struct{ service.SettingRepository }

func (publicClientReleaseSettings) GetValue(context.Context, string) (string, error) {
	return "https://site.example/", nil
}
func TestClientReleasePublicHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("DATA_DIR", root)
	repo := &publicClientReleaseRepo{}
	settings := service.NewSettingService(publicClientReleaseSettings{}, &config.Config{})
	h := NewClientReleaseHandler(service.NewClientReleaseService(repo, settings))
	r := gin.New()
	r.GET("/latest", h.Latest)
	r.GET("/notes", h.Notes)
	r.GET("/files/:version/:name", h.File)
	r.HEAD("/files/:version/:name", h.File)
	request := func(method, path, rangeHeader string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Range", rangeHeader)
		r.ServeHTTP(w, req)
		return w
	}
	require.Equal(t, 404, request("GET", "/latest", "").Code)
	file := service.ClientReleaseFile{Name: "qxapi-linux-amd64", ClientReleaseFileSpec: service.ClientReleaseFileSpec{Size: 10, SHA256: "hash"}}
	repo.releases = []*service.ClientRelease{
		{Product: "qxapi", Version: "1.0.0", Status: service.ClientReleasePublished, Notes: "中文", Files: []service.ClientReleaseFile{file}},
		{Product: "qxapi", Version: "2.0.0", Status: service.ClientReleaseDraft, Files: []service.ClientReleaseFile{file}},
		{Product: "qxapi", Version: "3.0.0", Status: service.ClientReleaseWithdrawn, Files: []service.ClientReleaseFile{file}},
	}
	dir := filepath.Join(root, "client-releases", "qxapi", "1.0.0")
	require.NoError(t, os.MkdirAll(dir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, file.Name), []byte("0123456789"), 0600))
	w := request("GET", "/latest?lang=en", "")
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"version":"1.0.0","notes":"中文","url":"https://site.example/downloads","assets":{"qxapi-linux-amd64":{"url":"/api/v1/client-releases/qxapi/files/1.0.0/qxapi-linux-amd64","size":10,"sha256":"hash"}}}`, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	w = request("GET", "/notes?after=0.9.0&upto=1.0.0&limit=1&lang=en", "")
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"releases":[{"version":"1.0.0","notes":"中文","url":"https://site.example/downloads"}]}`, w.Body.String())
	require.JSONEq(t, `{"releases":[]}`, request("GET", "/notes?after=1.0.0", "").Body.String())
	for _, path := range []string{"/notes?after=dev", "/notes?upto=bad", "/notes?limit=-1", "/notes?limit=0", "/notes?limit=abc"} {
		require.Equal(t, 400, request("GET", path, "").Code, path)
	}
	path := "/files/1.0.0/" + file.Name
	w = request("GET", path, "")
	require.Equal(t, 200, w.Code)
	require.Equal(t, "0123456789", w.Body.String())
	require.Equal(t, "10", w.Header().Get("Content-Length"))
	w = request("GET", path, "bytes=2-5")
	require.Equal(t, 206, w.Code)
	require.Equal(t, "2345", w.Body.String())
	require.Equal(t, "bytes 2-5/10", w.Header().Get("Content-Range"))
	require.Equal(t, "4", w.Header().Get("Content-Length"))
	first := request("GET", path, "bytes=0-4")
	second := request("GET", path, "bytes=5-")
	require.Equal(t, "0123456789", first.Body.String()+second.Body.String())
	w = request("HEAD", path, "")
	require.Equal(t, 200, w.Code)
	require.Empty(t, w.Body.String())
	require.Equal(t, "10", w.Header().Get("Content-Length"))
	require.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
	require.Equal(t, 416, request("GET", path, "bytes=30-").Code)
	for _, tc := range []struct {
		path   string
		status int
		reason string
	}{
		{"/files/2.0.0/" + file.Name, 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"/files/3.0.0/" + file.Name, 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"/files/9.0.0/" + file.Name, 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"/files/1.0.0/qxapi-linux-arm64", 404, "CLIENT_RELEASE_NOT_FOUND"},
		{"/files/dev/" + file.Name, 400, "CLIENT_RELEASE_INVALID_VERSION"},
		{"/files/1.0.0-dirty/" + file.Name, 400, "CLIENT_RELEASE_INVALID_VERSION"},
		{"/files/1.0.0-%2Fetc/" + file.Name, 404, ""},
		{"/files/1.0.0/secret.txt", 400, "CLIENT_RELEASE_INVALID_FILE_NAME"},
		{"/files/1.0.0/..", 400, "CLIENT_RELEASE_INVALID_FILE_NAME"},
	} {
		w = request("GET", tc.path, "")
		require.Equal(t, tc.status, w.Code, tc.path)
		if tc.reason != "" {
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, tc.reason, body["reason"])
		}
	}
	repo.releases[0].Status = service.ClientReleaseWithdrawn
	require.Equal(t, 404, request("GET", path, "").Code)
	require.Equal(t, 404, request("GET", "/latest", "").Code)
}
