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

## 常见错误

- 改了 schema 却未运行/提交 Ent 生成代码。
- 修改已经应用的迁移，导致 checksum mismatch。
- 在 `_notx.sql` 中混入 DML、事务控制或普通 DDL。
- 原生 SQL 忘记软删除条件或 `updated_at = NOW()`，与 Ent 行为不一致。
- 事务中继续使用 `r.client`/`r.sql` 的非事务连接，造成部分提交。
- 在 service/handler 中依赖 PostgreSQL、Ent 或 Redis 的具体错误与类型。
