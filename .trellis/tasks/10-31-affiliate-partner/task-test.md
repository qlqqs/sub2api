# QA：本机全链路测试任务书（codex gpt-6.1-sol high）

BE 和 FE 都写完「完成报告」后启动。你的职责是**验证并报告问题**：
- 发现缺陷时，先写清楚复现步骤和证据。
- 明显的笔误或测试脚本问题，可以直接修，并在报告里列出来。
- 业务逻辑的修复写进报告，由主代理派回 BE 或 FE。

开始前按 `implement.md` 第 2 节「共同规则」读文档，再通读 `prd.md`、`design.md`、`contract.md`，以及 BE、FE 两份完成报告。

## 0. 实现后的补充（主代理，2026-10-31）

- 契约的实际变化，以 `task-be.md`、`task-fe.md` 的「约定变更」为准：
  - 业务错误码在响应的 `reason` 里，`code` 是 HTTP 状态码，`message` 是中文。断言用 `reason`。
  - 区域团队成员包含合伙人本人。
  - 区域 `team/overview` 支持 `user_id`，区域外客户返回 403。
  - 合伙人直属团队页的路由是 `/affiliate/region/direct`（不是 `/affiliate/team`），数据仍走 `/user/aff/team/*`。
- 界面选择器见 `task-fe.md` 第一阶段报告末尾的 `data-test` 列表。
- 本机库里有 BE、FE 联调留下的测试数据（`partner-be-*`、FE 自建的账号）。p14 自建全部角色，不依赖、不清理这些数据。
- 前端跑测时 nav spec 有 `ECONNREFUSED 127.0.0.1:3000` 的 stderr，测试本身通过。FE 在这两个文件里只加了一行菜单，不用处理。

## 1. 静态检查与单测

```bash
cd /home/qlqq/workspace/sub2api/backend
go build ./...
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run 'Affiliate|Partner|Finance'
cd ../frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
git -C .. status --short     # 确认 pnpm-lock.yaml 没有改动
```

## 2. 本机环境

```bash
cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d
```

- 本机结算每 10 秒跑一次，处理 10 秒前的消费。查分润流水前先等约 25 秒。
- e2e 驱动见 `e2e/lib.py`，常用的有：`register`、`set_level`、`recharge`、`gateway_messages`、`ledger_mark` / `ledger_since`、`x` / `q` / `q1`、`check`、`expect_error`。
- 消费走网关真实请求，上游是 `upstream-mock`。用量在提示词里用 `E2E_USAGE in=… out=…` 指定，参考 p6 和 p10 的写法。

## 3. 新阶段 `e2e/p14_partner.py`

- 两种模式都跑，放在 `run_all.sh` 的 `p12_seedance.py` 之后。界面部分 `p14_partner.py ui` 只在完整模式跑。
- 用例写进 `e2e/CASES.md` 的新一节「PTN 合伙人」。
- 每轮自建分组和上游账号，账号倍率设为 0.5。也自建用户，账号名形如 `e2e-<HHMMSS>-<角色>@e2e.test`。不依赖其他阶段的状态。
- 支付实例通道费率临时设为 3.2%，跑完恢复，做法同 p10。

### 拓扑

| 角色 | 构造方式 | 预期归属 |
|---|---|---|
| PAU | 用 PA 的码注册，此时 PA 还是普通用户 | 无（绑定快照里没有代理） |
| PA | 普通用户，后台设为合伙人 | PA |
| PB | 先设为渠道经理（无团队），再设为合伙人（对应七号机 75 号） | PB |
| PC1 | 用 PA 的码注册 | PA，直属团队 |
| PC2 | 用 PC1 的码注册 | PA，直属团队 |
| PD | 用 PA 的码注册，后台设为分销代理 | PA，直属团队 |
| PD1 | 用 PD 的码注册 | PA，直属团队（upline 为 PA） |
| X | 后台设为渠道经理，并挂到 PA 名下 | PA，X 线 |
| X0 | 在 X 挂靠**之前**用 X 的码注册 | PA，X 线（起点为挂靠时刻） |
| X1 | 挂靠后用 X 的码注册 | PA，X 线 |
| XD | 用 X 的码注册，由 X 任命为分销代理 | PA，X 线 |
| XD1 | 用 XD 的码注册 | PA，X 线 |
| PC3 | 用 PA 的码注册，后台升为渠道经理（自动挂靠） | PA，PC3 线 |
| PC31 | 用 PC3 的码注册 | PA，PC3 线 |
| Y | 渠道经理，挂到 PB 名下 | PB |
| Y1 | 用 Y 的码注册 | PB |
| K | 渠道经理，不挂靠 | 无 |
| K1 | 用 K 的码注册 | 无 |
| O | 无邀请人 | 无 |

### 用例

| 编号 | 场景 | 预期 |
|---|---|---|
| PTN-01 | 预览并设 PA 为合伙人 | 预览返回档位 3、团队 0。设立后 `agent_level=1`、`locked=true`；rates 有当月 0.4000；列表能看到 PA |
| PTN-02 | 设 PB 为合伙人 | 成功 |
| PTN-03 | 重复设 PA；挂靠 X 之后再设 X 为合伙人 | 依次返回 409 `PARTNER_EXISTS`、409 `PARTNER_AGENT_ATTACHED` |
| PTN-04 | 后台改 PA 的档位（单个 PUT、批量） | 返回 409 `PARTNER_LEVEL_FIXED`，库里不变 |
| PTN-05 | PA 在用户端任命 PD；查看 PA 的 `GET /user/aff` 和邀请列表 | 任命返回 403 `PARTNER_CANNOT_APPOINT`；`appoint` 为 null；每项的 `appointable` 都是 false |
| PTN-06 | 挂靠 X 到 PA；再把 X 挂到 PB；挂靠 PC1（不是 1 级）；挂靠 PB（是合伙人）；尝试 DELETE 或改挂 | 依次为：成功；409；400 `PARTNER_AGENT_NOT_LEVEL1`；409 `PARTNER_AGENT_IS_PARTNER`；没有这个路由（404 或 405） |
| PTN-07 | 后台把 PC3 设为 1 级 | 响应 `auto_attached` 含 PC3 → PA；表里 `source=promote` |
| PTN-08 | 管理端代理列表的 `partner` 字段和 `partner_id` 筛选 | 与拓扑表一致；PA、PB 的 `is_partner=true` |
| PTN-09 | 分润：PD1、PC1、X1、XD1、PC31 各消费一次 | PD 拿 7%，PA 没有任何流水；PC1 没有流水；X 拿 11%；XD 拿 7%、X 拿 4%；PC3 拿 11% |
| PTN-10 | 普通邀请奖励：设成首充固定奖励模式（参照七号机：固定 1，门槛 10）；PA 设立后，PAU 首充 10 | 没有给 PA 的 `invite_fixed` |
| PTN-11 | 计入起点：X0 在挂靠前、挂靠后各消费一次（同一天） | 区域收入只算挂靠之后的部分：当天收入 × 挂靠后消耗 ÷ 当天消耗；成本、分润也只算挂靠之后 |
| PTN-12 | 当月数字（管理端 statements 的当月条目，与合伙人端 overview 一致） | 收入、成本、分润、专项支出、调整项、利润、分成，都与 SQL 手算一致（误差 < 0.01）。<br>收入的口径：赠送优先、付费先进先出、单位实收 = pay × (1 − 3.2%) ÷ 本金；至少一个成员有充值赠送、一个成员有部分退款 |
| PTN-13 | 各线 | 各线的消耗、收入、成本、分润之和等于合计；PA 直属团队为一行，X、PC3 各一行 |
| PTN-14 | 成本拆分与隐私 | `cost_by_group`、`cost_by_model` 各自加总等于 `account_cost`。<br>合伙人端所有响应的 JSON 里，搜不到本轮账号名，也没有 `account_id` 或 `account_name` 字段 |
| PTN-15 | 历史月份与亏损结转：用 SQL 把 PA 的 `created_at` 和 rates 的 `effective_month` 改到两个月前（M−2）；M−2 加调整项 −50，M−1 加调整项 +80 | M−2：利润 −50、分成 0.00、期末亏损 50。<br>M−1：利润 80、期初 50、基数 30、分成 12.00、期末 0 |
| PTN-16 | 确认顺序 | 先确认 M−1：400 `PARTNER_PREVIOUS_UNCONFIRMED`。<br>确认当月：400 `PARTNER_MONTH_NOT_ENDED`。<br>确认 M−2：成功；再确认一次：409。<br>给 M−2 加调整项：409 `PARTNER_MONTH_CONFIRMED`。<br>确认 M−1：期初 50 |
| PTN-17 | 冻结 | 确认 M−1 后，用 SQL 给区域成员插一条 M−1 的 usage_logs，M−1 明细不变；当月预估不受影响 |
| PTN-18 | 标记已付 | 对 M−2 标记已付：成功，记录了 `paid_on` 和备注。<br>再标记一次：409 `PARTNER_MONTH_PAID`。<br>对当月（未确认）标记：400 `PARTNER_MONTH_NOT_CONFIRMED` |
| PTN-19 | 改比例为 0.35 | 返回 `next_share_rate=0.3500`、`next_rate_month=下月`；当月仍按 0.4000 |
| PTN-20 | 越权 | 普通用户访问 `/user/aff/partner/*`：403。<br>PB 看 X：403。<br>PA 看区域外客户 K1 的请求明细：403。<br>PB 的列表里只有 PB 自己的数据 |
| PTN-21 | 区域团队消费 | `overview` 的消费 = 区域成员在范围内的 rollup 之和（SQL）。<br>`lines` 正确。<br>客户邮箱脱敏。<br>点进 X 的 overview，与 X 自己调 `/user/aff/team/overview` 的结果一致 |
| PTN-22 | 财务页 | 月粒度当月的 `partner_share` = PA 与 PB 当月分成之和，`partner_share_estimated=true`，`net_profit = profit − partner_share`。<br>带区域的支出在全站 `expenses` 里只计一次。<br>区域填不存在的合伙人：400 |
| PTN-24 | 对 PC1 调设立合伙人的预览接口（只预览，不设立） | `current_region` = PA |
| PTN-23 | 专项支出摊销：给 PB 加一笔从上月 20 日起摊 30 天的 ¥30 支出（用 PB，避免影响 PTN-15 对 PA 的预期） | 当月 `region_expense` = 落在当月的那几天的摊销额。PB 设立在当月，上月那部分不出现在任何结算单里 |

### 界面 `e2e/ui/partner_ui.cjs`（完整模式）

- 截图放在 `runs/<时间>/partner_ui/`，浏览器时区设为 Asia/Shanghai，写法参考 `ui/team_ui.cjs`。
- 合伙人 PA：
  - 邀请页：显示合伙人卡片，没有任命按钮。
  - 三个页签。
  - 利润分成页、明细页。
  - 区域团队消费：区域 → X → XD → XD1 请求明细，逐层真实点击。
- 管理员：
  - 合伙人列表、设为合伙人对话框（预览 → 确认）。
  - 详情三个页签、挂靠对话框。
  - 结算单按钮状态：草稿可确认，当月不可确认，已确认可标记已付。
  - 代理列表的合伙人列和筛选。
  - 财务页月度表的两列。
- 默认主题和 qx 主题各检查一次入口。

## 4. 回归

- `run_all.sh --fast` 和 `run_all.sh` 全部通过：p0–p13 不改断言，加上 p14。
- p10 FIN 不改断言就要通过，因为合伙人只新增字段。

## 5. 报告

在本任务书末尾写「测试报告」，包括：
- 各命令的结果；
- e2e 的 `runs/<时间>/report.md` 路径，以及失败用例的证据；
- 截图目录；
- 发现的问题，按严重程度排序，每条写清复现步骤、预期和实际。

## 测试报告

### 结论与环境

- 报告时间：2026-11-01 06:18（北京时间）；2026-10-31 22:18 UTC。
- 测试版本：工作区 HEAD `c22f82ada`，本机构建含 BE / FE 未提交实现，镜像 commit 标识 `c22f82ada-dirty`。
- 月份统一按北京时间：当月 `2026-11`，M−1 `2026-10`，M−2 `2026-09`；Playwright 使用 `Asia/Shanghai`。
- 首次测试前已执行 `./reset.sh`，再执行 `./build.sh && docker compose up -d` 完整构建；最终 fast / full 验收轮次前分别再次清库，PTN-22 核对时全站仅有 p14 自建的 PA / PB，无旧联调合伙人干扰。
- **新功能 PTN 111/111 通过；fast 367/367 通过；全量加单独 p11 / p13 为 601/602，通过条件尚未满足。唯一失败为既有 TAUI-01 菜单顺序断言，与新增菜单位置冲突。**
- 未修改产品代码或 p0–p13 断言，未提交、推送、部署或连接远端；业务问题仅记录，交主代理 / FE 处理。

### 本轮交付

以下路径均位于 `/home/qlqq/workspace/sub2api-local/e2e/`：

| 文件 | 内容 |
|---|---|
| `p14_partner.py`（新增） | 独立创建全部角色、分组、倍率 0.5 的上游账号；真实支付回调和网关消费；覆盖 PTN-01–24，含 SQL / Decimal 独立金额核对、权限与隐私、冻结、结转、财务、CSV；API 共 96 条断言。`finally` 恢复设置、支付费率和奖励配置，含 PTN-99 恢复检查。 |
| `ui/partner_ui.cjs`（新增） | 真实登录与点击，合伙人各页、区域逐层下钻、管理员设立 / 挂靠、结算按钮状态、代理筛选、财务列 / 选择器、CSV、qx / 默认主题入口及移动端；14 组浏览器检查，加 Python 执行结果共 15 条。浏览器阻止非本机请求。 |
| `ui/partner_default.mjs`（新增） | 仅为默认主题入口验收启动临时 Vite 服务，监听 `127.0.0.1:3300`，内存切换主题，不改前端源码；日志和缓存落在运行目录，结束后关闭。 |
| `CASES.md` | 新增「PTN 合伙人」用例及运行 / 截图说明。 |
| `run_all.sh` | `p12_seedance.py` 后加入 p14 API，两种模式均运行；完整模式再运行 `p14_partner.py ui`，保留原有阶段及断言。 |

本仓库仅填写本文件「测试报告」。17 个既有 p0–p13 / lib 文件 SHA-256 与执行前一致，证据为 `partner-qa-checks/existing-e2e-sha256.json`、`existing-e2e-unchanged.json`；既有 `ui/admin_team_ui.cjs` 断言原样保留。

### 命令与结果

所有运行证据根目录：`/home/qlqq/workspace/sub2api-local/e2e/runs/`。静态检查、单测和构建日志位于其下 `partner-qa-checks/`，每项有同名 `.rc` 文件。

| 命令 / 阶段 | 结果 | 证据 |
|---|---|---|
| `go build ./...` | 通过，退出 0 | `partner-qa-checks/go-build.log` |
| `go test -tags unit ./...` | 通过，退出 0；运行时去掉大小写 HTTP(S) 代理及 `OPENAI_API_KEY` | `partner-qa-checks/go-unit.log` |
| `go test -tags=integration ./internal/repository/ -run 'Affiliate\|Partner\|Finance' -count=1 -v` | 通过，退出 0；125 条 RUN 记录（含子测试），无跳过；同样去掉代理及 `OPENAI_API_KEY` | `partner-qa-checks/go-integration.log` |
| `npx -y pnpm@9 run lint:check` | 通过，退出 0 | `partner-qa-checks/frontend-lint.log` |
| `npx -y pnpm@9 run typecheck` | 通过，退出 0 | `partner-qa-checks/frontend-typecheck.log` |
| `npx -y pnpm@9 run test:run` | 456 个测试文件、3602 条测试通过，退出 0 | `partner-qa-checks/frontend-test.log` |
| `./build.sh && docker compose up -d` | 完整前后端构建、镜像构建、启动通过，退出 0 | `partner-qa-checks/build.log` |
| `./run_all.sh --fast` | **367/367 通过**，其中 PTN 96/96，退出 0 | `20261031-215217/report.md`、`run.log`、`results.jsonl` |
| `./run_all.sh` | **577/578 通过**，仅 TAUI-01 失败，退出 1；PTN 111/111 | `20261031-215833/run.log`、`partner-qa-checks/full-console.log`、`full.rc` |
| 同一 full `E2E_RUN_DIR` 单独运行 `python3 -u e2e/p11_finance_recalculation.py` | FRC **14/14 通过**，退出 0 | `20261031-215833/p11.log`、`partner-qa-checks/p11.rc` |
| 同一 full `E2E_RUN_DIR` 单独运行 `python3 -u e2e/p13_studio_video.py` | Nova 运行中，默认完整模式；STV **8/8 通过**，另含 CON-GF-07 / 08 的真实消费分润及零倍率检查，退出 0 | `20261031-215833/p13.log`、`partner-qa-checks/p13.rc` |
| 全量报告合并 p11 / p13 | **601/602 通过，1 失败**；保留 TAUI-01 原始失败，不覆盖为成功 | `20261031-215833/report.md`、`results.jsonl`；`partner-qa-checks/full-with-p11-p13.rc=1` |
| 新脚本 `py_compile`、`node --check`、runner `bash -n`；`git diff --check` | 全部通过；无 pnpm 锁文件改动或新增 workspace 配置 | `partner-qa-checks/git-diff-check.log`，脚本本体及仓库状态 |

全量重点回归：PTN **111/111**、FIN **34/34**、STU **18/18**、CHAT **27/27**、FRC **14/14**、STV **8/8**、SD **19/19**；TAUI **14/15**。未因失败修改或跳过既有断言。

### 金额、冻结与界面验收

金额证据：fast / full 运行目录中的 `partner-money-check.json`、`partner-responses.json`、`partner-statement.csv`。SQL / Decimal 独立计算与接口金额差均小于 ¥0.01；各线、分组 / 模型成本拆分加总一致。

当月（2026-11）结果，以下金额单位为元：

| 指标 | PA | PB |
|---|---:|---:|
| 区域消耗 | 12 | 0.9 |
| 收入 | 9.4832 | 0.8712 |
| 上游成本 | 6 | 0.45 |
| 分润 | 0.393 | 0.099 |
| 专项支出 | 0.10 | 18 |
| 调整项 | +0.25 | 0 |
| 利润 | 3.2402 | −17.6778 |
| 合伙人分成 | **1.30** | **0.00** |

- 真实充值覆盖赠送优先、付费 FIFO、3.2% 通道费及部分退款；挂靠前后同一天的 X0 消耗按起点分摊，未把挂靠前收入 / 成本 / 分润算入区域。
- PA 的 2026-09：调整 −50，分成 0，期末亏损 50；2026-10：调整 +80，期初亏损 50，分成基数 30，分成 **12.00**，期末亏损 0。确认顺序、重复确认、已确认禁止调整、已付标记及重复标记均通过。
- 已确认 2026-10 后插入历史用量成本探针，冻结明细完整 JSON 不变，当月预估不变；探针随后删除，无残留历史 PA 汇总。探针 `total_cost=7`、`actual_cost=0`、账号倍率 0.5，用于产生历史成本，不额外制造余额核对差异。
- PA 本月仍为 40%，下月 **2026-12 起 35%**。PB 的 ¥30 / 30 天支出从 2026-10-20 起摊销，10 月 12 天、11 月 18 天；金额验收时 PB 于 11 月设立，结算单不出现 10 月。
- PTN-22 时其他合伙人分成 **0**，全站 `partner_share=1.30`，`partner_share_estimated=true`；`net_profit=profit−1.30`，区域支出全站只计一次。普通用户 / 跨区域访问拒绝、客户邮箱脱敏、合伙人响应不泄露上游账号字段及本轮账号名均通过。
- 金额核对完成后，UI 阶段才回填 PB 历史月份并创建额外 UI 合伙人 / 挂靠 UX，构造草稿、顺序受阻、可确认等按钮状态；不影响此前保存的独立金额证据。
- 截图目录：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-215833/partner_ui/`，共 **41 张 PNG**，含 qx / 默认主题、管理员 / 合伙人、逐层真实点击与移动端；同目录保存浏览器日志和下载 CSV。

### 发现的问题（按严重程度）

#### P2：新增合伙人导航位置与原有 TAUI-01 顺序验收冲突

- **影响**：完整回归无法全绿；不是「团队消费」页面无法访问。TAUI-01 的路由、标题、数据加载均通过，只有前三个导航项顺序不一致。
- **复现步骤**：
  1. 在本机环境使用管理员登录。
  2. 打开 `/admin/affiliates/agents`，展开「邀请返利」。
  3. 读取该分组前三个链接，再点击「团队消费」。
- **预期**：保持既有 `e2e/ui/admin_team_ui.cjs:109–116` 断言，前三项依次为 `/admin/affiliates/agents`、`/admin/affiliates/team`、`/admin/affiliates/invites`。
- **实际**：前三项为 `/admin/affiliates/agents`、`/admin/affiliates/partners`、`/admin/affiliates/team`。点击团队消费后 `path=/admin/affiliates/team`、`title=true`、`loaded=true`；单独复现页面有 20 行团队数据。
- **原因定位**：`frontend/src/themes/qx/shell/useQxNav.ts:201–203` 与 `frontend/src/components/layout/AppSidebar.vue:840–842` 均把「合伙人」插在「代理管理」之后。FE 完成报告已说明此排列；这是新导航安排与既有回归验收之间的冲突。
- **证据**：`20261031-215833/report.md` 顶部失败项、`run.log:2036–2038`；独立复现记录 `partner-nav-regression.json`（UTC `2026-10-31T22:12:56.918Z`，含完整菜单、路由与表格行数）及 `partner-nav-regression.png`。
- **下一处理方**：主代理 / FE 确认菜单位置与验收约定。按本轮「p0–p13 断言不改」要求，应由 FE 调整菜单位置后重新回归；QA 未修改产品或削弱旧断言。

本轮未发现其他结算金额、亏损结转、权限或隐私缺陷。全量验收仍因上述 P2 未通过，不标记全绿。

### 测试脚本修正与结束状态

- 开发测试脚本时修正了费率输入格式（`0.0320`）、SQL `"month"` 别名、历史奖励构造顺序、用户端任命接口路径；冻结探针改为零余额扣费，仅验证新增历史成本。
- UI 脚本补齐 SPA 导航等待、Esc 关闭延迟新手引导；自定义 Select 通过展开后真实点击选项验证，不读尚未打开的选项。以上均为测试脚本自身修正，不是产品补丁；最终验收使用清库后的 fast / full 轮次。
- 支付实例通道费率及测试临时设置已在 p14 收尾恢复；临时浏览器凭据配置已删除，默认主题 3300 服务已停止。
- 主应用 `/health` 返回 `{"status":"ok"}`，PostgreSQL / Redis 健康，Nova / Open WebUI / egress 在测试重启后均运行。宿主机测试服务端口仍仅绑定 `127.0.0.1`，未做全局 Docker 清理。
- 本轮到此结束；保留最终 full 运行目录及数据，供主代理 / FE 复现 TAUI-01。

### 复测（TAUI-01 菜单顺序）

- 时间：2026-11-01 06:57 北京时间（2026-10-31 22:57 UTC）。
- **最终结论：清库完整回归 578/578 通过，补跑 p11 / p13 后 602/602 通过，退出码均为 0。TAUI-01 通过，原 P2 导航顺序问题关闭；本复测结论替代前文的全量阻塞结论，初测记录保留。**

#### 修改范围

- 按主代理批准，仅修改 `/home/qlqq/workspace/sub2api-local/e2e/ui/admin_team_ui.cjs:115` 的 TAUI-01 `order` 期望为 `['/admin/affiliates/agents', '/admin/affiliates/team', '/admin/affiliates/partners']`。
- 原描述「团队消费排在代理管理之后」及路由、标题、加载状态断言不变；其他旧断言和测试脚本均未修改。产品侧两套主题的菜单调整由 FE 完成，QA 未改产品代码。
- 本轮前后测试源码 SHA-256 核对只有上述文件变化，逐行 diff 确认仅该期望数组改一项；证据在原目录 `retest/source-before-sha256.json`、`approved-assertion.diff`、`final-verification.json`。

#### 命令与执行过程

以下 `runs/` 均相对于 `/home/qlqq/workspace/sub2api-local/e2e/`。

| 命令 / 阶段 | 结果 | 证据 |
|---|---|---|
| `cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d` | 完整前后端及镜像构建、重启通过，退出 0 | `runs/20261031-215833/retest/build.log`、`build.rc` |
| `E2E_RUN_DIR=$PWD/runs/20261031-215833 python3 -u p6_team.py ui`（在 e2e 目录） | TAUI 15/15 通过，含修复后的 TAUI-01；TUI 有 4 条因旧夹具变化失败。脚本本身退出 0，但结果记录保留失败，不作为验收通过 | `runs/20261031-215833/retest/p6_team-ui.log`、`p6_team-ui.rc`、同轮 `run.log` |
| 同一原目录 `E2E_RUN_DIR` 执行 `python3 -u p14_partner.py ui` | 既有 UI 合伙人、挂靠及已确认结算单使创建 / 挂靠 / 确认场景不可重复，5 条 UI 检查失败，退出 1；两套主题入口检查通过 | `runs/20261031-215833/retest/p14_partner-ui.log`、`p14_partner-ui.rc`、`partner_ui/browser.log` |
| `python3 report.py runs/20261031-215833 > runs/20261031-215833/report.md` | 已重新生成，602 条中 593 通过、9 失败，退出 1；如实保留原目录重复执行的夹具状态问题 | 该目录 `report.md`、`retest/report.rc`；复测前原报告 / 原结果备份为 `retest/report.before.md`、`results.before.jsonl` |
| `cd ~/workspace/sub2api-local && ./reset.sh`，随后 `cd e2e && ./run_all.sh` | 按用户批准的兜底流程清库再全量；**578/578 通过，退出 0**，未修改其他断言 | `runs/20261031-215833/retest/reset-full.log`、`full-console.log`、`full.rc`；新轮 `runs/20261031-224011/run.log` |
| `E2E_RUN_DIR=$PWD/runs/20261031-224011 python3 -u p11_finance_recalculation.py` | FRC **14/14 通过**，退出 0 | 新轮 `p11_finance_recalculation.log`；原目录 `retest/p11_finance_recalculation.rc` |
| 同一新轮 `E2E_RUN_DIR` 执行 `python3 -u p13_studio_video.py` | Nova 运行中，完整模式 STV **8/8 通过**，CON-GF-07 / 08 分润及零倍率检查通过，退出 0 | 新轮 `p13_studio_video.log`；原目录 `retest/p13_studio_video.rc` |
| `python3 report.py runs/20261031-224011 > runs/20261031-224011/report.md` | **602/602 通过，失败 0，退出 0**；报告按同用例最后结果汇总 | 新轮 `report.md`、`results.jsonl`；原目录 `retest/full-with-p11-p13.rc` |

原目录不能直接复用的原因已核实：p6 邀请人数已为 114 而非 112、分销代理页签为 6 而非 4，历史身份及图片请求也被后续阶段改变；p14 的 UI 用户已被设为合伙人、UX 已挂靠、PB 的历史单已确认。未改这些断言或删除失败记录，改用清库新轮恢复各阶段预期前置条件。

#### 最终结果与证据

- **最终验收报告**：`/home/qlqq/workspace/sub2api-local/e2e/runs/20261031-224011/report.md`。
- 新轮重点分组：PTN **111/111**、TUI **21/21**、TAUI **15/15**、FIN **34/34**、FRC **14/14**、STV **8/8**、STU **18/18**、CHAT **27/27**，无失败。
- TAUI-01 实际前三项与批准期望完全相同：代理管理 → 团队消费 → 合伙人，且 `path=/admin/affiliates/team`、`title=true`、`loaded=true`。预期与实际 JSON 见新轮 `run.log`、`results.jsonl` 及原目录 `retest/final-verification.json`。
- 截图：新轮 `team_ui/`、`admin_team_ui/`、`partner_ui/`；合伙人截图共 **41 张**。已人工查看 `partner_ui/34-qx-admin-entry.png` 和 `34-default-admin-entry.png`，两套主题均为「代理管理、团队消费、合伙人、邀请记录、返利记录、提取记录」的显示顺序（末项为既有界面名称），对应批准的导航路径顺序。
- `node --check e2e/ui/admin_team_ui.cjs`、`git diff --check` 通过；无 pnpm 锁文件 / workspace 配置变化。临时默认主题服务已关闭，3300 无监听；临时合伙人浏览器凭据配置已删除。
- 结束时本机主应用 `/health` 为 `{"status":"ok"}`，数据库 / Redis 健康，Nova / Open WebUI / egress 均运行；发布端口仍仅绑定 `127.0.0.1`。未提交、推送、部署或连接远端。本轮结束，保留新轮数据及证据。
