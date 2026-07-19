<!-- CUSTOM: Safe operating guide for isolated secondary-development stacks. -->

# Feature 源码开发与 Custom 部署指南

本仓库将 Feature 分支日常开发与 Custom 部署分开管理。不要使用官方
`docker-compose.yml` 部署 Custom 分支，该文件保留上游官方镜像部署方式。

## 一、Feature 分支源码开发

Feature 开发模式只在 Docker 中运行 PostgreSQL 和 Redis，Go 后端与 Vite
前端直接使用当前源码运行。修改前端后可以通过 HMR 热更新，不需要重新构建
Docker 镜像。

```bash
docker compose --env-file deploy/.env.dev -f deploy/docker-compose.dev.yml up -d
```

首次使用时安装仓库固定的前端工具链：

```bash
corepack enable
corepack prepare pnpm@11.13.0 --activate
pnpm --dir frontend install --frozen-lockfile
```

使用测试依赖配置运行宿主机 Go 后端。`DATA_DIR` 应与 Docker 应用数据目录
分开，避免多个进程共用同一份配置和运行状态：

```bash
set -a
. deploy/.env.dev
set +a

export DATA_DIR="$PWD/deploy/dev_source_data"
export SERVER_HOST=127.0.0.1
export SERVER_PORT="${SOURCE_BACKEND_PORT:-18081}"
export SERVER_MODE=debug
export DATABASE_HOST=127.0.0.1
export DATABASE_PORT="${DEV_POSTGRES_PORT:-15433}"
export DATABASE_USER="${POSTGRES_USER}"
export DATABASE_PASSWORD="${POSTGRES_PASSWORD}"
export DATABASE_DBNAME="${POSTGRES_DB}"
export DATABASE_SSLMODE=disable
export REDIS_HOST=127.0.0.1
export REDIS_PORT="${DEV_REDIS_PORT:-16380}"
export REDIS_PASSWORD="${REDIS_PASSWORD}"
export REDIS_DB="${REDIS_DB:-0}"
export AUTO_SETUP=true
export ADMIN_EMAIL="${ADMIN_EMAIL}"
export ADMIN_PASSWORD="${ADMIN_PASSWORD}"
export JWT_SECRET="${JWT_SECRET}"
export TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY}"

(cd backend && go run ./cmd/server)
```

在另一个终端启动 Vite，并将前端 API 代理到宿主机后端：

```bash
VITE_DEV_PROXY_TARGET="http://127.0.0.1:${SOURCE_BACKEND_PORT:-18081}" \
VITE_DEV_PORT="${VITE_DEV_PORT:-3000}" \
pnpm --dir frontend dev
```

浏览器访问 `http://127.0.0.1:3000`。源码后端健康检查地址为
`http://127.0.0.1:18081/health`。

### 通过开发域名访问 Vite

如果通过手动管理的反向代理域名（例如 `subdev.qlqq.de`）访问 Vite，可能会看到
以下提示：

```text
Blocked request. This host ("subdev.qlqq.de") is not allowed.
```

本仓库的 `frontend/vite.config.ts` 已将 `server.allowedHosts` 设置为 `true`，以允许
手动管理的开发反向代理转发 Host 请求头。Vite 只在启动时读取配置；修改配置后
必须完全停止旧的 Vite 进程，再重新执行上面的启动命令，不能只刷新浏览器页面。

如果重启后仍然报错，请检查 `frontend/` 下是否残留了 `vite.config.js` 或
`vite.config.d.ts`。旧版 TypeScript 构建配置可能在源码旁生成这些文件，而 Vite 会
优先加载旧的 `vite.config.js`，导致 `vite.config.ts` 中的最新设置被忽略。可执行：

```bash
rm -f frontend/vite.config.js frontend/vite.config.d.ts
```

然后重新启动 Vite。当前 `frontend/tsconfig.node.json` 已将这类编译产物输出到
`node_modules/.cache/`，后续执行前端构建不应再在 `frontend/` 根目录生成影子配置。

`allowedHosts: true` 仅适合本开发环境。不要将 Vite 开发端口直接暴露到公网；公网
入口应由受控的反向代理提供，并配置访问限制。

只停止开发依赖容器：

```bash
docker compose --env-file deploy/.env.dev -f deploy/docker-compose.dev.yml down
```

如需进行集成验证，可构建嵌入前端页面的 Go 二进制：

```bash
pnpm --dir frontend run build
cd backend
CGO_ENABLED=0 go build -tags embed -trimpath -o bin/sub2api ./cmd/server
```

## 二、Custom Docker 部署

Custom 分支使用独立的 Compose 文件和数据目录。应用仅绑定到
`127.0.0.1:18080`，PostgreSQL 和 Redis 只在本 Compose 网络内可访问。

先检查渲染后的配置：

```bash
docker compose \
  --env-file deploy/.env.custom \
  -f deploy/docker-compose.custom.yml \
  config

# 构建当前源码并启动完整 Custom 栈
docker compose \
  --env-file deploy/.env.custom \
  -f deploy/docker-compose.custom.yml \
  up -d --build

docker compose \
  --env-file deploy/.env.custom \
  -f deploy/docker-compose.custom.yml \
  ps
```

反向代理由你手动配置，上游地址填写：

```text
http://127.0.0.1:18080
```

不要将以下命令作为日常操作：`down -v`、`docker volume prune`、
`docker system prune`。每次执行 Compose 命令都必须明确带上 `--env-file`
和 `-f`，避免误操作官方生产项目。
