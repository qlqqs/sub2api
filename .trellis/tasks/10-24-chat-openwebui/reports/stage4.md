# 阶段 4 报告：sub2api 后端（Go）

日期：2026-10-24。分支：`sub2api-custom`。实现依据：任务目录的 PRD、design、contract（A/B/C）、implement 4.1–4.4 与 poc-c。

## 1. 结论

阶段 4 已完成，指定的全量单元测试与构建均通过（退出码 0），可交阶段 6 本机联调。没有修改 `frontend/`、契约、其他项目或本机测试环境；没有启动/重启容器，没有提交、推送或部署。工作树中的前端改动属于另一位代理，本阶段没有覆盖它们。

## 2. 改动文件

### 实现

| 文件（均相对 `backend/`） | 内容 |
|---|---|
| `internal/config/config.go` | 新增 `ChatConfig.WebUIURL`、`chat.webui_url` 默认值，使 `CHAT_WEBUI_URL` 可由 Viper 加载 |
| `internal/server/router.go` | Studio 同处新增一行 Chat 安装调用，位于 SPA 中间件之前 |
| `internal/server/chat_proxy.go` | 会话、状态、静态/动态请求识别、反代、Key 注入、自有接口、安全头、大小限制 |
| `internal/handler/chat_backend.go` | 复用现有 Auth/User/APIKey/ModelPlaza 服务的窄适配层，不新增 Wire 依赖 |
| `internal/service/chat_model_prices.go` | 从模型广场领域数据生成用户实付口径 `price_text` |
| `internal/pkg/apicompat/chat_citations.go` | 引用类型、转换、字符偏移、去重、搜索段落边界和缓冲聚合挂接 helper |
| `internal/pkg/apicompat/types.go` | 增加 citations/annotations 字段，全部采用 `omitempty` |
| `internal/pkg/apicompat/anthropic_to_responses_response.go` | Anthropic 流式/非流式引用映射挂接 |
| `internal/pkg/apicompat/responses_to_chatcompletions.go` | CC 流式/非流式 annotations；终止 output 缺失时的缓冲重建 |
| `internal/pkg/apicompat/responses_stream_event_wire.go` | 定制 JSON marshaler 保留引用；annotation 事件明确输出为 0 的索引 |
| `internal/service/gateway_forward_as_chat_completions.go` | 一行聚合挂接，保留上游 SSE 的 `citations_delta` |
| `internal/service/gateway_forward_as_responses.go` | 同上 |
| `internal/service/openai_gateway_chat_completions_anthropic_native.go` | 同上，覆盖 OpenAI 网关的 native-Anthropic 兼容路径 |
| `internal/service/openai_gateway_responses_anthropic_native.go` | 同上 |

### 测试与基线

- `internal/config/chat_config_test.go`
- `internal/handler/chat_backend_test.go`
- `internal/server/chat_proxy_test.go`
- `internal/service/chat_model_prices_test.go`
- `internal/service/chat_gateway_citations_test.go`
- `internal/pkg/apicompat/chat_citations_test.go`
- `internal/pkg/apicompat/chat_citations_regression_test.go`
- `internal/pkg/apicompat/testdata/chat_no_citations.golden`

没有数据库迁移、依赖/锁文件修改、Wire 生成文件修改。

## 3. 逐项完成情况

### 4.1 配置

- `CHAT_WEBUI_URL` 对应 `chat.webui_url`，空值关闭。
- status 使用标准 `{code:0,data:{enabled}}` envelope，`enabled` 表示配置非空，不探测上游在线状态。
- 配置为空/非法时反代不可用；上游连接失败返回 503，不回落为 SPA 首页。

### 4.2 会话与反代

- POST session 读取 Bearer access token，复用 `AuthService.ValidateToken` 和 `UserService.GetByID`，检查签名、有效期、最新用户状态及 token version；Cookie 中存当前 access token。
- Cookie：`s2a_chat`、`Path=/_chat`、HttpOnly、SameSite=Strict；Secure 与现有 Cookie 一致，由 TLS 或 `X-Forwarded-Proto=https` 判定。响应 `expires_at` 为 RFC3339。
- DELETE 不要求有效登录；设置 `Max-Age=0` 清除 Cookie。未配置时也先清除，再返回契约要求的 503。
- 静态白名单与 HTML 导航免 Cookie，不注入用户/Key；API、WS、OAuth、自有接口不可借 HTML Accept 绕过鉴权。拒绝路径中的歧义段（如 `static/../api`）。
- 转发前删除所有客户端 `X-Sub2API-*`、`X-Forwarded-*`、`X-Real-IP`、`Forwarded`，然后只注入本站确定的值。注入位于 ReverseProxy Rewrite 内，避免恶意 `Connection` 头把可信头作为 hop-by-hop 头删掉。
- 注入用户 ID、合成邮箱、用户名/邮箱前缀、admin/user 角色、用户 IP。对话 Key 通过校验时同时注入 Key 与 Key-Platform；生图 Key 单独校验。
- Key 归属/状态/过期/总额度/分组状态异常均不注入明文；额度、余额、并发、RPM、IP ACL 等最终仍由真实网关鉴权和计费执行。
- WebSocket 支持真实升级及双向消息；握手注入身份，但永不注入对话 Key、生图 Key、Key-Platform 或客户端 Key-ID。
- 只移除 sub2api 的 `s2a_chat` Cookie，保留 Open WebUI 自己的 Authorization 与 Cookie；不会把控制台 JWT Cookie 发给 fork。
- 剥掉 `/_chat`，保留查询串；单 `/` 开头的 Location 补前缀，已经带前缀的不重复处理，绝对 URL/`//` URL 不改写；所有 Set-Cookie 中 `Path=/` 改为 `Path=/_chat`。
- 上游 CSP/X-Frame-Options 删除，响应使用契约默认 CSP 或精确 artifact-frame CSP，以及 `SAMEORIGIN`，不会叠加重复策略；其他站点路径安全头不变。
- 上传 `/_chat/api/v1/files/*` 为 25 MiB，其余 5 MiB；固定长度和未知长度请求都在访问上游前检查，超限 413。
- 反代 Transport 禁用环境 HTTP 代理，内部用户 Key 不会因宿主机代理环境变量被送往第三方代理。

### 自有接口（contract C）

- `keys` GET/POST、`groups` GET、`model-prices` GET 均由 `s2a_chat` Cookie 鉴权。
- 成功为原始 JSON；错误为 `{"error":{"code","message"}}`，文案为中文，不泄露底层错误。
- Key 列表和创建响应均使用白名单 DTO，不含明文 Key，亦不返回完整 service 模型。
- 可创建分组由 `APIKeyService.GetAvailableGroups` 取得；创建继续调用现有 `Create`，不绕过归属、分组权限、数量限制。
- 写接口只接受 `application/json`（允许 charset 参数），否则 415；创建 body 只接受 name/group_id。
- 价格复用模型广场服务、广场可见性过滤及用户专属价格数据；查他人 Key 返回 404，没有价格的模型不返回。

### 4.3/4.3a/4.3b 引用与生图

- Anthropic `text.citations` / `citations_delta` 的 `web_search_result_location` 映射为 Responses 扁平 `url_citation`。
- Responses annotation 事件、`output_item.done`、terminal output 均保留引用；转为 CC 时输出嵌套 `delta.annotations[].url_citation` / `message.annotations[].url_citation`，不会重复输出同一 annotation。
- Responses 偏移以各 output_text part 为基准；CC 扁平内容使用累计字符偏移，包含插入的换行。中文用 Unicode 字符计数而非 UTF-8 字节数。
- 搜索前后补 `\n\n`；没有 citations 时不补分隔符。Anthropic 和原生 Responses→CC 路径都覆盖。
- 真实非流式网关入口强制读取上游 SSE，因此除了转换器还给四个 SSE 聚合入口各加一行挂接；否则非流式会在进入转换器前丢引用。
- 没改 token usage、搜索计费、余额扣减或 usage 写入逻辑。
- `__sub2api/keys` 和 `/groups` 返回平台与生图能力，规则见下一节。

## 4. `supports_images` 规则

必须同时满足：

1. 存在分组；
2. `service.GroupAllowsImageGeneration(group)` 为真（即分组启用 `allow_image_generation`）；
3. platform 为 `openai`、`grok` 或 `composite`。

依据：`server/routes/gateway.go` 的 imagesHandler 分发到 OpenAI/Grok；Composite 先经 `compositeTargetPlatformMiddleware` 按模型解析到具体平台，再进入相同分发。Anthropic/Gemini/Antigravity 即使有自己的原生生图协议，也不因此支持这里的 `/v1/images/generations`。

这是“该分组平台允许此接口”的能力提示，不保证任意模型/任意时刻都有可调度账号；Composite 仍须有实际可落到 OpenAI/Grok 的模型路由。Key 是否可选还要看 `usable`。生图 Key 注入同时检查这两者。

## 5. `price_text` 规则

依据只读核对：`frontend/src/components/modelPlaza/PlazaModelPricingTable.vue`、`utils/balanceCurrency.ts`、`utils/pricing.ts`。

- 数据源是 `ModelPlazaService.ListGroups`，不重建定价目录、分组卡、渠道规则或长上下文计费规则。
- 展示实付口径：模型单独价格因子优先，其次用户专属，再其次分组；图片/视频独立价格因子启用时沿用广场的覆盖规则。
- 当前 fork 广场“本站价”用 **¥ 余额单位**，金额不按美元汇率再换算。自动/默认美元汇率只影响广场“相比官方价”列，不影响本站单价，所以两种汇率模式下此接口相同。
- token：`输入 ¥… / 百万 tokens · 输出 ¥… / 百万 tokens`；按张/按次：`¥… / 张` 或 `/ 次`。
- 保留模型广场解析后的区间/分辨率档位；分时价格显示标准时段与具体时段、时区、工作日标志。缺失单价不伪造为 0；没有任何可展示价格的模型不返回。
- 数字按 10 位有效数字去掉浮点噪声，普通小数至少 2 位；极小数和大数采用与 JS Number.toString 相同的指数阈值。
- 最终文案不含“倍率”。

## 6. IP 透传结论

**无需修改网关信任逻辑。**

源码链路：

1. Chat 与 Studio 一样调用 `ip.GetSecurityClientIP`，解析出的单个用户 IP 写入 XFF/X-Real-IP。
2. fork 应只从可信 TCP 对端读取该 IP，然后调用网关时带同值 XFF（contract D、Nova 契约 §6）。
3. 网关 `SessionBindingContext` 为请求设置解析模式；Chat Completions、Responses、Images handlers 使用 `ip.GetClientIP(c)` 填入 `RecordUsageInput` / `OpenAIRecordUsageInput.IPAddress`。
4. `gateway_usage_billing.go` / `openai_gateway_usage.go` 将该 IP 原样写入 UsageLog；Key ACL 使用同一安全 IP 解析规则。

新增测试证明：在**关闭原始转发头兼容接管**、只信任指定内部 CIDR 时，可信 TCP 对端的 XFF 同时成为 usage/ACL 的 IP；不可信对端的伪造 XFF 不被接受。代理侧也覆盖可信入口与伪造转发头剥离。

部署注意：现有项目支持“原始转发头兼容接管”，其开启时会覆盖 trusted_proxies。阶段 6 应明确关闭，并配置可信代理 CIDR；不要为了工作台启用全网信任。本阶段没有更改现场配置，也没有宣称已执行容器内 usage 落库联调。

## 7. 测试结果

2026-10-24 最终验证：

```bash
cd backend
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go build ./...
```

两条命令均 **退出码 0**。执行 shell 额外设置 `GOPROXY=off`，使用本机缓存的依赖/工具链，未下载依赖。实际工具链为仓库 go.mod 指定的 Go 1.27.2，没有修改 go.mod。

- 新增 25 个顶层 `TestChat*` 测试（另含表驱动子用例）；专项与全量均通过。
- 最近一轮实际运行 service 全包耗时约 216 秒；后续最终复核允许 Go 使用测试缓存。
- `gofmt -l` 对全部改动 Go 文件无输出；`git diff --check -- backend` 通过。
- 最终命令日志：`/tmp/chat-stage4-final-unit.log`、`/tmp/chat-stage4-final-build.log`。


专项覆盖：

- config 环境变量/空值；真实 JWT 签名错误、过期、封禁、版本撤销；会话设置/清除。
- 未配置/不可达 503、无 Cookie 401、静态免 Cookie、HTML 不能绕过 API/WS/OAuth/自有接口。
- 伪造身份/转发头剥离、真实身份/角色/用户名、他人/禁用/过期/耗尽/失效分组 Key 不注入。
- 生图 Key、Key-Platform、平台生图开关与 Composite/Grok 能力；Key 列表/创建无明文。
- 前缀与查询串、Location、多个 Set-Cookie、默认/artifact CSP、全站其他路径头保持不变。
- 真实 WebSocket 升级/回声，所有 Key 头缺失；5/25 MiB 与未知长度超限；415 和非法创建输入。
- 价格优先级、免费价格、独立图片价格、档位、分时、精度、缺失价格。
- G1/G2 流式/非流式、多引用、中文偏移、终止事件保留、去重、缺失 terminal output 的重建。
- 真实 Gateway/native-Anthropic 四个非流式 SSE 聚合入口和 Gateway 流式输出，usage 数量保持不变。
- **无 citations 逐字节回归**：golden 从改动前 HEAD 的转换器捕获，覆盖普通文本、thinking/function_call，以及“搜索但无引用”。只规范化随机 item ID 与生成时间，序列化 JSON/事件顺序精确比较，不是用新实现自我生成预期。

本机没有 golangci-lint，未声称 lint 已执行；已人工复查新增写操作和类型断言的 error/ok 检查。未运行 Docker 集成测试或 e2e，按任务要求留给主会话。

## 8. 阶段 6 本机联调说明

### 配置与网络

sub2api compose 添加：

```yaml
environment:
  CHAT_WEBUI_URL: http://chat-webui:8080
  # 示例：实际采用部署分配的聊天网络 CIDR，并保留已有可信入口代理 CIDR。
  SERVER_TRUSTED_PROXIES: <已有可信入口CIDR>,<聊天内部网络CIDR>
  SECURITY_TRUST_FORWARDED_IP_FOR_API_KEY_ACL: "false"
```

同时核对后台持久化的 `api_key_acl_trust_forwarded_ip` 设置为 false：数据库设置可以覆盖环境默认值。不要使用 `0.0.0.0/0` 或无差别信任全部私网。

- sub2api 与 fork 共用独立 `internal: true` 的聊天网络。fork 使用别名 `chat-webui`，不映射宿主机端口。
- fork：`S2A_GATEWAY_URL=http://sub2api:8080`（用网络内真实别名）、`S2A_TRUSTED_PROXY_CIDRS=<聊天网络CIDR>`、`S2A_EMBED_MODE=1`；其余变量按 contract G / 阶段 3。
- uvicorn 保持 `--no-proxy-headers`，不设 `--root-path`；本站 `/_chat` 前缀由 Go 剥除。
- 无需配置新的 sub2api 内部共享密钥。生图测试分组需启用 `allow_image_generation`。
- 应测试：用户 IP→fork→网关→usage_logs；配置 Key IP 白名单验证同值通过/异值拒绝。此项不能用本文的单元测试代替最终落库验证。

### upstream-mock 引用场景

poc-c 记载已有 `poc_ext.py` 扩展，本阶段没有读取/改动/重启 mock 或容器。主会话先确认扩展是否仍启用；如果存在，通常不需重写 mock，只需启用对应账号/触发条件。

需要的响应形状：

- Anthropic `/v1/messages`：`message_start`；搜索前 text；`server_tool_use(name=web_search)`；`web_search_tool_result`；搜索后 text_delta；`citations_delta.citation={type:web_search_result_location,url,title,cited_text}`；message_delta/message_stop。至少两个引用，最好有中文。非流式客户端也要返回这一 SSE，因为网关强制上游 stream。
- OpenAI `/v1/responses`：`web_search_call`，text delta，`response.output_text.annotation.added`（扁平 type/url/title/start_index/end_index，含 output/content/annotation index），`output_item.done` 与 completed.output 同时保留相同 annotations；另测仅 done 中有注释、completed.output 为空的缓冲兜底。
- 对照组不带 citations，文本必须原样拼接；有引用的搜索前后才补空行。
- poc-c 的 OpenAI mock 触发 Key 是 `sk-poc-chat-upstream`；Anthropic 触发条件是 tools 含 web_search。均为 mock fixture，不是生产凭据。
- 生图继续使用 `b64_json`，避免把外部 URL 下载行为混入本阶段验证。
- `/v1/chat/completions` 流式检查 `delta.annotations[].url_citation`，非流式检查 `message.annotations`；不要只检查界面“来源”按钮，因为 fork 的非流式 UI 解析是另一层职责。

## 9. 已知边界与注意事项

- 尚未做本机容器/浏览器联调；实际搜索上游是否支持 web_search、fork 的身份/Key 消费、SSO/切账号、图片、文件及 artifact 隔离均需阶段 6 验收。
- 为同时满足“搜索段落补空行”和“无引用字节不变”，搜索后首段暂存至收到引用或该段/输出结束；上游很晚才给引用时，这一段输出会相应延迟。普通无搜索流不受影响。
- 搜索按次收费保持现状，不新增收费；本文新增 token usage 断言不代表已完成真实扣费端到端验收。
- 生图能力是接口能力，不是具体模型/账号在线状态保证。
- Cookie Secure 判定按既有规则，边缘反向代理必须正确覆盖 `X-Forwarded-Proto`。
- 未调用会自动提交的 Trellis archive/journal 流程；代码及本报告留给主会话统一审查并征得用户同意后提交。

## 10. 约定变更提议

没有实现所必需的接口改约定，未修改 `contract.md`。

建议主会话作两处文档澄清（不改变接口）：

1. C.4 示例目前写 `$`，但“与模型广场一致”在当前 fork 实际是 `¥` 本站余额口径；建议示例改为 `输入 ¥3.00 / 百万 tokens …`，说明美元汇率只用于官方价折扣比较。
2. C.1 的 `supports_images` 注释可明确 OpenAI/Grok/Composite 及分组生图开关，避免其他阶段误以为仅 OpenAI 支持。

设计 §2.2 曾有“角色固定 user”的旧表述，本实现按最终 contract B 和 design §2.4 使用 admin/user；implement 4.2 中旧的 `__sub2api/artifact-frame` 条目没有实现，按最终 contract F 使用 `/_chat/embed/artifact-frame.html` 精确路径设置 CSP。

## 补充（2026-10-24，联调修复）

按主会话更新后的 contract A、B.3、B.4 完成以下补充，本轮仅修改 `backend/internal/server/chat_proxy.go`、`backend/internal/server/chat_proxy_test.go`，并追加本节。

### 实现与回归覆盖

1. **姓名头 ASCII 化**：用户名（为空时仍取邮箱前缀）通过 `url.QueryEscape` 后将空格产生的 `+` 替换为 `%20`；只保留 RFC 3986 unreserved 字节 `A-Za-z0-9-._~`，其他 UTF-8 字节为大写 `%XX`，不改变原始用户名。HTTP 与 WebSocket 共用此身份头注入路径。
   - `TestChatProxyNameHeaderEncoding`：中文、首尾及中间空格、百分号本身（`100%20` → `100%2520`）、emoji、完整 unreserved 字符集、reserved 字符、邮箱前缀回退；断言实际上游收到的值，并确认客户端伪造姓名头被覆盖。
   - `TestChatProxyWebSocketNoKeys`：使用 `中文 100% 😀`，验证握手上游收到 `%E4%B8%AD%E6%96%87%20100%25%20%F0%9F%98%80`、101 升级与 echo 成功，仍不注入任何 Key 相关头。
2. **退出清双 Cookie**：DELETE 同时发送 `s2a_chat` 与 `token` 的清除 Cookie，均为空值、`Max-Age=0; Path=/_chat; HttpOnly; SameSite=Strict`；Secure 继续使用与创建会话相同的 TLS / `X-Forwarded-Proto` 判定。无需有效登录；未配置工作台时也先清除，再返回既有 503。
   - `TestChatProxyDeleteSessionClearsBothCookies`：18 个组合覆盖工作台启用/未配置、未登录/失效/有效登录、HTTP/直接 HTTPS/反代 HTTPS；逐个断言 Cookie 名称、值、Path、无 Domain、Max-Age、HttpOnly、SameSite、Secure 与 no-store。
3. **401 错误码**：保留主会话的 `CHAT_SESSION_INVALID` 修改。
   - `TestChatProxyInvalidSessionErrorCode`：缺失 Cookie、空 Cookie、无效 Cookie、仅携带 Open WebUI token，分别请求 API、WebSocket 路径、OAuth、自有 keys 接口；精确断言 HTTP 401 与 `{"error":{"code":"CHAT_SESSION_INVALID","message":"登录已失效"}}`，并断言上游调用次数为零。

### 验证结果

在 `backend/` 执行，额外设置 `GOPROXY=off` 防止依赖下载；使用本地缓存依赖，未访问外网目标。

- 定向回归：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./internal/server -run TestChat -count=1`，通过（0.151s，退出码 0）。
- 全量单测：`env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...`，通过（退出码 0；未变更包复用 Go 测试缓存）。
- 构建：`go build ./...`，通过（退出码 0）。
- 已执行 gofmt；`git diff --check` 通过。golangci-lint 本机未安装，未宣称 lint 通过；新增实现没有忽略返回 error 的写操作。
- 日志：`/tmp/chat-stage4-supplement-targeted.log`、`/tmp/chat-stage4-supplement-unit.log`、`/tmp/chat-stage4-supplement-build.log`。

### 联调与约定说明

无需新增环境变量或网络配置。阶段 6 请再次验证中文/emoji 用户名下真实 uvicorn WebSocket 握手，以及 A 账号退出切换到 B 后旧 Open WebUI token 不再残留；本轮仅在 Go 本地 httptest 服务验证，未操作容器或浏览器联调环境。

未修改 frontend、contract 或其他阶段文件，未执行 Git 写操作。此次没有新增约定变更提议；Open WebUI 侧应按已更新的 B.4 对姓名头做 UTF-8 百分号解码。
