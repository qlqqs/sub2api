package service

// CUSTOM(fork): 区域利润纯计算；分润直接取账本金额，不重复应用分组系数。
import (
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type PartnerMonthInput struct {
	PartnerID        int64
	Month            string
	Rate, LossBefore decimal.Decimal
	Data             *PartnerMonthsData
	Replays          map[int64]FinanceRevenue
}

func PartnerMonthRate(rates []PartnerRate, id int64, month string) decimal.Decimal {
	var latest string
	rate := decimal.Zero
	for _, r := range rates {
		if r.PartnerID == id && r.Month <= month && r.Month >= latest {
			latest = r.Month
			rate = r.Rate
		}
	}
	return rate
}

func ComputePartnerMonth(in PartnerMonthInput) PartnerStatementDetail {
	out := PartnerStatementDetail{Lines: []PartnerLine{}, CostByGroup: []PartnerCostGroup{}, CostByModel: []PartnerCostModel{}, Adjustments: []PartnerAdjustment{}, Warnings: []string{}}
	lines := map[int64]financeAmounts{0: {}}
	counts := map[int64]int{}
	groups := map[int64]financeAmounts{}
	groupNames := map[int64]string{}
	models := map[string]financeAmounts{}
	for _, a := range in.Data.State.Attachments {
		if a.PartnerID == in.PartnerID && a.AttachedAt.In(AffiliateTeamLocation).Format("2006-01") <= in.Month {
			lines[a.AgentID] = financeAmounts{}
		}
	}
	lineFor := func(id int64) financeAmounts {
		if lines[id] == nil {
			lines[id] = financeAmounts{}
		}
		return lines[id]
	}
	for id, m := range in.Data.Members {
		if m.PartnerID != in.PartnerID || m.StartAt.In(AffiliateTeamLocation).Format("2006-01") > in.Month {
			continue
		}
		counts[m.LineAgentID]++
		line := lineFor(m.LineAgentID)
		startDay := financeDay(m.StartAt)
		for day, d := range in.Replays[id].Days {
			if day[:7] != in.Month || day < startDay {
				continue
			}
			rev := d.RevenuePaid.Add(d.RevenueCommission)
			if day == startDay {
				partial := in.Data.StartDays[id]
				if partial.Total.IsPositive() {
					rev = rev.Mul(partial.After).Div(partial.Total)
				} else {
					rev = decimal.Zero
				}
				if partial.Total.LessThan(d.Consumption) {
					out.Warnings = append(out.Warnings, "成员计入起点当天的部分明细已被清理，收入按现存明细折算")
				}
			}
			line.add("revenue", rev)
		}
	}
	var fixed int64
	for _, u := range in.Data.Usage {
		m, ok := in.Data.Members[u.UserID]
		if !ok || m.PartnerID != in.PartnerID || u.Day[:7] != in.Month {
			continue
		}
		line := lineFor(m.LineAgentID)
		line.add("consumption", u.Consumption)
		line.add("account_cost", u.Cost)
		if groups[u.GroupID] == nil {
			groups[u.GroupID] = financeAmounts{}
		}
		groupNames[u.GroupID] = u.GroupName
		if models[u.Model] == nil {
			models[u.Model] = financeAmounts{}
		}
		for _, amounts := range []financeAmounts{groups[u.GroupID], models[u.Model]} {
			amounts.add("consumption", u.Consumption)
			amounts.add("account_cost", u.Cost)
		}
		fixed += u.FixedRequests
	}
	for _, l := range in.Data.Finance.Ledger {
		m, ok := in.Data.Members[l.SourceUserID]
		if !ok || m.PartnerID != in.PartnerID || l.At.Before(m.StartAt) || l.At.In(AffiliateTeamLocation).Format("2006-01") != in.Month || l.Action != "accrue" {
			continue
		}
		switch l.RewardType {
		case "agent_direct", "agent_diff", "invite_percent", "invite_fixed", "":
			lineFor(m.LineAgentID).add("commission", l.Amount)
		}
	}
	totals := financeAmounts{}
	ids := make([]int64, 0, len(lines))
	for id := range lines {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		a := lines[id]
		for k, v := range a {
			totals.add(k, v)
		}
		line := PartnerLine{Members: counts[id], PartnerLineAmounts: partnerLineAmounts(a)}
		if id != 0 {
			u := in.Data.State.User(id)
			line.LineAgent = &u
		}
		out.Lines = append(out.Lines, line)
	}
	sort.SliceStable(out.Lines, func(i, j int) bool {
		return partnerDecimal(out.Lines[i].Revenue).GreaterThan(partnerDecimal(out.Lines[j].Revenue))
	})
	for id, a := range groups {
		out.CostByGroup = append(out.CostByGroup, PartnerCostGroup{id, groupNames[id], financeMoney(a["consumption"]), financeMoney(a["account_cost"])})
	}
	sort.Slice(out.CostByGroup, func(i, j int) bool {
		a, b := out.CostByGroup[i], out.CostByGroup[j]
		cmp := partnerDecimal(a.AccountCost).Cmp(partnerDecimal(b.AccountCost))
		if cmp == 0 {
			return a.GroupID < b.GroupID
		}
		return cmp > 0
	})
	for model, a := range models {
		out.CostByModel = append(out.CostByModel, PartnerCostModel{model, financeMoney(a["consumption"]), financeMoney(a["account_cost"])})
	}
	sort.Slice(out.CostByModel, func(i, j int) bool {
		a, b := out.CostByModel[i], out.CostByModel[j]
		cmp := partnerDecimal(a.AccountCost).Cmp(partnerDecimal(b.AccountCost))
		if cmp == 0 {
			return a.Model < b.Model
		}
		return cmp > 0
	})
	expense, adjustment := decimal.Zero, decimal.Zero
	for _, e := range in.Data.Finance.Expenses {
		if e.PartnerUserID == nil || *e.PartnerUserID != in.PartnerID {
			continue
		}
		at, _ := time.ParseInLocation("2006-01-02", e.OccurredOn, AffiliateTeamLocation)
		for i := 0; i < e.AmortizeDays; i++ {
			if at.AddDate(0, 0, i).Format("2006-01") == in.Month {
				expense = expense.Add(FinanceAmortization(partnerDecimal(e.Amount), e.AmortizeDays, i))
			}
		}
	}
	for _, a := range in.Data.State.Adjustments {
		if a.PartnerID == in.PartnerID && a.Month == in.Month {
			out.Adjustments = append(out.Adjustments, a)
			adjustment = adjustment.Add(partnerDecimal(a.Amount))
		}
	}
	profit := totals["revenue"].Sub(totals["account_cost"]).Sub(totals["commission"]).Sub(expense).Add(adjustment)
	base := decimal.Max(decimal.Zero, profit.Sub(in.LossBefore))
	loss := decimal.Max(decimal.Zero, in.LossBefore.Sub(profit))
	status := "draft"
	if in.Month == in.Data.Finance.ComputedAt.In(AffiliateTeamLocation).Format("2006-01") {
		status = "current"
	}
	out.Totals = PartnerMonthTotals{Month: in.Month, Status: status, ShareRate: in.Rate.StringFixed(4), Consumption: financeMoney(totals["consumption"]), Revenue: financeMoney(totals["revenue"]), AccountCost: financeMoney(totals["account_cost"]), Commission: financeMoney(totals["commission"]), RegionExpense: financeMoney(expense), Adjustment: financeMoney(adjustment), Profit: financeMoney(profit), LossBefore: financeMoney(in.LossBefore), LossAfter: financeMoney(loss), ShareBase: financeMoney(base), ShareAmount: base.Mul(in.Rate).StringFixed(2)}
	if fixed > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("%d 条请求走固定成本账号，成本按 0 计", fixed))
	}
	at, _ := time.ParseInLocation("2006-01", in.Month, AffiliateTeamLocation)
	// CUSTOM(fork): 提示只看本区域，未知分润类型不计入区域支出。
	scoped := &FinanceData{}
	replays := map[int64]FinanceRevenue{}
	for id, m := range in.Data.Members {
		if m.PartnerID == in.PartnerID {
			replays[id] = in.Replays[id]
		}
	}
	for _, order := range in.Data.Finance.Orders {
		if m, ok := in.Data.Members[order.UserID]; ok && m.PartnerID == in.PartnerID {
			scoped.Orders = append(scoped.Orders, order)
		}
	}
	out.Warnings = append(out.Warnings, financeDataWarnings(scoped, FinanceRange{Start: in.Month + "-01", End: at.AddDate(0, 1, -1).Format("2006-01-02")}, replays)...)
	seen := map[string]bool{}
	w := out.Warnings[:0]
	for _, v := range out.Warnings {
		if !seen[v] {
			seen[v] = true
			w = append(w, v)
		}
	}
	out.Warnings = w
	return out
}
func partnerLineAmounts(a financeAmounts) PartnerLineAmounts {
	return PartnerLineAmounts{financeMoney(a["consumption"]), financeMoney(a["revenue"]), financeMoney(a["account_cost"]), financeMoney(a["commission"]), financeMoney(a["revenue"].Sub(a["account_cost"]).Sub(a["commission"]))}
}
func partnerDecimal(s string) decimal.Decimal { d, _ := decimal.NewFromString(s); return d }
func (s *PartnerState) User(id int64) PartnerUserRef {
	if v, ok := s.Snapshots[id]; ok {
		return v.User
	}
	return PartnerUserRef{ID: id, Deleted: true}
}
func (s *PartnerState) Team(id int64) PartnerTeamCount {
	var out PartnerTeamCount
	for _, v := range s.Snapshots {
		if v.OwnerID != nil && *v.OwnerID == id {
			out.Owned++
		}
		if v.UplineID != nil && *v.UplineID == id {
			out.ViaUpline++
		}
	}
	return out
}
func (s *PartnerState) Info(id int64, month string) *PartnerInfo {
	p, ok := s.Partners[id]
	if !ok {
		return nil
	}
	out := &PartnerInfo{RegionName: p.RegionName, ShareRate: PartnerMonthRate(s.Rates, id, month).StringFixed(4), Since: p.CreatedAt}
	next := partnerNextMonth(month)
	for _, r := range s.Rates {
		if r.PartnerID == id && r.Month == next {
			rate := r.Rate.StringFixed(4)
			out.NextShareRate = &rate
			out.NextRateMonth = &next
		}
	}
	return out
}
func partnerNextMonth(month string) string {
	t, _ := time.ParseInLocation("2006-01", month, AffiliateTeamLocation)
	return t.AddDate(0, 1, 0).Format("2006-01")
}
