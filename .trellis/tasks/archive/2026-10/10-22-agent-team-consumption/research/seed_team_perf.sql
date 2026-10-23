-- 代理团队消费页性能造数（仅用于本机 sub2api-local，勿在一号机执行）。规模见 design.md 第 8 节：
--   2 万用户；最大一级代理 B（perf-00001）有 5000 直属客户、5 个二级代理（各 300 客户）；
--   另有 10 个一级代理分掉其余用户；2000 活跃用户（B 的团队 500 个）；
--   usage_logs 90 天、每个活跃用户每天 15 条余额消费（聚合查询读汇总表，行数只取决于用户 × 模型 × 天）；B 的代理分润流水约 100 万行（每个活跃客户每小时一条）。
-- 用法：docker compose exec -T postgres psql -U sub2api -d sub2api -v ON_ERROR_STOP=1 < seed_team_perf.sql
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

-- 角色：1 = B；2..6 = B 的二级；7..5006 = B 直属；5007..6506 = 二级的客户（上方一级 B）；
--       6507..6516 = 其他一级；6517..20000 = 其他一级的客户。
CREATE TEMP TABLE perf_roles AS
SELECT p.n, p.id,
       CASE WHEN p.n = 1 OR p.n BETWEEN 6507 AND 6516 THEN 1
            WHEN p.n BETWEEN 2 AND 6 THEN 2 ELSE 3 END AS level,
       CASE WHEN p.n = 1 OR p.n BETWEEN 6507 AND 6516 THEN NULL
            WHEN p.n BETWEEN 2 AND 5006 THEN 1
            WHEN p.n BETWEEN 5007 AND 6506 THEN 2 + (p.n - 5007) / 300
            ELSE 6507 + (p.n - 6517) % 10 END AS owner_n,
       CASE WHEN p.n BETWEEN 5007 AND 6506 THEN 1 END AS upline_n
FROM perf_users p;

INSERT INTO user_affiliates (user_id, aff_code, agent_level, inviter_id, agent_owner_id, agent_owner_level,
                             agent_upline_id, agent_owner_bound_at, created_at, updated_at)
SELECT r.id, 'PF' || r.n, r.level, o.id, o.id, CASE WHEN o.id IS NULL THEN NULL WHEN r.owner_n BETWEEN 2 AND 6 THEN 2 ELSE 1 END,
       u.id, CASE WHEN o.id IS NULL THEN NULL ELSE NOW() - interval '120 days' END, NOW(), NOW()
FROM perf_roles r
LEFT JOIN perf_users o ON o.n = r.owner_n
LEFT JOIN perf_users u ON u.n = r.upline_n
ON CONFLICT (user_id) DO UPDATE
SET agent_level = EXCLUDED.agent_level, inviter_id = EXCLUDED.inviter_id, agent_owner_id = EXCLUDED.agent_owner_id,
    agent_owner_level = EXCLUDED.agent_owner_level, agent_upline_id = EXCLUDED.agent_upline_id,
    agent_owner_bound_at = EXCLUDED.agent_owner_bound_at;

-- 活跃用户：B 直属 300 + 二级客户 200 + 其他 1500 = 2000。
CREATE TEMP TABLE perf_active AS
SELECT p.n, p.id FROM perf_users p
WHERE p.n BETWEEN 7 AND 306 OR p.n BETWEEN 5007 AND 5206 OR p.n BETWEEN 6517 AND 8016;
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
        SELECT a.id, 1, 1,
               (ARRAY['claude-sonnet-4-5', 'gpt-5', 'gemini-2.5-pro', 'claude-haiku-4-5'])[1 + (a.n + k) % 4] || '-up',
               (ARRAY['claude-sonnet-4-5', 'gpt-5', 'gemini-2.5-pro', 'claude-haiku-4-5'])[1 + (a.n + k) % 4],
               1 + (a.n % 2),
               1000 + k * 10, 200 + k, 5000, 100,
               0.02 + (k % 7) * 0.01, 0.01 + (k % 7) * 0.005, 0,
               day_start + make_interval(mins => (k * 47 + a.n) % 1440)
        FROM perf_active a
        CROSS JOIN generate_series(1, 15) AS k
        WHERE day_start + make_interval(mins => (k * 47 + a.n) % 1440) < NOW();
    END LOOP;
END $$;

-- B 的代理分润流水：B 团队的 500 个活跃客户，90 天每小时一条。
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, reward_type, created_at, updated_at)
SELECT b.id, 'accrue', 0.01, a.id,
       CASE WHEN a.n BETWEEN 5007 AND 6506 THEN 'agent_diff' ELSE 'agent_direct' END,
       NOW() - make_interval(hours => h), NOW() - make_interval(hours => h)
FROM perf_users b
JOIN perf_users a ON a.n BETWEEN 7 AND 306 OR a.n BETWEEN 5007 AND 5206
CROSS JOIN generate_series(1, 90 * 24) AS h
WHERE b.n = 1;

ANALYZE usage_logs;
ANALYZE user_affiliate_ledger;
ANALYZE user_affiliates;
SELECT (SELECT COUNT(*) FROM usage_logs) AS usage_logs,
       (SELECT COUNT(*) FROM user_affiliate_ledger) AS ledger,
       (SELECT id FROM users WHERE email = 'perf-00001@perf.local') AS agent_b;
