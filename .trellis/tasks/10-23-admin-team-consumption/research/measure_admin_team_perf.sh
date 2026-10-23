#!/usr/bin/env bash
# 测量管理端团队消费页各接口在 7 天 / 90 天区间的耗时（本机 sub2api-local，先执行 seed_admin_team_perf.sql 并启动含迁移 252 的镜像）。
# 管理员登录，每个请求跑 3 次取中位数；另测分润汇总的全量重建与触发器开销（都在回滚的事务里，不改数据）。
set -euo pipefail
LOCAL=/home/qlqq/workspace/sub2api-local
BASE=http://127.0.0.1:28080/api/v1
cd "$LOCAL"
set -a; source .env; set +a
psql_q() { docker compose exec -T postgres psql -U sub2api -d sub2api -At -c "$1"; }

TOKEN=$(curl -s --noproxy '*' -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["access_token"])')
L1=$(psql_q "SELECT id FROM users WHERE email = 'perf-00001@perf.local'")
L2=$(psql_q "SELECT id FROM users WHERE email = 'perf-00011@perf.local'")
# perf-00100：n % 10 = 0 且 n % 3 ≠ 0，是 perf-00001 的直属活跃客户。
CUSTOMER=$(psql_q "SELECT id FROM users WHERE email = 'perf-00100@perf.local'")

TODAY=$(TZ=Asia/Shanghai date +%F)
D7=$(TZ=Asia/Shanghai date -d '6 days ago' +%F)
D90=$(TZ=Asia/Shanghai date -d '89 days ago' +%F)

measure() {
  local label=$1 path=$2 times=()
  for _ in 1 2 3; do
    local out code t
    out=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code} %{time_total}' -H "Authorization: Bearer $TOKEN" "$BASE$path")
    code=${out% *}; t=${out#* }
    [[ $code == 200 ]] || { echo "$label HTTP $code"; return; }
    times+=("$t")
  done
  printf '%-36s %s s\n' "$label" "$(printf '%s\n' "${times[@]}" | sort -n | sed -n 2p)"
}

echo "== 接口（中位数）"
for range in "7d:$D7" "90d:$D90"; do
  name=${range%%:*}; start=${range#*:}
  q="start_date=$start&end_date=$TODAY"
  measure "summary overview $name"            "/admin/affiliates/team/overview?$q"
  measure "summary agents team_cost $name"    "/admin/affiliates/team/agents?$q"
  measure "summary agents earnings asc $name" "/admin/affiliates/team/agents?$q&sort_by=earnings&sort_order=asc&page=2"
  measure "summary agents level 2 $name"      "/admin/affiliates/team/agents?$q&level=2"
  measure "summary agents search $name"       "/admin/affiliates/team/agents?$q&search=perf-0001"
  measure "viewer overview L1 $name"          "/admin/affiliates/team/$L1/overview?$q"
  measure "viewer customers L1 $name"         "/admin/affiliates/team/$L1/customers?$q"
  measure "viewer agents L1 $name"            "/admin/affiliates/team/$L1/agents?$q"
  measure "viewer overview L1>L2 $name"       "/admin/affiliates/team/$L1/overview?$q&agent_id=$L2"
  measure "viewer requests customer $name"    "/admin/affiliates/team/$L1/customers/$CUSTOMER/requests?$q"
done

echo "== 规模"
psql_q "SELECT 'usage_logs', COUNT(*) FROM usage_logs UNION ALL
        SELECT 'agent ledger', COUNT(*) FROM user_affiliate_ledger WHERE reward_type IN ('agent_direct', 'agent_diff') UNION ALL
        SELECT 'reward rollups', COUNT(*) FROM affiliate_agent_reward_daily_rollups UNION ALL
        SELECT 'reward rollups 90d', COUNT(*) FROM affiliate_agent_reward_daily_rollups WHERE bucket_date >= '$D90'" | tr '|' ' '
psql_q "SELECT pg_size_pretty(pg_total_relation_size('affiliate_agent_reward_daily_rollups'))"

echo "== 汇总与流水一致（不一致的键数）"
psql_q "SELECT COUNT(*) FROM (
  SELECT user_id, COALESCE(source_user_id, 0) AS s, (created_at AT TIME ZONE 'Asia/Shanghai')::date AS d, SUM(amount) AS a, COUNT(*) AS c
  FROM user_affiliate_ledger WHERE reward_type IN ('agent_direct', 'agent_diff') GROUP BY 1, 2, 3
) raw
FULL JOIN affiliate_agent_reward_daily_rollups r ON r.user_id = raw.user_id AND r.source_user_id = raw.s AND r.bucket_date = raw.d
WHERE raw.a IS DISTINCT FROM r.amount OR raw.c IS DISTINCT FROM r.entries"

echo "== 全量重建（= 迁移 252 的回填，回滚）"
docker compose exec -T postgres psql -U sub2api -d sub2api -q <<'SQL' | grep -E 'Time|rebuild'
\timing on
BEGIN;
SELECT rebuild_affiliate_agent_reward_daily_rollups(DATE '1970-01-01', DATE '9999-12-30');
ROLLBACK;
SQL

echo "== 触发器开销：5000 条单行插入（每条一个语句，同结算），回滚；依次为 不同汇总键 有/无触发器、同一汇总键 有/无触发器"
# 造数的用户 ID 连续：CUSTOMER + i 都是存在的用户（来源外键）。
for source_expr in "current_setting('perf.c')::bigint + i" "current_setting('perf.c')::bigint"; do
  for disable in "" "ALTER TABLE user_affiliate_ledger DISABLE TRIGGER user_affiliate_ledger_reward_rollup_insert;"; do
    docker compose exec -T postgres psql -U sub2api -d sub2api -q -v l1="$L1" -v c="$CUSTOMER" <<SQL | grep -E 'Time' | sort -t: -k2 -n | tail -1
SET perf.l1 = :'l1';
SET perf.c = :'c';
\\timing on
BEGIN;
$disable
DO \$\$ BEGIN FOR i IN 1..5000 LOOP
  INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, reward_type, created_at, updated_at)
  VALUES (current_setting('perf.l1')::bigint, 'accrue', 0.01, $source_expr, 'agent_direct', NOW(), NOW());
END LOOP; END \$\$;
ROLLBACK;
SQL
  done
done
