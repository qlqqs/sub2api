# FE-S：sub2api 前端任务书（claude opus-5.5 high）

工作目录：`/home/qlqq/workspace/sub2api/frontend`。

开始前按 `prd.md` 的「共同规则」读取文档，并读取 `.trellis/spec/` 中前端相关的规范。接口以 `contract.md` 的 C1 为准。

工作区里已有未提交的前端改动，要保留，不要回滚：模型广场和 qx 主题的视频价格单位「/ 秒」，涉及

- `PlazaModelPricingTable.vue`
- `i18n/locales/{zh,en}/dashboard.ts`
- `themes/qx/QxPricing.vue`
- `themes/qx/locales/{zh,en}.ts`

后端字段由 BE-S 并行实现。开发时先按约定写类型和界面；单测里 mock 接口即可，不需要等后端完成。

## 1. 返利设置：分组分润系数

位置：`src/views/admin/SettingsView.vue` 的返利设置区块（`data-test="affiliate-rules"`）。在「1. 代理利润」这一节里，紧挨着利润池和二级比例，新增「分组分润系数」。

仅在 `affiliate_rebate_base === 'consumption'` 时显示，因为充值基数模式下不生效。

交互：

- 一个小表格，每行包含：
  - 分组选择，用现有的分组下拉组件或 API，看 SettingsView 里其他选择分组的地方怎么写；
  - 系数 %：数字输入，0–100，步长 1，允许小数；
  - 换算预览；
  - 删除按钮。
- 底部有「添加分组」按钮。已经选过的分组不能重复选择。
- 换算预览（只读文字）：

  > 一级代理 X% · 二级代理 Y% · 一级差额 Z%

  按当前表单的利润池比例 P 和二级比例 R2 计算：
  - X = P×f
  - Y = R2×f
  - Z = (P−R2)×f
  - 保留 2 位小数。
  - 例如 P=11、R2=7、f=30 时，显示 3.3% / 2.1% / 1.2%。
- 说明文字（一行即可）：

  > 该分组的消费先按系数折算再分润；未设置的分组按 100%，0 表示不分润。只影响之后的结算。

- 保存时随设置一起 PUT。前端也做同样的校验（越界、重复、未选分组），校验不通过时不能保存，并就地提示。
- factor=100 的行，保存时由后端去掉，前端不需要特殊处理。
- 分组已被删除（`group_name` 为空）的行，显示「已删除分组 #id」，允许删除这一行。

修改的文件：

- `src/api/admin/settings.ts`：加类型。
- `SettingsView.vue`：
  - 加 form 字段、默认值 `[]`；
  - 加载和保存的映射，参考 `affiliate_settle_interval_minutes` 的写法。
- i18n 的 zh 和 en。

单测：在 `src/views/admin/__tests__/SettingsView.spec.ts` 补充：

- 显示和隐藏条件；
- 添加、删除行；
- 预览计算；
- 保存的 payload；
- 校验阻止保存。

## 2. 团队请求明细标签

- 找到代理团队页的请求列表：
  - 用户端：`/affiliate` 团队客户的请求明细；
  - 管理端：`AdminAffiliateTeamViewerView.vue` 中对应的列表。
  - 两边大概率共用一个组件，grep `requests` 或 `TeamRequest` 确认。
- 每行增加 `rebate_factor`。当它不等于 100 时，在「分组」那一列的分组名旁边显示小标签「分润 X%」：
  - 样式参考现有的小 badge；
  - X 去掉多余的 0，例如 30 显示为 30%，12.5 显示为 12.5%。
- 补充类型、i18n 和单测。

## 3. 渠道定价里视频价格的币种标签（检查，按需修改）

- `PricingEntryCard.vue` 里视频按秒价格的标签是「$」。
- 检查整个渠道定价界面（token、按次、图片价格）现在用的币种符号：
  - 如果全部都是「$」，就不要改，保持一致，在完成报告里说明。
  - 如果只有视频这一处与其他地方不一致，就改成一致。
- 本站余额单位是 ¥，美元汇率只用于展示。不要引入新的币种逻辑。

## 4. 自检

```bash
cd frontend
npx -y pnpm@9 run lint:check && npx -y pnpm@9 run typecheck && npx -y pnpm@9 run test:run
git status   # 确认 pnpm-lock.yaml 没有变化
```

- 如果 `node_modules` 有问题，先运行 `CI=true npx -y pnpm@9 install --frozen-lockfile`。
- 界面截图：
  - 等 BE-S 完成后，由主代理通知你，或者你看 BE-S 任务书的完成报告。
  - 然后运行 `cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d`。
  - 再用 `e2e/ui/shots.sh` 截图：
    - 管理端设置页的返利区块，设置一个系数 30 的分组；
    - 团队请求明细带标签的样子。
  - 截图放在 `/tmp/seedance-fe-s/`。
- 注意：`./build.sh` 会编译当前工作区的前后端。BE-S 还没完成时，可以只跑单测，截图放到最后做。

## 完成报告

（在此追加）

### FE-S 完成报告（2026-10-31）

**截图待后端完成**：写报告时 `task-be-sub2api.md` 还没有完成报告，所以没跑 `./build.sh`，也没截图（`/tmp/seedance-fe-s/` 未生成）。后端就绪后，按第 4 节运行 `cd ~/workspace/sub2api-local && ./build.sh && docker compose up -d`，再用 `e2e/ui/shots.sh` 截这两张图：`/admin/settings` 的返利区块，以及带「分润 30%」标签的团队客户请求明细。
已核对 BE-S 进行中的代码，字段名和本前端一致：`affiliate_group_rebate_factors`、`group_name`、`rebate_factor`。

#### 改动文件

1. 分组分润系数（C1）
   - `src/api/admin/settings.ts`：新增 `AffiliateGroupRebateFactor { group_id, factor, group_name? }`；`SystemSettings` 和 `UpdateSettingsRequest` 增加 `affiliate_group_rebate_factors`。
   - `src/views/admin/SettingsView.vue`：
     - 在「代理利润」一节中，利润池和分销代理比例下方新增「分组分润系数」（`data-test="affiliate-group-factors"`），只在 `affiliate_rebate_base === 'consumption'` 时显示。
     - 每行有分组 `Select`、系数 % 输入（0–100，step 1，可填小数）和删除按钮；行下方显示换算预览或就地错误。
     - 分组列表复用页面已有的 `adminAPI.groups.getAll()` 调用，结果另存为 `affiliateFactorGroups`（不过滤订阅类型）。各行可选项会排除其他行已选的分组；没有可选分组时，「添加分组」按钮置灰。
     - 新增行的默认值是 `{group_id: 0, factor: 100}`，表示「未选分组」。
     - 行内分组不在当前分组列表里（已删除或已停用）时，改为显示静态文字：有 `group_name` 显示名称，否则显示「已删除分组 #id」。这一行可以删除，系数仍可编辑。
     - 校验：未选分组、分组重复、系数为空或超出 0–100，都在对应行就地提示（有错误时替换预览文字），并接入 `affiliateValidationError`，阻止保存并弹出 toast。
     - 保存时随设置一起 PUT `affiliate_group_rebate_factors: [{group_id, factor}]`，不发送 `group_name`。加载沿用通用的 settings 映射，默认值为 `[]`。
     - 换算预览用当前表单的 P 和 R2 计算，经 `formatRatePercent` 保留 2 位小数并去掉末尾 0。例如 P=11、R2=7、f=30 时显示「渠道经理 3.3% · 分销代理 2.1% · 渠道经理差额 1.2%」。
     - **用词说明**：任务书里写的是「一级代理 / 二级代理 / 一级差额」，界面上的现有术语是「渠道经理 / 分销代理」，所以预览改用现有术语，含义不变。
   - i18n：`i18n/locales/{zh,en}/admin/settings.ts` 新增 `admin.settings.features.affiliate.groupFactors.*`。

2. 团队请求明细标签
   - 用户端 `/affiliate/team/customers/:id` 和管理端查看器共用 `components/user/affiliate-team/TeamRequestTable.vue`。
   - `src/types/index.ts`：`AffiliateTeamRequest` 增加 `rebate_factor: number`。
   - `TeamRequestTable.vue`：「分组」列在分组名旁边加了 `TeamBadge`「分润 X%」，只在 `rebate_factor !== 100` 时显示；X 用 `formatRatePercent` 去掉末尾 0（30 → 30，12.5 → 12.5）。
   - i18n：`i18n/locales/{zh,en}/dashboard.ts` 新增 `affiliate.team.rebateFactor`（「分润 {rate}%」/「Rebate {rate}%」）。这两个文件里原有的「/ 秒」改动保留未动。

3. 渠道定价的币种标签：已检查，未修改
   - `PricingEntryCard.vue` 中 token 价格（`$/MTok`）、按次价格（`$`）、图片和视频默认价格（`$`）全部是「$」，视频与其他位置一致，按任务书要求不改。

#### 测试

- 新增 `src/components/user/affiliate-team/__tests__/TeamRequestTable.spec.ts`：系数为 30、12.5、0 时显示标签，系数为 100 时不显示。
- `src/views/admin/__tests__/SettingsView.spec.ts` 新增「分组分润系数」describe，共 7 个用例：
  - 只在消费基数下显示；
  - 回显已有分组和已删除分组；
  - 预览计算，包括修改利润池和系数后的更新；
  - 添加和删除行，已选分组不能重复选择，按钮置灰；
  - 保存的 payload；
  - 未选分组、系数越界、系数为空、分组重复时就地提示并阻止保存。
- 自检命令（在 `frontend/` 下）：
  - `npx -y pnpm@9 run lint:check`：通过
  - `npx -y pnpm@9 run typecheck`：通过
  - `npx -y pnpm@9 run test:run`：444 个文件、3532 个用例全部通过
  - `git status`：`pnpm-lock.yaml` 没有变化，也没有生成 `pnpm-workspace.yaml`。

#### 遗留问题

- 界面截图要等 BE-S 完成后补做，见本报告开头。

#### 截图补充（BE-S 完成后，2026-10-31）

- 环境：在 `~/workspace/sub2api-local` 运行 `./build.sh && docker compose up -d`，编译当前工作区（`90bff42b5-dirty`），健康检查通过。
- 造数：沿用 BE-S 的 p5 GF 用例留下的数据，没有另外造数。
  - 代理 `e2e-154955-ca@e2e.test`（id 2824）名下的客户 2827 在分组 277（`e2e-gf30-154955`）和 278（`e2e-gf0-154955`）各有一条请求。
  - 通过管理端接口设置 `affiliate_rebate_base=consumption`、`affiliate_group_rebate_factors=[{group_id:277,factor:30}]`；当时利润池 P=30，二级比例 R2=20。
- 截图：
  - `/tmp/seedance-fe-s/admin-settings-group-factors.png`：管理端「功能开关 → 代理利润」区块。系数行显示分组 `e2e-gf30-154955`、系数 30%，折算预览为「渠道经理 9% · 分销代理 6% · 渠道经理差额 3%」，与 P×f、R2×f、(P−R2)×f 的计算一致。
  - `/tmp/seedance-fe-s/team-requests_affiliate_team_customers_2827.png`：代理端团队客户的请求明细。只有 `e2e-gf30-154955` 那一行显示「分润 30%」；gf0 分组（未设置系数）和无分组的行都不显示标签。
- 结论：两处显示正常，没有需要修复的问题。管理端团队查看器和代理端共用 `TeamRequestTable.vue`，没有单独截图。
- 脚本：`~/workspace/sub2api-local/e2e/ui/shots.cjs` 新增 `GROUP_FACTOR_UI=1` 开关，写法同 `SIGNUP_BONUS_UI`：切到功能设置页签，只截「代理利润」区块。
  - 用法：`GROUP_FACTOR_UI=1 ./shots.sh <管理员邮箱> <前缀> /admin/settings`
- 恢复：截图后已把设置改回 `affiliate_rebate_base=recharge`、`affiliate_group_rebate_factors=[]`，并通过 GET 确认生效。
