# 技术设计（已批准，本地实施中）

## 边界

- 以 PRD 编号清单为唯一功能范围；此文不额外授权功能或用户流程。
- 保留现有链路：Agent → 本地 Magpie 网关 → sub2api。账号会话与模型 Key 分离。
- Magpie 保持独立 fork；sub2api 只按确实缺少的接口做必要改动，不复制桌面代码到服务端仓库。

## 源码复用与裁剪切入点

| 区域 | 基线来源 | 策略 |
| --- | --- | --- |
| 桌面/导航/表单 | internal/gui/assets/index.html、app.js、app.css、theme.js | 沿用原版；最小增加本站登录及必要状态 |
| Agent 配置 | internal/agent/、internal/edit/ | 保留原适配器及配置安全写入，不重写成直连模式 |
| 供应商 | internal/provider/ | 接入本站；禁止其他供应商配置和订阅发现；旧会话迁移不能依赖登录原供应商 |
| 会话迁移 | internal/sessions/codex_provider.go:186、:378 | 保留文件/SQLite Provider 更新、备份、活动会话保护；补足当前版本验证 |
| 内部功能组织 | internal/pluginrt/pluginrt.go:38、plugins/all/all.go | 按依赖裁剪，不能删整个 plugins 目录 |
| 保留内部功能 | plugins/sessions/sessions.go:14、plugins/library/library.go:13、plugins/redact/redact.go:21 | 原版将其注册为内置插件；去掉外部插件模块仍须保留其工作路径 |
| 统计 | plugins/usage/、internal/gui/assets/ 中现有用量展示 | 映射服务端数据；不新增会话标识关联；错误按原版样式展示 |
| 更新/发行 | internal/update/update.go、.github/workflows/release.yml | 独立更新 feed 与构建，不沿用上游 secrets/发布目标 |

## 登录：客户端内邮箱/密码直登（已确认）

- 复用 `POST /api/v1/auth/login`，请求 `{email, password}`；继续使用既有 `/api/v1/auth/refresh` 与 `/api/v1/auth/logout`，不新建桌面授权后端。
- 使用原版表单/弹窗控件呈现输入、提交中和失败状态。用户明确没有验证码和 2FA，本期不添加相应 UI/SDK，也不实现浏览器授权。
- 正常响应按已有 AuthResponse 解析 access_token、refresh_token、expires_in 和 user，按账号 API 的响应封装解包；不能仅凭 HTTP 200 认定成功。验证挑战或缺少有效令牌时保持未登录。
- 仅通过可信本站地址发送登录凭据；正式站点 HTTPS，本地测试仅 loopback。密码只用于当前提交，不保存到配置/备份。
- 凭据存储沿安全边界实现；账号 JWT 不传给模型请求。刷新失败按原版错误样式提示重新登录；退出清理账号会话，不擅自删除用户会话文件或共享 Key。
- 服务端 auth_handler.go:241 仍保留已有验证码/2FA 分支。部署前提变化时客户端报错，不在本任务中关闭安全检查。

## 站点依据

用户于 2026-10-25 明确默认内置域名为 `api.qlqqx.com`。生产默认 origin 统一为 `https://api.qlqqx.com`：登录/资料/统计走 `/api/v1/*`，模型流量走分组对应协议路径。账号 JWT 与模型 Key 仍严格分离，不能因域名相同混用凭据。仅本机测试允许显式 loopback override，不开放任意远端地址或重定向。

## 仓库可见性（已确认）

- 二开仓库为 **`qlqqs/qxapi-client`**，直接公开；名称和可见性已确认，尚未创建远端或推送代码。
- 保留上游许可与来源说明；发布前检查配置、测试数据、截图、构建产物及拟发布历史，不包含凭据或用户会话数据。
- 最终摘要拟定反馈使用该仓库 Issues，更新使用该仓库 Releases 的发布资产/元数据；不新增更新服务器，不继续使用原版 feed。实施前由最终方案确认，首次公开上传另获同意。
  - 2026-10-30 更新：更新源改为 sub2api 托管，不再使用 GitHub Releases，见 `10-30-qxapi-update-feed`。

## 品牌（已确认）

- 客户端显示名称为 **QxAPI**，用于窗口标题、产品名称及关于页面等原有品牌位置。
- 该选择不授权重设计视觉风格，也不代表沿用原版应用标识、更新源或图标商标；内部标识及发行地址在实施/发行契约中明确。

## 视觉约束

唯一设计来源是 Magpie 当前原版。复用 app.css 的 --bg/--card/--fg/--line/--accent/--font 等变量及已有控件；保留 theme.js 的主题和字号处理。实现前获取原版截图，新增页面用相同视口对比，不进行四套新风格探索，不引入另一套组件库。

## 风险与待核查

1. 仅隐藏 Plugins 页面不等于禁用第三方执行，需遍历 CLI/API/恢复备份等入口。
2. 删除 provider discovery 不能连带删除 Agent 检测、Codex 历史 Provider 读取及迁移。
3. 已发现视频 API Key 兼容路径、独立的 imagemcp 与内置搜索模块，见 research.md；实现时验证本站模型类型/权限/协议，不静默删除或自建替代品。
4. 同时更改 rollout 与 SQLite 可能发生部分失败；先以隔离 fixtures 验证原行为，不对用户真实会话试改。
5. 远端统计与本地上下文分析不是同一数据域；前者切源，后者按保留项继续，不绑定消费。
6. 多平台签名、WebView 与更新兼容须在目标平台实测；不能把 Linux 通过声明为 Windows/macOS 已验收。

## 文档状态

已记录确认的范围、核心依赖证据及发行默认方案。运行兼容性按 implement.md 验证；最终方案已获用户批准，尚未完成全部验收。
