# 配置测试与部署环境

## Goal

为二次开发版本提供一份可同时服务于 `custom` 和 `test` 的 Docker Compose 配置，使两套环境可以在本机官方 sub2api 生产 Docker 持续运行的情况下独立构建、启动、验证和停止，且不修改或复用生产容器、端口、数据、网络、环境变量与镜像标签。

## Background

- 本机已经部署并运行官方 sub2api 生产 Docker；生产栈必须保持不变。
- 当前 `deploy/docker-compose.dev.yml` 使用固定 `*-dev` 容器名、默认宿主机端口 `8080`，并与 local 配置共用 `deploy/data`、`deploy/postgres_data`、`deploy/redis_data`。
- 用户已决定采用“一份 Compose + `DEPLOY_ENV=custom/test` 动态命名”的方案，并要求保留显式 `container_name`。
- Docker 容器名不能包含 `/`；环境名称统一使用正确拼写 `custom` 和 `test`。

## Requirements

1. 直接改造现有 `deploy/docker-compose.dev.yml`，不新增 Compose 文件，也不修改现有官方生产 Compose 的运行状态或资源。
2. `DEPLOY_ENV` 必须显式设置且只允许操作规约中的 `custom` 或 `test`；容器名分别渲染为：
   - `sub2api-${DEPLOY_ENV}`
   - `sub2api-${DEPLOY_ENV}-postgres`
   - `sub2api-${DEPLOY_ENV}-redis`
3. 启动命令必须同时使用与环境对应的 Compose project name：`sub2api-custom` 或 `sub2api-test`。
4. 应用宿主端口必须由独立 env 文件显式配置，推荐 custom 使用 `127.0.0.1:18080`，test 使用 `127.0.0.1:18081`；不得回退到生产常用的 `8080`。
5. custom 与 test 必须使用带 `DEPLOY_ENV` 前缀的独立 bind 目录保存应用、PostgreSQL 和 Redis 数据，不得使用当前公共 bind 目录或生产卷。
6. PostgreSQL 和 Redis 必须由每套环境自己的 Compose project 提供，不映射宿主机端口，不连接生产数据库或 Redis。
7. custom 与 test 必须使用不同的 env 文件和 secrets，包括数据库密码、Redis 密码、JWT、TOTP 与管理员密码。
8. `SERVER_MODE` 必须允许通过环境变量设置，custom 推荐 `release`，test 推荐 `debug`。
9. 提供不含真实秘密的 custom/test env 示例，以及明确携带 `--env-file` 和 `--file` 的构建、启动、查看、停止和静态验证命令；Compose 顶层 `name` 根据 `DEPLOY_ENV` 固定项目名。
10. 所有验证默认只执行静态 Compose 渲染；任何实际容器启动、停止、删除或 Docker 清理操作均不作为自动实施步骤。

## Acceptance Criteria

- [x] 同一份二开 Compose 能分别使用 `DEPLOY_ENV=custom` 和 `DEPLOY_ENV=test` 渲染成功。
- [x] custom 的三个容器名为 `sub2api-custom`、`sub2api-custom-postgres`、`sub2api-custom-redis`。
- [x] test 的三个容器名为 `sub2api-test`、`sub2api-test-postgres`、`sub2api-test-redis`。
- [x] 两套环境使用不同 project name、宿主端口、env 文件和 bind 数据目录，不引用生产数据资源。
- [x] PostgreSQL 和 Redis 不暴露宿主端口，应用内部连接目标保持为 `postgres` 和 `redis`。
- [x] Compose 配置缺少 `DEPLOY_ENV`、宿主绑定地址、宿主端口或数据库密码时快速失败，而不是使用危险默认值。
- [x] 文档说明先运行 `docker compose config`，并明确禁止无上下文执行 `down -v`、`volume prune` 或 `system prune`。
- [x] 不启动、停止、重建或修改现有生产 Docker 资源。

## Out of Scope

- 修改或迁移官方生产 Docker 栈。
- 让 custom/test 共用生产 PostgreSQL、Redis、卷或 Docker 网络。
- 自动配置公网反向代理、DNS、TLS、CI/CD 或远程镜像仓库。
- 自动执行 custom/test 容器的首次启动与数据库迁移。
