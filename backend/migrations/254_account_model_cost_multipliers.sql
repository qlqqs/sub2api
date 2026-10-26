-- CUSTOM(fork): 按上游模型覆盖账号成本倍率；只加配置表，不改变扣费和调度。
CREATE TABLE IF NOT EXISTS account_model_cost_multipliers (
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model_pattern VARCHAR(255) NOT NULL,
    rate_multiplier NUMERIC(10,4) NOT NULL CHECK (rate_multiplier >= 0 AND rate_multiplier <= 10000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, model_pattern)
);
COMMENT ON TABLE account_model_cost_multipliers IS '账号模型成本倍率，精确优先、最长前缀通配';
