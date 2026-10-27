# 上游增量维护记录

工作流入口：`../spec/guides/upstream-sync-guide.md`。

- `decisions.jsonl`：追加事件是事实来源；按 unit_sha 聚合，后续事件更新状态而不覆盖历史。
- `state.json`：当前摘要，可与事件和 Git 交叉核对。
- 最近任务：`../tasks/10-27-upstream-sync/`。

**审查游标仅表示范围决定覆盖，并非所有补丁已技术审查/测试。** 查看 review_depth；选择实施 scope_triage_only 项时补技术审查。selected_partial 只采用所列源提交，其余仍延期。

## 无需重复阅读的增量检查

1. 先核对 source repo、schema、source/custom 基线和最新事件序号。
2. 获取已授权的新目标后，检查旧游标是新目标祖先；否则停止。
3. 从 first-parent 新单元中剔除已有同版本有效决定的项，再加入达到重开条件的旧项。
4. 独立显示 selected 未集成、deferred、delegated_out_of_scope 队列；不能只看游标。
5. 无变化时不新增 review_decision。实际验证/移入/提交/回退用新事件关联 unit_sha。

本目录不保存凭据、原始请求/用户数据。测试报告保存在任务目录；敏感或体积大的原始 e2e 状态只留隔离本机目录，引用脱敏摘要。

## 当前批次状态

- 冻结上游/本地 main：`f178e5ae32659e8901644a9f2378a0439ffa36ae`；35 个 first-parent 单元已有范围决定，非全部技术审查。
- M1–M4 已本地提交到 custom（14 产品文件）：`b95be64f6d06a9c617b4690ac1f0d50c78366d4d`；**未推送、未部署**。
- 后端与前端单元/构建/vet通过；完整本机 e2e **394/395、FIN-24 FAIL**；原始生产依赖审计 **exit 1**，不能称质量门禁全绿。
- 事实摘要与下一步：`../tasks/10-27-upstream-sync/verification/commit-report.md`。恢复时先处理本批质量跟进及后续交付授权，不重复生成相同范围决定；新上游仍从游标之后开始。
