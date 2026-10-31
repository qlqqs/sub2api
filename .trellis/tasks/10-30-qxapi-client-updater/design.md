# 设计：qxapi-client 更新器接入

接口约定见父任务 [design.md](../10-30-qxapi-update-feed/design.md)。下面写的是 qxapi-client 里要改的地方，都只在 `product.Active()` 时生效；非 QxAPI 模式保持与上游一致，一个字节都不改。

## 更新源

- `update.Feed()`、`NotesFeed()`：QxAPI 模式下返回 `product.API + "/api/v1/client-releases/qxapi/latest"` 和 `.../notes`。`MAGPIE_UPDATE_FEED`、`MAGPIE_NOTES_FEED` 继续生效，它们由 `QXAPI_*` 映射而来。
- 删除 `ErrNoFeed`（update.go:83-92），以及 `notes.go` 中 `askNotes`、`ShowNotes` 的提前返回。`ReleasePage` 改为返回 feed 中的 `url`；没有时返回 `product.Site + "/downloads"`。
- 资产 URL 和 `url` 是相对路径时，以 feed 的最终请求地址为基准解析，在 `LatestIn` 解码之后统一完成；notes 里每条的 `url` 同样处理。下载代码仍然拿到绝对地址，不用改。sub2api 的 `frontend_url` 没设置时，`url` 就是 `/downloads`（本机测试环境即如此）。
- 传输：复用当前 `qxapi` 分支已有的 `product.Failover`，与 `qxapi.New` 的做法一致。2026-10-31 用户授权后已独立合入 `qxapi-doh`（`f414c255`）：连接建立失败按 `api.qlqqx.com → ai.qlqqx.com → www.qlqqx.com` 切换，直连 QxAPI 域名使用限定范围的 DoH；代理保留原 DNS 行为，`dns: "system"` 可关闭 DoH。相对 URL 解析需验证实际成功请求的 origin，不能假定外层 Response.Request 总是保留 transport 内部改写的地址。
- QxAPI 模式下 `source.Do` 的 GitHub 镜像不起作用，也不加特殊处理。

## 默认开启

- 删除 `settings.go:1687` 中强制设置 `NoAutoUpdate` 的那一行，默认值回到上游。
- `internal/settings/qxapi_test.go:39` 改为断言默认开启。
- 本机已有的测试配置里可能保存了 `noAutoUpdate: true`，手动清除即可。没有用户，不写迁移。

## 资产名

- `AppAsset()`、`BinaryAsset()` 改为用 `product.Identity`，得到 `qxapi-darwin-<arch>.zip`、`qxapi-<os>-<arch>[.exe]`。
- zip 内的应用包名以 DMG 中的 `QxAPI.app` 为准（`tools/qxapi-dmg.py:72`）：`Stage` 中查找解压后的 `.app` 时按产品的应用包名查找，不再用 `Identity("magpie.app")`（它得到的是 `qxapi.app`，在区分大小写的卷上找不到）。
- macOS 的 `sameSigner`：新旧都是 ad-hoc 签名，TeamIdentifier 都为空，校验可以通过，不用改。

## 文案与改名

- 已经经过 `product.Words` 或 `cliWords` 的文案不用改。
- 下面这些原样写着 magpie 或 usemagpie.ai，要改：
  - `blocked.go:36,38`；
  - `admin.go:64` 的 osascript 授权提示；
  - `update.go:930`；
  - `update_cli.go:99-102,176,180,207`；
  - `cli/main.go:141` 的帮助；
  - `app.js` 中更新和镜像相关的文案（19462-19481 一带）和 docker pull 提示。
- `update.Site` 在 QxAPI 模式下取 `product.Site`。
- 帮助：撤销 `help_cli.go:38-40` 的 "QxAPI does not update itself"，以及 `qxapi_words.go:36` 对 `update` 的隐藏。
- 镜像：QxAPI 模式下隐藏 `update mirror` 子命令、`--mirror` 参数和设置中的镜像行；`UpdateMirror` 不再读取。
- 新增一个测试：在 QxAPI 模式下收集更新相关的错误、命令行输出和帮助文本，断言其中不含 `magpie` 和 `usemagpie`。

## 打包流程（`.github/workflows/qxapi-macos.yml`）

- 版本号：`GITHUB_REF_NAME` 以 `v` 开头时，`VERSION` 取去掉 `v` 之后的部分，作为正式构建；否则沿用现在的测试版本号。
- Windows：不变，只是版本号来自 tag。
- macOS：在 `tools/qxapi-dmg.py` 生成 DMG 的同时，用 `ditto -c -k --keepParent QxAPI.app qxapi-darwin-<arch>.zip` 打包同一个经过签名验证的应用包。`CFBundleShortVersionString` 使用正式版本号。
- Linux：新增一个 job，在 `ubuntu-24.04` 和 `ubuntu-24.04-arm` 上安装 GTK3 和 WebKitGTK 4.1 的开发包，然后运行 `make release-linux VERSION=...`。私有仓库能否使用 arm 托管运行器，第一次运行时确认；不能用的话，改为在本机或用 Docker 构建 arm64，并在验收记录中写明。
- 汇总 job：下载各平台产物，生成 release.json（`tools/qxapi-release-manifest.py`，带测试），然后上传一个合并后的 artifact。仍然不创建 GitHub Release。

## 不改的部分

试启动、回滚、`update-blocked.json`、空闲等待、自动重启和 handover，全部沿用上游代码。
