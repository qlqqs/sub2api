# $ 与倍率显示点盘点（2026-10-22，子代理只读扫描）

I found everything you asked for. All paths are relative to `/home/qlqq/workspace/sub2api/frontend/src` unless they start with `backend/`. Some files repeat the same kind of display many times; for those I give one line listing every line number instead of one row per line.

## A) "$" / USD money displays in frontend (no tests or specs)

### Shared helpers (fixing these fixes everything that calls them)
| Location | What it does | Class |
|---|---|---|
| `utils/format.ts:61-62` | `formatCurrency(amount, 'USD')` uses Intl currency style; returns `'$0.00'` for null. Only called in `views/user/AffiliateView.vue` and `components/account/AccountTodayStatsCell.vue` | depends on caller |
| `utils/pricing.ts:23` | `formatScaled` returns `` `$${s}` `` | OFFICIAL_PRICE (also used for the Plaza paid price, see below) |
| `utils/usagePricing.ts:48` | `formatTokenPricePerMillion` returns `` `$${formatted}` `` | UNSURE (unit price = cost ÷ tokens) |
| `utils/affiliateRewards.ts:33` | `formatUsdShort` returns `` `${sign}$${…}` ``; `describeInviteeBonusValue` (:56-60) calls it | BALANCE |
| `components/payment/currency.ts:4` (map), `:29` | `currencySymbol('USD')` → `$`. Many files set `creditedAmountSymbol = currencySymbol('USD')` | BALANCE when used as `creditedAmountSymbol` |

### BALANCE (user-facing)
| Location | What is displayed |
|---|---|
| `components/layout/AppHeader.vue:376-378` (`formatHeaderMoney`), used at :78, :91, :95, :100, :150, :153, :290 | Header available / frozen / total balance |
| `components/user/dashboard/UserDashboardStats.vue:14` | Balance |
| same file :57, :58, :62, :63 | Today / total actual cost and standard cost |
| same file :161, :167 | Per-platform total / today cost |
| same file :205 | Platform quota used / limit |
| `components/user/dashboard/UserDashboardCharts.vue:51-52` | Per-model actual and standard cost |
| `components/user/dashboard/UserDashboardRecentUsage.vue:27-28` | Recent usage actual and total cost |
| `components/user/profile/ProfileInfoCard.vue:70` (helper :275-276) | Profile balance |
| `components/user/profile/ProfileBalanceNotifyCard.vue:29` (input prefix), `:36` (placeholder `$${systemDefaultThreshold}`) | Low-balance alert threshold |
| `components/common/SubscriptionProgressMini.vue:261` | Subscription `$used/$limit` |
| `views/user/KeysView.vue:216, 222` | Key today / total cost |
| same file :235 | Key quota used / limit |
| same file :265, :293, :321 | 5h / 1d / 7d usage vs limit |
| same file :717, :721, :784, :788, :830, :834, :876, :880 | Edit modal: used vs limit values |
| same file :698, :764, :810, :856 | `$` prefix on the quota and rate-limit inputs |
| `views/KeyUsageView.vue:836-838` (`usd()`), used at :338, :383, :621, :631, :648, :653, :699, :717, :744, :751, :758, :774, :800, :808 | Public key-usage page: cost, quota, limits, wallet balance |
| `views/user/RedeemView.vue:14, 102, 119, 440` | Balance, redeemed amount, new balance, history `±$` |
| `views/user/SubscriptionsView.vue:110, 147, 188` | Daily / weekly / monthly usage vs limit |
| `views/user/PaymentView.vue:106` | `+$` recharge bonus |
| same file :110 | Credited balance |
| same file :114, :700, :709 | First-recharge bonus (`formatUsdShort`) |
| same file :173, :177, :181 | Selected plan daily / weekly / monthly limit |
| `views/user/PaymentResultView.vue:60` | `+$` bonus |
| same file :64 | `'$' + order.amount` for balance orders |
| `views/user/UserOrdersView.vue:65` | Refund dialog: credited order amount |
| `components/payment/SubscriptionPlanCard.vue:58, 62, 66` | Plan daily / weekly / monthly limits |
| `components/payment/AmountInput.vue:146` | Quick-amount second line `'$' + quote.credited` |
| `creditedAmountSymbol` (credited order amount, bonus, refund):<br>`components/payment/PaymentStatusPanel.vue:26, 28`<br>`components/payment/PaymentQRDialog.vue:48`<br>`components/payment/StripePaymentInline.vue:26`<br>`components/payment/OrderTable.vue:22, 24` | Credited amount / bonus |
| `views/user/AffiliateView.vue:81, 99, 102, 105, 147, 328` (`formatCurrency`) | Affiliate quota, history quota, rebates, transfer toast |
| same file :281-292 (`formatUsdShort`) | Invite reward text |
| `views/auth/RegisterView.vue:579` | Invitee bonus hint |
| `views/user/BatchImageGuideView.vue:2387-2389` (`formatMoney`), used at :2397-2399 | Batch-image job cost / hold amount |
| Shared by user and admin usage pages:<br>`components/admin/usage/UsageStatsCards.vue:68, 77`<br>`components/admin/usage/UsageTable.vue:211, 416, 420, 424, 428, 480, 485, 489, 493, 507, 511`<br>`components/charts/ModelDistributionChart.vue:146, 152, 229, 454, 476`<br>`components/charts/GroupDistributionChart.vue:77, 83, 227`<br>`components/charts/EndpointDistributionChart.vue:110, 113, 272`<br>`components/charts/TokenUsageTrend.vue:158` | Usage actual / standard / partial costs |

### BALANCE (admin-only)
| Location | What is displayed |
|---|---|
| `views/admin/UsersView.vue:442` | User balance |
| `components/admin/user/UserBalanceModal.vue:6, 11, 16` | Current balance, amount-input `$`, new balance |
| `components/admin/user/UserBalanceHistoryModal.vue:34, 45, 324` | Balance, total recharged, history `±$` |
| `components/user/PlatformUsageBreakdown.vue:5, 15, 36, 38`<br>`components/user/PlatformCostCell.vue:5, 9` (both only used in admin UsersView) | Platform usage costs |
| `components/admin/announcements/AnnouncementReadStatusDialog.vue:38` | User balance |
| `views/admin/DashboardView.vue:117, 129, 153, 165, 583` | Actual and standard spend |
| `components/admin/usage/UserTokenRanking.vue:74` | Ranking actual cost |
| `components/charts/UserBreakdownSubTable.vue:26, 32` | Actual / standard cost |
| `views/admin/GroupsView.vue:214, 218, 232, 242, 256` (`formatUsd`, :5658-5659) | Subscription-group usage and limits |
| same file :343, :353, :363 | Group today / yesterday / total usage |
| `views/admin/SubscriptionsView.vue:264, 266, 301, 303, 338, 340` | Usage vs group limits |
| `views/admin/orders/PlanEditDialog.vue:30-32` | Group daily / weekly / monthly limits |
| `views/admin/RedeemView.vue:133` | Balance code value |
| `views/admin/PromoCodesView.vue:79, 350` | Promo bonus amount, usage bonus |
| `views/admin/affiliates/AdminAffiliateRecordsTable.vue:197-201, 486` | Affiliate quota and rewards |
| `views/admin/affiliates/AdminAffiliateAgentsView.vue:117, 337` | Agent profit, contribution |
| `views/admin/affiliates/AffiliateOfflineWithdrawDialog.vue:71, 326, 327` | Available quota, withdraw result |
| `views/admin/SettingsView.vue:8968` | `$` prefix on the low-balance threshold input |
| same file :13279-13284 | Affiliate example amounts (`formatUsdShort`) |
| `creditedAmountSymbol`:<br>`components/admin/payment/AdminRefundDialog.vue:38, 46, 69, 73, 98, 110`<br>`components/admin/payment/AdminOrderDetail.vue:34, 38, 84`<br>`components/admin/payment/AdminOrderTable.vue:61, 63`<br>`views/admin/orders/AdminOrdersView.vue:38, 69, 76` | Credited amount, refund, user balance |
| `views/admin/GroupsView.vue:5442-5461`, shown via :983, :1166, :1471, :2623, :2806, :3111 | Final per-image / video / web-search price after the group multiplier. This is UNSURE: it is the amount deducted from balance, but it comes out of the price editor |

### OFFICIAL_PRICE
| Location | What is displayed |
|---|---|
| `components/modelPlaza/PlazaModelPricingTable.vue:222-275` (`official()`, :455-457) | Official reference price per 1M tokens |
| same file :124-177 and :197-203 (`paidPerMillion` / `paidRequestPrice`, :426-451) | **UNSURE.** "Paid price" = channel price × effective rate. In practice this is a balance amount but it uses the same `$` formatter |
| `components/channels/SupportedModelChip.vue:261-268`<br>`components/channels/PricingRow.vue:23` | User Available Channels: channel unit prices |
| `components/admin/channel/IntervalRow.vue:18, 23, 28, 33, 38, 87` | `$/M` and `$` labels |
| `components/admin/channel/PricingEntryCard.vue:107, 196, 232` | `$/MTok` and `$` labels |
| `views/admin/GroupsView.vue:941, 952, 963, 2581, 2592, 2603` | `1K/2K/4K ($)` image price inputs |
| same file :1085, 1096, 1107, 1143, 2725, 2736, 2747, 2783 | `($/s)` video price inputs |
| `components/admin/usage/UsageTable.vue:434-446` (`formatTokenPricePerMillion`), :476 (image unit price) | **UNSURE.** Derived unit price |

### REAL_CURRENCY
| Location | What is displayed |
|---|---|
| `paymentAmountSymbol` / `formatPaymentAmount` (pay amount, fee, base in the order currency):<br>`AdminRefundDialog.vue:42`<br>`AdminOrderDetail.vue:22, 26, 30`<br>`AdminOrderTable.vue:56`<br>`OrderTable.vue:17`<br>`PaymentQRDialog.vue:52`<br>`StripePaymentInline.vue:30, 43`<br>`AdminOrdersView.vue:70`<br>`PaymentStatusPanel.vue:34`<br>`PaymentResultView.vue:48, 52, 56, 85`<br>`StripePaymentView.vue:20`<br>`PaymentView.vue` `formatSelectedPaymentAmount` :98, :102<br>`AmountInput.vue:144`<br>`PlanEditDialog.vue:168, 170` | Paid / fee / base amounts |
| `components/payment/SubscriptionPlanCard.vue:29, 39` | Plan price and original price. Falls back to `'USD'` when `plan.currency` is empty (:158) |
| `views/admin/orders/AdminPaymentPlansView.vue:101` | Same `'USD'` fallback |
| `components/account/CNProviderBalanceCell.vue:120` | Upstream provider balance. Shows the currency code or `¥`, never `$` |
| `components/account/AccountUsageCell.vue:411` | Grok upstream prepaid balance |
| **`components/payment/AmountInput.vue:58`** | **UNSURE / likely mislabeled.** The `$` prefix sits on the custom amount input, but that amount is in the *payment currency* (credited = amount × multiplier) |

### UNSURE: admin upstream-account costs
These are account cost (A) / user billed (U) for upstream accounts, measured in official-price USD, and probably should stay `$`.
- `components/account/AccountStatsModal.vue:72, 77, 78, 123, 132, 198, 204, 253, 259, 308, 314, 418, 424, 553, 562, 611, 644, 648`
- `components/admin/account/AccountStatsModal.vue:60, 65, 66, 110, 119, 173, 179, 223, 229, 277, 283, 388, 517, 526, 575, 608, 612`
- `components/account/AccountUsageCell.vue:538, 545, 626, 633`
- `components/account/UsageProgressBar.vue:16, 23`
- `components/account/QuotaBadge.vue:45, 47`
- `components/account/AccountCapacityCell.vue:11`
- `components/account/AccountTodayStatsCell.vue:39, 46`
- `components/account/CreateAccountModal.vue:2889, 2904`
- `components/account/EditAccountModal.vue:2764, 2779`
- `components/account/QuotaDimensionRow.vue:81`
- `components/account/QuotaNotifyToggle.vue:49`
- Account-cost columns in shared components (all behind `showAccountCost` / `showAccountBilling`):
  - `UsageTable.vue:230, 522`
  - `UsageStatsCards.vue:72`
  - `ModelDistributionChart.vue:149`
  - `GroupDistributionChart.vue:80`
  - `UserBreakdownSubTable.vue:29`
  - `DashboardView.vue:123, 159`

### i18n strings (zh line / en line)
Keys from files under `admin/` get an `admin.` prefix (`zh/admin/index.ts` spreads them into `admin`).

**BALANCE**
- `keys.quotaAmount` "额度金额 (USD)" — zh/dashboard.ts:296, en:291
- `keys.quotaAmountPlaceholder` — zh:297, en:292
- `keys.resetQuotaConfirmMessage` "…（${used}）…" — zh:303, en:298
- `keys.rateLimit5h` / `rateLimit1d` / `rateLimit7d` "(USD)" — zh:309-311, en:304-306
- `auth.promoCodeValid` "获得 ${amount} 赠送余额" — zh/common.ts:290, en:291
- `payment.rechargeRatePreview` "1 {currency} = {usd} USD" — zh/misc.ts:436, en:412
- `payment.admin.insufficientBalance` "将扣至 $0" — zh:572, en:548
- `admin.dashboard.actualSpending` "实际消费 ($)" — zh/admin/overview.ts:53, en:53
- `admin.affiliates.withdraw.amount` "（USD）" — zh:480, en:480
- `admin.users.platformQuota.columns.daily` / `weekly` / `monthly` — zh:929-931, en:932-934
- `admin.users.platformQuota.subtitle` — en:929 only
- `admin.groups.subscription.dailyLimit` / `weeklyLimit` / `monthlyLimit` — zh:1182-1184, en:1185-1187
- `admin.redeem.amount` "金额 ($)" — zh/admin/resources.ts:300, en:296
- `admin.redeem.form.balanceHint` "余额金额（美元）" — zh:338, en:347
- `admin.promo.bonusAmount` "赠送金额 ($)" — zh:473, en:476
- `admin.settings.features.affiliate.perInviteeCapDesc` — zh/admin/settings.ts:111, en:111
- `admin.settings.features.affiliate.inviteReward.perInviteeCapHint` — zh:159, en:159
- `admin.settings.platformQuota.daily` / `weekly` / `monthly` — zh:476-478, en:481-483
- `admin.settings.payment.balanceRechargeMultiplierHint` "每支付 1 CNY 可获得多少 USD 余额" — zh:785, en:790
- `admin.settings.payment.balanceRechargePreview` "1 CNY = {usd} USD" — zh:786, en:791
- `admin.settings.payment.rechargeBonus.noticePlaceholder` "满 $100…" — zh:822, en:827

**OFFICIAL_PRICE**
- `modelPlaza.table.unitPerMillion` "$ / 1M token" — zh/dashboard.ts:706, en:702. Used for both the paid and the official column.
- `admin.groups.videoPricing` description ($0.05/s…) — zh/admin/overview.ts:1208, en:1211
- `admin.groups.explicitPricing.description` / `searchPricePer1k` — zh:1220-1221, en:1223-1224
- `admin.groups.voicePricing.*` — zh:1233-1236, en:1236-1239
- `admin.groups.webSearchPricing.pricePerCall` and its hint "$0.01/次（官方 $10/1000 次）" — zh:1241, 1243; en:1244, 1246

**REAL_CURRENCY**
- `admin.settings.payment` subscription USD→CNY rate hint — zh:789, en:792/794
- `admin.settings.payment.field_paymentCurrencyHint` — zh:898, en:903
- `payment.admin.currencyPlaceholder` — zh/misc.ts:601, en:577
- `admin.accounts.usageWindow.grokUsed` / `grokBalance` / `grokMonthlyLimit` / `grokOverageShort` — zh/admin/accounts.ts:527, 528, 530, 532; en:1664, 1665, 1667, 1669

**UNSURE (admin, account quota or estimate)**
- `admin.accounts.quotaLimitHint` "（美元）" — zh:263, en:470
- `admin.accounts.usageWindow.estimatedTotalCost` "${cost}" — zh:545, en:1682

**Not money:** `headerOverride.invalidName`, `useKeyModal.grok` shell `$` strings, and the HTML in `onboarding.admin.groupMultiplier` (zh/misc.ts:186).

## B) Rate-multiplier displays

**Shared components**
- **`components/common/GroupBadge.vue`**
  - Props: `rateMultiplier`, `userRateMultiplier`, `peakRateMultiplier`, `alwaysShowRate`.
  - :16-17 shows the group rate struck through plus the user's own rate (`{{x}}x`).
  - :116 builds the label `` `${rateMultiplier}x` ``.
  - :90 shows the peak window via `formatPeakRateWindow` → `"HH:MM-HH:MM ×N (UTC±)"` (`utils/peak-rate.ts:31`).
- **`components/common/GroupOptionItem.vue`**: rate pill at :31-32 (struck-through + user rate) and :35 (`{{rate}}x 倍率`); peak window at :111.
- **`components/modelPlaza/PlazaModelPricingTable.vue`**
  - `effectiveRate = userRateMultiplier ?? rateMultiplier` (:383).
  - Rate column :289 (period rate `x`), :294 (`requestRate` image/video `x`), :297-298 (struck-through + effective), :300.
  - Header label `modelPlaza.table.rate` at :43.
- `components/modelPlaza/PlazaFilterBar.vue:77`: `{{ r }}x` filter chips. Rates come from `ModelPlazaContent.vue:103` (`user_rate_multiplier ?? rate_multiplier`).
- `utils/peak-rate.ts:31`: `×{peak_rate_multiplier}`.

**User-facing**
| Location | Data source |
|---|---|
| `views/user/KeysView.vue:165-175` (GroupBadge), :551-575 (selector), :1174-1183 (GroupOptionItem) | `group.rate_multiplier` + `userGroupRates[id]` from `/groups/rates` (:1651); groups from `/groups/available` (:1643) |
| `components/channels/AvailableChannelsTable.vue:98-103, 132-137, 236-242, 271-277` (GroupBadge always-show-rate); peak at :373 | `g.rate_multiplier` + `userGroupRates` prop, loaded in `views/user/AvailableChannelsView.vue:112` |
| `components/modelPlaza/PlazaGroupSection.vue:9-19` (badge), :56-67 (pricing table: rate, user, image, video, peak) | `/model-plaza` group: `rate_multiplier`, `user_rate_multiplier`, `image_rate_multiplier`, `video_rate_multiplier`, `peak_rate_multiplier` |
| `components/payment/SubscriptionPlanCard.vue:48` (`rateDisplay` `×N`, :152-155), peak :163 | `plan.rate_multiplier` from checkout-info. Group rate only, no user rate |
| `views/user/PaymentView.vue:162` (`×{{selectedPlan.rate_multiplier}}`), :243 (`×{{sub.group.rate_multiplier}}`), peak :362, :852 | plan / subscription group |
| `views/user/SubscriptionsView.vue:53` (`×`), peak :291 | `subscription.group.rate_multiplier` |
| `components/admin/usage/UsageTable.vue:503` (shared with user UsageView) | `log.rate_multiplier` per request. Line :517 is the account multiplier, admin only |

**Admin-only**
| Location | What it shows |
|---|---|
| `views/admin/GroupsView.vue:264-266` | `{{value}}x` column |
| same file :619, :2258, :931, :2571, :1075, :2715, :1210, :2850 | Rate / image / video / peak multiplier inputs |
| `components/admin/group/GroupRateMultipliersModal.vue:14, 178` | Group rate and per-user final rate |
| `components/admin/user/UserAllowedGroupsModal.vue:73, 153` | `defaultRate x` + custom-rate input; source `AdminUser.group_rates` |
| `views/admin/ChannelsView.vue:316` | Group rate |
| `components/common/GroupSelector.vue:31, 44` | `admin.groups.rateAndAccounts` "{rate}x 费率"; used in account / announcement editors |
| `components/account/AccountGroupsCell.vue:11, 63` | Group rate |
| `components/admin/user/UserApiKeysModal.vue:34-38, 94-98` | Group rate |
| `components/admin/monitor/MonitorKeyPickerDialog.vue:55-56` | Group rate + admin's own rates via `getUserGroupRates`, `MonitorFormDialog.vue:796` |
| `views/admin/SubscriptionsView.vue:242, 592, 601` | Group rate |
| `views/admin/RedeemView.vue:329, 340` | Group rate |
| `views/admin/UsersView.vue:421` | Group rate |
| `views/admin/orders/AdminPaymentPlansView.vue:26` | Group rate |
| `views/admin/orders/PlanEditDialog.vue:27, 139` (`(${rate}x)`) | Group rate |
| `views/admin/SettingsView.vue:3963, 3990, 4293, 4322` | Group rate |
| `components/account/UpstreamBillingRateCell.vue:15-25, 195` | i18n `admin.accounts.upstreamBilling.groupRate`, `userRate`, `peakRate`, `effectiveRate` "{value}x" (zh/admin/accounts.ts:197-204) |

Other rate i18n strings: `admin.groups.rateAndAccounts` (zh/admin/overview.ts:1115); `modelPlaza.peakNote` / `timePricingRateHint` (zh/dashboard.ts:680, 702).

`views/admin/AccountsView.vue:352` (`rate_multiplier x`) is the **account** billing multiplier, not a group rate.

## C) Backend user-facing "$" outputs (no comments)
- `backend/internal/service/notification_email_service.go`:
  - :1262 and :1271: balance-low template `${{current_balance}}`, `${{threshold}}` (en / zh)
  - :1282-1283 and :1290-1291: recharge-success template `${{recharge_amount}}`, `${{current_balance}}`
  - Values come from `balance_notify_service.go:368-369` and `payment_fulfillment.go:519, 530`
- `backend/internal/service/balance_notify_service.go`:
  - :483, :485, :486: built-in balance-low email `$%.2f` (user)
  - :401 (`thresholdDisplay "$%.2f"`), :525, :527, :550: account quota-alert email. Admin-facing; this is upstream account quota
- `backend/internal/handler/gateway_handler.go:1720, 1723, 1806, 1851`: `"unit": "USD"` in the key `/v1/usage` JSON. `frontend/src/utils/ccswitchImport.ts:39` reads it
- `backend/internal/service/cline_usage.go:325`: `"balance %.4g USD below threshold"`. Upstream account reason (REAL_CURRENCY), admin-only

## Extra answers
1. **`balance_recharge_multiplier`**
   - Served by `GET /api/v1/payment/checkout-info` (`backend/internal/server/routes/payment.go:47`; `backend/internal/handler/payment_handler.go:149, 169`).
   - Typed in `types/payment.ts:76`, called via `api/payment.ts:38-39`, read in `PaymentView.vue:549, 587-590`.
   - Checkout plans carry `rate_multiplier` and peak fields for **the plan's group only** (`payment_handler.go:129-131`), with no user-specific rate.
   - **`PaymentView.vue` does not load the user's available groups or rates.** It imports `paymentAPI`, `userAPI` and the subscription store, but not `userGroupsAPI`. To show them it would need:
     - `userGroupsAPI.getAvailable()` → `GET /groups/available` (`api/groups.ts:16`; `backend/internal/handler/api_key_handler.go:322`). Returns `Group[]` with `rate_multiplier`, `peak_*`, `image_/video_rate_multiplier`.
     - `userGroupsAPI.getUserGroupRates()` → `GET /groups/rates` (`api/groups.ts:25`; `api_key_handler.go:344`). Returns `Record<groupId, rate>`.
   - Alternatively, `/model-plaza` (`api/modelPlaza.ts:91`) returns groups that already include `user_rate_multiplier`.
2. **How the frontend gets a user's own (exclusive) rate:**
   - `/groups/rates` map, stored as `userGroupRates[group.id]` (KeysView :1381, :1651; AvailableChannelsView :67, :112).
   - `ModelPlazaGroup.user_rate_multiplier` (`api/modelPlaza.ts:67`; `backend/internal/handler/model_plaza_handler.go:85`).
   - `UpstreamBillingData.user_rate_multiplier` (`types/index.ts:1150`; `backend/internal/handler/gateway_key_billing.go:21`).
   - Admin side: `AdminUser.group_rates` (`types/index.ts:109`).
   - Components receive it as the prop `user-rate-multiplier` / `userRateMultiplier`.
