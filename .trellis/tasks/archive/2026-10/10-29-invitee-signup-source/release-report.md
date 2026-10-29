# 注册赠送邀请来源发布记录（2026-10-29）

## 授权、提交与 CI

用户明确要求「提交推送部署」。本次提交全部来自该任务，未混入其他工作；推送到 `origin/sub2api-custom`，无 force push。

- 功能/运行 revision：`2ffc49db745c01351d0cef662c60053d7aebcfed`，提交 `Add invitation source filter for signup gifts`。
- 原生产 revision：`1dafd13b7fa2645e54b2b9d07ad24c60840f488c`。
- Custom Image CI：`40827319767`，frontend-test、backend-test（unit + integration）、backend-lint、image 四项全部成功。
- 镜像：`ghcr.io/qlqqs/sub2api:custom-2ffc49d`。
- digest：`sha256:1cc744674cbca11a06c11270a2324114396d6ba20f3c5076cb0fccc78b52d234`。
- 按不可变提交标签拉取，拉取和启动后都核对完整 OCI revision；不凭 mutable latest 推断运行版本。

## 部署与备份

仅操作一号机 `/root/sub2api-custom` 主应用。部署锁、旧 revision、Compose 项目目录、服务镜像以及无进行中财务重算的前置检查均通过。

- 备份：`/root/sub2api-custom/backups/pre-invitee-signup-source-20261029T062225Z`，目录700/文件受限。
- 含 Postgres custom dump、应用 data、原 compose/env、旧镜像 ID/revision；pg_restore 列表、tar列表与 SHA256 校验通过。备份及凭据未进入 Git。
- 停应用：06:22:25 UTC；健康恢复：06:22:36 UTC（北京时间14:22:36），约11秒。
- 更新命令：`docker compose up -d --no-deps --pull never sub2api`。
- 所有其他运行容器名称/ID前后相同，包括 PostgreSQL、Redis、Nova、Chat及无关项目。
- compose/env SHA256 前后相同；没有修改监听、网络、反代或认证配置。
- 本次无新增迁移，源码迁移差异为空，生产 schema_migrations 数量前后相同。
- 已准备旧镜像回退路径，本轮未触发回退，也未恢复/重算生产数据。

## 业务设置保持不变

上线前后核对有效设置：

- 邀请返利：开启。
- 注册赠送：`0.50000000`，即 ¥0.50。
- 邀请来源：`all`，即全部邀请（缺省兼容）。

没有开启新的奖励或擅自切换邀请来源；管理员可在新下拉框中自行选择。

## 上线验证

- 生产 loopback、主站 `www.qlqqx.com`、API域名 `api.qlqqx.com` 的只读 smoke 全部通过。
- 健康、首页、注册页、邀请页、设置页和Nova入口200；主站6项和Nova11项同源静态资源均200。
- 设置和注册功能 chunk 返回200，设置 chunk 包含新来源字段/控件，注册 chunk 包含赠送提示。
- 匿名调用 `/api/v1/auth/affiliate-signup-bonus`，无效邀请码仅返回 `{amount:0}`，没有注册/绑人/发放副作用；公开设置金额为0.5。
- 管理设置、用户邀请、财务、工作台/Chat受保护接口仍未认证401。
- 应用 healthy、revision准确；启动日志未发现 panic/FATAL/迁移失败模式。
- 公网检查使用本机已有网络代理并保持 TLS 校验；未修改生产网络设置。
- 生产没有新建测试用户、支付或发放赠送；业务矩阵/界面验证以本机验收为准，不宣称生产付费全链路实测。

## 证据与收尾

- 本机CI/部署/生产与双域名检查：`/tmp/s2a-signup-source-release/`。
- 远端脚本、部署日志及启动日志：`/root/sub2api-custom/backups/signup-source-release-logs/`。
- 本机完整质量与验收结果见 `verification.md`（前端3512、fast e2e252、来源专项31、UI11）。CI补齐后端lint和全量集成检查；不改变未运行完整p5/p6/p8/p9的事实。
- 后续归档/journal提交仅修改 `.trellis/`，由工作流 paths-ignore 跳过镜像重建；线上应用保持上述功能SHA。其他活动任务不归档。
