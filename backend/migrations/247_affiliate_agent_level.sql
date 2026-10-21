-- 代理等级与按消费结算返利。
-- agent_level：1 最高、3 最低（默认）。等级决定能获得几层返利：3 级无返利，2 级仅直属一层，1 级两层。
-- agent_level_locked：管理员手动降级后锁定，不再自动升级。
-- rebate_settled_at：按消费结算时已处理到的 usage_logs.created_at 水位；初始化为迁移时刻，历史消费不补发。
-- 用时间而不是 id 做水位，是因为异步批量写入会让 id 与提交顺序不一致，结算时留出时间滞后即可避免漏算。
-- rebate_settled_consumption：已计入返利的累计消费，用于限制「计入返利的消费 ≤ 真实充值」。
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS agent_level SMALLINT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS agent_level_locked BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS agent_level_source VARCHAR(16) NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS agent_level_updated_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS rebate_settled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS rebate_settled_consumption DECIMAL(20,8) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_agent_level ON user_affiliates(agent_level);

-- 消费结算产生的返利记录本次覆盖到的 usage_logs.created_at 上界，便于审计；充值来源的返利为 NULL。
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_usage_until TIMESTAMPTZ NULL;

COMMENT ON COLUMN user_affiliates.agent_level IS '代理等级：1 最高，3 最低（默认）';
COMMENT ON COLUMN user_affiliates.agent_level_locked IS '手动降级后锁定，不再自动升级';
COMMENT ON COLUMN user_affiliates.agent_level_source IS '等级来源：default/auto/manual';
COMMENT ON COLUMN user_affiliates.agent_level_updated_at IS '等级最近变更时间';
COMMENT ON COLUMN user_affiliates.rebate_settled_at IS '按消费结算返利已处理到的 usage_logs.created_at';
COMMENT ON COLUMN user_affiliates.rebate_settled_consumption IS '已计入返利的累计消费金额';
COMMENT ON COLUMN user_affiliate_ledger.source_usage_until IS '消费结算返利覆盖到的 usage_logs.created_at 上界';
