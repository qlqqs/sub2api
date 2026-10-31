# 设计：sub2api 客户端版本发布

接口约定、文件名和生命周期见父任务 [design.md](../10-30-qxapi-update-feed/design.md)，本文件只写 sub2api 内部的实现方式。

## 数据

迁移 `backend/migrations/263_client_releases.sql`。按 fork 小表规则（`.trellis/spec/backend/database-guidelines.md`）使用原生 SQL 仓储，不新增 Ent schema，写法参考 `repository/plugin_repo.go`。

```sql
client_releases(
  id bigserial pk, product varchar(32) not null default 'qxapi',
  version varchar(64) not null, notes text not null default '',
  manifest jsonb not null,               -- release.json 原文
  status varchar(16) not null,           -- draft | published | withdrawn
  published_at timestamptz, created_at, updated_at,
  unique(product, version))

client_release_files(
  id bigserial pk, release_id bigint references client_releases on delete cascade,
  name varchar(128) not null, size bigint not null, sha256 char(64) not null,
  created_at, unique(release_id, name))
```

文件按版本号排序：在 Go 里用与客户端相同的解析和比较函数，不在 SQL 里排序。每个产品最多几十条记录，没有性能问题。

## 存储

- 根目录：`$DATA_DIR/client-releases`，`DATA_DIR` 为空时用 `./data`，与 `resolvePluginRootDir`（`service/plugin_package.go:45`）同一规则。生产环境落在 `/app/data` 卷上。
- 文件路径：`<root>/qxapi/<version>/<name>`。`version` 和 `name` 都已通过校验：版本号规则，文件名白名单 `^qxapi-(windows|darwin|macos|linux)-(amd64|arm64)(\.exe|\.zip|\.dmg)?$`。
- 上传：`PUT /api/v1/admin/client-releases/:id/files/:name`，请求体就是文件本身，不用 multipart。这样不经过 Gin 的 32MB 内存上限，也不会把内容写到容器的 `/tmp`。
  - 先用 `http.MaxBytesReader` 把请求体限制在 release.json 声明的 size。
  - 边写临时文件 `<name>.part`，边计算 sha256。
  - 大小和 sha256 都对上后，改名为正式文件并写入数据库；不一致时删掉 `.part`，返回 400。
- 删除版本时连同目录一起删除。

## 接口

管理接口在 `routes/admin.go` 的 admin 组下新增 `/client-releases`：

| 方法 | 路径 | 说明 | TOTP |
|---|---|---|---|
| GET | `` | 列表，含每个版本的文件上传情况 | |
| POST | `` | 请求体为 release.json，创建草稿 | ✓ |
| POST | `/:id/files/:name/check` | 上传前检查：状态允许上传、文件名在 release.json 中；返回 `null` | ✓ |
| PUT | `/:id/files/:name` | 上传一个文件 | ✓ |
| PUT | `/:id/notes` | 修改更新说明 | |
| POST | `/:id/publish` | 发布并执行保留 5 个版本的清理 | ✓ |
| POST | `/:id/withdraw` | 撤回 | ✓ |
| DELETE | `/:id` | 删除草稿或已撤回版本 | ✓ |

管理接口用 `response.Success` 包装，`data` 的格式如下（前后端按此对接）：

```ts
interface ClientRelease {
  id: number
  product: string            // 'qxapi'
  version: string
  notes: string
  status: 'draft' | 'published' | 'withdrawn'
  published_at: string | null
  created_at: string
  updated_at: string
  files: Array<{             // 按 release.json 中的文件名排序
    name: string
    size: number             // release.json 声明的大小
    sha256: string
    uploaded: boolean        // client_release_files 中有这一行
  }>
}
```

- `GET` 返回 `ClientRelease[]`，按版本号从新到旧。
- `POST`（Content-Type `application/json`，请求体为 release.json 原文）、`PUT files`（Content-Type `application/octet-stream`）、`PUT notes`（`{"notes": "..."}`）、`POST withdraw` 返回更新后的 `ClientRelease`。
- `POST publish` 返回 `{"release": ClientRelease, "removed_versions": ["0.1.0"]}`，`removed_versions` 是这次清理掉的版本。
- `DELETE` 返回 `null`。
- 错误用 `infraerrors`，message 用英文（与现有代码一致），前端按 reason 显示中英文文案，未知 reason 显示 message：

  | reason | HTTP | 场景 |
  |---|---|---|
  | `CLIENT_RELEASE_INVALID_MANIFEST` | 400 | release.json 解析失败、没有文件、size/sha256 格式不对 |
  | `CLIENT_RELEASE_INVALID_VERSION` | 400 | 版本号不符合规则 |
  | `CLIENT_RELEASE_INVALID_FILE_NAME` | 400 | 文件名不在白名单 |
  | `CLIENT_RELEASE_VERSION_EXISTS` | 409 | 版本号已存在 |
  | `CLIENT_RELEASE_NOT_FOUND` | 404 | id 不存在 |
  | `CLIENT_RELEASE_FILE_NOT_IN_MANIFEST` | 400 | 上传的文件不在 release.json 中 |
  | `CLIENT_RELEASE_FILE_MISMATCH` | 400 | 上传内容的大小或 sha256 与 release.json 不符 |
  | `CLIENT_RELEASE_FILES_MISSING` | 400 | 发布时还有文件没上传 |
  | `CLIENT_RELEASE_TOO_OLD` | 409 | 发布版本落在保留的 5 个非草稿版本之外（撤回计入），不改变状态或删除文件；message: `Release is older than the five retained versions` |
  | `CLIENT_RELEASE_STATUS_CONFLICT` | 409 | 当前状态不允许该操作（只有草稿和已撤回可上传、删除；已发布不能再发布；只有已发布可撤回） |

  草稿和已撤回版本可以重新上传同名文件，覆盖原文件。

- 前端每个文件先调 `check`，通过后再 `PUT`。step-up 授权只有 15 分钟（`StepUpGrantTTL`），逐个上传几百 MB 可能超时；如果直接 `PUT` 时才被 `STEP_UP_REQUIRED` 拒绝，浏览器已经发出整个文件，服务端不读请求体就关闭连接，浏览器往往只看到连接被重置，拿不到 403。`check` 不带文件，过期时能正常弹出 TOTP 对话框。

公开接口在新文件 `routes/client_releases.go` 中注册，挂在 v1 下，使用 `panelRateLimiter.PublicIP()`：

- `GET /client-releases/qxapi/latest`、`GET /client-releases/qxapi/notes`：用 `c.JSON` 直接输出约定结构，不用 `response.Success`。
- `GET`/`HEAD /client-releases/qxapi/files/:version/:name`：从数据库查到已发布版本中的这个文件后，再 `c.File`，Range 和 HEAD 由 `http.ServeContent` 处理。路径只从数据库拼出，不直接使用请求里的路径。

更新说明里的 `url` 由 `SettingKeyFrontendURL`（`frontend_url`）加 `/downloads` 组成。

## 分层

- `service/client_release_service.go`：版本解析与比较、release.json 校验、上传、发布、清理。
- `repository/client_release_repo.go`：原生 SQL。
- `handler/admin/client_release_handler.go`、`handler/client_release_handler.go`。
- 在 wire 中注册：`handler/handler.go`、`handler/wire.go`、`cmd/server/wire_gen.go`，并加 `// CUSTOM` 标记。

## 前端

- 路由 `/admin/client-releases`（`router/index.ts`，`requiresAdmin`）。
- 侧边栏：`AppSidebar.vue` 和 `themes/qx/shell/useQxNav.ts` 的 system 分组都要加，`useQxNav.drift.spec.ts` 会检查两边一致。
- 页面 `views/admin/ClientReleasesView.vue`：列表、对话框和确认框参照 `AnnouncementsView.vue`，文件选择和 TOTP 验证参照 `PluginsView.vue`。
- 上传：`api/admin/clientReleases.ts`，请求体直接放 `File`，设 `timeout: 0` 和 `onUploadProgress`。一次选中多个文件，按顺序逐个上传，并显示每个文件的进度和校验结果。
- i18n：`i18n/locales/{zh,en}/admin/clientReleases.ts`，并在 `admin/index.ts` 中汇总。

## 回滚

迁移只新增表。回退代码后，两张新表和数据目录残留不影响其他功能。
