# 选型调研：生图 / 视频工作台（2026-10-23）

## 结论

二开 tianjiangqiji/nova-image-studio（fork `qlqqs/nova-image-studio`）。它本身没有账号与计费、按「Base URL + Key」调用上游，
正好留出接入 sub2api 账号、Key 与网关计费的位置。参考 nuotian4-spec/nova-image-studio（同为 AGPL 的 Sub2API 嵌入版）的嵌入层。

## 项目侧现状（sub2api）

- 生图：`/v1/images/{generations,edits}`（OpenAI / Grok 分组）、异步 `/v1/images/*/async` + `/v1/images/tasks/:id`（需对象存储）、批量 `/v1/images/batches*`、Gemini `/v1beta/models/*:generateContent`。
- 视频：Grok `/v1/videos*`、Seedance `/api/v3/contents/generations/tasks`；计费在轮询到完成时才发生。无 Sora / Veo / Kling。
- 计费：图片按张（1K/2K/4K 档），视频按秒；分组倍率、预扣与结算已具备。
- 网关只认 API Key；用户 Key 列表 `/api/v1/keys` 返回明文与分组（含 `platform`、`allow_image_generation`）。
- `/batch-image` 页面已是「浏览器选 Key 直调网关」的模式。
- `utils/embedded-url.ts` 会把用户 JWT 以 `?token=` 拼进被嵌入 URL，不能用于嵌入第三方页面。
- 全局安全头：`X-Frame-Options: DENY`、CSP `frame-ancestors 'none'`、`script-src` 带 nonce（`middleware/security_headers.go`、`config.go:35`）。

## 候选对比（摘要）

| 项目 | 结论 | 主要原因 |
|---|---|---|
| Nova Studio（tianjiangqiji） | 选用 | AGPL-3.0；Next 静态导出 + Node + SQLite；无账号、BYOK；功能丰富；有现成 Sub2API 嵌入版可参考 |
| NovaNova Studio（Swayingleaves） | 不选 | GPL-3.0；Java + Next 服务端 + PG + Redis + Nginx 5 容器；自有账号 / 积分 / 管理员统一渠道 Key，与「sub2api 是唯一身份与账单」冲突；存储仅 COS/OSS/Kodo；有未鉴权开放代理（`web/src/app/webdav-proxy/route.ts`）与 SSRF 回读（`/media/registerRemoteMedia` + `downloadMedia`） |
| CookSleep/gpt_image_playground | 不选（可作过渡） | MIT，React，只有生图，Key 与历史在浏览器 |
| LobeHub / Open WebUI / Jaaz / Cherry Studio | 不选 | 许可证限制（衍生需商业授权 / 品牌条款 / 禁多人部署 / AGPL 桌面端），自带账号体系 |
| chatgpt-web-midjourney-proxy / Nexior | 不选 | Vue，但接口绑定 new-api 或自家平台 |
| new-api / one-hub | 参考 | 均无原生生图视频工作台，用外部聊天链接或 iframe |

## Nova Studio 代码要点（本地克隆核实）

- 后端 `backend/server.js`：`PORT` 默认 3000；SQLite `tasks` / `task_items` 无归属字段；`GET /api/nova/tasks/:id`、`/api/nova/images/:taskId/:n` 不鉴权；WebSocket 按 taskId 订阅不鉴权；限流按 IP 与 apiKey hash。
- API Key 只存进程内存（`apiKeys.set(taskId, …)`），不入库。
- CDP 默认关闭（`NOVA_CDP_ENABLED`），仅连本机回环调试浏览器。
- 网页复刻在浏览器内运行，预览 iframe `sandbox="allow-scripts"`（不透明源）。
- 提示词广场：只读 `backend/prompts.json`，`PROMPT_GALLERY_MODE` 1/2/3。
- 前端：Tailwind 4 + shadcn，主题集中在 `frontend/src/app/globals.css` CSS 变量；无 i18n；localStorage 键 `nova-*`，localforage 库名 `nova-image`。

## 嵌入版（nuotian4-spec）要点

- 基于 v3.3.0，比上游多 16 个提交、缺上游 11 个提交（插件运行时、Agent、桌面端），只能移植不能合并。
- 父页契约：`sub2api:nova-studio-config` / `sub2api:nova-studio-revoke`；`basePath` `/_nova`；API 仍在 `/api/nova/*`。
- `frontend/src/lib/embed/*`：`mode.ts`、`protocol-map.ts`、`parent-registry.ts`、`nova-auth-fetch.ts`（只给 `/api/nova/*` 附带面板 JWT，`/v1` 走 API Key）。
- `backend/rewrite-parent-gateway-base-url.js`：非官方地址改写为 `SUB2API_GATEWAY_URL`，但**官方地址放行**——需删除放行。
- WebSocket 未带任何身份；后端未校验会话。

## 外部资料

- OpenAI 弃用：Sora 2 / Videos API 2026-09-24 移除；dall-e 2026-05-12 移除；gpt-image-1 2026-10-23 停用 —— https://developers.openai.com/api/docs/deprecations
- Nova Studio：https://github.com/tianjiangqiji/nova-image-studio ；嵌入版：https://github.com/nuotian4-spec/nova-image-studio
- NovaNova Studio：https://github.com/Swayingleaves/novanova-studio
