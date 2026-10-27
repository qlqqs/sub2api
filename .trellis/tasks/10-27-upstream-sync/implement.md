# 实施与验证进度（已提交、推送并部署，保留质量跟进）

## 当前状态

- 已获：创建任务、子代理规划研究、2026-10-27 用户“开始合并”的实施批准。
- 后续已获：“提交”及“推送更新部署”；实际交付见 verification/release-report.md。
- 当前 task.status=in_progress；首批四组修复已移入 custom 工作区，验证有已记录的失败/限制。本地提交、推送和部署均已获批并执行。

## A. 规划与审查证据

- [x] 创建任务，排除模型更新。
- [x] 明确原工作区有并行未提交改动，不做切换/暂存/覆盖。
- [x] 两个子代理将既有审查结果写入任务 research，主代理核对。
- [x] PRD 归并检查、上下文清单校验（implement 4 条、check 3 条）。
- [x] 向用户展示最终计划并取得新回复批准。

## B. 获批后的上游镜像同步

- [x] 重新读取 AGENTS、任务、相关 specs；记录 HEAD/工作区/索引/worktree 和 refs。
- [x] fetch 上游，冻结完整目标 SHA；若较研究快照新增提交，只增加审查单元，不自动扩展首批实施范围。
- [x] 验证 main 为上游目标祖先；在独立干净工作区只做 fast-forward，同步本地 main，不 push。
- [x] 保存同步前后 SHA/时间，不将 fetch/main 更新标作已审查或已集成。

## C. 台账与规范查询步骤

- [x] 创建长期 state/decisions，导入历史基线与既有研究结果；main 范围所有单元有明确决定，缺证据者保持待审。
- [x] 在规范中固定 first-parent 增量查询、决定对照、暂缓项检查和证据校验步骤；首版不新增自动化脚本。
- [ ] 覆盖：重复运行不重审、游标缺口、merge resolution、已选择待集成、暂缓重开、out-of-scope 移交、PR head/base 变化、等价/部分移植、回退、上游历史分叉、非法账本。
- [x] 写规范并加索引；分开列审查队列和待实施/暂缓队列。

## D. 独立候选中的首批修复

- [x] 从固定 custom HEAD 准备候选工作区；对子代理分配不重叠写集，禁止他们操作原工作区或共享账本。
- [x] 安全链：cacc80b → 4c940c3/59100d3 → 8286268；保留定制构建入口与 Node 版本。
- [x] COW：38b72d7；CF：e0fa4b9；overload queued/metadata：e068c3d。
- [x] 每项记录实际源 SHA、依赖、适配差异、测试，暂记 applied_in_candidate；不自动创建代码提交。
- [x] 其余项按计划暂缓；模型项移交，不实施。

## E. 验证（已执行；实际结果见 verification/final-report.md）

在候选工作区：

```bash
cd backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -race -tags unit ./internal/service -run 'TestSetAccountModelRateLimitSnapshot'
go build ./...
cd ../frontend
CI=true npx -y pnpm@9 install --frozen-lockfile
npx -y pnpm@9 run lint:check
npx -y pnpm@9 run typecheck
npx -y pnpm@9 run test:run
npx -y pnpm@9 audit --prod --audit-level=high
```

- [x] 补跑选中补丁的 403、capacity、WS bridge 和定制响应/citations/图片流回填测试；具体测试名由源补丁和现有测试确定，不猜测已执行。
- [x] 检查冻结安装没有意外改锁；记录环境/依赖下载限制及原有审计问题。golangci-lint 本机缺失时明确 CI 待验证，不假报通过。
- [x] 串行使用本机环境，候选生产构建通过；完整 e2e 已执行，395 项中 394 PASS/1 FAIL（FIN-24），不得记全绿。
- [x] 保存精确候选/当前工作区文件指纹；台账标 applied_in_custom_worktree 并记录 e2e/audit 限制，未标 integrated，不用基线 SHA 冒充补丁树。

## F. 当前分支集成与交付门禁

- [x] 核对原 custom 状态与14文件指纹；仅移入本轮批准路径，无 stash/reset/暂存，未标任务完成。
- [x] 并行成果先提交为 a18a7a3a4，候选从该基线验证后移入；最终原/候选 HEAD 与产品指纹一致，保护他人成果。
- [x] 用户在收到测试限制后批准本地提交：`b95be64f6d06a9c617b4690ac1f0d50c78366d4d`；记录真实 integrated 映射及提交指纹，不将提交视为质量豁免。
- [x] 用户批准后已推送origin/main及custom；CI全绿、备份后部署一号机主应用，未更新其他项目。
- [x] 固化规范和长期台账，记录本批结果、暂缓原因、下一次起点；无提交/推送/部署。

## G. 最后更新现有记录文档（每轮必做）

- [x] 更新已有 decisions 追加日志与 state 快照；核对源/目标 SHA、审查游标、集成状态和暂缓重开条件。
- [x] 更新本轮 implement、验证报告、task.json；保留待授权/待整合状态，不提前标记完成。
- [x] 通过现有会话记录流程追加当前开发 journal 并同步 workspace index；不覆盖其他并行任务记录。
- [x] 更新受影响的现有 guide、spec/index 和运维当前状态文档；无变化的相关文档注明检查结果及无需修改原因。
- [x] 未部署不修改生产版本事实；推送/部署只有实际发生并取得证据后才更新。
- [x] 对照现有文档做版本/状态一致性核验；最终交付列出已更新文档、未完成项、下一轮增量起点。记录同步未完成不得宣称本轮收尾完成。

## 风险检查

- 必须重新验证当前目录状态，不能拿历史“69 项”当作锁。
- `git apply --check` 仅说明文本适配；无冲突不等于测试通过。
- 安全补丁不扩大到未批准的其他 Docker 构建入口，不自动删除安全审计例外。
- c591412 身份与延迟策略改变、Anthropic 跨 provider 入账边界均不夹带到首批。
- main 的上游镜像推进不携带本项目台账；所有台账/规范只在 custom 侧持久化。

## 实际执行补充（2026-10-27）

- 同步过程中并行任务提交了 a18a7a3a4，候选据此固定基线；不是本任务的提交。
- 已停止两个实现子代理并由主代理接手，全部测试进程已结束后才接管；后端定向/race/unit/build与前端检查结果见 verification。
- 14 个批准产品文件已按 exact-byte 对照无暂存移入 custom 工作区（custom-worktree-transfer.json）；新提交尚无，不能标 integrated。
- 主代理生产构建和当前工作区 go vet 通过；本机全量 e2e 完成：394/395，FIN-24 原始失败保留，原因见 fin24-analysis.md，不宣称全量通过。

## 最终状态说明

见 verification/final-report.md。C 节场景清单保留未全勾选：35单元覆盖、merge差异指纹、重复查询、缺口停止已实际验证；PR变化、上游改写、回退及人工重开以规范约定为主，未宣称全部故障演练。F节本地提交/推送/部署已完成，整体质量跟进仍未完成。

## 本地提交更新

用户后续“提交”批准仅本地提交。产品提交 `b95be64f6d06a9c617b4690ac1f0d50c78366d4d`，证据见 verification/commit-report.md；旧执行补充保留提交前事实。本轮未推送、未部署，任务保持in_progress。

## 正式发布更新

CI 40546398450全成功；一号机于2026-10-27 17:20:07 UTC健康运行1fa919f68。备份、只读检查及其他容器不变有证据，详见verification/release-report.md。以上执行快照中的“未推送/部署”“CI待验证”仅代表当时状态，当前以本节为准。FIN-24与原始audit仍未修复，不归档任务。
