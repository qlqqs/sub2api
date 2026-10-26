//go:build unit

package service

// CUSTOM(fork): 先固定收入重放口径，所有预期金额用十进制比较。
import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func fd(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func fe(day, kind, units, cash string, order int64) FinanceEvent {
	at, _ := time.ParseInLocation("2006-01-02", day, AffiliateTeamLocation)
	return FinanceEvent{At: at, Kind: kind, Amount: fd(units), Cash: fd(cash), OrderID: order}
}
func TestFinanceRevenueReplay(t *testing.T) {
	cases := []struct {
		name                                                           string
		events                                                         []FinanceEvent
		actual, revenue, free, paid, commission, overdraft, difference string
	}{
		{"赠送优先", []FinanceEvent{fe("2026-01-01", "paid", "10", "9.68", 1), fe("2026-01-01", "free", "2", "0", 0), fe("2026-01-01", "consume", "5", "0", 0)}, "7", "2.904", "2", "7", "0", "0", "0"},
		{"先进先出跨天", []FinanceEvent{fe("2026-01-01", "paid", "10", "8", 1), fe("2026-01-02", "paid", "10", "9", 2), fe("2026-01-02", "consume", "15", "0", 0)}, "5", "12.5", "0", "5", "0", "0", "0"},
		{"退款扣回", []FinanceEvent{fe("2026-01-01", "paid", "10", "9.68", 1), fe("2026-01-02", "refund", "5", "5", 1), fe("2026-01-03", "consume", "5", "0", 0)}, "0", "4.68", "0", "0", "0", "0", "0"},
		{"退款不扣回", []FinanceEvent{fe("2026-01-01", "paid", "10", "9.68", 1), fe("2026-01-02", "refund", "0", "5", 1), fe("2026-01-03", "consume", "10", "0", 0)}, "0", "4.68", "0", "0", "0", "0", "0"},
		{"已消耗后退款", []FinanceEvent{fe("2026-01-01", "paid", "10", "9.68", 1), fe("2026-01-01", "consume", "10", "0", 0), fe("2026-01-02", "refund", "0", "5", 1)}, "0", "4.68", "0", "0", "0", "0", "0"},
		{"管理员扣余额", []FinanceEvent{fe("2026-01-01", "paid", "10", "8", 1), fe("2026-01-01", "free", "2", "0", 0), fe("2026-01-02", "adjust_out", "4", "0", 0), fe("2026-01-03", "consume", "3", "0", 0)}, "5", "2.4", "0", "5", "0", "0", "0"},
		{"分润转入再消耗", []FinanceEvent{fe("2026-01-01", "commission", "5", "5", 0), fe("2026-01-02", "consume", "3", "0", 0)}, "2", "3", "0", "0", "2", "0", "0"},
		{"透支", []FinanceEvent{fe("2026-01-01", "consume", "3", "0", 0)}, "-3", "0", "0", "0", "0", "3", "0"},
		{"未登记期初", []FinanceEvent{fe("2026-01-01", "paid", "10", "8", 1), fe("2026-01-02", "consume", "5", "0", 0)}, "7", "2.4", "2", "7", "0", "0", "2"},
		{"透支后充值不补确认", []FinanceEvent{fe("2026-01-01", "consume", "3", "0", 0), fe("2026-01-02", "paid", "10", "10", 1)}, "7", "0", "0", "7", "0", "3", "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := ReplayFinanceRevenue(tc.events, fd(tc.actual))
			rev, free, over := decimal.Zero, decimal.Zero, decimal.Zero
			for _, d := range r.Days {
				rev = rev.Add(d.RevenuePaid).Add(d.RevenueCommission)
				free = free.Add(d.FreeConsumed)
				over = over.Add(d.OverdraftConsumed)
			}
			for name, pair := range map[string][2]decimal.Decimal{"revenue": {fd(tc.revenue), rev}, "free": {fd(tc.free), free}, "paid": {fd(tc.paid), r.Paid}, "commission": {fd(tc.commission), r.Commission}, "overdraft": {fd(tc.overdraft), over}, "difference": {fd(tc.difference), r.Difference}} {
				require.True(t, pair[0].Equal(pair[1]), "%s want %s got %s", name, pair[0], pair[1])
			}
		})
	}
}

func TestFinanceAmortizationAndRange(t *testing.T) {
	for _, days := range []int{1, 3, 30, 3660} {
		sum := decimal.Zero
		for i := 0; i < days; i++ {
			sum = sum.Add(FinanceAmortization(fd("100.01"), days, i))
		}
		require.True(t, sum.Equal(fd("100.01")))
	}
	now := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	span, err := ParseFinanceRange("", "", "", now)
	require.NoError(t, err)
	require.Equal(t, "2026-10-26", span.End)
	_, err = ParseFinanceRange("2020-01-01", "2026-01-01", "day", now)
	require.Error(t, err)
	_, err = ParseFinanceRange("2026-02-30", "2026-03-01", "day", now)
	require.Error(t, err)
}
func TestFinanceSummaryCrossMonthAndFixedCosts(t *testing.T) {
	now := time.Now()
	span, err := ParseFinanceRange("2026-01-31", "2026-02-02", "month", now)
	require.NoError(t, err)
	d := &FinanceData{Users: []FinanceUser{{ID: 1, Balance: fd("4")}}, Events: []FinanceUserEvent{{1, fe("2026-01-31", "free", "10", "0", 0)}, {1, fe("2026-02-01", "consume", "6", "0", 0)}}, Costs: []FinanceCost{{Day: "2026-02-01", UserID: 1, AccountID: 3, Cost: fd("1.2")}, {Day: "2026-02-01", UserID: 1, AccountID: 1, Cost: fd("99")}}, FixedIDs: []int64{1}, Expenses: []FinanceExpense{{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-01-31", Amount: "10", AmortizeDays: 3}}}, ComputedAt: now}
	out := financeSummary(d, span)
	tot := out["totals"].(map[string]any)
	require.Equal(t, "-11.2", tot["profit"])
	require.Equal(t, "1.2", tot["account_cost_internal"])
	require.Equal(t, "10", tot["expenses"])
	require.Nil(t, tot["profit_margin"])
	series := out["series"].([]map[string]any)
	require.Len(t, series, 2)
	require.Equal(t, "2026-01", series[0]["period"])
	require.Equal(t, "3.33333333", series[0]["expenses"])
	require.Equal(t, "6.66666667", series[1]["expenses"])
}

func TestFinanceRefundTargetsOrderAndSpillover(t *testing.T) {
	events := []FinanceEvent{fe("2026-01-01", "paid", "10", "10", 1), fe("2026-01-02", "paid", "10", "5", 2), fe("2026-01-02", "free", "2", "0", 0), fe("2026-01-03", "refund", "12", "5", 2), fe("2026-01-04", "consume", "3", "0", 0)}
	r := ReplayFinanceRevenue(events, fd("7"))
	require.Equal(t, "3", r.Days["2026-01-04"].RevenuePaid.String())
	require.Equal(t, "7", r.Paid.String())
	require.True(t, r.Free.IsZero())
	require.True(t, r.Difference.IsZero())
}
func TestFinanceSameDayCreditBeforeConsumption(t *testing.T) {
	consume := fe("2026-02-01", "consume", "5", "0", 0)
	credit := fe("2026-02-01", "free", "5", "0", 0)
	credit.At = credit.At.Add(23 * time.Hour)
	r := ReplayFinanceRevenue([]FinanceEvent{consume, credit}, decimal.Zero)
	require.Equal(t, "5", r.Days["2026-02-01"].FreeConsumed.String())
	require.True(t, r.Overdraft.IsZero())
}

// CUSTOM(fork): 邮箱包含匹配与纯数字 ID 精确匹配为 OR，不接受带符号或部分 ID。
func TestFinanceSearchEmailOrExactUserID(t *testing.T) {
	user := FinanceUser{ID: 123, Email: "Alice456@Example.COM"}
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"", true}, {"ALICE", true}, {"example.com", true}, {"456", true},
		{"123", true}, {"000123", true}, {"12", false}, {"1234", false},
		{"+123", false}, {"-123", false}, {"123.0", false}, {"1e2", false},
		{"123 ", false}, {"１２３", false}, {"9223372036854775808", false}, {"unknown", false},
	} {
		t.Run(tc.query, func(t *testing.T) { require.Equal(t, tc.want, financeSearch(user, tc.query)) })
	}
}
