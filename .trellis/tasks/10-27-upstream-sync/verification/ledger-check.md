# 台账检查

通过：35 个 first-parent 单元覆盖、唯一事件/连续 seq、所有源 merge diff SHA256、所选子提交覆盖、证据路径、重复查询新审查队列为空、模拟一个缺口仅列缺项且连续游标停止在缺口之前。

状态计数：{"selected": 5, "deferred": 21, "selected_partial": 1, "skipped": 4, "delegated_out_of_scope": 4}

上述检查使用 Python 标准库和只读 Git 命令，没有新增自动化脚本；不代表人工重开条件能自动识别。所有 target_commits 当前为空，未虚构 custom 集成提交。

## 本轮收尾复核

45条事件（保留原37条，追加验证、6单元工作区移入及记录同步）；35个审查单元仍完整且不重复。逐一重新校验源diff SHA256、子提交覆盖、证据路径、连续序号、重复空队列、模拟缺口停止。6个选中单元均为 applied_in_custom_worktree，target_commits仍为空。当前/候选14文件指纹一致、main/HEAD与记录一致、索引为空、diff --check通过；任务保持in_progress。e2e/audit失败与后续门禁已写摘要与现有journal，非集成/发布完成。

## 本地提交后复核

用户批准提交后，6条选入单元追加 integrated 映射到 b95be64f6d06a9c617b4690ac1f0d50c78366d4d，产品提交14个blob与已测指纹一致；原工作区记录与提交前事件保留历史含义。52条事件连续唯一，源范围处置仍35项，台账/任务/journal一致。CI与质量限制未变，未推送或部署。

## 发布后复核

62条事件连续唯一；6个选中单元保留原始source审查，并追加pushed/deployed及固定release_head。CI与生产证据匹配1fa919f68；origin/main=f178e5ae3；目标产品提交b95be64f6的14文件与测试指纹一致。现有state、README、task及journal已更新发布事实；旧事件和验证失败保持历史，不改写。
