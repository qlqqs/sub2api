# 数据库规范

## 数据边界

主数据存储是 PostgreSQL，实体访问以 Ent 为主；Redis 用于缓存、并发控制和短生命周期状态。业务代码依赖 `internal/service/` 中定义的 repository/cache 接口，具体实现位于 `internal/repository/`。

参考实现：

- `internal/service/user_service.go` 定义 `UserRepository`，`internal/repository/user_repo.go` 返回该接口的实现。
- `internal/service/proxy_service.go` 定义 `ProxyRepository`，`internal/repository/proxy_repo.go` 负责 Ent/SQL 映射。
- `internal/repository/error_translate.go` 将 Ent、`database/sql` 和 PostgreSQL 唯一约束错误翻译成业务错误。

## Ent schema 与命名

- 手写 schema 位于 `backend/ent/schema/`；修改后运行 `go generate ./ent`，提交所有生成文件。
- 表名和列名使用 `snake_case`，表通常为复数。需要明确表名时通过 `entsql.Annotation{Table: "..."}` 声明；参见 `schema/group.go` 和 `schema/setting.go`。
- 主键使用 `int64`。时间列优先复用 `mixins.TimeMixin`，PostgreSQL 类型为 `timestamptz`。
- 支持软删除的实体复用 `mixins.SoftDeleteMixin`。普通查询自动附加 `deleted_at IS NULL`；仅恢复、审计或物理清理场景使用 `mixins.SkipSoftDelete(ctx)`。
- JSON 数据明确声明 `jsonb`；金额/倍率字段在 schema 中明确 PostgreSQL decimal 精度，参见 `schema/group.go`。
- 唯一性如果受软删除影响，使用带 `WHERE deleted_at IS NULL` 的部分唯一索引，而不是无条件 `Unique()`。

## 查询与映射

- 常规 CRUD 使用 Ent builder，并始终传递调用方的 `context.Context`。
- repository 负责 Ent entity 与 service model 的转换，例如 `userEntityToService` 和 `applyUserEntityToService`；不要让 Ent 类型穿透到 handler/service 合约。
- 需要原子增量、批量 `ANY($n)`、行锁、复杂聚合或 Ent 无法清晰表达的语句时可以使用参数化原生 SQL。参考 `user_repo.go` 的 `BatchUpdateLimits`、`proxy_repo.go` 的 `FOR NO KEY UPDATE`。
- 动态 SQL 只拼接由程序控制的列/片段，用户值必须走 `$1` 参数。标识符确需动态生成时使用 `pq.QuoteIdentifier` 等结构化转义。
- 查询多行时关闭 rows 并检查 `rows.Err()`；更新需要区分“不存在”时检查 `RowsAffected()`。
- repository 使用 `translatePersistenceError` 统一映射 not-found 和 unique-conflict；不要把驱动错误文本当稳定业务判断。

## 事务

- 一个用例涉及多次相关写入时，在拥有这些持久化操作的 repository/服务事务端口中建立事务。
- Ent 事务通过 `client.Tx(ctx)` 创建，并用 `dbent.NewTxContext(ctx, tx)` 传播；repository 内调用 `clientFromContext`/`dbent.TxFromContext` 复用外层事务。
- 建立事务后立即安排兜底 rollback，仅在全部操作成功后 commit：

```go
tx, err := r.client.Tx(ctx)
if err != nil { return err }
defer func() { _ = tx.Rollback() }()
txCtx := dbent.NewTxContext(ctx, tx)
// 所有读写使用 txCtx / tx.Client()
return tx.Commit()
```

`internal/repository/user_repo.go` 的 `Create`/`Update` 和 `proxy_repo.go` 的 `Update` 是可复用样例。若方法允许嵌套调用，必须识别 `dbent.ErrTxStarted` 并复用上下文事务，不能悄悄落回非事务 client。

## 迁移

- SQL 文件位于 `backend/migrations/`，命名为递增的 `NNN_description.sql`（历史上也有 `108a` 之类兼容编号；新迁移沿用下一个可用编号）。
- 迁移由 `migrations/migrations.go` 嵌入，并由 `internal/repository/migrations_runner.go` 在启动时执行；`schema_migrations` 保存 SHA256。
- 已进入任何环境的迁移不可修改、删除、重命名或重新排序。修正必须新增前向迁移。
- 普通 `*.sql` 自动包在事务中。含 `CREATE/DROP INDEX CONCURRENTLY` 的文件必须以 `_notx.sql` 结尾，只包含幂等并发索引语句，并使用 `IF EXISTS`/`IF NOT EXISTS`。
- runner 不解析 goose 的 Up/Down 段；同一文件中不要放可执行的 Down SQL。
- DDL 尽量幂等，说明数据回填、安全检查或锁影响。复杂迁移增加针对 SQL 或最终 schema 的 integration test；参考 `migrations_schema_integration_test.go` 和 `openai_long_context_billing_migration_integration_test.go`。

## 触发器维护的汇总表

现有三类汇总：`usage_logs` 上 222/223 的分组日汇总（水位 + 定时重建）；251 的用户日汇总 `usage_user_daily_rollups`（代理团队消费页）；`user_affiliate_ledger` 上 252 的分润日汇总 `affiliate_agent_reward_daily_rollups`（受益代理 × 来源 × 北京时间日，管理端团队消费页）。后两者用语句级触发器增量维护，新增类似汇总时按 252 的写法：

- 用 `AFTER INSERT/DELETE/UPDATE ... REFERENCING NEW/OLD TABLE ... FOR EACH STATEMENT`，一个函数按 `TG_OP` 分支（转换表只在对应事件里存在，引用不存在的转换表会报错）；带转换表的触发器不能写列清单。
- 所有写入都走「先 `GROUP BY` 主键（同一语句同一键只能出现一次）、再 `ORDER BY` 主键」的 upsert，扣减就是累加负数：并发批次按相同顺序加锁，避免死锁。扣减后计数 `<= 0` 的行删除。
- UPDATE 按键合并「旧行扣减、新行累加」，`HAVING` 只写净变化不为 0 的键：只改其他列的 UPDATE（如流水的解冻 `frozen_until`、提现回写快照列）不写汇总。251 是先全部扣减再全部累加，只适合几乎没有 UPDATE 的表。
- 函数体包 `EXCEPTION WHEN OTHERS THEN RAISE WARNING`：汇总失败不能让计费明细 / 入账写入失败。
- 日界固定写 `(created_at AT TIME ZONE 'Asia/Shanghai')::date`，Go 侧用 `service.AffiliateTeamLocation`（固定 +8，不依赖 tzdata）。外键 `SET NULL` 的列在汇总键里用 `COALESCE(col, 0)`，外键动作产生的 UPDATE / DELETE 同样经过触发器。
- 迁移内先建触发器再回填（同一事务，无空档）；回填与运维修复复用同一个 SQL 函数，例如 `SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '2026-10-01', DATE '2026-10-22');`。
  - 修复函数是「删区间」「按明细重建」两条语句、两个快照：若并发批次恰好在两步之间提交一个汇总里还没有的新键，会被重复累加。252 的修复函数先 `LOCK TABLE ... IN SHARE MODE`，并发写入等到修复事务结束；251 未加锁（`usage_logs` 是热表，加锁会阻塞网关写入），修复时选低峰并避开当天。
- 整月删分区不经过触发器：保留期清理（`CleanupUsageLogs`）里按日期删除过期汇总。
- 多维读取尽量一次扫描：`GROUP BY GROUPING SETS ((bucket_date), (model), (user_id))` 同时出趋势、分布和按用户汇总（见 `affiliate_team_repo.go` 的 `GetAggregates`、`affiliate_team_admin_repo.go` 的 `GetPlatformAggregates`）。
- 开销（本机 PG 18，`research/perf.md`，任务 `10-23-admin-team-consumption`）：每个语句约多 0.05–0.08 ms；但同一事务内对同一汇总行反复更新会拉长版本链，单行插入循环 5000 次时每条约 1–1.5 ms。批量写入同一键的明细时合并成一条语句。
- 核对汇总与明细用按主键 `FULL JOIN` 比较，不要对两个大的聚合结果做 `EXCEPT`（260 万行流水上 `EXCEPT` 十几分钟不结束，`FULL JOIN` 2.5 s）。

## fork 自有配置小表

- fork 新增的配置数据（如 `group_model_rate_multipliers`，迁移 253）用**独立新表 + 原生 SQL 仓储**，不改上游 Ent schema 和 `groups` 等上游表：上游频繁改动的 schema、认证缓存快照和仓储文件一加字段就会在每次合并时冲突。上游的 `user_group_rate_multipliers` 也是同样做法。
- 外键 `REFERENCES groups(id) ON DELETE CASCADE`；分组是软删除，读全表时 `JOIN groups ... WHERE groups.deleted_at IS NULL`。整组替换在一个事务里「先删后插」。
- 请求热路径读这类小表时用进程内整表快照（`map[groupID]` 索引，30 秒过期 + singleflight），管理端保存后清本进程快照；多实例最多 30 秒后生效。不要为它改认证缓存快照或升快照版本。
- 设置类数据（如美元汇率 `fx_usd_cny_*`）直接用 `settings` 表的 key，key 常量定义在 fork 文件里。

## 常见错误

- 改了 schema 却未运行/提交 Ent 生成代码。
- 修改已经应用的迁移，导致 checksum mismatch。
- 在 `_notx.sql` 中混入 DML、事务控制或普通 DDL。
- 原生 SQL 忘记软删除条件或 `updated_at = NOW()`，与 Ent 行为不一致。
- 事务中继续使用 `r.client`/`r.sql` 的非事务连接，造成部分提交。
- 在 service/handler 中依赖 PostgreSQL、Ent 或 Redis 的具体错误与类型。

## 注册可选赠送与外层事务（2026-10-27）

- **范围**：注册附带的可选赠送，特别是 pending OAuth 的 `FinalizeOAuthEmailAccount` 外层事务。
- **签名/存储**：`AffiliateRepository.CreditInviteeSignupBonus(ctx, userID, amount)`；`user_affiliate_ledger` 的 action/reward_type 为 `invitee_signup_bonus`；迁移262的独立 `(source_user_id,reward_type)` 部分唯一索引。
- **合约**：账本和余额增量同事务，每个来源用户最多一次；注册绑定 `bound=true` 后才发，管理员补绑不发。仓储返回错误后，外层注册仍须能提交。
- **错误边界**：赠送 SQL 放在保存点内，失败先 `ROLLBACK TO SAVEPOINT` 再 `RELEASE`，保留原错误链。不能只在 service 里吞错误——PostgreSQL 的 aborted 状态会让外层提交失败。连接不可用/请求取消仍属于注册本身无法完成的基础设施故障，不承诺恢复。
- **正例/基线/反例**：金额5→余额+5、一条赠送；重复→false、无增量；触发器拒绝余额更新→账本无残留、余额不变、外层后续写入和 commit 仍成功。
- **测试**：`affiliate_signup_bonus_integration_test.go` 覆盖索引、并发、回滚、故障保存点、真实充值上限、累计赠送与余额历史；`TestCompleteEmailOAuthRegistrationUsesAffiliateCodeFromPendingSession` 覆盖 pending 最终建号与会话重放。
- **错误与正确**：不要在持有事务连接时为刷新响应调用不识别 tx 的普通读取；SQLite 单连接会阻塞，PostgreSQL 可能读到旧余额。pending 响应只有令牌，提交后客户端读余额；非事务注册才刷新响应中的余额。不能通过增大测试连接池或超时掩盖这类问题。

## 注册赠送邀请来源（2026-10-29）

1. **范围**：注册赠送按直接邀请人分为代理/普通用户，不改变首充、分润或旧赠送账本。
2. **签名/存储**：settings 键 `affiliate_invitee_signup_bonus_source`，枚举 `all|agent|user`、缺省 `all`；后台请求字段为可选指针，省略必须保留原值。`AffiliateRewardConfig.signupBonusFor(inviterWasAgent)` 为发放、detail 与预览共享的来源匹配逻辑。
3. **契约**：`bindInviterWithOwner` 在绑定事务中把 `binding.InviterWasAgent` 附到非公开的返回字段，注册发放用该快照，不能用本次邀请自动升级后的等级。普通邀请人有代理上级仍属普通来源。`POST /api/v1/auth/affiliate-signup-bonus` 接收 `{aff_code}`，标准成功 envelope 内只返回 `{amount}`，只读、匿名、no-store、独立 IP 限流；用户邀请 detail 根据本人当前等级返回适用金额，已领取合计继续读账本。
4. **校验/错误**：管理端显式空或非法来源为400；服务层拒绝非空非法来源。预览无效/不存在/已删除邀请人、来源不匹配、总开关或金额关闭均返回0；基础设施错误按 ErrorFrom 返回，不泄露用户身份或数据库详情。预览不创建档案、不绑定、不锁定发放金额。
5. **正例/基线/反例**：all 保持原赠送；agent 只给代理直接邀请的新用户；普通用户归属代理不算 agent。本次注册促成代理升级仍按绑定前普通身份判断；金额5改10后新注册发10，历史5不变。
6. **必需测试**：`TestSignupSourceMatrix`、`TestSignupSourceUsesDirectInviterSnapshot`、`TestSignupSourceOAuth`、`TestPreviewAffiliateSignupBonus`、`TestSignupSourceSettingRoundTripAndPartialUpdate`；原注册赠送并发/保存点集成测试继续通过；本机 SGS 用真实注册核对余额和账本。
7. **错误与正确**：错误是用 `AgentOwnerID != nil` 判定代理邀请，或注册页直接读全站公开金额；正确是直接邀请人绑定快照与来源预览。公开金额旧字段为兼容保留，不代表每个邀请码都适用。
