# 技术设计

## 隔离与输入
从实施时核对后的 custom HEAD 建立新候选 worktree，不复用旧批次候选。六个源SHA固定，merge单元采用相对第一父的完整diff，不整体merge main、不自动cherry-pick提交。上游若新增，只记录观察，不扩展本批。
本地main镜像已在独立worktree；确认该目录干净且未被其他工作占用后仅ff到本批目标，不能确认则保留并报告，不强制操作；不推送origin/main。

## 最小改动边界
- R1：backend/go.mod、go.sum，采用上游x/*依赖组合，不改Go/Node工具链或其他构建入口。
- R2：frontend/package.json、pnpm-lock.yaml、vite.config.ts、SubscriptionPlanCard.spec.ts；必要的国际化回归测试。保留Node 24.22.1和自定义CI；适配已有套餐测试，不覆盖定制断言。保留runtime-only别名，移除过时JIT define，不放宽CSP。若应用代码出现实际v11不兼容，只修相关调用并记录。
- R3：openai_gateway_passthrough.go、openai_gateway_response_handling.go及上游非流式测试；必要时补既有定制响应回归。保留原始下游stream意图，上游OAuth仍按要求流式请求；聚合后明确JSON响应头，非completed终止提取response，沿用usage/output处理。
- R4：channel_service.go、ChannelsView.vue、中英文channels文案及配套测试。只合并web_search_emulation平台维度，不对features_config做通用深合并；保留显式空对象/null清空语义。
- R5：openai_codex_models_service.go及对应缓存测试。在singleflight闭包内再次读缓存；补测试证明已刷新时不再fetch，不用睡眠制造竞态。
- R6：admin_user.go、admin_service_role_test.go。仅替换三类业务错误，保留赠送/线下充值及删除清理实现。

## 验证与回退
先R1及R3–R6，再R2，分别检查，最后六项一起构建和验收。此分批仅是验证顺序，不自动创建提交。候选没有数据库迁移；本机环境不reset数据，不做全局清理。
记录源diff与候选指纹。进入custom时，仅移入批准文件；原有无关修改及本批任务文档保留。如果HEAD或同文件改变，重新整合和验证，不覆盖。未提交候选可保留作回退/比较现场，不reset主工作区。

## 台账
父任务长期state/decisions沿用，只追加事件；旧批次发布事实不能覆写为本批。为本批11个新增单元记录6项选入、3项Gemini延期/移交、MCP延期、VERSION跳过。前端更新为selected_partial，明确CI Node修改不采用（定制pin已满足），其余适配保留证据。所有前序有决定才推进范围审查游标，测试或实施状态另记。提交前target_commits为空，最多记applied_in_custom_worktree。
