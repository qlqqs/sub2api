# 上游增量维护记录

工作流入口：`../spec/guides/upstream-sync-guide.md`。

- `decisions.jsonl`：追加事件是事实来源；按 unit_sha 聚合，后续事件更新状态而不覆盖历史。
- `state.json`：当前摘要，可与事件和 Git 交叉核对。
- 最近批次：`../tasks/10-31-upstream-maintenance-six/`；父任务 `../tasks/10-27-upstream-sync/`。

**审查游标仅表示范围决定覆盖，并非所有补丁已技术审查/测试。** 查看 review_depth；选择实施 scope_triage_only 项时补技术审查。selected_partial 只采用所列源提交，其余仍延期。

## 无需重复阅读的增量检查

1. 先核对 source repo、schema、source/custom 基线和最新事件序号。
2. 获取已授权的新目标后，检查旧游标是新目标祖先；否则停止。
3. 从 first-parent 新单元中剔除已有同版本有效决定的项，再加入达到重开条件的旧项。
4. 独立显示 selected 未集成、deferred、delegated_out_of_scope 队列；不能只看游标。
5. 无变化时不新增 review_decision。实际验证/移入/提交/回退用新事件关联 unit_sha。

本目录不保存凭据、原始请求/用户数据。测试报告保存在任务目录；敏感或体积大的原始 e2e 状态只留隔离本机目录，引用脱敏摘要。

## 2026-10-27 历史批次状态

- 冻结上游/本地及 origin/main：`f178e5ae32659e8901644a9f2378a0439ffa36ae`；35单元有范围决定，非全部技术审查。
- M1–M4产品提交 `b95be64f6d06a9c617b4690ac1f0d50c78366d4d`；发布revision `1fa919f68290ce11f972eca62109ce43535de966` 已推送并部署一号机sub2api-custom。
- CI 40546398450 unit/integration/lint/前端/镜像成功；生产只读健康/资源/鉴权通过，其他容器未重建。
- 保留：本机e2e **394/395、FIN-24 FAIL**；原始生产依赖审计 **exit 1**。不因发布成功抹去这些结果。
- 当前证据与后续：`../tasks/10-27-upstream-sync/verification/release-report.md`。下次从游标之后新单元及触发重开的旧项继续，不重复范围处置。

## 当前批次：2026-10-31
- 审查游标/本地main=d851a3847，46个first-parent单元有范围决定；origin/main未随本批推送。
- 六项集成为5957c6000并推送custom；CI 41182664896全通过，七号机已部署同一revision，健康及限定只读验收通过。
- 本机e2e442/442；原始/策略audit及正常缓存CSP失败仍开放，不冒充全绿。
- 发布证据：`../tasks/10-31-upstream-maintenance-six/verification/release-report.md`。旧批次历史保留。
