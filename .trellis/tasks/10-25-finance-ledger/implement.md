# 财务明细页 — 实施计划

## 顺序

1. **迁移与仓储**
   - [x] 迁移 254 / 255 / 256（§2），附迁移测试（参照 `group_model_rate_migration_test.go`）。
   - [x] 仓储：账号模型成本倍率、其他支出、财务取数（订单 / ledger / 兑换码 / 优惠码 / rollup / usage_logs 聚合）。
   - [x] 集成测试（testcontainers）覆盖取数 SQL。
2. **写路径：账号模型成本倍率**
   - [x] `account_model_cost.go` + 三处接入点；单测：未配置时快照不变、精确 / 通配命中、按上游模型匹配。
   - [x] 管理接口 + 账号编辑页「模型成本倍率」。
3. **通道费率**
   - [x] 支付实例字段 + 编辑表单；下单时快照到订单；退款时写 `refund_balance_deducted`。
4. **收入确认引擎**（纯函数，先写单测）
   - [x] 用例：赠送优先、先进先出、跨天、退款（扣回 / 不扣回）、管理员扣余额、分润转入再消耗、透支、未登记期初。
5. [x] **成本、分润、支出聚合与接口**（§5–§8），含 CSV 导出、审计日志。
6. [x] **前端页面** `/admin/finance`：总览、利润明细、充值与赠送、消耗与成本、分润、其他支出、用户对账、设置；i18n 中英文。
7. [x] **本机 e2e**：新增 `e2e/p10_finance.py`（FIN 用例写进 `CASES.md`），完整模式执行；界面抽查截图。

## 校验命令

```bash
cd backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run 'Finance|AccountModelCost'
go build ./...
cd ../frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d
cd e2e && ./run_all.sh          # p0–p10
```

## 评审节点

- 第 4 步引擎单测通过后，先给用户看一份用本机数据算出的利润明细，确认口径再做页面。
- 全部通过后经用户同意再提交、推送、部署。

## 部署（一号机）

1. `docker compose pull && docker compose up -d`，迁移自动执行。
2. 后台：支付实例通道费率填 3.2%；财务设置固定成本账号选 1 号；2 号账号按需填模型成本倍率。
3. 一次性补算：usage_logs id 236（4 号、1 张）`account_stats_cost` 从 0.016 改为 0.05（SQL 在部署时给出，执行前确认）。
4. 在「其他支出」录入 vm2api 订阅账号历史购买记录。

## 回滚

- 回退镜像到上一个 `custom-<sha7>`；新表 / 新列留着无害。
- 账号模型成本倍率出问题时清空该账号配置即可恢复原成本计算。

## 后端交付记录

后端实现与测试、口径补充、完整文件清单见 [backend-report.md](./backend-report.md)。前端及本机 e2e 仍由各自负责人确认，不由后端勾选整项。

## 本机验证记录（2026-10-26，主会话）

- 完整 e2e `run_all.sh`（p0–p10，含 p8 / p9）：366 / 366 通过（runs/20261025-215635）；之后 cost_mode 改为按配置判断，p10 单独重跑两次 30 / 30。
- 界面：`e2e/ui/finance_ui.sh`（FINUI-01..05：支出弹窗默认 30 天、保存回显、模型成本倍率回显、支付实例通道费率），各标签页截图抽查。
- 后端 `go test -tags unit ./...` 全过；前端 lint / typecheck / 425 文件 3355 用例全过，锁文件无改动。
- 已知：账号列表的「N 个模型成本倍率」提示在「倍率」列里，该列上游默认隐藏，需在列设置里打开。

## 追加：关联支出与一次性扣除（2026-10-26）

- 后端：`breakdown?dim=account` 新增 `expense_cost`（关联该账号的支出在范围内的摊销额），毛利扣除；无用量账号也出行。
- 前端：支出表单「摊销方式」一次性扣除 / 按天摊销；按账号拆分新增「关联支出」列。
- 验证：p10 FIN-31 / FIN-32 新增并通过（p10 全部通过）；界面 FINUI-06（一次性隐藏天数、列表显示「一次性」）、FINUI-07（关联支出列）通过，FINUI-03 仍因「倍率」列默认隐藏而失败（已知问题，与本次无关）；前端 lint / typecheck / 3358 用例全过，后端 Finance 单测与 build 通过。
