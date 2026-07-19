# 配置源码开发运行环境

## Goal

为当前二次开发版本提供一套基于源码编译和运行的本地开发方式，减少每次修改前端或后端后完整重建 Docker 镜像的等待时间，同时继续隔离既有生产环境。

开发流程采用双轨模式：`feature/*` 分支使用宿主机 Go + Vite 热更新，`custom` 分支使用独立 Docker Compose 持续部署；最终仍支持从源码构建嵌入式前端二进制进行集成验证。

## Background

- 当前测试栈通过 `deploy/docker-compose.dev.yml` 从源码执行完整 Docker 多阶段构建，将前端产物嵌入 Go 二进制。
- 测试栈使用独立的 `sub2api-test` 项目、数据目录和凭证，不得影响正在运行的生产项目 `sub2api-deploy`。
- 用户希望参考官方“方式四：源码编译”，在宿主机执行前端和后端编译，以便开发和定制。
- 当前测试 PostgreSQL 和 Redis 只暴露在 Compose 内部网络；宿主机源码进程暂时无法直接连接。
- 前端已经提供 Vite 开发服务器：默认端口为 `3000`，并可通过 `VITE_DEV_PROXY_TARGET` 将 `/api`、`/v1` 和 `/setup` 代理到宿主机后端。
- 前端已固定使用 pnpm `11.13.0`；生产构建会直接输出到 `backend/internal/web/dist`。
- 后端支持环境变量覆盖配置，自动初始化与 SQL 迁移也适用于宿主机进程；宿主机运行时应显式使用独立 `DATA_DIR`。
- 当前 Makefile 构建目标不包含 `-tags embed`，不能直接作为最终嵌入式二进制构建命令。
- 用户已确定不再让一份动态 Compose 同时承担开发和 custom 部署：开发使用 `deploy/docker-compose.dev.yml`，custom 部署新增 `deploy/docker-compose.custom.yml`，官方 `deploy/docker-compose.yml` 保持不变。
- 开发与 custom 部署继续分别使用 git 忽略的 `.env.test` 和 `.env.custom` 保存秘密，避免把数据库密码、JWT、TOTP 和管理员密码写入 YAML。

## Requirements

- 提供可重复执行的源码开发与编译命令，不要求重新克隆仓库或全局安装 pnpm。
- 前端使用仓库锁定的 pnpm 版本和锁文件安装依赖，并能构建到后端嵌入目录。
- 后端支持使用 `-tags embed` 构建包含前端页面的可执行文件。
- 日常 feature 开发支持宿主机 Go 后端与 Vite 前端分离运行，前端修改通过 HMR 生效。
- 源码运行使用独立的测试数据库、Redis、数据目录、端口和秘密，不连接或修改生产资源。
- `deploy/docker-compose.dev.yml` 只负责隔离的测试 PostgreSQL 和 Redis，并通过仅绑定 `127.0.0.1` 的独立端口供宿主机后端访问；不得包含或隐式启动应用容器。
- 新增 `deploy/docker-compose.custom.yml`，从当前 custom 工作树构建并运行完整应用、PostgreSQL 和 Redis；只发布应用回环端口，不发布数据库或 Redis 端口。
- `deploy/docker-compose.yml` 保持官方镜像部署语义，不改造成 custom 部署文件。
- test 与 custom 使用固定且不同的 Compose project、容器名、端口、数据目录、数据库、Redis 和秘密。
- 提供清晰的启动、停止、构建、运行、健康检查和故障排查说明。
- 最终 Docker 构建仍作为发布前验证方式，源码开发方式不得改变生产部署。

## Acceptance Criteria

- [ ] 开发者可仅启动测试 PostgreSQL 和 Redis，并从宿主机连接，且数据库和 Redis 端口只绑定到 `127.0.0.1`。
- [ ] 开发者可在宿主机运行 Go 后端并连接 test 依赖，Vite 可代理到该后端并提供前端热更新。
- [ ] 开发者可从当前源码编译包含前端的 Go 二进制，在独立测试端口启动并通过 `/health` 检查。
- [ ] custom 工作目录可使用 `docker-compose.custom.yml` 构建当前源码并运行完整隔离栈，反向代理目标保持为 custom 应用回环端口。
- [ ] `docker-compose.dev.yml`、`docker-compose.custom.yml` 和官方 `docker-compose.yml` 职责明确，互不复用应用容器、数据目录或依赖网络。
- [ ] 文档给出首次准备、开发依赖启动、Go/Vite 运行、嵌入式编译、custom 部署、停止和健康检查命令。
- [ ] 所有真实秘密继续位于 git 忽略的环境文件或本地配置中，提交文件不包含真实密码和令牌。
- [ ] 生产容器、生产 Compose 项目、生产数据库、Redis、卷和反向代理配置均不被修改。

## Out of Scope

- 修改正式生产部署方式。
- 自动配置公网反向代理、DNS 或 TLS。
- Go 后端自动热重载；后端修改后允许手动重启源码进程。
- 复用生产 PostgreSQL、Redis 或应用数据。
