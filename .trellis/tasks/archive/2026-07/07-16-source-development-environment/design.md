# 配置源码开发运行环境 - 技术设计

## 1. 配置边界

- `deploy/docker-compose.dev.yml` 专用于 feature 分支的源码开发依赖，只定义 PostgreSQL 和 Redis，并将端口绑定到宿主机 `127.0.0.1`。
- `deploy/docker-compose.custom.yml` 是二开 custom 部署文件，完整运行当前工作树构建的应用、PostgreSQL 和 Redis；不修改官方 `deploy/docker-compose.yml` 的官方镜像语义。
- `.env.test` 与 `.env.custom` 分别提供非共享的端口、数据库、Redis 和应用秘密；真实文件继续被 `.gitignore` 忽略。

## 2. Feature 源码运行链路

1. 使用 `.env.test` 启动 dev Compose 的 PostgreSQL 和 Redis。
2. 宿主机 Go 进程读取独立 `DATA_DIR` 与 test 数据库/Redis连接变量。
3. 宿主机 Vite 使用 `VITE_DEV_PROXY_TARGET` 代理到 Go 后端，并保留现有 HMR 行为。
4. 后端修改后手动重启 Go 进程；前端修改由 Vite 热更新。

开发后端端口、Vite 端口、PostgreSQL 端口和 Redis 端口都不使用 custom 或官方生产端口。

## 3. Custom 部署链路

- custom Compose 顶层 project name 固定为 `sub2api-custom`。
- 应用容器名为 `sub2api-custom`，依赖容器名带 `-postgres` 和 `-redis` 后缀。
- 应用镜像从当前工作树的根目录和根 `Dockerfile` 构建，使用独立 `custom_data`、`custom_postgres_data`、`custom_redis_data` bind 目录。
- 仅应用发布 `127.0.0.1:18080:8080`；PostgreSQL 和 Redis 只在 Compose 网络内可见。
- 反向代理不由本任务配置，用户后续手动指向 `127.0.0.1:18080`。

## 4. 兼容与安全

- 开发 Compose 与 custom Compose 不共享容器名、project、数据目录、宿主机端口或网络。
- 不把秘密写入 YAML；示例 env 只保留占位符，真实 env 文件不提交。
- 不执行生产 Compose 的停止、重建、删除或清理命令。
- 开发依赖端口只绑定回环地址，避免向局域网暴露数据库和 Redis。

## 5. 回滚

配置回滚只删除 custom Compose 与源码开发文档/脚本挂点，并恢复 dev Compose 的变更；不触碰生产运行时资源和数据目录。
# 配置源码开发运行环境 - 实施计划

## 1. Compose 配置

- [ ] 将 `deploy/docker-compose.dev.yml` 收窄为 PostgreSQL/Redis 开发依赖，并添加 test 专用回环端口变量。
- [ ] 新增 `deploy/docker-compose.custom.yml`，完整运行当前源码构建的 custom 应用栈。
- [ ] 补充 custom/test 示例环境变量及 `.gitignore` 检查，确保不提交真实秘密。

## 2. 源码开发命令

- [ ] 提供 Corepack/pnpm 安装、Vite HMR、Go 后端运行、嵌入式前端构建和健康检查命令。
- [ ] 确保宿主机后端使用 test 依赖地址、独立 `DATA_DIR` 和独立端口。
- [ ] 明确 custom 构建、启动、日志、停止命令以及手动反向代理目标。

## 3. 文档与验证

- [ ] 更新二开部署说明，区分 feature 源码开发、custom Docker 部署和官方生产部署。
- [ ] 静态渲染 dev/custom Compose，核对项目名、容器名、端口、数据目录和镜像来源。
- [ ] 启动 dev PostgreSQL/Redis，运行源码后端与 Vite，验证 `/health` 和前端代理。
- [ ] 构建 custom Docker 镜像并验证应用、数据库、Redis 健康状态，不修改官方生产栈。

## 4. 关键验证命令

```bash
docker compose --env-file deploy/.env.test -f deploy/docker-compose.dev.yml config
docker compose --env-file deploy/.env.custom -f deploy/docker-compose.custom.yml config
go test ./...
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
```

## 5. 风险与回滚点

- 若 dev Compose 仍会隐式创建应用容器，停止实施并修正 service 定义。
- 若 custom Compose 引用官方应用镜像或生产数据目录，停止实施并修正后再启动。
- 不使用 `down -v`、`volume prune` 或 `system prune`；只操作明确的 custom/test Compose 文件。
