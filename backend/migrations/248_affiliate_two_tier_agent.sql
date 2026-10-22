-- 两级代理级差分润：绑定时冻结分润结构，奖励流水按类型区分。
--
-- user_affiliates：
--   agent_owner_id / agent_owner_level / agent_upline_id 是绑定时写死的分润结构快照：
--   归属代理、归属代理当时的档位（1 / 2）、归属代理为二级时其上方最近的一级代理。
--   之后任何人升级、降级都不改这三列。两列 ID 不建外键：账号删除后保留 ID，结算时按
--   users 判断是否仍存在，以区分「绑定时就没有一级」与「一级后来被删除」。
--   agent_level_updated_by 是最近一次改等级的操作人（管理员或任命的一级代理）。
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS agent_owner_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS agent_owner_level SMALLINT NULL,
    ADD COLUMN IF NOT EXISTS agent_upline_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS agent_owner_bound_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS agent_level_updated_by BIGINT NULL;

CREATE INDEX IF NOT EXISTS idx_user_affiliates_agent_owner_id ON user_affiliates(agent_owner_id);
CREATE INDEX IF NOT EXISTS idx_user_affiliates_agent_upline_id ON user_affiliates(agent_upline_id);

COMMENT ON COLUMN user_affiliates.agent_level IS '代理等级：1 一级代理，2 二级代理，3 普通用户（默认）';
COMMENT ON COLUMN user_affiliates.agent_level_source IS '等级来源：default/auto/manual/agent（由一级代理任命）';
COMMENT ON COLUMN user_affiliates.agent_owner_id IS '绑定时确定的归属代理（快照，不建外键）';
COMMENT ON COLUMN user_affiliates.agent_owner_level IS '归属代理在绑定时的档位（1/2）';
COMMENT ON COLUMN user_affiliates.agent_upline_id IS '归属代理为二级时，绑定时其上方最近的一级代理（快照，不建外键）';
COMMENT ON COLUMN user_affiliates.agent_owner_bound_at IS '归属确定时间';
COMMENT ON COLUMN user_affiliates.agent_level_updated_by IS '最近一次修改等级的操作人（管理员或一级代理）';

-- user_affiliate_ledger：reward_type 取代 246 引入的 rebate_level。
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS reward_type VARCHAR(24) NULL;

ALTER TABLE user_affiliate_ledger
    DROP COLUMN IF EXISTS rebate_level;

COMMENT ON COLUMN user_affiliate_ledger.reward_type IS '奖励类型：agent_direct/agent_diff/invite_percent/invite_fixed/invitee_bonus；非奖励流水为 NULL';
COMMENT ON COLUMN user_affiliate_ledger.action IS 'accrue|transfer|withdraw|invitee_bonus';

-- 首充奖励每个被邀请人只发一次，并发由唯一索引兜底。
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliate_ledger_first_recharge
    ON user_affiliate_ledger(source_user_id, reward_type)
    WHERE reward_type IN ('invite_fixed', 'invitee_bonus');

CREATE INDEX IF NOT EXISTS idx_user_affiliate_ledger_reward_type
    ON user_affiliate_ledger(user_id, source_user_id, reward_type);

-- 按层返利已删除，清理旧设置项。
DELETE FROM settings WHERE key IN ('affiliate_rebate_levels', 'affiliate_rebate_rate_l2');
