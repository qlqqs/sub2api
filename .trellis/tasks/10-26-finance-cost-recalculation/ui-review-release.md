# 历史重算UI审查发布（2026-10-26）

## 范围与授权
用户承接UI审查结果明确“提交更新部署”。只发布sub2api主应用；不更新Nova/Chat，不执行生产重算，不提交无关 `design-previews/`。

## 变更
- 未确认预览由创建人取消，事务清租约阻止旧worker回写；迁移261扩展状态约束，不修改历史业务数据。
- 中英文短文案、错误映射、去重提示与按状态展示；重新预览保留原范围，统计同步失败只引导重新同步。
- 后端service/handler/真实PG集成与迁移测试，前端组件与i18n回归。

## 已有本机验证
前一会话日志：`/tmp/recalc-fe.log` 全部436文件/3442测试通过；最终小改动 `/tmp/recalc-fe2.log` 19文件/131测试通过；`/tmp/recalc-local-build.log` 最终全栈构建/容器更新成功；`/tmp/recalc-ui/` 浏览器生成预览→取消截图。

本发布会话重跑的检查与正式部署证据将在完成后追加；CI未通过不得部署。

## 发布前复验
- 后端全量 `go test -tags unit ./...`、财务重算/基线PG集成及 `go build ./...` 成功（去除代理与OPENAI_API_KEY）。
- 本机已有临时工具 golangci-lint v2.14.0 全量检查：0 issues。
- pnpm9 lint、typecheck、全部436文件/3442测试通过；未修改锁文件。
- 本机最终镜像浏览器重新走生成预览→取消，显示“已取消”，无pageerror；只读HTTP/静态资源及新取消/重预览控件检查通过。
- 证据：`/tmp/s2a-recalc-release/{backend,frontend,lint,ui}.log`。

## 正式发布结果
- 功能提交：`99ab810ed8b2974b425e072fca5258342b52474a`，已推送 `origin/sub2api-custom`。
- Custom Image CI：`40360471074` 全绿（backend-test含全量unit/integration、backend-lint、frontend-test、image）。
- 镜像：`ghcr.io/qlqqs/sub2api:custom-99ab810`，digest `sha256:2f0d808c0783a06535fb17f55c7d4d0b7a43d803edde24402f26ee44ede2c234`；拉取和运行时均核对完整OCI revision。
- 备份：一号机 `/root/sub2api-custom/backups/pre-recalc-ui-20261026T164930Z`，权限受限；含Postgres custom dump、应用data、compose/env及旧镜像ID。pg_restore目录读取、tar列表、SHA256检查通过。
- 旧镜像：`sha256:684428dbefc74d938c3bb7dc7a8db064a3c62a76a11237c24946a2f03b55838a`。
- 2026-10-26 16:49:30 UTC停主应用，16:49:38 UTC部署健康检查成功；compose只对sub2api执行 `--no-deps --pull never`，未重建Nova/Chat/PG/Redis或其他容器，前后其他运行容器ID完全一致。
- 迁移261存在，状态约束包含cancelled；数据库checksum与runner的TrimSpace后文件SHA256一致（`dd3c46f1bcca115834b3a14758fc095cb2d75d075a7f8d9beaf30ad65a1cc38e`）。
- 线上只读检查：health/首页/Nova入口200；首页6个与Nova11个静态资源200；重算列表、Chat入口/元数据与工作台用户API未认证401；财务JS包含cancel-preview和preview-again控件。主应用healthy、revision准确，无匹配到panic/FATAL/迁移失败日志。
- 未创建线上测试用户、未修改成本配置或执行历史重算；生产验证为健康/资源/鉴权/迁移检查，不宣称生产业务全链路操作通过。
- 本机证据：`/tmp/s2a-recalc-release/{ci,deploy,production-smoke}.log`；远端发布脚本及日志：`/root/sub2api-custom/backups/recalc-ui-release-logs/`。未提交任何凭据、备份或无关design-previews。
