# 实施与验收记录

## 授权门槛
- [x] 用户批准规划及实现，本任务 in_progress。
- [x] 读取本机说明、三个仓库约定及相关 Trellis specs；保留其他任务改动。
- [x] 2026-10-26 用户明确授权：提交、推送剩余工作台改动，连同财务一起更新一号机配套部署。

## 实施
- [x] 只读核实当前部署目标组，隔离本机 fixture，不混用真实用户凭据。
- [x] 迁移259，持久一次性claim、按ID绑定与同名Key、注册fail-open；Wire已生成。
- [x] 当前用户元数据与偏好API，严格字段白名单、初始化冲突保护、显式清空。
- [x] Nova默认模型与medium请求、持久偏好、结构化余额错误与充值操作。
- [x] OWUI自己的条件初始化、用户档位持久化、精确生图模型及无fallback。
- [x] 只读模型发现余额豁免，付费请求与Key权限检查不变。
- [x] 单元/数据库集成/三项目构建和本机Playwright验收；详见validation.md。
- [x] 收尾补注册入口fail-open回归、错误分类测试和OWUI中英文提示。
- [x] 更新实际设计与后端workbench code-spec。

## 测试方法
- 后端测试移除 http_proxy/https_proxy/HTTP_PROXY/HTTPS_PROXY/OPENAI_API_KEY；`go test -tags unit ./...`、`go build ./...`、`go test -tags integration ./internal/repository -run Workbench -count=1`。
- Nova前端 `npx vitest run`，后端 `node --test`；隔离build.sh包含Next TypeScript校验。
- OWUI `npx vitest run src/lib/embed`、`npm run embed:check`、隔离venv pytest embed；独立build.sh。
- 脚本 scripts/local_setup.py、local_ui.cjs、local_flow.cjs、local_persistence.cjs 仅localhost；Playwright加载本机依赖及字体。
- 凭据/会话在 `/tmp/s2a-workbench-e2e/state.json`，不写入任务、报告、git；安全结果另存reports/。

## 收尾限制
- sub2api前端没有本任务代码改动；财务报告其全量检查通过，不能冒称本任务重新执行。
- Nova独立tsc及OWUI全量svelte-check存在上游/既有测试类型问题，未全部通过，报告明确保留这一限制。
- 全局golangci-lint本机未安装。未运行完整所有repo integration或全阶段p0–p11，不把定向通过称为全覆盖。
- 获授权后按三个仓库分别提交；CI成功、线上备份后部署，不顺带执行财务历史重算。
