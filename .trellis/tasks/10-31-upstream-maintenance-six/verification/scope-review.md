# 本批范围审查

目标 d851a38477597157e78450d30af2d8504764d085；11个主线单元均有范围决定，不等于全部技术验收。

- `b2e7c6094`：selected — 管理员保护只改业务错误映射，用户选定
- `0a2a29b32`：selected — x/net及配套x/*安全版本，用户选定
- `8b3f33f28`：selected — 保留渠道搜索平台维度并补齐配置UI，用户选定
- `bf02c1a6e`：delegated_out_of_scope — 模型列表/识别混合变更，移交模型适配批次，未做能力专项审查
- `7311b865f`：deferred — Gemini同账号重试配置不在用户六项清单；待独立重试语义验收
- `184193f01`：deferred — Gemini绑定缓存池重试不在用户六项清单；待缓存/池专项验收
- `dece26153`：selected — singleflight内重检缓存，增加确定性测试
- `a9240d501`：skipped — 仅选择维护修复，不更新VERSION冒充完整上游版本
- `a3298b46e`：selected — OAuth非流式JSON及终止事件处理，保留定制响应与usage
- `7386bc6ae`：deferred — 管理MCP跨OAuth/迁移/余额权限，需独立适配赠送与线下充值字段
- `d851a3847`：selected_partial — 选择前端安全依赖/构建/测试部分；保留Node24.22.1定制pin，不采用三个workflow的Node版本替换

旧批次暂缓事项不自动重开；不继承旧批次发布授权。
