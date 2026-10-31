# 验证中发现的独立问题

## UI-UPDATE-WEBKIT-RO：旧更新界面的 ResizeObserver 投递循环

- 状态：已在工作区修复，定向验证通过，完整 GUI 回归待执行；客户端尚未提交/发布。
- 责任范围：GUI 共享顶栏布局，不是新更新源、发布接口或 QxAPI 镜像开关。
- 影响：WebKit 的更新界面回归捕获页面错误 `ResizeObserver loop completed with undelivered notifications.`。
- 首次完整回归：119 tests，106 pass / 13 fail（包含受子测试失败影响的父测试）。
- 实际失败用例：`update-check` 的 zh install failure；`update-download-percent` 的 zh；`update-notes-ask` 的三个 en；`update-stuck` 的 en/ja 1000px（folder/container 两组）。

### 基线与环境排除

1. 用 `git archive 3e89c59c internal/gui/assets internal/gui/tests` 解到当前会话的临时目录，不切换工作区、不改基线。
2. 在相同 WebKit/user-local 运行库下运行上述四个测试文件；同样的用例报同样的 ResizeObserver 错误。
3. 使用完整的 Liberation/WQY/IPA 字体路径重跑仍失败，不能仅归因于最初缺字体。
4. 不过滤 `pageerror`，不把报错改成 warning 或跳过测试。

日志：
- `/tmp/qxapi-update-regression-webkit-20261031.log`
- `/tmp/qxapi-update-webkit-baseline-20261031.log`
- `/tmp/qxapi-update-webkit-fonts-20261031.log`

### 定位

只在临时副本中包装 ResizeObserver，记录注册栈和错误前的回调。最后两个回调来自 `app.js` 的 `topFit`（`new ResizeObserver(fitTop)`，观察 `.actions`）和 `tools`。`fitTop` 同步删除/添加顶栏布局 class，改变所观察元素的尺寸，具备触发同帧投递循环的条件。

诊断日志：`/tmp/qxapi-update-webkit-observer-diag.log`。临时副本将 `topFit` 合并到下一帧后，原 ResizeObserver 错误消失，但出现 1000px 顶栏布局尚未稳定的横向溢出断言（10px），所以不能把这一行改动直接当成完整修复。诊断结果在 `/tmp/qxapi-update-webkit-rootcause-20261031.log`；这不是已并入的修复。若修改共享布局，须单独评审并跑全 GUI 套件，不能只凭更新页通过合入。


### 2026-10-31 后续修复与证据

- 单独把 ResizeObserver 回调延后一帧仍有真实瞬时溢出：日文 1000px 时 `.actions` 右侧到 1010px（关闭按钮也越界），两帧后才恢复，见 `/tmp/qxapi-header-diagnose.log`。不能只增加测试等待。
- 正式修复：尺寸通知合并到下一帧，actions 内容/hidden/class 变化通过 MutationObserver 在绘制前同步 fit；布局 class 和断点不变。新 `header-update-fit.test.cjs` 检查首次响应无溢出、字体变更及后续无 RO 错误。
- 第一轮相关 WebKit 137 tests：135 pass / 2 fail（一个缩放子测试及父测试），原更新/What's new 的 RO 失败已消失。日志 `/tmp/qxapi-header-related-webkit.log`。
- 新发现缩放测试的固定 1200pt 阈值是假设错误：英语 125%、1270pt（1016 CSS px）未压缩 nav 右缘 867.90，actions 左缘 865.78，实际已经重叠约 2.12px，并非“有空位”。撤回本次 header 修复的副本上同样失败，见 `/tmp/qxapi-header-zoom-baseline.log`、`/tmp/qxapi-header-zoom-measure.log`。
- 修改缩放测试为测量实际未压缩几何：有足够空间时仍强制零压缩，保留宽度递减的单调性/窄窗口必须压缩等断言，并要求确实覆盖有空间的情况；没有改产品断点或强制固定字体。
- 新增 header 与修正后的 zoom 测试在两引擎共 32 tests 全部通过，无 skip：`/tmp/qxapi-header-final-regression.log`。完整 GUI 回归尚不计通过。

## GUI-FULL-20261031：全量新发现项（诊断中）

完整 GUI 仍在运行，正式 assets/tests 保持冻结，哈希复核未变化。为避免把新失败直接叫作“基线问题”，另在 `/tmp/qxapi-full-gui-diagnosis.DsKp6F/` 对比当前资产与 `git archive f414c255` 的原始资产；所有实验修改仅位于该临时目录，未应用回工作区。

- `account-discovery.test.cjs`：当前与基线均在 line 112 失败，实际订阅开关是本地化 Off/On，作为比较对象的 `#redactList .row:first .opt` 却为空。DOM 证据显示第一行已是 `What was masked` 的 Open 操作行，Mask secrets 的分段开关在后面；测试把“第一行”误当作“第一个开关”。试验改为选首个 `.segs` 的 `.opt`，保留原开关文字/状态/确认/保存断言，验证进行中。仅改 `.row` 为 `.row.pref` 不足以修复，因为 Open 行也有 pref。
- `account-arrange.test.cjs`：当前 WebKit 重现点击/等待保存超时，基线单次通过，尚不能据此判断为本次回归或纯随机。几何诊断显示第一个 Smart 路由按钮在 y≈2091，850px 视窗之外；Playwright 隐式滚动与项目“只有读者滚轮才能滚动”的规则可能冲突。临时试验先向 editor 的 ebody 发真实滚轮直到目标可见，不放宽原断言，验证进行中。单独移除新 header 的 rAF、保留 MutationObserver 的副本仍失败，不足以归咎于 rAF。

日志：`current-*`、`base-*`、`discovery-diagnostic.log`、`arrange-geometry.log`、`seg-discovery.log`、`arrange-wheel.log`。修复候选只有完成定向与负向证据后，且本轮完整测试结束后，才考虑应用。

## GO-DAVSYNC-CONTENT：同步分片／恢复内容不一致（未解决）

最终 full 的两个 davsync 失败及后续捕获见 `/tmp/qxapi-davsync-fixes.lDxl3z/RESULT.md`。large-library 在 loopback fixture 下出现过分片身份不一致，以及长度同为 39,146,837 但 hash 不同的恢复文本。尚不能判定 fixture 或生产代码根因，也未证明两个失败同因；没有 davsync 源码修改。后续只捕获同一次失败在源文本、上传 blob、下载重组、恢复文件四处的字节，定位首次差异。不以单次通过、无 DATA RACE 提示或超时解释此前内容差异。

`TestZedAppPath` 的独立 fixture 缓存问题已完成定向修复及 race20/mutation；详见 verification.md。完整套件结果保留为失败。

### GUI 临时候选补充验证（正式文件仍冻结）

- discovery 候选改用 `#redactList .segs` 的首个开关，与订阅开关比较。Chromium 16/16、WebKit 16/16 通过；后者使用断网 runner，日志 `seg-discovery-webkit.log`。仓库外仅改变实际 `renderDiscovery` 的 On 标签，候选测试在原文字断言处真实失败（`wrong-on-label` 对本地化 On），退出 1；见 `discovery-label-mutation.log`。没有删减原默认值、确认/取消、保存或不滚动断言。
- arrange 的真实 editor 滚轮候选在 WebKit 与 Chromium 各完整文件通过（`arrange-wheel.log`、`arrange-wheel-chromium.log`）。这支持“先让屏外目标可见再点击”的测试操作修正，但尚未将偶发原失败全部归因；候选仍只在临时目录，不修改滚动保护或生产布局。
- 主代理续接后复核全量运行的 `source-before.sha256`：全部一致。GUI full 尚未结束且已有其他失败，以上定向结果不替代全量验收。

### GO-DAVSYNC-CONTENT 有界捕获结果

`/tmp/qxapi-boundary-capture.5n8txK/REPORT.md`：16 个样本未重现故障，最后样本四边界字节一致，没有异常 offset。捕获命令因 overlay-only 包的隐式 vet 错误退出 1，不计 PASS。没有新增修复依据，两项问题仍未解决；停止重复诊断，保留现有失败及成功样本证据。GUI full 仍继续原有运行，未修改冻结源码。

### 验收范围调整（用户确认）

用户已明确允许不处理 QxAPI 用不到功能的测试失败。GO-DAVSYNC-CONTENT 仍未解决，但因远程同步在 QxAPI 禁用，移出本次更新器交付阻塞列表，不再调查。已移除订阅/插件功能的 GUI 失败同样只记录；不应用其临时测试修正。共用界面、更新链路及保留的本地备份/模型同步不在豁免范围。原失败证据与平台未测项保留，详见 verification.md。
