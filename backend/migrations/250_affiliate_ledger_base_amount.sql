-- 返利流水记录计算基数。
--
-- user_affiliate_ledger：
--   base_amount：这笔返利按多少基数计算——充值返利 / 首充奖励为到账充值额，消费结算为本次
--   计入返利的余额消费。费率会被调整，无法从返利金额反推，因此入账时写死。
--   历史流水无法可靠回填，保持 NULL。
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS base_amount NUMERIC(20,8) NULL;

COMMENT ON COLUMN user_affiliate_ledger.base_amount IS '返利计算基数（充值额或计入返利的消费），历史流水为 NULL';
