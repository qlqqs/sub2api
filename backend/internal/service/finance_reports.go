package service

// CUSTOM(fork): 财务报表只聚合已保存的业务事实，利润与消耗毛利分开输出。
import (
	"context"
	"sort"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

func (s *FinanceService) Summary(ctx context.Context, span FinanceRange) (map[string]any, error) {
	data, err := s.repo.Load(ctx, span)
	if err != nil {
		return nil, err
	}
	return financeSummary(data, span), nil
}
func financeSummary(data *FinanceData, span FinanceRange) map[string]any {
	replays := financeReplays(data)
	daily := map[string]financeAmounts{}
	start, end := span.Bounds()
	for t := start; t.Before(end); t = t.AddDate(0, 0, 1) {
		daily[financeDay(t)] = financeTotals()
	}
	snapshot := financeAmounts{}
	for _, k := range []string{"balance_total", "balance_paid", "balance_paid_cash_value", "balance_commission", "balance_free", "balance_unattributed", "frozen_balance", "affiliate_available", "affiliate_frozen"} {
		snapshot[k] = decimal.Zero
	}
	for _, u := range data.Users {
		r := replays[u.ID]
		if !u.Deleted {
			snapshot.add("balance_total", u.Balance)
			snapshot.add("balance_paid", r.Paid)
			snapshot.add("balance_paid_cash_value", r.PaidCash)
			snapshot.add("balance_commission", r.Commission)
			snapshot.add("balance_free", r.Free)
			snapshot.add("balance_unattributed", r.Difference)
			snapshot.add("frozen_balance", u.Frozen)
			snapshot.add("affiliate_available", u.Available)
			snapshot.add("affiliate_frozen", u.AffiliateFrozen)
		}
		for day, v := range r.Days {
			m := daily[day]
			if m == nil {
				continue
			}
			m.add("revenue_paid", v.RevenuePaid)
			m.add("revenue_commission", v.RevenueCommission)
			m.add("free_consumed", v.FreeConsumed)
			m.add("overdraft_consumed", v.OverdraftConsumed)
			m.add("consumption", v.Consumption)
		}
	}
	fixed := map[int64]bool{}
	for _, id := range data.FixedIDs {
		fixed[id] = true
	}
	for _, c := range data.Costs {
		if fixed[c.AccountID] {
			continue
		}
		m := daily[c.Day]
		if m == nil {
			continue
		}
		m.add("account_cost", c.Cost)
		if d := replays[c.UserID].Days[c.Day]; d != nil && d.Consumption.IsPositive() {
			m.add("account_cost_internal", c.Cost.Mul(d.FreeConsumed).Div(d.Consumption))
		}
	}
	for _, l := range data.Ledger {
		if l.Action == "accrue" && l.RewardType != "invitee_bonus" {
			if m := daily[financeDay(l.At)]; m != nil {
				m.add("commission", l.Amount)
			}
		}
	}
	for _, e := range data.Expenses {
		at, _ := time.ParseInLocation("2006-01-02", e.OccurredOn, AffiliateTeamLocation)
		amount, _ := decimal.NewFromString(e.Amount)
		for i := 0; i < e.AmortizeDays; i++ {
			day := financeDay(at.AddDate(0, 0, i))
			if m := daily[day]; m != nil {
				m.add("expenses", FinanceAmortization(amount, e.AmortizeDays, i))
			}
		}
	}
	periods := map[string]financeAmounts{}
	totals := financeTotals()
	for day, m := range daily {
		period := day
		if span.Granularity == "month" {
			period = day[:7]
		}
		if periods[period] == nil {
			periods[period] = financeTotals()
		}
		for k, v := range m {
			periods[period].add(k, v)
			totals.add(k, v)
		}
	}
	keys := make([]string, 0, len(periods))
	for k := range periods {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	series := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		row := financeFinalize(periods[k])
		row["period"] = k
		series = append(series, row)
	}
	return map[string]any{"range": span, "totals": financeFinalize(totals), "series": series, "snapshot": snapshot, "warnings": financeWarnings(data, span, replays), "computed_at": data.ComputedAt.In(AffiliateTeamLocation)}
}
func (s *FinanceService) Recharge(ctx context.Context, span FinanceRange) (map[string]any, error) {
	d, err := s.repo.Load(ctx, span)
	if err != nil {
		return nil, err
	}
	amounts := financeAmounts{"cash_received": decimal.Zero, "channel_fee": decimal.Zero, "refund_cash": decimal.Zero, "net_cash": decimal.Zero, "principal": decimal.Zero, "commission_transfer_in": decimal.Zero}
	gift := financeAmounts{}
	for _, k := range []string{"recharge_bonus", "invitee_bonus", "promo", "admin_adjust_in", "admin_adjust_out", "redeem_code", "total"} {
		gift[k] = decimal.Zero
	}
	orders := []map[string]any{}
	count := 0
	for i := len(d.Orders) - 1; i >= 0; i-- {
		o := d.Orders[i]
		if o.OrderType != "balance" {
			continue
		}
		if o.RefundAt != nil && span.Contains(*o.RefundAt) {
			amounts.add("refund_cash", o.Refund)
		}
		if !span.Contains(o.CompletedAt) {
			continue
		}
		count++
		fee := o.Pay.Mul(o.FeeRate)
		amounts.add("cash_received", o.Pay)
		amounts.add("channel_fee", fee)
		amounts.add("principal", o.Principal)
		gift.add("recharge_bonus", o.Bonus)
		if len(orders) < 500 {
			orders = append(orders, map[string]any{"id": o.ID, "out_trade_no": o.OutTradeNo, "user_id": o.UserID, "user_email": o.Email, "completed_at": o.CompletedAt.In(AffiliateTeamLocation), "payment_type": o.PaymentType, "pay_amount": financeMoney(o.Pay), "channel_fee_rate": o.FeeRate.StringFixed(4), "channel_fee": financeMoney(fee), "principal": financeMoney(o.Principal), "bonus": financeMoney(o.Bonus), "refund_cash": financeMoney(o.Refund), "refund_balance_deducted": financeMoney(o.RefundDeducted), "status": o.Status})
		}
	}
	for _, e := range d.Events {
		if span.Contains(e.Event.At) && (e.Event.Kind == "free" || e.Event.Kind == "adjust_out") {
			gift.add(e.Event.Source, e.Event.Amount)
		}
	}
	for _, l := range d.Ledger {
		if !span.Contains(l.At) {
			continue
		}
		if l.Action == "invitee_bonus" {
			gift.add("invitee_bonus", l.Amount)
		}
		if l.Action == "transfer" {
			amounts.add("commission_transfer_in", l.Amount)
		}
	}
	for k, v := range gift {
		if k != "total" && k != "admin_adjust_out" {
			gift.add("total", v)
		}
	}
	amounts["net_cash"] = amounts["cash_received"].Sub(amounts["channel_fee"]).Sub(amounts["refund_cash"])
	totals := map[string]any{"order_count": count, "gift": gift}
	for k, v := range amounts {
		totals[k] = financeMoney(v)
	}
	return map[string]any{"totals": totals, "orders": orders, "orders_truncated": count > 500}, nil
}
func (s *FinanceService) Breakdown(ctx context.Context, span FinanceRange, dim string, limit int) (map[string]any, error) {
	if dim != "account" && dim != "group" && dim != "model" && dim != "user" {
		return nil, infraerrors.BadRequest("FINANCE_DIM_INVALID", "拆分维度无效")
	}
	if limit < 1 || limit > 1000 {
		return nil, infraerrors.BadRequest("FINANCE_LIMIT_INVALID", "条数须为 1–1000")
	}
	d, err := s.repo.Load(ctx, span)
	if err != nil {
		return nil, err
	}
	fixed := map[int64]bool{}
	for _, id := range d.FixedIDs {
		fixed[id] = true
	}
	type row struct {
		key, label, mode                         string
		requests, images                         int64
		modelRate, perImage, fixed               bool
		consumption, subscription, cost, expense decimal.Decimal
	}
	rows := map[string]*row{}
	for _, c := range d.Costs {
		var key, label string
		switch dim {
		case "account":
			key = strconv.FormatInt(c.AccountID, 10)
			label = financeLabel(c.AccountName, c.AccountDeleted)
		case "group":
			key = strconv.FormatInt(c.GroupID, 10)
			label = financeLabel(c.GroupName, c.GroupDeleted)
		case "model":
			key = c.Model
			label = key
		case "user":
			key = strconv.FormatInt(c.UserID, 10)
			label = financeLabel(c.UserEmail, c.UserDeleted)
		}
		r := rows[key]
		if r == nil {
			r = &row{key: key, label: label, mode: "rate"}
			rows[key] = r
		}
		r.requests += c.Requests
		r.images += c.Images
		r.perImage = r.perImage || c.HasPerImagePricing
		r.modelRate = r.modelRate || c.ModelRate
		r.fixed = r.fixed || fixed[c.AccountID]
		r.consumption = r.consumption.Add(c.Consumption)
		r.subscription = r.subscription.Add(c.Subscription)
		if !fixed[c.AccountID] {
			r.cost = r.cost.Add(c.Cost)
		}
	}
	// CUSTOM(fork): 仅账号维度归集关联支出，沿用总览逐日摊销与尾差口径，不修改按量成本快照。
	if dim == "account" {
		accounts := make(map[int64]FinanceExpenseAccount, len(d.ExpenseAccounts))
		for _, a := range d.ExpenseAccounts {
			accounts[a.ID] = a
		}
		for _, e := range d.Expenses {
			if e.AccountID == nil {
				continue
			}
			at, _ := time.ParseInLocation("2006-01-02", e.OccurredOn, AffiliateTeamLocation)
			amount, _ := decimal.NewFromString(e.Amount)
			expense := decimal.Zero
			for i := 0; i < e.AmortizeDays; i++ {
				if span.Contains(at.AddDate(0, 0, i)) {
					expense = expense.Add(FinanceAmortization(amount, e.AmortizeDays, i))
				}
			}
			if !expense.IsPositive() {
				continue
			}
			id := *e.AccountID
			key := strconv.FormatInt(id, 10)
			r := rows[key]
			if r == nil {
				a := accounts[id]
				r = &row{key: key, label: financeLabel(a.Name, a.Deleted), mode: "rate", fixed: fixed[id], perImage: a.HasPerImagePricing, modelRate: a.ModelRate}
				rows[key] = r
			}
			r.expense = r.expense.Add(expense)
		}
	}
	ordered := make([]*row, 0, len(rows))
	for _, r := range rows {
		ordered = append(ordered, r)
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].cost, ordered[j].cost
		if dim == "account" {
			a, b = a.Add(ordered[i].expense), b.Add(ordered[j].expense)
		}
		if dim == "user" {
			a, b = ordered[i].consumption, ordered[j].consumption
		}
		if a.Equal(b) {
			return ordered[i].key < ordered[j].key
		}
		return a.GreaterThan(b)
	})
	truncated := len(ordered) > limit
	if truncated {
		ordered = ordered[:limit]
	}
	out := make([]map[string]any, 0, len(ordered))
	for _, r := range ordered {
		v := map[string]any{"key": r.key, "label": r.label, "requests": r.requests, "images": r.images, "consumption": financeMoney(r.consumption), "subscription_consumption": financeMoney(r.subscription), "account_cost": financeMoney(r.cost), "gross_profit": financeMoney(r.consumption.Sub(r.cost))}
		if dim == "account" {
			v["expense_cost"] = financeMoney(r.expense)
			v["gross_profit"] = financeMoney(r.consumption.Sub(r.cost).Sub(r.expense))
			// CUSTOM(fork): 当前配置优先级：固定 > 按张/次规则 > 模型倍率 > 默认倍率。
			if r.modelRate {
				r.mode = "model_rate"
			}
			if r.perImage {
				r.mode = "per_image"
			}
			if r.fixed {
				r.mode = "fixed"
			}
			v["cost_mode"] = r.mode
		}
		out = append(out, v)
	}
	return map[string]any{"dim": dim, "rows": out, "truncated": truncated}, nil
}
func (s *FinanceService) Affiliate(ctx context.Context, span FinanceRange) (map[string]any, error) {
	d, err := s.repo.Load(ctx, span)
	if err != nil {
		return nil, err
	}
	accrued := financeAmounts{"agent_direct": decimal.Zero, "agent_diff": decimal.Zero, "invite_percent": decimal.Zero, "invite_fixed": decimal.Zero, "total": decimal.Zero}
	total := financeAmounts{"transferred": decimal.Zero, "withdrawn": decimal.Zero, "available": decimal.Zero, "frozen": decimal.Zero}
	users := map[int64]FinanceUser{}
	byUser := map[int64]financeAmounts{}
	for _, u := range d.Users {
		users[u.ID] = u
		if !u.Deleted {
			total.add("available", u.Available)
			total.add("frozen", u.AffiliateFrozen)
		}
	}
	for _, l := range d.Ledger {
		if !span.Contains(l.At) {
			continue
		}
		key := ""
		switch l.Action {
		case "accrue":
			if l.RewardType == "invitee_bonus" {
				continue
			}
			key = "accrued"
			if _, ok := accrued[l.RewardType]; ok {
				accrued.add(l.RewardType, l.Amount)
			}
			accrued.add("total", l.Amount)
		case "transfer":
			key = "transferred"
			total.add(key, l.Amount)
		case "withdraw":
			key = "withdrawn"
			total.add(key, l.Amount)
		default:
			continue
		}
		if byUser[l.UserID] == nil {
			byUser[l.UserID] = financeAmounts{"accrued": decimal.Zero, "transferred": decimal.Zero, "withdrawn": decimal.Zero}
		}
		byUser[l.UserID].add(key, l.Amount)
	}
	ids := make([]int64, 0, len(byUser))
	for id, m := range byUser {
		if m["accrued"].IsPositive() {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := byUser[ids[i]]["accrued"], byUser[ids[j]]["accrued"]
		if a.Equal(b) {
			return ids[i] < ids[j]
		}
		return a.GreaterThan(b)
	})
	if len(ids) > 50 {
		ids = ids[:50]
	}
	top := []map[string]any{}
	for _, id := range ids {
		u := users[id]
		m := byUser[id]
		top = append(top, map[string]any{"user_id": id, "user_email": u.Email, "accrued": financeMoney(m["accrued"]), "transferred": financeMoney(m["transferred"]), "withdrawn": financeMoney(m["withdrawn"]), "available": financeMoney(u.Available)})
	}
	out := map[string]any{"accrued": accrued, "top_beneficiaries": top}
	for k, v := range total {
		out[k] = financeMoney(v)
	}
	return out, nil
}
func (s *FinanceService) Reconcile(ctx context.Context, page, size int, onlyMismatch bool, search string) ([]map[string]any, int64, error) {
	// 对账为全时段，成本/支出没有参与；只查询当天避免无意义地扫历史明细。
	span, _ := ParseFinanceRange(financeDay(time.Now()), financeDay(time.Now()), "day", time.Now())
	d, err := s.repo.Load(ctx, span)
	if err != nil {
		return nil, 0, err
	}
	replays := financeReplays(d)
	items := []map[string]any{}
	var total int64
	for _, u := range d.Users {
		r := replays[u.ID]
		if !financeSearch(u, search) || (onlyMismatch && r.Difference.Abs().LessThan(decimal.RequireFromString("0.01"))) {
			continue
		}
		total++
		if total <= int64((page-1)*size) || len(items) >= size {
			continue
		}
		items = append(items, map[string]any{"user_id": u.ID, "user_email": u.Email, "role": u.Role, "deleted": u.Deleted, "paid_in": financeMoney(r.PaidIn), "paid_cash": financeMoney(r.CashIn), "free_in": financeMoney(r.FreeIn), "free_out": financeMoney(r.FreeOut), "commission_in": financeMoney(r.CommissionIn), "refund_deducted": financeMoney(r.RefundDeducted), "consumption": financeMoney(r.Consumption), "overdraft": financeMoney(r.Overdraft), "expected_balance": financeMoney(r.Expected), "actual_balance": financeMoney(u.Balance), "difference": financeMoney(r.Difference), "remaining": financeAmounts{"paid": r.Paid, "commission": r.Commission, "free": r.Free}})
	}
	return items, total, nil
}
