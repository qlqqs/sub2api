# 设计：创作工作台（Nova Studio 嵌入）

需求见 `prd.md`，选型依据见 `research/selection.md`。本文件只写技术方案。

## 1. 拓扑

```
浏览器 ──https──> 1Panel（不改，整域名反代）──> 127.0.0.1:18080 sub2api
                                                   │
             /_nova/*  /api/nova/*（含 /api/nova/ws）│ sub2api 内部转发（fork 自有中间件）
                                                   ▼
                                  nova-studio:3000（共享网络 sub2api-studio，无宿主机端口）
                                                   │
                     上游调用一律 → http://sub2api:8080（内网网关，带 X-Forwarded-For）
```

- 两个 compose 项目：
  - `sub2api-custom`：新增命名网络 `sub2api-studio`（固定网段，如 `172.31.250.0/24`），sub2api 加入；网络由这边创建，保证 sub2api 可单独运行。
  - `nova-studio`（`/root/nova-studio`）：`networks: sub2api-studio: external: true`，服务别名 `nova-studio`，数据卷 `./data`（SQLite + 图片）。
- 本机：`sub2api-local` 创建 `sub2api-local-studio`；新目录 `~/workspace/sub2api-local-nova`，compose 项目 `sub2api-local-nova`，容器 `sub2api-local-nova`。

## 2. sub2api 后端（fork 自有文件 + 一行挂接）

### 2.1 配置

- `studio.nova_url`（环境变量 `STUDIO_NOVA_URL`，viper AutomaticEnv 映射），例如 `http://nova-studio:3000`。为空即关闭。
- 公开设置里增加 `studio_enabled`（= nova_url 非空），供前端决定是否显示入口。
- `server.trusted_proxies` 部署时加入共享网段；上线前确认一号机是否开启了「兼容模式信任原始转发头」（`ip.requestUsesLegacyForwardedIPTrust`），两种模式下 Nova 的 `X-Forwarded-For` 都要能得到正确 IP。

### 2.2 转发中间件 `internal/server/studio_proxy.go`

- 在 `router.go` 中于 `SecurityHeaders` 之后、前端 SPA 中间件之前 `r.Use(...)` 一行挂接（SPA 中间件不排除 `/_nova/`，必须先于它处理）。
- `httputil.ReverseProxy`（原生支持 WebSocket Upgrade），目标 `studio.nova_url`；未配置或连接失败返回 503 JSON。
- 路径与鉴权：

  | 路径 | 鉴权 | 注入 |
  |---|---|---|
  | `/_nova/*` | 无（静态资源） | — |
  | `/api/nova/ws` | 无 JWT；Nova 用一次性 ticket 校验（见 3.3） | `X-Forwarded-For` |
  | `/api/nova/*` 其余 | 复用 JWT 中间件（`NewJWTAuthMiddleware`），失败 401 | `X-Sub2API-User-Id`、`X-Sub2API-User-Role`、`X-Forwarded-For`、`X-Real-IP` |

- 一律删除客户端带来的 `X-Sub2API-*` 头再注入；删除 `Authorization` 头不转给 Nova（Nova 不需要 JWT）。
- 客户端 IP 用 `ip.GetClientIP(c)`。
- 响应头（仅这两类路径）：覆盖全局安全头 —— `X-Frame-Options: SAMEORIGIN`；CSP 专用策略：
  `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; connect-src 'self'; worker-src 'self' blob:; frame-src 'self' blob: data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'`。
  （Next 静态导出含内联脚本，nonce 方案不可用；connect-src 只允许同源，保证浏览器端也无法直连外部上游。具体值在 PoC 中按实际报错收敛。）
- 不经过网关的计费 / 限流中间件；保留 RequestLogger，但不记录 query（ticket）。

### 2.3 Nova 使用的 sub2api 用户接口（不新增接口）

Nova 前端以同源方式、携带父页传入的 sessionToken 直接调用：`GET /api/v1/auth/me`（余额、角色）、`GET /api/v1/keys`、`GET /api/v1/groups/available`、`POST /api/v1/keys`（一键创建）、`GET /api/v1/model-plaza`（用途分类与单价）。调网关 `GET /v1/models` 时用所选 API Key，绝不携带 sessionToken。

## 3. Nova（`qlqqs/nova-image-studio`，分支 `sub2api-custom`）

### 3.1 嵌入层（移植自 `sub2api-embed` 远端）

- 移植 `frontend/src/lib/embed/*`、`hooks/useEmbedRuntime.ts`、`nova-auth-fetch.ts` 及相关调用点到上游 main，`basePath`/`assetPrefix` = `/_nova`。
- `NOVA_EMBED_MODE=1`（服务端）/ `?embedded=1`（前端）开启嵌入模式：隐藏视频工作台与 BYOK 设置、锁定备份导出 Key、关闭 PWA service worker（避免缓存旧版本与作用域问题）。

### 3.2 固定网关

- 后端新增 `resolveGatewayBaseUrl()`：嵌入模式下**忽略**所有客户端提供的 baseUrl（任务 body、`x-nova-base-url`、插件 `defaultBaseUrl`），恒返回 `SUB2API_GATEWAY_URL`；删除嵌入版对官方地址的放行。`SUB2API_GATEWAY_URL` 为空时嵌入模式拒绝启动。
- 所有出站请求加 `X-Forwarded-For: <X-Forwarded-For from sub2api>`。

### 3.3 身份与隔离

- 中间层 `requireUser(req)`：嵌入模式下从 `X-Sub2API-User-Id` 读取用户（只接受 `NOVA_TRUSTED_PROXY_CIDRS` 配置的 TCP 来源 CIDR，默认仅 loopback，否则 403）；缺失则 401。
- SQLite 迁移：`tasks.user_id`（及二期 `plugin_media` / 插件任务）；索引 `(user_id, created_at)`。
- 读取类接口（任务详情、ack、图片、插件任务、素材）按 `user_id` 过滤，不匹配返回 404。
- WebSocket：`POST /api/nova/ws-ticket`（经 JWT）签发一次性 ticket（随机 32 字节、60 秒、绑定 user_id）；`/api/nova/ws?ticket=` 建连时校验，连接绑定 user_id；`subscribeTasks` 只接受本人 taskId；`subscribeQueue` 只推本人排队信息。
- 限额：嵌入模式下关闭 Nova 原有的按 IP / apiKeyHash 限流与单来源排队上限（全部经 sub2api 转发，IP 维度本就不可靠），不新增按用户的限额；保留全局 `NOVA_TASK_CONCURRENCY`。滥用控制交给网关（用户并发、RPM、Key 额度、余额）。
- 保留期：图片与任务按 `NOVA_RETENTION_DAYS`（默认 7）清理。

### 3.4 浏览器存储分区

- 所有 `localStorage` 键（`nova-*`、`theme`）与 localforage 实例名加 `u<userId>:` 前缀（统一走一个 `storageKey()` 帮助函数）。
- 收到 `revoke` 或 userId 变化：清空内存中的 Key，切换存储分区并重新加载状态。

### 3.5 父页契约（postMessage，同源，双向校验 `event.origin === location.origin`）

父页只负责会话，不再传 Key / 模型：
```ts
// 父 → Nova：iframe 加载后发送；sub2api 刷新 token 或切换主题时重发（revision 递增）
{ type: 'sub2api:nova-studio-session', revision, sessionToken, userId, role: 'admin'|'user', theme: 'light'|'dark' }
// 父 → Nova：退出登录 / 换账号
{ type: 'sub2api:nova-studio-revoke' }
// Nova → 父
{ type: 'nova:ready' }
{ type: 'nova:balance-changed' }            // 父页刷新控制台侧余额
{ type: 'nova:navigate', path: '/purchase' } // 「去充值」等跳转由父页路由完成
```
嵌入版原有的 `sub2api:nova-studio-config`（携带 Key）不再使用；`protocol-map.ts` 的协议映射保留，由 Nova 设置页根据分组平台推导。

### 3.5A 设置页

- 「模型与密钥」分区替换原「模型」分区（BYOK 模型注册表）：
  - 三行（生图 / 对话 / 视频）；Key 来自 `/api/v1/keys`，按 `group.platform` / `allow_image_generation` / 状态 / 额度 / 过期过滤与置灰。
  - 模型：用所选 Key 调 `/v1/models`，与 `/api/v1/model-plaza` 的计费模式（image / token / video）交叉得到用途分类与单价。
  - 协议推导：group.platform + 模型 → `openai_images` / `grok_images` / `gemini_generate_content` / `openai_responses` / `openai_chat_completions` / `anthropic_messages`。
  - 一键创建：`/api/v1/groups/available` 选分组 → `POST /api/v1/keys` → 自动选中。
  - 持久化：`storageKey('studio-prefs')` 只存 `{ image:{keyId,model}, text:{keyId,model} }`；Key 明文每次会话从 `/api/v1/keys` 读取，只在内存。
  - 生成时把 `apiKey` 随任务交给 Nova 后端（沿用其「只存内存」机制）。
- 删除「关于」分区；设置页底部放源码链接。
- 「插件」分区：`role === 'admin'` 才渲染；后端若有插件管理类接口，校验 `X-Sub2API-User-Role === 'admin'`，否则 403（只读的插件列表供视频工作台使用，二期）。
- 「备份」分区：导出时剔除所有 Key。
- 余额：工具栏右侧显示 `/api/v1/auth/me` 的余额，任务结束后刷新并发 `nova:balance-changed`。

### 3.5B 无缝呈现

- 嵌入模式隐藏 Nova 自身品牌顶栏，保留模式导航；根元素 `height: 100dvh`、背景透明或与控制台一致；只保留 Nova 内部滚动。
- 弹窗（设置、确认框）只覆盖 iframe 区域，属预期；遮罩颜色与控制台一致。

### 3.6 主题

- `frontend/src/app/globals.css` 的 CSS 变量换为 QxAPI 色板（主色 `#2340D0`）与线条风格（边框 1px、去大面积底色），深色变量同步；硬编码颜色按需替换。
- 去掉品牌、赞助、桌面端入口；页脚加「源码」链接（AGPL）。

### 3.7 镜像

- 薄镜像：宿主机构建 `frontend/out` 与后端依赖后 COPY 进 `node:20-alpine`（本机根分区紧张，不做多阶段）。CI 在该仓库构建 `ghcr.io/qlqqs/nova-image-studio:custom-latest`。
- 环境变量：`NOVA_EMBED_MODE=1`、`SUB2API_GATEWAY_URL=http://sub2api:8080`、`NOVA_CDP_ENABLED=false`、`PROMPT_GALLERY_MODE=1`、`NOVA_RETENTION_DAYS=7`、`NOVA_TRUSTED_PROXY_CIDRS=<共享网段 CIDR>`。

## 4. sub2api 前端（fork 自有）

- 路由 `/studio`（name `UserStudio`，需登录），视图 `src/themes/qx/studio/StudioView.vue`；QxShell 导航「创作工作台」放在「API 密钥」附近（`featureFlag` 取 `studio_enabled`），AppSidebar 同步（漂移测试）。
- 页面只有一个 iframe：`src="/_nova/?embedded=1"`、无边框、占满内容区、内容区自身不滚动，`allow="clipboard-write"`。
- 会话桥接：发送 `sub2api:nova-studio-session`；监听 auth store 的 token / 用户 / 主题变化重发；退出登录或切换账号发 revoke 并卸载 iframe。
- 处理 Nova 消息：`nova:balance-changed` → 刷新用户信息；`nova:navigate` → 路由跳转（只允许站内路径白名单）。
- i18n：只有导航文案（zh / en）。

## 5. 测试

- sub2api：转发中间件单测（未配置 503、JWT 必需、头剥离与注入、安全头覆盖、WS 路径不要求 JWT、其他路径安全头不变）。
- Nova：嵌入契约、固定网关（各种 baseUrl 输入都只打网关）、归属校验、ticket、存储分区单测。
- Nova 前端：设置页 Key 过滤 / 协议推导 / 持久化、插件分区按角色显示单测。
- sub2api 前端：会话桥接（重发、revoke、导航白名单）单测。
- e2e（本机）：新增 p7 `p7_studio.py`：两个用户经 `/api/nova/*` 跑生图（上游 `upstream-mock`），核对 usage_logs 金额与 IP、跨用户 404、伪造 baseUrl 无外连（upstream-mock 记录 + Nova 日志）；Playwright 截图 `/studio`。

## 6. 发布与回滚

- 开启：部署 nova-studio compose → sub2api 设置 `STUDIO_NOVA_URL` 与 trusted_proxies → 重启 sub2api。
- 回滚：清空 `STUDIO_NOVA_URL` 重启 sub2api（入口消失、转发 503），`docker compose down` nova-studio；sub2api 数据不受影响。

## 7. 风险

- 上游 Nova 迭代快：改动集中在 `lib/embed/*`、`backend/embed/*`（新建）与主题文件，定期 merge upstream。
- Next 静态导出的内联脚本迫使 `/_nova` 使用 `'unsafe-inline'`；靠 connect-src 'self' 与同源隔离限制影响面，且该策略只作用于 `/_nova`。
- 同源 iframe 中 Nova 页面可读 sub2api 的 `localStorage.auth_token`（同源特性）；Nova 是我们自己构建的代码，接受该信任关系；不引入任何第三方脚本。
