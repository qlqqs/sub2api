//go:build unit

package service

// CUSTOM(fork): 财务仅在整月分成合计和月序列追加字段，不改原利润。
import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type partnerFinanceRepo struct {
	FinanceRepository
	data *FinanceData
}

func (r *partnerFinanceRepo) Load(context.Context, FinanceRange) (*FinanceData, error) {
	return r.data, nil
}

type partnerSharesStub struct {
	from, to string
	shares   map[string]PartnerShare
}

func (s *partnerSharesStub) PartnerShares(_ context.Context, from, to string) (map[string]PartnerShare, error) {
	s.from, s.to = from, to
	return s.shares, nil
}
func TestPartnerFinanceWholeMonthSharesAndDayFields(t *testing.T) {
	repo := &partnerFinanceRepo{data: &FinanceData{ComputedAt: time.Now(), Expenses: []FinanceExpense{{FinanceExpenseInput: FinanceExpenseInput{OccurredOn: "2026-02-02", Amount: "3", AmortizeDays: 1}}}}}
	shares := &partnerSharesStub{shares: map[string]PartnerShare{"2026-02": {Amount: decimal.NewFromInt(2), Estimated: true}}}
	svc := &FinanceService{repo: repo, partners: shares}
	for _, granularity := range []string{"day", "month"} {
		span, err := ParseFinanceRange("2026-02-02", "2026-02-03", granularity, time.Now())
		require.NoError(t, err)
		got, err := svc.Summary(context.Background(), span)
		require.NoError(t, err)
		require.Equal(t, "2026-02", shares.from)
		require.Equal(t, "2026-02", shares.to)
		totals := got["totals"].(map[string]any)
		require.Equal(t, "-3", totals["profit"])
		require.Equal(t, "2", totals["partner_share"])
		require.Equal(t, "-5", totals["net_profit"])
		require.Equal(t, true, totals["partner_share_estimated"])
		require.Equal(t, "whole_months", totals["partner_share_scope"])
		for _, row := range got["series"].([]map[string]any) {
			if granularity == "day" {
				require.NotContains(t, row, "partner_share")
				require.NotContains(t, row, "net_profit")
				require.NotContains(t, row, "partner_share_estimated")
			} else {
				require.Equal(t, "2", row["partner_share"])
				require.Equal(t, "-5", row["net_profit"])
			}
		}
	}
	svc.partners = nil
	span, _ := ParseFinanceRange("2026-02-02", "2026-02-03", "day", time.Now())
	got, err := svc.Summary(context.Background(), span)
	require.NoError(t, err)
	totals := got["totals"].(map[string]any)
	require.Equal(t, "0", totals["partner_share"])
	require.Equal(t, totals["profit"], totals["net_profit"])
	require.Equal(t, false, totals["partner_share_estimated"])
}
func TestPartnerWarningsDoNotExposeOtherRegions(t *testing.T) {
	in := partnerProfitFixture()
	in.Data.Finance.Orders = []FinanceOrder{{UserID: 99, OrderType: "subscription", CompletedAt: partnerTestTime("2026-02-03 12:00"), LegacyFee: true}}
	in.Replays[99] = FinanceRevenue{Difference: decimal.NewFromInt(9), Overdraft: decimal.NewFromInt(1)}
	out := ComputePartnerMonth(in)
	require.NotContains(t, out.Warnings, "范围内存在订阅订单，未计入利润")
	require.NotContains(t, out.Warnings, "存在透支消耗，超额部分未确认收入")
	require.Empty(t, out.Warnings)
}

// CUSTOM(fork): 固定说明只留在财务页，合伙人的真实异常提示仍保留。
func TestPartnerWarningsExcludeFinanceReplayNotice(t *testing.T) {
	for _, usage := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty month", true: "with usage"}[usage], func(t *testing.T) {
			in := partnerProfitFixture()
			if !usage {
				in.Replays = map[int64]FinanceRevenue{}
			}
			out := ComputePartnerMonth(in)
			require.Empty(t, out.Warnings)
			require.NotNil(t, out.Warnings)
			require.Empty(t, PartnerPublicStatement(out).Warnings)
		})
	}
	in := partnerProfitFixture()
	in.Data.Finance.Orders = []FinanceOrder{{UserID: 2, OrderType: "subscription", CompletedAt: partnerTestTime("2026-02-03 12:00"), LegacyFee: true}}
	replay := in.Replays[2]
	replay.Difference = decimal.NewFromInt(9)
	replay.Overdraft = decimal.NewFromInt(1)
	in.Replays[2] = replay
	out := ComputePartnerMonth(in)
	require.Equal(t, []string{
		"范围内存在订阅订单，未计入利润",
		"历史订单使用支付实例当前通道费率或退款扣回兼容值",
		"存在未登记入账或缺失流水，请查看用户对账；推断期初仅按免费余额处理",
		"存在透支消耗，超额部分未确认收入",
	}, out.Warnings)
}

func TestFinanceWarningsKeepReplayNoticeAndExistingOrder(t *testing.T) {
	span := FinanceRange{Start: "2026-02-01", End: "2026-02-28"}
	require.Equal(t, []string{financeReplayNotice}, financeWarnings(&FinanceData{}, span, nil))
	data := &FinanceData{
		Orders: []FinanceOrder{{OrderType: "subscription", CompletedAt: partnerTestTime("2026-02-03 12:00"), LegacyFee: true}},
		Ledger: []FinanceLedger{{Action: "accrue", At: partnerTestTime("2026-02-03 12:00"), RewardType: "unknown"}},
	}
	replays := map[int64]FinanceRevenue{1: {Difference: decimal.NewFromInt(9), Overdraft: decimal.NewFromInt(1)}}
	require.Equal(t, []string{
		"范围内存在订阅订单，未计入利润",
		"历史订单使用支付实例当前通道费率或退款扣回兼容值",
		"存在未知类型分润，已计入总支出，未归入四种分类",
		"存在未登记入账或缺失流水，请查看用户对账；推断期初仅按免费余额处理",
		"存在透支消耗，超额部分未确认收入",
		financeReplayNotice,
	}, financeWarnings(data, span, replays))
	svc := NewFinanceService(&partnerFinanceRepo{data: &FinanceData{ComputedAt: partnerTestTime("2026-03-02 00:00")}})
	summary, err := svc.Summary(context.Background(), span)
	require.NoError(t, err)
	require.Equal(t, []string{financeReplayNotice}, summary["warnings"])
}
