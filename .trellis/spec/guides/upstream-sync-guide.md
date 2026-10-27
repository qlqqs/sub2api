# 上游同步与选择性合并工作流

## 1. 范围与入口

适用于把 Wei-Shaw/sub2api 已合并 main 同步为本地纯上游 main，再按需将通用维护修复带入 sub2api-custom。模型上新/能力/价格族适配由独立任务负责，本流程不自动实施。首次只读准备后按 Trellis 获取建任务和最终计划批准。

每次先读：
1. 本机 `/home/qlqq/workspace/AGENTS.md` 和仓库 AGENTS.md。
2. `.trellis/upstream-sync/state.json`、`decisions.jsonl`（长期事实）。
3. 上一批任务验证摘要和当前开发 journal/index（证据及待办）。

main 不放定制代码、台账、任务文档；这些只存 custom。任务归档不得移走长期账本。

## 2. 固定输入并同步 main

实施前记录 `git status --porcelain -uall`、HEAD/main/origin/main/upstream/main、worktree、索引、进行中的 Git 操作及并行任务。不能自动 stash、reset、clean 或替别人提交；用户没有要求时不 prune 失效 worktree。

获批后：

```bash
git fetch --no-tags upstream main
TARGET=$(git rev-parse upstream/main)
git merge-base --is-ancestor main "$TARGET"
# 确认 main 未被占用且路径不存在，再建立独立 worktree：
git worktree add <mirror-worktree> main
git -C <mirror-worktree> merge --ff-only "$TARGET"
```

任一步出现分叉、缺失历史或其他工作区占用，停止并报告，不 force/reset。固定 TARGET，过程中远端再更新也不移动本轮输入。记录 fetch 与 main 快进分别成功与否；origin/main 推送另行授权。

## 3. 增量审查与台账约定

账本是单写入者追加 JSONL，state 是可核对的摘要；主代理统一写，子代理交付证据。每个事件有 schema_version、递增 seq、唯一 event_id、run_id、时间、actor、event_type；同一决定只记录一次，更正/reopen/revert 用新事件指向原事件，不覆盖历史。

读取时按 seq 顺序聚合：review_decision 建立单元范围决定，integration_progress 只更新明确提供的进度字段，保留原决定的 source/paths/review_depth/reopen_conditions；validation_result 是批次证据，不作为新的审查单元。不能只取同 unit_sha 的最后一行而丢失原始审查信息。

审查单元是上游 main first-parent 节点，merge 看相对第一父提交的完整差异并记录 covered_commits，包含合并解决产生的变化。普通提交按父差异。不要同时把 merge 与子提交重复计数；patch-id 和 -x 只作为来源/候选等价线索。

```bash
# 用 state 中固定的 baseline/游标和 TARGET，不凭“最新”记忆：
git log --first-parent --reverse --format='%H %s' <review-cursor>..<target-sha>
git diff --stat <unit-sha>^1 <unit-sha>
git diff <unit-sha>^1 <unit-sha>
```

### 必须区分

| 字段/事实 | 含义 |
|---|---|
| last_fetched | 本轮 fetch 观察到的 SHA，不等于 main 已快进 |
| local_main_sha | 本地纯上游镜像位置，不等于 origin/main 已推送 |
| last_contiguous_reviewed | 所有前序单元都有范围处置，不意味着全部技术审查/测试完成 |
| decision | selected / selected_partial / deferred / skipped / delegated_out_of_scope |
| review_depth | scope_triage_only / selected_patch_and_dependencies_reviewed / technical_review_deferred 等；必须诚实填写 |
| integration_status | not_applied / applied_in_candidate / verified_candidate / applied_in_custom_worktree / integrated / reverted |
| target_commits | 真正包含改动的 custom 提交；未提交时为空，不能填基线提交冒充 |
| validation | 验证对象 SHA+未提交文件指纹、命令、实际退出码、证据 |
| shipment | 推送、镜像、部署、真机验收分别记；任一发生不推断其余发生 |

selected_partial 必须列选入源 SHA 和未采用部分的决定。历史导入基线只记录已有证据，不伪造基线前逐项审查。模型项只记录移交及原因，不假称能力适配已审。

新审查队列 = 新增尚无有效决定的单元 + 触发重开的旧单元。若前面缺决定，连续游标停止于缺口之前，但之后已做的决定保留，不重审。延期项持续列于待办，不能随游标推进消失。延期的 scope_triage_only 项若决定实施，须补技术审查，而不是重新重复范围讨论。

重开条件：用户选择、上游纠正/回退、依赖满足、custom 相关行为变化、风险策略变更或期限到达。路径交集只作提示，人工检查跨层依赖。未触发条件且源 SHA/审查依据未变，不重复翻旧提交。

PR 用 repo+number+head_sha+base_sha 记录版本，未合并 PR 不推进 main 游标；head/base 改变复核增量，正式合并后对照最终 merge diff，不能只用同一 PR 编号继承通过状态。

## 4. 选择性集成与分工

从冻结 custom HEAD 建立独立候选 worktree，而不是从 main 开始；不整体 merge main。小范围 `git show --format= --binary <source> | git apply --check` 后再应用；文本适配通过不等于安全。保留 custom 行为，必要适配记录理由和 diff。

新提交需本机验证后用户同意，不用 cherry-pick 默认提交绕过授权。候选工作区的未提交改动必须有 patch/file 指纹。多人执行时分配不重叠写集，Git index/集成/台账归主代理；停代理后确认其测试进程状态并接手，不重复启动。

主工作区清洁且 HEAD 未变时，经确认只把本批批准文件移入；其他任务改变 HEAD 或同文件时停止覆盖，重新整合和复验。`applied_in_custom_worktree` 仍不是 integrated；只有真实目标提交及证据才升级状态。候选通过不能证明与另一批未提交变更兼容。

## 5. 验证与错误处理

- 按源补丁依赖顺序，结合定制响应模型名、引用、图片流、财务和分润审查。
- 后端测试移除 http_proxy/https_proxy/HTTP_PROXY/HTTPS_PROXY/ALL_PROXY/all_proxy 与 OPENAI_API_KEY；实际工具链以模块内 `go version` 为准。前端固定 `npx -y pnpm@9`，不调用全局 pnpm。
- 同步安全版本时检查 go.mod、实际镜像、CI 断言、锁文件；不夹带未批准构建入口升级。不延长安全例外掩盖告警，区分原始审计失败与例外策略通过。
- 本机 `SUB2API_REPO=<candidate> /home/qlqq/workspace/sub2api-local/build.sh` 可指定候选，但镜像、构建目录、compose/DB/e2e 数据共享，必须协调串行运行。不得使用旧基线前端产物冒充新构建，不全局清理 Docker。
- 完整测试若跳过/失败，记录实际范围、原因、是否基线复现，不把跳过计为通过。
- 数据库迁移按完整文件名+TrimSpace SQL SHA256 管理；相同数字前缀不一定冲突，也不保证执行顺序/对象安全。新批次有迁移时验证空库和已有 custom 库，不修改/重命名已应用迁移。
- 账本损坏、事件序号重复、游标越过缺口、源历史分叉或目标证据不存在：停止推进，追加明确修正，不静默跳过。

正例：候选测试通过 → 记录 verified_candidate；移入当前工作区未提交 → applied_in_custom_worktree；获准提交后 → integrated。反例：main 更新或 apply-check 成功后直接标全量已合入/已部署。

## 5A. 获批推送与部署时的检查

- 先核对远端与本地祖先关系及本轮实际发布范围（包括分支中早已存在、尚未发布的其他任务提交），不强推。main/custom一起发布时可原子推送，但不把main同步当作custom全量合并。
- 等固定head的Custom Image全部必要任务成功后部署；确认该流程是否覆盖独立安全审计，不把CI成功当作原始审计零告警。
- 核对目标部署目录、旧运行revision及是否有进行中重算；按固定custom短SHA镜像拉取并比对完整OCI revision，留存旧镜像ID及配置/数据库/data备份并验证可读性和校验和。
- 只更新获准主应用；不顺带重建PG/Redis/Nova/Chat。启动失败回到旧应用镜像，不自动恢复数据库覆盖新业务数据。核对前后其他容器ID，不仅依赖命令参数推断隔离。
- 上线验证明确区分只读健康/静态资源/未认证鉴权与真实业务全链路；不擅自做付费推理或财务操作。
- 最后同步下面第6节记录并提交推送纯文档。Custom Image忽略 `.trellis/**` 等文档路径时，文档HEAD可晚于实际运行revision：分别记录，不能把文档提交SHA冒充部署镜像SHA。

## 6. 最后一步：更新现有记录文档（每轮必做）

1. 追加已有 decisions，并更新 state；记录同步前后 SHA、每项决定、依赖/适配、测试证据、未完成项及下一轮起点。
2. 更新本轮 implement、验证摘要、task.json；待提交/待授权事项不写成已完成，不为了归档提前完成。
3. 读取当前 active journal，通过现有 add_session 流程追加记录并更新 index。尚未获准提交时必须使用 `--no-commit`，无新提交时 `--commit -`；不把基线或他人提交列成本任务成果。
4. 更新确有变化的现有 guide/spec/index/运维当前说明；归档历史报告保留原事实。未部署则不更新生产版本；无需改动的相关文档标明原因。
5. 交叉核对记录，最终列出更新文件、未完成事项、审查与集成队列、未推送/未部署状态。记录未同步完，不算本轮收尾完成。

下一次默认先执行第 1–3 节读取/增量流程，不重新审查未变化且已有有效决定的旧提交。
