# 本机验证报告（2026-10-26）

## 范围与授权
以下验证记录来自localhost隔离环境，验证时无提交、推送或部署。2026-10-26 后续用户已明确授权发布，发布结果另记。本轮财务提交9d87d64cd为另一任务，shared Wire工作区保留combined版本，不再生成。迁移259与260均已本机应用，没有修改已应用SQL。

## 自动检查
| 项目 | 结果 | 日志 |
|---|---|---|
| sub2api 后端全量unit | exit0，service 214.312s | /tmp/s2a-workbench-final-unit.log |
| sub2api go build ./... | exit0 | /tmp/s2a-workbench-final-build.log |
| Workbench真实PG integration | 通过5.437s | /tmp/s2a-workbench-final-integration.log |
| 收尾新增注册hooks/错误分类unit | 通过service0.388s；handler无匹配测试，不计为覆盖 | /tmp/s2a-workbench-wrap-unit.log |
| 收尾go build ./... | exit0 | /tmp/s2a-workbench-wrap-build.log |
| Nova前端全量Vitest | 81文件/758通过 | /tmp/s2a-nova-final-unit.log |
| Nova后端node --test | 133通过 | /tmp/s2a-nova-backend-unit.log |
| OWUI后端embed pytest | 440通过/2 warnings | /tmp/s2a-chat-final-python.log |
| OWUI前端embed Vitest（含收尾i18n） | 14文件/89通过 | /tmp/s2a-chat-wrap-unit.log |
| OWUI embed codemod幂等 | 通过 | /tmp/s2a-chat-final-codemod.log |
| Nova隔离production build | Next编译及TypeScript成功，镜像启动 | /tmp/s2a-nova-build.log |
| sub2api/Chat隔离build | 功能版本已成功；收尾版本另见下节 | /tmp/s2a-workbench-local-build.log、/tmp/s2a-chat-build.log |

## 浏览器与接口证据
安全结果已复制到reports/{ui,flow,persistence}.json，不含凭据。
- 首次界面6项通过，pageerror为空：正常、零余额、删除默认Key三类用户的Nova/Chat。
- 真实流程9项通过，pageerror为空：正常Nova生图完成；零余额异步任务展示「账户余额不足，请充值后重试」和「去充值」，无可见403；真实Chat请求及mock上游Responses均为gpt-6.1-sol/medium；生图任务模型gpt-image-2.5-sunburst。
- 跨新浏览器登录持久化4项通过：Nova人工Key不覆盖、Nova{}清空不覆盖、Chat改high后UI仍高且服务端保存、Chat显式null Key不覆盖。
- local_setup.py已验证默认Key唯一/归属/组名/ID、元数据/medium、零余额模型发现200而付费生图403 INSUFFICIENT_BALANCE、初始化失败仍注册成功、存量管理员不具资格。
- 截图在/tmp/s2a-workbench-e2e/{ui,flow,persistence}/；已检查WB-chat.png、WZERO-image-result.png等。state.json只在mode700临时目录，不纳入任务。
- setup临时目标设置已finally恢复；后续普通本机注册不能假定继续使用fixture组。测试只涉及本任务账号，未清理或写财务fixture。

## 限制与未执行项
- OWUI全量 `npm run check` **未通过**：5695 errors/189 warnings/313 files，日志/tmp/s2a-chat-final-types.log；广泛上游类型债务，新增workbench/reasoning相关错误已修正且定向grep无诊断，但没有独立HEAD全量基线比较，不能宣称所有报错已证明与本任务无关。
- Nova独立 `tsc --noEmit` **未通过**：既有test fixture类型诊断，日志/tmp/s2a-nova-types.log；生产Next构建内的TypeScript通过，不能等同全仓库test代码类型通过。
- sub2api前端本任务无修改；财务协作方报告436文件/3438测试通过及lint/typecheck通过，本任务未重复执行。
- golangci-lint本机未安装。没有运行全量repository integration、p0–p11或外部真实供应商请求；只跑任务相关真实PG集成和隔离mock链路。
- 缺失默认模型/档位无fallback由单元测试覆盖，没有新增专门的缺模型浏览器fixture。邮箱/OAuth注册hooks已补unit；真实第三方登录未跑外部身份提供商。
- 未回放用户生产失败请求，不把本机结构化错误路径的修复称为生产唯一根因已证实。

## 收尾版本
- 收尾仅错误分类、测试、中英文提示及文档，无Wire签名/迁移修改。
- sub2api `--no-web`重建及Chat独立重建日志：/tmp/s2a-workbench-wrap-local-build.log、/tmp/s2a-chat-wrap-build.log。两者均exit0，主服务和Chat容器healthy，Nova保持运行。收尾版Playwright重新抽查6项通过、pageerror为空（/tmp/s2a-workbench-wrap-ui.log）；三个仓库git diff --check通过，index仍为空，锁文件/依赖清单未变更。

## 授权发布后的补充
用户授权后，三个仓库已提交推送、CI通过并更新一号机配套部署；后端全量golangci-lint也已通过。迁移259/260、目标group ID 2、镜像revision及只读健康检查均确认。详见release.md；原全仓库类型检查等限制继续保留。
