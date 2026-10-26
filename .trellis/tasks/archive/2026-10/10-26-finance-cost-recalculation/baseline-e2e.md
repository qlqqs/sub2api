# Retained baseline 本机专项 E2E

## 状态

2026-10-26：主代理确认本机健康重建且迁移 260 已应用后执行，**14/14 测试组通过，进程 exit 0**。`python3 -m py_compile` 通过。

最新运行目录：`/tmp/finance-cost-recalculation/baseline-e2e-20261026-133030`，其中 `run.log`、`results.jsonl` 保存完整结果。没有运行生产请求或重算、没有重置环境、没有构建/重启容器。

## 文件与隔离

- 唯一执行脚本：仓库外 `/home/qlqq/workspace/sub2api-local/e2e/p11_finance_recalculation.py`。
- 复用 `lib.py` 的 loopback HTTP、固定 `sub2api-local-postgres` 和本机凭据；无生产地址、SSH 或远端凭据。
- 每次选择过去 7–84 天内没有明细、小时/日汇总、修复标记的独立日期，并再次在建 fixture 事务中检查不占用已有数据。
- 建立专用 disabled 用户、Key、两个上游账号；三个明确的 synthetic usage（目标 2 条、同日未选账号 1 条），显式种入匹配小时/日聚合和活跃用户索引。
- 不重置环境、不覆盖旧聚合、不修改 coverage frontier、不禁用触发器、不清除现有数据；保留 fixture 与批次便于核对。
- 此脚本验证真实运行应用的持久重算 API，但用量由 SQL 合成，不声称经过真实上游/网关计费；支付 fixture 同样不是支付回调链路。

## 新增测试（全部通过）

| 编号 | 断言 |
|---|---|
| FRC-07 | 非空旧明细不 opt-in，2 条均明确 coverage skip；确认后仍不改成本 |
| FRC-08 | opt-in 后预览 2 条 changed、2→4；配置审计包含完整日 3 条、SHA256、旧小时/日金额与操作者；预览无写成本 |
| FRC-09 | 确认至 completed；财务 breakdown、账号 stats API=4，小时/日 account_cost 合计=4.5（保留未选账号 .5）；修复标记存在；原旧聚合审计仍保留；coverage 未移动；保护字段逐行不变 |
| FRC-10 | 重复确认、已完成任务 retry-sync 幂等返回 completed、新零差额预览执行不双加；首次 applied_at 不变 |
| FRC-11 | 旧小时聚合和明细不匹配时拒绝基线，不改任何成本 |
| FRC-12 | 预览后账号倍率改变，确认 409，禁止写入旧预览 |
| FRC-13 | 预览后同日**未选账号**明细变化，确认 409，全日摘要保护有效 |
| FRC-14 | payment_fee 携带 accept_retained_baseline=true 返回 400 |

原 FRC-01–06 合成支付、空请求生命周期、鉴权/no undo、渠道 CAS 测试保留。

保护检查包含目标和同日 peer 的除两项成本快照以外全部 usage 字段、专用 users/api_keys/accounts 全行、该用户分润流水和订单、除 account_cost/computed_at 外小时/日聚合字段。HTTP dashboard/trend 本身不公开账号成本，因此只验证其请求/客户费用不变；账号成本同步直接核对真实小时/日表，不把 trend.cost 当作账号成本。

## 执行证据

- 成功历史批次：**41**；日期 **2026-10-18**。目标账号 2 条成本从 **2→4**；全日含未选账号 3 条成本从 **2.5→4.5**。
- 财务 breakdown 和账号 stats API 实际读取修正值；看板小时/日 SQL 合计修正值一致，客户趋势请求数与实际扣费不变。
- 审计保留 Version1、Asia/Shanghai、旧汇总、3 条全日来源摘要、删除代际、actor/确认/完成时间；标记代际核对通过，coverage frontier 未动。
- 拒绝用例使用独立日期：2026-10-17（聚合不匹配）、2026-10-12（倍率改动）、2026-10-11（未选账号同日明细改动）。这是顺序修改后确认的拒绝检查，**未启动并发压测或并发任务**。
- 首次试跑在 FRC-09 成功后因测试错误地期待 completed retry-sync 返回 409 中止；核对现有 repository 合约是幂等 no-op 200，只修脚本断言后用新的空日期完整重跑通过。首次失败日志保留在 `baseline-e2e-20261026-132928/`，不计为通过的完整轮次。没有为测试修改业务代码。

## 复现命令

须协调本机稳定窗口再执行，输出独立目录以免混淆此前运行结果：

```bash
cd /home/qlqq/workspace/sub2api-local/e2e
E2E_RUN_DIR=/tmp/finance-cost-recalculation/baseline-e2e-<timestamp> python3 p11_finance_recalculation.py
```

该套件不替代 repository 并发锁、超限/边界、异常提交恢复的集成测试，也不证明部署或生产基线执行完成。
