-- 代理团队消费页按时间范围汇总收益：WHERE user_id = ? AND created_at 区间，仅索引扫描。
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_user_created
    ON user_affiliate_ledger (user_id, created_at) INCLUDE (source_user_id, reward_type, amount);
