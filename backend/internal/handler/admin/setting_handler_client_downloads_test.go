//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func validClientDownloads() []dto.ClientDownload {
	return []dto.ClientDownload{{
		ID: "example", Name: "Client",
		Links: []dto.ClientDownloadLink{{Label: "Linux", Platform: "linux", Arch: "x64", URL: "https://example.com/client"}},
	}}
}

func TestUpdateClientDownloadsRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func([]dto.ClientDownload) []dto.ClientDownload
		field  string
	}{
		{"bad URL", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].URL = "javascript:alert(1)"; return v }, "client 1 link 1 url"},
		{"relative URL", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].URL = "/client"; return v }, "client 1 link 1 url"},
		{"empty URL", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].URL = " "; return v }, "client 1 link 1 url"},
		{"long URL", func(v []dto.ClientDownload) []dto.ClientDownload {
			v[0].Links[0].URL = "https://example.com/" + strings.Repeat("x", 2048)
			return v
		}, "client 1 link 1 url"},
		{"bad platform", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].Platform = "darwin"; return v }, "client 1 link 1 platform"},
		{"bad arch", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].Arch = "amd64"; return v }, "client 1 link 1 arch"},
		{"too many links", func(v []dto.ClientDownload) []dto.ClientDownload {
			v[0].Links = make([]dto.ClientDownloadLink, 21)
			return v
		}, "client 1 links"},
		{"no links", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links = nil; return v }, "client 1 links"},
		{"too many clients", func(v []dto.ClientDownload) []dto.ClientDownload { return make([]dto.ClientDownload, 21) }, "client_downloads"},
		{"empty name", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Name = " \t "; return v }, "client 1 name"},
		{"long name", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Name = strings.Repeat("字", 51); return v }, "client 1 name"},
		{"long description", func(v []dto.ClientDownload) []dto.ClientDownload {
			v[0].Description = strings.Repeat("字", 301)
			return v
		}, "client 1 description"},
		{"long version", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Version = strings.Repeat("x", 33); return v }, "client 1 version"},
		{"bad icon", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].IconURL = "data:image/png,test"; return v }, "client 1 icon_url"},
		{"bad homepage", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].HomepageURL = "//example.com"; return v }, "client 1 homepage_url"},
		{"empty label", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].Links[0].Label = " "; return v }, "client 1 link 1 label"},
		{"long label", func(v []dto.ClientDownload) []dto.ClientDownload {
			v[0].Links[0].Label = strings.Repeat("x", 51)
			return v
		}, "client 1 link 1 label"},
		{"duplicate id", func(v []dto.ClientDownload) []dto.ClientDownload { return append(v, v[0]) }, "client 2 id"},
		{"duplicate trimmed id", func(v []dto.ClientDownload) []dto.ClientDownload {
			v = append(v, v[0])
			v[1].ID = " example "
			return v
		}, "client 2 id"},
		{"bad id", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].ID = "invalid.id"; return v }, "client 1 id"},
		{"long id", func(v []dto.ClientDownload) []dto.ClientDownload { v[0].ID = strings.Repeat("x", 33); return v }, "client 1 id"},
		{"second link field", func(v []dto.ClientDownload) []dto.ClientDownload {
			v[0].Links = append(v[0].Links, v[0].Links[0])
			v[0].Links[1].URL = "ftp://example.com"
			return v
		}, "client 1 link 2 url"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
			rec := doUpdateSettings(t, h, map[string]any{"client_downloads": tt.change(validClientDownloads())}, nil)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), tt.field)
			require.NotContains(t, repo.values, service.SettingKeyClientDownloads)
		})
	}
}

func TestUpdateClientDownloadsTrimsGeneratesIDsAndReturnsSavedList(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	items := validClientDownloads()
	items[0] = dto.ClientDownload{
		ID: " \t", Name: " 客户端 ", Description: " 简介 ", IconURL: " https://example.com/icon.png ",
		Version: " 1.0 ", HomepageURL: " https://example.com ",
		Links: []dto.ClientDownloadLink{{Label: " Linux ", Platform: " linux ", Arch: " x64 ", URL: " https://example.com/client "}},
	}
	items = append(items, items[0])
	rec := doUpdateSettings(t, h, map[string]any{"client_downloads_enabled": false, "client_downloads": items}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "false", repo.values[service.SettingKeyClientDownloadsEnabled])
	stored := dto.ParseClientDownloads(repo.values[service.SettingKeyClientDownloads])
	require.Len(t, stored, 2)
	require.Regexp(t, `^[a-zA-Z0-9_-]{1,32}$`, stored[0].ID)
	require.Regexp(t, `^[a-zA-Z0-9_-]{1,32}$`, stored[1].ID)
	require.NotEqual(t, stored[0].ID, stored[1].ID)
	expected := dto.ClientDownload{ID: stored[0].ID, Name: "客户端", Description: "简介", IconURL: "https://example.com/icon.png", Version: "1.0", HomepageURL: "https://example.com", Links: validClientDownloads()[0].Links}
	require.Equal(t, expected, stored[0])
	var result struct {
		Data dto.SystemSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.False(t, result.Data.ClientDownloadsEnabled)
	require.Equal(t, stored, result.Data.ClientDownloads)
	// Disabled public visibility must not hide the editable list from admin GET.
	getRec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(getRec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, getRec.Code, getRec.Body.String())
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &result))
	require.False(t, result.Data.ClientDownloadsEnabled)
	require.Equal(t, stored, result.Data.ClientDownloads)
}

func TestUpdateClientDownloadsOptionalFieldsAndEmptyList(t *testing.T) {
	raw, err := json.Marshal(validClientDownloads())
	require.NoError(t, err)
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyClientDownloadsEnabled: "false", service.SettingKeyClientDownloads: string(raw)})
	for _, body := range []map[string]any{{"site_name": "Changed"}, {"client_downloads_enabled": nil, "client_downloads": nil}} {
		rec := doUpdateSettings(t, h, body, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, "false", repo.values[service.SettingKeyClientDownloadsEnabled])
		require.JSONEq(t, string(raw), repo.values[service.SettingKeyClientDownloads])
	}
	rec := doUpdateSettings(t, h, map[string]any{"client_downloads_enabled": true}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyClientDownloadsEnabled])
	require.JSONEq(t, string(raw), repo.values[service.SettingKeyClientDownloads])
	rec = doUpdateSettings(t, h, map[string]any{"client_downloads": []dto.ClientDownload{}}, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyClientDownloadsEnabled])
	require.Equal(t, "[]", repo.values[service.SettingKeyClientDownloads])
}

func TestClientDownloadsValidationBoundaries(t *testing.T) {
	items := make([]dto.ClientDownload, 20)
	platforms := []string{"windows", "macos", "linux", "android", "ios", "other"}
	arches := []string{"", "x64", "arm64", "x86", "universal"}
	for i := range items {
		items[i] = validClientDownloads()[0]
		items[i].ID = "" // Generate unique IDs for all 20 clients.
		items[i].Name = strings.Repeat("字", 50)
		items[i].Description = strings.Repeat("字", 300)
		items[i].Version = strings.Repeat("字", 32)
		items[i].Links = make([]dto.ClientDownloadLink, 20)
		for j := range items[i].Links {
			items[i].Links[j] = dto.ClientDownloadLink{Label: strings.Repeat("字", 50), Platform: platforms[j%len(platforms)], Arch: arches[j%len(arches)], URL: "http://example.com/" + strings.Repeat("x", 2048-len("http://example.com/"))}
		}
	}
	require.NoError(t, normalizeClientDownloads(items))
}

func TestClientDownloadsAuditDiff(t *testing.T) {
	before := &service.SystemSettings{ClientDownloadsEnabled: true, ClientDownloads: "[]"}
	after := &service.SystemSettings{ClientDownloadsEnabled: false, ClientDownloads: `[{"id":"example"}]`}
	require.ElementsMatch(t, []string{"client_downloads_enabled", "client_downloads"}, diffSettings(before, after, &service.AuthSourceDefaultSettings{}, &service.AuthSourceDefaultSettings{}, UpdateSettingsRequest{}))
}
