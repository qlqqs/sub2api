# UI 与实现约定

清单见 `research/inventory.md`（逐文件逐行）。本文件给出归类裁决与统一写法。

## 1. 统一工具（唯一定义处）
新建 `frontend/src/utils/balanceCurrency.ts`（CUSTOM），导出：
- `BALANCE_SYMBOL = '¥'`：站内余额单位符号。
- `formatBalance(amount, opts?)`：`¥` + 数字，负数为 `-¥1.23`，支持与原各处一致的小数位/千分位选项；null → `¥0.00`（或调用方原来的占位）。
- `OFFICIAL_SYMBOL = '$'`：官方用量/官方单价符号（保持 `$`）。
- `formatRateRatio(rate)`：返回 `¥{x} = $1`，x 去掉多余尾零，最多 4 位小数（0.5 → `¥0.5 = $1`，1 → `¥1 = $1`，0.125 → `¥0.125 = $1`）；rate 为 null/非正数时返回 `-`。
- `effectiveRate(groupRate, userRate)`：`userRate ?? groupRate`。
各页面不得再写死 `$` / `¥` 表示余额单位，一律调用上述工具。`components/payment/currency.ts` 中所有 `creditedAmountSymbol = currencySymbol('USD')` 改为 `BALANCE_SYMBOL`。

## 2. 归类裁决
| 类别 | 显示 | 说明 |
|---|---|---|
| BALANCE（余额、到账、扣费 actual_cost、Key/订阅/分组/平台限额、兑换码、优惠码、返利、提现、余额提醒阈值、余额流水、排行中的实际消费） | ¥ | 用户端与管理端一致 |
| 标准费用 total_cost / standard cost | $，文案称「官方用量」或「官方价」 | 仪表盘、图表、使用记录中的标准费用列 |
| 模型单价（官方价、渠道单价、后台价格编辑器的输入与单位 `$/M`、`$/MTok`、`($/s)`） | $ | 管理员录入的是官方美元价 |
| 模型广场「本站价」= 单价 × 生效倍率 | ¥ | 表头单位 `¥ / 1M token`；官方价列表头 `$ / 1M token` |
| 使用记录推导单价（cost ÷ tokens，`formatTokenPricePerMillion`） | ¥ | 来自实际扣费 |
| 后台分组「乘倍率后最终单价」（GroupsView 5442-5461 展示值） | ¥ | 即实际扣费；输入框仍是 $ |
| 上游账号成本（AccountStatsModal 等中的 account cost / A） | $ | 上游官方价口径；同组件里的「用户计费 / U」改 ¥ |
| 上游账号配额、配额告警邮件、Grok/DeepSeek 等上游余额 | 保持原样 | 真实/上游币种 |
| 支付订单实付金额、手续费、套餐售价 | 保持订单币种 | REAL_CURRENCY |
| `AmountInput.vue:58` 自定义金额输入前缀 | 改为支付币种符号（`currencySymbol(支付币种)`） | 修正原误标 `$` |
| `/v1/usage` JSON 的 `"unit": "USD"` | 不改 | 机器可读字段，第三方工具依赖 |

## 3. 比例「¥x = $1」显示位置
| 位置 | 规则 |
|---|---|
| `GroupBadge.vue` / `GroupOptionItem.vue` | 原 `{rate}x` 改为 `formatRateRatio(effective)`；有专属倍率时原样保留「划线分组值 + 专属值」结构，两边都用比例；`title` 悬停显示原倍率（如「倍率 0.5x」） |
| 模型广场 `PlazaModelPricingTable` 倍率列、`PlazaFilterBar` 倍率筛选 | 比例；表头「倍率」改「价格比例」 |
| 时段（高峰）倍率 `utils/peak-rate.ts` | `×N` 改为该时段的比例 `¥N = $1` |
| 生图/视频独立倍率 | 同上，用各自的生效倍率 |
| 订阅套餐卡片、充值页套餐/订阅、用户订阅页 | `×N` → `¥N = $1` |
| 使用记录每条请求的倍率列（UsageTable:503） | `¥x = $1`；账号倍率列（:517，管理端）保持 `x` |
| 管理端倍率输入框、分组列表倍率列、用户专属倍率编辑、上游计费倍率单元格 | 保持 `x`（管理员配置的是倍率本身） |

## 4. 充值页（PaymentView）
- 金额区域：「充 ¥100 → 余额 ¥100」（到账用 `formatBalance`）。原「当前倍率：1 CNY = x USD」删除（充值倍率固定 1）。
- 新增「各分组价格」小卡片：调用 `userGroupsAPI.getAvailable()` + `getUserGroupRates()`，每行「分组名　¥x = $1」，按比例升序；加载失败时整块隐藏，不影响充值。
- 首充奖励、充值赠送等金额改 ¥。

## 5. 仪表盘与使用记录
- 用户仪表盘消费卡：主值「¥{actual}」，副值「官方用量 ${total}」（替代原「实际 / 标准」两个 $）。
- 管理端仪表盘与图表同理：实际消费 ¥，标准费用 $。
- 使用记录：扣费列 ¥；悬停提示「官方价 ${total_cost}」。

## 6. 文案（zh / en 同步）
- 「金额 (USD)」「(USD)」「($)」「（美元）」等余额语境 → `（¥）`/`(¥)`；「每支付 1 CNY 可获得多少 USD 余额」→「每支付 1 元到账多少余额」；预览「1 CNY = {usd} USD」→「1 元 = ¥{usd} 余额」。
- 模型单价语境的 `$` 文案不改。
- 英文界面同样用 `¥`/`¥`（站内计价单位与语言无关）。

## 7. 后端邮件
- `notification_email_service.go` 余额不足与充值成功模板（中英）中的 `$` → `¥`。
- `balance_notify_service.go` 用户余额提醒内置邮件（:483-486）`$%.2f` → `¥%.2f`；账号配额告警（:401、:525-550，管理员、上游口径）不改。
- 相关后端测试若断言 `$` 需同步更新。

## 8. 测试
- 新增 `utils/__tests__/balanceCurrency.spec.ts`：formatBalance、formatRateRatio（含尾零、null、负数、专属倍率优先）。
- 更新断言 `$` 的既有 spec；GroupBadge / PlazaModelPricingTable / PaymentView 至少各有一条比例显示断言。

## 9. 实现中的调整（已落地）
- 符号统一为半角 `¥`（余额与比例一致，也与现有 CNY 订单金额的 `¥` 一致）。
- 免费分组（倍率 0）显示 `¥0 = $1`；null/负数显示 `-`。
- 高峰时段比例 = 生效倍率 × 时段倍率（与后端计费一致），不是只显示时段倍率。
- 使用记录悬停明细里的输入/输出/缓存分项费用与推导单价来自未乘倍率的标准费用，保持 `$`；只有实际扣费、用户计费为 `¥`。
- 充值页「各分组价格」不含订阅分组（订阅不扣余额）；到账行「充 ¥X → 余额 ¥X」常显。
- `GroupBadge` 是共享组件，管理端用到它的地方同样显示比例；管理端倍率输入、分组列表倍率列、`GroupSelector` 仍显示 `x`。
- 保持 `$`：标准费用、上游账号成本、单价输入与渠道单价、使用记录 CSV 导出、`/v1/usage` 的 `"unit": "USD"`。
