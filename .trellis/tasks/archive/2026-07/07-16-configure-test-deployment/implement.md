# 配置测试与部署环境 - 实施计划

## Implementation Checklist

1. 修改现有 `deploy/docker-compose.dev.yml`。
   - 保留当前三服务配置、源码构建和健康检查。
   - 使用 `DEPLOY_ENV` 动态生成三个显式容器名。
   - 顶层 project name、宿主绑定地址和端口使用必填变量。
   - 应用、PostgreSQL 和 Redis 使用带环境前缀的独立 bind 目录。
   - 不发布 PostgreSQL 或 Redis 宿主端口。
   - `SERVER_MODE` 改为由 env 配置。
2. 新增 custom 和 test 的无秘密 env 示例。
   - 分别设置 `DEPLOY_ENV=custom/test`。
   - 使用不同的推荐端口、数据库名和示例镜像标签。
   - 敏感值只写占位符并说明必须替换。
3. 更新 `deploy/.gitignore`，忽略二开环境的真实 env 文件，同时保留示例文件可跟踪。
4. 新增简洁部署说明。
   - 提供 config、build、up、ps、logs、down 的完整命令。
   - 强调 env file 和 Compose file 参数不能省略；project 由顶层 `name` 固定。
   - 记录生产保护检查和禁止命令。
5. 只做静态验证，不启动任何容器。

## Validation Commands

```bash
docker compose \
  --env-file deploy/.env.custom.example \
  --file deploy/docker-compose.dev.yml \
  config
```

```bash
docker compose \
  --env-file deploy/.env.test.example \
  --file deploy/docker-compose.dev.yml \
  config
```

检查渲染结果：

- 容器名分别带 `custom` 或 `test`。
- 端口分别为 `127.0.0.1:18080` 和 `127.0.0.1:18081`。
- DB/Redis 无宿主端口。
- bind 路径分别带 `custom_` 或 `test_` 前缀，网络不声明 `external`。
- 应用从当前源码构建，不引用官方生产应用镜像。
- 数据库和 Redis 主机分别为 `postgres`、`redis`。

## Review Gates

- 实施前：用户批准 PRD、设计和实施范围。
- 静态验证前：确认命令仅执行 `docker compose config`，不带 `up`、`down`、`pull` 或清理操作。
- 完成前：复查 diff，确保没有修改生产 env、运行时数据目录或无关业务文件。

## Rollback Points

- Compose 静态渲染失败：仅修改新增配置，不执行任何容器操作。
- 发现与生产资源同名或引用 external 资源：停止实施并修正配置。
- 不自动删除卷、网络、镜像或容器。
