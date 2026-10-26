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
