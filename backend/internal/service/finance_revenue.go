package service

// CUSTOM(fork): 收入确认纯函数，只重放财务事件，不触碰实际余额和业务扣费。
import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

type FinanceEvent struct {
	At           time.Time
	ID           int64
	Kind, Source string
	OrderID      int64
	Amount, Cash decimal.Decimal
}
type FinanceRevenueDay struct {
	RevenuePaid, RevenueCommission, FreeConsumed, OverdraftConsumed, Consumption decimal.Decimal
}
type FinanceRevenue struct {
	Days                                                                                  map[string]*FinanceRevenueDay
	Paid, PaidCash, Commission, Free, Debt                                                decimal.Decimal
	PaidIn, CashIn, FreeIn, FreeOut, CommissionIn, RefundDeducted, Consumption, Overdraft decimal.Decimal
	Expected, Difference, Unregistered                                                    decimal.Decimal
}
type financeBatch struct {
	orderID     int64
	commission  bool
	units, cash decimal.Decimal
}

func ReplayFinanceRevenue(events []FinanceEvent, actual decimal.Decimal) FinanceRevenue {
	r := FinanceRevenue{Days: map[string]*FinanceRevenueDay{}}
	ordered := append([]FinanceEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		da, db := financeDay(a.At), financeDay(b.At)
		if da != db {
			return da < db
		}
		if (a.Kind == "consume") != (b.Kind == "consume") {
			return b.Kind == "consume"
		}
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.ID < b.ID
	})
	// 已登记净流量保持有符号，不能把透支截为零后再推断期初。
	for _, e := range ordered {
		switch e.Kind {
		case "paid", "free", "commission":
			r.Expected = r.Expected.Add(e.Amount)
		case "refund", "adjust_out", "consume":
			r.Expected = r.Expected.Sub(e.Amount)
		}
	}
	r.Difference = actual.Sub(r.Expected)
	if r.Difference.IsPositive() {
		r.Unregistered = r.Difference
		r.Free = r.Unregistered
	}
	batches := make([]*financeBatch, 0)
	orders := make(map[int64]*financeBatch)
	head := 0 // 已耗尽的批次不再重扫，FIFO 总复杂度线性。
	// 免费优先 + 批次 FIFO；扣回不是消费，不能确认收入。
	drain := func(amount decimal.Decimal, day *FinanceRevenueDay) {
		n := decimal.Min(r.Free, amount)
		r.Free = r.Free.Sub(n)
		amount = amount.Sub(n)
		if day != nil {
			day.FreeConsumed = day.FreeConsumed.Add(n)
		}
		for head < len(batches) && amount.IsPositive() {
			b := batches[head]
			if !b.units.IsPositive() {
				head++
				continue
			}
			n = decimal.Min(b.units, amount)
			cash := b.cash.Mul(n).Div(b.units)
			if n.Equal(b.units) {
				cash = b.cash
			}
			b.units = b.units.Sub(n)
			b.cash = b.cash.Sub(cash)
			amount = amount.Sub(n)
			if day != nil {
				if b.commission {
					day.RevenueCommission = day.RevenueCommission.Add(cash)
				} else {
					day.RevenuePaid = day.RevenuePaid.Add(cash)
				}
			}
		}
		if amount.IsPositive() {
			r.Debt = r.Debt.Add(amount)
			if day != nil {
				day.OverdraftConsumed = day.OverdraftConsumed.Add(amount)
				r.Overdraft = r.Overdraft.Add(amount)
			}
		}
	}
	for _, e := range ordered {
		key := financeDay(e.At)
		d := r.Days[key]
		if d == nil {
			d = &FinanceRevenueDay{}
			r.Days[key] = d
		}
		switch e.Kind {
		case "free":
			r.FreeIn = r.FreeIn.Add(e.Amount)
			n := decimal.Min(r.Debt, e.Amount)
			r.Debt = r.Debt.Sub(n)
			r.Free = r.Free.Add(e.Amount.Sub(n))
		case "paid", "commission":
			b := &financeBatch{orderID: e.OrderID, commission: e.Kind == "commission", units: e.Amount, cash: e.Cash}
			if b.commission {
				r.CommissionIn = r.CommissionIn.Add(e.Amount)
			} else {
				r.PaidIn = r.PaidIn.Add(e.Amount)
				r.CashIn = r.CashIn.Add(e.Cash)
			}
			if b.units.IsPositive() {
				n := decimal.Min(r.Debt, b.units)
				b.cash = b.cash.Sub(b.cash.Mul(n).Div(b.units))
				b.units = b.units.Sub(n)
				r.Debt = r.Debt.Sub(n)
			}
			batches = append(batches, b)
			if !b.commission {
				orders[b.orderID] = b
			}
		case "consume":
			r.Consumption = r.Consumption.Add(e.Amount)
			d.Consumption = d.Consumption.Add(e.Amount)
			drain(e.Amount, d)
		case "adjust_out":
			r.FreeOut = r.FreeOut.Add(e.Amount)
			drain(e.Amount, nil)
		case "refund":
			r.RefundDeducted = r.RefundDeducted.Add(e.Amount)
			r.CashIn = r.CashIn.Sub(e.Cash)
			remaining := e.Amount
			cash := e.Cash
			if b := orders[e.OrderID]; b != nil {
				n := decimal.Min(b.units, remaining)
				b.units = b.units.Sub(n)
				remaining = remaining.Sub(n)
				c := decimal.Min(b.cash, cash)
				b.cash = b.cash.Sub(c)
				cash = cash.Sub(c)
				// 无余额不能继续承载未确认现金；尾差不冒充利润。
				if b.units.IsZero() {
					b.cash = decimal.Zero
				}
			}
			if cash.IsPositive() {
				d.RevenuePaid = d.RevenuePaid.Sub(cash)
			}
			if remaining.IsPositive() {
				drain(remaining, nil)
			}
		}
	}
	for _, b := range batches {
		if b.commission {
			r.Commission = r.Commission.Add(b.units)
		} else {
			r.Paid = r.Paid.Add(b.units)
			r.PaidCash = r.PaidCash.Add(b.cash)
		}
	}
	return r
}
func financeDay(t time.Time) string { return t.In(AffiliateTeamLocation).Format("2006-01-02") }
