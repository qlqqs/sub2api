# 技术设计：上游镜像、选择性集成与增量台账

> 用户已批准实施及交付，主应用已发布；当前事实见 verification/release-report.md。旧验证证据保留原结论；模型更新完全排除。

## 1. 最小行为差距及职责

当前只有批次 journal，无法自动区分“同步到了哪里”“看过哪些变化”“哪些实际落地”。引入单一事实台账和标准化只读查询步骤，不做自动合并机器人。main 保持纯上游；custom 持有台账与规范。

本轮新增/修改：
- `.trellis/upstream-sync/state.json`：schema version、源/目标仓库标识、历史导入基线、last_fetched、last_contiguous_reviewed、事件序号。
- `.trellis/upstream-sync/decisions.jsonl`：主代理单写入的追加事件；每条含审查单元/源 SHA、决定、理由、依赖、重开条件、证据、集成状态及目标 SHA。
- `.trellis/spec/guides/upstream-sync-guide.md` 与 guides index：固定流程、授权、隔离、迁移与验证规则。
- 本任务内研究文档与验证记录：批次证据，不替代长期账本。

不使用默认被忽略的 docs/*、根 scripts 或 .trellis/scripts 保存工作流。首版采用规范中的 Git 只读查询与结构化台账，不新增自动化脚本；是否制作工具后续另议。不修改 Trellis 托管脚本或 bundled skills。

## 2. main 镜像与候选隔离

获批实施后先读取工作树、引用和 worktree 状态并记录基线。fetch 只更新上游 remote tracking，不自动合并。将最新远端观察冻结成本轮 target SHA，检查 local main 为其祖先；独立干净 worktree 中执行 fast-forward-only。遇分叉停止，不 reset/force，不自动清理 /tmp/s2a-mid。origin/main 的发布是独立授权动作。

维护补丁候选工作区从固定 custom HEAD 创建，不从 main 创建。子代理按不重叠写集工作；共享 Git index、台账与最终整合由主代理负责。首批安全链、COW、CF 可分写集，gateway 重试与后续流式计费避免并行改同一文件。

首次应用采用不创建提交的方式，只修改隔离候选工作区。新 Git 提交要在本机验证后取得同意。候选分支的存在不表示补丁已进入 custom。

## 3. 审查单元与队列

- main first-parent 上的每个新节点为审查单元，merge 节点查看相对第一父提交的完整差异并列明所含子提交，不能忽略 merge resolution。
- 源提交快照固定为 full SHA；根据祖先关系识别已整体导入的基线。历史 8f2cbb53 仅标 imported_baseline，不虚构逐提交审查。
- 审查决定：selected、deferred、skipped、delegated_out_of_scope；模型类使用 delegated_out_of_scope 并明确没有做功能专项审查。
- 集成状态：not_applied、applied_in_candidate、verified_candidate、applied_in_custom_worktree、integrated、reverted；另记是否推送/部署。部分摘取在原单元保留未采用内容及原因。
- 队列 = 游标之后尚无明确决定的单元 + 人工确认达到重开条件的旧单元。已经决定的后续节点不会因前面存在缺口而重复出现；游标只推进到连续覆盖处。
- deferred 不阻塞审查游标，但持续列于暂缓摘要；selected 未集成持续列于实施摘要；审查队列与实施队列分离。
- reopen 条件包括上游更正/回退、依赖满足、custom 相关实现变化、策略变化、到期或用户要求。标准查询只检查可证明的条件，主代理另行确认需要人工判断的条件，不宣称能自动证明语义不变。
- PR 以 repo/number/head_sha/base_sha 标识版本。未合并 PR 不进入 main 游标；最终合并增量重新对照之前证据。
- 检查引用祖先关系、未知 schema、损坏/重复事件、游标缺口、目标 SHA 缺失、未验证却标 integrated 等不一致；发生不一致停止推进并报告，不自动修复或跳过。

## 4. 首批补丁与范围

依赖链/详细文件证据见 research/maintenance-patch-assessment.md。

1. 安全链：cacc80b → 4c940c3、59100d3 → 8286268。只改必要 Go/前端依赖与 custom 实际构建链；保留 Node 24.22.1 与自定义 CI。
2. COW：38b72d7；不引入调度缓存常驻优化。
3. Cloudflare 1010：e0fa4b9；真实权限 403 行为保持。
4. Overload metadata：e068c3d；仅 queued/metadata 安全重试，不连带出站身份和 Retry-After 策略变化。

94c7d6b + 4355236 成组记录为后续独立批次；c591412 的拆分、图片计费、管理员受限 Key、备份恢复变更均暂缓。模型类全部移交其他任务。

## 5. 验证与业务保护

- 定向验证后端并发、403 分类、首输出边界和前端依赖；保留真实文本、工具、图片 partial、citations 输出后的不可重放边界。
- 后端测试移除代理环境及 OPENAI_API_KEY；前端固定 pnpm 9。工具链按 go.mod 确认，不假设宿主默认 Go 已满足版本。
- 独立安全审计可能有原有过期例外；如失败如实记录，不增加例外来消除告警。
- `SUB2API_REPO` 可指向候选 worktree 编译本机环境，但构建输出、镜像、compose、e2e 数据共享，需与其他任务串行协调；不做 reset 或破坏性清理。
- 原工作区不干净时不直接回灌覆盖；待并行任务完成并获准后，在新的 custom HEAD 上重新整合、运行最终回归。
- 迁移按完整文件名与 TrimSpace 内容 SHA256 记录。同编号不同名不等于对象/顺序安全；本批原则上无新增产品迁移。未来批次同时测试既有库升级与空库全量迁移，不改写已执行 SQL。

## 6. 回退及完成定义

未提交候选可留作审查现场，不触碰原工作区；不在脏工作区自动 reset。已合入后若需回退，生成可审查的逆向变更并记录源/目标和原因，提交与部署重新授权。只有变更确实进入 custom 的目标提交且证据匹配目标树，才标 integrated；推送和部署另记。用户只批准规划时，所有实施清单保持未完成。

## 7. 每轮最后一步：更新现有记录（必做）

由主代理统一完成文档同步，子代理只提供证据，不并行覆盖共享记录：

1. 更新已有 `.trellis/upstream-sync/decisions.jsonl`（追加事件）和由其派生的 `state.json`，核对 source/custom SHA、游标、选择与集成状态、重开条件。
2. 更新本轮 `implement.md`、验证报告和 `task.json` 的实际进度；阻塞或待授权保持未完成，不为了归档改成完成。
3. 按现有 Trellis 会话记录流程追加 `.trellis/workspace/qlqqs/` 当前 journal 并同步 index；先读取当前 active journal，不写死 journal-1，也不手工破坏自动管理区块。
4. 若本轮改变维护规则或既有行为，更新相应 guide、规范索引及仍用于描述当前状态的运维文档；不改写旧的归档报告来伪装历史结果。
5. 仅有真实授权及交付证据才更新推送/部署记录。未部署时保留原生产版本，标记本轮“未部署”。
6. 交叉核对账本、任务、journal 和当前状态文档，最终列出更新路径、仍未完成项及下一轮入口。无相关变化的文档记录“已核对，无需修改”及原因。

此步骤是可复用工作流的完成门禁，不只是首次创建台账时执行一次。
