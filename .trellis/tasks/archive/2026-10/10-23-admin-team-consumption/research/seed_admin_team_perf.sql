-- 管理端团队消费页性能造数（仅用于本机 sub2api-local，勿在一号机执行）。规模见 design.md 第 7 节：
--   2 万用户：10 个一级代理（perf-00001..00010），各 5 个二级（perf-00011..00060）；
--   其余 19,940 人全部是渠道客户：n % 3 = 0 归属某个二级（上方一级为该二级的上级），其余直属某个一级；
--   2000 活跃客户（perf-00061..02060），usage_logs 90 天、每人每天 15 条余额消费；
--   分润流水按「每小时结算」：每个活跃客户 90 天里每天约 11 个小时有消费，每次结算给归属代理一条 agent_direct，
--   有上方一级时再给一级一条 agent_diff，共约 264 万行。
-- 另造边界：perf-00060 降级为普通用户（老客户仍归属他），perf-00059 软删除，
--   perf-00061..00070 在分润之后改挂到 perf-00010（原受益人的行不再计入这部分分润）。
-- 用法：在执行迁移 252 之前的库上运行（流水写入不经过 252 的触发器），随后启动新镜像，迁移内回填：
--   docker compose exec -T postgres psql -U sub2api -d sub2api -v ON_ERROR_STOP=1 < seed_admin_team_perf.sql
\timing on
BEGIN;

INSERT INTO users (email, password_hash, username, created_at, updated_at)
SELECT 'perf-' || lpad(g::text, 5, '0') || '@perf.local', 'x',
       CASE WHEN g % 3 = 0 THEN '' ELSE '客户' || g END, NOW(), NOW()
FROM generate_series(1, 20000) AS g;

CREATE TEMP TABLE perf_users AS
SELECT substring(email from 6 for 5)::int AS n, id FROM users WHERE email LIKE 'perf-%@perf.local';
CREATE UNIQUE INDEX ON perf_users (n);
ANALYZE perf_users;

CREATE TEMP TABLE perf_roles AS
SELECT p.n, p.id,
       CASE WHEN p.n <= 10 THEN 1 WHEN p.n <= 60 THEN 2 ELSE 3 END AS level,
       CASE WHEN p.n <= 10 THEN NULL
            WHEN p.n <= 60 THEN 1 + (p.n - 11) / 5
            WHEN p.n % 3 = 0 THEN 11 + p.n % 50
            ELSE 1 + p.n % 10 END AS owner_n,
       CASE WHEN p.n > 60 AND p.n % 3 = 0 THEN 1 + (p.n % 50) / 5 END AS upline_n
FROM perf_users p;

INSERT INTO user_affiliates (user_id, aff_code, agent_level, inviter_id, agent_owner_id, agent_owner_level,
                             agent_upline_id, agent_owner_bound_at, created_at, updated_at)
SELECT r.id, 'PF' || r.n, r.level, o.id, o.id,
       CASE WHEN o.id IS NULL THEN NULL WHEN r.owner_n BETWEEN 11 AND 60 THEN 2 ELSE 1 END,
       u.id, CASE WHEN o.id IS NULL THEN NULL ELSE NOW() - interval '120 days' END, NOW(), NOW()
FROM perf_roles r
LEFT JOIN perf_users o ON o.n = r.owner_n
LEFT JOIN perf_users u ON u.n = r.upline_n
ON CONFLICT (user_id) DO UPDATE
SET agent_level = EXCLUDED.agent_level, inviter_id = EXCLUDED.inviter_id, agent_owner_id = EXCLUDED.agent_owner_id,
    agent_owner_level = EXCLUDED.agent_owner_level, agent_upline_id = EXCLUDED.agent_upline_id,
    agent_owner_bound_at = EXCLUDED.agent_owner_bound_at;

-- usage_logs 的外键依赖：两个分组、一个上游账号、一个 API Key（挂在第一个客户名下，只为满足外键）。
CREATE TEMP TABLE perf_infra AS
WITH g1 AS (INSERT INTO groups (name) VALUES ('perf-group-a') RETURNING id),
     g2 AS (INSERT INTO groups (name) VALUES ('perf-group-b') RETURNING id),
     acc AS (INSERT INTO accounts (name, platform, type) VALUES ('perf-upstream', 'anthropic', 'apikey') RETURNING id),
     k AS (INSERT INTO api_keys (user_id, key, name) SELECT id, 'sk-perf-admin-team', 'perf' FROM perf_users WHERE n = 61 RETURNING id)
SELECT (SELECT id FROM g1) AS group_a, (SELECT id FROM g2) AS group_b, (SELECT id FROM acc) AS account_id, (SELECT id FROM k) AS api_key_id;

CREATE TEMP TABLE perf_active AS
SELECT r.n, r.id, o.id AS owner_id, u.id AS upline_id
FROM perf_roles r
JOIN perf_users o ON o.n = r.owner_n
LEFT JOIN perf_users u ON u.n = r.upline_n
WHERE r.n BETWEEN 61 AND 2060;
ANALYZE perf_active;
COMMIT;

-- usage_logs：逐天插入，每天 2000 × 15 = 3 万行。
DO $$
DECLARE
    day_start TIMESTAMPTZ;
    d INT;
BEGIN
    FOR d IN 0..89 LOOP
        day_start := date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai' - make_interval(days => d);
        INSERT INTO usage_logs (user_id, api_key_id, account_id, model, requested_model, group_id,
                                input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
                                total_cost, actual_cost, billing_type, created_at)
        SELECT a.id, i.api_key_id, i.account_id,
               (ARRAY['claude-sonnet-4-5', 'gpt-5', 'gemini-2.5-pro', 'claude-haiku-4-5'])[1 + (a.n + k) % 4] || '-up',
               (ARRAY['claude-sonnet-4-5', 'gpt-5', 'gemini-2.5-pro', 'claude-haiku-4-5'])[1 + (a.n + k) % 4],
               CASE WHEN a.n % 2 = 0 THEN i.group_a ELSE i.group_b END,
               1000 + k * 10, 200 + k, 5000, 100,
               0.02 + (k % 7) * 0.01, 0.01 + (k % 7) * 0.005, 0,
               day_start + make_interval(mins => (k * 47 + a.n) % 1440)
        FROM perf_active a
        CROSS JOIN perf_infra i
        CROSS JOIN generate_series(1, 15) AS k
        WHERE day_start + make_interval(mins => (k * 47 + a.n) % 1440) < NOW();
    END LOOP;
END $$;

-- 分润流水：每小时结算一次，活跃客户每天约 11 个小时有消费。
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, reward_type, created_at, updated_at)
SELECT a.owner_id, 'accrue', 0.01, a.id, 'agent_direct', NOW() - make_interval(hours => h), NOW() - make_interval(hours => h)
FROM perf_active a
CROSS JOIN generate_series(1, 90 * 24) AS h
WHERE (h + a.n) % 24 < 11;

INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, reward_type, created_at, updated_at)
SELECT a.upline_id, 'accrue', 0.005, a.id, 'agent_diff', NOW() - make_interval(hours => h), NOW() - make_interval(hours => h)
FROM perf_active a
CROSS JOIN generate_series(1, 90 * 24) AS h
WHERE a.upline_id IS NOT NULL AND (h + a.n) % 24 < 11;

-- 边界：降级、软删除、改挂。
UPDATE user_affiliates SET agent_level = 3 WHERE user_id = (SELECT id FROM perf_users WHERE n = 60);
UPDATE users SET deleted_at = NOW() WHERE id = (SELECT id FROM perf_users WHERE n = 59);
UPDATE user_affiliates
SET agent_owner_id = (SELECT id FROM perf_users WHERE n = 10), agent_owner_level = 1, agent_upline_id = NULL
WHERE user_id IN (SELECT id FROM perf_users WHERE n BETWEEN 61 AND 70);

ANALYZE usage_logs;
ANALYZE user_affiliate_ledger;
ANALYZE user_affiliates;
SELECT (SELECT COUNT(*) FROM usage_logs) AS usage_logs,
       (SELECT COUNT(*) FROM user_affiliate_ledger WHERE reward_type IN ('agent_direct', 'agent_diff')) AS agent_ledger,
       (SELECT id FROM users WHERE email = 'perf-00001@perf.local') AS agent_1;
