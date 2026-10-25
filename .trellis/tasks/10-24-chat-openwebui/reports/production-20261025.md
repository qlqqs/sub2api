# 对话工作台修复发布与一号机复测

记录时间（UTC）：2026-10-25T05:09:29+00:00

## 授权与发布

用户在本地验证汇报后回复“同意”，批准提交/推送、更新一号机配套对话工作台、少量真实模型请求及测试凭据清理。

- fork：`59fd038`（生图 b64 优先/按 Key 选模型）+ `126b01b`（模型价格两行排版）。
- GitHub Custom Image run `40163659961`：backend-test、frontend-test、image 均 success。
- 一号机目录：`/root/sub2api-custom-chat`，只重建 compose 服务 `webui`。
- 拉取不可变镜像 tag `ghcr.io/qlqqs/open-webui:custom-126b01b`，核验 OCI revision 为 `126b01b6cc02210bb1f6c1e6bce6ab5fba562a18` 后再标记本地 custom-latest 并启动。
- digest：`sha256:1700643826a8c7f19e3b8f75180ffa144fd779ea6a2792b22af32188f2dd5175`；容器 healthy，复测后约 276.7 MiB 内存。
- sub2api-custom 主应用、配套 Nova、出站代理的 image 与 StartedAt 前后完全相同，没有重启或改动其他部署。

## 回滚

已在一号机保留 `ghcr.io/qlqqs/open-webui:rollback-before-image-fix-20261025`（旧 revision `66fc695`），以及 `/root/sub2api-custom-backups/chat-compose.before-image-fix-*` 与权限 600 的 env 备份。

必要时将回滚镜像重新标记为本地 custom-latest，再在 chat 目录执行 `docker compose up -d --no-deps --pull never webui`。不要重新 pull latest 覆盖回滚；无需重启主应用。

## 真机验证（真实浏览器，无模型/价格 fixture 注入）

- 入口首页、`/chat`、`/studio` 返回 200；chat status enabled；匿名 chat API 返回 401，仍带严格 CSP。
- 测试用户所选 Key 模型列表剩 7 个聊天模型，内部模型与生图模型不出现在聊天下拉；名称/价格两行，每行 44px。
- 原生搜索返回 Open WebUI 官方 GitHub 仓库，并显示“1 个引用来源”。
- 对话内生图成功显示 **1024×1024** 图片，URL 是 `/_chat/api/v1/files/.../content`，浏览器 naturalWidth/naturalHeight 非零；刷新会话后仍可显示。
- 深色跟随控制台，切回浅色正常。
- 0 路径逃逸、0 外站请求、0 主文档 CSP 违规、0 pageerror；WebSocket 正常传输。

## 实际计费

测试用户 ID 12、专用 Key ID 14 的本次新增记录共 **6 条**（搜索、工具循环、标题及生图）。

- 生图记录恰好 **1 条 / 1 张**，模型 `gpt-image-2`，费用 **$0.15**。
- 本次新增记录合计 **$0.157474992**。
- 全部记录归属所选用户与 Key，客户端 IP 为公网 IP，非容器地址。
- 统计只包括本次补丁上线后的复测，不包含上一会话的失败请求费用。

## 清理与边界

- 只撤销 `chat-real-test`（Key 14），其余 Key ID 列表前后相同；数据库确认此 Key 已软删除。
- 清除该测试用户 refresh sessions，通过正常改密接口将测试临时密码轮换为未保存随机值，使旧 access token 指纹失效。
- 实测旧密码登录拒绝、旧 access token 401、旧 chat Cookie 401、被删除 Key 调网关 401、旧 refresh token 401；DELETE chat/session 清理两种 Cookie。
- 不删除测试用户、不删除聊天/计费记录、不更改其他用户，也未运行会修改支付/返利设置的历史远端 cleanup。
- 本机六个临时凭据/helper 文件 `.upw/.tok/.refresh/.sess/ask.sh/r.sh` 已按精确路径删除，未改动用户的集中凭据文件。

## 证据与剩余门槛

持久证据：`~/workspace/owui-poc/chat-production-final/`（截图、UI JSON、费用摘要、清理结果、容器状态；不含登录凭据）。

本地回归：438 后端测试、65 前端测试、27 项 p9、8 步浏览器、完整 p0–p9 336/336；类型检查仍有上游 5696 个既有错误，但新错误为 0。

**发布、真机复测与清理已完成。** 主会话已查看并展示 `01-models-light.png` 与 `08-image-reloaded.png`。PRD 要求用户最终视觉确认，尚未收到独立的截图反馈，因此保留任务 in_progress，等待反馈后归档，不虚报已获视觉确认。
