# 代码证据（2026-10-26）

- `frontend/src/components/admin/finance/FinanceSettingsTab.vue`：只编辑固定成本账号，需增量扩展；`AdminFinanceView.vue` 控制报表 tabs/刷新，设置区须拆开报表日期工具条。
- `AccountModelCostEditor.vue`、`FinanceExpenseDialog.vue`、`components/common/{DataTable,BaseDialog,Select,SearchInput,DateRangePicker}.vue`：可直接复用。
- `views/admin/ChannelsView.vue:457-599,1076,1400`：渠道统计规则编辑和转换仍内嵌，需最小提取共用；定价项可复用 `components/admin/channel/PricingEntryCard.vue`，不能复制仅按张简化编辑器。
- `service/account_stats_pricing.go:8,263`：自定义规则 → 应用客户倍率前成本 → 模型文件 → 回退；应用函数默认 requestCount=1，历史重算不能不经依据校验直接复用入口。
- `service/account_model_cost.go`：精确匹配优先、最长通配前缀；内存快照有失效/旧值兜底。预览必须可靠读规则，不用加载失败后的陈旧回退冒充当前配置。
- `service/usage_log.go:132-210`：保留 service tier、思考等级、tokens、图片数、图片输出 tokens、时间和成本快照；缺少历史上游长上下文策略等信息时不可猜原规则。
- `repository/finance_repo.go:83,140`：支付订单费率 NULL 回退实例费率；请求成本 COALESCE(account_stats_cost,total_cost) × COALESCE(account_rate_multiplier,1)。固定账号另按财务口径排除。
- `service/finance.go`：当前 FinanceService 本身没有设计文档所述的 60s 缓存；必须以代码为准，不能凭父任务旧设计新增无用缓存。
- `repository/dashboard_aggregation_repo.go:111`：已有 RecomputeRange，会删除桶再建小时/日及活跃用户聚合；不能对明细已经部分清理的旧桶盲目全量重建。
- 同文件 `:233` 与数据库规范：清理按行/分区，分组汇总有锁与失效机制；新重算必须检查与聚合/保留期任务的并发顺序。
- `service/dashboard_service.go:25,270-325`：Redis dashboard 缓存及异步写回，单次 Delete 不足以防止在途旧查询重新写回。
- `repository/usage_billing_repo.go:205`：AccountQuotaCost 在计费写路径增量更新配额；历史修正不调用该路径。
- `service/payment_config_service.go:166`：实例通道费率是可选 PATCH 字段，可仅更新费率、不带凭据。
- `server/routes/finance.go`：既有管理员财务 routes；新接口在同组，继续复用外层鉴权/审计。
