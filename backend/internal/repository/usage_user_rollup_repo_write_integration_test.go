//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// 应用写 usage_logs 走批量路径（多行 VALUES 的数据修改 CTE + ON CONFLICT DO NOTHING），
// 用户日汇总的语句级触发器必须对这些写入同样生效，且被冲突跳过的行不计入。
func TestUsageUserRollupTracksRepositoryBatchWrites(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUsageLogRepositoryWithSQL(client, integrationDB)

	user := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("usage-rollup-batch-%d@example.com", time.Now().UnixNano())})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-usage-rollup-batch-" + uuid.NewString(), Name: "k"})
	account := mustCreateAccount(t, client, &service.Account{Name: "acc-usage-rollup-batch-" + uuid.NewString()})
	group := mustCreateGroup(t, client, &service.Group{Name: "usage-rollup-batch-" + uuid.NewString()})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
	})

	at := time.Date(2026, 10, 2, 16, 30, 0, 0, time.UTC) // 北京时间 10-03 00:30
	newLog := func(requestID string, billingType int8, input int, cost float64, requestedModel string, groupID *int64) *service.UsageLog {
		return &service.UsageLog{
			UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID, RequestID: requestID,
			Model: "claude-upstream", RequestedModel: requestedModel, GroupID: groupID,
			InputTokens: input, OutputTokens: 10, CacheReadTokens: 5, ImageCount: 0,
			TotalCost: cost, ActualCost: cost, BillingType: billingType, CreatedAt: at,
		}
	}

	// 批量 Create：并发提交同一批，含重复 request_id（只插入一次）与订阅消费（不计入）。
	logs := []*service.UsageLog{
		newLog("rollup-r1", service.BillingTypeBalance, 100, 1.5, "claude", &group.ID),
		newLog("rollup-r2", service.BillingTypeBalance, 200, 2.5, "claude", &group.ID),
		newLog("rollup-r1", service.BillingTypeBalance, 100, 1.5, "claude", &group.ID),
		newLog("rollup-r3", service.BillingTypeSubscription, 999, 9.9, "claude", &group.ID),
	}
	var wg sync.WaitGroup
	errs := make([]error, len(logs))
	for i, log := range logs {
		wg.Add(1)
		go func(i int, log *service.UsageLog) {
			defer wg.Done()
			_, errs[i] = repo.Create(ctx, log)
		}(i, log)
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	// CreateBestEffort 的批量路径；空 requested_model 回落到 model，无分组记为 0。
	require.NoError(t, repo.CreateBestEffort(ctx, newLog("rollup-r4", service.BillingTypeBalance, 50, 0.5, "", nil)))
	require.NoError(t, repo.CreateBestEffort(ctx, newLog("rollup-r4", service.BillingTypeBalance, 50, 0.5, "", nil)))
	require.Eventually(t, func() bool {
		var n int
		err := integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE user_id = $1", user.ID).Scan(&n)
		return err == nil && n == 4
	}, 5*time.Second, 20*time.Millisecond)

	type row struct {
		date, model       string
		groupID, requests int64
		input, cacheRead  int64
		cost              string
	}
	read := func() []row {
		rows, err := integrationDB.QueryContext(ctx, `
			SELECT bucket_date::text, model, group_id, requests, input_tokens, cache_read_tokens, actual_cost::text
			FROM usage_user_daily_rollups WHERE user_id = $1 ORDER BY model, group_id`, user.ID)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		var out []row
		for rows.Next() {
			var r row
			require.NoError(t, rows.Scan(&r.date, &r.model, &r.groupID, &r.requests, &r.input, &r.cacheRead, &r.cost))
			out = append(out, r)
		}
		require.NoError(t, rows.Err())
		return out
	}
	require.Equal(t, []row{
		{date: "2026-10-03", model: "claude", groupID: group.ID, requests: 2, input: 300, cacheRead: 10, cost: "4.0000000000"},
		{date: "2026-10-03", model: "claude-upstream", groupID: 0, requests: 1, input: 50, cacheRead: 5, cost: "0.5000000000"},
	}, read())

	// 删除明细（保留期清理同样是 DELETE）后汇总随之扣减并删除。
	_, err := integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id = $1 AND request_id = 'rollup-r2'", user.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), read()[0].requests)
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM usage_logs WHERE user_id = $1", user.ID)
	require.NoError(t, err)
	require.Empty(t, read())
}
