# 本轮上游范围分流记录

冻结目标 `f178e5ae32659e8901644a9f2378a0439ffa36ae`；共 35 个 first-parent 审查单元。

**“已审查游标”表示已作范围处置，并非全部技术审核通过。** deferred/scope_triage_only 下次不要重复做范围筛选；决定实施时必须继续技术审查。用户明确排除的模型项仅移交，不代表模型审查已完成。

| 上游单元 | 决定 | 审查深度 | 理由 |
|---|---|---|---|
| `4942b2afc` | selected | selected_patch_and_dependencies_reviewed | M4 queued/metadata 安全重试；保留所有有效输出不可重放边界 |
| `d744c2201` | deferred | scope_triage_only | 账号筛选持久化 UX 改善；非首批批准范围，后续界面维护批次审查 |
| `9a1680042` | selected | selected_patch_and_dependencies_reviewed | M1 Go/follow-redirects 与 CI 版本安全链 |
| `4c7e9b5b6` | deferred | scope_triage_only | Vertex 配额是账号能力及跨层功能变更，需独立需求与计费验收 |
| `98f83b726` | deferred | scope_triage_only | 改变 Gemini/混合分组生图准入且含迁移，需核对既有分组行为与空库/升级库 |
| `b75123051` | deferred | technical_review_deferred | 已技术审查 94c7d6b+4355236 依赖；计费敏感，暂缓到独立 exactly-once/断流验收批次 |
| `b6c366380` | deferred | scope_triage_only | Antigravity 请求元数据和调度性能优化，非首批必要依赖 |
| `b3e66f79a` | deferred | scope_triage_only | 模型映射签名性能优化，非首批必要依赖；不顺带重构 |
| `a34f37397` | selected_partial | selected_patch_and_dependencies_reviewed | M2 仅摘取快照 COW，不采用余下 scheduler memo 性能优化 |
| `624874b31` | deferred | scope_triage_only | 账号配额累计 SQL 优化，触及账务写路径，独立并发/数据库回归后再选 |
| `5e45fa482` | deferred | scope_triage_only | DeepSeek 图片负载兼容修复，非本次批准范围，后续协议维护批次 |
| `888c60573` | deferred | scope_triage_only | Gemini 工具 schema 规范化，非本次批准范围，需工具兼容测试 |
| `aa48a8bbd` | selected | selected_patch_and_dependencies_reviewed | M3 Cloudflare 1010 分类；保持真实 403 处罚 |
| `7da09cab4` | skipped | scope_triage_only | 赞助资料，不需要引入定制维护补丁 |
| `21cc92693` | selected | selected_patch_and_dependencies_reviewed | M1 根 Docker Go 版本；保留 custom Node pin |
| `dba770d56` | delegated_out_of_scope | scope_triage_only | Grok Fast 价卡/型号支持由模型任务处理 |
| `c5b811284` | deferred | technical_review_deferred | 已技术审查但暂缓：含出站身份重写及 Retry-After 延迟策略；需拆分后单独批准 |
| `828626804` | selected | selected_patch_and_dependencies_reviewed | M1 DOMPurify 安全版本及锁文件 |
| `ac1b8f127` | skipped | scope_triage_only | 赞助资料，不需要引入定制维护补丁 |
| `5c185c5c5` | skipped | scope_triage_only | 未整体升级 v0.2.18，不能仅改 VERSION 冒充完整上游版本 |
| `37067e8e3` | deferred | scope_triage_only | 管理员受限 Key 92 文件/权限和迁移变更，需适配定制支付财务与管理员端点，单独任务 |
| `3f1c8f36c` | skipped | scope_triage_only | 赞助资料，不需要引入定制维护补丁 |
| `ba81c7fd7` | deferred | scope_triage_only | 多图按张计费含开关/迁移，需 Nova 预估扣费分润独立验收，非本批 |
| `b332f169f` | delegated_out_of_scope | scope_triage_only | GPT-6.1 Terra 上新明确移交模型任务；这里不审查适配实现 |
| `15701ba18` | delegated_out_of_scope | scope_triage_only | Grok 模型目录/能力审计移交模型任务 |
| `0232cb713` | deferred | scope_triage_only | 新增短冷却策略，本次仅范围分流，需按各供应商频率/配额语义专项审查 |
| `3bd166bab` | deferred | scope_triage_only | 版本字符串 UTF-16 兼容修复，本次仅范围分流，后续审核 Unicode 边界与测试 |
| `bb021d330` | deferred | scope_triage_only | Gemini 文件图片计数，影响计费口径，留后续图片计费批次 |
| `3f1ae7ee7` | delegated_out_of_scope | scope_triage_only | DeepSeek 型号价格时点调整移交模型定价任务 |
| `98bcee7a8` | deferred | prior_pr_review_final_merge_pending | 已看过 PR 小补丁且文本适配通过；现已正式合并，待独立广场/工作台价格验收，不扩大首批 |
| `8e231ff5d` | deferred | scope_triage_only | DeepSeek cache-hit usage 识别，影响账单，后续独立用量回归 |
| `af4462eba` | deferred | scope_triage_only | DeepSeek developer 角色转换，后续协议兼容批次，不扩大首批 |
| `2c9d9760d` | deferred | scope_triage_only | 分页 offset 溢出修复，新增于本轮 fetch；需专项输入边界/端点回归后再选 |
| `449b65a2e` | deferred | scope_triage_only | 图片上传分段大小校验，需独立附件/上传边界测试 |
| `f178e5ae3` | deferred | scope_triage_only | Antigravity 非流式畸形工具调用错误处理，需跨协议错误/计费回归 |

本轮没有自动引入新增范围，没有因 main 快进而合并所有业务变更；最终 merge diff 指纹与覆盖子提交保存在长期账本。
