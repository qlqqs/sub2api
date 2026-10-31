# 设计：更新源接口约定

两个子任务都按本文件实现。字段和行为照搬 magpie 客户端已经在读的格式（qxapi-client `internal/update/update.go:54-66`、`notes.go:78-107`），sub2api 只负责按这个格式输出。

## 公开接口（不登录）

前缀 `/api/v1/client-releases/qxapi`。返回**原始 JSON**，不套 sub2api 的 `{code, data}` 外壳：magpie 直接解码响应体。

### `GET /latest`

返回已发布版本中版本号最大的一个；没有已发布版本时返回 404。忽略 `lang` 参数（只有中文）。

```json
{
  "version": "0.1.1",
  "notes": "中文 markdown",
  "url": "https://api.qlqqx.com/downloads",
  "assets": {
    "qxapi-windows-amd64.exe": {
      "url": "/api/v1/client-releases/qxapi/files/0.1.1/qxapi-windows-amd64.exe",
      "size": 61689344,
      "sha256": "小写十六进制"
    }
  }
}
```

- `version` 不带 `v`。
- `url` 是客户端不能自行安装时打开的页面。本期指向现有的 `/downloads`，由管理员维护外链。由设置项 `frontend_url` 加 `/downloads` 组成；`frontend_url` 未设置时是相对路径 `/downloads`，客户端按资产 URL 的同一规则解析。
- `assets[*].url` 是站内相对路径。客户端以实际请求成功的更新源地址为基准拼出完整地址，主域名连不上、切到备用域名时，下载也跟着走同一个域名。
- `assets` 包含该版本的全部文件，DMG 也在内。客户端只按文件名挑自己需要的那个。

### `GET /notes?after=&upto=&limit=&lang=`

```json
{"releases": [{"version": "0.1.1", "notes": "...", "url": "https://api.qlqqx.com/downloads"}]}
```

- 只列已发布版本，按版本号从新到旧。
- 有 `upto` 时只列 `<= upto` 的版本，有 `after` 时只列 `> after` 的版本，有 `limit` 时最多返回 limit 条。
- `lang` 忽略。

### `GET /files/:version/:name`

- 返回文件内容，支持 Range 和 HEAD 请求。
- 只提供已发布版本的文件；草稿和已撤回版本的文件返回 404。

## 文件名（三端一致）

| 文件 | 用途 | 来源 |
|---|---|---|
| `qxapi-windows-amd64.exe`、`qxapi-windows-arm64.exe` | 首次安装与升级 | `make release-qxapi-windows-unsigned` |
| `qxapi-darwin-arm64.zip`、`qxapi-darwin-amd64.zip` | macOS 升级（`ditto -c -k --keepParent` 打包的 `QxAPI.app`） | 打包流程新增 |
| `qxapi-macos-arm64.dmg`、`qxapi-macos-amd64.dmg` | macOS 首次安装 | `tools/qxapi-dmg.py` |
| `qxapi-linux-amd64`、`qxapi-linux-arm64` | 首次安装与升级 | `make release-linux` |

客户端按 `qxapi-darwin-<arch>.zip` 和 `qxapi-<os>-<arch>[.exe]` 挑选文件。不发布单独的 CLI 包；没有图形界面的构建不参与自动更新。

## release.json（打包流程生成，上传时校验）

```json
{
  "version": "0.1.1",
  "files": {
    "qxapi-windows-amd64.exe": {"size": 61689344, "sha256": "..."}
  }
}
```

- 版本号只从 release.json 读取，不在管理页手工填写。这样可以避免元数据版本和二进制里盖的版本不一致：不一致时，客户端会认为自己一直落后，反复下载安装。
- 上传的每个文件都必须出现在 release.json 中，且 size 和 sha256 一致。
- release.json 中的文件全部上传后才能发布。

## 版本号规则

与客户端 `parse`（update.go:182-201）一致：

- 三段数字，可带 `-` 预发布后缀。
- 带 `+` 或含 `dirty` 的版本拒收。
- 比较方式与客户端 `Newer` 相同：语义化版本顺序（semver）。

## 生命周期

| 操作 | 效果 |
|---|---|
| 新建 | 上传 release.json，生成草稿（版本号不能重复） |
| 上传文件 | 逐个上传，边接收边校验 |
| 填写说明 | 中文 markdown |
| 发布 | 进入 latest 和 notes；随后按版本号保留最大的 5 个非草稿版本，更早的版本连同文件删除；比保留的 5 个都旧时拒绝发布 |
| 撤回 | 从 latest 和 notes 中移除，文件不再提供下载；可以重新发布 |
| 删除 | 草稿或已撤回版本可删除，文件一并删除 |

已升级的客户端不会降级，所以撤回只能阻止还没升级的客户端，不能把已经升上去的拉回来。
