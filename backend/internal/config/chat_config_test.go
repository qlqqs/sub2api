package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestChatConfigEnvironment(t *testing.T) {
	for _, value := range []string{"", "http://chat-webui:8080"} {
		t.Run(value, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			t.Setenv("CHAT_WEBUI_URL", value)
			cfg, err := Load()
			require.NoError(t, err)
			require.Equal(t, value, cfg.Chat.WebUIURL)
		})
	}
}
