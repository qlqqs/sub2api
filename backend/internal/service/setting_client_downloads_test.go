//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestClientDownloadsSettingsDefaultsAndPublicVisibility(t *testing.T) {
	for _, tt := range []struct {
		name       string
		values     map[string]string
		enabled    bool
		adminJSON  string
		publicJSON string
	}{
		{"missing keys seed defaults", map[string]string{}, true, defaultClientDownloads, defaultClientDownloads},
		{"saved empty stays empty", map[string]string{SettingKeyClientDownloads: "[]"}, true, "[]", "[]"},
		{"disabled hides seeds", map[string]string{SettingKeyClientDownloadsEnabled: "false"}, false, defaultClientDownloads, "[]"},
		{"disabled retains saved list", map[string]string{SettingKeyClientDownloadsEnabled: "false", SettingKeyClientDownloads: defaultClientDownloads}, false, defaultClientDownloads, "[]"},
		{"null becomes empty", map[string]string{SettingKeyClientDownloads: "null"}, true, "[]", "[]"},
		{"invalid becomes empty", map[string]string{SettingKeyClientDownloads: "{"}, true, "[]", "[]"},
		{"object becomes empty", map[string]string{SettingKeyClientDownloads: "{}"}, true, "[]", "[]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			admin, err := NewSettingService(&settingGetAllRepoStub{values: tt.values}, &config.Config{}).GetAllSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.enabled, admin.ClientDownloadsEnabled)
			require.JSONEq(t, tt.adminJSON, admin.ClientDownloads)
			svc := NewSettingService(&settingPublicRepoStub{values: tt.values}, &config.Config{})
			public, err := svc.GetPublicSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, tt.enabled, public.ClientDownloadsEnabled)
			require.JSONEq(t, tt.publicJSON, public.ClientDownloads)
			injected, err := svc.GetPublicSettingsForInjection(ctx)
			require.NoError(t, err)
			raw, err := json.Marshal(injected)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			require.JSONEq(t, tt.publicJSON, string(fields["client_downloads"]))
			enabledJSON, err := json.Marshal(tt.enabled)
			require.NoError(t, err)
			require.JSONEq(t, string(enabledJSON), string(fields["client_downloads_enabled"]))
		})
	}
}

func TestClientDownloadsInitialDefaults(t *testing.T) {
	repo := &forwardedIPMigrationRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
	require.Equal(t, "true", repo.values[SettingKeyClientDownloadsEnabled])
	require.JSONEq(t, defaultClientDownloads, repo.values[SettingKeyClientDownloads])
}
