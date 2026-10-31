# sub2api 客户端版本发布页与更新源接口

## Goal

在 sub2api 增加一个独立的管理页，管理员在这里上传 QxAPI 各版本、各平台的安装包并发布；sub2api 按父任务 [design.md](../10-30-qxapi-update-feed/design.md) 的约定提供不需登录的 latest、notes 和文件下载接口，供 QxAPI 客户端自动更新。

## Requirements

- R1 管理页 `/admin/client-releases`：版本列表（版本号、状态、文件数、发布时间）、新建、上传文件、填写中文更新说明、发布、撤回、删除。
- R2 新建版本时上传打包流程生成的 release.json，版本号和文件清单从中读取；版本号按约定校验，不能重复。
- R3 文件逐个上传，上传时显示进度；文件名、大小、sha256 必须与 release.json 一致，不一致的拒收，不留下残留文件。单个文件按 200MB 量级考虑。
- R4 release.json 中的文件全部上传后才能发布。
- R5 发布后只保留版本号最大的 5 个非草稿版本，更早的版本连同文件自动删除。
- R6 上传、发布、撤回、删除需要管理员登录，并再次验证 TOTP（与插件上传一致）。
- R7 公开接口 latest、notes、files 不需要登录，按约定输出原始 JSON；文件下载支持 Range 和 HEAD。
- R8 文件保存在持久数据卷 `/app/data` 下，容器重建后仍在。
- R9 中英文界面文案齐全，侧边栏两处入口（`AppSidebar.vue`、`useQxNav.ts`）都要加。

## Acceptance Criteria

- [ ] AC-1 在 `sub2api-local` 新建 0.1.0，上传全部文件并发布后，`curl /api/v1/client-releases/qxapi/latest` 返回约定格式，各文件的 sha256 与本地计算的一致。
- [ ] AC-2 上传一个内容被改过的文件时被拒收，存储目录里没有残留。
- [ ] AC-3 缺文件时不能发布；未登录、非管理员或未通过 TOTP 时，管理接口全部拒绝。
- [ ] AC-4 `curl -r` 分段下载拼接后与原文件 sha256 一致；HEAD 返回正确的 Content-Length。
- [ ] AC-5 notes 的 after、upto、limit 行为符合约定；草稿和已撤回版本不出现在 latest、notes、files 中。
- [ ] AC-6 依次发布 6 个版本后，最老版本的记录和文件目录都已删除。
- [ ] AC-7 `docker compose up -d` 重建容器后，文件和下载正常。
- [ ] AC-8 管理页在中文和英文、浅色和深色主题、窄窗口下显示正常；后端单测和前端 lint、类型检查、单测通过。

## 不做

- `/downloads` 展示上传的安装包（下一期）。
- 多产品或多渠道：路径里的 `qxapi` 先写死，不做产品管理。
