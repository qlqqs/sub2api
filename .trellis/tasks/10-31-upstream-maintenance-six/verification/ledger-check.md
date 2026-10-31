# 台账及落地一致性检查

- seq连续且event_id唯一；本批11个first-parent范围单元均有明确决定，源diff SHA256全部匹配。
- 历史至本批共46个单元都有范围处置；同一固定target再查新增队列为空。延期项保留，不重开旧范围。
- 六项当前均applied_in_custom_worktree，target_commits为空；没有伪造integrated/pushed/deployed。
- 合并review_decision及后续progress后，原范围、深度、来源和重开条件仍在。
- 主/候选21产品文件哈希一致；HEAD未变，index无暂存；原有工作台guide指纹不变。
- 当前工作区冻结安装和typecheck均exit0，pnpm锁与候选一致。
- 本轮scope记录完整，质量门禁仍有基线audit和正常缓存CSP失败；不宣称通过。
