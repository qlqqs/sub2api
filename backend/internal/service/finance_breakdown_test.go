//go:build unit

package service

// CUSTOM(fork): 关联支出仅影响账号拆分，跨月截断、尾差与总览保持同一口径。
import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type financeBreakdownRepo struct {
	FinanceRepository
	data *FinanceData
}

func (r *financeBreakdownRepo) Load(context.Context, FinanceRange) (*FinanceData, error) {
	return r.data, nil
}

func TestFinanceBreakdownAccountExpenses(t *testing.T) {
	ctx := context.Background()
	span := FinanceRange{Start: "2026-02-01", End: "2026-02-02", Granularity: "day"}
	id1, id2, id3, id4 := int64(1), int64(2), int64(3), int64(4)
	data := &FinanceData{
		FixedIDs: []int64{1},
		Costs: []FinanceCost{
			{AccountID: 1, AccountName: "固定账号", GroupID: 8, Model: "m", UserID: 9, Requests: 2, Images: 3, Consumption: fd("20"), Cost: fd("99")},
			{AccountID: 5, AccountName: "按量账号", GroupID: 8, Model: "m", UserID: 9, Requests: 1, Consumption: fd("10"), Cost: fd("4")},
			{AccountID: 6, AccountName: "并列账号", Requests: 1, Cost: fd("3")},
		},
		ExpenseAccounts: []FinanceExpenseAccount{
			{FinanceAccount: FinanceAccount{ID: 1, Name: "固定账号"}},
			{FinanceAccount: FinanceAccount{ID: 2, Name: "已停用", Deleted: true}, HasPerImagePricing: true, ModelRate: true},
			{FinanceAccount: FinanceAccount{ID: 3, Name: "模型账号"}, ModelRate: true},
		},
		Expenses: []FinanceExpense{
			{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-01-31", Category: "upstream_subscription", Amount: "10", AmortizeDays: 3, AccountID: &id1}},
			{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-02", Category: "server", Amount: "3", AmortizeDays: 1, AccountID: &id2}},
			{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-01", Category: "domain", Amount: "1", AmortizeDays: 1, AccountID: &id3}},
			{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-01", Category: "other", Amount: "2", AmortizeDays: 1}},
			{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-03", Category: "other", Amount: "100", AmortizeDays: 1, AccountID: &id4}},
		},
	}
	svc := NewFinanceService(&financeBreakdownRepo{data: data})
	out, err := svc.Breakdown(ctx, span, "account", 200)
	require.NoError(t, err)
	rows := out["rows"].([]map[string]any)
	require.Len(t, rows, 5)
	require.Equal(t, []string{"1", "5", "2", "6", "3"}, []string{rows[0]["key"].(string), rows[1]["key"].(string), rows[2]["key"].(string), rows[3]["key"].(string), rows[4]["key"].(string)})
	require.Equal(t, "6.66666667", rows[0]["expense_cost"])
	require.Equal(t, "13.33333333", rows[0]["gross_profit"])
	require.Equal(t, "0", rows[0]["account_cost"])
	require.Equal(t, "fixed", rows[0]["cost_mode"])
	require.Equal(t, "0", rows[1]["expense_cost"])
	require.Equal(t, "6", rows[1]["gross_profit"])
	require.Equal(t, "已停用（已删除）", rows[2]["label"])
	require.Equal(t, "per_image", rows[2]["cost_mode"])
	require.Equal(t, "model_rate", rows[4]["cost_mode"])
	for _, row := range []map[string]any{rows[2], rows[4]} {
		require.Equal(t, int64(0), row["requests"])
		require.Equal(t, int64(0), row["images"])
		for _, k := range []string{"account_cost", "consumption", "subscription_consumption"} {
			require.Equal(t, "0", row[k])
		}
	}
	require.Equal(t, "-3", rows[2]["gross_profit"])
	limited, err := svc.Breakdown(ctx, span, "account", 1)
	require.NoError(t, err)
	require.Equal(t, true, limited["truncated"])
	require.Equal(t, rows[:1], limited["rows"])

	// CUSTOM(fork): 分组/模型/用户不补行、不输出关联支出字段，原排序与毛利逐项一致。
	without := *data
	without.Expenses, without.ExpenseAccounts = nil, nil
	for _, dim := range []string{"group", "model", "user"} {
		actual, err := svc.Breakdown(ctx, span, dim, 200)
		require.NoError(t, err)
		expected, err := NewFinanceService(&financeBreakdownRepo{data: &without}).Breakdown(ctx, span, dim, 200)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		for _, row := range actual["rows"].([]map[string]any) {
			require.NotContains(t, row, "expense_cost")
		}
	}
	// CUSTOM(fork): 无用量行同样遵守固定优先，任何类别的关联支出均保留。
	data.FixedIDs = append(data.FixedIDs, 3)
	fixedOut, err := svc.Breakdown(ctx, span, "account", 200)
	require.NoError(t, err)
	fixedRow := fixedOut["rows"].([]map[string]any)[4]
	require.Equal(t, "fixed", fixedRow["cost_mode"])
	require.Equal(t, "1", fixedRow["expense_cost"])

	summary := financeSummary(data, span)
	require.Equal(t, "12.66666667", summary["totals"].(map[string]any)["expenses"])
	// 账号归属不能让总览重复计支出。
	unlinked := *data
	unlinked.Expenses = append([]FinanceExpense(nil), data.Expenses...)
	for i := range unlinked.Expenses {
		unlinked.Expenses[i].AccountID = nil
	}
	require.Equal(t, summary, financeSummary(&unlinked, span))
}
