-- CUSTOM(fork): 通道抽成不同于用户承担的 fee_rate。NULL 历史订单由报表显式兜底。
ALTER TABLE payment_provider_instances ADD COLUMN IF NOT EXISTS channel_fee_rate NUMERIC(6,4) NOT NULL DEFAULT 0 CHECK (channel_fee_rate BETWEEN 0 AND 0.2);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS channel_fee_rate NUMERIC(6,4) CHECK (channel_fee_rate BETWEEN 0 AND 0.2);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS refund_balance_deducted NUMERIC(20,8) CHECK (refund_balance_deducted >= 0);
