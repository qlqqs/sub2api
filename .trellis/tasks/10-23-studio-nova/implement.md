# 执行计划：创作工作台（Nova Studio 嵌入）

按阶段推进，每个阶段末尾有检查点；涉及两个仓库：
- **S** = sub2api（`~/workspace/sub2api`，分支 `sub2api-custom`）
- **N** = Nova（`~/workspace/nova-image-studio`，新分支 `sub2api-custom`，基于 `upstream/main`）
- **L** = 本机环境（`~/workspace/sub2api-local`、新建 `~/workspace/sub2api-local-nova`，均在仓库外）

## 阶段 0：PoC（先验证可行性，再做完整功能）

1. N：建分支，移植嵌入层（`sub2api-embed` 的 `lib/embed/*`、`useEmbedRuntime`、`nova-auth-fetch`、basePath `/_nova`），本地 `npm` 构建通过（Nova 自己用 npm + package-lock，不涉及 sub2api 的 pnpm 规则）。
2. S：`studio_proxy.go` 最小版（转发 + JWT + 头注入 + 安全头覆盖 + 503），`router.go` 一行挂接，单测。
3. L：`sub2api-local` compose 增加共享网络与 `STUDIO_NOVA_URL`；`sub2api-local-nova` 的 compose、build.sh、薄镜像。
4. S：最小 `/studio` 页（无缝 iframe + 会话消息）；N：设置页先用最简单的 Key 选择打通。
5. 检查点：Playwright 打开 `/studio` 出图（upstream-mock 生图）；核对 usage_logs 金额与客户端 IP；控制台无 CSP / X-Frame-Options 报错。**结果汇报给用户再继续。**

## 阶段 1：Nova 安全改造（N）

1. 固定网关：`resolveGatewayBaseUrl()`，删除官方地址放行，`x-nova-base-url`、插件 `defaultBaseUrl`、`/api/nova/proxy/*` 全部改走网关；出站带 `X-Forwarded-For`。
2. 身份：`requireUser`，只接受共享网段来源；SQLite 迁移 `tasks.user_id`；所有读取接口按用户过滤（404）。
3. WebSocket ticket；订阅与队列状态按用户。
4. 嵌入模式关闭 Nova 原有 IP / Key 限流与单来源排队上限；保留期 7 天清理（环境变量）。
5. 浏览器存储分区（`storageKey()`），revoke / 换账号处理。
6. 关闭：视频工作台、BYOK、PWA service worker（嵌入模式）、CDP。
7. 单测：`cd backend && npm test`、`cd frontend && npm test`（以仓库实际脚本为准）。

## 阶段 2：设置页与父页面

1. N：设置页「模型与密钥」——Key 过滤、模型分类、协议推导、单价、一键创建、持久化、首次进入自动打开。
2. N：去掉「关于」、源码链接、「插件」仅管理员（前端 + 后端角色校验）、备份剔除 Key、工具栏余额与 `nova:balance-changed`、错误中文提示与「去充值」（`nova:navigate`）。
3. S 前端：路由 `/studio`、导航入口（`studio_enabled`）、AppSidebar 同步、会话桥接、消息处理。
4. S 后端：公开设置 `studio_enabled`；转发注入 `X-Sub2API-User-Role`。
5. S 前端检查：`npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run`；`git status` 无锁文件改动。

## 阶段 3：UI 主题（N）

1. `globals.css` 换 QxAPI 变量（浅 / 深），去品牌与赞助；嵌入模式隐藏品牌顶栏、单层滚动，做到与控制台无缝。
2. 截图给用户看（按「小改动不跑全量」的约定，迭代期只改 + 截图），确认后再进入阶段 4。

## 阶段 4：全量验证（L）

1. S 后端：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`、`go build ./...`。
2. `./build.sh && docker compose up -d`；`sub2api-local-nova` 构建并启动。
3. e2e：`./run_all.sh`（p0–p6 不回归）+ 新增 `p7_studio.py`：
   - 两用户生图，金额 / IP 核对；
   - 跨用户任务、图片、WS 订阅 404；
   - 伪造 baseUrl / `x-nova-base-url`（官方地址、内网地址）无外连；
   - 未配置 `STUDIO_NOVA_URL` 时入口隐藏、503；
   - Playwright `/studio` 截图，换账号后无上一用户数据。
4. 更新 `e2e/CASES.md`。

## 阶段 5：提交与部署（需用户逐项同意）

1. 用户同意后：S 提交并推送（CI 构建 `custom-latest`）；N 推送分支并配置该仓库 CI 构建 `ghcr.io/qlqqs/nova-image-studio:custom-latest`。
2. 用户同意后部署一号机：确认内存 / 磁盘与转发 IP 模式 → 建 `/root/nova-studio` → 更新 `/root/sub2api-custom`（网络、`STUDIO_NOVA_URL`、trusted_proxies）→ 冒烟测试。
3. 回滚预案：清空 `STUDIO_NOVA_URL` 重启 sub2api；`docker compose down` nova-studio。

## 规范更新

- 完成后把「fork 自有反代中间件挂接方式」「同源嵌入页面的安全头覆盖」写入 `.trellis/spec/`（后端 / 前端各一处），并更新 `AGENTS.md` 的本机环境说明（新增 `sub2api-local-nova`）。


## 2026-10-24 恢复执行进度

只完成 Nova 后端安全基础（阶段 1 部分内容），sub2api 与共用本机环境仍不动。
新增安全测试 9 项、后端全量 124 项通过；全量 lint 有 6 项经 HEAD 核实的原有问题。
前端、外部结果图片安全下载、PoC 网关联调未完成，当前不可部署。
详见 `research/progress-2026-10-24.md`；后续从 Nova 前端会话桥/设置页接入继续，不重复建任务或启动子代理。
