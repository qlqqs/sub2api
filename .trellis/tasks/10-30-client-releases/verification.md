# 本机链路验证记录

## 结论与环境

**PASS：本轮要求的 1–8 项均完成，未发现功能 bug。**

- 执行时间：服务端时间 2026-10-31 04:03:42–04:05:25（UTC+08:00）。
- 目标：`http://127.0.0.1:28080`，compose 项目 `sub2api-local`。
- 使用用户已构建并启动的 `1d690e1f4-dirty` 工作区镜像；实际镜像 ID：`sha256:d77040426325d49a56549e564705999a24baa7fe60da9ab4ef482b2d131479bc`。
- 全部 HTTP 操作由脚本调用 curl；统一 `--noproxy '*'`，非法路径测试使用 `--path-as-is`。
- 未重新 build、未 reset 数据库、未修改代码、未改变设置、未提交/推送/部署。唯一仓库新增文件是本记录。
- 新建的空工作目录：`/tmp/client-releases-e2e.sdOWpq`。其中 `verify.py` 是本轮驱动，`events.jsonl` 保存请求路径、状态、关键响应和断言证据；每次请求另存响应体和响应头。
- 初始管理列表为 `[]`；结束后保留五个已发布版本 `0.9.2`–`0.9.6`。没有创建或修改 `0.0.x`。

下文命令使用：

```bash
BASE=http://127.0.0.1:28080
A=/api/v1/admin/client-releases
P=/api/v1/client-releases/qxapi
RUN=/tmp/client-releases-e2e.sdOWpq
# 管理请求运行时使用 -H "@$RUN/auth.header"；该临时文件由登录 token 生成。
```

## 1. 管理员登录

从 `/home/qlqq/workspace/sub2api-local/.env` 读取 `ADMIN_EMAIL` / `ADMIN_PASSWORD`，在 Python 内构造 JSON，经 stdin 传入 curl：

```bash
curl --noproxy '*' -sS -X POST "$BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' --data-binary @-
```

实际：**HTTP 200**，返回 access_token，用户角色 `admin`。随后 `GET $A` **200**，`data: []`。

只读查询管理设置得到：`step_up_enabled: false`、`frontend_url: ""`。本轮没有触发 STEP_UP_REQUIRED，也没有切换任何设置；公开接口的 `url: "/downloads"` 与本机空 frontend_url 一致。凭据/token 不写入本记录，执行用的临时凭据响应和 header 文件在结束后删除。

## 2. 占位文件与 release.json

先 `mktemp -d /tmp/client-releases-e2e.XXXXXX`，再用 Python `os.urandom` 写文件，按实际字节数和 SHA256 生成各版本 `release.json`。

- `0.9.1`、`0.9.6`：均包含以下 **8 个文件名全集**；windows-amd64 文件各为独立随机的 **64 MiB = 67,108,864 字节**。
- `0.9.2`–`0.9.5`：各包含 `qxapi-linux-amd64`，536 字节。
- 所有文件都是本机验证占位内容，不是客户端安装包。

下表为最终保留的 `0.9.6`；重建后逐个完整下载并验证，SHA256 均匹配本地文件、release.json 和 latest 元数据。

| 文件 | 字节数 | SHA256 |
|---|---:|---|
| `qxapi-windows-amd64.exe` | 67108864 | `3f2fb597f28f4f4a8325853315b87ea6cd060303c6da6dd0fb3ecbadf3bd908c` |
| `qxapi-windows-arm64.exe` | 542 | `7a7d50f429853cdf6842661543b7ef3bc4cfe2ece672353220fbae90d2749932` |
| `qxapi-darwin-arm64.zip` | 541 | `bea1b745ca72d1686bd9aa7ab27c248f2dbd507690e64ef792a5071556e31138` |
| `qxapi-darwin-amd64.zip` | 541 | `c29465c55f247f3fd2b8b5d3aeba9da2b99afdfe51f7ebfb5f9daa2eb6bf034d` |
| `qxapi-macos-arm64.dmg` | 540 | `cf2f8f7c60a67cedac515bfd9dea9c019cc09ee8f5f17cb2ad313f82d7383e7a` |
| `qxapi-macos-amd64.dmg` | 540 | `fefc8b70bfb8fc7029d82fbce4d580f4dc9032460408e61447e804cc9fff59f7` |
| `qxapi-linux-amd64` | 536 | `b9128748bd484f13b6b7df15784b754bf5d570f944d387e76cc288ab91a96002` |
| `qxapi-linux-arm64` | 536 | `968b910dc931bdd837e1034ab7745b235eeeb3d0a5ef095b984e8cdf12b67022` |

## 3. 新建 → check → PUT → notes → 发布及公开下载

实际逐版本、逐文件执行（非 multipart）：

```bash
curl --noproxy '*' -sS -X POST -H "@$RUN/auth.header" \
  -H 'Content-Type: application/json' --data-binary "@$RUN/0.9.6/release.json" "$BASE$A"
curl --noproxy '*' -sS -X POST -H "@$RUN/auth.header" "$BASE$A/7/files/qxapi-windows-amd64.exe/check"
curl --noproxy '*' -sS -X PUT -H "@$RUN/auth.header" \
  -H 'Content-Type: application/octet-stream' \
  --data-binary "@$RUN/0.9.6/qxapi-windows-amd64.exe" "$BASE$A/7/files/qxapi-windows-amd64.exe"
curl --noproxy '*' -sS -X PUT -H "@$RUN/auth.header" \
  -H 'Content-Type: application/json' \
  --data-binary '{"notes":"## 本机验证 0.9.6\n\n- 中文更新说明\n- curl 上传、校验和下载"}' "$BASE$A/7/notes"
curl --noproxy '*' -sS -X POST -H "@$RUN/auth.header" "$BASE$A/7/publish"
```

所有成功操作均 **HTTP 200**。check 的实际响应：

```json
{"code":0,"message":"success","data":null}
```

新建返回 `status: "draft"`、文件 `uploaded: false`；每次 PUT 返回对应文件 `uploaded: true`；发布后 `status: "published"`，中文 notes 原样保留。

| 版本 | ID | 文件数 | 新建/check/上传/notes/发布 |
|---|---:|---:|---|
| 0.9.1 | 1 | 8 | 均 200 |
| 0.9.2 | 3 | 1 | 均 200 |
| 0.9.3 | 4 | 1 | 均 200 |
| 0.9.4 | 5 | 1 | 均 200 |
| 0.9.5 | 6 | 1 | 均 200 |
| 0.9.6 | 7 | 8 | 均 200 |

### latest / 完整下载 / Range / HEAD

```bash
curl --noproxy '*' -sS "$BASE$P/latest?lang=en"
curl --noproxy '*' -sS -D "$RUN/download.headers" \
  -o "$RUN/download.bin" "$BASE$P/files/0.9.6/qxapi-windows-amd64.exe"
curl --noproxy '*' -sS -r 0-99 -D "$RUN/range.headers" \
  -o "$RUN/range.bin" "$BASE$P/files/0.9.6/qxapi-windows-amd64.exe"
curl --noproxy '*' -sS -I "$BASE$P/files/0.9.6/qxapi-windows-amd64.exe"
```

实际结果：

- latest **200**，原始 JSON 顶层为 `version/notes/url/assets`，没有 code/data 包装；首次版本 `0.9.1`，最终 `0.9.6`，各包含 8 个 assets。`lang=en` 未改变中文 notes。
- `url` 为 `/downloads`；asset URL 为 `/api/v1/client-releases/qxapi/files/<version>/<name>`，size/sha256 与生成清单逐项一致。
- 完整文件 **200**，`Content-Type: application/octet-stream`，`Content-Length: 67108864`。
- `-r 0-99` **206**，`Content-Range: bytes 0-99/67108864`，`Content-Length: 100`；响应字节与原文件前 100 字节一致。
- `-I` **200**，`Content-Length: 67108864`，`Accept-Ranges: bytes`。curl `-I` 的输出文件是响应头，不将该文件字节数当作 HTTP body 长度。
- `0.9.1` 的 8 个文件全部完整下载，SHA256 均匹配；64 MiB 文件的实际 SHA256：

```text
c3b992367f1c50ed05e475b3772a91df53c3fafd408a002d20602c097b71ef89
```

### notes 查询组合

均为 `curl --noproxy '*' -sS "$BASE$P/notes?<query>"`，全部 **200**，原始 JSON `{"releases":[...]}`：

| query | 返回版本（顺序） |
|---|---|
| 无参数，最终状态 | 0.9.6、0.9.5、0.9.4、0.9.3、0.9.2 |
| `after=0.9.3` | 0.9.6、0.9.5、0.9.4 |
| `upto=0.9.4` | 0.9.4、0.9.3、0.9.2 |
| `after=0.9.2&upto=0.9.5&limit=2&lang=en` | 0.9.5、0.9.4 |
| `after=0.9.6&upto=0.9.6&limit=1` | `[]` |

组合查询实际响应：

```json
{
  "releases": [
    {
      "version": "0.9.5",
      "notes": "## 本机验证 0.9.5\n\n- 中文更新说明\n- curl 上传、校验和下载",
      "url": "/downloads"
    },
    {
      "version": "0.9.4",
      "notes": "## 本机验证 0.9.4\n\n- 中文更新说明\n- curl 上传、校验和下载",
      "url": "/downloads"
    }
  ]
}
```

## 4. 拒绝场景

以下是实际请求与结果，路径以 `/api/v1` 开头；错误 reason 保留完整值。

| 场景 | 方法与路径 | HTTP | reason |
|---|---|---:|---|
| 未登录读取管理列表 | `GET /api/v1/admin/client-releases` | 401 | 未返回 reason 字段 |
| 未登录新建 | `POST /api/v1/admin/client-releases` | 401 | 未返回 reason 字段 |
| 重复版本 0.9.1 | `POST /api/v1/admin/client-releases` | 409 | `CLIENT_RELEASE_VERSION_EXISTS` |
| 缺文件发布 | `POST /api/v1/admin/client-releases/1/publish` | 400 | `CLIENT_RELEASE_FILES_MISSING` |
| 草稿下载（未上传） | `GET /api/v1/client-releases/qxapi/files/0.9.1/qxapi-linux-amd64` | 404 | `CLIENT_RELEASE_NOT_FOUND` |
| 篡改同长度内容 | `PUT /api/v1/admin/client-releases/1/files/qxapi-linux-amd64` | 400 | `CLIENT_RELEASE_FILE_MISMATCH` |
| 超出声明大小 | `PUT /api/v1/admin/client-releases/1/files/qxapi-linux-amd64` | 400 | `CLIENT_RELEASE_FILE_MISMATCH` |
| 非法版本号 | `GET /api/v1/client-releases/qxapi/files/dev/qxapi-linux-amd64` | 400 | `CLIENT_RELEASE_INVALID_VERSION` |
| 编码 .. 版本号 | `GET /api/v1/client-releases/qxapi/files/%2e%2e/qxapi-linux-amd64` | 400 | `CLIENT_RELEASE_INVALID_VERSION` |
| 非法文件名 | `GET /api/v1/client-releases/qxapi/files/0.9.1/secrets.txt` | 400 | `CLIENT_RELEASE_INVALID_FILE_NAME` |
| 编码 .. 文件名 | `GET /api/v1/client-releases/qxapi/files/0.9.1/%2e%2e` | 400 | `CLIENT_RELEASE_INVALID_FILE_NAME` |
| 草稿下载（已上传） | `GET /api/v1/client-releases/qxapi/files/0.9.1/qxapi-linux-amd64` | 404 | `CLIENT_RELEASE_NOT_FOUND` |
| 撤回版本下载 | `GET /api/v1/client-releases/qxapi/files/0.9.2/qxapi-linux-amd64` | 404 | `CLIENT_RELEASE_NOT_FOUND` |
| 清理后的旧版本下载 | `GET /api/v1/client-releases/qxapi/files/0.9.1/qxapi-linux-amd64` | 404 | `CLIENT_RELEASE_NOT_FOUND` |

篡改文件：将 `0.9.1/qxapi-linux-amd64` 第一个字节 XOR 1，保持 **536 字节**；超长文件：原文件末尾增加 1 字节，实际 **537 字节**、清单声明 **536 字节**。两次均先 check **200**，再 PUT **400**：

```json
{"code": 400, "message": "File size or SHA256 does not match the manifest", "reason": "CLIENT_RELEASE_FILE_MISMATCH"}
```

每次失败后立即检查：

```bash
docker exec sub2api-local sh -c 'find /app/data/client-releases -type f -name "*.part" -print'
docker exec sub2api-local sh -c 'test ! -e /app/data/client-releases/qxapi/0.9.1/qxapi-linux-amd64 && echo absent'
```

两轮均退出 0：find 无输出，正式文件检查输出 `absent`。全部测试结束后再次检查，无 `.part`。

编码路径保留原始编码发送，未被 curl 先规范化：

```bash
curl --noproxy '*' --path-as-is -i "$BASE$P/files/%2e%2e/qxapi-linux-amd64"
curl --noproxy '*' --path-as-is -i "$BASE$P/files/0.9.1/%2e%2e"
```

两者均 400，分别为 INVALID_VERSION / INVALID_FILE_NAME，没有返回文件内容。

## 5. 撤回最新版后回退，再发布

当时已发布 `0.9.1`、`0.9.2`：

```bash
curl --noproxy '*' -sS -X POST -H "@$RUN/auth.header" "$BASE$A/3/withdraw"
curl --noproxy '*' -sS "$BASE$P/latest"
curl --noproxy '*' -sS "$BASE$P/notes"
curl --noproxy '*' -sS -X POST -H "@$RUN/auth.header" "$BASE$A/3/publish"
```

实际：

1. 撤回前 latest **200**，`version: "0.9.2"`。
2. withdraw **200**，`status: "withdrawn"`。
3. `0.9.2` 文件下载 **404**；latest **200** 回到 `0.9.1`；notes **200** 只包含 `0.9.1`。
4. 重新发布 **200**，`removed_versions: []`；latest **200** 恢复 `0.9.2`。

## 6. 发布第六个版本，清理最老版本

发布到 `0.9.5` 时：notes 含五个版本；`test -d /app/data/client-releases/qxapi/0.9.1` 退出 0，目录存在。

发布 `0.9.6`（ID 7）**HTTP 200**，响应关键字段：

```json
{"code":0,"data":{"release":{"id":7,"version":"0.9.6","status":"published"},"removed_versions":["0.9.1"]}}
```

核对结果：

- 管理列表、notes 均只包含 `["0.9.6","0.9.5","0.9.4","0.9.3","0.9.2"]`；latest 为 `0.9.6`。
- `test ! -e /app/data/client-releases/qxapi/0.9.1 && echo absent`：退出 0，输出 `absent`。
- `0.9.1` 文件下载 **404 CLIENT_RELEASE_NOT_FOUND**。
- 前五个版本发布和 `0.9.2` 重发均返回 `removed_versions: []`，第六个版本准确返回 `["0.9.1"]`。

## 7. force-recreate 后再次下载

执行用户指定命令，没有 build/pull/reset：

```bash
cd /home/qlqq/workspace/sub2api-local
docker compose up -d --force-recreate sub2api
```

实际命令退出 0：

```text
 Container sub2api-local-redis  Running
 Container sub2api-local-postgres  Running
 Container sub2api-local  Recreate
 Container sub2api-local  Recreated
 Container sub2api-local-postgres  Waiting
 Container sub2api-local-redis  Waiting
 Container sub2api-local-postgres  Healthy
 Container sub2api-local-redis  Healthy
 Container sub2api-local  Starting
 Container sub2api-local  Started

```

- 重建前容器：`918f86bac7a1baa1b212bd32563a435168e9d087a9f795fc08e6c33c4c8e4795`。
- 重建后容器：`94cf187b80562d6255a5ae4875eb90de44b0866c524423149c6d30c8ce074c06`。
- 镜像 SHA256 不变；挂载仍为 `sub2api-local_app_data:/app/data`。
- 等待 `/health` 返回 **200 {"status":"ok"}** 后，重新完整下载 `0.9.6` 的全部 8 个文件，均 **200**，SHA256 全部通过。
- 64 MiB 文件的本地原文件、重建前下载、重建后下载 SHA256 三者相同：

```text
3f2fb597f28f4f4a8325853315b87ea6cd060303c6da6dd0fb3ecbadf3bd908c
```

- 重建后再次 `-r 0-99`：**206**，`Content-Range: bytes 0-99/67108864`；再次 `-I`：**200**，`Content-Length: 67108864`。
- latest 仍为 `0.9.6`，管理列表仍为五个已发布版本，文件 `uploaded: true`。

## 8. 留存状态

未手工执行清理接口或 SQL。`0.9.1` 仅由发布时的保留策略自动删除；其余保留：

| 版本 | ID | 状态 | 文件数 |
|---|---:|---|---:|
| 0.9.6 | 7 | published | 8 |
| 0.9.5 | 6 | published | 1 |
| 0.9.4 | 5 | published | 1 |
| 0.9.3 | 4 | published | 1 |
| 0.9.2 | 3 | published | 1 |

容器内实际目录：

```text
/app/data/client-releases/qxapi/0.9.2
/app/data/client-releases/qxapi/0.9.3
/app/data/client-releases/qxapi/0.9.4
/app/data/client-releases/qxapi/0.9.5
/app/data/client-releases/qxapi/0.9.6

```

本轮没有前端浏览器/UI 验证，未修改前端或后端代码。未发现需要修复的问题。

## 9. 过旧版本拒绝发布修复与回归

本轮完成时间：2026-10-31 04:16:40 +08:00（执行机实际时间）。本节是界面测试发现问题后的独立修复轮次；前述首次验收记录保留不变。本轮按用户授权修改后端、执行 `--no-web` 构建并更新本机容器。

### 修复与覆盖

原实现将新发布的旧版本计入保留排序后立即删除，旧测试也认可这一行为。现在明确发布前置条件：候选版本必须处于按版本号排序最大的五个非草稿版本内，撤回版本计入；否则在调用仓储发布和清理文件之前返回 `CLIENT_RELEASE_TOO_OLD`，不改变状态或文件。删除原来的“发布后自身被清理”处理分支。

- service 回归：五个更高版本，分别包含 0、1、5 个撤回版本，发布 `0.0.1` 均拒绝；断言不调用仓储 Publish、记录/时间戳完全不变、全部文件内容保留。
- 边界回归：只有四个非草稿版本时，旧版本仍能作为第五个发布；额外的更高草稿不占保留名额。
- handler 回归：HTTP 409、reason 和 message 精确匹配；草稿状态及文件保持不变。
- 子任务 reason 表、父任务生命周期发布规则已同步。

### 单测、格式与静态检查

```bash
cd backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY \
  -u ALL_PROXY -u all_proxy -u OPENAI_API_KEY \
  go test -tags unit ./internal/service/... ./internal/handler/...
gofmt -l .
go vet ./...
```

均退出 0。`gofmt -l .`、`go vet ./...` 无输出。单测实际输出：

```text
ok  	github.com/Wei-Shaw/sub2api/internal/service	217.111s
ok  	github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2	(cached)
ok  	github.com/Wei-Shaw/sub2api/internal/handler	42.988s
ok  	github.com/Wei-Shaw/sub2api/internal/handler/admin	1.170s
ok  	github.com/Wei-Shaw/sub2api/internal/handler/dto	0.035s
ok  	github.com/Wei-Shaw/sub2api/internal/handler/quotaview	0.034s
```

日志：`/tmp/client-release-too-old-tests.log`、`/tmp/client-release-too-old-gofmt.log`、`/tmp/client-release-too-old-vet.log`。`git diff --check` 通过。

### 本机构建与启动

```bash
cd /home/qlqq/workspace/sub2api-local
./build.sh --no-web && docker compose up -d
```

退出 0；镜像 `sub2api-local:latest (1d690e1f4-dirty)`，镜像 ID：

```text
sha256:dc40596cf24f0845e7b4d96cfbfe5c56eff04cdb8b52cafa98b36449ae8e22d9
```

应用容器成功 Recreated / Started。首次紧接启动的健康探测返回以下原始输出（curl 退出 56，验收脚本尚未运行）：

```text
curl: (56) Recv failure: Connection reset by peer
tail: cannot open '/tmp/client-release-too-old.bspbis/run.log' for reading: No such file or directory
```

随后使用 `curl --noproxy '*' --retry 20 --retry-delay 2 --retry-all-errors --fail -sS http://127.0.0.1:28080/health`，返回 **200 `{"status":"ok"}`**，再启动回归脚本。

### curl 回归实际结果

脚本和响应证据：`/tmp/client-release-too-old.bspbis/verify.py`、`events.jsonl`、`run.log`、各步骤的响应 JSON/header。请求全部通过 curl 发到 `127.0.0.1:28080`；管理员凭据读取本机 `.env`，token 不写入仓库，临时认证文件已删除。

关键命令模式（`$BASE` 为本机地址，`$AUTH` 为临时 Authorization header 文件，`$TMP` 为上述临时目录）：

```bash
# 登录请求体从 .env 生成，经 stdin 传递，不打印凭据
curl --noproxy '*' -sS -X POST -H 'Content-Type: application/json' \
  --data-binary @- "$BASE/api/v1/auth/login"
curl --noproxy '*' -sS -X POST -H "@$AUTH" -H 'Content-Type: application/json' \
  --data-binary "@$TMP/release.json" "$BASE/api/v1/admin/client-releases"
curl --noproxy '*' -sS -X POST -H "@$AUTH" \
  "$BASE/api/v1/admin/client-releases/9/files/qxapi-linux-amd64/check"
curl --noproxy '*' -sS -X PUT -H "@$AUTH" -H 'Content-Type: application/octet-stream' \
  --data-binary "@$TMP/qxapi-linux-amd64" "$BASE/api/v1/admin/client-releases/9/files/qxapi-linux-amd64"
curl --noproxy '*' -sS -X POST -H "@$AUTH" "$BASE/api/v1/admin/client-releases/9/publish"
```

| 操作 | HTTP | 实际结果 |
|---|---:|---|
| 登录 | 200 | 成功取得 token；未遇到 step-up 拒绝，未更改设置 |
| 初始管理列表 | 200 | `0.9.6` 至 `0.9.2` 五个已发布版本 |
| 创建 `0.0.1` | 200 | ID `9`，`status: draft`，`published_at: null` |
| 上传前 check | 200 | `{"code":0,"message":"success","data":null}` |
| PUT 上传 | 200 | `qxapi-linux-amd64`，25 bytes，`uploaded: true` |
| 发布 `0.0.1` | **409** | 下方精确响应；未发布、未清理文件 |
| 拒绝后管理列表 | 200 | 与发布请求前完整记录逐项相等，含状态和时间戳 |
| latest / notes | 200 / 200 | 与发布前完整响应相等；latest 仍是 `0.9.6` |
| 下载草稿文件 | 404 | `CLIENT_RELEASE_NOT_FOUND` |
| DELETE `/api/v1/admin/client-releases/9` | 200 | `{"code":0,"message":"success","data":null}` |
| 最终管理列表 | 200 | 完全恢复初始五条记录，保留 `0.9.2–0.9.6` |

发布原始响应：

```json
{"code":409,"message":"Release is older than the five retained versions","reason":"CLIENT_RELEASE_TOO_OLD"}
```

发布请求前后分别执行：

```bash
docker exec sub2api-local sh -c \
  'find /app/data/client-releases/qxapi -type f -exec sha256sum {} \; | sort'
```

全部文件路径及 SHA256 完全一致。测试文件 SHA256：

```text
a039180cf7f83f937b5e8dd7521446bd1ac02c22a176a5e17d00f242dd326e66
```

最后通过 DELETE 删除本次创建的 ID 9，并确认：

```bash
docker exec sub2api-local sh -c \
  'test ! -e /app/data/client-releases/qxapi/0.0.1 && echo absent'
# absent
```

脚本退出 0，记录 `PASS`。本轮未改 frontend/，未重置数据库，未提交、推送或部署七号机。与更新后的设计无出入。

## 10. 管理页界面（Playwright，真实登录）

脚本：`sub2api-local/e2e/ui/client_releases_ui.sh <输出目录>`（仓库外）。最终一轮：`e2e/runs/client-releases-ui-202521/`，输出 `{"ok":true,"version":"1.0.391922"}`。

- 侧边栏"系统"分组出现"客户端版本"。
- 选 release.json 新建草稿，显示"已上传 0/3 个文件"。
- 一次选 3 个文件：正确文件上传成功；内容改过一个字节的文件显示"文件大小或 SHA-256 与 release.json 不一致"；`stray.txt` 显示"不在 release.json 中"且不上传；此时发布按钮不可点。
- 重新选正确文件和剩余文件后"已上传 3/3"，填写说明、确认框发布后状态变为"已发布"；`latest` 的 version 和 3 个 sha256 与 release.json 一致。
- 中文/英文 × 浅色/深色 1440px、390px 窄窗口截图，页面横向溢出均为 0；窄窗口下文件表在卡片内横向滚动。
- 第一轮暴露"比保留的 5 个都旧的版本发布后立即被清理"，已改为 409 `CLIENT_RELEASE_TOO_OLD`（见第 9 节）。

## 11. 最终代码上的全量检查

- 后端：`go test -tags unit ./...`（去掉代理和 OPENAI_API_KEY）退出码 0，58 个包 ok；`gofmt -l` 无输出。日志 `/tmp/client-release-unit-final2.log`。
- 前端：`lint:check`、`typecheck`、`check:i18n`、`test:run`（441 文件、3516 用例）通过，无锁文件改动；之后只改了两条 i18n 文案，`check:i18n` 再次通过。
