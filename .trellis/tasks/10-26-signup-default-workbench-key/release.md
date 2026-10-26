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
