-- 管理员为二级代理设置上级一级代理。
--
-- user_affiliates：
--   inviter_was_agent：绑定时邀请人是否为代理（即归属代理就是邀请人），绑定时写死。
--   普通邀请奖励与邀请人首充奖励只在它为 false 时发放。之前这一点由「agent_owner_id = inviter_id」
--   推断；管理员改归属后该推断不再成立，因此单独存一列，并按旧推断规则回填。
--   agent_owner_set_by：最近一次由管理员改归属的操作人 ID（绑定写入时为 NULL）。
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_was_agent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS agent_owner_set_by BIGINT NULL;

UPDATE user_affiliates
SET inviter_was_agent = TRUE
WHERE inviter_id IS NOT NULL
  AND agent_owner_id IS NOT NULL
  AND agent_owner_id = inviter_id
  AND inviter_was_agent = FALSE;

COMMENT ON COLUMN user_affiliates.inviter_was_agent IS '绑定时邀请人是否为代理（是则不发普通邀请奖励与邀请人首充奖励）';
COMMENT ON COLUMN user_affiliates.agent_owner_set_by IS '最近一次由管理员设置归属代理的操作人 ID';
