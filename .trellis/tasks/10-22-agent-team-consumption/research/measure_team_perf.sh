#!/usr/bin/env bash
# 测量团队消费页四个接口在 7 天 / 90 天区间的耗时（本机 sub2api-local，先执行 seed_team_perf.sql）。
# 用 e2e 用户的密码哈希登录造数账号 B（perf-00001），每个请求跑 3 次取中位数；两个区间之间停 61 秒避开每分钟 60 次的重型限流。
set -euo pipefail
LOCAL=/home/qlqq/workspace/sub2api-local
BASE=http://127.0.0.1:28080/api/v1
cd "$LOCAL"
set -a; source .env; set +a
PW="${E2E_USER_PASSWORD:-$ADMIN_PASSWORD}"
psql_q() { docker compose exec -T postgres psql -U sub2api -d sub2api -At -c "$1"; }

# 造数账号沿用某个 e2e 注册用户的密码哈希（e2e 用户密码 = $PW）。
psql_q "UPDATE users SET password_hash = (SELECT password_hash FROM users WHERE email LIKE 'e2e-%' AND email NOT LIKE 'admin%' ORDER BY id LIMIT 1) WHERE email = 'perf-00001@perf.local'" >/dev/null
TOKEN=$(curl -s --noproxy '*' -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"perf-00001@perf.local\",\"password\":\"$PW\"}" | python3 -c 'import sys,json; print(json.load(sys.stdin)["data"]["access_token"])')
L2=$(psql_q "SELECT id FROM users WHERE email = 'perf-00002@perf.local'")
CUSTOMER=$(psql_q "SELECT id FROM users WHERE email = 'perf-00007@perf.local'")

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
  printf '%-34s %s s\n' "$label" "$(printf '%s\n' "${times[@]}" | sort -n | sed -n 2p)"
}

for range in "7d:$D7" "90d:$D90"; do
  name=${range%%:*}; start=${range#*:}
  [[ $name == 90d ]] && sleep 61
  q="start_date=$start&end_date=$TODAY"
  measure "overview team $name"            "/user/aff/team/overview?$q"
  measure "customers cost desc $name"      "/user/aff/team/customers?$q&sort_by=cost&sort_order=desc"
  measure "customers last_used asc $name"  "/user/aff/team/customers?$q&sort_by=last_used_at&sort_order=asc&page=50"
  measure "customers search $name"         "/user/aff/team/customers?$q&search=%E5%AE%A2%E6%88%B712"
  measure "agents $name"                   "/user/aff/team/agents?$q"
  measure "overview agent $name"           "/user/aff/team/overview?$q&agent_id=$L2"
  measure "customers agent $name"          "/user/aff/team/customers?$q&agent_id=$L2"
  measure "overview customer $name"        "/user/aff/team/overview?$q&user_id=$CUSTOMER"
  measure "requests customer $name"        "/user/aff/team/customers/$CUSTOMER/requests?$q&page=3"
done
