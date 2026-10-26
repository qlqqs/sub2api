# 执行计划（已批准，本地实施中）

## 规划收敛

- [x] 记录用户确认的 01–47 清单与新增 UI 原版风格约束。
- [x] 创建 PRD、设计草案和执行计划；任务保持 planning。
- [x] 确认客户端内邮箱/密码直登，无验证码/2FA，不做浏览器授权。
- [x] 确认客户端显示名称为 QxAPI。
- [x] 确认二开仓库直接公开，不先建私有仓库。
- [x] 确认公开仓库名称为 qlqqs/qxapi-client；站点地址已有源码文档依据。
- [x] 在最终摘要确认使用仓库 Issues/Release 作为反馈及更新来源，不另建服务。
- [x] 核查并记录核心内部插件依赖、媒体/搜索路径及远程统计基础字段，见 research.md；实际 DTO 映射和全入口回归在实施阶段验证。
- [x] 完成 PRD 收敛复读，确认清单编号/约束/源码锚点与验收映射保留。
- [x] 提交最终规划摘要，用户已明确批准。
- [x] 获得批准后已运行 task.py start；必要时按独立交付物拆子任务，不默认调用子代理。

## 实施顺序（仅计划）

1. 建立独立 Magpie 工作区和上游基线，读取该仓库 AGENTS 与开发规范，保护两仓库已有改动；制作原版 UI 截图及回归基线。
2. 绘制删除入口 → handler/CLI → 后台任务 → 内部依赖映射，按清单删除；保留会话/Library/网关/规则脱敏/工具依赖。
3. 按已确认方案接入本站登录、必要分组/Key 和模型配置；新增 UI 复用原控件及主题。
4. 替换统计数据源；移除原版会话费用展示及不再需要的消费关联，保留上下文分析。
5. 回归会话 Provider 迁移及恢复；使用隔离 HOME/CODEX_HOME/SQLite fixtures，不接触用户真实凭据和会话。
6. 替换品牌/深链/数据目录/更新 feed，去掉推广及统计上报；保留许可。公开发布前检查待上传文件与历史不含凭据、真实会话或其他隐私数据。
7. 全链路本地验证与跨平台验收；记录未测项。用户同意后再提交、推送、部署。

## 检查计划

### Magpie

以 fork 原版 Makefile 与 internal/gui/tests/README.md 为准，进入独立工作区运行，不在 sub2api 根目录执行：

- `make test`：原版 go vet / go test（Linux 涉及 production,gtk3 及系统依赖）。
- `make test-ui`：原版 Node/Playwright 浏览器回归；先确认环境与依赖。
- `make build`：本机桌面构建；目标平台发布签名单独验证。
- 增补对应登录、远端统计、删除入口、会话迁移及浅/深色 UI 回归。只适配被删除功能的既有测试，不跳过保留能力回归。

### sub2api（仅实际涉及修改时）

- 后端：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`，以及 `go build ./...`。
- 前端：`npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run`；检查锁文件无意外修改。
- 按根 AGENTS 通过 `/home/qlqq/workspace/sub2api-local/build.sh` 和本地 compose 验证；只用本地测试凭据，不连接一号机，不清理其他容器。

## 关键验收

- 不靠 CSS 隐藏删除能力；CLI、深链、Web API、备份恢复不能重新启用删除项。
- 直接登录使用无验证码/2FA 的本地测试站点；覆盖密码错误、网络失败、刷新/退出及意外验证挑战，不改服务端安全规则。
- 首次配置及切供应商后会话能继续使用；配置/数据库部分失败不报成功。
- 用量按远端口径展示，无本地会话费用，无凭据/原版匿名上报请求泄露。
- 保留搜索/媒体/Library/WSL/本地备份/CLI/TUI/Web 功能，不以删依赖名义顺带裁掉。
- 新 UI 与原版同屏截图对比：字体、色彩、控件、密度、主题、缩放、窄窗口。

## 当前执行记录

已建立独立工作区并实现初版账号客户端与入口约束；账号客户端单测和 race 测试通过，nogui 本地构建通过。整体裁剪、远程统计 UI、多平台与端到端验收尚未完成。未迁移真实会话、创建远端、提交、推送或部署。

### 本轮最小变更边界

- 补齐账号连接路径的实际错误：首次登录因删除不存在的 provider 失败；刷新过期仍显示已登录；重选 Key 丢失模型偏好；新建 Key 后列表未刷新。
- 责任层：`internal/qxapi` 账号状态、`internal/provider` 本站配置读写、`internal/gui/qxapi.go` 请求验证与连接、`assets/qxapi.js` 原版控件中的交互。新增对应测试及子系统文档。
- 本轮优先完成并验证这个纵向切片，再推进裁剪与远程统计；不修改会话迁移实现、不改服务器安全机制，不触碰真实凭据/Agent 配置。

### 默认域名确认

- 2026-10-25 用户指定默认内置域名 `api.qlqqx.com`；账号接口与模型接口统一 origin 为 `https://api.qlqqx.com`。更改 `internal/product` 的集中常量与对应 allowlist 测试，不向生产发登录/模型测试请求。

### 域名调整验证与续接点

- 集中默认 origin 已改为 `https://api.qlqqx.com`，`SiteAddress` 与 `ModelAPI` 共用该值；账号/模型凭据仍隔离。
- 已通过隔离 HOME 的定向 Go 测试：`internal/product`、`internal/qxapi`、`internal/provider`、`internal/gui`（BuiltInOrigin / ModelAPITestOverride / QxAPI / Login / Expired / Late / Refresh / Keys / OnlyTrusted / Redirect / Logout）。未访问生产环境。
- 中断前已修复首次登录被不存在 provider 阻止、刷新失效的账号状态、退出本地清理、新建 Key 后按 ID 重用、重连保留模型偏好，并补 handler/provider 测试。新增文件尚未提交。
- 浏览器 Chromium headless-shell 可运行；完整 Chromium 缺 libcups，WebKit 未安装。本次账号 UI 测试第一轮到退出步骤因测试选择器匹配三按钮失败，已收窄至非 qx-tools 行后重跑。不能据此宣称全部 UI/平台通过。
- 整体任务仍未完成：远端统计 UI 映射（注意服务端 `/usage/stats` 无参数默认最近约 7 天，绝非累计）、所有裁剪入口审计、品牌/发行/目录身份隔离、全量回归及真实本机链路验收仍待继续。不得把当前工作区描述为已完成客户端。

### 用户指定设计模型及图标（2026-10-25）

- 用户指定图标已原样保存到独立 fork `docs/brand/qxapi-logo-source.png`；不重新生成/重画替代品牌。
- 用户明确前端由 Opus 5.5 High 设计。通过 Paseo 模型列表确认可用后，启动 `claude/claude-opus-5-5`，`thinkingOptionId=high`，agent `b255db56-6c9e-4749-8869-a694ae0c9cc9`，工作区 `wks_bc7cc20c81064df6`。
- 原 UI worker 已停止并交回，避免并发写前端。Opus 拥有 assets/、前端 tests/、DESIGN.md；后端和 native icon/更新实现者不在其写入范围。
- 原版风格、不扩功能、已确认裁剪清单仍有效，未授权提交/发布。
- 新增远程 stats/requests/CSV 后端、概览和请求页面实现已落盘（未提交）；中英浅深4项远程统计浏览器测试通过，账号5语言浅深10项通过。tray panel 统计仍需 Opus 接到新远端 schema。CLI bridge 正由 runtime worker 编写，主代理需在 gui/qxapiRoutes 绑定 gateway.QxAPIUsage。
