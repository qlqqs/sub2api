package service

import "encoding/json"

// defaultClientDownloads is used only when the setting key is missing.
// An explicitly saved empty array must not restore these seeds.
const defaultClientDownloads = `[
  {
    "id": "codex",
    "name": "Codex",
    "description": "OpenAI Codex 桌面客户端，配置本站 API 地址和密钥即可使用。",
    "icon_url": "", "version": "", "homepage_url": "",
    "links": [
      {"label": "Windows", "platform": "windows", "arch": "", "url": "https://codexapp.agentsmirror.com/latest/win"},
      {"label": "Windows x64", "platform": "windows", "arch": "x64", "url": "https://codexapp.agentsmirror.com/latest/win-x64"},
      {"label": "Windows ARM64", "platform": "windows", "arch": "arm64", "url": "https://codexapp.agentsmirror.com/latest/win-arm64"},
      {"label": "Apple Silicon Mac", "platform": "macos", "arch": "arm64", "url": "https://codexapp.agentsmirror.com/latest/mac-arm64"},
      {"label": "Intel Mac", "platform": "macos", "arch": "x64", "url": "https://codexapp.agentsmirror.com/latest/mac-intel"}
    ]
  },
  {
    "id": "cc-switch",
    "name": "CC Switch",
    "description": "一键切换 Claude Code、Codex 等工具的 API 配置。",
    "icon_url": "", "version": "v4.0.20", "homepage_url": "",
    "links": [
      {"label": "Windows x64", "platform": "windows", "arch": "x64", "url": "https://dl.ccswitch.io/v4.0.20/CC-Switch-v4.0.20-Windows.msi"},
      {"label": "Windows ARM64", "platform": "windows", "arch": "arm64", "url": "https://dl.ccswitch.io/v4.0.20/CC-Switch-v4.0.20-Windows-arm64.msi"}
    ]
  }
]`

func parseClientDownloadSettings(settings map[string]string) (bool, string) {
	enabled := true
	if raw, ok := settings[SettingKeyClientDownloadsEnabled]; ok {
		enabled = raw == "true"
	}
	raw, ok := settings[SettingKeyClientDownloads]
	if !ok {
		raw = defaultClientDownloads
	}
	// Always expose an array, including for corrupt or null stored values.
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil || items == nil {
		raw = "[]"
	}
	return enabled, raw
}
