# 接口约定（阶段 1–5 各子代理共用，2026-10-24）

本文件是 fork 后端（阶段 1/3）、fork 前端（阶段 2）、sub2api 后端（阶段 4）、sub2api 前端（阶段 5）之间的唯一接口约定。
实现中发现需要改约定时：不要自行改对方的代码，在自己的报告里写明「约定变更提议」，由主会话统一调整。

路径约定：浏览器看到的路径带 `/_chat` 前缀；Open WebUI 后端收到的路径不带前缀（sub2api 剥掉）。

## A. 浏览器 → sub2api

| 项 | 说明 |
|---|---|
| `POST /api/v1/chat/session` | 控制台常规 Bearer JWT 鉴权。响应 `Set-Cookie: s2a_chat=<当前 access token>; Path=/_chat; HttpOnly; SameSite=Strict`（HTTPS 时加 `Secure`，判定方式同 sub2api 现有 Cookie）；body 标准 `{code:0,data:{expires_at}}`。父页在 token 刷新后再次调用。 |
| `DELETE /api/v1/chat/session` | 清除该 Cookie 与 Open WebUI 自己的 `token` Cookie（均 `Max-Age=0`、`Path=/_chat`）。不要求登录有效（退出时 token 可能已过期）。父页发现账号变化时先 DELETE 再 POST。 |
| `GET /api/v1/chat/status` | `{code:0,data:{enabled:bool}}`；`enabled` = 配置了 `CHAT_WEBUI_URL`。公开或 JWT 均可（与 studio status 一致）。 |
| `/_chat/*` | 反代到 Open WebUI，见 B。 |
| `/_chat/__sub2api/*` | sub2api 自有接口，Cookie 鉴权，见 C。不转发给 Open WebUI。 |

iframe 内对 `/_chat/` 发出的每个 `fetch` 带：
- `X-Sub2API-Key-Id: <对话 Key ID>`（用户在「模型与密钥」里选的；未选不带）
- `X-Sub2API-Image-Key-Id: <生图 Key ID>`（可选；未选不带）

`<img>`、WebSocket 握手、文件下载不带这两个头，也不需要（不调模型）。

## B. sub2api → Open WebUI（反代 `/_chat/*`）

1. 先删除客户端带来的所有 `X-Sub2API-*`、`X-Forwarded-*`、`X-Real-IP`、`Forwarded`。
2. 静态资源免 Cookie：`/_chat/_app/*`、`/_chat/static/*`、`/_chat/assets/*`、`/_chat/favicon*`、`/_chat/manifest.json`、`/_chat/embed/*`，以及 HTML 页面导航（`GET`、`Accept` 含 `text/html`、路径不在 `/_chat/api/`、`/_chat/ws/`、`/_chat/oauth/` 下）。这些请求不注入身份头。
3. 其余请求：校验 `s2a_chat` Cookie（复用 JWT 校验：签名、过期、封禁、token 版本）。无效 → 401 `{"error":{"code":"CHAT_SESSION_INVALID","message":"登录已失效"}}`，不转发（iframe 据此 code 区分 sub2api 的 401 与 Open WebUI 自己的 401）。
4. 注入：

   | 头 | 值 |
   |---|---|
   | `X-Sub2API-User-Id` | 用户 ID |
   | `X-Sub2API-Email` | `u<用户ID>@sub2api.internal` |
   | `X-Sub2API-Name` | 用户名，空则邮箱 `@` 前部分；**按 UTF-8 百分号编码（RFC 3986 unreserved 以外的字节全部编码），保证头值为 ASCII**（中文头值会让 uvicorn 的 WebSocket 握手崩溃）；Open WebUI 侧解码 |
   | `X-Sub2API-User-Role` | sub2api 管理员 `admin`，其他 `user` |
   | `X-Sub2API-Key` | `X-Sub2API-Key-Id` 指向的 Key 明文：属于当前用户、状态可用、未过期、额度未用完时才注入；否则不注入。WebSocket 升级请求永不注入。 |
   | `X-Sub2API-Key-Platform` | 上述对话 Key 所属分组的平台（`anthropic` / `openai` / `gemini` / `antigravity` / …，取 sub2api 分组 platform 原值）；注入 Key 时才注入 |
   | `X-Sub2API-Image-Key` | `X-Sub2API-Image-Key-Id` 指向的 Key 明文：同上校验，且分组支持生图（见 C.1 `supports_images`） |
   | `X-Forwarded-For` / `X-Real-IP` | 用户真实 IP（单个值，解析规则同 studio_proxy） |

5. 剥掉 `/_chat` 前缀后转发（查询串保留）；支持 WebSocket（`/_chat/ws/socket.io/`）。
6. 响应：根相对的 `Location`（以单个 `/` 开头）补 `/_chat` 前缀；`Set-Cookie` 的 `Path=/` 改为 `Path=/_chat`（兜底，fork 自己也会改）；删除上游 `X-Frame-Options`、`Content-Security-Policy`；设 `X-Frame-Options: SAMEORIGIN` 与下列 CSP：

   - 默认（`/_chat/*`）：
     ```
     default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob: data:; worker-src 'self' blob:; frame-src 'self' blob: data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'
     ```
   - Artifact 沙箱页（精确路径 `/_chat/embed/artifact-frame.html`）：
     ```
     default-src 'none'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob: data:; style-src 'self' 'unsafe-inline' blob: data:; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' data: blob:; connect-src 'none'; frame-src 'none'; worker-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'
     ```
7. 请求体上限：`/_chat/api/v1/files/` 上传 25 MiB，其他 5 MiB；超限 413。
8. 未配置 `CHAT_WEBUI_URL` → `/_chat/*` 与 `/api/v1/chat/session` 返回 503；上游不可达 → 503。

## C. `/_chat/__sub2api/*`（sub2api 自有，`s2a_chat` Cookie 鉴权，返回原始 JSON，不包 `{code,data}`）

错误：`{"error":{"code":"<机器码>","message":"<中文>"}}` + 对应 HTTP 状态。
写操作要求 `Content-Type: application/json`（否则 415），防跨站表单。

### C.1 `GET /_chat/__sub2api/keys`

```json
{"keys":[{
  "id": 12, "name": "默认", "status": "active",
  "usable": true, "unusable_reason": "",            // disabled / expired / quota_exhausted / group_unavailable，中文说明放 message 由前端映射
  "group": {"id": 3, "name": "Claude", "platform": "anthropic"},
  "supports_chat": true,
  "supports_images": false,                          // 分组开启 allow_image_generation 且平台为 openai / grok / composite
  "quota": 0, "quota_used": 0,                       // 美元；0 = 不限
  "expires_at": null                                 // RFC3339 或 null
}]}
```

不返回明文、不返回脱敏片段以外的任何密钥信息（可以有 `"masked": "sk-…abcd"`）。

### C.2 `POST /_chat/__sub2api/keys`

请求 `{"name":"对话","group_id":3}`；只允许当前用户可用的分组。响应 `{"key": <同 C.1 单项>}`，无明文。

### C.3 `GET /_chat/__sub2api/groups`

```json
{"groups":[{"id":3,"name":"Claude","platform":"anthropic","description":"","supports_images":false}]}
```

当前用户可以建 Key 的分组（与控制台「创建 Key」下拉一致）。

### C.4 `GET /_chat/__sub2api/model-prices?key_id=12`

```json
{"models":[{"id":"claude-sonnet-4-5","price_text":"输入 ¥3.00 / 百万 tokens · 输出 ¥15.00 / 百万 tokens"}]}
```

- 取模型广场在该 Key 分组下对当前用户的价格；`price_text` 由 sub2api 按模型广场的展示口径生成（币种 / 汇率模式与广场一致），不出现「倍率」字样。
- 没有价格的模型不返回该项（前端不显示单价）。
- Key 不属于本人 → 404。

## D. Open WebUI 侧（fork 后端）

- 只信任 TCP 对端在 `S2A_TRUSTED_PROXY_CIDRS` 内的请求的 `X-Sub2API-*`；其他对端剥头，非静态请求 403。
- 对话 Key：`X-Sub2API-Key`（只对 http 作用域读入 ContextVar）。为空时模型调用返回 400 `{"detail":"请选择可用的 Key"}`。
- 生图 Key：`X-Sub2API-Image-Key`；为空时回退对话 Key。
- OpenAI 平台 Key（`X-Sub2API-Key-Platform` = `openai`）的连接用 `api_type=responses`；其他走 Chat Completions。
- `/api/config` 响应新增：

  ```json
  "sub2api": {"embed": true, "hide_branding": false, "native_web_search": true, "image_generation": true}
  ```

  非嵌入模式不输出该字段（前端据此判断嵌入模式）。
- `/api/config` 的 token 用户与身份头不一致时按未登录处理（返回未登录形态的 config，而不是 401），让前端重新 signin。
- `/api/models` 拉网关模型列表失败时（如余额不足时网关 `/v1/models` 返回 `{"code":"INSUFFICIENT_BALANCE",...}`），返回网关的 HTTP 状态码，body 为 `{"detail":"<中文提示>","s2a_gateway_error":{"status":<int>,"code":"<网关 code 或 error.type/code>","message":"<网关原文>"}}`；前端据此显示与对话错误相同的中文提示（余额不足带「去充值」），而不是空模型列表。
- 屏蔽表补充：`GET /api/v1/functions/`（精确路径，只读）对普通用户放行（聊天界面显示管理员装的过滤器 / 动作）；`GET /api/v1/groups/`（只读）对管理员放行（预设助手访问控制选择器）。
- 用户选择的 Key ID 存在 Open WebUI 用户设置 `ui.sub2api = {"key_id": 12, "image_key_id": 34}`（原版 `/api/v1/users/user/settings` 接口，按用户隔离）。后端不需要读它。

- 工具服务器配置（`/api/v1/configs/tool_servers` 及 verify）仅 admin 可写，并持久化；其余 configs 写入口一律禁止。
- 健康检查：`/health` 不做匿名例外；容器 HEALTHCHECK 用静态资源（如 `/static/favicon.png`）。
- 管理员（admin 角色）在嵌入模式下**没有**读取 / 修改他人会话、文件、笔记、日历的特权（PRD R3.1）；共享只走原版显式授权（预设助手、提示词等 Workspace 资源）。

## E. postMessage（父页 `/chat` ↔ iframe `/_chat/`）

双方都校验 `event.origin === location.origin`；iframe 侧另校验 `event.source === window.parent`，父页侧校验 `event.source === iframe.contentWindow`。

```ts
// 父 → iframe
{ type: 'sub2api:chat-session', revision: number, theme: 'light' | 'dark', locale: 'zh-CN' | 'en-US' }
{ type: 'sub2api:chat-revoke' }       // 退出 / 换账号：iframe 清掉 owui: 前缀的本地存储并显示空白页
// iframe → 父
{ type: 'chat:ready' }                // iframe 启动后发；父页收到后回 chat-session
{ type: 'chat:balance-changed' }      // 每轮回复结束
{ type: 'chat:navigate', path: '/purchase' | '/keys' }   // 白名单外忽略
{ type: 'chat:session-expired' }      // iframe 内 /_chat 请求收到 sub2api 的 401（s2a_chat 失效）时发；父页重新签发 Cookie 后回 chat-session（revision+1），iframe 可重试一次
```

- 父页退出 / 换账号时也会直接清掉同源存储里所有 `owui:` 前缀的键，并用 `s2a_chat_owner` 记录本地数据属于哪个用户 → Open WebUI 前端的用户数据只能写在 `owui:` 前缀下，不得使用 `s2a_` 前缀；IndexedDB 等其他存储由 iframe 在收到 revoke 时自己清。
- `chat-session` 里的 `locale` / `theme` 与当前不同时，iframe 要即时切换（不能只认首次加载的 `?lang`）。

- iframe `src` = `/_chat/?lang=<locale>&theme=<light|dark>`（theme 用于首屏预置，避免闪烁）。父页在加载 iframe 前 `POST /api/v1/chat/session`，失败时显示错误，不加载 iframe。
- `revision` 每次父页重新签发 session Cookie（token 刷新）时加 1；iframe 收到更大的 revision 时不需要做任何事（Cookie 已更新），仅用于调试。

## F. Artifact 沙箱页（fork 前端自带，sub2api 只设 CSP）

- 文件：fork `static/embed/artifact-frame.html`（构建后在 `build/embed/artifact-frame.html`，浏览器路径 `/_chat/embed/artifact-frame.html`）。
- 使用方：`<iframe sandbox="allow-scripts …（不含 allow-same-origin）" src="/_chat/embed/artifact-frame.html">`。
- 协议由 fork 前端自行定义并写进 fork 的 `docs/sub2api-embed.md`；因为沙箱页是不透明源，父 → 沙箱用 `targetOrigin '*'`，沙箱侧校验 `event.source === window.parent`，父侧校验 `event.source === iframe.contentWindow`。

## G. 环境变量（fork 镜像默认值，阶段 3 写入 Dockerfile；sub2api 本机 / 一号机 compose 覆盖）

| 变量 | 值 |
|---|---|
| `S2A_EMBED_MODE` | `1` |
| `S2A_GATEWAY_URL` | `http://sub2api:8080`（只允许无路径 origin） |
| `S2A_TRUSTED_PROXY_CIDRS` | 部署时设为对话网络网段 |
| `S2A_HIDE_BRANDING` | `0`（我们的部署 `.env` 设 `1`） |
| `S2A_NATIVE_WEB_SEARCH` | `true` |
| `S2A_BUILTIN_TOOL_CATEGORIES` | `time,user_input,files,chats,notes,tasks,calendar,image_generation`（生图在原生函数调用模式下以内置工具提供） |
| `S2A_SLIM_DOC_LOADERS` | `true` |
| `S2A_IMAGE_MAX_N` | `4` |
| 其余 | design §2.3、§3.2、§4、§5.1 列出的原版变量 |

sub2api：`CHAT_WEBUI_URL`（如 `http://chat-webui:8080`），空 = 关闭。
