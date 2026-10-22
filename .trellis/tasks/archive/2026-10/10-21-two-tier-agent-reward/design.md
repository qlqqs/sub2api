# 设计：两级代理级差分润与普通用户邀请奖励

## 0. 前提与清理
- **全新部署**：没有存量用户、历史返利和旧设置需要兼容，不做数据回填和旧设置推导。
- 丢弃未提交的「返利矩阵」改动，回到 5273473ef。
- 删除前两轮的「按层返利 / 等级解锁层数」代码（逐层走链、rebate_levels、level_rebate_rate_percents、unlocked_layers、affiliate_rebate_levels / affiliate_rebate_rate_l2 设置及界面）；结算框架（定时任务、领导锁、时间水位、真实充值封顶）原样复用。
- 迁移只追加不改：246、247 保留（项目规则：已提交迁移不修改），新增 248 收尾。

## 1. 数据模型（迁移 248_affiliate_two_tier_agent.sql）

### user_affiliates 新增
| 列 | 类型 | 说明 |
|---|---|---|
| agent_owner_id | BIGINT NULL（不建外键） | 绑定时确定的归属代理；邀请人本身是代理时等于 inviter_id。不建外键，账号删除后保留 ID，结算与展示时 LEFT JOIN users 判断是否仍存在 |
| agent_owner_level | SMALLINT NULL | 归属代理在绑定时的档位（1 / 2），结算按它取比例，之后归属代理升降级不影响 |
| agent_upline_id | BIGINT NULL（不建外键） | 归属代理为二级时，绑定时其上方最近的一级代理；归属代理为一级或绑定时上方没有一级则为 NULL。同样不建外键 |
| agent_owner_bound_at | TIMESTAMPTZ NULL | 归属确定时间（审计用） |
| agent_level_updated_by | BIGINT NULL | 最近一次改等级的操作人（管理员或一级代理） |

索引：`idx_user_affiliates_agent_owner_id`、`idx_user_affiliates_agent_upline_id`。agent_level_source 取值增加 `agent`（由一级代理任命）。

三列合起来就是这条关系的分润结构快照：谁拿哪一档、差额给谁，绑定后不再改。当前等级（agent_level）只用于：新绑定时的解析、本人消费的分法、任命权限、界面展示。

普通邀请奖励的资格不另存：`inviter_id IS NOT NULL AND agent_owner_id IS DISTINCT FROM inviter_id` 即「绑定时邀请人是普通用户」。

### user_affiliate_ledger
| 变更 | 说明 |
|---|---|
| 新增 reward_type VARCHAR(24) NULL | agent_direct / agent_diff / invite_percent / invite_fixed / invitee_bonus；非奖励流水（转出等）为 NULL |
| 删除 rebate_level（246 引入） | 由 reward_type 取代 |

唯一部分索引（首充奖励只发一次，DB 兜底并发）：
`UNIQUE (source_user_id, reward_type) WHERE reward_type IN ('invite_fixed','invitee_bonus')`。

### 消费水位修正
绑定邀请人（注册绑定或管理员补绑）时把 rebate_settled_at 置为绑定时刻，避免管理员给老用户补绑后，把绑定前的消费算给新邀请人。

### 代理利润设置
| 键 | 说明 | 默认 |
|---|---|---|
| affiliate_rebate_rate（沿用上游键） | 利润池 P（一级比例） | 20 |
| affiliate_agent_level2_rate | 二级比例 R2，≤ P | 10 |
| affiliate_agent_custom_rate_enabled | 专属比例开关 | false |

沿用上游的 affiliate_rebate_rate 作为利润池，减少与上游代码的分叉；affiliate_rebate_levels / affiliate_rebate_rate_l2 删除。

新增普通邀请 / 被邀请人奖励设置：

| 键 | 取值 | 默认 |
|---|---|---|
| affiliate_invite_reward_mode | off / percent / first_recharge | off |
| affiliate_invite_reward_rate | 0-100（percent 模式） | 0 |
| affiliate_invite_reward_duration_days | 0=永久 | 0 |
| affiliate_invite_reward_per_invitee_cap | 0=无上限 | 0 |
| affiliate_invite_reward_fixed_amount | ≥0（first_recharge 模式） | 0 |
| affiliate_invite_reward_min_recharge | 首充门槛（真实充值 USD） | 0 |
| affiliate_invitee_bonus_mode | off / percent / fixed | off |
| affiliate_invitee_bonus_value | 比例或金额 | 0 |
| affiliate_invitee_bonus_min_recharge | 首充门槛 | 0 |
| affiliate_invitee_bonus_max | 比例模式封顶，0=不封顶 | 0 |

沿用：affiliate_rebate_base、affiliate_settle_interval_minutes、冻结期、代理利润有效期、单人上限、升级方式 / 门槛 / 人数口径。

## 2. 归属确定（绑定时）
`BindInviterByCode` 与 `AdminBindInviter` 在同一事务内，按**当时**各人的等级解析，然后写死：
```
O = inviter                                  if inviter.agent_level ∈ {1,2}
O = firstCurrentAgent(inviter.agent_owner_id) otherwise   // 沿 agent_owner_id 链向上，跳过当前已是普通用户的账号
O 为空 → agent_owner_id / level / upline 全为 NULL
O.level == 1 → owner=O, owner_level=1, upline=NULL
O.level == 2 → owner=O, owner_level=2, upline=firstCurrentLevel1(O.agent_owner_id)  // 向上第一个当前等级为 1 的代理，没有则 NULL
```
「向上」统一沿 agent_owner_id 链走（结构链），判断用的是当前 agent_level。
解析归属链时若经过被绑定人本人（管理员给已有客户的老代理补绑邀请人时可能出现），返回 ErrAffiliateCycle 拒绝绑定，不只靠走链时的 visited 兜底。
之后再触发邀请人的自动升级评估（现有逻辑）。

## 3. 分润计算
输入：消费者 C、返利基数 E（充值模式为余额订单实付到账 amount − bonus_amount，消费模式为本周期可计入的余额消费，已按真实充值封顶）。
订阅订单不产生任何奖励：`affiliateRebateBaseAmount` 对 subscription 返回 0，与消费模式的封顶口径、首充口径、人数口径一致。

```
rate1(x) = 专属开关开且 x 有专属比例 ? 专属 : P
rate2(x) = 专属开关开且 x 有专属比例 ? 专属 : R2
O = C.agent_owner_id, L = C.agent_owner_level, T = C.agent_upline_id   // 全部来自 C 的绑定时快照，不看 O / T 现在的等级

switch C.当前等级:
  case 1:  无代理利润
  case 2:  本人消费视为一级的直属客户：
           L == 1 → O 得 rate1(O)·E                       [agent_direct]
           L == 2 → T 存在 → T 得 rate1(T)·E                [agent_direct]；O 不拿
                    T 为空 → 无代理利润
  case 3:  O 为空（无邀请人或归属代理已删除） → 无代理利润
           L == 1 → O 得 rate1(O)·E                       [agent_direct]
           L == 2 → T 存在 → O 得 rate2(O)·E                [agent_direct]
                             T 得 max(0, rate1(T)−rate2(O))·E [agent_diff]
                    T 为空（绑定时就没有一级） → O 得 max(rate2(O), P)·E [agent_direct]
                    T 已删除（ID 在，users 中不存在） → O 得 rate2(O)·E，差额不发

另算普通邀请奖励（percent 模式）：
  C 有邀请人 I 且 C.agent_owner_id ≠ I（绑定时 I 是普通用户），在有效期与单人上限内
  → I 得 X%·E [invite_percent]
```
- 快照语义：O、T 之后被降级、升级、取消任命，这条关系的分法不变；只有账号删除会去掉被删者自己的那份，其他人的份额不变。两列都不建外键，删除后 ID 保留，结算时 LEFT JOIN users 判断是否仍存在：这样能区分「绑定时就没有一级」（agent_upline_id IS NULL → O 拿满 P）和「一级后来被删」（ID 在、用户不在 → O 只拿 R2，差额不发），后台也能如实显示「已删除」。
- 沿用：代理利润有效期（从 C 绑定时起算）、冻结期、环检测。
- 单人上限按「受益人 × 来源用户 × 奖励类型」统计：代理利润（agent_direct + agent_diff）用 affiliate_rebate_per_invitee_cap，普通邀请比例奖励用 affiliate_invite_reward_per_invitee_cap；`GetAccruedRebateFromInvitee` 要带 reward_type 过滤，否则两类奖励互相挤占额度。
- 没有邀请人的用户（包括管理员直接设为二级的）没有归属链，本人消费不产生利润；`ListUsersWithPendingConsumption` 的 inviter_id IS NOT NULL 条件因此仍然成立。
- 入账进入受益人的返利余额（aff_quota，可转余额），与现有返利一致。
- 同一函数服务两种基数：充值模式在支付成功后的现有入账步骤调用；消费模式在定时结算中调用。

### 示例（P=30，R2=20，普通邀请比例 5%）
`B(一级) → A(二级) → U1(普通) → U2(普通)`

| 场景 | 消费 100 | A | B | U1 |
|---|---|---|---|---|
| 初始 | U2 | 20 | 10 | 5（普通邀请） |
| 初始 | A | 0 | 30 | — |
| 初始 | B | — | 0 | — |
| U1 升二级后 | U2（仍归 A） | 20 | 10 | 5 |
| U1 升二级后 | U1 新邀请的 U4 | 0 | 10 | 20 |
| U1 升二级后 | U1 本人 | 0 | 30 | — |
| A 升一级后 | U2（快照 A=二级，上方 B） | 20 | 10 | 5 |
| A 升一级后 | A 新邀请的 U5 | 30 | 0 | — |
| B 降为普通后 | U2（快照不变） | 20 | 10 | 5 |
| B 降为普通后 | A 新邀请的 U6（上方无一级） | 30 | 0 | — |
| B 账号删除后 | U2 | 20 | — | 5 |
| 二级 C 绑定时上方无一级 | C 的客户 | C 得 30 | | |

## 4. 首充奖励（支付成功后）
在 `applyAffiliateRebateForOrder` 之后新增 `applyFirstRechargeRewards`，仅处理真实支付的余额订单（不含管理员充值、兑换码、订阅订单）：
1. 「首充」= 该用户第一笔达到门槛的真实余额充值：实付到账 = amount − bonus_amount ≥ 门槛，且 ledger 中尚无该用户对应 reward_type 的记录。未达门槛的订单直接跳过，不消耗资格。
2. 被邀请人奖励与邀请人固定奖励各有门槛，分别独立判定；同一订单可能只触发其中一个。
3. 被邀请人奖励（用户有邀请人即可，邀请人是代理也给）：比例模式 = 实付到账 × X%（受封顶限制），或固定金额；直接加到用户余额，并写 ledger（reward_type=invitee_bonus）。赠送余额不计入真实充值，天然不产生代理利润。
4. 邀请人固定奖励（first_recharge 模式且绑定时邀请人是普通用户）：给邀请人返利余额加固定金额（受冻结期影响），写 ledger（reward_type=invite_fixed）。
   邀请人是代理（一级或二级）时不发：被邀请人照常拿首充奖励，代理只拿该用户后续的分润。
5. 幂等：是否已发以 ledger 为准（`EXISTS source_user_id = C AND reward_type = ?`），并发由唯一部分索引兜底，冲突视为已发放。与订单审计日志同事务，失败写 AFFILIATE_FIRST_RECHARGE_FAILED 审计，不影响订单完成。

## 5. 升级
默认手动（affiliate_agent_upgrade_mode 缺省为 manual），管理员可切换为按人数自动升级。
沿用现有：手动 / 自动开关、二级门槛、一级门槛、人数口径（全部直属 / 仅真实充值）、手动降级锁定、「重新评估」。升级、降级都只改 agent_level，不改任何已有关系的快照；降级者继续按快照拿已有客户的分润，只是之后新绑定的人不再把他当代理。
新增触发点：人数口径为「仅真实充值」时，直属用户首充完成后（第 4 节同一步骤）评估其邀请人。

### 5.1 一级代理任命二级代理
- 对象：一级代理自己的**直属且归属自己**的用户（inviter_id = 自己 且 agent_owner_id = 自己）。
  限定归属自己，保证任命后该二级的差额回到任命人；成为代理前邀请的人归属别的代理，不能任命。
- 操作：设为二级（普通 → 二级）、取消二级（二级 → 普通）。只能取消自己任命的；管理员设置或自动升级来的二级不能由一级代理改动。
- 开关与上限（新增设置）：
  | 键 | 默认 |
  |---|---|
  | affiliate_agent_appoint_enabled 允许一级代理任命二级 | true |
  | affiliate_agent_appoint_max 每个一级最多任命几个（0=不限） | 0 |
- 记录：agent_level_source 新增取值 `agent`；新增列 agent_level_updated_by（操作人 ID，管理员或一级代理），用于后台显示「由 a@x.com 任命」。
- 取消任命视同手动降级：锁定自动升级；被取消者已有客户的分润按快照继续，之后新邀请的人归任命人。
- 效果与其它升级一致：只影响该用户之后新邀请的人；已有关系不变。二级比例用全局 R2（或管理员设的专属比例），一级代理不能自定义。
- 任命人之后被降级：已任命的二级保持不变，不连带。
- 接口：`POST /api/v1/user/aff/invitees/:user_id/agent-level`，body `{ "agent_level": 2 | 3 }`；服务端在事务内锁行校验（自己是一级、开关开启、对象符合条件、未超上限）。

## 6. 管理端（详见 ui.md）
- 设置页「邀请返利」分四块：
  1. 代理利润：返利基数、结算周期、利润池 P、二级比例 R2（校验 R2 ≤ P）、专属比例开关、有效期 / 冻结 / 单人上限；下方实时示例（消费 100 时二级、一级各得多少）。
  2. 代理升级：现有控件。
  3. 普通用户邀请奖励：模式（关闭 / 按比例 / 首充固定金额）及对应参数。
  4. 被邀请人首充奖励：模式（关闭 / 比例 / 固定金额）、数值、门槛、封顶。
- 专属用户列表：等级显示为「普通用户 / 二级代理 / 一级代理」，新增「归属代理」列。
- 用户总览、返利记录：显示归属代理、奖励类型。

## 7. 用户邀请页
- 普通用户：邀请链接；「邀请好友你得什么 / 好友得什么」；升级为二级的进度。
- 代理：我的档位与比例（已套专属比例）、归属我的客户数、按奖励类型分组的收益；二级显示升一级进度。
- 邀请列表增加「归属」标记（归我 / 归上级代理）。

## 8. 接口变更
- 设置 GET/PUT：affiliate_rebate_rate 保留（含义改为利润池）；移除 affiliate_rebate_levels / affiliate_rebate_rate_l2；新增第 1 节的其它设置键。
- 用户邀请详情：移除 rebate_levels / level_rebate_rate_percents / unlocked_layers；新增 agent_rate_percent、owned_customer_count、invite_reward（模式与参数）、invitee_bonus（模式与参数）、reward_totals（按类型汇总）。
- 管理端用户总览 / 代理管理列表 / 邀请记录：新增 agent_owner_id、agent_owner_email、agent_owner_level、agent_upline_id、agent_upline_email（快照值，被删者显示「已删除」）。
- 返利记录：rebate_level 改为 reward_type。
- 公开设置接口（未登录可读）：新增 affiliate_invitee_bonus 的模式 / 数值 / 门槛 / 封顶，供注册页展示；不暴露利润池、二级比例、邀请人奖励等任何其它返利设置。

## 9. 测试
- 单元：第 3 节示例表逐行；专属比例开关；差额不为负；绑定时上方无一级拿满 P；绑定时解析跳过当前普通用户与二级；快照冻结（O / T 升降级后分法不变）；T 删除后差额不发、O 删除后无利润；绑定时环检测；普通邀请资格；有效期 / 上限按奖励类型分开；冻结；订阅订单不计；设置 clamp 与 R2 ≤ P。
- 首充：只发一次（重复调用、并发唯一索引冲突）、门槛、非首单不发、邀请人是代理时只发被邀请人奖励。
- 集成（Postgres）：唯一索引、绑定时归属与水位写入、消费结算端到端。
- 前端：设置页四块的读写与校验、邀请页按角色渲染。

## 10. 风险与默认处理
- 首充后退款：已发首充奖励不自动收回（与现有返利一致）；邀请人奖励受冻结期保护，建议开启冻结期。
- 降级代理：已有客户的分润按快照不变（降级者继续拿），只影响之后新绑定的人与本人消费的分法。管理员若要彻底切断，只能删除账号。
- 代理账号删除：快照列不建外键、不级联，被删者的那份不再发放，其他参与者份额不变；后台显示「已删除」。
- 改比例后：充值模式立即生效；消费模式下未结算的部分按结算时的比例计算。
- 管理员补绑老用户：绑定时只重置 rebate_settled_at，不动 rebate_settled_consumption，绑定前的真实充值仍计入封顶额度。全新部署下影响很小，接受。
- 订阅订单：不进任何奖励口径；用户只买订阅不充余额，不会触发首充奖励，也不计入「仅充值过」人数。
