# FE-N：Nova 前端任务书（claude opus-5.5 high）

仓库：`/home/qlqq/workspace/nova-image-studio`，分支 `sub2api-custom`，只改 `frontend/`。

开始前按 `prd.md` 的「共同规则」读取文档。重点读：

- `docs/sub2api-embed.md`：嵌入模式的约束，尤其第 5、6、9、10、11 节；
- `docs/plugins/ui-schema.md`。

接口以 `contract.md` 的 C4、C5 为准。

后端由 BE-N 并行实现。插件定义 `backend/plugins/qx-seedance/` 由 BE-N 编写。

- 你先按 C5 的字段推进，读它的 `ui.schema.json`。还没写出来时，参考已删除的 `ccode-h3`：`git show HEAD:backend/plugins/ccode-h3/ui.schema.json`。
- 素材链接输入的字段格式，以 BE-N 完成报告里的说明为准。BE-N 的完成报告追加在 `/home/qlqq/workspace/sub2api/.trellis/tasks/10-31-seedance-video/task-be-nova.md` 末尾。

## 目标

在嵌入模式（`NEXT_PUBLIC_NOVA_EMBED=1`）的创作工作台里，加一个「视频」入口，用 qx-seedance 插件生成视频。

## 1. 视频 Key 与模型

`src/lib/embed/key-policy.ts` 现在有 `StudioPurpose`，包含图片和文本等用途。

- 新增 `video` 用途。可用的 Key 满足以下条件：
  - 所在分组的模型广场里有 `billing_mode=video` 的模型；
  - 或者模型名匹配 `/seedance/i`。
- 在设置页「模型」分区（`ModelKeySettings.tsx`）增加「视频 Key」选择，沿用图片 Key 的交互：
  - 下拉选择已有 Key；
  - 支持一键创建名为「创作工作台·视频」的 Key（看 `studioKeyName` 的现有规则）；
  - 只能选择支持视频的分组。
- 本地存储按 `u<userId>:` 分区（看 `storage-partition.ts`）。
- 如果「工作台默认值初始化」（`workbench-defaults.ts`）对图片 Key 有自动选择的逻辑，视频 Key 也照做：当用户只有一个支持视频的分组 Key 时，自动选中它。

## 2. 视频生成界面

- 入口：在工作台主导航或模式切换里增加「视频」。
  - 先看嵌入模式下现有的插件 UI 是怎么被隐藏的：设置里的插件分区只对管理员显示，但插件生成面板本身在嵌入模式可能也被关掉了。grep `embed` 和 `plugin` 确认。
  - 复用上游已有的插件生成面板。只针对嵌入模式做必要的调整，不重写一套。
  - 嵌入模式只显示 `qx-seedance`，不显示插件管理入口。插件管理仍然只对管理员显示。
- 表单按 `ui.schema.json` 渲染：
  - 模型：Seedance 2.0 / 2.5；
  - 生成方式（6 种）；
  - 提示词、分辨率、比例、时长、有声。
  - 按 C5 的规则联动：
    - 2.5 才有「自动时长」；
    - edit 模式不显示时长；
    - 各模式需要的素材槽：首帧、尾帧、参考图、参考视频、参考音频。
  - 如果 schema 渲染器缺少联动能力，在前端补上，保持通用，不要写死 qx-seedance。
- 素材输入：
  - 上传：走 `POST /api/nova/plugin-media`，带会话的 fetch，用 `auth-fetch.ts`。
  - 公网链接：输入框，只接受 `https://`。
  - 上传的视频在前端读取时长：用 `<video>` 的 metadata 读取本地 blob。
  - 参考视频的个数和总时长按模型上限校验：
    - 2.0：3 个，15 秒；
    - 2.5：10 个，30 秒。
    - 公网链接读不到时长，不参与校验。
  - 预览：用本地 blob，或 BE-N 提供的 preview 接口。
- 提交：
  - `POST /api/nova/plugin-tasks`，body 里的 `apiKey` 为视频 Key 的明文，取法与图片 Key 相同。
  - 嵌入模式不传 `baseUrl`。
- 进度：
  - 沿用插件任务的 WebSocket 推送和 GET 轮询。
  - 显示排队中、生成中（有进度时显示进度）、完成、失败。
  - 视频生成可能需要几分钟到几十分钟，文案要说明可以离开页面，回来再看。
- 结果：
  - 用带会话的 fetch 获取 `/api/nova/plugin-results/<taskId>/<n>`，转成 blob 后用 `<video controls>` 播放，并提供下载。
  - 参考 `AuthedImg.tsx` 和 `authed-image.ts`，做一个 `AuthedVideo`。
  - 视频大，blob 要按需加载（进入视口或点击时），离开时 `revokeObjectURL`。
- 保留期提示：
  - 结果卡片上显示「视频保留 3 天，请及时下载」。
  - 文案只写一处，不要到处重复。

## 3. 价格预估

- `src/lib/embed/cost-estimate.ts` 新增 `estimateVideoCost`：
  - 单价：取模型广场里视频 Key 所在分组、对应模型的按秒价格。
  - 计费秒数：
    - 成片秒数；
    - 自动时长按模型上限：2.0 为 15，2.5 为 30；
    - edit 模式：成片秒数等于参考视频时长，读不到时按模型上限。
    - 有参考视频时，加上 `max(floor(参考总秒数), ceil(成片×2/3))`。
  - 显示格式：`预计 ¥X（N 秒 × ¥单价）`。
- 在视频表单的提交按钮旁边显示，复用 `EmbedCostEstimate.tsx` 的样式。
- 模型广场的数据结构看 `sub2api-client.ts` 和 `key-policy.ts` 里的 `findPlazaModel`。视频价格的字段名可能和图片不同，需要读 sub2api 的模型广场接口确认：`/home/qlqq/workspace/sub2api/backend` 里 grep `plaza`。

## 4. 删除 ccode-h3 的前端引用

- grep `ccode-h3`、`Ccode H3`、`minimax-h3`，删除前端里专属于该插件的引用，测试中的引用也要处理：
  - `src/lib/__tests__/plugin-schema.test.ts`
  - `src/lib/embed/__tests__/registry.test.ts`
  - 等等。
- 改用 qx-seedance，或改用通用的测试插件定义。
- 不要删除与 ccode-h3 插件无关的功能，例如 `ccode-task-socket.ts`。删之前先确认用途。

## 5. 测试与构建

单测（vitest）：

- `estimateVideoCost`：普通、自动时长、edit、有参考视频；
- 视频用途的 Key 过滤；
- 参考视频上限校验；
- schema 联动；
- `AuthedVideo` 的加载与 revoke。

```bash
cd frontend && npx vitest run
npx eslint src/lib/embed src/components/embed   # CI 只对嵌入目录跑 eslint
```

本机构建：

- BE-N 完成后：`cd ~/workspace/sub2api-local-nova && ./build.sh && docker compose up -d`
- 再用 `~/workspace/sub2api-local/e2e/ui/studio_ui.sh` 的方式，或自己写 Playwright 脚本，截图视频页：
  - 表单的六种模式；
  - 价格预估；
  - 完成后的播放。
- 截图放在 `/tmp/seedance-fe-n/`。
- 用户账号和 Key 的准备方法见 `e2e/ui/studio_ui.sh` 和 `e2e/p8_studio.py`。视频需要 Seedance 分组：可以借用 `p12_seedance.py` 的 setup 创建分组和账号，或者自己通过管理端接口创建（上游指向 upstream-mock）。

## 完成报告

（在此追加）

## 约定变更

FE-N 提出（需要 BE-N 在 `qx-seedance/ui.schema.json` 与文档里配合）：

1. 视频素材字段新增 `maxTotalSeconds`，写法与 `maxCount` 相同（整数或 `{ byFacet, values, default }`），表示该槽内视频的总时长上限（秒）。前端对上传视频读取时长后校验；公网链接读不到时长，不参与校验。
   - `referenceVideos`：`{ "byFacet": "targetModel", "values": { "seedance-2.0": 15, "seedance-2.5": 30 } }`
   - `sourceVideo`：如 mmgate 文档对编辑 / 延长的原视频也有同样的总时长限制，同样加上。
   - `docs/plugins/ui-schema.md` 的 media 表格补一行；`validate.js` 是否校验由 BE-N 决定（前端不依赖）。
2. `switch` 字段写进 `layout.toolbar` 时，前端在工具栏渲染为开关按钮（`generate_audio` 现在就在 toolbar 里，原先会被跳过而不显示）。`ui-schema.md` 里「switch 写进 toolbar 会跳过」的说明需改为「渲染为开关按钮」。
3. 工具栏里的选项字段现在也遵守 `showIf`（edit 模式不显示时长）。属于前端修复，不改约定。
4. （第二轮）选项新增 `unavailableWhen`，语法同 `availableWhen`，命中时该选项不可选（两者同时写时都要满足）。用来表达 BE-N 提出的「2.5 的 i2v / firstlast 只能 adaptive」，不需要 OR 语法，前端也不写死 qx-seedance：
   - `ratio` 除 `adaptive` 外的 6 个选项加 `"unavailableWhen": { "targetModel": ["seedance-2.5"], "mode": ["i2v", "firstlast"] }`。
   - 前端已实现（`plugin-schema.ts` 的 `isOptionAvailable`，切到该组合时比例自动收敛为 adaptive）。需要 BE-N 改 `ui.schema.json`，并在 `input.js` 的 `isOptionAvailable`、`validate.js`（条件校验）和 `ui-schema.md` 里同步；后端驱动的权威校验保持不变。
   - schema 没加之前，前端会显示全部比例，提交后由后端驱动报错。

约定变更第 1、2、4 条 BE-N 已落实（`maxTotalSeconds`、toolbar switch 文档、ratio 的 `unavailableWhen` 与 `input.js`），前端单测已按真实 schema 校验。

## 完成报告（FE-N，2026-10-31）

只改了 `frontend/`；未提交、推送、stash，未连接远端。

### 改动

**视频 Key 与模型（第 1 节）**
- `lib/embed/key-policy.ts`：`groupPurposeSupport` / `filterKeysForPurpose` / `filterGroupsForPurpose` 增加可选的 `plaza` 参数；`video` 用途的条件是：分组在模型广场里有 `billing_mode=video` 的模型，或者模型名匹配 `/seedance/i`。新增 `perSecondVideoPrice`（按分辨率分档，匹配不到时取第一档；视频独立倍率覆盖分组倍率）。`formatModelPrice` 的视频单位改为「秒」。`studioKeyName` 增加 `purpose` 参数：视频一律命名为「创作工作台-视频」（沿用现有的 `-` 后缀规则）。原因是 Seedance 分组必须开启生图，否则会被当成生图+对话共用分组，名字只剩「创作工作台」。
- `lib/embed/embed-registry.ts`：`StudioPrefs.video`，存本地，按 `u<userId>:` 分区。
- `lib/embed/studio-state.ts`：sub2api 的工作台偏好接口（`SaveWorkbench` 白名单）只接受 image/text，所以视频这一行不发给 sub2api，每次保存后再合并回来。还没选视频 Key、且只有一把支持视频的可用 Key 时，自动选中它（`workbench-defaults.ts` 的 `soleVideoKey`），模型取它的第一个视频模型。新增 `videoSelectionOf`。
- `components/embed/ModelKeySettings.tsx`：「即将上线」占位换成真正的视频行（Key 下拉、模型下拉、单价、一键创建），复用 `PurposeRow`。设置弹窗的说明文字改为「生图、对话与视频」。

**视频工作台（第 2 节）**
- `WorkspaceShell.tsx` / `WorkspaceModeTabs.tsx`：嵌入模式显示「视频工作台」；没选 Key 时的提示跳到「模型与密钥」。插件管理分区仍然只对管理员显示（未改动）。
- `PluginWorkbench.tsx`：复用上游面板。嵌入模式下只显示 `qx-seedance`，用视频 Key 的明文作为 apiKey，不传 baseUrl；默认模型取设置页的视频模型，设置页改了模型时只同步模型，保留当前生成方式。上传的视频用 `<video>` 读取时长；支持公网链接素材；用模型广场价格预估费用。「重用参数」改用任务里保存的 facets，因为生成方式是 facet，多个 variant 共用同一个模型 ID，只按模型反查会丢掉生成方式。
- 通用 schema 能力（`plugin-schema.ts`，没有写死 qx-seedance）：
  - `maxTotalSeconds`：素材总时长校验，读不到时长的素材不参与。
  - `allowRemoteUrl`：允许公网链接。
  - 选项级的 `unavailableWhen`。
  - `SchemaToolbar.tsx`：工具栏里的字段遵守 `showIf`（edit 模式不显示时长）；`switch` 渲染为开关按钮（生成声音）。
- `SchemaMediaField.tsx`：「链接」输入只接受 `https://`。链接素材显示为「主机名/…/文件名」，不在浏览器里加载。视频胶囊上显示时长。
- `plugin-upload-runner.ts`：`PendingMedia.file` 可以为 null，此时用 `remoteUrl`。链接不上传、不进进度列表，提交时按原顺序和上传地址混在同一个数组里。
- `plugin-task-client.ts`：`baseUrl` 改为可选。创建和查询的响应经过 `inspectEmbedTaskPayload`，网关错误转成中文；任务到终态时刷新余额并通知父页（实测余额 ¥100 → ¥96.25）。
- 进度：沿用 GET 轮询，进入视频页时每 5 秒一次。现有代码里插件任务没有 WebSocket 推送，这次没有新增。上传完成后显示「视频生成通常需要几分钟到几十分钟，可以离开页面，稍后回来查看结果。」
- 结果：新增 `lib/embed/authed-video.ts` 和 `components/embed/AuthedVideo.tsx`。视频进入视口后，用带会话的 fetch 取成 blob 再播放；换地址或卸载时 revoke。预览弹窗使用这个组件。嵌入模式下的下载也走带会话的 fetch，并去掉了「复制链接」「新窗口打开」和上游直链探测。素材预览用本地 blob，没有使用 BE-N 的 preview 接口。
- 保留期文案只定义在 `lib/embed/copy.ts` 的 `VIDEO_RETENTION_NOTICE` 一处，显示在结果卡片（视频生成历史）的标题下。`PluginPriceTag` 在嵌入模式下不显示 manifest 申报价。

**价格预估（第 3 节）**
- `lib/embed/cost-estimate.ts`：
  - `billedVideoSeconds`：成片秒数；自动时长按模型上限；edit 的成片秒数等于原视频时长，读不到时按上限；有参考视频时加上 `max(floor(参考总秒数), ceil(成片×2/3))`；上限 60 秒。
  - `estimateVideoCost`：文案为 `预计 ¥X（N 秒 × ¥单价）`，并发时末尾加 `× n`。
  - `estimatePluginVideoCost`：从表单推出预估输入。模型上限取 `priceQuantityField` 在当前组合下的最大可选值；时长字段不可见时视为 edit；参考视频取所有可见的视频素材槽。
- 显示样式复用 `EmbedCostEstimate.tsx`（抽出 `CostEstimateText`）。价格字段核对结果：模型广场接口里视频的每秒单价在 `pricing.per_request_price`，或在 `intervals[].per_request_price` 里按 `tier_label`（分辨率）分档；倍率规则与 sub2api 的 `PlazaModelPricingTable.requestRate` 一致。sub2api 后端不需要新增接口。

**删除 ccode-h3（第 4 节）**
- `plugin-schema.test.ts` 改为读取真实的 `backend/plugins/qx-seedance`，通用求解规则改用测试内置的最小 schema。`canvas-video-generation-service.test.ts` 的内联插件改名为 `test-video`。`frontend/src` 中已没有 `ccode-h3`、`Ccode H3`、`minimax-h3`。`ccode-task-client` / `ccode-task-socket` 是内置任务功能，与该插件无关，未改动。

### 测试

- `cd frontend && npx vitest run`：**85 个文件、787 个测试全部通过**。
  - 新增：
    - `embed/__tests__/video-cost-estimate.test.ts`：单价、普通、自动时长、edit、参考视频、60 秒上限、从表单推出输入。
    - `embed/__tests__/authed-video.test.ts`：进入视口才加载、带 Authorization、卸载和换地址时 revoke、404 显示中文错误。
    - `lib/__tests__/plugin-upload-runner.test.ts`：链接与上传素材混合、不传 baseUrl。
    - `components/plugin/__tests__/SchemaToolbar.test.tsx`：showIf、switch、链接输入。
  - 扩充：
    - `key-policy`：视频用途过滤、Key 命名。
    - `workbench-defaults`：唯一视频 Key 自动选中。
    - `studio-defaults-state`：视频行不发给 sub2api、保存后合并回来、自动选中。
    - `plugin-schema`：六种模式的素材槽、时长、2.5 自动时长、2.5 的 i2v/firstlast 只能 adaptive、`maxTotalSeconds`、参考视频总时长校验、`unavailableWhen`。
- `npx eslint src/lib/embed src/components/embed`（CI 范围）：通过。另外检查了 `src/components/plugin`、`src/lib/plugin-*.ts`：只剩 `PluginHistoryList.tsx` 的两条警告（`no-non-null-asserted-optional-chain`、未使用的 `downloadingId`），用 HEAD 版本对照确认改动前就存在。
- `npx tsc --noEmit`：只有改动前就存在的三个测试文件的类型错误（`agent-chat-client`、`antigravity-gemini-preset`、`protocol-layout-contracts`），新代码没有引入错误。
- 本机构建：`~/workspace/sub2api-local-nova/build.sh` 完整构建（含前端）后 `docker compose up -d`，最后一次构建包含全部改动。

### 端到端截图（BE-N、BE-S 都已完成）

截图在 `/tmp/seedance-fe-n/`。脚本是 `video_ui.cjs` 和 `setup_fe_n.py`（复用 e2e 的 `lib.py`，状态写在 `/tmp/seedance-fe-n/run`，不影响 e2e 的 `runs/`）。

测试数据：新建分组「Seedance 视频 FE-N」，上游账号用 `model_mapping` 把 seedance-2.0/2.5 映射到 mock 的模型名，渠道按秒定价 0.75 / 1.125，用户充值 100，并在该分组建一把 Key。

| 截图 | 内容 | 结果 |
|---|---|---|
| `01-settings-video-key.png` | 设置页视频行 | 唯一的视频 Key 已自动选中，模型 seedance-2.0，显示「单价：¥0.75 / 秒」 |
| `02`–`07-mode-*.png` | 六种模式 | 素材槽：t2v 无；i2v 首帧必填；firstlast 首帧、尾帧必填；reference 参考图 0/9、视频 0/3、音频 0/3；edit 和 extend 原视频必填。edit 不显示时长，预估 15 秒 × ¥0.75 |
| `08-reference-media-estimate.png` | 参考生成：上传 1 秒的 mp4，加一张链接参考图 | `预计 ¥6.75（9 秒 × ¥0.75）`，即 5 + max(1, 4) |
| `09`/`10` | 2.5 的时长选项 | 有「自动」；选自动后 `预计 ¥33.75（30 秒 × ¥1.125）` |
| `11-processing.png` → `12-completed.png` → `13-playback.png` | 提交 5 秒文生视频 → 完成 → 播放 | 约 21 秒完成；播放器 src 是 blob:，readyState 4，没有报错；余额扣 ¥3.75，与预估一致 |

最后一轮没有 console 错误，也没有 HTTP 4xx/5xx。

### 遗留与需要配合的地方

1. **Seedance 分组必须开启 `allow_image_generation`**：第一次实测报错「Image generation is not enabled for this group」（p12 建分组时也开了这个开关）。生产分组请确认已开启。开启后这把 Key 也会出现在 Nova「生图」的下拉里，选中后会提示「没有可用于生图的模型」，不影响使用。如果希望网关对 Seedance 不检查这个开关，需要 BE-S 判断。
2. 视频 Key 的选择只存在本地浏览器，换设备不同步。如果要同步，需要 BE-S 在 `SaveWorkbench` 的白名单里加 `video` 行，前端去掉 `withVideo` 的分支即可。
3. 视频用途依赖模型广场判断。管理员关闭模型广场时，没有可选的视频 Key，也不显示预估。
4. 保留期文案重复了一处：qx-seedance 的 `familyDescription` 写着「按秒计费 · 视频保留 3 天」，会出现在模型弹层里。建议 BE-N 去掉其中的「视频保留 3 天」，只保留前端那一处。
5. 「参考生成至少一项素材」和「2.0 不能只有音频」由后端驱动校验，前端不提前拦截，提交后任务失败并显示后端给的原因。
6. 公网链接素材读不到时长：不参与总时长校验，不计入参考秒数；edit 模式下按模型上限预估。
