# 实施：qxapi-client 更新器接入

先决条件：`10-30-client-releases` 已在 `sub2api-local` 可用（端到端验证需要；单测不需要）。当前分支已有基础 Failover，直接复用；不依赖或合并 `qxapi-doh`。

## 步骤

1. 更新源与相对 URL：改 `Feed`、`NotesFeed`、`LatestIn`、`askNotes`、`ReleasePage`。反转 `internal/update/qxapi_test.go`，改为用 httptest 返回约定 JSON，断言请求路径、资产 URL 的解析结果和 notes 的解析结果。
2. 资产名和 `.app` 名：`AppAsset`、`BinaryAsset`、`Stage`，加单测，覆盖 darwin、linux、windows。
3. 默认开启：改 `settings.go` 和 `settings/qxapi_test.go`。
4. 文案、帮助和镜像：逐处修改，新增无 magpie 字样的扫描测试。GUI 文案同步 en、zh 及其他语言的 i18n 键。
5. 打包流程：版本号、macOS zip、Linux job、release.json 生成脚本和测试。在测试 tag 上手动触发一次（例如 `v0.0.1-rc.1`），下载产物核对。
6. 端到端：
   - 本机 Linux：构建 0.1.0 和 0.1.1，在 `sub2api-local` 上发布 0.1.1，用 `QXAPI_UPDATE_FEED=http://127.0.0.1:28080/api/v1/client-releases/qxapi/latest` 运行 0.1.0，观察自动升级。
   - 把一个文件改坏后再测一次，确认拒绝安装。
   - Windows、macOS：在测试机上用 `ssh -L 28080:127.0.0.1:28080` 把 sub2api-local 转发到本机回环地址，重复上面的测试。没有测试机就写明未测。

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
