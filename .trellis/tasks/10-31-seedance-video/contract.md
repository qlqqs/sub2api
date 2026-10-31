# 接口约定（所有代理以此为准）

## C1 分组分润系数（BE-S ↔ FE-S）

**设置项**：`affiliate_group_rebate_factors`

- 存在 settings 表里，值为 JSON 数组。
- 管理端 `GET /api/v1/admin/settings` 和 `PUT /api/v1/admin/settings` 增加同名字段：

  ```json
  "affiliate_group_rebate_factors": [ { "group_id": 12, "factor": 30 } ]
  ```

**字段规则**

- `factor` 是百分比，取值 0–100，允许小数，保存时保留 2 位。
  - 0：该分组不分润。
  - 100：与未设置相同。保存时去掉 factor=100 的条目，减少噪声。
- `group_id` 必须是存在的分组。
  - 后端保存时校验：分组不存在、factor 越界、group_id 重复，一律返回 400。
  - PUT 不带这个字段时，保持原值不变（沿用 settings 的 `*T` 指针更新风格）。
- GET 返回数组时按 `group_id` 升序排列，元素里附带 `group_name`（只读，前端展示用；PUT 时忽略）。

**团队请求明细**

- 两个接口：
  - 用户端 `/api/v1/user/aff/team/customers/:user_id/requests`
  - 管理端查看器对应接口
- 每一行增加 `rebate_factor`（number，百分比）。
  - 按该行所在分组的**当前**系数计算，没有设置的分组为 100。
  - 删除了分组或 `group_id` 为空的行，也是 100。
- 前端只在 `rebate_factor !== 100` 时显示标签「分润 X%」。

**生效范围**

- 只影响「返利基数 = 消费」模式下的消费结算。充值基数模式不受影响。

## C2 Seedance 计费（BE-S 内部；QA 依赖）

**计费秒数**

- 无参考视频：计费秒数 = 成片秒数。
- 有参考视频（建任务时 `content` 里存在 `type=video_url` 的条目）：计费秒数从上游成功响应的 `usage.completion_tokens` 反推：
  - `frames = completion_tokens × 1024 / (w × h)`
  - `w × h` 取自尺寸表，按状态响应里的 `resolution` 和 `ratio` 查找。
  - `billed = round((frames − 1) / 24)`
  - 结果小于成片秒数时，按成片秒数计。
- 计费秒数上限为 60。

**usage_logs 字段**

- `video_duration_seconds` 记录计费秒数。
- `output_tokens` 照旧记录 `completion_tokens`，用于对账。

**上游模拟**（`upstream_mock.py`，QA 和 BE-S 共用）

- 成功响应的 `completion_tokens` 按同一公式生成：
  - `w×h×(out×24+1+refBilled×24)/1024`，向下取整。
  - `refBilled` = `max(floor(参考视频总秒数), ceil(out×2/3))`。
  - 参考视频秒数由提示词里的 `E2E_REF_SECONDS=<浮点数>` 指定；没有参考视频时取 0。
- 响应带 `resolution`、`ratio`、`duration`。
  - `ratio` 取请求值；`adaptive` 或缺省时返回 `16:9`。

## C3 参考素材公开链接（BE-N ↔ BE-S ↔ FE-N）

**链接格式**

```
<NOVA_PUBLIC_BASE_URL>/api/nova/public-media/<file>?exp=<unix秒>&sig=<hex>
```

- `sig = HMAC-SHA256(NOVA_MEDIA_SIGNING_SECRET, "<file>\n<exp>")`，以 hex 编码。
- 有效期 24 小时。

**Nova 配置**

- `NOVA_PUBLIC_BASE_URL`：生产为 `https://www.qlqqx.com`，本机为 `http://127.0.0.1:28080`。
- `NOVA_MEDIA_SIGNING_SECRET`：
  - 不配置时，自动生成并持久化在数据目录（如 `<data>/media-signing.key`，权限 600）。
  - 嵌入模式下，`NOVA_PUBLIC_BASE_URL` 必须配置。

**sub2api 转发**

- 对 `GET` / `HEAD` `/api/nova/public-media/*`，**不做 JWT 校验**，直接转发给 Nova。
- 转发前删除客户端带来的全部 `X-Sub2API-*` 头，不注入身份头。
- 响应头增加 `Cache-Control: private, max-age=3600`；其余响应头沿用 Nova 返回的。
- 其他 `/api/nova/*` 路径行为不变。

**Nova 处理**

- 嵌入模式下，这条路径免除 `requireUser`，只校验签名和过期时间。
- 签名错误、过期、文件不存在，一律返回 404。
- 支持 `Range` 请求（视频拉取会用到）。

**谁使用这个链接**

- Nova 后端构造上游请求时，把用户上传素材的内部地址替换成签名公开链接。
- 用户填写的公网链接（`https://`）原样透传，不经过 Nova。

## C4 Nova 插件任务（嵌入模式，BE-N ↔ FE-N）

**开放的接口**

- 嵌入模式开放 `/api/nova/plugin-tasks*` 和 `/api/nova/plugin-media`（上传 `POST`）。
  - 两者都要求 `requireUser`。
  - 插件任务记录 `user_id`，按 `ownsTask` 校验归属；不是本人的任务一律返回 404。
- `/api/nova/plugin-media/<file>` 的匿名 `GET` 被 sub2api 的 JWT 层拒绝（401，到不了 Nova）；带登录态访问这个原始地址时 Nova 返回 404。
  - 前端预览自己上传的素材时，使用新增的 `GET /api/nova/plugin-media/<file>/preview`：要求 `requireUser`，并且素材必须属于本人。
  - 也可以直接用本地 blob 预览，二选一，由 BE-N 决定，并在完成报告中写明。
- `/cdp` 仍然返回 404。

**上游地址与密钥**

- 嵌入模式下，插件的 baseUrl 一律固定为 `SUB2API_GATEWAY_URL`（qx-seedance 的路径前缀是网关根，网关提供 `/api/v3/contents/generations/tasks`）。
  - 忽略请求体和凭据里的 baseUrl。
  - 请求带 `X-Forwarded-For: <用户 IP>`。
  - 禁止自动重定向（与图片任务一致）。
- `apiKey` 由前端随 `POST /api/nova/plugin-tasks` 提供，是用户在设置里给「视频」选择的 sub2api Key 明文（与现有图片 Key 的取法相同）。
  - 只保存在内存中，与现状一致。

**任务结果**

- 成功后，Nova 后端把上游 `content.video_url` 下载到本地。`assets` 里的 URL 改为 Nova 的鉴权地址 `/api/nova/plugin-results/<taskId>/<n>`：
  - 要求 `requireUser` 并校验归属；
  - 响应头 `private, no-store`；
  - 支持 Range。
- 前端用带会话的 fetch 取得 blob 后播放和下载，不要直接用 `<video src>` 指向裸地址。
- 如果嵌入模式下视频走 `fetch → blob`，CSP 需要 `media-src 'self' blob:`。sub2api 的 studio CSP 由 BE-S 增加这一项。

**状态与保留**

- 任务状态：`queued → processing → completed / failed`，沿用现有插件任务的序列化格式。
- 轮询上限：12 小时。
- 视频任务保留期由 `NOVA_VIDEO_RETENTION_DAYS` 控制，默认 3 天，从终态开始计算。图片任务仍用 `NOVA_RETENTION_DAYS`（7 天）。
- 上传素材随任务一起清理；未绑定任务的素材 24 小时后清理。

**价格预估（FE-N）**

- 前端从 sub2api 用户接口读取当前视频 Key 所在分组的模型价格：模型广场接口，按秒的视频价格，金额已经是 ¥。
- 预估 = 单价 × 预估计费秒数（含参考视频规则；参考视频总秒数取前端读取的时长；`-1` 按模型上限）。
- 使用哪个接口由 FE-N 读 sub2api 代码确定。如果需要后端新增，写在任务书「约定变更」里。

## C5 qx-seedance 插件定义（BE-N 编写，FE-N 渲染）

**基本信息**

- id `qx-seedance`，名称「Seedance 视频」，kind `video`，outputs `["video"]`。
- credential 的 source 为 client，嵌入模式下由 sub2api Key 提供。
- 运行参数：
  - `runtime.maxPollMs` = 12 小时
  - `pollIntervalMs` = 10000

**模型**

- 两个模型，id 用对外模型名：
  - `seedance-2.0`：名称 Seedance 2.0
  - `seedance-2.5`：名称 Seedance 2.5
- 生产账号的 `model_mapping` 会把这两个名字映射到 mmgate 的真实模型 ID。
- 价格字段写 `per-second`，CNY，分别为 0.75 和 1.125。仅作展示兜底，实际以分组价格为准。

**生成方式**（facet `mode`）

| mode | 名称 | content 组成 |
|---|---|---|
| `t2v` | 文生视频 | text |
| `i2v` | 图生视频 | text + 1 张 `image_url`（role `first_frame`） |
| `firstlast` | 首尾帧 | text + `first_frame` + `last_frame` 两张图 |
| `reference` | 参考生成 | text + 参考图（role `reference_image`，≤9）+ 参考视频（role `reference_video`，2.0 ≤3 / 2.5 ≤10）+ 参考音频（role `reference_audio`，≤3），至少一项素材 |
| `edit` | 视频编辑 | text + 1 个 `reference_video`；`omni_reference_task_type: "edit"` |
| `extend` | 视频延长 | text + 1 个 `reference_video`；`omni_reference_task_type: "extend"` |

**参数**

- `resolution`：480p / 720p / 1080p，默认 720p。
- `ratio`：adaptive / 16:9 / 4:3 / 1:1 / 3:4 / 9:16 / 21:9，默认 adaptive。
- `duration`：
  - 2.0：4–15，默认 5；
  - 2.5：4–30，默认 5；
  - 2.5 另有「自动」选项，传 -1。
  - edit 模式不显示时长（成片与原视频等长），不传 duration。
- `generate_audio`：布尔，默认 true。
- `watermark`：false，固定值，不显示。

**content 字段**

- role 和 `omni_reference_task_type` 的实际取值，以 mmgate 文档 https://mmgate.cn/docs/seedance/create 为准。BE-N 写插件前用 Playwright 渲染文档核对（环境说明见 `/home/qlqq/workspace/AGENTS.md`），并在完成报告中写明核对结果。
- 以上表格是按记忆整理的，与文档不一致时以文档为准。

**参考视频总时长限制**

- 2.0 ≤15 秒，2.5 ≤30 秒。
- 前端校验：上传的视频读取时长；公网链接无法读取时长时不校验，由上游报错。
