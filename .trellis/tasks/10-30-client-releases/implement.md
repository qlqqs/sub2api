# 实施：sub2api 客户端版本发布

## 步骤

1. 迁移 263 和仓储，加仓储单测（unit tag）。
2. service：版本解析与比较（与客户端 `parse`、`Newer` 的规则一致，表驱动测试照抄客户端的用例）、release.json 校验、上传校验、发布与清理、撤回、删除，加单测。
3. handler 和路由：管理接口加 TOTP；公开接口 latest、notes、files 使用原始 JSON 和 `c.File`；接入 wire。加 handler 测试：Range、HEAD、草稿 404、路径中的非法版本号和文件名。
4. 前端页面、API、路由、两处侧边栏、中英文 i18n。
5. 本机验证（见下），把结果写进本任务的验收记录。

## 验证命令

```bash
cd backend && env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u all_proxy go test -tags unit ./...
cd frontend && npx -y pnpm@9 lint:check && npx -y pnpm@9 typecheck && npx -y pnpm@9 check:i18n && npx -y pnpm@9 test:run
cd /home/qlqq/workspace/sub2api-local && ./build.sh && docker compose up -d
```

## 本机链路（sub2api-local，127.0.0.1:28080）

1. 用 `.env` 中的管理员登录。把 qxapi-client 打包流程的产物放进一个新建的空目录（或用占位文件自己生成 release.json），通过管理页走完新建、上传、发布。
2. `curl` latest、notes、files（含 `-r` 分段下载和 `-I`），对比 sha256。
3. 篡改上传、缺文件发布、未登录访问、草稿下载：逐项确认被拒绝。
4. 连续发布 6 个版本，确认最老的被清理。
5. `docker compose up -d --force-recreate sub2api` 后再下载一次。
6. 管理页截图：中文和英文、浅色和深色主题、窄窗口。

## 审核点

- 提交、推送、部署七号机，每项另获用户同意。
- 部署后先在生产环境上传一个测试版本，确认下载速度和完整性，再上传正式的 0.1.0；上传正式版本也要另获同意。
