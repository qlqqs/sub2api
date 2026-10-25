# 客户端下载页（后台可配置下载链接）

## 需求

- 新增「客户端下载」页 `/downloads`，列出 Codex、CC Switch 等客户端及各平台下载链接。
- 入口在顶栏「模型广场」旁边（qx 主题公共顶栏 `QxSiteHeader`、登录后顶栏 `AppHeader`）。
- 后台「系统设置」可配置：开关 + 客户端列表（名称、简介、图标、版本、主页、各平台下载链接）。
- 分工：后端 codex/gpt-6-astra（high），前端 Claude Opus 5.5（high）。

## 接口契约

### 设置项

| key | 类型 | 默认 |
|---|---|---|
| `client_downloads_enabled` | bool（"true"/"false"） | `true` |
| `client_downloads` | JSON 数组（存字符串） | 下方种子列表 |

键缺失时（老库升级）按默认值处理，所以部署后不用手动配置就能看到 Codex / CC Switch。
管理员存过 `[]` 就是空列表，不再回退到种子。

### 数据结构

```jsonc
// ClientDownload
{
  "id": "codex",                 // [a-zA-Z0-9_-]{1,32}；缺省时后端生成；列表内唯一
  "name": "Codex",               // 必填，trim 后 1..50 字符
  "description": "…",            // 可空，≤ 300 字符
  "icon_url": "",                // 可空；非空须为绝对 http(s) URL，≤ 2048
  "version": "",                 // 可空，≤ 32 字符
  "homepage_url": "",            // 可空；非空须为绝对 http(s) URL，≤ 2048
  "links": [ClientDownloadLink]  // 1..20 个
}
// ClientDownloadLink
{
  "label": "Windows x64",        // 必填，trim 后 1..50 字符
  "platform": "windows",         // windows | macos | linux | android | ios | other
  "arch": "x64",                 // "" | x64 | arm64 | x86 | universal
  "url": "https://…"             // 必填，绝对 http(s) URL，≤ 2048
}
```

- 客户端最多 20 个；顺序即展示顺序。
- 校验失败返回 400，错误信息指明第几个客户端 / 第几个链接、哪个字段。
- 字符串字段一律 trim 后再存。

### 接口

- `GET /api/v1/admin/settings`：返回 `client_downloads_enabled`、`client_downloads`（数组，不是字符串）。
- `PUT /api/v1/admin/settings`：两个字段均可选（指针语义，缺省保留原值）；校验同上。
- `GET /api/v1/settings/public` 与 HTML 注入的 `window.__APP_CONFIG__`：同样返回这两个字段。
  开关为 false 时公共接口仍返回 `client_downloads_enabled: false`，`client_downloads` 返回 `[]`。

### 种子列表（默认值）

```json
[
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
]
```

## 前端

- 路由 `/downloads`（`requiresAuth: false`，同模型广场可匿名直接访问），开关关闭时重定向首页，列表为空时显示空状态；`?embedded=1` 时嵌入 AppLayout（同模型广场）。
- 布局参照 ccswitch.io/zh/download：按系统分标签页（只列有下载的系统），默认选中访客的系统；
  每个客户端一个「下载 {系统} 版 · {架构}」主按钮，架构自动识别（UA + Chromium 客户端提示 `architecture`），
  其余架构列为「其他版本」；没有该系统版本的客户端显示「暂无 {系统} 版」；底部按系统给出选架构提示。
- 识别不到架构时：优先不区分架构的链接，其次 Mac 选 Apple Silicon、其余选 x64。
- 后台系统设置「通用」页新增「客户端下载」卡片：开关 + 客户端 / 链接的增删、上移下移。
- zh / en i18n。

## 验收

- 后端：单测覆盖默认值、校验、公共接口开关；`api_contract_test.go` 更新；`go test -tags unit ./...` 通过。
- 前端：lint、typecheck、test:run 通过；本机环境截图检查下载页、顶栏入口、后台设置卡片。
