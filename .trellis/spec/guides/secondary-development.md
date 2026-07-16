# 二次开发与上游同步约束

> CUSTOM: fork 二次开发规范。适用于在本 fork（`custom` / `feature/*`）上叠加自定义功能，并持续吸收上游 `Wei-Shaw/sub2api` 更新。

本指南补充 backend / frontend 层规范，不替代各层具体约定。目标：让自定义与上游边界清晰，降低 `sync/*` 冲突成本。

## 何时必读

出现以下任一情况时，先读本指南再改代码：

- 在本 fork 上开发或审查自定义功能；
- 准备修改上游已有核心文件（handler、service、巨型 Vue 页面、共享 schema 等）；
- 准备合入 `custom` 前的自检；
- 开始 `sync/vX.Y.Z` 或按需中途同步前；
- 新增自定义 SQL migration 或改动依赖。

## 分支与同步（摘要）

| 分支 | 职责 |
| --- | --- |
| `main` | 仅镜像已接纳的上游稳定 release（fast-forward） |
| `custom` | 可部署的二开版本 |
| `feature/<name>` | 单功能开发 |
| `sync/vX.Y.Z` | 稳定 release 升级验证 |
| `sync/upstream-main-YYYYMMDD` | 按需中途同步（例外路径） |

规则：

1. **现在就能二开**：以当前可用 `custom` 为起点，不等 release。
2. **默认同步稳定 release**；release 过远或急需上游修复时，才走按需中途同步，且必须审查、验证、可回滚。
3. feature 开发期间只从 `custom` 更新，**不得**直接 `merge upstream/main`。
4. 禁止 force push / 改写共享历史；推送时写明分支名，避免 `--all` / `--tags`。
5. `.trellis/`、`.cursor/`、`.agents/` 仅用独立 `chore` 提交，不与业务混提。

完整流程见任务文档 `.trellis/tasks/07-15-downstream-upstream-sync/`（design / implement）。

## 扩展优先级（必须按序评估）

| 优先级 | 方式 | 对 sync 的影响 |
| --- | --- | --- |
| 1 | 配置 / 特性开关 / 环境变量 | 最低 |
| 2 | 新模块 + 薄注册点（路由 / DI / 导航） | 低 |
| 3 | 包装 / 装饰现有 service | 中 |
| 4 | 修改上游核心逻辑 | 高；必须说明为何 1–3 不可行 |

禁止默认选择优先级 4。

## 代码落点

1. **优先新增文件**：backend 新 handler/service/repository；frontend 新页面/组件/API 模块。
2. **上游文件只做窄改**：仅改必要挂点（路由注册、DI 装配、导航菜单、配置 schema、迁移注册等）。
3. **冲突高发区克制**：超大 Vue 页面、巨型 service、OAuth/billing/gateway 核心路径；避免在大段逻辑中间硬插业务。
4. **跨层成套**：API 变更同时规划 handler → service → repository → migration → frontend API/类型/页面。
5. **禁止顺手重构上游**：格式化、重命名、无关提取不得与功能提交混做。

## 二开标记 `CUSTOM:`

在**被修改的上游原有文件**内使用统一可检索标记：

- Go：`// CUSTOM: <简述>` 或 `// CUSTOM:begin` … `// CUSTOM:end`
- Vue/TS/JS：`// CUSTOM: <简述>` 或 `{/* CUSTOM: <简述> */}`
- SQL（自定义迁移文件头）：`-- CUSTOM: <简述>`

规则：

- 纯自定义新文件：文件头一条 `CUSTOM` 说明即可。
- 修改上游文件：每个逻辑插入点至少一条标记。
- sync 前运行：`rg "CUSTOM:"` 列出全部分叉点。
- 删除自定义时同步删除标记。

## 自定义 migration

上游命名（见 `backend/migrations/migrations.go`）：

- 格式：`NNN_description.sql`（零填充数字前缀 + 下划线小写描述）
- 幂等；**禁止修改已应用的上游 migration 内容**（checksum 校验）

自定义约束：

1. 文件名必须包含 `custom_` 段，例如下一序号可用时：`178_custom_<slug>.sql`。
2. 选取序号时：使用当前仓库最大数字前缀的下一可用号；**若与即将合入的上游 migration 撞号**，在 `sync/*` 中仅调整自定义文件名/序号以消除冲突，仍不改上游文件内容。
3. 文件头写：`-- CUSTOM: <简述>`。
4. 每次 sync 审查 `backend/migrations/` 新增与顺序兼容性；生产升级前备份并演练。
5. 迁移为 forward-only；回滚依赖备份或补偿迁移，不依赖 down。

## 依赖边界

- 新增 Go / npm 依赖须说明用途；优先复用已有依赖。
- 不主动大版本升级上游已锁定依赖，除非本次 sync 上游已升，或存在无更低风险替代的安全漏洞。
- lockfile 变更与真实依赖变更同提交。

## 提交分类

| 类型 | 允许 | 禁止混入 |
| --- | --- | --- |
| `feat` / `fix`（feature） | 单一业务意图 + 测试 | workflow 目录、无关格式化、大范围整理上游 |
| `fix`（sync 兼容） | 吸收上游必需的冲突解决与适配 | 新业务功能 |
| `chore`（workflow） | `.trellis/`、`.cursor/`、`.agents/` | 业务代码 |
| `chore`（deps） | 依赖与 lockfile | 功能逻辑 |

## 合入 `custom` 前自检

- [ ] 是否按扩展优先级 1–3 评估过，而非直接改核心？
- [ ] 上游文件 diff 是否仅为挂点级改动？
- [ ] `rg "CUSTOM:"` 能否列出本次分叉点？
- [ ] 自定义 migration 是否含 `custom_` 且未改上游 migration？
- [ ] 提交是否单一意图、可独立 revert？
- [ ] 适用测试与构建是否通过（见项目 `make` 目标）？

## sync 前额外检查

- [ ] `rg "CUSTOM:"` 对照将冲突的上游文件
- [ ] 列出 `backend/migrations/` 变更；自定义与上游新迁移顺序兼容
- [ ] 兼容修复单独提交，不夹带新业务
- [ ] 验证失败则放弃 `sync/*`，保持 `custom` 不变

## 与其它指南的关系

- 写业务代码前仍读 [代码复用](./code-reuse-thinking-guide.md) 与 [跨层](./cross-layer-thinking-guide.md)。
- backend / frontend 层规范仍适用；本指南只约束 **fork 边界与同步友好性**。

## 相关功能规范

- 上游账号余额查询（只读、敏感凭证、`CUSTOM:` 窄挂点）：[backend/upstream-balance.md](../backend/upstream-balance.md)
