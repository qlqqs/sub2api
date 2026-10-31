# 客户端更新验收（2026-10-31）

## 范围与环境

- 客户端原基线：`qxapi` 分支 `3e89c59c`。2026-10-31 按用户授权以独立 merge commit `f414c255` 合入 `qxapi-doh`（`bb8ee238`）。更新器 WIP 已恢复；上述验证时尚未提交、推送或发布。后续授权见文末。
- sub2api 已获授权提交：`d661f4cef`（管理页与更新源），`1d4bc6a8a`（任务启用、纠正 Failover 先决条件、日志空白格式）。原有 `embedded-workbench-guide.md` 修改未包含在提交中。
- 原基线有两域名 Failover；合并后是 api → ai → www，新增限定范围的直连 DoH。纯 merge 与更新器集成验证分开记录。
- 独立联调环境：`127.0.0.1:28081`，compose project `qxapi-updater-test`，独立 PostgreSQL、Redis 和 app_data。配置在仓库外 `sub2api-local/e2e/updater-isolated/compose.json`；凭据只读 `sub2api-local/.env`。原 `28080` 环境及其版本不动。
- 新环境 `/health` 为 200，首次 `/api/v1/client-releases/qxapi/latest` 为 404（空库）。

## 主代理：界面与文案

代码：`internal/gui/api.go`、`assets/app.js`、`assets/qxapi-words.js`、`assets/i18n.js`。

- QxAPI 隐藏 GitHub 下载镜像设置及镜像错误的 GitHub 恢复按钮；镜像写入 API 返回 404。
- 沿用原更新界面和翻译边界；容器提示不再推荐尚未发布的 QxAPI 镜像地址。
- 自动更新开关可关可开，默认 6 小时；中文说明正文不随 UI 语言变化。
- 新增 `qxapi-update.test.cjs`、`TestQxAPIUpdateMirrorIsUnavailable`。Go 测试由子代理统一运行。

### 浏览器测试

均使用浏览器本地 fixtures，所有请求限于 `http://qxapi.test`，不访问生产或用户账号。

1. Chromium 新增测试：11 tests（1 父测试 + 10 组合），全部通过。
2. WebKit 新增测试：11 tests（同上），全部通过。
3. 每个引擎覆盖 en/zh/zh-TW/ja/de，1100px 浅色及 390px 深色，未登录账号；检查默认开关、切换请求、中文说明弹窗、found/downloading/ready/blocked/error/available 状态、无 magpie/GitHub 镜像入口。
4. Chromium 原更新、What's new、i18n 和品牌回归：131 tests 全部通过。
5. WebKit 原更新与 What's new 回归：119 tests，106 pass / 13 fail（含父测试）。失败均记录 `ResizeObserver loop completed with undelivered notifications.`，不能计为全绿；详见 `known-issues.md`。在未改动基线 `3e89c59c` 的临时 git archive 上，四个失败文件复现相同错误。

命令（仓库根目录）：

```bash
source /tmp/qxapi-webkit-libs.sMbz3W/env.sh
BROWSER=chromium ARTIFACT_DIR=/tmp/qxapi-update-ui-20261031 \
  node --test internal/gui/tests/qxapi-update.test.cjs
BROWSER=webkit ARTIFACT_DIR=/tmp/qxapi-update-ui-20261031 \
  node --test internal/gui/tests/qxapi-update.test.cjs
BROWSER=chromium node --test --test-concurrency=2 \
  internal/gui/tests/update-*.test.cjs internal/gui/tests/whats-new*.test.cjs \
  internal/gui/tests/i18n-keys.test.cjs internal/gui/tests/qxapi-brand.test.cjs
BROWSER=webkit node --test --test-concurrency=2 \
  internal/gui/tests/update-*.test.cjs internal/gui/tests/whats-new*.test.cjs
```

日志：

- `/tmp/qxapi-update-ui-final-20261031.log`
- `/tmp/qxapi-update-webkit-final-20261031.log`
- `/tmp/qxapi-update-regression-fixedenv-20261031.log`
- `/tmp/qxapi-update-regression-webkit-20261031.log`

截图：`/tmp/qxapi-update-ui-20261031/`。已目检 Chromium 中文深色 390px、英文浅色 1100px；中文字体通过 `sub2api-local/e2e/ui/fonts.conf` 加载。

### 负向验证

将 assets 复制到临时目录，仅去掉 `if (!qxapiProduct)` 的镜像行保护，设置 `QXAPI_UPDATE_ASSETS` 运行新增 Chromium 测试。出现真实断言失败：`no GitHub mirror setting`，`1 !== 0`；不是编译错误或 skip。正式源码未改动，临时资产目录已清理。

日志：`/tmp/qxapi-update-mutation-20261031.log`。

### 环境修复记录

首次 Chromium 旧回归因 `channel: chromium` 使用的完整浏览器缺 `libcups.so.2` 而无法启动；补上已有 `/tmp/qx-model-browser-deps/root/usr/lib/x86_64-linux-gnu` 路径后通过。新增测试最初截图缺中文字体，加载上述 fonts.conf 后重跑通过。

WebKit 最初缺 GTK/GStreamer 等运行库。仅将 apt 包解压到临时目录 `/tmp/qxapi-webkit-libs.sMbz3W/root`，不安装系统包；独立启动脚本保留 LD_LIBRARY_PATH（上游 wrapper 会覆盖），设置本地 Mesa EGL vendor/driver 路径后 smoke 与新增测试通过。没有更改共享浏览器缓存。`PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=1` 用于绕过只检查系统库的预检，浏览器本身正常启动并执行测试。

## 子代理：更新核心、打包和 Go 测试

完整运行结果与 DoH 集成定向结果见下文。不将交叉编译或 browser fixtures 写成 Windows/macOS 原生升级实测，也不将 nogui harness 写成 Linux 桌面自动重启验收。

## DoH 独立合并（2026-10-31）

`f414c255` 的两个父提交为 `3e89c59c` 和 `bb8ee238`。提交前更新器 WIP 已备份并 stash，merge 自动完成无冲突；下列检查针对纯 merge，源码、索引及文件指纹前后未变：

- product/netproxy/settings 整包通过。
- `QxAPI|Failover|DNS` 定向：provider/gui 通过；qxapi 无匹配测试，不计为覆盖。
- product/netproxy 整包 race 通过。
- 三组命令均退出 0；日志 `/tmp/qxapi-doh-premerge-checks.NNXQUj/`。
- 使用临时 HOME、固定 Go 缓存、断网 Docker、`--pid=host` 修正 PID 与 `/proc` 不一致。

恢复 WIP 时 `internal/product/failover.go` 冲突已解决：保留三域名/pinned 循环，并通过 `roundTrip` 记录实际成功 origin。stash 和 `/tmp/qxapi-pre-doh-merge.664vFz` 备份保留。恢复后的集成验证另记，不把纯 merge 的通过当成更新器通过。

## 完整 Go 运行与未完成验收

合并前唯一完整运行自然结束，退出 1：70 包通过、4 包失败、19 包跳过，另有 273 条测试级 skip。日志 `/tmp/qxapi-backend-checks.GAquL2/`。没有重跑新的完整套件。

- 旧 runner 的 PID namespace 与宿主 `/proc` 不一致；修正后进程类定向测试通过，不代表所有失败都属环境。
- `TestCoreScriptBudget` 曾报 39475 > 39400；主代理缩短 `qxapi-words.js` 头部注释，未提高预算，修复后的测试另记。
- 尚未解决/归因完成：`TestFolderVarsAreListed` 的 `QXAPI_TEST_SITE` 检查、`TestClaudeAccountServesModelsListedSinceItsFetch`（完整运行失败、定向通过）、`TestInstalledVersion`、`TestOmpDetectedWithPiDir`、`TestZedSecretService`（D-Bus socket 不存在或拒绝连接）。
- Linux nogui / Windows vet、build 已通过；原生 Linux GUI 构建缺开发库；缺 Bun 的测试有 skip，`TestAskShellOwnSession` 因缺 zsh 跳过。
- 28081 E2E 为 **BLOCKED / 未通过**：HTTP 423 管理员合规确认须用户亲自完成，未代确认、绕过或发布测试版本。
- Windows/macOS 原生升级与远端正式打包未验证。

### 合并后的 GUI 定向复验

恢复 WIP 并缩短头部注释后，`qxapi-update.test.cjs` 在 Chromium/WebKit 两引擎共 22 tests 全部通过（含 2 父测试），0 fail/skip。命令：加载上述本地运行库 env 后 `node --test internal/gui/tests/qxapi-update.test.cjs`；日志 `/tmp/qxapi-update-post-doh-ui-20261031.log`。QxAPI 脚本实际总计 39261 字节，低于原 39400 上限；Go guard 后续定向复验已通过（见下文）。未重跑全部浏览器套件，已记录的 WebKit 共享顶栏问题仍未解决。


### 恢复 WIP 后的三域名集成（gpt-6-astra）

被测状态：`f414c255` + updater WIP。日志 `/tmp/qxapi-doh-updater-integration.R5g9zq/`，命令见 `summary.log`。

- product/netproxy/update/settings 整包定向运行通过；CLI 更新、自动更新、参数、容器及品牌定向通过。
- `TestCoreScriptBudget` 与 `TestQxAPIUpdateMirrorIsUnavailable` 通过，预算未提高。
- product/netproxy/update 整包 race 通过；修正独立预期链后再次定向及整包 race 通过。
- api、api→ai、api→ai→www 成功路径覆盖相对 asset/page/notes URL、重定向、SHA、sticky origin、外层覆盖 `Response.Request`、pinned ai/www 失败后返回 primary。
- ai 的 HTTP 响应、已连接错误、已收响应字节错误和取消不会继续向 www 重试；整包保留不可重放 body、第三方域名及 override 边界。
- 6 项 mutation 全部出现真实行为失败并恢复：origin 记录、第三方重定向记录、相对 Location、updater origin 读取、Failover hook、删除 Secondary。见 `mutation-summary.log` 与 `mutation-*.log`，最终指纹见 `final-source-sha256.txt`。
- gofmt、范围内 diff check 无输出。

本轮未重新跑 full 或 vet/build；公共 DoH 未联网实测（使用本地 TLS/注入 DoH fixtures），有缺 Bun/private IPv4/macOS 的 skip；已有完整测试失败、WebKit 问题及 28081 门禁仍未解除。

集成审阅发现 `whatsnew.go` 的 get/before 在获取 notes 前确定 ReleasePage，首次响应可能错过自定义链接。主代理已将查询移到 fetch/cache 处理后；补测与变异验证已通过，见下一节。


### 更新说明首响应链接回归

`TestQxAPIWhatsNewReleasePage` 通过真实 `/api/whatsnew` 路由覆盖 get、before、二者首次失败、get 的 latest fallback。五个场景各用独立产品子进程/不同版本号，未预热 release-page 缓存；检查相对 URL、首次响应、缓存不重复请求，以及 before 扩大范围失败后保留旧说明和链接。

日志 `/tmp/qxapi-whatsnew-regression.596J1N/`：

- `go test -tags nogui -count=1 -timeout 15m -v -run 'Test(QxAPI(WhatsNewReleasePage|UpdateMirrorIsUnavailable)|WhatsNew|FetchNotes|WithWaiting)' ./internal/gui` 通过，无 skip，见 `focused.log`。
- 同组 `-race` 通过，无 skip，见 `race.log`。
- 仓库外 Go overlay 分别恢复 get/before 旧顺序，两次退出 1，真实断言为首次链接返回 `/downloads` 而非 fixture 自定义页面。见 `mutation-get.log`、`mutation-before.log` 和 `mutation-summary.log`。
- 正式 whatsnew.go 前后 SHA-256 不变；恢复后的 focused 通过（`restored-focused.log`），gofmt/diff check 无输出。

本轮未执行 full/vet/build，也未操作 28081 合规确认或发布版本。全部本轮测试结束；客户端更新器 WIP 仍未提交。以上定向通过不解除此前完整测试失败和真机/E2E 验收缺口。


## 合规确认后的收尾（2026-10-31）

用户已表示亲自完成 28081 合规确认，子代理继续核验门禁并执行测试环境 E2E；不代确认/绕过，不操作 28080 或生产。28081 的 `0.0.0.0` 端口绑定是用户此前明确要求，尚未改回，仅 sub2api Web 端口对外映射，数据库/Redis 无宿主端口。

### GUI 失败项修复

- 工作区修复顶栏 ResizeObserver 和 Update 异步出现时的首帧溢出，详情见 `known-issues.md`。布局 class/断点未改。
- 新 header + 修正 zoom 定向双引擎 32 tests 通过：`/tmp/qxapi-header-final-regression.log`。
- 新 header 在修正后的断网 Docker runner、临时 HOME 下双引擎 22 tests 通过：`/tmp/qxapi-header-isolated.log`。
- Mutation 目录 `/tmp/qxapi-header-mutations._zyh_l0o/`：原 header 实现退出 1，出现真实 `ResizeObserver loop completed with undelivered notifications.`；去掉绘制前内容处理退出 1，出现实际溢出；强制不必要压缩退出 1，zoom 几何断言报 1 != 0。正式源码不动。
- 单独恢复同步 ResizeObserver、但保留 MutationObserver 的实验通过（`sync-resize.log`、`/tmp/qxapi-header-font-mutation.log`），不把它算作检出；负向依据是上述完整还原与另外两项实际失败。
- 新测试首次错误等待 Settings 页面不存在的 `#nav button.on`，启动超时后终止该轮，改为等待实际 `#about .row.pref` 再验证；这不是产品失败，也不计该轮为通过。首日志 `/tmp/qxapi-header-fit-regression.log`。

### 完整验证进行中

主代理已启动 `make test-ui UI_TEST_CONCURRENCY=4`（未设置 BROWSER，覆盖两引擎），临时 HOME/固定依赖/断网 runner。日志 `/tmp/qxapi-final-gui.LQmsqi/full.log`；开始时源码哈希 `source-before.sha256`，结束后写 `exit` 与 `source-after.sha256`。GUI资产及测试从启动后冻结。**尚未结束，不计为通过。**

子代理获授权在 E2E 与 Go 定向修复后启动一套最终完整 Go 测试和 vet/build，限制并行避免与浏览器争抢；结果另记。Windows/macOS 原生升级依旧未测。


### 28081 限定范围 E2E 已完成

子代理报告与主代理审阅：`sub2api-local/e2e/updater-isolated/runs/20261031T043535Z-oy3qidsq/REPORT.md`、`result.json`、`post-run-audit.json`。

- 登录/admin releases GET 均 200，确认门禁已解除，未代确认合规。
- 测试 release id=1、version=0.1.1，create/upload/publish 均 200；真实匿名 latest/notes 与相对路径解析、下载字节 SHA 校验通过。
- 从同一源码快照构建真实 root-main nogui 0.1.0/0.1.1（仅 stamp 不同），由额外 probe 调用真实 update 包，在沙箱替换目标文件；替换后断网运行 version 从 QxAPI 0.1.0 变为 0.1.1。
- 客户端内存将 expected SHA 改零时下载被拒绝，.new 清理，旧目标 hash 与实际运行版本保持不变；没有篡改服务器存储文件。
- release 已撤回（未删除）；latest/file 404、notes 为空。保留归属账本，后续不重复发布。临时 HOME/.new/runner 已清理，日志未含 .env 凭据值。28080/生产/真实安装未访问。
- **能力边界**：使用 GUI 资产名承载 nogui 字节仅用于隔离测试，probe 主动设 GUI=true。真正 nogui 客户端选择的是 qxapi-cli-linux-amd64，本次服务器无该资产，StageBinary 正确拒绝。不是主程序 CLI 自动更新路径、原生 GUI 自动重启、六小时调度或 Windows/macOS 验收通过，也不是八文件正式打包验收。

### Go / GUI 全量当前状态

- Zed 私有 D-Bus 测试 fixture 已修复，审阅确认仅测试 daemon 配置，不修改宿主策略或激活真实凭据服务。focused 与 20 次 race 通过，mutation 退出 1；日志 `/tmp/qxapi-zed-secret-hqbqts34/`。
- 子代理继续处理其他 Go 顺序/fixture 失败；最终完整 Go suite/vet/build **尚未开始**，定向稳定后按授权执行一套。
- GUI 全量仍在运行，**已经出现失败，不能视作全绿**，包括 account-arrange 的 WebKit 和 account-discovery 的多语言场景。保留整轮日志与冻结源码，待整轮结束归类，不在运行中修改测试或资产。尚不能判断这些新增失败是否来自本次变更。


### 最终 Go 完整运行结果（已结束，未全绿）

日志 `/tmp/qxapi-final-go.U0ZwAj/`，实际命令、退出码及原始输出见 `RESULT.md`、`results.json`、`*.command`、`*.exit` 和 `full-go-test.jsonl`。只执行这一套，未重跑覆盖失败。

`go test -p=2 -tags nogui -count=1 -timeout 45m -json ./...` 退出 1：72 包通过、2 包失败、19 包 skip/no-test；顶级测试 7581 pass、3 fail、239 skip（含子测试 308 条 skip 事件）。此前负责修复的项目及 GUI budget/mirror guard 均在全量通过。

剩余失败：
- `internal/agent::TestZedAppPath`：得到 Zed Preview.app，与 CLI bundle 预期不符。
- `internal/davsync::TestGitHubPartsBranchMovedMeanwhile/another_file`：reading it back: <nil>。
- `internal/davsync::TestGitHubSyncLargeLibrary`：the second computer's library: <nil>。

这三项尚未确认根因，不能直接归为环境或不稳定；子代理获得有界追加任务，只在对应测试/fixture 范围诊断，若是生产缺陷先报告方案，不自动扩大改动。未授权第二套 full。

vet/build：Linux nogui 和 Windows CGO_ENABLED=0 的 vet/build 通过；Linux native/gtk3 的 vet/build 因缺 fontconfig、GLib/GObject/GIO、GTK/WebKitGTK/libsoup pkg-config 开发依赖失败。未安装系统开发包，用户态浏览器运行库不代表 native 编译环境已具备。产物仅在上述日志目录 bin/ 内。

源码清单前后相同（`source-changes.json` 为 `{}`），测试结束无本轮进程残留。全部临时 HOME/固定 Go1.26.8 缓存/断网 runner，GOMAXPROCS=2/-p=2；既有 Bun、平台、真实程序 skip 如实保留。

### 已完成测试修复的根因审阅

生产逻辑未因这些 fixture 修复被放宽：
- QXAPI_TEST_SITE 精确归为产品 URL，不是 agent 配置目录，保留其他环境变量检查。
- 安装/移除工具可能同一目录 mtime tick，测试固定该边界后调用已有 proc.Forget，避免把工具发现缓存时序当作版本检测结果。
- 两个 Claude logout fixture 的 Allowances 后台读取越过自身 HOME/钩子生命周期；defer 等待读取结束后才由 t.Cleanup 还原环境。146 个前序测试下复现 race，并经受控延迟 mutation 验证。
- logins 权限错误 fixture 不需要 synctest 虚拟时钟；移除 bubble 避免全局通知 channel 跨 bubble 关闭，保留实际权限故障、原凭据不被覆盖及错误路径/时间戳断言。
- Zed 使用私有 D-Bus 配置/socket，不继承宿主激活服务及 AppArmor 策略，不更改宿主策略。

证据 `/tmp/qxapi-backend-failure-fixes.LD3gIy/VERIFICATION.md` 与 `/tmp/qxapi-zed-secret-hqbqts34/`；相关目标、Claude 146 前序、logins、Zed 分别 race count20 通过，负向 overlay 均出现真实行为失败。

### 完整 Go 之后的有界修复（未重跑 full）

- `TestZedAppPath` 已修复 fixture：先查询空 PATH、再创建 CLI symlink 的目录 mtime 可能未跨 tick，导致发现缓存继续返回不存在。测试固定该时间边界，在创建后调用已有 `proc.Forget()`；四项原路径断言保留。修复前受控 3/3 失败、移除刷新 overlay 3/3 失败、五项相关测试及 race count20 通过。唯一新增修改为 `internal/agent/zed_credential_other_test.go`；证据 `/tmp/qxapi-zed-apppath-isfnnz88/`。这是 Linux fixture 路径选择验证，不是 macOS 实机验收。
- 两项 davsync 仍未修复。`/tmp/qxapi-davsync-fixes.lDxl3z/RESULT.md` 记录 large-library 再次出现分片身份不一致及等长度恢复文本 hash 不同；此前 another_file 断言本轮未再复现。race20 超时/停止的运行不计通过，诊断自己引入的锁停顿不计原故障。尚未定位首次差异边界，不修改生产同步算法。
- 下一步仅授权一次有界四阶段字节捕获，不重跑 full/vet/build。既有完整 Go 结果仍为失败；Zed 的后续定向通过不能改写那一轮结果。

### GUI 全量期间的仓库外候选验证

`/tmp/qxapi-full-gui-diagnosis.DsKp6F/counterpart.sh` 经断网 runner 执行：discovery WebKit 16/16、arrange Chromium 全文件通过（对应 Chromium/WebKit 的另一引擎结果已在该目录保留）。`negative.sh` 的临时 discovery 生产标签 mutation 退出 1，实际文字断言失败；正式源文件未改。修复候选与原因见 known-issues.md，仍待 full 结束后决定应用。完整 GUI 的新增失败保持原始记录，不因这些候选通过而忽略。

### 四边界有界诊断结束：未复现，不计修复

2026-10-31 06:11:21–06:26:37 UTC，唯一捕获运行约 10 分半，完成 16 个 large-library 样本。未出现原断言或分片 SHA 失败。最终保留样本的源文本、Seal 输入、上传/mock 分片、实际下载重组、解密、恢复收集文本均一致；文件末尾 LF 是既有 writeText/readText 行为，不是之前等长度内容差异的解释。

**命令退出 1，不记 PASS**：诊断遗漏 `-vet=off`，Go test 隐式 vet 对 overlay-only 新包报目录不存在；测试本体仍执行，错误原样保留，没有为消除错误追加运行。记录 `/tmp/qxapi-boundary-capture.5n8txK/{REPORT.md,command.txt,capture.log,boundary-analysis.json}`，最终样本字节在 `samples/sample-016/`。主代理审阅报告与源码变更清单 `{}`。

两项 davsync 保持未解决：本次没有真实失败样本、首次异常边界或修复补丁；观测改变时序与分配，成功样本不能排除此前故障。停止追加 Go 诊断，不自动开启新一轮。七个冻结修复文件保持一致，本轮进程/容器已清理。

### 用户确认的本次验收范围（2026-10-31）

用户明确要求“用不到的功能测试失败不用管”。后续不再为 QxAPI 已移除功能修测试，不将这些失败作为本次更新器交付阻塞；保留原始失败结果，不改成通过、不删断言、不新增 skip。

- 两项 GitHub davsync 失败移出本次阻塞列表：QxAPI 的 `internal/gui/qxapi_policy.go` 屏蔽 `/api/davsync`，`renderSync` 不请求/显示远程同步配置，CLI policy 测试也检查其禁用行为。此排除不包含保留的本地备份恢复、QxAPI 模型同步或更新包下载。
- 已移除订阅账号/插件管理相关 GUI 失败仅记录；两个仓库外测试候选不再为本任务写回。已有无关测试修复保持原状，待提交拆分时单独说明。
- 更新、SHA 校验、notes、三域名故障切换、默认设置与共用 GUI 布局仍属于范围；不能按测试名称中带 Update 就一概纳入或排除。
- 原 GUI 全量继续原有运行，不额外启动全量；当前轮中的 QxAPI 更新页已在 Chromium/WebKit、五种语言及 1100/390px 全部通过（`full.log` 的 QxAPI update settings and states while signed out 两组）。整体原始结果仍按实记录。
- 正式八包构建、原生 GUI 重启及 Windows/macOS 真机升级尚未完成，不因缩小无关功能范围而计为通过。该阶段尚未授权提交/推送/生产发布；后续授权见下一节。


### 最终验收范围与 sub2api 发布授权（2026-10-31）

用户已明确授权提交、推送及七号机更新部署。sub2api 本轮仅提交本任务的 design、implement、known-issues 和 verification 文档；后端管理页/feed 实现在 `d661f4cef`，任务启用记录在 `1d4bc6a8a`。用户无关的 `.trellis/spec/guides/embedded-workbench-guide.md` 修改保持原样，不进入提交。

- 用户接受 QxAPI 已禁用功能的已记录测试失败不阻塞本次交付；这不是把原失败改记为通过，也不豁免仍启用的更新、SHA、notes、Failover、默认设置或共享布局。
- Windows/macOS 真机验证、正式客户端八包构建及客户端发布由用户/客户端主代理另行负责。当前没有这些平台实测或正式包验收通过的结论。
- 28081 限定 E2E 已验证实际 updater、匿名 feed/notes、相对 URL、下载 SHA、沙箱内真实 Linux nogui 文件替换与坏 hash 保留旧文件；只证明记录快照，不是原生 GUI 自动重启。测试版本 id=1／0.1.1 已撤回，latest/file 404、notes 空；不向生产上传测试包。
- sub2api 发布仅更新七号机主服务，沿用既有部署方式，保留 PostgreSQL、Redis、Nova、Open WebUI 与持久数据；部署前核对 revision/image 和配置，保存可回滚镜像及配置。生产只做 health、管理页资源、匿名 latest/notes、迁移表及持久目录检查。没有正式发布包时 latest 404 属于预期。
- 本次不重跑客户端 Go/davsync/GUI 或修改客户端；此前完整 Go 失败、未测项及 GUI 全量状态仍以原始记录为准。

生产提交 SHA、推送结果、部署镜像、健康检查与回滚命令写入仓库外部署报告；文档提交本身不代表生产已更新。

客户端主代理补充：已普通推送 `origin/qxapi`，远端 HEAD 为 `68f78dc732239c46d0796b860e057f629a9ec9f4`；三个新增提交分别为 `7c2775e4`（fixtures）、`a0295955`（header）、`68f78dc7`（updater），工作区干净。持久验收记录在客户端 `docs/qxapi-updater-verification.md`。截至交接时 GUI 原 full 仍运行；客户端 tag、workflow dispatch 和 Release 尚未执行，不影响本次服务器发布。
