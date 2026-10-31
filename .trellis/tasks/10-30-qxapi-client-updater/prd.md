# qxapi-client 接入 sub2api 更新源与三平台发布包

## Goal

qxapi-client（分支 `qxapi`，工作区 `/home/qlqq/workspace/qxapi-client`）沿用 magpie 原有的自动更新流程：检查、下载、sha256 校验、替换、重启，以及 Windows 被拦截时的回滚。更新源改为 sub2api，接口约定见父任务 [design.md](../10-30-qxapi-update-feed/design.md)。打包流程按正式版本号产出三平台安装包和 release.json。

## Requirements

- R1 QxAPI 构建从 `https://api.qlqqx.com/api/v1/client-releases/qxapi/latest` 检查更新，从 `.../notes` 读取更新说明，不再请求 usemagpie.ai；`QXAPI_UPDATE_FEED`、`QXAPI_NOTES_FEED` 可以覆盖，供本机测试使用。
- R2 检查更新不需要登录 QxAPI 账号。
- R3 自动更新默认开启，沿用 magpie 默认值（每 6 小时检查，空闲时自动重启），用户可以在设置和 `qxapi update auto` 中关闭。
- R4 按平台挑选安装包：`qxapi-darwin-<arch>.zip`、`qxapi-windows-<arch>.exe`、`qxapi-linux-<arch>`。
- R5 更新流程中用户可见的 magpie 字样全部改为 QxAPI / qxapi，包括界面、命令行、报错、系统授权提示和帮助；usemagpie.ai 改为本站地址。不做任何兼容旧版本的处理。
- R6 QxAPI 的更新文件不经过 GitHub，"更新镜像"功能在 QxAPI 中去掉，包括设置项、`qxapi update mirror` 和相关文案。
- R7 "新版本内容"弹窗和设置中的版本说明从 sub2api 读取中文说明。
- R8 打包流程：在 tag `v<版本>` 上手动触发时，以该版本号构建 Windows amd64/arm64、macOS arm64/amd64（DMG 和升级 zip）、Linux amd64/arm64，并生成 release.json。在非 tag 上触发时仍然出测试包。

## Acceptance Criteria

- [ ] AC-1 用 `QXAPI_UPDATE_FEED` 指向 `sub2api-local`：Linux 上 0.1.0 自动发现 0.1.1，下载、安装并重启后显示 0.1.1。
- [ ] AC-2 Windows、macOS 各完成一次同样的升级；做不到的平台在验收记录中写明未测。
- [ ] AC-3 sha256 不符时拒绝安装，原版本继续运行。
- [ ] AC-4 Windows 上模拟被策略拦截（沿用现有测试）时会回滚，提示中没有 magpie。
- [ ] AC-5 新装的 QxAPI 默认开启自动更新；关闭后不再自动检查。
- [ ] AC-6 在 QxAPI 模式下运行更新相关的命令行输出、界面文案和错误信息，扫描结果中没有 `magpie` 和 `usemagpie`（Go module 路径、代码标识符和测试数据除外）。
- [ ] AC-7 打包流程在 `v0.1.0` 上产出 8 个文件和 release.json，release.json 中的 sha256 与文件一致，二进制中盖的版本是 0.1.0。
- [ ] AC-8 `go test ./...`（`-timeout 45m`）、GUI 测试中与更新相关的文件，以及 i18n-keys 全部通过。

## 不做

- 代码签名、公证、更新元数据签名。
- 不在本任务中改更新流程之外残留的 magpie 地址（omarchy、mcpauth、library market 等），另开任务。
