# 实施步骤

## 门禁

- 用户已明确回复「开始」，已执行 `task.py start`。
- 开始编辑前使用 trellis-before-dev 加载后端/前端相关规范；当前主 agent inline 实施，不派发子代理。

## 实施顺序

1. 全量搜索 `affiliate_invitee_signup_bonus`、`InviteeSignupBonus`、绑定结果的所有构造及 fake/mock，记录实际影响文件。
2. 扩展统一奖励配置、常量、设置 DTO/读写/校验/审计、公开类型及兼容逻辑；补缺省、非法枚举、旧请求省略字段测试。
3. 由服务层事务包装附上绑定身份快照，新增来源匹配并接入赠送；补代理/普通/代理名下普通用户、自动升级、OAuth、幂等回归。
4. 邀请 detail 按本人身份筛选金额；新增匿名只读预览服务、handler、限流路由及测试，不泄露用户数据。
5. 后台复用现有 Select 组件新增来源字段；更新 API 类型、表单读取/提交、i18n。注册页接入预览并处理加载/失败/请求竞争；我的邀请文案按 PRD 修改。
6. 扩展前后端测试；本机 e2e 基线重置新字段为 all，新增六场景与动态配置、历史不变、UI 刷新用例，更新本机 CASES.md。
7. trellis-check 质量检查，确认仅预期文件变化和规划需求覆盖。

## 验证

在 backend：
```bash
env -u http_proxy -u https_proxy -u HTTP_PROXY -u HTTPS_PROXY -u OPENAI_API_KEY go test -tags unit ./...
go test -tags=integration ./internal/repository/ -run Affiliate
go build ./...
```
在 frontend（不使用全局 pnpm）：
```bash
npx -y pnpm@9 run lint:check
npx -y pnpm@9 run typecheck
npx -y pnpm@9 run test:run
```
在 `/home/qlqq/workspace/sub2api-local`：
```bash
./build.sh && docker compose up -d
cd e2e && ./run_all.sh --fast
```
随后执行新增针对性来源筛选/提示 UI 用例，刷新验证 5→10 及切换来源、历史金额保持原值。Playwright 按本机说明加载依赖环境，凭据仅取本地环境，不输出密码。前后检查 git status，确认锁文件未变。

## 高风险审查点

- 不拿上级归属身份冒充直接邀请人身份，不拿自动升级后的身份代替绑定快照。
- 不在旧客户端省略字段时重置管理员选择。
- 不让公开 settings 的全站金额绕过来源筛选；请求乱序不残留错误提示。
- 不修改原奖励账本、已领取金额与财务分类；重复发放测试必须仍有效。
- 不使用远端环境、不全局清理 Docker；提交/推送/部署均需另外获准。

## 实际影响文件与交付

- 后端：统一奖励配置、settings 键/视图/默认值/更新、管理设置 DTO/handler/审计、绑定返回快照、邀请 detail、匿名预览 service/handler/auth 路由；相关 service/handler/API contract 测试。
- 前端：管理 settings 类型/表单、auth 预览 API、RegisterView、两语言三处文案；注册页/API/设置页测试。
- specs：后端赠送来源契约、前端异步提示契约。
- 本机仓库外测试资产与详细结果见 `verification.md`。
- 复用原表和仓储事务，无 migration、Ent 或 Wire 变更。
