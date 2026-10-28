-- 注册赠送与首充奖励独立幂等：单独索引，不改变旧奖励的冲突索引推断。
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_affiliate_ledger_signup_bonus_once
    ON user_affiliate_ledger (source_user_id, reward_type)
    WHERE reward_type = 'invitee_signup_bonus';

COMMENT ON COLUMN user_affiliate_ledger.reward_type IS '奖励类型：agent_direct/agent_diff/invite_percent/invite_fixed/invitee_bonus/invitee_signup_bonus；非奖励流水为 NULL';
COMMENT ON COLUMN user_affiliate_ledger.action IS 'accrue|transfer|withdraw|invitee_bonus|invitee_signup_bonus';
