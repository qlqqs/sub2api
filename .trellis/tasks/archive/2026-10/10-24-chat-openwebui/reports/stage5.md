# 阶段 5 报告：sub2api 控制台前端（2026-10-24）

只改了 `frontend/`。没有动 `backend/`，没有执行 git 写操作，也没有碰本机容器。

## 改动文件

新增 `frontend/src/themes/qx/chat/`：

| 文件 | 作用 |
|---|---|
| `ChatView.vue` | `/chat` 页面。只有一个同源 iframe，无边框，占满内容区（`calc(100dvh - 4rem)`，宽度随侧栏收起和移动端变化，与 studio 相同）。状态有：加载中、错误（带重试）、未开启、其他标签页登录状态已变（带刷新按钮）、iframe。 |
| `chatBridge.ts` | contract E 的消息类型与纯函数：`parseChatMessage`（校验 origin 和 source）、`isAllowedChatNavigatePath`（白名单只有 `/purchase`、`/keys`）、`buildChatSession`、`toChatLocale`（zh→zh-CN，其余→en-US）、`chatFrameSrc`（`/_chat/?lang=<locale>`）。 |
| `chatSession.ts` | `POST /api/v1/chat/session`、`DELETE /api/v1/chat/session`（超时 5 秒）、`revokeChatSession()`（退出时调用）、`onChatRevoke()`（页面注册撤销回调）、`clearChatLocalData()` 和 `claimChatLocalData()`（同源存储里 `owui:` 前缀数据的清理与归属）。 |
| `chatStatus.ts` | `GET /api/v1/chat/status`。按页面生命周期缓存；请求失败按未启用处理（写法同 `studioStatus.ts`）。 |
| `locales/zh.ts`、`locales/en.ts` | 文案，合并到 `qxTheme.chat`。 |
| `__tests__/chatBridge.spec.ts`、`chatSession.spec.ts`、`ChatView.spec.ts`、`chatNav.spec.ts` | 共 25 个用例。 |

修改的上游 / 主题文件，都是一两行挂接，带 `CUSTOM` 注释：

| 文件 | 挂接点 |
|---|---|
| `src/router/index.ts` | 新路由 `/chat`（name `Chat`，`requiresAuth`，`titleKey: qxTheme.chat.title`，`qxFullBleed: true`），放在 `/studio` 之前。 |
| `src/stores/auth.ts` | `logout()` 开头 `await revokeChatSession()`。所有退出路径都经过这里（顶栏退出、管理员合规弹窗退出）。该调用永不抛错，不会阻止退出。 |
| `src/themes/qx/shell/useQxNav.ts` | CONSOLE 分区加 `{ path: '/chat', label: qxTheme.chat.title, icon: 'chat', featureFlag: flagChat }`，排在「API 密钥」之后、「创作工作台」之前。`flagChat` 在 status 返回 `enabled === true` 时才为真；未加载或失败都隐藏。 |
| `src/themes/qx/shell/navIcons.ts` | `chat: { icon: 'chat' }`。用 `Icon.vue` 已有的 heroicons outline 对话气泡，与 studio 的 `sparkles` 同一套线框风格。 |
| `src/themes/qx/locales/{zh,en}.ts` | 引入 `chat` 文案。 |

## 每项完成情况

1. **ChatView**：已完成。
   - 挂载后等 status 为 `true`，先 `POST /chat/session`。成功后才渲染 iframe；失败时显示「无法连接对话服务」和重试按钮，不渲染 iframe。
   - status 为 `false` 时，直接访问 `/chat` 显示「对话暂未开放。」，不发 POST。
   - iframe 设了 `allow="clipboard-read; clipboard-write; fullscreen"`。
2. **chatBridge**：已完成。
   - 父页只接受 `origin === location.origin` 且 `source === iframe.contentWindow` 的消息。发消息时 `targetOrigin = location.origin`。
   - 收到 `chat:ready` 回 `sub2api:chat-session {revision, theme, locale}`；iframe `load` 时也补发一次，无害。
   - 主题（`isDocumentDark()`）或语言变化时重发。语言变化不重载 iframe，`src` 只在首次加载和换账号时取当时的语言。
   - `authStore.token` 变化时重新 POST，成功后 revision + 1 并重发。
   - `chat:balance-changed` → `authStore.refreshUser()`，顶栏余额来自这里。
   - `chat:navigate` 只放行 `/purchase`、`/keys`，用 `router.push`。
3. **chatStatus**：已完成。入口隐藏用侧栏 featureFlag；直接访问时显示「未开启」。
4. **路由与导航**：已完成，见上表。
5. **退出 / 切换账号**：已完成，分三种情况：
   - 退出：`authStore.logout()` → `revokeChatSession()`，顺序是：
     1. 通知打开着的 ChatView：它先发 `sub2api:chat-revoke`，然后卸载 iframe；
     2. 清同源 localStorage / sessionStorage 里所有 `owui:` 键，以及归属标记 `s2a_chat_owner`；
     3. `DELETE /chat/session`。

     之后才走原有的 `authAPI.logout()` 和 `clearAuth()`。
   - 同页换账号（`user.id` 变化）：revoke → 清本地数据 → 为新用户重新 POST → 重载 iframe（`:key` 递增）。
   - 其他标签页退出或换账号（`storage` 事件、回到页面时的 `visibilitychange`）：比对 localStorage 里的 `auth_user.id` 和本页用户。不一致或 token 已清空时，revoke、卸载 iframe，并提示「登录状态已在其他页面变化，请刷新页面」。如果是同一用户、token 换了（别的标签页刷新了 token），只重新签发 Cookie。
   - 陈旧响应保护：每次签发和撤销都递增 `attempt`，撤销或换账号之后才回来的 POST 结果会被丢弃，不会加载 iframe。
6. **vitest**：已完成，见下节。

## 测试结果

```
cd frontend
npx -y pnpm@9 run lint:check   # 通过，无输出
npx -y pnpm@9 run typecheck    # 通过
npx -y pnpm@9 run test:run     # 412 个文件 / 3282 个用例全部通过
```

`git status` 确认：`pnpm-lock.yaml` 没有改动，没有生成 `pnpm-workspace.yaml`。

新增用例覆盖以下几组：
- 桥的 origin / source 校验：异源、`'null'` 源、非 iframe 来源、iframe 未就绪、非对象数据、未知类型、Nova 的消息类型。
- 导航白名单：拒绝 `/usage`、`/admin/users`、带查询串、尾斜杠、外链、`//`、`javascript:`、非字符串。
- 签发与 iframe 加载：先 POST 后加载 iframe；POST 失败不加载 iframe，重试后加载；status 未加载时不签发；未开启时不签发。
- 会话下发：ready 后回 session，targetOrigin 为同源；主题或语言变化时重发，且不重载 iframe；token 刷新后 revision + 1。
- 消息处理：余额变化触发刷新；导航只跳白名单。
- revoke：退出时先发 revoke，再清 `owui:` 数据、DELETE、卸载 iframe；DELETE 失败或回调抛错都不阻止退出；不清 `auth_token`、`theme` 等非 `owui:` 键。
- 换账号：同页换账号会重新签发并重载；本地数据归属于别的用户时先清空；其他标签页换账号时撤销并提示；陈旧响应被丢弃。
- 侧栏入口：status 关闭或请求失败时隐藏「对话」；开启时排在 `/keys` 之后、`/studio` 之前。

## 给本机联调的说明

- 前端只按 contract A / E 调用：
  - 后端路由已核对存在：`backend/internal/server/chat_proxy.go` 的 `GET /api/v1/chat/status`、`POST/DELETE /api/v1/chat/session`；
  - 前端经 `apiClient`（baseURL `/api/v1`，`withCredentials`）发请求，POST 会带 Bearer。
- 入口只在 qx 主题侧栏里出现，与 studio 相同。主题关闭时上游 `AppSidebar` 没有这个入口，但 `/chat` 路由仍可访问。
- 建议联调时检查以下几点：
  1. 没设 `CHAT_WEBUI_URL` 时，侧栏没有「对话」，直接访问 `/chat` 显示「对话暂未开放。」。
  2. 设了之后，进入 `/chat`，在 Network 里能看到 POST `/api/v1/chat/session` 先于 `/_chat/?lang=zh-CN` 发出，响应带 `Set-Cookie: s2a_chat…Path=/_chat`。
  3. 切换深浅色和语言时，iframe 能收到 `sub2api:chat-session`。确认 fork 的桥会响应 `locale` 变化，而不只认 `?lang`。
  4. 退出时能看到 DELETE `/api/v1/chat/session`；换账号后再进 `/chat`，看不到上一个用户的会话和草稿。
  5. 页面只有一层滚动，确认 `/chat` 下外壳 `<main>` 不出现滚动条。实现与 studio 相同，即 `qxFullBleed` 加 `calc(100dvh - 4rem)`。
- 余额刷新依赖 iframe 每轮回复结束发 `chat:balance-changed`。

## 已知问题 / 限制

- token 刷新的覆盖范围：
  - `authStore` 的主动刷新（到期前 120 秒）会更新 `authStore.token`，能触发重新签发。
  - `apiClient` 拦截器在 401 时做的被动刷新只写 localStorage，不更新 store。这种情况由 `visibilitychange` 和其他标签页的 `storage` 事件比对兜底，同一标签页内不一定立刻重签。
  - 正常使用时主动刷新先发生，影响很小。如需更稳，可以让 iframe 在收到 401 时发一条消息请求父页重签，见提议 2。
- 主动清理只覆盖 localStorage / sessionStorage。iframe 如果还用了 IndexedDB 或 Cache Storage，父页不清，需要 fork 在 revoke 时自行处理，或确认 Open WebUI 不用它们存用户数据。
- 离开 `/chat`（切到别的控制台页面）不发 revoke、不清 Cookie：同一用户回来时，草稿和设置会保留。只有退出和换账号时才撤销。这与 studio 不同：studio 离开页面也会撤销。
- 如果令牌失效被拦截器整页跳回 `/login`，不会走 `logout()`，Cookie 会留着，其中的 token 本身已失效。下一个用户进入 `/chat` 时，`claimChatLocalData` 会先清掉上一个用户的 `owui:` 数据，POST 再覆盖 Cookie。

## 约定变更提议

1. **contract E 补充父页的同源存储清理**：
   - 父页与 iframe 同源，共用 localStorage / sessionStorage。本实现在退出、换账号、检测到归属用户不同时，由父页直接删除所有 `owui:` 键，并用 `s2a_chat_owner` 记录本地数据属于哪个用户。
   - 这是对 iframe 侧 revoke 清理的冗余兜底：iframe 收到 revoke 前可能已被卸载，也可能根本没打开。建议写进 contract E，并要求 fork 侧：
     - 存储键前缀固定为 `owui:`，不在前缀外写用户数据；
     - 不使用 `s2a_` 前缀的键。
2. **（可选）增加 `chat:session-expired`（iframe → 父）**：iframe 收到 sub2api 的 401（Cookie 失效）时发这条消息，父页重新 POST session 后回 `sub2api:chat-session`（revision + 1）。这样拦截器被动刷新 token 的情况也能立即恢复。本阶段没有实现，等主会话决定。
3. **说明 locale 的动态切换**：E 节已规定 `sub2api:chat-session` 带 `locale`，父页在语言变化时只发消息、不重载 iframe。请确认 fork 的桥收到不同的 `locale` 时会切换界面语言。

## 补充（联调反馈，按更新后的 contract A / E 节）

### 改动

1. **iframe src 带 theme**：
   - `chatFrameSrc(locale, theme)` 生成 `/_chat/?lang=<zh-CN|en-US>&theme=<light|dark>`，theme 取加载那一刻的控制台深浅色（`isDocumentDark()`）。
   - 之后主题或语言变化仍只经 `sub2api:chat-session` 下发，不重载 iframe。
2. **账号变化时先 DELETE 再 POST**：
   - 新增 `chatOwnedByOther(userId)`：localStorage 里的 `s2a_chat_owner` 存在，且不是当前用户，才算账号变化。没有记录不算，因为那是首次使用，或者正常退出时已经清掉并 DELETE 过。
   - `issue(reset)`：reset 为真时先 `DELETE /api/v1/chat/session`，再 `POST`。DELETE 失败按签发失败处理：显示错误、不 POST、不加载 iframe。
   - 归属标记只在签发成功后更新，所以点重试仍会先 DELETE。
   - 覆盖两个场景：
     - 同页换账号：归属标记仍是旧用户，于是 revoke 之后先 DELETE，再 POST，再重载 iframe；
     - 加载前发现本地数据属于别的用户：先 DELETE 再 POST，然后清 `owui:` 数据，再加载 iframe。
   - 普通首次加载、同一用户再次进入、token 刷新都只 POST。
3. **`chat:session-expired`**（新版 contract E 已收录，原报告的提议 2）：
   - 父页校验 origin 和 source 后重新 POST（不 DELETE），成功后 revision + 1 并回 `sub2api:chat-session`。
   - 一次重签还没结束时，后续的 expired 消息直接忽略，避免 iframe 连续报错时反复请求。

改动文件：`themes/qx/chat/chatBridge.ts`、`chatSession.ts`、`ChatView.vue` 及对应的三个测试文件。上游挂接点没有新增改动。

### 测试

新增和更新的用例（chat 目录现在共 30 个）：
- src 带 `theme=light`，深色加英文时为 `lang=en-US&theme=dark`；语言和主题变化后 src 不变。
- 首次加载和 token 刷新不调 DELETE。
- 同页换账号：DELETE 在旧 POST 之后、新 POST 之前（用 `invocationCallOrder` 断言）。
- 本地数据属于别的用户：先 DELETE 再 POST，清空本地数据后加载 iframe。
- 账号变化时 DELETE 失败：显示错误、不 POST、不加载、归属标记不变；重试时再次 DELETE，成功后加载。
- 同一用户再次进入：不 DELETE，保留本地数据。
- `chat:session-expired`：伪造来源被忽略；重签一次且 revision 变为 2；重签进行中重复收到的消息被合并；重签结束后再收到会再签一次。
- `chatOwnedByOther` 的三种情况。

三条命令结果：

```
npx -y pnpm@9 run lint:check   # 通过
npx -y pnpm@9 run typecheck    # 通过
npx -y pnpm@9 run test:run     # 412 个文件 / 3287 个用例全部通过（输出里的 "Failed to select all account results" 是已有用例故意打印的 stderr，不是失败）
```

`git status` 确认：`pnpm-lock.yaml` 没有改动，没有生成 `pnpm-workspace.yaml`。

### 给联调的补充说明

- 换账号场景下，Network 里应该依次看到 `DELETE /api/v1/chat/session` → `POST /api/v1/chat/session` → `/_chat/?lang=…&theme=…`。DELETE 的响应应同时清掉 `s2a_chat` 和 Open WebUI 的 `token`，两个都是 `Path=/_chat`、`Max-Age=0`。
- 发 `chat:session-expired` 后最多重试一次，由 iframe 侧负责；父页只做重签。
