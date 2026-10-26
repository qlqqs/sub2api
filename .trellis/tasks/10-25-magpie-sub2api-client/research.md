# 源码核查记录

基线：Magpie `8d7d46e3bdb02258ef155b1d77412d1e9581b9d6`；sub2api 当前工作区。以下为静态源码证据，不代表运行测试通过。

## 内部模块与外部插件分离

- `plugins/all/all.go` 注册内置模块；`internal/pluginrt/pluginrt.go:38` 维护 Requires/Provides。
- 保留 `plugins/sessions`、`library`、`gateway`、`usage`、`redact`、`search` 的必要实现。
- `plugins/tools/tools.go` 是**第三方插件工具**的 MCP 暴露，不能把它误认为所有 MCP/生图工具的唯一入口。该外部插件能力随插件模块删除；Library 的 MCP 管理和 `internal/imagemcp` 不属于本次删除项。
- `plugins/search/search.go` 的依赖为 settings/gateway，可调用可搜索模型或搜索 API；属于用户确认保留的搜索功能，不因删除一般模型供应商添加而默默删掉专用搜索配置。
- `plugins/redact/redact.go` 为网关注册规则脱敏器。删本地隐私模型不等于去掉规则脱敏，不能破坏其 fail-closed 行为。
- `plugins/groups`、`rotation`、`routing`、`warmups`、`otel` 等对应删除产品能力；仍需实现阶段遍历入口和其他包调用点，不能只删注册。

## 视频不是只能使用 Grok 订阅

- `internal/gateway/video.go:70–121` 存在 customVideomakers/videoBase：普通 Provider 可通过 Chat 或 Responses base URL 调用 OpenAI 形状的视频接口。
- 视频模型来自目录中的 kind=video 或原版模型种类设置，不仅按名称猜测；接入本站时复用现有设置/元数据路径。
- `internal/imagemcp/imagemcp.go:808–856` 调用本地 `/v1/videos`、状态轮询和内容下载；不要求第三方插件工具运行时。
- sub2api `backend/internal/server/routes/gateway.go` 已有视频创建/查询/内容接口及平台限制。保留功能不代表每个用户分组都有视频权限；本地用可控 fixtures 验证三步链路及不支持错误。

## 远程统计

- `backend/internal/handler/usage_handler.go:408` 的 Stats 使用当前用户过滤器，返回过滤后的汇总，并去除上游管理字段。
- `backend/internal/service/usage_service.go` 的统计类型与 `frontend/src/api/usage.ts` 展示了输入/输出、缓存创建/读取、总 Token 与 actual_cost 相关字段；实施以 endpoint 实际 DTO 为准，不仅依赖历史类型。
- 使用 `/api/v1/usage/stats`、必要的 dashboard/trend 等既有用户接口，不请求管理员接口，不新增本地会话标识上传。
- 展示实际扣费，不能混用 total_cost（标价）或上游账号成本。远程故障显示原版错误/空状态，不回退本地估价。
- 本地上下文分析与消费统计拆开：上下文仍保留，费用按远端范围展示；用户明确不需要会话费用关联。

## 发行与 UI

- `internal/update/update.go` 及 `.github/workflows/release.yml` 原本依赖上游 feed/独立发布仓库；必须改为 QxAPI 的发布链路，不能复用上游 secrets。
- 最终方案拟使用公开仓库 `qlqqs/qxapi-client` 的 Issues/Release，不额外建设反馈系统或更新服务器；具体资产命名/清单兼容在独立 fork 中实现。
- `internal/gui/assets/app.css:1`、`theme.js:1`、`index.html:24` 是视觉和交互依据，禁止移植网站 Vue 主题。
- 原版 `internal/gui/tests/README.md` 有 Chromium/WebKit、浅深主题及窄窗口 fixtures。沿用测试方式，不读取用户真实配置/密钥。

## 实施验证而非已完成事项

- 原版构建、测试、跨平台安装、真实本站调用、图表字段映射及 Codex 迁移均尚未实际运行。
- 必须用隔离 HOME、测试数据库和本地 sub2api 环境验证；技术兼容问题不能成为额外删功能或扩大功能范围的授权。
