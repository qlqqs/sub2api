# BE-N：Nova 后端任务书（codex gpt-6.1-sol high）

仓库 `/home/qlqq/workspace/nova-image-studio`，分支 `sub2api-custom`。

负责范围：

- `backend/`
- `docker/`
- `.gitignore`
- `docs/sub2api-embed.md`、`docs/plugins/*`
- 本机环境 `~/workspace/sub2api-local-nova/`（compose 和 env）

**不改 `frontend/`**：前端由 FE-N 负责。仓库里已有未提交的 `docs/sub2api-embed.md` 改动（七号机部署说明），在它的基础上继续改，不要回滚。

开始前按 `prd.md` 的「共同规则」读取文档，重点读 `docs/sub2api-embed.md` 和 `docs/plugins/*.md`（manifest、provider、ui-schema、testing）。

接口以 `contract.md` 的 C3、C4、C5 为准。

---

## 1. 删除 ccode-h3

- 删除 `backend/plugins/ccode-h3/` 整个目录。
- `.gitignore` 的 `!backend/plugins/ccode-h3/` 改成 `!backend/plugins/qx-seedance/`。
- 清理后端、测试和文档中**专属于 ccode-h3 插件**的引用：
  - `backend/plugin-runtime/verify.js`
  - `backend/test/plugin-runtime.test.js`
  - `docs/plugins/*.md`
  - `README*.md`
  - 测试里用到 ccode-h3 fixtures 的地方，改用 qx-seedance 的 fixtures。
- 不要动名字带 ccode 但与该插件无关的核心功能，例如 `frontend/src/lib/ccode-task-socket.ts`，前端也不归你。
- grep 后列出剩余的 `ccode-h3` 字符串，写进完成报告，交给 FE-N 处理前端那部分。

## 2. 新增 qx-seedance 插件（C5）

- 目录：`backend/plugins/qx-seedance/`，包含：
  - `manifest.json`
  - `provider.json`（声明式）或 `index.js`（脚本式）
  - `ui.schema.json`
  - `README.md`
  - `fixtures/`
- 声明式还是脚本式由你决定：
  - 声明式模板只有 `$switch`、`concat`，没有 map，要拼出 Ark 的 content 数组，需要每个槽位写一个 `$switch`。
  - 六种模式加上多个参考素材，用脚本式（`buildSubmit` / `parsePoll` 等，见 `docs/plugins/provider.md`）大概率更清晰。
- 开工前用 Playwright 渲染 https://mmgate.cn/docs/seedance/create，核对以下内容的准确取值，结果写进完成报告：
  - content 的 role
  - `omni_reference_task_type`
  - `duration`、`ratio`、`resolution`、`generate_audio`、`watermark`
  - 2.0 / 2.5 的限制
  - 环境说明见 `/home/qlqq/workspace/AGENTS.md`。之前渲染过的结果在 `/tmp/mmgate-scrape/s.js`，可以参考。
- 上游接口：
  - 提交：`POST {base}/api/v3/contents/generations/tasks`
  - 轮询：`GET {base}/api/v3/contents/generations/tasks/{id}`
  - 状态映射：
    - `queued` / `running` → processing
    - `succeeded` → completed
    - `failed` / `cancelled` / `expired` → failed
    - 错误信息取 `error.message`。
  - 结果：`content.video_url`，有些情况还有 `content.last_frame_url`，按需读取。
- 素材：
  - 用户上传的素材，在提交时替换为 C3 的签名公开链接。
  - 用户填写的 `https://` 链接原样透传。
  - 素材字段要能同时接受这两种来源。如果插件 schema 现在不支持链接输入，就扩展 runtime 的 `input.js` 和 `validate.js`，并告诉 FE-N 字段格式。
- `manifest.permissions.hosts`：嵌入模式下 baseUrl 固定为网关，出网闸门需要放行网关 origin。看现有 `assertUrlAllowed` 是怎么处理嵌入模式的，按需调整。
- fixtures：每种模式至少一个提交 body 快照，由 `plugin-runtime.test.js` 校验。

## 3. 嵌入模式开放插件任务（C4）

修改 `backend/server.js` 中 `handleApi` 的嵌入分支：

- 去掉对 `plugin-tasks`、`plugin-media` 的 404。`/cdp` 仍然返回 404。
- 插件任务写入 `tasks.user_id`：
  - `GET /api/nova/plugin-tasks/:id`、`/ack` 按 `ownsTask` 和过期时间校验，不满足时返回 404，与内置任务一致。
- 素材上传记录归属用户。预览接口按 C4 实现，或者说明前端改用本地 blob。
- 匿名 `GET /api/nova/plugin-media/<file>` 在嵌入模式下返回 404。

固定网关与限流：

- 嵌入模式下 `createPluginTask` 使用 `SUB2API_GATEWAY_URL`，忽略 `body.baseUrl` 和 `credential.defaultBaseUrl`。
- 提交和轮询请求带 `X-Forwarded-For: <用户 IP>`，禁止自动重定向，与图片任务的做法一致（参考 `docs/sub2api-embed.md` 第 6 节和现有图片网关请求代码）。
- 嵌入模式关闭按 IP 和 apiKeyHash 的限流，与图片任务一致，保留全局并发。

轮询与超时：

- 任务的后端轮询一直进行到终态。
- `maxPollMs` 取插件声明的 12 小时。
- 注意 `TASK_TTL_MS`：处理中的任务的 `expires_at` 不能早于轮询结束时间，否则会被清理掉。

结果下载：

- 成功后下载 `video_url` 到 `NOVA_IMAGE_DIR` 或新的 `NOVA_VIDEO_DIR`，由你决定，并写进 docs。
- 只允许 https。本机 e2e 用环境变量 `NOVA_RESULT_DOWNLOAD_ALLOW_HTTP=1` 放开 http，只写在本机 compose 中。
- 下载限制：
  - 最多跟随 3 次重定向，每一跳都必须满足协议规则；
  - 单个文件上限 1GB；
  - 超时 10 分钟。
- 下载失败时，任务标记为失败，错误信息说明上游已经生成但下载失败，并附上上游任务 ID，方便人工核对。
  - 此时网关已经计费，所以文案要准确。

结果读取：

- `assets` 的 URL 改为 `/api/nova/plugin-results/<taskId>/<n>`：
  - 要求 `requireUser`，校验归属和过期时间；
  - 响应头 `private, no-store`；
  - 支持 Range；
  - Content-Type 取文件类型。
- 视频任务保留期：
  - `NOVA_VIDEO_RETENTION_DAYS`，默认 3 天，从终态开始计算；
  - 清理定时器同时清理视频文件和素材；
  - 图片任务仍用 `NOVA_RETENTION_DAYS`。

重启：

- Nova 重启时，内存里的 apiKey 会丢失。处理中的插件任务无法继续轮询，标记为失败。
- 错误信息写明「服务重启，任务中断；如上游已生成，可联系管理员按任务 ID <upstreamTaskId> 处理」。
- 不要把 key 写进磁盘。

## 4. 签名公开链接（C3）

- 路由：嵌入模式下 `GET` / `HEAD` `/api/nova/public-media/<file>?exp=&sig=` 免除 `requireUser`。
  - 签名错误、过期或文件不存在，返回 404。
  - 支持 Range，Content-Type 取素材类型。
  - 只能访问素材目录中的文件，文件名按现有 `serveFile` 的规则校验。
- 配置：
  - `NOVA_PUBLIC_BASE_URL`：嵌入模式必须配置，否则拒绝启动。
  - `NOVA_MEDIA_SIGNING_SECRET`：缺省时在数据目录生成 `media-signing.key`（权限 600）并持久化。
- 生成签名链接的函数，供插件提交时调用。有效期 24 小时。
- 未绑定任务的素材 24 小时后清理（看 `media.js` 现有的清理逻辑，调整到这个时长）。

## 5. 文档与镜像

- `docs/sub2api-embed.md`：
  - 第 5、6、7、8 节改为「插件任务已开放」的新规则；
  - 新增 public-media 和 plugin-results；
  - 第 13 节补充七号机需要新增的环境变量：`NOVA_PUBLIC_BASE_URL=https://www.qlqqx.com`、`NOVA_VIDEO_RETENTION_DAYS=3`（`NOVA_MEDIA_SIGNING_SECRET` 自动生成）。
- `docker/sub2api-embed.Dockerfile`：确认会把 `backend/plugins/qx-seedance` 打进镜像，并且不再包含 ccode-h3。
- 本机 `~/workspace/sub2api-local-nova/docker-compose.yml` 增加：
  - `NOVA_PUBLIC_BASE_URL=http://127.0.0.1:28080`
  - `NOVA_VIDEO_RETENTION_DAYS=3`
  - `NOVA_RESULT_DOWNLOAD_ALLOW_HTTP=1`
  - 让 Nova 能访问上游模拟 `sub2api-local-upstream-mock:18998`（下载 mock 视频用）：加入 `sub2api-local_default` 外部网络，或用其他最小改动。
  - 上游模拟返回的 `video_url` 需要是 Nova 能访问的地址。这需要 BE-S 改 `upstream_mock.py` 的视频 URL 并提供一个小 mp4。在完成报告里写明需求，主代理会转达；也可以直接看 mock 现状后提出具体改法。

## 6. 测试

运行：

```bash
cd backend && env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test
```

新增测试：

- 嵌入模式插件任务的归属：他人访问返回 404；
- baseUrl 固定为网关；
- XFF；
- 签名链接：正确、篡改、过期；
- 视频保留期；
- 重启后处理中的任务被标记为失败；
- 下载的协议规则和大小上限；
- qx-seedance 六种模式的提交 body；
- 状态映射。

本机构建：

- `cd ~/workspace/sub2api-local-nova && ./build.sh --no-web && docker compose up -d`
- 前端由 FE-N 构建，全部完成后再整体构建一次。
- 构建前确认 `sub2api-local` 在运行。

## 完成报告

（在此追加）

## 给 FE-N 的说明

- 插件使用脚本驱动 `qx-seedance`；facet 为 `targetModel`（`seedance-2.0` / `seedance-2.5`）及 `mode`（`t2v` / `i2v` / `firstlast` / `reference` / `edit` / `extend`）。variants 共 12 行，model 与 targetModel 相同。
- 普通字段：`prompt`、`resolution`、`ratio`、`duration`、`generate_audio`，默认分别为 `''`、`720p`、`adaptive`、`5`、`true`。edit 不显示且不提交 duration。
- 素材仍是 `media.<key>: string[]`，上传返回的 `/api/nova/plugin-media/<file>` 地址和用户填写的 HTTPS URL 可在同一数组混合，不引入对象。schema media 字段增加 `allowRemoteUrl: true`；外链不下载、不改写，上传地址仅在发上游前转换为 24 小时签名链接。
- 素材 key：`firstFrame`（i2v/firstlast，必填1）、`lastFrame`（firstlast，必填1）、`referenceImages`（reference，最多9）、`referenceVideos`（reference，2.0最多3/2.5最多10）、`referenceAudios`（reference，最多3）、`sourceVideo`（edit/extend，必填1）。参考模式至少一项素材。
- 预览方案：新增 `GET /api/nova/plugin-media/<file>/preview`，会话鉴权并检查素材归属，支持 Range；上传响应额外返回 `previewUrl`。本地 blob 也可使用。提交仍用原始 `url`，不能提交 previewUrl。任务结果按 C4 用带会话 fetch→blob 播放/下载。
- mmgate 文档差异：2.5 首帧/首尾帧 ratio 仅 adaptive（后端驱动校验，前端需限制选项）；2.0 参考生成不能仅有音频，需参考图或视频（后端校验）；edit duration 无效，按契约不传。watermark=false 仍传，但 mmgate 说明此参数不生效且成片默认无水印。

- 已完成 FE-N 请求：`referenceVideos` / `sourceVideo` 均增加按 targetModel 的 `maxTotalSeconds`（15/30秒）；文档补充该字段及 toolbar switch 开关按钮说明。validate.js/verify.js 没有针对 toolbar switch 的报错或跳过逻辑，无需调整。后端不依赖客户端时长校验。
- 补充：现有 `availableWhen` 只支持 AND，2.5 首帧比例由后端驱动权威校验，FE-N 请在 UI 对这两个模式限制为 adaptive，当前 schema 不新增 OR 语法。

## 约定变更

C5 的 ratio 选项按文档收紧：2.5 的 i2v/firstlast 仅 adaptive。其他模式保持原列表。素材字段和预览方案见上，不改变 C3/C4 路径。

## 完成报告

### 实现与文件清单

已完成任务书 1–6 节后端工作。分支仍为 `sub2api-custom`；未提交、推送、切分支或连接生产；没有修改 `frontend/` 或 sub2api 代码。原有 `docs/sub2api-embed.md` 的七号机部署说明已保留。

- 删除：`backend/plugins/ccode-h3/` 全目录（含旧 fixtures）。
- 新增插件：`backend/plugins/qx-seedance/{manifest.json,index.js,ui.schema.json,README.md}`。
- 新增 fixtures：`backend/plugins/qx-seedance/fixtures/{01-t2v,02-i2v,03-firstlast,04-reference,05-edit,06-extend}/`，每个目录均有 `input.json`、`expected-request.json`、`upstream-submit.json`、`upstream-poll.json`、`expected-result.json`。
- `backend/server.js`：开放插件任务/上传，写入用户归属，GET/ack/results 校验归属和过期；网关固定、Key 内存保存、独立后端轮询；处理期间 expires_at 为 NULL；终态按视频保留期计算；重启失败说明含上游任务 ID；成功受控下载；签名公开素材与鉴权预览、结果路由；结果和素材随任务回收。GET 只读，移除原先前端 GET 补轮询与缓存，避免与下载竞争。
- `backend/embed/security.js`：嵌入模式缺少 `NOVA_PUBLIC_BASE_URL` 时拒绝启动。
- `backend/plugin-runtime/executor.js`：嵌入出网仅固定网关 origin，提交和轮询带用户 XFF、拒绝重定向；请求超时覆盖响应体读取；轮询上限放宽到 12 小时。
- `backend/plugin-runtime/input.js`、`validate.js`：media 支持 `allowRemoteUrl` 与 HTTPS 外链；上传地址清单单独用于绑定；字段使用 schema 默认值；脚本驱动可做跨槽权威校验。
- `backend/plugin-runtime/media.js`：素材用户归属迁移、按用户去重、绑定前检查用户/类型/文件；签名链接生成和校验；自动持久化 600 密钥；孤儿素材默认 24 小时回收，已绑定素材不受孤儿 TTL 清理。
- 新增 `backend/plugin-runtime/files.js`：共用 GET/HEAD、单段 Range 文件读取。
- 新增 `backend/plugin-runtime/results.js`：流式下载、HTTPS/逐跳协议检查、最多 3 跳、1GB、10 分钟、失败清理 `.part`，本地视频结果存储与删除。
- `backend/plugin-runtime/verify.js`：内置插件示例引用更新。
- 测试：修改 `backend/test/{plugin-runtime.test.js,embed-security.test.js,embed-server.integration.test.js}`，新增 `backend/test/plugin-storage.test.js`。
- `.gitignore`：内置插件例外改为 qx-seedance。
- `README.md`、`README.zh-CN.md`：按任务书第 1 节替换内置插件说明/安装示例。
- `docs/plugins/{README,LLM,cookbook,errors,lifecycle,manifest,provider,quickstart,testing,ui-schema}.md`：替换旧插件引用，补脚本驱动、链接输入、预览、下载与保留规则。ui-schema 增加 `maxTotalSeconds`，switch 在 toolbar 渲染为开关按钮。
- `docs/sub2api-embed.md`：更新 5–8 节，记录 public-media、plugin-results、preview、签名与结果目录；第 13 节增加生产所需环境变量。
- `docker/sub2api-embed.Dockerfile`：设置 `/data/videos` 与默认视频保留 3 天；构建检查 qx-seedance 驱动存在。删除的旧插件不会进入 COPY。
- 本机 `/home/qlqq/workspace/sub2api-local-nova/docker-compose.yml`：增加公共基址、视频保留期、仅本机 HTTP 下载开关，接入 `sub2api-local_default` 以读取 mock 视频。

### mmgate 文档核对

已使用本机 Playwright + 无头 Chromium **渲染** `/docs/seedance/create` 和 `/docs/billing`，不是仅抓 HTML。抓取脚本/文本位于 `/tmp/mmgate-scrape/{seedance.js,seedance.txt,models-billing.txt}`。

核对结果：

- `content` 采用 `{type:'text',text:...}` 与 `{type:'image_url'|'video_url'|'audio_url', <type>:{url:...}, role:...}`。
- role 与契约一致：`first_frame`、`last_frame`、`reference_image`、`reference_video`、`reference_audio`；首帧/首尾帧不能与多模态参考混用，尾帧必须有首帧。
- `omni_reference_task_type`：`auto` / `reference` / `edit` / `extend`，edit/extend 至少一个视频。2.0 模型本身没有该参数，但 mmgate 接受并据此校验/计费，实际编辑/延长仍依靠提示词；插件按契约传 edit/extend。
- resolution 为 `480p` / `720p` / `1080p`，缺省 720p；ratio 为契约七种值。**2.5 的首帧/首尾帧仅 adaptive**，后端已拦截其他值。
- 2.0 时长 4–15、默认 5；2.5 时长 4–30，不传或 -1 为自动。edit 时长参数无效，插件不传。generate_audio 缺省 true；watermark 可传但不生效，成片无水印，插件仍固定 false。
- 参考视频：2.0 最多 3 个/总计 15 秒，2.5 最多 10 个/总计 30 秒。编辑/延长的 sourceVideo 也属于参考视频，故同样设置 maxTotalSeconds；2.5 编辑原视频至少 4 秒。
- 2.0 不能只给音频，需图片或视频（后端已校验）。文档标注视频 mp4/mov、音频 wav/mp3；输入链接必须可公开下载。
- billing 文档确认有参考视频时：成片 + max(floor(参考总秒数), ceil(成片×2/3))；仅图片/音频不加参考视频费用，自动时长先按模型上限冻结。
- 另渲染 `/docs/models` 时页面显示「暂时没有上架的视频模型」，不能从该页独立核对各模型分辨率矩阵。实现按 create 文档与契约开放三个分辨率；未使用真实 mmgate Key 发付费请求。

### 测试与本机构建

全部最终检查通过：

1. `cd backend && env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test`：**121/121，通过，无跳过**。最终输出 `/tmp/nova-backend-all-tests-final.log`。
2. `cd backend && node plugin-runtime/verify.js`：qx-seedance 结构与 **六种模式 fixtures 全部通过**。
3. 改动的 runtime、驱动、嵌入安全和测试文件 ESLint 通过（使用已有 `backend/node_modules/.bin/eslint`）。`git diff --check` 通过。
4. 测试覆盖：跨用户任务/ack/结果/预览 404、未鉴权上传/结果要求身份、固定网关和提交/轮询 XFF、拒绝 API 重定向、签名正确/篡改/过期/密钥持久化、视频 3 天与图片 7 天、ack 不缩期、过期文件/绑定素材回收、孤儿 24 小时回收、重启失败及上游 ID、处理中不提前过期、逐跳协议/3 跳/大小/超时/临时文件清理、状态映射与六种 body 快照。
5. 先确认 `sub2api-local` 正常运行，执行 `cd ~/workspace/sub2api-local-nova && ./build.sh --no-web && docker compose up -d`：成功。Nova 日志已加载 qx-seedance（2 个模型），容器运行中。
6. 容器检查：内置插件只有 qx-seedance；`/data/media-signing.key` 权限 600；Nova 可读取 `http://sub2api-local-upstream-mock:18998/_media/sample.mp4`，Range 得到 206 / video/mp4 / 32 字节。
7. 经实际本机 sub2api → Nova 做签名公开路由冒烟：匿名合法 Range=206、HEAD=200、篡改签名=404、匿名 preview=401，Cache-Control 为 `private, max-age=3600`。测试素材和记录已立即删除。

过程中两次测试断言失败，均已修正并重跑：

```text
AssertionError: actual 259200001, expected 259200000
```

这是终态时间与过期时间分别读取 Date.now 导致 1ms 差异；现在过期时间统一从 completed_at 加视频保留期计算。

```text
{"publicRange":206,"bytes":"nova","publicHead":200,"invalidSignature":404,"anonymousPreview":401}
public route smoke failed
```

这是冒烟脚本将 `bytes=0-3` 的 4 字节结果误写成 5 字节期望；改为 `nova` 后同一路由全部通过，非服务实现故障。

### 剩余引用、遗留和协作需求

- 最后对工作树源码执行 `rg 'ccode-h3'`（排除 .git、node_modules、构建产物）无匹配；FE-N 已同步清理原先的前端测试引用。旧插件删除条目仍会显示在 git diff 的文件名中，这是预期。
- **FE-N**：上传、HTTPS 混合数组、previewUrl、结果鉴权地址均已可用；maxTotalSeconds 与 toolbar switch 文档请求已完成。仍请落实 2.5 i2v/firstlast 的 adaptive 选项限制；后端已做权威校验，schema 当前未引入 OR 条件扩展。前端完成后由 FE-N 做完整前端构建/整镜像刷新与 UI 验证（本次只做 --no-web）。
- **BE-S**：观察到 mock 已提供 `/_media/sample.mp4` 并返回 Nova 可达的容器域名 URL，网络连通已验证，当前无需再改 mock。public-media 免 JWT 转发已通过实际本机冒烟；工作台 CSP 的 `media-src 'self' blob:` 仍由 BE-S 负责。
- **QA/主代理**：继续完整业务链路和六模式 UI e2e；此报告不把本机 HTTP 桩当作真实 mmgate 验收。生产发布前配置 `NOVA_PUBLIC_BASE_URL=https://www.qlqqx.com`、`NOVA_VIDEO_RETENTION_DAYS=3`，保留 /data 持久化；不要配置本机 HTTP 下载开关。

## 补充完成

已同步 FE-N「约定变更」第 4 条，前述 adaptive 选项联动的协作需求已关闭：

- `backend/plugins/qx-seedance/ui.schema.json`：ratio 除 adaptive 外的六个选项均增加 `unavailableWhen: { targetModel: ["seedance-2.5"], mode: ["i2v", "firstlast"] }`。
- `backend/plugin-runtime/input.js`：选项须满足 availableWhen，且不得命中 unavailableWhen；缺省未声明禁用条件时保持可选。
- `backend/plugin-runtime/validate.js`：按 availableWhen 相同规则校验 unavailableWhen 的条件结构。
- `docs/plugins/ui-schema.md`：增加字段语法、联合判断和 Seedance 示例说明，移除原先无法用 schema 表达该限制的说明。
- `backend/test/plugin-runtime.test.js`：新增通用可用/禁用条件组合、非法条件结构和 Seedance 比例矩阵测试。2.5 i2v/firstlast 的六种固定比例均被拒绝，adaptive 通过；2.0 i2v/firstlast 的 16:9 通过，2.5 文生视频的 16:9 仍通过。插件驱动的原有权威校验未修改，并有直接测试覆盖。

验证结果：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY node --test` **124/124 通过**（无跳过），输出 `/tmp/nova-backend-unavailable-when-tests.log`；`node plugin-runtime/verify.js` 结构与六种 fixtures 全部通过；本轮改动 JS 文件 ESLint 和 `git diff --check` 通过。本轮未构建或重启镜像，未修改前端，未提交、推送或连接远端。
