# 新用户工作台默认配置（fork）

## 1. Scope / Trigger
修改注册后的默认Key创建、Nova/OWUI首次配置、用户偏好或模型发现余额校验时适用。仅新用户资格，不扩展管理员创建或存量补建；三个仓库分别维护自身实现。

## 2. Signatures
- `AuthService.bootstrapWorkbench(ctx,user)` / `WorkbenchState(ctx,userID,kind)` / `SaveWorkbench(ctx,userID,kind,prefs,initialize)`。
- 迁移259：`settings.signup_workbench_group_id`、`workbench_signups(user_id PK,group_id)`、`workbench_preferences(user_id,workbench PK,preferences,updated_at)`。
- 会话认证 `GET/POST/PUT /api/v1/user/workbench/:workbench`；Chat保留段 `GET /_chat/__sub2api/workbench`。
- OWUI自己的settings接口接受embed-only `sub2api_initialize:true`，由 `embed/workbench.py::initialize_settings` 条件更新；sub2api不跨库写入。

## 3. Contracts
- 目标只在迁移时按唯一精确名称 `Codex 稳定分组` 绑定一次，运行时按ID取当前名称。不能每次找同名第一组，也不能把本机fixture ID硬编码进生产。
- 注册成功后持久claim，复用 `APIKeyService.Create`。claim先于创建，重复/并发最多尝试一次；创建失败不阻断注册，也不自动重试。
- 元数据含eligible/group_id/group_name/initialized/preferences/text_model/image_model/reasoning_effort/reasoning_allowed。客户端匹配本人、同组ID、同名、可用Key；进入工作台不创建Key。
- `POST`初始化冲突时保留旧偏好并返回权威结果；`PUT`显式覆盖。初始化状态由行/标记存在确定，不能根据配置是否为空推断。偏好只存ID和模型，未知字段剔除，禁止明文密钥持久化。
- Nova偏好在sub2api，OWUI偏好在自身user.settings.ui；双方均保留人工设置和清空。首次默认文本 `gpt-6.1-sol`/medium、生图 `gpt-image-2.5-sunburst` 必须精确可用，缺失时提示，不静默fallback。
- OWUI条件SQL必须在写入时复核未初始化/未人工选Key或模型，防止读后写竞态。reasoning的显示和真实请求共用持久用户选择。

## 4. Validation & Error Matrix
| 条件 | 行为 |
|---|---|
| 目标缺失、不可访问或创建失败 | 注册仍成功，工作台无Key则提示配置 |
| 非法kind/偏好 | 400 WORKBENCH_INVALID / WORKBENCH_PREFERENCES_INVALID |
| 偏好服务未提供 / 未知DB错误 | 503 WORKBENCH_UNAVAILABLE / 安全500，不泄露SQL |
| 模型或medium不可用 | 要求人工选择，不发付费请求探测，不任选首个模型 |
| GET /v1/models且用户无余额 | 仅跳过余额阈值；保留Key状态/额度/过期、分组权限和订阅检查 |
| 生图/对话POST且余额不足 | 保持原网关拒绝，UI提示充值 |
| 任意403/402、insufficient_quota | 不能仅凭状态判断用户余额；quota提示Key额度 |

## 5. Good / Base / Bad
- Good：新注册生成同组同名Key；首次配置准确模型；改Key或清空后跨浏览器登录仍保留。
- Base：老用户没有workbench_signups资格，维持原手工配置；独立Nova/OWUI模式不启用默认行为。
- Bad：默认Key删除后再进入自动补建；按同名异组Key初始化；把空偏好当作未初始化；错误由结构体降为“403”。

## 6. Tests Required
- Go `workbench_defaults_test.go`：并发一次、失败不重试、白名单与错误分类；`workbench_registration_test.go`：邮箱/OAuth新建hooks失败不阻断及已有登录不调用。
- Repository `workbench_defaults_integration_test.go`：真实Postgres claim、初始化竞争、显式清空、双工作台隔离、旧用户无资格。
- Middleware `workbench_models_test.go`：只允许精确method/path余额豁免；本机零余额GET models200，付费image POST仍拒绝。
- Nova `studio-defaults-state.test.ts` 与 `workbench-defaults.test.ts`：会话撤销、手动CAS胜出、跨会话清空、缺失模型无fallback；gateway-errors测试结构化错误与非余额反例。
- OWUI `test_workbench_defaults.py` 测真实SQLite原子初始化/显式清空，`test_image_models.py` 测精确生图模型；前端embed测试同名异组与用户设置保护。
- 本机Playwright必须断言真实请求model与medium、零余额异步卡片及充值操作、手动Key/档位/清空后新登录；不能只看初始化接口成功。

## 7. Wrong vs Correct
- 错：`if (!prefs.key) initialize()`；对：独立的持久初始化标记，空选择也可已初始化，写入用条件SQL并采用服务端返回值。
- 错：403统一映射充值，或只修改文案映射丢失上游code；对：任务创建/轮询到展示全程保留结构化code/message，明确识别余额并区分Key配额。
- 错：只给UI标注medium；对：验证Responses reasoning.effort或Chat Completions reasoning_effort的真实上游请求。
