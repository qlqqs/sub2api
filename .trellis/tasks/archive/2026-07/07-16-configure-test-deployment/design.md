# 配置测试与部署环境 - 技术设计

## Architecture

直接改造现有 `deploy/docker-compose.dev.yml`。文件通过顶层 `name: sub2api-${DEPLOY_ENV}` 自动固定 project name，从而让同一份配置服务两个环境，无需新增或复制 Compose 文件。

用户要求保留显式容器名，因此三个服务使用：

```yaml
container_name: sub2api-${DEPLOY_ENV:?DEPLOY_ENV is required}
container_name: sub2api-${DEPLOY_ENV:?DEPLOY_ENV is required}-postgres
container_name: sub2api-${DEPLOY_ENV:?DEPLOY_ENV is required}-redis
```

Compose 插值只能检查变量是否存在，不能原生限制枚举值。`custom` / `test` 约束由独立 env 示例、固定命令和文档规约保证；静态验证检查渲染后的容器名与 project name。

## Isolation Boundaries

### Containers and project

- Custom project：`sub2api-custom`。
- Test project：`sub2api-test`。
- 容器名带对应环境后缀，防止与官方生产固定名称冲突。
- 所有运维命令必须完整携带 env file 和 Compose file；project 由顶层动态 `name` 固定。

### Ports

- 容器内应用端口固定为 `8080`。
- 宿主绑定地址和端口必须显式注入，不提供 `8080` 默认值。
- 推荐 custom 为 `127.0.0.1:18080`，test 为 `127.0.0.1:18081`。
- PostgreSQL 和 Redis 不发布宿主端口。

### Data and networks

- 继续使用便于本地开发的 bind 目录，但路径改为 `deploy/${DEPLOY_ENV}_data`、`deploy/${DEPLOY_ENV}_postgres_data` 和 `deploy/${DEPLOY_ENV}_redis_data`。
- 即使 custom 与 test 从同一仓库启动，数据路径也不会重叠。
- 网络同理由 project 隔离；不使用 `external: true`，不加入生产网络。

### Database and Redis

- 应用固定通过服务 DNS `postgres` 和 `redis` 访问本栈依赖。
- 每套环境拥有独立 PostgreSQL 和 Redis 容器、卷、密码。
- 数据库名分别建议为 `sub2api_custom` 和 `sub2api_test`，作为误连防护；不把 Redis DB 编号当作隔离手段。

### Runtime configuration and secrets

- 应用继续从当前源码构建，不复用官方生产镜像。
- `SERVER_MODE` 由 env 注入，custom 使用 `release`，test 使用 `debug`。
- custom/test 使用独立 env 文件；仓库只提交示例，不提交真实秘密。

## Operational Flow

1. 从对应示例创建仓库外的私密 env 文件并设置权限为 `0600`。
2. 使用对应 env 文件执行 `docker compose config`，由 `DEPLOY_ENV` 渲染固定 project name。
3. 人工核对容器名、端口、卷、网络、镜像及 DB/Redis 连接目标。
4. 仅在核对通过后，由用户手动执行 build/up。
5. 查看、停止操作继续使用同一组 env file 和 Compose file 参数。

## Compatibility and Trade-offs

- 保留显式 `container_name` 满足可识别性要求，但 project 无法自动为容器名加前缀，也不适合水平扩容。因此 `DEPLOY_ENV` 和命令规约是硬约束。
- 单 Compose 减少 custom/test 配置漂移；代价是环境差异必须通过 env 表达。
- 带环境前缀的 bind 目录保留当前开发部署的直观备份方式，同时避免 custom/test 串数据。

## Rollback

- 未实际启动容器时，回滚仅涉及本次修改和新增的配置文件。
- 若用户后续手动启动，只使用目标 project 的完整命令停止；默认不使用 `down -v`。
- 数据库迁移为 forward-only；镜像回滚前必须评估 schema 兼容性并恢复目标环境自己的备份，绝不操作生产备份或卷。
