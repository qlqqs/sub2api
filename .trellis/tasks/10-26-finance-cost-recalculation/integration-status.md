# 集成状态（2026-10-26，进行中，未完成交付）

## 已接通
- Finance settings 四区、BaseDialog、共享成本规则编辑、未保存离开确认；旧渠道入口共用同一编辑器。
- 渠道成本条件 GET/PUT，版本冲突不回退无条件保存。旧 schema 不存的 Fast/Flex、image_input_price、区间倍率、非空 time_pricing 明确拒绝，绝不静默截断。
- 持久预览/确认/执行/同步任务与审计，无撤销；租约、幂等、创建者校验、原值与配置指纹。
- SQL 直接读取成本白名单配置；执行持有配置表锁与价格版本读锁直到事务提交/回滚。锁顺序经过 FK 删除/TRUNCATE/原入口修改集成验证。
- Wire 重新生成，启动/关闭、admin routes、history slot、locale 已接通。
- 账号成本沿用原数值口径，支付手续费 CNY；不同单位不混加。

## 未解决的产品边界（不能宣称历史成本功能已全部完成）
旧 `usage_dashboard_*` 汇总不含请求成员清单，也没有足以证明旧明细完整性的历史删除水位。computed_at、最大 ID、行数相等都不足以证明选中请求已计入旧汇总：并发晚提交和已清理明细可能使净差额重复或残留账目被覆盖。

当前验证性实现使用迁移 258 开始建立的**前瞻完整性边界**：可证明完整的日期才允许按源明细替换 account_cost；更早记录在预览中明确 `unproven_aggregate_coverage` 跳过，保留原成本。它并不能完成用户最初想要的“现在就修正升级前历史请求”。本机首次边界为 2026-10-27 20:00+08，安全整天起点为 2026-10-28 00:00+08，说明限制实质存在，不能当作已完成。

- 不得通过人为回拨完整性边界、默认所有旧桶完整、清空旧汇总来刷绿测试。
- 不得因为本限制冻结正常看板更新；普通聚合保留既有行为。
- 是否允许缩小首次历史覆盖范围、或需要额外的完整性核验/历史补证机制，应另行确认。任务继续 in_progress；未获批提交/部署。

## 测试记录（不是完整验收）
- 前端全量 lint/typecheck 通过；436 suites / 3432 tests 通过；最终接线/字段拒绝与 locale 有补充专项测试。
- 后端全量 `go test -tags unit ./...` 及 `go build ./...` 通过（修正新增 cleanup 参数测试后）。
- 相关 repository 集成 `-run 'Finance|Recalculation|DashboardAggregation'` 通过。
- 配置锁/作用域/删除/截断/价格读锁的独立测试与 race 测试通过。
- 本机 build.sh 全栈构建通过，仅更新 `sub2api-local` 应用；其他容器/一号机未操作。
- 本机 p11（仓库外）7 组通过：支付预览无写入、幂等、确认完成、余额/订单非成本字段/分润不变、配置漂移409、无撤销、渠道成本条件写入；使用明确的合成支付夹具，不冒充真实支付全链路。
- 上游历史成本真实 happy path E2E **尚未通过/尚不能覆盖旧历史**。快速全链路回归（包含原财务 FIN）178/178 通过。视觉矩阵 1440/390、浅/深色、四区与弹窗 36 张/75 检查通过；发现并修复 Escape 触发关闭二次确认及 390px 定价卡裁剪，补充复测 47/47 通过（20 张刷新截图），共 56 张截图，无 JS/HTTP 错误和配置写入。

日志在 `/tmp/finance-cost-recalculation/`；仓库外专项脚本 `/home/qlqq/workspace/sub2api-local/e2e/p11_finance_recalculation.py`。

## 最后集成修复
- `coverage_unknown` worker 错误补入 store.Fail 白名单，确保原子回滚后持久化 failed/applying 并清租约，不重复 claim；真实 PostgreSQL 回归通过，明细成本和应用标记不变、不可误走 retry-sync。
- 最新后端全量 unit + build 通过；无依赖/锁文件改动、无业务提交/推送/线上操作。

## 等待用户决策（不是发布批准）
原始需求要修正升级前历史，上述前瞻边界尚不满足。默认保留旧汇总，不按剩余明细强制覆盖。
下一轮应询问：是否允许以现存明细重建旧日期**成本汇总**，明确已清理明细原来保留在汇总中的成本将不再计入？未确认前不得实施此口径，也不得将当前限制宣称为完整历史重算。

最终验证日志：backend-verified.log、frontend-verified.log、integration-verified.log、local-build-verified.log、e2e-recalculation-verified.log、e2e-fast.log。UI 报告 `/tmp/finance-cost-recalculation/ui/README.md`。

## 2026-10-26 20:51 一号机只读核验更新
用户补充实际调用很少并授权查看。现存 176 请求（两天/5账号），小时/日汇总均为 176 请求、记录成本 1.339320464，逐桶无差额，无管理清理任务。详见 production-readonly-audit.md。建议以本次现存明细建立一次性明确基线，避免不必要的旧账复杂化；仍需正式实现基线确认/事务对账，而不是直接伪造覆盖水位。本次未做任何线上写入或部署。
