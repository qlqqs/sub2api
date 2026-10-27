# Upstream sync：增量审查与集成进度设计

日期：2026-10-27（Asia/Shanghai）。任务状态：planning；用户仅批准任务创建与本研究文档落盘，未批准实施、Git ref 修改、fetch、测试、提交、推送或部署。

范围仅为 upstream main 同步、通用维护补丁的选择性集成、持久化审查/集成记录及可重复的 upstream-sync 工作流。不涉及模型更新或接入流程。

## 1. 已观察的基线与证据

以下为本会话先前只读检查所得快照，不是本次重新读取或远端最新状态声明；其他代理可能继续修改工作区。

| 项目 | 2026-10-27 观察值 |
|---|---|
| 仓库 / 当前分支 | `/home/qlqq/workspace/sub2api` / `sub2api-custom` |
| custom HEAD | `98894b242c672937173f5af630eb65e68c565c6d` |
| 工作区 | 69 项未提交变更，含已跟踪修改和未跟踪文件 |
| 本地 main / 缓存 origin/main | `8f2cbb53d9d1c40e1865b07ac460571f7d4c48de` |
| 缓存 upstream/main | `dba770d56a0ee0ae5bad1b3dc5e11614d323163c`；提交时间 `2026-10-22T10:33:03+08:00`，不是 fetch 时间 |
| 图关系 | main 是缓存 upstream/main 的祖先，`main..upstream/main` 有 34 个提交；main 是 custom 的祖先，`main...custom` 左右计数为 0 / 84 |
| remotes | origin = `https://github.com/qlqqs/sub2api.git`；upstream = `https://github.com/Wei-Shaw/sub2api.git` |
| worktree | 当前仓库之外有 `/tmp/s2a-mid` 的失效 detached 登记；不可擅自 prune |

未 fetch，无法确认远端当前 head 或最近成功 fetch 时间。上述 SHA 是本地缓存证据，不应直接写成 `last_fetched` 的新事件。

已有历史记录：

- `.trellis/workspace/qlqqs/journal-1.md` Session 5：2026-10-20 同步上游与定制分支，记录基线 `8f2cbb53d`。
- `.trellis/tasks/archive/2026-10/09-21-admin-bind-affiliate-inviter/design.md`、`implement.md`：2026-10-20 同步兼容性、迁移保留与测试证据。
- 尚未发现逐提交审查/选择/集成台账；旧批次总结不能证明逐项曾按新流程审查。
- `.gitignore` 忽略 `docs/*`、根目录 `scripts`；随意在其中新增流程文件可能无法正常纳入版本控制。

## 2. 建议持久化位置（均为 proposed，未创建）

除当前任务研究文档外，以下全部仅为提议：

| proposed 路径 | 用途 |
|---|---|
| `.trellis/spec/guides/upstream-sync-guide.md` | 流程、安全门禁、审查单元与测试规范；将来在 guides index 添加入口 |
| `.trellis/upstream-sync/state.json` | 可重建的状态快照、基线、成功 fetch 证据、连续审查游标 |
| `.trellis/upstream-sync/decisions.jsonl` | 单写入者追加事件；逐项决定、重开、应用、集成、验证、交付及撤销 |

台账应保存在 custom 分支，不能提交到应与上游一致的 main。每轮任务保存批次计划和证据，长期台账不随任务归档丢失。初期不需要新脚本；自动化可后续单独审批。

## 3. 最小 schema 与状态语义

### state.json（派生快照，不是独立真相源）

- `schema_version`、`source_repo`、`source_ref`、`target_branch`。
- `baseline: {sha, kind: imported_baseline, evidence_paths}`：可用 `8f2cbb53d...`，明确其祖先关系与历史批次来源，不伪造此前逐提交审查。
- `last_fetched: {sha, fetched_at, run_id}` 或 null：仅成功 fetch 后更新；与 `observed_cached_head` 分开。
- `last_contiguous_reviewed: {sha, epoch, event_seq}`：固定基线之后，每个连续审查单元都有明确结论的最远位置，不代表入选、集成或部署。
- `last_event_seq`、`last_event_id`：与追加事件日志核对；日志先落盘、快照后更新，中断后由日志重建。
- `active_run: {run_id, upstream_target_sha, custom_base_sha, task_path}`：冻结本轮输入，不追随移动 ref。

### decisions.jsonl（追加事件，单写入者）

每条至少包含：

- `schema_version, event_id, idempotency_key, seq, run_id, recorded_at, actor`。
- `unit_key`：源仓库身份 + 完整 source commit SHA + review-unit 规则版本；PR 编号只是关联信息。
- `source: {commit_sha, parents, first_parent_sha, covered_commits, diff_digest}`。
- 可选 `pr: {repo, number, reviewed_head_sha, reviewed_base_sha, merged_commit_sha}`；未取得的字段为 null，不猜测。
- `event_type`、`review_revision`、`supersedes_event_id`、`decision`、`reason`、`dependencies`、`evidence_paths`。
- `reopen: {triggers, due_at, related_units}`，以及所依据的 custom 基线、相关路径/契约指纹。
- `integration: {method, source_shas, candidate_shas, custom_shas, custom_base_sha, verified_target_sha, equivalence_kind, evidence_paths}`。
- `validation: {target_sha, dirty_snapshot_digest, commands, outcomes, evidence_paths}`；若验证含未提交改动，不能仅用 HEAD 冒充精确验证对象。
- 可选 `shipment: {push_remote, push_ref, pushed_sha, image_digest, environment, deployed_sha, verified_at, evidence_paths}`；各字段分别记录，不从前一阶段推断后一阶段。

### 必须区分的状态

1. **reviewed**：已有针对固定输入的审查决定；决定可为 selected、deferred、skipped。
2. **selected**：批准进入候选集，依赖明确；还未必应用成功。
3. **applied_in_candidate**：补丁已在隔离候选中形成变更或提交；不是 custom 已集成。
4. **integrated**：记录的 custom 提交确实可从目标 custom head 到达，相关补丁/适配仍存在，所要求的验证已通过。回滚必须追加事件。
5. **shipped**：交付证据单独记录。push、CI 出镜像、部署、真机验证互不等价；若使用单一 shipped 状态，应明确要求目标部署及验收证据齐全。

## 4. 增量与“恰好一次”审查

这里的恰好一次指同一不可变输入/审查版本不重复计入、可恢复地复用决定，不承诺永远无需重审。

1. 经实施授权后 fetch，冻结上游目标与 custom 基线；先验证旧基线/游标仍在目标祖先链上。上游历史改写、缺对象或本地分叉时停止自动推进，人工建立新 epoch 和映射，不能跳过丢失历史。
2. 使用上游 main 的 first-parent 顺序构造审查单元。普通提交检查相对父提交增量；merge 提交检查相对第一父提交的完整净增量，并记录新引入的非第一父历史。
3. **覆盖 merge resolution**：不能只审子提交或 PR diff；必须检查最终 merge tree 相对第一父的增量，必要时对照双方父提交/自动合并结果，涵盖合并时额外修改。一个 merge 单元的子提交仅作覆盖证据，不重复计算为独立已审查单元。
4. 队列 = 新增未决定单元 + 到达重开条件的历史单元。跳过已有同版本有效决定；早期存在缺口时只阻止连续游标，不丢掉后续已完成决定。
5. 主代理单独追加事件；子代理返回结构化建议，不并发写日志。用 `idempotency_key` 拒绝重复事件；重复相同结果复用，矛盾结果需明确新 revision，不能静默覆盖。
6. deferred/skipped 若含完整理由与重开条件，可推进审查游标，但不进入集成计数。游标是历史覆盖水位；旧项重开进入独立 pending 队列，水位不伪装为“所有决定当前仍有效”。批次完成要求新范围覆盖且应处理的 reopen 队列已处理或明确延期。
7. PR head 或 base 改变，旧审查只适用于旧 SHA 对；创建新 revision，对比变化及依赖影响，保留可复用证据。未合并 PR 不推进 main 游标；合并后对照实际 source commit/tree，不能用 PR 标题或编号继承通过结果。

### cherry-pick / 等价证明

- Git 祖先关系只能说明提交在图中可达，不能证明没有后续回滚或语义破坏。
- cherry-pick 通常改变 SHA，不可因上游提交不是 custom 祖先就断言未集成。
- `-x` trailer 仅为来源声明；patch-id 仅为候选等价线索，不能代替依赖、上下文、二进制/merge 及最终行为核对。
- 允许一对多/多对一 source→custom 映射，记录直接应用、冲突适配或人工语义等价的方式与证据。
- custom 后续相关代码变化、回滚或依赖移除时，历史集成事件保留，当前有效性需重验，必要时追加 reverted/reopen 事件。

## 5. deferred / skipped / reopen

- deferred：当前不集成，明确缺失依赖、风险或等待条件，设置责任方和可检查触发器/日期。
- skipped：当前策略明确不需要，记录为什么；不是永久免审。
- 触发器：上游后续修复/依赖到位、PR head/base 改变、custom 相关实现变化、测试暴露回归、策略或需求改变、到期、用户要求。
- 只重开受影响范围；不要每次同步把所有历史补丁重新读一遍。路径交集仅用作提示，不能替代跨层依赖与契约判断。

## 6. 安全执行边界与验证序列（待授权）

1. 重新只读检查 status、refs、worktrees、正在进行的 merge/cherry-pick/rebase，以及磁盘空间和其他代理占用；记录固定输入，不修改他人文件。
2. 当前脏工作区保持原样，不自动 stash/checkout/reset。main 同步放在独立干净 worktree，只允许 fast-forward；分叉停止，禁止 force。更新 origin/main 需单独推送授权。
3. 选择性候选从固定 **custom HEAD** 建立，而非从 main 建立；只应用批准的通用维护单元与必要依赖，不整体 merge main 导致未选内容混入。
4. 候选 worktree 完成单元/集成/构建验证。它只证明“已提交 custom 基线 + 所选补丁”，不证明与当前 69 项未提交改动兼容。
5. 父工作区改动由用户/主代理协调后再整合；若 custom 基线已移动，重新整合并验证最终组合。候选通过与父 custom 通过分开记录。
6. 本机测试全部通过后，仍须用户同意才提交、推送、部署。不得触碰无关项目/容器；一号机项目仅为 sub2api-custom。

### 共享本机环境必须串行

`/home/qlqq/workspace/sub2api-local/build.sh` 支持 `SUB2API_REPO` 指向候选 worktree，但 `image/` 输出目录、`sub2api-local:latest` 镜像、compose 服务、数据库及 e2e 数据都是共享的。多代理不能同时 build/up/reset/e2e；主代理需取得明确独占窗口并记录使用的 SHA/快照和镜像身份。

不同 worktree 的前端产物和 node_modules 不能假定可用；跨基线不得错误复用 `--no-web` 产物。禁止全局 Docker 清理、远端 e2e 或把本机监听改成 0.0.0.0。

## 7. 迁移与测试约束

证据：`backend/internal/repository/migrations_runner.go:174–218`、`.trellis/spec/backend/database-guidelines.md`。

- runner 对完整 SQL 文件名排序；`schema_migrations.filename` 为主键；checksum 为 `SHA256(strings.TrimSpace(SQL))`。不是“最大编号之后才执行”。
- 同数字前缀但文件名不同可共存；这不保证 SQL 对象、顺序或依赖兼容。升级库可能已运行较高排序的 custom 迁移，空库与升级库的实际执行顺序因此可能不同。
- 已应用迁移不可修改、删除、重命名；修复用前向迁移，不任意修改数据库 checksum 或增加兼容豁免。必须验证空库迁移与既有 custom 数据库升级两条路径。
- 后端单测使用 unit tag，去除大小写 HTTP(S) 代理变量及 OPENAI_API_KEY，避免本地桩请求被代理和意外外部 API 调用；补充受影响 repository 集成测试与 go build。
- 前端固定 `npx -y pnpm@9`，禁止全局 pnpm；跑 lint/typecheck/test 与必要构建，检查锁文件未漂移。
- 本机镜像和 e2e 使用用户指定隔离环境；相关业务、迁移、路由共存和跨层契约回归不能由“补丁无冲突”替代。未运行、跳过、既有失败应与通过明确区分。

## 本文落盘边界

本次仅创建本研究文档；未创建以上 proposed 路径，未执行 fetch/tests，未修改产品、spec、脚本或 Git refs。PRD/design/implement 由主代理维护，补丁评估由另一代理维护。
