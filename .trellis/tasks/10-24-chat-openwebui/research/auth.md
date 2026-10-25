# 调研：Open WebUI v0.12.3 认证与身份（2026-10-24）

源码：`~/workspace/open-webui`（tag v0.12.3，HEAD 91ea4e3），路径相对 `backend/open_webui/` 或仓库根。

## 1. 可信头登录

- 环境变量 `env.py:808-811`：`WEBUI_AUTH_TRUSTED_EMAIL_HEADER` / `_NAME_` / `_GROUPS_` / **`_ROLE_HEADER`**。
- 只在 `POST /api/v1/auths/signin` 完整处理（`routers/auths.py:656-781`）：无邮箱头 400；邮箱小写；不存在则 `signup_handler` 建号（随机密码）；GROUPS 只同步到已存在的组；ROLE 为 admin/user/pending 时覆盖库内角色；签发会话。
- 每个请求只做邮箱一致性检查（`utils/auth.py:458-464`）：JWT 用户邮箱与头不一致 → 401。**头缺失时不检查；角色 / 组不重新同步。**
- 可信头模式禁用改密码（`routers/auths.py:321-322`）；前端 `/api/config` 下发 `auth_trusted_header`（`main.py:2410`），`/auth` 页自动 signin（`src/routes/auth/+page.svelte:227-228,278-291`）。
- 新用户先按 `ui.default_user_role`（默认 `pending`，`config.py:1748`）入库；**总用户数为 1 时提升为 admin 并关闭注册**（`routers/auths.py:824-827`），首个用户是「主管理员」，其他管理员不能改它（`routers/users.py:945-961`）。
- 邮箱变化会新建账号 → 用合成邮箱 `u<id>@sub2api.internal`。
- 坑：`ENABLE_PASSWORD_AUTH=false` 在可信头分支之前就拒绝 signin（`routers/auths.py:663-667`），不能设。

## 2. 会话

- JWT HS256（`WEBUI_SECRET_KEY`），声明 `id`、`typ:'session'`、`auth_method`、`session_stamp`、`mfa_verified`、`jti`、`iat`（`utils/auth.py:47-48,228-239,721-731`）；有效期默认 `4w`（`config.py:2546`）。
- **配置持久化**：数据库 `config` 表优先于环境变量（`models/config.py:143-149`）；`ENABLE_PERSISTENT_CONFIG=false` 强制以环境变量为准（`config.py:3334-3340`）。
- 前端双存：`localStorage.token`（所有 API 调用显式 `Authorization: Bearer`，481 处 / 32 文件）+ httponly Cookie `token`（`utils/auth.py:732-740`，`GET /auths/` 每次刷新）。
- `get_current_user`（`utils/auth.py:389-503`）取凭据顺序：Bearer → Cookie `token` → `request.state.token`（`asgi_middleware.py:126-132`，含 `x-api-key`）；`sk-` 按 API Key；否则解码 JWT + `is_valid_token`（session_stamp、typ、MFA、Redis jti 撤销，`:272-322`）。
- `get_verified_user`（user/admin，`:564-573`）、`get_admin_user`（`:622-628`）、`get_human_user`（要求 `auth_type=='jwt'` 且 `typ=='session'`，`:763-766`）。
- socket.io：前端 `auth: {token: localStorage.token}`、仅 websocket（`src/routes/+layout.svelte:180-188`）；后端 `connect` / `user-join` 用 `get_verified_user_by_token`（`socket/main.py:550-606`），每 30 秒复检（`:408-430`）；**不读 HTTP 头 / Cookie**。实时语音 `routers/audio/realtime.py:375`、终端 `routers/terminals.py:304` 同样走 token。

## 3. 登出与撤销

- `POST /api/v1/auths/signout`（`routers/auths.py:903-1007`）：`invalidate_token` 写 Redis jti（**无 Redis 不生效**），删 Cookie，可返回 `WEBUI_AUTH_SIGNOUT_REDIRECT_URL`。
- `revoke_user_tokens`：轮换 `auth.session_stamp`（存库，不依赖 Redis）并断开 socket（`utils/auth.py:359-367`、`models/auths.py:258-280`）；管理员接口 `POST /api/v1/users/{id}/sessions/revoke`（`routers/users.py:921-934`）；`disconnect_user_sessions`（`socket/main.py:512-533`）。

## 4. 嵌入模式切入点

前提：
- uvicorn `forwarded_allow_ips` 默认 `'*'`（`backend/open_webui/__init__.py:90,110`、`backend/start.sh:74,111`）→ 任何对端的 XFF 都会改写 `scope['client']`。需 `--no-proxy-headers`（或空 `FORWARDED_ALLOW_IPS`），自写中间件先按真实对端判 CIDR 再解析 XFF（uvicorn 0.51.0 行为未验证）。
- 凭据冲突：Open WebUI 占用 `Authorization` 与 `token` Cookie；`<img>`（头像 `/api/v1/users/{id}/profile/image`）与 WebSocket 升级无法带自定义头 → sub2api 会话需同时以**同源 Cookie（如 `s2a_session`，path `/_chat`）**与 fetch 头（`X-Sub2API-Session`，在 `+layout.svelte:1147` 已有的 `window.fetch` 覆写处加）提供给代理。

方案 B（侵入最小）：复用内置可信头 + 一次性换 JWT
1. 新增纯 ASGI 中间件（`main.py:861` 附近，http + websocket）：非可信对端删除 `X-Sub2API-*`；可信对端校验 User-Id 正整数、Role ∈ {admin,user}，注入 `X-Sub2API-Email: u<id>@sub2api.internal`。
2. 环境变量：`WEBUI_AUTH_TRUSTED_EMAIL_HEADER=X-Sub2API-Email`、`WEBUI_AUTH_TRUSTED_ROLE_HEADER=X-Sub2API-User-Role`、`DEFAULT_USER_ROLE=user`、`ENABLE_SIGNUP=false`、`ENABLE_LOGIN_FORM=false`、`ENABLE_PASSWORD_CHANGE_FORM=false`、`ENABLE_PERSISTENT_CONFIG=false`。
3. 补丁：`utils/auth.py:458-464` 头缺失也拒绝 + 每请求按角色头同步角色；`routers/auths.py:824-827` 去掉首用户自动 admin。
4. 前端基本不改：/auth 自动登录；头不一致 401 → `clearExpiredSession`（`+layout.svelte:997-1016`）→ 重新登录。隐藏登出 / Account。
代价：仍签发最长 4 周的 Open WebUI JWT（无代理 Cookie 不可用，且受邮箱一致性约束）。

方案 A（更彻底）：每请求按头认身份
- 改 `get_current_user`（`utils/auth.py:389`）、`get_optional_verified_user_from_request`（`:600-619`）、`/api/config` 内的 token 解码（`main.py:2280-2305`）、`GET /auths/`（`routers/auths.py:176-238`）、前端无 token 时不取会话用户（`+layout.svelte:1358`）、socket connect / user-join / 复检（`socket/main.py:408-430,551-606`，改读 `environ['asgi.scope']` 头）、`realtime.py:375`、`terminals.py:304`。
- 必须保留 JWT 分支：自动化任务、子代理内部伪请求靠 `typ:automation/subagent` JWT（`utils/automations.py:402-405`、`utils/subagents.py:55-69`）。
- 6～8 处改动。

## 5. 用户模型与权限

- `user` 表（`models/users.py:145-176`）：`id` 为 **UUID 字符串**（`models/auths.py:301`），`email` 唯一、`role` 默认 pending、`info` / `settings` JSON 等。→ sub2api ID 经合成邮箱映射（或存 `info`）。
- `auth` 表（`models/auths.py:30-37`）：email、password、active、mfa、session_stamp。
- `DEFAULT_USER_PERMISSIONS`（`config.py:1972-2055`）可用 `USER_PERMISSIONS_*` 覆盖；多组权限取并集；admin 通常跳过权限检查；`DEFAULT_GROUP_ID` 新用户自动入组（`routers/auths.py:829-833`）。

## 6. 其他认证路径

| 路径 | 位置 | 默认 | 关闭 |
|---|---|---|---|
| API Key（`sk-` / `x-api-key`） | `utils/auth.py:415,506-533`；创建 `routers/auths.py:1497` | `ENABLE_API_KEYS=False`（`config.py:2530`） | 保持；`USER_PERMISSIONS_FEATURES_API_KEYS=false` |
| 密码登录 | `routers/auths.py:759-776` | 开 | 可信头分支优先（`:672`）；不能设 `ENABLE_PASSWORD_AUTH=false` |
| 注册 | `routers/auths.py:847-900` | — | `ENABLE_SIGNUP=false` + `ENABLE_LOGIN_FORM=false` + `ENABLE_INITIAL_ADMIN_SIGNUP=false`（`env.py:759`） |
| 管理员添加用户（直接返回 JWT） | `routers/auths.py:1049-1116` | 仅 admin | 无开关 → 补丁或代理层屏蔽 |
| 管理员改邮箱 / 密码 / 角色 | `routers/users.py:937-1063` | 仅 admin | 改邮箱会破坏映射 → 补丁禁止 |
| OAuth | `main.py:2927-2947`、`utils/oauth.py:1909-1935` | 需配置 provider | `ENABLE_OAUTH=false`，不配 `*_CLIENT_ID` |
| OAuth token 交换 / 后通道登出 | `routers/auths.py:1592`、`main.py:2957` | False | 保持 |
| LDAP | `routers/auths.py:408-419` | false | 保持 + `ENABLE_PERSISTENT_CONFIG=false` |
| SCIM | `main.py:922-923` | False | 保持 |
| 启动建管理员 | `main.py:400-404` | 需 `WEBUI_ADMIN_*` | 不设 |
| `WEBUI_AUTH=False` | `routers/auths.py:728-758` | True | 保持（需 `WEBUI_SECRET_KEY`） |
| MFA | `utils/mfa.py:52-57` | False | 保持 |

补充：`XFRAME_OPTIONS` 未设时不发（`utils/security_headers.py:51-57,71-77`）；`CORS_ALLOW_ORIGIN` 默认 `*` 且 `allow_credentials=True`（`config.py:2179`、`main.py:864-870`）→ 收紧。未逐个审计所有 router 是否都经 `get_current_user`。
