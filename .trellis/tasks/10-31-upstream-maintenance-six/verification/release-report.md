# 六项上游更新：推送与七号机发布

## 授权与版本
用户在知悉既有依赖审计及CSP/304问题后明确要求“推送、更新部署”。开始时工作区干净，已存在产品提交 `5957c600052bee92c7feaf5fd22b7af9647413df`，21个产品文件与候选指纹一致；直接快进推送origin/sub2api-custom，没有重复提交或改写历史。该现有提交包含已有的embedded-workbench七号机文档修正。origin/main未推送。

## CI与发布
- Custom Image **41182664896 全部通过**：后端unit/integration、golangci-lint、前端lint/typecheck/Vitest及镜像构建。govulncheck未运行；独立security-scan不在该工作流内。
- 固定镜像 `ghcr.io/qlqqs/sub2api:custom-5957c60`，digest `sha256:117884bf31a4d3c84e71f36e7e944d0ab6f793380e18737646ac96efe0bc6652`。拉取和启动后均核对完整OCI revision。
- 七号机 `/root/sub2api-custom` 从 `27d51f9b5` 更新为 `5957c6000`；只更新sub2api服务，使用`--no-deps --pull never`。
- 停止时间2026-10-31 08:17:21 UTC，发布检查完成08:17:41 UTC（北京时间16:17:41）；08:19:03 UTC复查healthy，restart count为0。
- 配置、迁移记录和所有其他运行容器名称/ID前后一致；没有重建PG、Redis、Nova、Open WebUI或egress。

## 备份与回退
备份目录 `/root/sub2api-custom/backups/pre-upstream-six-20261031T081719Z`，含PostgreSQL custom dump、应用data、compose/env和旧镜像归档。pg_restore目录、tar/gzip可读性及SHA256检查通过；回退脚本留在备份目录。本轮未触发回退，不恢复数据库覆盖线上数据。

## 上线检查
- 服务器回环只读检查：33次HTTP检查通过；首页/渠道页面及6个主站资源、Nova及11个资源正常；未认证管理/工作台/Chat接口401；客户端更新源200。
- 公网：健康GET、首页/渠道/Nova及新版JS的HEAD、未认证Chat GET，共7项通过。这是网络/资源头检查，不是公网浏览器交互验收。
- 早期公网全量资源GET探针下载超时；完整资源响应体由服务器回环检查验证，不把公网HEAD冒充全量下载。
- 未见启动panic/FATAL/迁移失败模式；没有创建测试用户、付费推理、业务配置或财务写入。上线不重跑生产业务全链路。

## 仍开放的质量项
原始及策略audit失败（follow-redirects/xlsx），正常缓存CSP/304 nonce问题均未修改。CI成功和本次发布不代表这些问题已解决。任务保留in_progress跟进，不归档为全绿。

实际发布证据：`release-verification.json`；原始日志及脱敏脚本在本机`/tmp/sub2api-six-release`。此前`final-report.md`/`final-verification.json`保留本地交付时间点事实；现行发布状态以本报告为准。收尾仅提交发布文档，运行镜像revision仍为5957c6000。

现有`docs/DEPLOYMENT_MIGRATION_20261029.md`已同步本机生产说明；该文件受仓库docs忽略规则约束，未强制纳入Git。可追踪发布证据保存在本任务目录。
