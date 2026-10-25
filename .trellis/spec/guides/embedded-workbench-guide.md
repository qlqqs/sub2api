# Embedded Workbench Guide（嵌入第三方工作台）

> 适用：把一个独立项目（fork）以同源 iframe 嵌进控制台，例如创作工作台 Nova（`/studio` ↔ `/_nova`、`studio_proxy.go`）、对话工作台 Open WebUI（`/chat` ↔ `/_chat`、`chat_proxy.go`）。
> 新增同类工作台时先读完本页，再看这两个现成实现与各自 fork 的 `docs/sub2api-embed.md`。

## 1. 共同约定

| 方面 | 约定 |
|---|---|
| 仓库 | 第三方代码只在 fork（`qlqqs/<项目>`，分支 `sub2api-custom`）；sub2api 只引用镜像。fork 改动保持为可变基的补丁集：新增文件集中在 `embed/` 一类目录，上游文件只做带标记注释的挂接（`sub2api-embed`）。 |
| 路由 | 控制台页面 `/<名称>`（`frontend/src/themes/qx/<名称>/`）＋ 同源前缀 `/_<名称>/*` 由 sub2api 反代；前缀不得与 `/api/v1`、`/v1`、`/v1beta`、SPA 路由冲突。sub2api 自有接口放在前缀下的保留段（如 `/_chat/__sub2api/*`）。 |
| 开关 | 一个配置项（`STUDIO_NOVA_URL`、`CHAT_WEBUI_URL`），为空 → 入口隐藏、前缀返回 503；上游不可达 → 503，不回落到 SPA。 |
| 身份 | 浏览器只带 sub2api 的会话凭据（Nova：Bearer；Open WebUI：`Path=/_chat` 的 HttpOnly Cookie，因为 `<img>` / WebSocket 带不了头）。反代先删除客户端的全部 `X-Sub2API-*`、`X-Forwarded-*`、`X-Real-IP`、`Forwarded`，再注入用户 ID / 角色 / 真实 IP。fork 只信任来自固定内部网段（compose 固定 IP）的头。头值必须是 ASCII（非 ASCII 先百分号编码，uvicorn 的 WebSocket 握手会因中文头崩溃）。 |
| 计费 | 用户在工作台里选自己的 Key（只存 Key ID）；反代按 ID 校验归属 / 状态后注入明文；浏览器永远看不到明文。fork 所有模型请求只发往网关并带用户真实 IP。 |
| 网络 | fork 容器不映射宿主机端口，只接入与 sub2api 共享的专用网络；不需要外网的工作台用 `internal: true`，需要出网的功能走白名单出站代理。 |
| 安全头 | 前缀下删除上游 CSP / X-Frame-Options，统一下发 `X-Frame-Options: SAMEORIGIN` 与严格 CSP（`script-src 'self'`、`connect-src 'self'`、`frame-ancestors 'self'`）；需要执行模型生成内容的预览页用单独路径 + `sandbox="allow-scripts"`（不带 `allow-same-origin`）+ 单独 CSP（`connect-src 'none'`）。 |
| 父子通信 | postMessage 双向校验 `origin === location.origin` 且 `source` 为对方窗口；父 → 子：会话 / 主题 / 语言 / 撤销；子 → 父：ready、余额变化、白名单内导航、会话失效。 |
| 本地存储 | fork 前端的 localStorage / sessionStorage 一律加自己的前缀（`owui:`、`nova:` 等），退出 / 换账号时父页按前缀清空，并清掉 fork 自己的 HttpOnly Cookie（由 sub2api 的 DELETE session 下发）。 |
| 品牌 | 许可证允许时由部署环境变量隐藏 fork 品牌；源码与公开镜像默认保留原品牌，版权与 LICENSE 链接始终保留。 |

## 2. 新增工作台的检查清单

- [ ] 写一份双方接口约定（参照 `.trellis/tasks/archive/**/10-24-chat-openwebui/contract.md`），前后端 / 两个仓库的子任务都只按它实现。
- [ ] sub2api：`<名称>_proxy.go` + `router.go` 一行挂接；单测覆盖未配置 503、无凭据 401、伪造头被剥、他人 Key 不注入、CSP 只作用于前缀、其他路径安全头不变。
- [ ] fork：信任边界中间件（不可信对端剥头 + 非静态 403）、出口只到网关、空 Key 报错不回退、屏蔽表（接口 + 界面 + 环境变量三处都关）、管理员没有读他人数据的特权。
- [ ] 本机环境 `~/workspace/sub2api-local-<名称>`（独立 compose 项目、命名卷、不映射端口）+ e2e 阶段（扣费与直调一致、usage IP、跨用户隔离、伪造头、停机 503 与恢复）+ Playwright 界面脚本（截图给用户确认）。
- [ ] `AGENTS.md` 本机环境章节补一节；一号机部署前查看资源并征得同意。

## 3. 生图结果与模型发现的跨层约定（2026-10-25）

- 网关 URL→base64 回填**会保留原 `url` 并新增 `b64_json`**，不是二选一。嵌入工作台生成和编辑图片时，只要存在非空 `b64_json` 就优先解码；只有 URL 时仍执行固定网关 origin 检查，不因展示失败而开放外站下载。
- fork 实现：`routers/images.py::image_generations/image_edits` 共同使用 `embed/images.py`；`resolve_model(request, configured)` 从 `gateway_models(request, image=True)` 取得生图 Key 的完整模型列表。列表不可用或无生图模型时，在发起付费 POST 之前返回错误。不要通过“试着生一张”探测模型能力。
- 对话下拉过滤只是展示策略：`visible_chat_models(models)` 不得裁剪 per-Key 原始缓存。生图模型虽不出现在聊天列表，仍应可被生图路径选中；不得复用其他用户/其他 Key 的可用模型。
- 测试不能仅覆盖“纯 URL”与“纯 base64”：必须覆盖两字段共存、URL-only 外站拒绝、生成/编辑/工具调用、所选模型不可用、跨 Key 缓存隔离。本机 CHAT-09b/09c 同时断言计费一张、返回本站文件且图片字节可读取。
- 此次问题根因：跨层隐含假设（把 URL 与 base64 当互斥字段）与 mock 缺失组合形态。放宽出网不会解决契约问题，反而破坏安全边界。
- 浏览器测试收集失败信息后也必须设置非零退出码；不得将“脚本正常结束”当成界面验收通过。
