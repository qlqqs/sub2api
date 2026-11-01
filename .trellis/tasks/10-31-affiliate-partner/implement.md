# 合伙人 — 执行计划

## 0. 前置条件

- Seedance 视频任务（`10-31-seedance-video`）的后端、前端改动已经测完并提交。两边都会改 `affiliate_consumption_settle.go`、`affiliate_distribution.go`、`affiliate_team_repo.go`、财务相关文件。
- 用户的待确认项已全部确认（2026-10-31 第二轮）：成本按分组和模型拆分、升级渠道经理时自动挂靠、设立和挂靠不可撤销。
- 用户已同意开工（2026-10-31）：Seedance 会话跑完后，主代理先提交 Seedance 的改动（只提交，不推送），再派子代理实现。
- `task.py start 10-31-affiliate-partner`。

## 1. 分工

| 代理 | 模型 | 范围 | 任务书 |
|---|---|---|---|
| BE（后端） | codex gpt-6-astra high（用户指定，mode full-access） | `backend/` 和 `docs/AGENT_PROFIT_SHARING_CN.md` | `task-be.md` |
| FE（前端） | claude opus-5.5 high（profile「前端任务执行」，mode bypassPermissions） | `frontend/` | `task-fe.md` |
| QA（测试） | codex gpt-6.1-sol high（mode full-access） | `~/workspace/sub2api-local` 环境、e2e（新增 `p14_partner.py`、`ui/partner_ui.cjs`、`CASES.md` PTN 节） | `task-test.md`（BE 和 FE 都完成后才启动） |

- BE 和 FE 同时开工。FE 先按 `contract.md` 写页面和单测，BE 完成后在本机环境联调。
- 主代理用 heartbeat 看门狗盯着 codex 代理，防止卡住无响应。

## 2. 共同规则（每个代理都要遵守）

1. 开始前先读：
   - `/home/qlqq/workspace/AGENTS.md`、`/home/qlqq/workspace/sub2api/AGENTS.md`
   - 本目录的 `prd.md`、`design.md`、`contract.md`，以及自己的任务书
   - 对应层的 `.trellis/spec/`（backend / frontend）
2. 不 `git commit`、不 `git push`、不 `git stash`、不 `git reset`、不切分支。工作区里有别人的未提交改动，不要回滚。
3. 只改自己负责的范围。确实要动对方的范围或改契约时，先停下，在任务书末尾的「约定变更」里写明，并通知主代理。
4. 不连七号机或任何远端，不用 `~/workspace/work/credentials.md`。
5. 禁止过度防御性编程：不为假设场景加校验、兜底、兼容分支或抽象。注释、文案简洁，写当前事实。代码风格跟随周边文件，sub2api 后端注释以中文为主，新代码标 `// CUSTOM(fork):`。
6. 工具：
   - 前端只用 `npx -y pnpm@9`，禁止全局 `pnpm`。
   - Go 单测：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`
   - 禁止 `docker system prune` 和 `docker image prune`。端口只监听 127.0.0.1。
7. 完成时在自己的任务书末尾追加「完成报告」，写明：
   - 改了哪些文件、做了什么
   - 跑了哪些测试、结果如何（失败要贴输出）
   - 遗留问题，以及需要别人配合的地方

## 3. 顺序与检查点

1. **BE**：迁移 → 写路径（合伙人、挂靠、自动挂靠、拦截、不发分润）→ 归属解析和利润引擎（纯函数和单测）→ 取数和集成测试 → 结算单 → 接口 → 财务联动 → 团队页的区域视角 → 文档。
2. **FE**：用户端（邀请页合伙人卡片、利润分成页签、区域团队消费）→ 后台合伙人页 → 代理列表 → 财务页 → 两套主题的导航。
3. **主代理检查点**：BE 完成报告提交后，主代理审一遍 `planAgentShares` 的改动和利润引擎的单测，再让 FE 联调。
4. **QA**：
   - 单测和前端检查。
   - `./build.sh && docker compose up -d`。
   - `run_all.sh --fast`、`run_all.sh`，其中 p14 两种模式都跑，界面部分只在完整模式跑。
   - 出报告。
5. **主代理验收**：
   - 看 QA 报告和截图。
   - 抽查金额是否和 SQL 一致。
   - 向用户汇报，经同意后提交、推送、部署。

## 4. 验证命令

```bash
cd /home/qlqq/workspace/sub2api/backend
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run 'Affiliate|Partner|Finance'

cd ../frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
git -C .. status --short     # 确认 pnpm-lock.yaml 没有改动

cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d
cd e2e && ./run_all.sh --fast && ./run_all.sh
```

## 5. 上线（用户同意后由主代理执行）

1. 提交并推送，等 CI 构建出 `custom-latest`。按 AGENTS.md 先备份七号机，再更新 `/root/sub2api-custom`。迁移 264 会自动执行。
2. 在后台把 75 号设为「三科大」、62 号设为「重大」合伙人，比例 40%。再按用户安排挂靠渠道经理。
3. 用户自己处理 vm2api：移出固定成本列表并设倍率。
4. 回滚：退回上一个镜像。62 号的档位手动改回普通用户。新表可以保留，旧代码不读。
