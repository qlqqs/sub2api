# 实施：qxapi-client 更新器接入

先决条件：`10-30-client-releases` 已在 `sub2api-local` 可用（端到端验证需要；单测不需要）。已按用户授权独立合入 `qxapi-doh`（`f414c255`），复用三域名 Failover 与限定范围 DoH；更新器测试需覆盖 ai 和 www 两级回退及成功 origin 的相对 URL 解析。

## 步骤

1. 更新源与相对 URL：改 `Feed`、`NotesFeed`、`LatestIn`、`askNotes`、`ReleasePage`。反转 `internal/update/qxapi_test.go`，改为用 httptest 返回约定 JSON，断言请求路径、资产 URL 的解析结果和 notes 的解析结果。
2. 资产名和 `.app` 名：`AppAsset`、`BinaryAsset`、`Stage`，加单测，覆盖 darwin、linux、windows。
3. 默认开启：改 `settings.go` 和 `settings/qxapi_test.go`。
4. 文案、帮助和镜像：逐处修改，新增无 magpie 字样的扫描测试。GUI 文案同步 en、zh 及其他语言的 i18n 键。
5. 打包流程：版本号、macOS zip、Linux job、release.json 生成脚本和测试。远端 workflow、tag 和正式产物核对需另获授权；本轮仅本地检查，不触发远端。
6. 端到端（原 28080 环境不动；测试 fixture/nogui 不能代替原生桌面验收）：
   - 本机 Linux：构建 0.1.0 和 0.1.1，在独立 `qxapi-updater-test` 环境上发布测试版 0.1.1，用 `QXAPI_UPDATE_FEED=http://127.0.0.1:28081/api/v1/client-releases/qxapi/latest` 运行 0.1.0，观察自动升级。
   - 把一个文件改坏后再测一次，确认拒绝安装。
   - Windows、macOS：在测试机上用 `ssh -L 28081:127.0.0.1:28081` 把 sub2api-local 转发到本机回环地址，重复上面的测试。没有测试机就写明未测。

## 验证命令

```bash
go build ./... && gofmt -l internal
GOOS=linux go vet ./... && GOOS=windows go vet ./...
go test -timeout 45m ./...        # 一台机器同时只跑一套完整测试，日志 tee 到自己的文件
# GUI：与 update、whatsnew、settings 版本行相关的文件，以及 i18n-keys，Chromium 和 WebKit 都跑，中英文都跑
python3 -B tools/qxapi-release-manifest_test.py
```

## 审核点

- 提交和推送另获同意。打 `v0.1.0` tag、把正式包上传到生产 sub2api 也各自另获同意。
- 改完后更新 `docs/subsystems/qxapi-account.md` 中关于"没有更新源"的段落，以及 `docs/windows-signing.md` 中涉及发布方式的内容。


## 2026-10-31 发布交接

用户现已授权 sub2api 提交/推送和七号机主服务部署；本次仅提交任务文档，既有后端实现随分支推送。客户端正式打包及 Windows/macOS 真机验证由用户另行执行，不在本次服务器部署中补测或伪记通过。QxAPI 禁用功能的测试失败按用户确认保留记录但不阻塞，范围及 E2E 边界见 `verification.md` 文末。禁止在生产发布 28081 的测试资产。
