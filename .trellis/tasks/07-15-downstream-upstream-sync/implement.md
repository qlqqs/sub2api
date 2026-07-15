# 执行计划

## 阶段一：立即开工准备（不阻塞业务）

- [x] 确认当前 `custom` 指向可用基线（当前为 `eb2b8632`），工作树状态已知。
- [x] 可选：创建只读安全指针 `backup/pre-custom-dev-eb2b8632`。
- [x] 从 `custom` 创建 `chore/trellis-workflow`，仅提交 `.trellis/`、`.cursor/` 和 `.agents/`。
- [x] 审查该提交不包含运行时状态、缓存、凭据或业务文件，验证后合并回 `custom`。
- [ ] 推送 `custom`（如需要），并为 `main`、`custom` 配置禁止 force push 的分支保护。
- [x] **此后即可从 `custom` 创建 `feature/<name>` 开始正式二次开发。**

## 阶段二：首次 release 基线对齐（可与功能开发并行，稍后执行）

- [ ] 获取最新上游引用：`git fetch upstream --tags --prune`。
- [ ] 查看最新 release：`gh release list --repo Wei-Shaw/sub2api --limit 10`。
- [ ] 选定目标 tag 后验证其包含当前基线：`git merge-base --is-ancestor eb2b8632 <tag>`（或当前 `main` 的 tip）。
- [ ] 查看 release 说明和差异：`gh release view <tag> --repo Wei-Shaw/sub2api`、`git log --oneline <old>..<tag>`。
- [ ] 若祖先验证失败，立即停止，不移动任何长期分支。
- [ ] 切换 `main`，执行 `git merge --ff-only <tag>`，确认与目标 tag 同提交后 `git push origin main`。
- [ ] 若 `custom` 尚无二开提交：可 `git merge --ff-only main`。
- [ ] 若 `custom` 已有二开提交：走阶段四 release 同步模板（`sync/vX.Y.Z`）。

## 阶段三：验证清单（每次合入 `custom` 前）

- [ ] 安装前端锁定依赖：`pnpm --dir frontend install --frozen-lockfile`。
- [ ] 运行后端单元测试：`make -C backend test-unit`。
- [ ] 运行后端集成测试：`make -C backend test-integration`。
- [ ] 运行前端检查：`make test-frontend`。
- [ ] 运行完整构建：`make build`。
- [ ] 检查 GitHub Actions 与本地结果一致；失败时不把待验证变更合入 `custom`。

## 后续功能开发模板

- [ ] 从 `custom` 创建 `feature/<name>`。
- [ ] 在 Trellis 中为具体功能单独规划，读取对应 backend/frontend 规范。
- [ ] 按扩展优先级选型：配置/开关 → 新模块+注册点 → 包装 → 最后才改上游核心。
- [ ] 优先新增独立文件；上游文件只做路由/DI/导航等窄挂点改动。
- [ ] 修改上游文件时添加 `CUSTOM:` 标记；纯自定义文件在文件头标注即可。
- [ ] 自定义 migration 使用含 `custom_` 的独立命名，禁止改上游 migration 内容。
- [ ] 保持提交小而聚焦（单一意图），并为高风险行为添加针对性测试。
- [ ] 开发期间只从 `custom` 更新 feature，不直接 merge `upstream/main`。
- [ ] 合入前自检：`rg "CUSTOM:"` 列出分叉点；上游 diff 是否仅挂点级。
- [ ] 完成检查后合并回 `custom`，确认 `custom` 仍可部署。

## 后续 release 同步模板（默认路径）

- [ ] 获取并核验目标 release tag，审查 release notes、提交和文件差异。
- [ ] sync 前运行 `rg "CUSTOM:"`（或等价）列出全部二开分叉点，对照将冲突的上游文件。
- [ ] 单独列出 `backend/migrations/` 新增或修改内容，并在生产升级前完成数据库备份。
- [ ] 核对自定义 migration 与上游新 migration 的顺序兼容性（不得改上游 migration 文件）。
- [ ] 用 `--ff-only` 将 `main` 更新到目标 tag。
- [ ] 从 `custom` 创建 `sync/<tag>`，执行 `git merge --no-ff main`。
- [ ] 在同步分支解决冲突；兼容修复单独提交，不夹带新业务功能。
- [ ] 运行阶段三的完整验证。
- [ ] 必要时在测试环境完成数据库升级和恢复演练。
- [ ] 验证通过后将同步分支以 `--ff-only` 合回 `custom`；失败则保留日志并放弃该同步分支。

## 按需中途同步模板（例外路径）

适用：release 过远，或急需上游已合入但未发版的修复/功能。

- [ ] 明确业务原因与目标范围（完整 `upstream/main` 还是特定 commits）。
- [ ] `git fetch upstream`，审查 `git log --oneline custom..upstream/main` 与 diff，重点看迁移与依赖。
- [ ] 从 `custom` 创建 `sync/upstream-main-YYYYMMDD`（或 cherry-pick 分支）。
- [ ] **不要**用这次同步更新长期 `main`（`main` 仍只镜像稳定 release）。
- [ ] 在同步分支合并/拣选、解决冲突、补兼容修复。
- [ ] 运行阶段三验证；涉及迁移则先备份/演练。
- [ ] 通过后合回 `custom`，并记录来源 commit range 与原因。
- [ ] 下次正式 release 同步时复核中间态变更，避免重复冲突。

## 风险门与停止条件

- 目标 release tag 不是当前 `main` 的后代时停止（release 路径）。
- 工作树出现非预期改动时停止并向用户确认。
- 发现迁移文件被修改、重排或 checksum 变化时停止，先完成迁移兼容性审查。
- 任何步骤需要 force push、历史重写或批量推送 tags 时停止，重新设计操作。
- 试图把无审查的 `upstream/main` 长期跟踪当作默认同步节奏时停止。
- 功能 PR 大面积改上游核心且未说明为何跳过扩展优先级 1–3 时，不合并。
- 测试失败或自定义部署产物尚未明确时，不宣告 `custom` 可用于生产。

## 阶段四：将二开约束沉淀为项目 spec（执行阶段，规划已写清要求）

- [x] 新增 `.trellis/spec/guides/secondary-development.md`（或 `fork-constraints.md`），内容从 `design.md`「代码级二次开发约束」提炼为可执行规范。
- [x] 更新 `.trellis/spec/guides/index.md`，挂上该指南及触发条件（fork 二开 / sync 前 / 改上游核心文件前）。
- [x] 可选：在 backend/frontend `index.md` 增加「二开时必读」链接。
- [x] 该步骤与 workflow chore 提交可同批或紧随其后，但不得与业务功能提交混在一起。
