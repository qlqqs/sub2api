# BE：后端任务书（codex gpt-6-astra high）

- 工作目录：`/home/qlqq/workspace/sub2api/backend`，另外负责更新 `docs/AGENT_PROFIT_SHARING_CN.md`。
- 开始前，按 `implement.md` 第 2 节「共同规则」读文档。
- 需求以 `prd.md` 为准，实现以 `design.md` 为准，接口以 `contract.md` 为准。三者冲突时，先停下来问主代理。

## 现状速览（读代码的入口）

**分润结算**
- `internal/service/affiliate_consumption_settle.go`
- `internal/service/affiliate_distribution.go`：`planAgentShares`（:55）、`planInviteShare`、`distributeRewards`（:147）
- `internal/service/affiliate_first_recharge.go`：`ApplyFirstRechargeRewards`

**节点与快照**
- `internal/repository/affiliate_repo.go`：`GetAffiliateNodes`（:388）、`BindInviter`（:114）
- `internal/service/affiliate_owner.go`：`resolveOwnerSnapshot`

**等级**
- `internal/service/affiliate_agent_level.go`：`AdminSetAgentLevel`（:162）、`AppointAgentLevel`（:220）、`EvaluateAgentLevel`（:77）
- `internal/repository/affiliate_repo_agent_level.go`：`PromoteAgentLevelAuto`（:35）、`SetAgentLevelManual`（:59）

**团队页**
- `internal/repository/affiliate_team_repo.go`：`scopeSQL`、`subAgentSetSQL`
- `internal/service/affiliate_team.go`：`resolveViewer` 及 Overview / ListCustomers / ListAgents / ListRequests
- `internal/service/affiliate_team_admin.go`：管理端 viewer 的写法可参考

**财务**
- `internal/service/finance.go`：`financeReplays`（:246）、`FinanceLedger`、`FinanceExpense`
- `internal/service/finance_reports.go`：`financeSummary`
- `internal/repository/finance_repo.go`：`Load`（:56）、ledger 查询（:99）

**路由**
- 管理端 `internal/server/routes/admin.go`：`registerAffiliateRoutes`（:841）
- 用户端 `internal/server/routes/user.go`：`/aff` 一组（:36 起）

**其他**
- `GET /user/aff` 由 `internal/service/affiliate_detail.go` 的 `GetAffiliateDetail` 处理。
- 管理端代理列表是 `internal/handler/admin/affiliate_handler.go` 的 `ListUsers`。
- 批量设档位是 `BatchSetAgentLevel`，返回 `{affected}`。

---

## A. 迁移

新增 `migrations/<下一个空号>_affiliate_partners.sql`（当前为 264），内容照抄 `design.md` §2，每张表和每列都加 COMMENT，风格参考 248 和 255。

## B. 写路径

1. **设为合伙人、预览**（design §3.1，contract 1.2 / 1.3）
   - 同一事务内写 partners、rates，并改 `user_affiliates` 的档位并锁定，最后清缓存。
2. **挂靠、预览**（design §3.2，contract 1.6 / 1.7）
   - 预览里的 `current_region` 用 §C 的解析函数算。
3. **自动挂靠**（design §3.3）
   - 在 `SetAgentLevelManual`（level=1）和 `PromoteAgentLevelAuto`（target=1）成功之后执行，与档位变更放在同一事务。
   - `AdminSetAgentLevel` 返回自动挂靠的列表，`BatchSetAgentLevel` 和 `UpdateUserSettings` 的响应都带上 `auto_attached`（contract 2.2）。
4. **拦截**（design §3.4）
   - 改合伙人档位返回 409 `PARTNER_LEVEL_FIXED`。
   - 合伙人任命或取消代理返回 403 `PARTNER_CANNOT_APPOINT`。
   - 展示层同步：合伙人的 `GetAffiliateDetail.Appoint` 为 nil，邀请列表的 `appointable` 为 false（contract 3.1）。
   - 设为合伙人的预览接口带上 `current_region`（contract 1.2）。
5. **分润不发给合伙人**（design §3.5）
   - `AffiliateNode` 增加 `Partner`，`GetAffiliateNodes` 用 EXISTS 带出这个字段。
   - 新增 `nodeRewardable`，替换以下 3 处奖励发放判断：
     - `planAgentShares` 的最终过滤
     - `planInviteShare`
     - 首充固定奖励
   - **不要动 `Active`**：绑定解析依赖它，合伙人在绑定时必须仍是有效的 1 级。
6. 错误统一用现有 `infraerrors` 的写法，code 用契约里的名字，message 用中文。

## C. 区域归属解析（纯函数，design §4）

- 新文件 `internal/service/affiliate_partner_members.go`，实现 `ResolvePartnerMembers`。
- 规则：
  - 自己是锚点时，直接用自己。
  - 否则按「归属代理 → 上级一级」的顺序找第一个锚点。
  - 计入起点取「锚点起点」和「绑定时间」中较晚的一个；`agent_owner_bound_at` 为空时用 `users.created_at`。
- 单测覆盖 PRD 列出的全部拓扑：
  - 合伙人直属客户
  - 客户的被邀请人（继承快照）
  - 合伙人名下的分销代理，及其客户（upline = 合伙人）
  - 挂靠的渠道经理本人及其客户
  - 渠道经理名下的分销代理，及其客户
  - 区域外的用户
  - 绑定晚于挂靠时，起点取绑定时间

## D. 取数（design §5.1）

- `PartnerRepository.LoadMonths`：在只读 REPEATABLE READ 事务里，与财务 Load 共用一个事务。
- 给 `FinanceRepository.Load` 加一个跳过 usage_logs 成本扫描的选项，避免重复扫描。财务页自己的调用保持不变。
- `FinanceLedger` 增加 `SourceUserID`，`finance_repo.go:99` 的 SQL 读出 `source_user_id`。
- 成本与消耗用 `unnest(user_ids, start_ats)` 精确到起点时刻。成本公式、模型口径与财务页 breakdown 一致：
  - 全部计费类型都计成本；
  - 固定成本账号按 0 计。
- 起点当天的消耗：分别取全天合计，以及 `created_at >= start_at` 的部分。
- 分润：只取起点之后的 accrue 流水，类型限定为 `agent_direct`、`agent_diff`、`invite_percent`、`invite_fixed`（NULL 按 `invite_percent`）。
- 区域专项支出：取 `partner_user_id = P` 且未删除的支出。

## E. 利润计算与月份序列（design §5.2 / §5.3）

- 新文件 `internal/service/affiliate_partner_profit.go`，实现纯函数 `ComputePartnerMonth`。
- 输出合计、lines（line 0 表示合伙人直属团队）、`cost_by_group`、`cost_by_model`、warnings。
- 金额用 `decimal` 计算；`share_amount` 四舍五入到 2 位，接口统一输出字符串。
- 月份序列：
  - 已确认的月份读快照，用它的 `loss_after` 接续下一个月；
  - 一次请求只 Load 一次，范围是第一个未确认月到当月。
- 单测：
  - 起点当天按比例折算；
  - 退款当天收入为负；
  - 亏损链：连续亏损、部分抵扣、全部抵扣；
  - 比例按月生效；
  - 四舍五入；
  - 调整项和专项支出；
  - 没有成员时全部为 0。

## F. 结算单操作（design §6，contract 1.8–1.13）

- 确认：事务内 `SELECT ... FOR UPDATE` 锁 `affiliate_partners` 这一行，校验顺序和月份范围，计算后写入快照。快照里的 `detail` 就是明细接口的返回结构。
- 标记已付、调整项、改比例、导出 CSV（UTF-8 BOM），按契约实现。
- 集成测试（testcontainers）：
  - 并发确认同一个月只能成功一次；
  - 确认后改 usage_logs，读到的结算单不变。

## G. 接口与注入

- 管理端：`/admin/affiliates/partners` 一组（contract §1），放在 `registerAffiliateRoutes` 里。现有审计中间件已覆盖整个 /admin，需要确认写操作都进了审计日志。
- 用户端：
  - `/user/aff/partner` 一组（contract §3.2 / §3.3），挂 `panelRateLimiter.Heavy()`；
  - 非合伙人返回 403 `PARTNER_FORBIDDEN`；
  - 返利功能开关的处理和团队页一致。
- 新的 service、repository、handler 按项目现有的 wire 方式注册。检查 `wire_gen.go` 是否需要重新生成。
- `FinanceService` 通过 `PartnerShareProvider` 接口拿分成，不能形成循环依赖（design §7）。

## H. 财务联动（design §7，contract 2.3）

- 支出增加 `partner_user_id` 的输入输出，以及 `partner_region_name`；校验合伙人存在。全站汇总里支出照旧只计一次。
- Summary：
  - 月粒度时，series 加 `partner_share`、`net_profit`、`partner_share_estimated`；
  - totals 加 `partner_share`、`net_profit`、`partner_share_estimated`、`partner_share_scope`；
  - 没有合伙人时全部为 "0"。
- 导出：月粒度增加两列。
- **回归**：现有 `finance_*_test.go` 和 e2e p10 必须不改断言就能通过；只有新增字段可以补断言。

## I. 团队消费页的区域视角（design §8，contract 3.3）

- 区域范围条件：
  - 将 `scopeSQL` 一般化为接受「锚点集合 S」，或者新增 `regionScopeSQL`；
  - 成员 = `owner ∈ S OR upline ∈ S OR user ∈ 挂靠渠道经理`。
- 新增 `overview`、`customers`（带 `owner` 和 `line_agent`，不返回 earnings）、`lines`，以及 `customers/:user_id/requests`（客户必须在区域内）。
- 逐级查看 `team/agents/:agent_id/*`：
  - 校验该代理挂靠在本合伙人名下，然后直接调用现有代理端的 service 函数，`viewerID = agent_id`；
  - 脱敏和隐私规则沿用现有实现。
- 单测或集成测试：区域汇总等于各成员之和；越权访问返回 403。

## J. 其他读接口

- `GET /user/aff` 增加 `partner` 对象（contract 3.1）。
- 管理端代理列表：
  - 每行加 `is_partner` 和 `partner`；
  - 支持 `partner_id` 筛选；
  - 用 §C 的解析函数计算（contract 2.1）。

## K. 文档

在 `docs/AGENT_PROFIT_SHARING_CN.md` 里加一节「合伙人」，写给运营看，用短句，不堆砌说明：
- 角色和区域；
- 利润口径和示例；
- 结算单；
- 不可撤销的操作；
- 合伙人不拿代理分润。

## L. 自测

```bash
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run 'Affiliate|Partner|Finance'
```

- 现有的 `affiliate_*_test.go` 和 `finance_*_test.go` 必须全部通过：没有合伙人时，行为和改动前完全相同。
- 写完后在本机环境 `./build.sh --no-web && docker compose up -d`，用 curl 走一遍主链路，在完成报告里给出示例响应：设为合伙人 → 挂靠 → 消费 → 查结算单。

## 约定变更

- 2026-10-31 BE：财务 net_profit 按原 profit - partner_share 计算；无合伙人时分成为 0、净利润等于原利润，字段名不变。
- 比例请求最多四位小数，与数据库精度一致，超出返回 PARTNER_INVALID。
- 区域团队沿用统一归属解析（包含合伙人本人），起点只限制利润。日期兼容 start_date/end_date，接受契约 start/end、days。

## 完成报告

完成时间：2026-10-31 19:52 UTC / 北京时间 2026-11-01 03:52。

### 实现范围

A–L 已实现。后端改动均在 `backend/`；运营说明写入 `docs/AGENT_PROFIT_SHARING_CN.md`。未修改前端、契约正文或其他代理的文件，未提交、推送、切分支或连接远端。

| 任务 | 实现 |
|---|---|
| A | 迁移 264：五张合伙人表、支出关联列、索引及表/列 COMMENT；本机与 testcontainers 实际执行通过。 |
| B | 设立/预览、永久挂靠/预览、手动与自动升级时同事务自动挂靠、整批档位保护、禁止合伙人任命；仅排除合伙人受益份额，不修改 Active 或二级代理比例。 |
| C | 统一纯函数归属解析：自身锚点优先，之后 owner→upline；起点取锚点与绑定时间较晚者，缺绑定时间用用户创建时间，保留删除用户历史。 |
| D | 独立只读 REPEATABLE READ 内共用财务 SQL；跳过重复成本扫描；unnest 成员/起点精确取数；所有计费类型成本、固定账号成本 0、requested_model 口径、首日 total/after、ledger.source_user_id。 |
| E | decimal 利润引擎、首日收入折算、退款负收入、ledger.amount 原值、专项支出摊销、调整、亏损结转、月比例、分成四舍五入到分；一次 Load 接续全部未确认月。 |
| F | partner 行锁串行确认；顺序/月范围校验、完整 DTO 冻结、付款登记、调整项增删、下月比例及取消、UTF-8 BOM CSV。 |
| G | 全套管理/用户路由、重型限流、返利开关与角色权限；Wire 重新生成；本机确认八次合伙人写操作均进入既有审计日志。 |
| H | 支出关联和存在性校验；财务 totals/月 series 的分成、净利润、预估标记；日 series 不增加这三项；月 CSV 增加两列；原财务回归通过。 |
| I | 区域 overview/customers/lines/requests，移除区域 earnings；所属代理/渠道线、邮箱脱敏；挂靠代理逐级查看复用原服务；区域汇总与成员合计一致，越权 403。 |
| J | 用户 aff.partner、appoint=null、appointable=false；管理列表 is_partner/partner/partner_id，区域筛选在 COUNT 和分页之前生效。 |
| K | 运营文档新增角色、区域、利润示例、亏损、结算、不可撤销和不领取代理分润说明。 |
| L | 全量 Go 单测、受影响集成测试、构建、本机镜像、原 p10 和 curl 主链路，详见下文。 |

### 改动文件（38 个后端文件）

- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/affiliate_handler.go`
- `backend/internal/handler/admin/affiliate_handler_agent_level_test.go`
- `backend/internal/handler/admin/finance_handler.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/affiliate_repo.go`
- `backend/internal/repository/affiliate_repo_agent_level.go`
- `backend/internal/repository/affiliate_team_repo.go`
- `backend/internal/repository/finance_repo.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/user.go`
- `backend/internal/service/affiliate_agent_level.go`
- `backend/internal/service/affiliate_detail.go`
- `backend/internal/service/affiliate_distribution.go`
- `backend/internal/service/affiliate_first_recharge.go`
- `backend/internal/service/affiliate_service.go`
- `backend/internal/service/affiliate_team.go`
- `backend/internal/service/finance.go`
- `backend/internal/service/finance_reports.go`
- `backend/internal/service/wire.go`
- `backend/internal/handler/affiliate_partner_handler.go`
- `backend/internal/handler/affiliate_partner_handler_test.go`
- `backend/internal/repository/affiliate_partner_integration_test.go`
- `backend/internal/repository/affiliate_partner_repo.go`
- `backend/internal/repository/affiliate_partner_write.go`
- `backend/internal/server/routes/affiliate_partner_routes_test.go`
- `backend/internal/service/affiliate_partner.go`
- `backend/internal/service/affiliate_partner_affiliate.go`
- `backend/internal/service/affiliate_partner_finance.go`
- `backend/internal/service/affiliate_partner_finance_test.go`
- `backend/internal/service/affiliate_partner_members.go`
- `backend/internal/service/affiliate_partner_profit.go`
- `backend/internal/service/affiliate_partner_service.go`
- `backend/internal/service/affiliate_partner_team.go`
- `backend/internal/service/affiliate_partner_test.go`
- `backend/migrations/264_affiliate_partners.sql`
- `docs/AGENT_PROFIT_SHARING_CN.md`
- `.trellis/tasks/10-31-affiliate-partner/task-be.md`

### 关键实现说明

- Seedance 分组系数已反映在账本；区域分润只累计 `ledger.amount`，不重新乘系数或从 `base_amount` 推导。
- 确认时，写事务持有合伙人行锁，取数使用独立只读 RR 事务；后一个确认请求取得锁后重新读取，返回 `PARTNER_MONTH_CONFIRMED`。
- 确认后的数字与明细不再重算，付款只更新付款元数据；用户端移除调整操作者邮箱，并脱敏渠道经理邮箱。
- 区域客户查询不会扫描分润表，也不会把被替换的 viewer 参数留在分页 COUNT SQL 中。
- 现有批量设档位测试只补充契约新增的 `auto_attached: []` 断言，原 `affected` 断言保持不变；原财务测试及 p10 断言未改。

### 测试与验证

| 检查 | 结果与证据 |
|---|---|
| `go build ./...` | 通过，退出 0；`/tmp/partner-be-build-final.log`。 |
| `env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...` | 两轮全量通过，最终退出 0；`/tmp/partner-be-unit-final.log`。 |
| 最后中文错误文案的定向回归 | `go test -tags unit ./internal/service ./internal/handler/... ./internal/server/routes -run 'Partner|AffiliateHandler.*AgentLevel|AffiliateHandler.*Settings' -count=1`，同样去掉代理和 OPENAI_API_KEY；通过。`/tmp/partner-be-final-patch.log`。 |
| `go test -tags=integration ./internal/repository/ -run 'Affiliate|Partner|Finance' -count=1` | 真实 testcontainers PostgreSQL/Redis，非跳过；最终退出 0，12.440s；`/tmp/partner-be-integration-final.log`。 |
| 格式与差异 | `gofmt -l` 无输出、`git diff --check -- backend` 通过；go.mod/go.sum 未改。 |
| `./build.sh --no-web && docker compose up -d` | 最终后端镜像已更新，容器 healthy，`/health` 返回 `{"status":"ok"}`；`/tmp/partner-be-local-build-final.log`。 |
| 原版 p10 | `E2E_RUN_DIR=/tmp/partner-be-finance-e2e python3 -u p10_finance.py`，34/34 通过；原断言未改。日志、逐项结果在该目录。 |
| curl 主链路与补充检查 | 设立→手动挂靠→真实网关消费→结算单；另测自动挂靠、筛选、区域汇总/客户/逐级查看、权限、历史月确认/付款、调整增删、比例、CSV、财务新字段和审计，全部通过。 |

新增 15 个顶层测试（含表驱动子用例），覆盖拓扑/起点、首日折算、退款、分组/模型成本、账本金额、亏损、比例/舍入、同事务回滚、并发确认只能成功一次、冻结后修改用量仍不变、隐私和权限。

本机未安装 golangci-lint，未运行该工具；留给 CI。未运行前端检查、完整 p14 或 UI 截图。

### curl 主链路与响应节选

执行时刻为 UTC 2026-10-31 19:43–19:44，即北京时间 2026-11-01 03:43–03:44，因此当月结算单为 **2026-11**。所有请求仅访问 `127.0.0.1:28080`，上游为本机 `sub2api-local-upstream-mock`。令牌与 Key 以下用变量表示。

**1. 设为合伙人**

```bash
curl --noproxy '*' -X POST http://127.0.0.1:28080/api/v1/admin/affiliates/partners -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' --data '{"user_id":3147,"region_name":"BE 本机验证区域","share_rate":"0.4"}'
```

```json
{"code":0,"message":"success","data":{"user":{"id":3147},"region_name":"BE 本机验证区域","share_rate":"0.4000","member_count":1,"since":"2026-11-01T03:43:55.29818+08:00"}}
```

**2. 挂靠渠道经理**

```bash
curl --noproxy '*' -X POST http://127.0.0.1:28080/api/v1/admin/affiliates/partners/3147/agents -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' --data '{"agent_user_id":3148}'
```

```json
{"code":0,"message":"success","data":{"user":{"id":3148},"source":"manual","agent_level":1,"team":{"owned":1,"via_upline":0}}}
```

**3. 客户 3149 充值后经真实网关消费**

充值经过本机易支付下单、模拟支付、签名回调入账。消费请求：

```bash
curl --noproxy '*' -X POST http://127.0.0.1:28080/v1/messages -H "Authorization: Bearer $CUSTOMER_KEY" -H 'Content-Type: application/json' --data '{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"partner smoke E2E_USAGE in=100000 out=10000 cr=0 cw=0"}]}'
```

```json
{"model":"claude-sonnet-4-5","usage":{"input_tokens":100000,"output_tokens":10000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}
```

数据库用量实扣 `0.45`，上游成本快照计算结果 `0.135`；消费结算后的账本为：

```json
[{"amount":"0.13500000","reward_type":"agent_direct","base_amount":"0.45000000"}]
```

**4. 查结算单**

```bash
curl --noproxy '*' http://127.0.0.1:28080/api/v1/admin/affiliates/partners/3147/statements/2026-11 -H "Authorization: Bearer $ADMIN_TOKEN"
```

```json
{"code":0,"message":"success","data":{"totals":{"month":"2026-11","status":"current","share_rate":"0.4000","consumption":"0.45","revenue":"0.45","account_cost":"0.135","commission":"0.135","region_expense":"0","adjustment":"0","profit":"0.18","loss_before":"0","loss_after":"0","share_base":"0.18","share_amount":"0.07"}}}
```

**补充验证**

- 区域团队汇总：消费 `0.45`、客户数 `3`、请求数 `1`、tokens `110000`；无 `earnings` 字段。
- 追加测试用户 3150 后，经批量升一级自动挂靠：`{"affected":1,"auto_attached":[{"agent_user_id":3150,"partner":{"id":3147},"region_name":"BE 本机验证区域"}]}`。区域筛选 total=4，渠道线含直属线共 3 行。
- 非合伙人及区域外客户：403 `PARTNER_FORBIDDEN`；改合伙人档位：409 `PARTNER_LEVEL_FIXED`；任命代理：403 `PARTNER_CANNOT_APPOINT`；不存在用户：404 `USER_NOT_FOUND`，中文「用户不存在」。
- 用户结算单邮箱脱敏，不含 `account_id`、`account_name`、供应商/单价或 `created_by_email`。
- 为覆盖已结束月接口，仅在本机将测试合伙人的设立月/首个比例月前移到 2026-10，未改任何用量；添加 +2.00 调整，另一个 −1.00 调整成功删除。确认结果利润 2、分成 0.80；付款登记返回 `status=paid`、`paid_on=2026-11-01`。
- 比例改为 35% 后，本月仍 `0.4000`，`next_share_rate=0.3500`，`next_rate_month=2026-12`。
- 合伙人 CSV 带 BOM；财务月 CSV 末两列为「合伙人分成」「平台净利润」。财务 totals 的分成为 `0.07`，`partner_share_scope=whole_months`；净利润等于原利润减分成，日 series 无新增月字段。
- 八次合伙人写操作均成功落审计，ID 11148、11149、11155–11160，覆盖设立、挂靠、调整增删、确认、付款和修改。

完整响应：`/tmp/partner-be-curl-results.json`；主链路脚本/日志：`/tmp/partner-be-curl.py`、`/tmp/partner-be-curl.log`；导出：`/tmp/partner-be-statement.csv`。测试会话 state 文件已设为 600、目录 700，报告不含凭据。


### 调试中出现并已修复的问题

1. Wire 默认生成命令缺少 CLI 的 go.sum 项；直接禁用 GOSUMDB 又被自动工具链校验拦截：

```text
missing go.sum entry for module providing package github.com/google/subcommands
go: golang.org/toolchain@v0.0.1-go1.27.3.linux-amd64: verifying module: checksum database disabled by GOSUMDB=off
```

使用本机已有 Go 1.27.3、已缓存 Wire 依赖和 `/tmp/partner-wire.mod`、`/tmp/partner-wire.sum` 运行 Wire 成功。未改 `backend/go.mod` / `go.sum`，未手改 `wire_gen.go`。

2. 原 handler 测试的非合伙人 stub 没有事务实现；新包装初版对所有档位都开启了附加事务：

```text
panic: runtime error: invalid memory address or nil pointer dereference
... (*agentLevelHandlerRepo).WithTx
... (*AffiliateService).AdminSetAgentLevelWithAttachments
```

根因是旧调用路径被不必要地扩展。现改为仅有合伙人仓储且设为一级时读取挂靠前后快照；其他档位沿用原接口。事务挂靠路径由真实数据库集成测试覆盖。

3. 原批量接口做完整 JSON 等值断言，新契约多了一个字段：

```text
expected: data { affected: 3 }
actual:   data { affected: 3, auto_attached: [] }
```

只给预期 JSON 增加新字段，未删除、弱化旧断言。

4. 区域客户分页首次集成测试失败：

```text
TestPartnerRegionAggregatesEqualMembers
count team customers: pq: could not determine data type of parameter $1
```

根因是替换 WHERE 范围后仍保留旧 viewer 占位参数。已在构造参数时区分区域/单代理路径，区域分润 CTE 为空结果；真实 PostgreSQL 测试和本机 curl 都覆盖了 COUNT、分页和汇总一致性。

### 遗留与协作

- BE 无已知功能阻塞。本轮只构建后端（`--no-web`），前端页面联调、截图、p14 和完整业务回归由 FE/QA 接续。
- 前端按上方「约定变更」处理：净利润为原利润减分成；无合伙人时净利润等于原利润；比例请求最多四位小数；区域成员含合伙人本人，利润起点不限制历史团队用量。
- 合伙人相关业务错误沿用标准 envelope：HTTP/body `code` 为状态码，业务码在 `reason`，`message` 为中文。
- `docs/AGENT_PROFIT_SHARING_CN.md` 已更新，但当前被仓库 `.gitignore` 的 `docs/*` 忽略，主代理提交时需要显式纳入该文件。未修改忽略规则，也未代为暂存。
- 完成报告写入后释放本机测试环境。合伙人及消费夹具保留供联调，不删除其他测试数据。


## 补充 1

### 改动

- `backend/internal/handler/affiliate_partner_handler.go`：区域 `GET /user/aff/partner/team/overview` 复用 `parseOptionalID` 解析 `user_id`，传入服务；非法 ID 返回 400。
- `backend/internal/service/affiliate_partner_team.go`：新增客户分支，先用统一区域归属解析校验客户；区域外、其他合伙人区域或不存在的客户返回 403 `PARTNER_FORBIDDEN`。从现有快照构造 `customer`，客户本人和所属代理都使用 `presentAffiliateTeamUser(affiliateTeamAudienceAgent, ...)`；删除账号只保留 ID。复用客户筛选查询，并以单客户 `MemberIDs` 聚合 summary / trend / models，避免回退为全区域数据。
- 区域客户主体遵循契约 §3.3，不输出 `lifetime_earnings` 等分润字段；不带 `user_id` 时保留原区域总览行为。无需修改契约、数据库结构或原代理端接口。
- 新增 `backend/internal/handler/affiliate_partner_team_test.go`：HTTP 200 + customer、403 + reason、非法 ID、直属/挂靠/间接归属、合伙人及渠道经理本人、客户和所属代理脱敏、删除账号、无参数区域回归、客户筛选及聚合范围、日期传递、下游错误和分润字段不外泄。
- `backend/internal/repository/affiliate_partner_integration_test.go`：新增 `TestPartnerTeamOverviewCustomerDataIsolation`，用真实 PostgreSQL 验证客户消费 5、区域消费 12；隔离其他区域消费 90 和该客户区间外消费 100，核对客户摘要、趋势、模型、筛选、主体脱敏以及区域外拒绝。

### 测试结果

在 `backend/` 执行，均通过：

```bash
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/handler -run 'TestPartnerTeamOverview' -count=1
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service ./internal/handler ./internal/server/routes -count=1
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags=integration ./internal/repository/ -run 'Partner' -count=1
```

- 构建退出码 0。新增 HTTP 定向单测通过（0.033s）；服务、处理器、路由包完整单测分别通过（216.153s / 43.120s / 10.392s）。
- Partner 集成测试通过（6.583s）；另加 `-v` 复核，以下四项均实际执行并通过，无跳过（5.674s）：
  - `TestPartnerCreateAttachAndAutomaticPromotionAtomic`
  - `TestPartnerLoadCostsLedgerAndFrozenConcurrentStatement`
  - `TestPartnerRegionAggregatesEqualMembers`
  - `TestPartnerTeamOverviewCustomerDataIsolation`
- `gofmt -l` 检查本轮四个 Go 文件无输出，`git diff --check -- backend` 通过。
- 日志：`/tmp/partner-be-supplement1-build.log`、`/tmp/partner-be-supplement1-focused.log`、`/tmp/partner-be-supplement1-unit.log`、`/tmp/partner-be-supplement1-integration.log`、`/tmp/partner-be-supplement1-integration-verbose.log`。

### 协作与环境

- 无遗留阻塞。前端继续按 `team/overview?user_id=<客户ID>` 调用，成功返回客户详情；越权按 `reason=PARTNER_FORBIDDEN` 处理。
- 本轮只修改上述后端文件及本任务书追加内容，未改前端。未运行 `build.sh`、未重启或访问 `sub2api-local`；集成测试只使用独立 testcontainers PostgreSQL/Redis。未提交、推送或访问远端。


## 补充 2

### 改动

- `backend/internal/service/finance.go`：将财务固定说明提为 `financeReplayNotice`，拆出仅包含实际数据异常的 `financeDataWarnings`。财务页继续调用 `financeWarnings`，原有提示内容和顺序不变。
- `backend/internal/service/affiliate_partner_profit.go`：合伙人结算只调用实际数据提示函数，不再生成财务固定说明；初始化渠道线时按北京时间判断 `attached_at` 所在月不晚于结算月。直属团队行、当月挂靠但零成员/零用量的线继续保留。
- `backend/internal/service/affiliate_partner_service.go`：已有确认/付款快照统一修正展示，以提取的固定说明常量识别旧提示，并排除结算月之后才挂靠的线；不改写落库快照，不重算冻结金额、状态或亏损结转。用户端、管理端和 CSV 通过相同服务入口取数。
- `backend/internal/service/affiliate_partner_finance_test.go`：新增无用量/有用量月提示隔离、真实异常保留、财务提示完整内容及顺序回归；更新其他区域提示隔离断言。
- `backend/internal/service/affiliate_partner_test.go`：新增 8–11 月渠道线范围、北京月界（UTC 10-31 15:59:59 / 16:00）、其他区域隔离、直属行保留，以及 confirmed/paid 冻结展示和原快照不变测试。
- `backend/internal/handler/affiliate_partner_handler_test.go`：新增用户端、管理端 JSON 和 CSV 回归，确认固定说明/未来线不出现，直属线及当月线保留，用户邮箱仍脱敏，冻结总额不变。

### 测试结果

在 `backend/` 执行，全部通过，无测试失败：

```bash
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service ./internal/handler -run 'TestPartnerWarnings|TestFinanceWarningsKeep|TestPartnerLinesStart|TestPartnerFrozenStatement|TestPartnerStatementEndpoints' -count=1 -v
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/service ./internal/handler ./internal/handler/admin ./internal/server/routes -run 'Partner|Finance' -count=1
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags=integration ./internal/repository/ -run 'Partner' -count=1 -v
```

- 构建退出码 0。新增/修改定向单测通过（service 0.261s，handler 0.066s）。
- Partner/Finance 相关单测通过：service 0.277s、handler 0.064s、handler/admin 0.043s、server/routes 0.033s。
- Partner 集成测试通过（6.069s），真实 PostgreSQL/Redis testcontainers，以下四项均实际执行，无跳过：
  - `TestPartnerCreateAttachAndAutomaticPromotionAtomic`
  - `TestPartnerLoadCostsLedgerAndFrozenConcurrentStatement`
  - `TestPartnerRegionAggregatesEqualMembers`
  - `TestPartnerTeamOverviewCustomerDataIsolation`
- 六个改动 Go 文件 `gofmt -l` 无输出，`git diff --check -- backend` 通过，`go.mod` / `go.sum` 无改动。
- 日志：`/tmp/partner-be-supplement2-build.log`、`/tmp/partner-be-supplement2-focused.log`、`/tmp/partner-be-supplement2-unit.log`、`/tmp/partner-be-supplement2-integration.log`。

### 协作与环境

- 无遗留阻塞或接口字段变更；前端无需调整调用。已有冻结单也会在重新读取时修正展示。
- 财务 warnings 的内容/顺序及 Summary 输出已由回归测试验证；按本轮要求未运行本机 p10。未访问或修改 `sub2api-local`，未运行 `build.sh`，未重启联调容器。由测试代理接续构建和端到端验证。
- 本轮仅改上述六个后端文件并在本任务书追加报告，未改前端、未提交/推送、未访问远端。

后续修订：已删除 `presentPartnerStatement`、两处调用及冻结单展示修正的 service/handler 测试，冻结单恢复原样返回快照（既有用户端脱敏保留），计算侧改动及其测试保留；重新执行 `go build ./...`、去掉代理和 `OPENAI_API_KEY` 的 Partner/Finance 相关单测及 Partner 集成测试全部通过（4 项实际执行，无跳过），未使用本机环境。
