# 技术设计：注册默认 Key 与工作台首次初始化

状态：实现及本机验收完成，2026-10-26 已获提交、推送、部署授权。行为契约以 prd.md R1–R10 为准。

## 1. 目标分组、资格与注册
- 已只读核实一号机 `/root/sub2api-custom`：唯一精确名称 `Codex 稳定分组`，ID 2，active/openai、支持生图。没有远端写入；本机使用隔离 fixture ID 145。
- 迁移 259 向 settings 写入 `signup_workbench_group_id`：只在唯一精确名称命中且设置不存在时绑定一次。随后按 ID 查询当前名称，不按名称运行时重找目标，不增加管理界面。
- `workbench_signups(user_id PK, group_id nullable FK)` 是新注册资格与持久一次性 claim。邮箱、legacy OAuth、token-pair OAuth 和 email OAuth 新建路径调用 `AuthService.bootstrapWorkbench`；旧登录和管理员创建不调用。
- 完成必须的注册步骤后复用 `APIKeyService.GetAvailableGroups/Create`；Key 名称来自当前组名，绑定目标 ID。失败仅记录脱敏告警，不使成功注册失败。
- claim 在创建前提交，故并发或重复调用最多尝试一次；失败／中断不自动重试，删除 Key 后不重建。这是 fail-open 辅助初始化，不承诺事务性必达。

## 2. 当前用户 API 与持久化
- `GET /api/v1/user/workbench/:workbench`（studio/chat）：认证用户自己的 eligible/group_id/group_name/initialized/preferences/text_model/image_model/reasoning_effort/reasoning_allowed；Cache-Control:no-store。
- `POST` 同路径保存首次初始化，数据库 ON CONFLICT 保留当前偏好并返回权威结果；`PUT` 显式保存或清空。请求 `{preferences:{...}}`，仅允许 Key ID/模型字段，去掉未知字段，最多4096字节偏好内容，不存明文密钥。
- `workbench_preferences(user_id,workbench PK,preferences jsonb,updated_at)` 存 Nova 的用户级偏好；行存在即已初始化，`{}` 也是显式清空，不等于未初始化。
- `GET /_chat/__sub2api/workbench` 复用元数据，受原 Chat Cookie 身份校验；不跨数据库修改 OWUI。
- 非法 workbench/偏好返回分类400；偏好服务不可用503；未知存储错误500，不当成400。返回的未知错误不泄露数据库详情。

## 3. Nova 初始化与请求
- `studio-state.ts` 仅对 eligible 用户初始化；先保留现有偏好，再找同 group_id、同当前名称、可用的本人 Key。无匹配则提示配置，不补 Key。
- `workbench-defaults.ts` 对文本 `gpt-6.1-sol`、生图 `gpt-image-2.5-sunburst` 精确匹配，缺失不给任意其他模型回退，medium 受分组允许档位约束。
- `nova-proxy-text.ts` 为新用户默认文本模型实际传递 Responses `reasoning.effort=medium` 或 Chat Completions `reasoning_effort=medium`，不是只改显示标签；独立模式不变。
- 用户级偏好保存在 sub2api；会话 generation 防止撤销后异步写回；初始化返回并采用数据库权威偏好。显式修改或清空跨浏览器登录保留。
- Nova 既有实现仍会在运行时内存使用明文 Key，本次不新增明文持久化，也不将其描述为 OWUI 的 Key-ID 反代协议。

## 4. Open WebUI 初始化与请求
- `embed/workbench-defaults.ts` 使用同组同名可用 Key、真实模型列表与档位列表，设置 `models`、`ui.sub2api`、`ui.sub2api_defaults`、`ui.sub2api_reasoning`。
- 首次请求使用自己的 `/api/v1/users/user/settings/update` 与 `sub2api_initialize:true`；`embed/workbench.py` 采用条件 SQL UPDATE，只在尚无 Key 选择、初始化标记或模型偏好时写入。显式保存 Key 设置会标记已初始化；返回并采用当前权威 settings。
- `ui.sub2api_initialized` 与当前选择分离，显式 `{key_id:null,image_key_id:null}` 不再触发初始化；已有模型设置也不覆盖。
- reasoning UI 和实际请求采用持久化的用户档位；选择 high 后重新登录仍为 high。旧用户保持原行为。
- `Chat.svelte` 禁止指定默认模型不可用时选任意模型；`embed/images.py` 生图/编辑使用精确默认生图模型，不可用则在付费请求前提示。
- 新增提示复用 embed/i18n.ts 的中英字典，不修改上游 locale。

## 5. 余额不足与权限边界
- 仅 `GET /v1/models` 跳过用户余额阈值检查，使无余额的新用户可发现默认模型；Key归属/状态/过期/额度、分组和订阅校验仍保留。生图与对话付费 POST 不放行。
- Nova `ccode-task-client.ts` 保留结构化任务错误，`task-events.ts` 提取 code/type/message/detail/error 后交给统一分类；不把错误降为单独状态403。
- 真实 INSUFFICIENT_BALANCE 显示「账户余额不足，请充值后重试」及去充值；裸403/402不推断余额，insufficient_quota归为Key额度。无生产失败请求回放，不声称已证明用户原生产路径的唯一根因。

## 6. 发布与回滚
- 本机独立 sub2api-local / -nova / -chat 已验证；用户现已授权三个仓库提交、推送、更新配套部署。
- 本机已应用259与财务260；不得改已应用迁移，财务代码保持冻结。
- 可回退应用版本；新增表与设置为增量兼容，不删除已创建Key或用户配置。默认目标无法解析时注册照常完成，工作台仍允许手动配置。
- 后续部署先备份并核对目标设置，再更新三个关联镜像；不操作其他同机部署。
