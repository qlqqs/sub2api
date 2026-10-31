# 后端实现与验证记录

范围：`implement.md` 步骤 1–3，以及补充的上传前 `check` 接口。不修改 frontend、不提交、不推送、不部署；本机链路验证等用户下一轮通知。

## 实现

- 迁移 263：两张独立 SQL 表；清单原文使用 JSONB，版本唯一，文件随版本级联删除。
- service：客户端同源版本比较、清单校验、流式上传、发布/撤回/删除、按版本号保留五个非草稿版本（包括 withdrawn）。
- 管理接口：契约 DTO、错误 reason/HTTP 状态；check 与 PUT 复用前置校验；需要 TOTP 的路由挂现有 stepUpAuth。
- 公开接口：原始 latest/notes JSON；文件仅从已发布版本的数据库记录解析路径，通过 c.File 提供 Range/HEAD；挂 PublicIP 限流。
- Wire：更新 provider 并重新生成 wire_gen.go；原文件与生成结果均带 CUSTOM 标记。
- 请求体链：全局默认 256 MiB，使用 MaxBytesHandler，无整包预读。仅上传路由跳过审计 body 捕获，操作审计保留。测试通过 200 MiB 原始请求体并核对落盘 SHA256。
- `response.Success` 的 `data` 带 omitempty，因此 check/Delete 传入 typed nil，确保按契约输出 `data: null`，不修改公共 response helper。

## 测试覆盖

- 直接复用 qxapi-client 的 Newer/Released 表驱动用例。
- release.json：解析、版本、文件白名单、size、sha256；版本归一化与重复。
- 上传：短包、超长、哈希错误、MaxBytesReader、重复覆盖、同版本并发上传、失败清理 `.part`、200 MiB 流式请求。
- 生命周期：缺文件拒绝发布、重复发布/非法状态、撤回/重发、删除、五版本保留（含撤回版本、排除草稿、过旧版本以 CLIENT_RELEASE_TOO_OLD 拒绝且状态/文件不变；四个非草稿版本时仍允许旧版本作为第五个发布）。
- 仓储：读写与错误映射、文件 upsert、发布+清理的事务提交/回滚。
- handler/routes：完整管理响应格式、check 全部 reason、管理员认证/step-up、原始公开 JSON、Range 分段拼接、HEAD、404、非法路径参数、notes after/upto/limit、PublicIP 共享限流。
- 审计：200 MiB 声明请求在 step-up 拒绝前没有读取 body，仍记录操作。

## 验证命令

在 backend 下：

```bash
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY -u all_proxy -u OPENAI_API_KEY go test -tags unit ./...
go build ./...
go vet ./...
gofmt -l .
```

- 定向 `go test -tags unit ... -run TestClientRelease -count=1`：六个包全部通过。
- build / vet：通过，无输出。
- gofmt：全 backend Go 文件扫描，无输出。
- git diff --check -- backend：通过。
- 最终全量单测：全部通过，退出码 0（service 215.333s、handler 44.182s、admin handler 1.189s、routes 10.487s）。完整输出：`/tmp/client-release-unit-final.log`。
- 未运行 sub2api-local build、curl 链路和容器重建测试；留待前后端一起验证。

## 已修正的检查失败（原始输出）

### Wire 工具依赖

```text
/home/qlqq/go/pkg/mod/github.com/google/wire@v0.7.0/cmd/wire/main.go:34:2: missing go.sum entry for module providing package github.com/google/subcommands (imported by github.com/google/wire/cmd/wire); to add:
    go get github.com/google/wire/cmd/wire@v0.7.0
cmd/server/main.go:3: running "go": exit status 1
```

改用复制到 /tmp 的 go.mod/go.sum，通过 `go run -mod=mod -modfile=<临时 go.mod> github.com/google/wire/cmd/wire` 生成成功。仓库 go.mod/go.sum 未变。

### 未使用 import

```text
ok      github.com/Wei-Shaw/sub2api/internal/service    0.089s
# github.com/Wei-Shaw/sub2api/internal/handler [github.com/Wei-Shaw/sub2api/internal/handler.test]
internal/handler/client_release_handler_test.go:8:2: "net/http" imported and not used
ok      github.com/Wei-Shaw/sub2api/internal/repository 0.036s
FAIL    github.com/Wei-Shaw/sub2api/internal/handler [build failed]
ok      github.com/Wei-Shaw/sub2api/internal/handler/admin  0.043s
ok      github.com/Wei-Shaw/sub2api/internal/server/routes  0.048s
ok      github.com/Wei-Shaw/sub2api/internal/server/middleware  0.027s
FAIL
```

已删除测试中未使用的 net/http import。

### 测试 fixture 排序后的状态修改

```text
ok      github.com/Wei-Shaw/sub2api/internal/service    0.055s
ok      github.com/Wei-Shaw/sub2api/internal/repository 0.035s
--- FAIL: TestClientReleasePublicHandlers (0.00s)
    client_release_handler_test.go:125:
            Error Trace:    /home/qlqq/workspace/sub2api/backend/internal/handler/client_release_handler_test.go:125
            Error:          Not equal:
                            expected: 404
                            actual  : 200
            Test:           TestClientReleasePublicHandlers
FAIL
FAIL    github.com/Wei-Shaw/sub2api/internal/handler    0.040s
ok      github.com/Wei-Shaw/sub2api/internal/handler/admin  0.685s
ok      github.com/Wei-Shaw/sub2api/internal/server/routes  0.044s
ok      github.com/Wei-Shaw/sub2api/internal/server/middleware  0.030s
FAIL
```

公开测试的 stub List 返回了共享切片，service 排序后测试按旧下标撤回了错误版本。stub 改为返回新切片（与 SQL 仓储行为一致），该测试通过。首轮全量测试同样捕获此 fixture 失败；没有修改公开接口的撤回行为。

## 修改文件

- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`
- `backend/internal/handler/admin/client_release_handler.go`
- `backend/internal/handler/admin/client_release_handler_test.go`
- `backend/internal/handler/client_release_handler.go`
- `backend/internal/handler/client_release_handler_test.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/repository/client_release_repo.go`
- `backend/internal/repository/client_release_repo_test.go`
- `backend/internal/repository/wire.go`
- `backend/internal/server/middleware/audit_log.go`
- `backend/internal/server/middleware/client_release_audit_test.go`
- `backend/internal/server/router.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/client_releases.go`
- `backend/internal/server/routes/client_releases_test.go`
- `backend/internal/service/client_release_service.go`
- `backend/internal/service/client_release_service_test.go`
- `backend/internal/service/client_release_version.go`
- `backend/internal/service/client_release_version_test.go`
- `backend/internal/service/wire.go`
- `backend/migrations/263_client_releases.sql`

## 契约与待验收

接口字段、HTTP 状态与 reason 未改变。本轮没有需要修改 design.md 的偏差。

后续按 implement.md 验证：本机管理员登录、新建/check/上传/发布、latest/notes/files（Range/HEAD）、SHA256、篡改/缺文件/未登录/草稿拒绝、六版本清理、force-recreate 后下载。
