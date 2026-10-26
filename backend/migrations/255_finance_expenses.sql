-- CUSTOM(fork): 财务手工支出；软删除保留审计线索。
CREATE TABLE IF NOT EXISTS finance_expenses (
    id BIGSERIAL PRIMARY KEY,
    occurred_on DATE NOT NULL,
    category VARCHAR(32) NOT NULL CHECK (category IN ('upstream_subscription','server','domain','other')),
    amount NUMERIC(20,2) NOT NULL CHECK (amount > 0),
    amortize_days INT NOT NULL DEFAULT 1 CHECK (amortize_days BETWEEN 1 AND 3660),
    account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    note VARCHAR(500) NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_finance_expenses_occurred_on ON finance_expenses(occurred_on) WHERE deleted_at IS NULL;
