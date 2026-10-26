# 实施顺序与验证

## Gate

- [x] 用户已确认 UI 并授权创建正式任务。
- [x] PRD 收敛：复用配置、同步修正、独立确认、审计（不提供撤销）、安全边界与验收已落盘。
- [x] 技术设计与代码证据已完成。
- [x] 用户批准最终方案：“开始执行任务”。已 task.py start（取消撤销，保留执行并发检查及不回补运行时配额边界）。

## 顺序

1. 建严格成本复算输入/结果、规则快照和纯函数测试，保护在线计算行为。核对所有原成本字段/单位与历史缺失分支。
2. 新增审计/任务 migration、原生 SQL repository、锁与输入指纹策略。先测幂等、预览失效、并发聚合、新增请求、清理、部分历史桶；确定增量同步设计不会覆盖新/旧聚合。
3. 实现预览、执行、失败续跑服务和管理员 routes/DTO/审计/Wire 生命周期。补完整单测、集成测试；生成代码按仓库要求更新。
4. 将四区集中配置正式接入 FinanceSettingsTab；提取渠道成本规则编辑共用件，复用账号/费用组件和既有 API；补原入口一致性、局部保存/失败测试。
5. 实现三步重算、批次列表/详情、进度恢复、不可撤销确认、执行冲突/失效/跳过状态。补 i18n、键盘、浅深色和窄屏验证。
6. 本机完整质量检查、构建、隔离环境用量与支付链路 e2e；保存前后 SQL 对账、截图与测试报告。费用、余额、分润和运行时配额逐项核对不变。
7. 更新规格、任务和交付说明；未经同意不 commit/push/deploy，不对一号机执行任何重算。

## 预计修改边界

- 新增 backend finance_recalculation service/repository/handler/tests 与 migrations；routes/handlers/Wire/start-stop 最小接线。
- account_stats_pricing/account_model_cost 中复用计算与快照读取边界，不改计费公式。
- dashboard aggregation/cache 与相关测试：仅同步/并发一致性所必需的改动。
- frontend finance 组件/API/types/i18n/tests；ChannelsView 成本编辑块最小提取并回归；不重新设计 shared BaseDialog/DataTable。
- 仓库外 sub2api-local/e2e 新增仅 loopback 用例，禁止远端目标，不改其他容器。

## 验证命令

```bash
cd backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags integration ./internal/repository/ -run 'Finance|Recalculation|DashboardAggregation'
go build ./...
cd ../frontend
npx -y pnpm@9 run lint:check
npx -y pnpm@9 run typecheck
npx -y pnpm@9 run test:run
cd /home/qlqq/workspace/sub2api-local
./build.sh && docker compose up -d
cd e2e
./run_all.sh --fast
# 新增财务重算专项脚本：数据建立→预览→确认→同步SQL核对→不可撤销提示/API边界→并发/失败续跑
```

不使用全局 pnpm，不改锁文件，不做全局 Docker 清理。golangci-lint 本机未装，明确交由获批推送后的 CI，不能声称本机 lint 已通过。

## 回滚点

规则纯函数先绿；存储/并发测试不过不接 UI；本机实际对账不过不请求部署。审计迁移为前向扩展，禁止删除旧 migration。已提交批次不提供撤销；误配置修正规则后重新生成批次。代码回滚不恢复历史数据，灾难恢复另行评估并征得用户授权。


## 2026-10-26 用户批准：现存明细首次基线

追加实施：后端基线审计/对账并发保护；UI 显式复选框/确认文案；默认拒绝/历史成功/不一致/漂移/事务回滚集成测试；本机非空 usage 历史重算端到端验证。现有全套验证结果仅为追加前基线，新改动必须重新验证。


## 2026-10-26 执行前提收敛：停流量重算

用户明确：重算会在停止所有请求的情况下进行，不会有新增记录，要求避免复杂化。

正式操作流程仅为：停止请求入口并等待在途请求结束 → 数据库备份 → 现存明细预览/确认重算 → 核对财务、账号、看板 → 恢复请求。停流量由操作人员完成；本页面不会自行停服务或创建备份。

不支持/不再扩展在线并发重算算法、自动停流量编排、全库历史补证或撤销。保留已实现的整日对账、旧值审计和单事务原子执行；锁只是误操作防护。恢复请求后的正常汇总不能被旧日期标记冻结，因此保留已验证的小范围标记版本修复（前向迁移260；不改257/258或其他任务的259）。

此授权仍不包含提交、推送、部署或修改一号机账目。
