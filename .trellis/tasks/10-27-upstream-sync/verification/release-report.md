# 本轮推送与一号机部署结果

## 授权与版本

用户在收到提交及验证限制后明确要求“推送更新部署”。已原子推送 `origin/main=f178e5ae3`、`origin/sub2api-custom=1fa919f68290ce11f972eca62109ce43535de966`。

- 上游维护产品提交：`b95be64f6d06a9c617b4690ac1f0d50c78366d4d`。
- 本次构建/部署 revision：`1fa919f68290ce11f972eca62109ce43535de966`（含工作流记录提交）。
- 同分支此前已有的线下充值/赠送区分提交 `a18a7a3a4` 随本次推送发布，组合本机测试及本次 CI 均包含它；不将它归为本任务新增维护补丁。
- 只更新一号机 `/root/sub2api-custom` 主应用；Nova、Chat、PG、Redis和其他项目未重建。

## CI 与镜像

Custom Image **40546398450 全部成功**：backend-test（unit + integration）、backend-lint、frontend-test、image。此前待验证的 CI lint / integration 已由该发布运行补齐。本机原始依赖审计不因此变为通过，独立 security-scan 不在此 workflow 内。

- 镜像：`ghcr.io/qlqqs/sub2api:custom-1fa919f`。
- digest：`sha256:18e4830d70c6ccaf1ae54af611f9f5b7f0962b351a4ca77ec40c9c050377aa07`。
- 拉取与启动后均核对完整 OCI revision；compose 使用 `--no-deps --pull never`，未盲拉 mutable tag。

## 备份与发布

- 备份：`/root/sub2api-custom/backups/pre-upstream-maintenance-20261027T171959Z`，权限受限；含 Postgres custom dump、应用 data、compose/env、旧镜像ID/旧revision。
- pg_restore 目录读取、tar目录读取及 SHA256 校验通过；备份及凭据未复制到 Git。
- 停应用：2026-10-27 17:19:59 UTC；健康确认：17:20:07 UTC（北京时间 2026-10-28 01:20:07），约8秒。
- 与旧生产 `99ab810ed` 相比无迁移文件变化；没有执行历史重算或修改业务配置。
- 其他所有运行容器的名称/ID快照前后一致；不是仅凭 compose 参数推断。
- 回退准备：旧镜像ID已存备份，脚本含应用镜像回退；本轮未触发回退、未恢复数据库。

## 上线只读验证

- /health、首页、Nova入口200；首页6个、Nova11个同源静态资源200。
- 财务重算列表、Chat入口、用户工作台API及Chat元数据未认证401。
- 已发布财务JS的取消预览/重新预览控件存在；健康状态healthy、revision准确，未发现 panic/FATAL/迁移失败日志模式。
- 未创建生产测试用户、未做付费推理、未做扣费全链路操作；不宣称生产业务全链路已测试。

## 保留限制与记录

本机e2e仍 **394/395、FIN-24失败**；原因与原始结果保留。原始依赖审计仍exit1（2 high、13 moderate、2 low），既有例外策略通过，未扩展例外。发布不伪造这些结果已修复。

实际发布摘要：`release-verification.json`；原始CI/部署/只读检查日志在本机 `/tmp/s2a-upstream-release/`，部署脚本及部署日志在远端 `backups/upstream-maintenance-release-logs/`。收尾文档提交仅更新 `.trellis/`，不会重新构建应用；运行revision继续是本报告列出的发布SHA。

本轮已更新长期state/decisions/README、工作流、PRD/design/implement/task和现有journal/index。旧验证/提交报告保留各自时间点；现行发布事实以本记录为准。任务保留in_progress用于既有测试/审计跟进与未演练流程场景，不把发布完成等同于这些待办完成。
