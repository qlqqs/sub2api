# 联合发布记录（2026-10-26）

## 授权与范围
用户明确授权“提交 推送剩下的，一起推送更新部署”。发布仅针对一号机 `sub2api-custom`、`sub2api-custom-nova`、`sub2api-custom-chat`，不操作同机其他部署，不执行财务历史重算。

## 功能提交
- sub2api：`1a2b2736d`；包含此前财务 `9d87d64cd` 及默认模型客户端导入修正 `ab177d1c7`。
- Nova：`8d649ed`。
- Open WebUI：`550b06b`。
- `design-previews/` 属其他任务，未提交。

## CI 阻断与最小修复
首次 sub2api Custom Image `40343360950` 的 backend-lint 报7项问题，阻止镜像发布：
- `dashboard_cache_fence_test.go` 3处类型断言未检查：改为 comma-ok + require.True。
- 两个财务标签校验函数：以等价 De Morgan 表达式替换否定组合条件。
- channel repository 两个已无运行时调用的包装方法：删除包装，保留原SQL实现，原unit测试直接调用同一底层函数。

不修改Wire、迁移、SQL语义、成本算法或线上数据；不增加lint豁免。使用与CI相同的golangci-lint v2.14.0，在 `/tmp/s2a-tools/lint/` 临时安装，不改仓库依赖。

修复验证：
- 定向 repository/admin unit通过。
- Finance/Recalculation/Workbench真实PG integration通过（11.244s）。
- `go build ./...` 通过。
- 本机 `build.sh --no-web` + compose更新通过，主服务healthy。
- golangci-lint v2.14.0全量检查通过（0 issues）。
- 后续CI、线上备份和部署结果在发布完成后补充。

## 最终CI
- sub2api `7beeb73fed8fdad639f11a9b41a70cc5b1d1dfed`：Custom Image `40344861707` 全绿（全量unit、integration、golangci-lint、前端lint/typecheck/Vitest、镜像）。
- Nova `8d649ed5eac26930e18326490cf0a4cd455cb9f1`：Custom Image `40343363686` 全绿。
- Open WebUI `550b06b01e68f83c9473faf033a9e67140f27112`：Custom Image `40343371028` 全绿。
- CI定义的检查全绿不代表OWUI全量svelte-check或Nova独立测试类型检查已修复，原限制继续有效。

## 备份与部署
- 使用不可变 `custom-<sha7>` 拉取并校验OCI revision，切换本地custom-latest，compose使用 `--pull never --no-deps`，没有更新数据库/Redis/egress镜像。
- 停应用后完成PostgreSQL custom-format dump及三个应用data目录tar；`pg_restore --list`、tar列表和SHA256验证通过。
- 备份：`/root/sub2api-custom/backups/pre-workbench-20261026T151851Z`，权限受限，含旧镜像ID及实际存在的compose/env文件。备份不得上传Git。
- 首次预检查发现Nova无.env，停服务前退出；改为只复制实际存在的.env后重试。
- 切换遇到专用chat网络IP冲突：主服务被停止/重建后，先启动的动态地址Chat占用了主服务固定 `192.168.241.2`，连旧镜像恢复也被同一冲突阻断。仅移除无状态Chat容器（未删data），先启动主服务取得固定地址，再启动Chat/Nova，成功恢复并完成新版本部署。未删网络、未全局清理、未操作其他项目。
- 后续联合更新必须先启动持有固定可信代理IP的主服务，再启动动态IP工作台；不要为了绕过冲突改可信代理CIDR。

## 线上只读验收
- 三个运行镜像的OCI revision与上述目标完全一致。
- 主服务、Chat healthy；Nova running且内部页面200。
- `/health`、首页、`/_nova/` 为200；首页6个及Nova11个同源脚本/样式资源全部200。
- 未认证的Chat入口、workbench用户API及Chat元数据API均401，未绕过身份校验。
- 迁移259/260已应用；workbench两张表存在；`signup_workbench_group_id=2`，对应既有 `Codex 稳定分组`。
- 线上未创建测试用户、未发付费模型请求、未改财务成本规则、未执行历史重算。首次注册及真实请求业务验证仍以本机隔离证据为准，不把只读健康检查宣称为生产全链路验收。

本机发布日志在 `/tmp/s2a-workbench-release/`；远端受限日志在 `/root/sub2api-custom/backups/workbench-release-logs/`。原自动脚本的exit-code=1保留作为网络失败证据；后续network-recovery及production-smoke记录最终成功，不能仅凭首次脚本状态判断当前部署。
