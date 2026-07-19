# 配置源码开发运行环境 - 实施计划

## 1. Compose 配置

- [x] 将 `deploy/docker-compose.dev.yml` 收窄为 PostgreSQL/Redis 开发依赖，并添加 test 专用回环端口变量。
- [x] 新增 `deploy/docker-compose.custom.yml`，完整运行当前源码构建的 custom 应用栈。
- [x] 补充 custom/test 示例环境变量及 `.gitignore` 检查，确保不提交真实秘密。

## 2. 源码开发命令

- [x] 提供 Corepack/pnpm 安装、Vite HMR、Go 后端运行、嵌入式前端构建和健康检查命令。
- [x] 确保宿主机后端使用 test 依赖地址、独立 `DATA_DIR` 和独立端口。
- [x] 明确 custom 构建、启动、日志、停止命令以及手动反向代理目标。

## 3. 文档与验证

- [x] 更新二开部署说明，区分 feature 源码开发、custom Docker 部署和官方生产部署。
- [x] 静态渲染 dev/custom Compose，核对项目名、容器名、端口、数据目录和镜像来源。
- [x] 启动 dev PostgreSQL/Redis；源码后端与 Vite 的运行命令已文档化，嵌入式构建已验证。
- [ ] 构建 custom Docker 镜像并验证应用、数据库、Redis 健康状态，不修改官方生产栈。

## 4. 关键验证命令

```bash
docker compose --env-file deploy/.env.test.example -f deploy/docker-compose.dev.yml config
docker compose --env-file deploy/.env.custom.example -f deploy/docker-compose.custom.yml config
pnpm --dir frontend run typecheck
pnpm --dir frontend run build
cd backend && CGO_ENABLED=0 go build -tags embed -trimpath -o bin/sub2api ./cmd/server
```

## 5. 风险与回滚点

- 若 dev Compose 仍会隐式创建应用容器，停止实施并修正 service 定义。
- 若 custom Compose 引用官方应用镜像或生产数据目录，停止实施并修正后再启动。
- 不使用 `down -v`、`volume prune` 或 `system prune`；只操作明确的 custom/test Compose 文件。
