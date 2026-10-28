# 注册赠送发布记录（2026-10-28 UTC）

## 授权与版本

用户明确要求「推送、等 CI 构建完再更新一号机」。开始本轮时功能已提交，工作区干净；直接推送 `origin/sub2api-custom`，未重复提交产品代码。

- 产品与运行 revision：`1dafd13b7fa2645e54b2b9d07ad24c60840f488c`（Add invitee signup bonus）。
- 原生产 revision：`1fa919f68290ce11f972eca62109ce43535de966`。
- CI：Custom Image `40624930942`，backend-test（unit + integration）、backend-lint、frontend-test、image 全部成功。
- 镜像：`ghcr.io/qlqqs/sub2api:custom-1dafd13`。
- 镜像 digest：`sha256:99c7ce5837ec1afea459edd1d2fa44b52d703fa043f15000ef2964058a3b2fa7`。
- 拉取及运行后核对完整 OCI revision，不依赖 mutable latest 判断版本。

## 备份与更新边界

仅操作一号机 `/root/sub2api-custom` 主应用，使用 `docker compose up -d --no-deps --pull never sub2api`。部署锁与旧 revision 前置检查通过。

- 备份目录：`/root/sub2api-custom/backups/pre-invitee-signup-bonus-20261028T032353Z`，权限受限。
- 包含 PostgreSQL custom dump、应用 data、原 compose/env、旧镜像 ID 和 revision。pg_restore/tar 目录读取及 SHA256 校验通过；凭据及备份未进入 Git。
- 停应用：03:23:54 UTC；健康确认：03:24:02 UTC，约 8 秒。
- 新迁移：`262_affiliate_invitee_signup_bonus.sql`，新增独立唯一索引及列注释；索引 valid/unique 均验证通过。
- schema_migrations checksum 与源码按迁移器 TrimSpace 后的 SHA256 一致：`6202b45c44a4fff61afbfb086749498dab70e6539ebb1c0ad035d418d2134137`。
- 所有其他运行容器的名称/ID快照前后一致，包括 PG、Redis、Nova、Chat 和无关部署。
- 未改 compose/env、监听、网络、反代、认证或业务配置；应用仍为既有 `0.0.0.0:18080 → 8080`。
- 已准备旧应用镜像回退，本轮未触发。迁移为增量索引，旧版应用兼容；未恢复或重算生产数据。

## 验证

- 本机既有验证证据已复核：fast 221/221、独立 p10 34/34、前端 3498 项、后端单元/集成/构建通过；CI 补齐后端 lint 与全量集成检查。
- 生产 loopback、`https://www.qlqqx.com`、`https://api.qlqqx.com` 只读 smoke 通过：健康/首页/注册/邀请/设置页面、主站6个及 Nova11个同源资源200；需认证的财务/工作台/Chat接口401；公开设置包含注册赠送字段且为0。
- API 域名首次本机直连 TLS handshake 被 reset；通过已有 shell 网络代理完整重试通过，TLS 证书校验未关闭，生产网络设置未修改。此网络尝试失败不记为首次通过。
- 应用 healthy，无 panic/FATAL/迁移失败日志模式。
- 注册赠送仍为 **0（未配置/关闭）**，没有擅自开启。
- 生产仅只读验证，没有创建测试用户、发放赠送或执行付费请求；不宣称生产业务全链路实测。
- 本任务未重跑完整本机 e2e p5/p6/p8/p9；不改变其他任务已有失败或依赖审计结论。

## 证据与收尾

本机证据：`/tmp/s2a-signup-release/`（CI JSON/watch、preflight、deploy、生产与双域名 smoke）。远端脚本及部署日志：`/root/sub2api-custom/backups/signup-bonus-release-logs/`。

本任务归档及 journal 提交只更改 `.trellis/`，按现有工作流 paths-ignore 不重建镜像；运行版本保持上述产品 SHA。其他任务不归档。
