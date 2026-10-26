-- Fork-owned defaults: bind the existing deployment group once, then use its ID.
INSERT INTO settings (key, value, updated_at)
SELECT 'signup_workbench_group_id', MIN(id)::text, NOW()
FROM groups WHERE deleted_at IS NULL AND name = 'Codex 稳定分组'
HAVING COUNT(*) = 1
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS workbench_signups (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS workbench_preferences (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workbench TEXT NOT NULL CHECK (workbench IN ('studio', 'chat')),
    preferences JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workbench)
);
