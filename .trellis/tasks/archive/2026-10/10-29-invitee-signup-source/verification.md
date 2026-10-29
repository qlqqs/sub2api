# 验证记录（2026-10-29）

## 变更边界与审查

- 配置从 settings → 奖励配置 → 管理 DTO/审计 → 前端表单完整读写；新键缺省 all，旧请求省略不重置，显式非法输入拒绝。
- 注册路径只新增来源条件，仍沿用原 CreditInviteeSignupBonus 事务/唯一约束。绑定快照由 service 事务包装附到返回对象，不改仓储接口、表结构、历史流水。
- 匿名预览只读、只返回 amount，独立限流/no-store；detail 和注册发放复用来源匹配。后台/API 合约测试已补新字段。
- UI 复用 Select；简短中英文文案；未改首充/分润。注册页取消过期请求，邀请码编辑时先隐藏旧额再防抖查询；输入为空沿用原注册归因回退规则。
- 无新依赖、无锁文件变化、无远端操作、无提交/推送；当前分支仍 sub2api-custom。

## 已通过检查

| 检查 | 结果 / 证据 |
|---|---|
| 后端全量 unit（去掉代理及 OPENAI_API_KEY） | 通过，`/tmp/signup-unit-final.log`，service 217.323s |
| 后端 API contract 最后复核 | 通过，`/tmp/signup-contract-final.log` |
| Affiliate 仓储集成 | 通过，`/tmp/signup-integration.log`；包含原赠送并发/保存点/财务口径 |
| `go build ./...` | 通过，`/tmp/signup-go-build.log` |
| 注册页/预览 API 最后定向测试 | 32项通过，`/tmp/signup-front-target-final.log`，包含手动编辑邀请码回归 |
| 本机 `run_all.sh --fast` | 252/252，`/home/qlqq/workspace/sub2api-local/e2e/runs/20261029-054135/report.md` |
| 新来源用例独立复跑 | 31/31，同目录 `sgs-locked-fixtures/report.md` |
| 前端 lint / typecheck | 通过，`/tmp/signup-lint-final.log`、`/tmp/signup-types-final.log` |
| 最终前端全量 | 440文件、3512项通过，`/tmp/signup-front-final.log` |
| 最终本机镜像构建/启动 | 通过，`/tmp/signup-local-build-final.log`，仅更新 sub2api-local |
| 最终浏览器 UI | 11/11，通过保存/刷新、手动改邀请码、来源切换、金额变化与关闭；`/tmp/signup-ui-final.log` |
| diff / 锁文件检查 | `git diff --check` 通过，无 pnpm-lock.yaml / pnpm-workspace.yaml 变化 |

截图与 UI JSON：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261029-054135/signup_source_ui/`。已人工查看后台桌面/窄屏、注册窄屏与邀请页截图，控件/文案无截断，新卡片无防御性说明。UI 完成后已恢复测试前的赠送设置。

## 本机测试资产（仓库外）

- `/home/qlqq/workspace/sub2api-local/e2e/p0_bootstrap.py`：新来源基线 all。
- `e2e/p3_signup_source.py`：三来源矩阵、预览、detail、真实注册及流水、金额更新、历史不变、兼容请求、邀请触发升级。
- `e2e/run_all.sh`：在 p3 后运行新阶段。
- `e2e/ui/signup_source_ui.{sh,cjs}`：真实下拉、保存刷新、注册/邀请页金额与隐藏、手动编辑、窄屏截图。
- `e2e/CASES.md`：用例及运行前提。

## 排查记录

- 首轮全量单测提示后台设置 API 快照缺新字段；已补两处完整响应合约，复跑通过。
- 新增 API 测试的 beforeEach 曾直接返回 mock，Vitest 将其作为 cleanup 调用；改成无返回 block，异常路径断言通过。
- UI 首轮保存被延迟的新手引导遮挡；测试脚本沿用本机约定，等待并 Esc 退出，不改产品。
- 整轮 e2e 后 SGS 普通邀请人被后续自动升级测试影响；为独立 UI 样本锁定身份，另保留未锁定的升级边界样本，重新验证。
- 人工审查发现手动编辑邀请码会让提示仍沿用链接金额；已让预览跟随实际提交候选码并加防抖、取消及回归测试，不改既有邀请归因。

## 未运行/后续

- 本机未安装 golangci-lint，留给 CI。
- 未运行完整模式 p5/p6/p8/p9；本次使用 fast 模式（含 p7、p10）及专项来源/UI 回归。
- 未提交、推送、部署；任务保持活动，待用户批准后再执行对应步骤，不自动归档或触发文档提交。

## 后续发布（2026-10-29）

以上「未提交/未部署」为本机验收时状态。用户随后明确授权；功能提交2ffc49db7已推送，CI40827319767全部通过，已备份并部署一号机sub2api-custom。生产与双域名只读检查通过，设置保持¥0.50/all。详见release-report.md。
