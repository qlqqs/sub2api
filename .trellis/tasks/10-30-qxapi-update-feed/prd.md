# QxAPI 客户端自动更新（sub2api 托管更新源）

## Goal

QxAPI 桌面客户端沿用 magpie 原有的自动更新机制，更新源和安装包改由 sub2api 托管：管理员在 sub2api 的独立管理页上传各版本安装包，sub2api 提供更新元数据和文件下载，客户端从这里检查、下载、安装新版本。

本任务替代 `10-25-magpie-sub2api-client` 中"安装包与更新元数据发布到 GitHub Releases、不另建更新服务"的方案（其 prd.md:109、design.md:39）。仓库 `qlqqs/qxapi-client` 是私有库，客户端无法匿名读取它的 Release。

## 已确认的决定（2026-10-30）

| 项 | 决定 |
|---|---|
| 首个版本 | `0.1.0`，tag `v0.1.0`，之后递增 |
| 平台 | Windows amd64/arm64、macOS arm64/amd64、Linux amd64/arm64 |
| 更新源 | sub2api 独立管理页上传安装包；公开 latest / notes / 文件下载接口，客户端检查更新不需要登录 |
| 完整性 | 照搬 magpie：HTTPS + 元数据中的 sha256；macOS 另比对签名者；更新元数据本身不签名 |
| Windows | 暂不签名，沿用 magpie 的试启动与回滚（9ca64d10，#894） |
| 自动更新 | 默认开启，沿用 magpie 默认值：每 6 小时检查，空闲时自动重启 |
| 更新说明 | 只写中文 |
| 版本保留 | 保留最近 5 个版本，更早的连同文件自动删除 |
| 命名 | 更新相关的 magpie 字样全部改为 qxapi；没有老用户，不做兼容 |
| 下载页 | `/downloads` 接入上传的安装包放到下一期，本期不改现有外链下载页 |

## 子任务

| 子任务 | 交付 |
|---|---|
| `10-30-client-releases`（sub2api） | 迁移、存储、管理接口、公开接口、管理页 |
| `10-30-qxapi-client-updater`（qxapi-client，分支 `qxapi`） | 更新器接入、改名、默认开启、三平台打包与 release.json |

两个子任务共用 [design.md](design.md) 中的接口约定。客户端的端到端验证依赖 sub2api 子任务先在本机测试环境可用。

## 整体验收

- [ ] AC-1 在 `sub2api-local` 上传 0.1.0 和 0.1.1 两组包并发布后，三平台各有一台装着 0.1.0 的客户端自动检查到 0.1.1，下载、校验、安装、重启后显示 0.1.1；未实际跑过的平台在验收记录中写明。
- [ ] AC-2 篡改文件导致 sha256 不符时客户端拒绝安装，原版本继续运行。
- [ ] AC-3 撤回最新版后，latest 回到上一个版本；已升级的客户端不降级。
- [ ] AC-4 发布第 6 个版本后，最老的版本及其文件被删除，latest 和 notes 不再出现它。
- [ ] AC-5 客户端界面、命令行、文件名和报错中不再出现 magpie / usemagpie.ai（Go module 路径和代码标识符除外）。
- [ ] AC-6 客户端只访问 qlqqx.com 的更新源，不再请求 usemagpie.ai。

## 本期不做

- `/downloads` 展示上传的安装包（下一期）。
- Windows 代码签名、macOS Developer ID 签名与公证。
- 更新元数据签名、灰度或强制更新、断点续传下载（magpie 均没有）。
- 提交、推送、部署七号机、上传正式版本：每项另获用户同意。
