# 现存明细基线实现与验证（2026-10-26）

## 执行边界

用户已批准现存明细基线，并明确**停止请求、等待在途请求结束后重算**。交付流程保持：停请求 → 备份 → 预览/确认 → 核对三处统计 → 恢复请求。不建设自动停流量编排或在线重算，不提供撤销。

本轮仅本机实现/验证，无一号机写入、备份、重算、提交、推送或部署。并行工作台任务的文件/迁移259保留，不属于本任务交付范围；本机组合构建会加载它，正式提交仍须分清归属。

## 已实现

- usage批次可显式接受现存明细基线（默认不接受），支付批次不能携带此选项。
- 按筛选所涉及**完整日期**核对所有账号，包括未选账号；31日/整日2000条上限。
- 小时/日请求数及账号成本、total_cost、actual_cost核对；缺失非空桶、不匹配即失败，不覆盖。
- 原小时/日金额、来源行数/摘要、删除版本写入原有config_snapshot，逐记录原/新值沿用审计；不回拨全局覆盖日期。
- Execute在单事务内复核、更新成本及小时/日汇总；余额、客户实际扣费、分润和运行时配额不变。
- 已发现并修正：旧日期修正标记导致恢复请求后汇总冻结；关系锁升级导致清理等待环。迁移260仅追加标记版本，旧迁移未改。版本失效策略保守，不声称逐请求完整来源追踪。
- UI复用既有居中弹窗；加入基线选项及停请求/备份/核对/恢复提示，不新增操作向导或自动运维机制。

## 验证证据

- 后端calculator/service/handler专项：`baseline-unit.log`通过。
- 仓库相关集成：`/tmp/finance-baseline-generation-tests.log`，`Finance|Recalculation|DashboardAggregation`通过（15.413s）。包括原值审计/事务回滚/恢复请求正常统计/清理保护；删除版本真实触发器、无命中DELETE及TRUNCATE回滚。
- 前端完整lint/typecheck/test：`frontend-baseline-verified.log`，436 suites /3438 tests通过；停流量文案后另做locale检查。
- 本机非空历史专项：**14/14组通过**；详见`baseline-e2e.md`。真实持久API、SQL合成fixture，不冒称真实网关/支付回调。
- 初次全后端验证遇到并行工作台ProvideAuthService/Wire生成中间态和已有内存阈值用例失败；随后cmd/server、内存用例单独重跑及go build通过，未修改无关测试阈值。最终整套重跑结果补记如下。

所有临时日志在`/tmp/finance-cost-recalculation/`。未运行本机未安装的golangci-lint；不可声称CI通过。

## 最终验证补记

- `backend-baseline-final.log`：完整 `go test -tags unit ./...` **exit0通过**（service217.135s）。`server-baseline-rerun.log`：cmd/server专项和 `go build ./...` 通过。
- `local-build-baseline-final.log`：停流量操作提示已纳入最终组合本机构建，仅本机app重建并重启，`/health`通过。迁移258校验和仍为136c67dd…，260前向迁移已应用于本机；无生产变化。
- 代码复审确认恢复请求后的聚合过滤分支均已更新、基线EXCLUSIVE关系锁解决已识别的锁升级环；没有继续扩展在线重算设计。
- 限制：删除版本是保守全局失效；无变化批次不会刷新已失效的修复标记，不宣称“零差额重验能恢复已失效桶”。本次只保证已接受首次基线及正常恢复流量，不实现新的清理后在线账本修复机制。
- `git diff --check`通过，前端锁文件无改动。无提交/推送/部署授权，保留未提交状态及并行工作台任务。
- 最终UI抽查47/47：1440/390、浅/深色，8张截图；停流量→等待在途→备份→核对→恢复文案、默认不勾选/支付隐藏重置、键盘和滚动通过，无JS/HTTP错误或业务写入。证据`/tmp/finance-cost-recalculation/ui/baseline/`。

## 用户授权提交后的独立快照检查

仅暂存财务代码，排除并行注册工作台任务和迁移259；共享repository/service Wire provider按财务内容单独暂存，wire_gen由隔离暂存快照重新生成，不覆盖工作区其他任务的生成内容。此finance-only快照的cmd/server、service、admin handler、repository、migrations相关单测以及`go build ./...`通过；独立testcontainers集成`Finance|Recalculation|DashboardAggregation`通过（9.342s）。日志`/tmp/finance-commit-check/`。提交授权不包含推送、部署或生产历史重算。
