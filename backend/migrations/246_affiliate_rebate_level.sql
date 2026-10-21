-- 多级代理返利：记录每笔返利流水对应的代理层级。
-- 1 = 充值用户的直接邀请人，2 = 邀请人的邀请人，以此类推。
-- 迁移前的 accrue 流水全部来自直接邀请人，回填为 1；非 accrue 流水保持 NULL。
ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS rebate_level SMALLINT NULL;

UPDATE user_affiliate_ledger
SET rebate_level = 1
WHERE action = 'accrue'
  AND rebate_level IS NULL;

COMMENT ON COLUMN user_affiliate_ledger.rebate_level IS '返利对应的代理层级（1=直属邀请人，2=二级，3=三级）；非返利流水为 NULL';
