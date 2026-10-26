package service

// CUSTOM(fork): 财务查询端口及 DTO；金额由 PostgreSQL numeric 直接读 decimal，不经 float64。
import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var financeExpenseAmountPattern = regexp.MustCompile(`^[0-9]{1,18}(\.[0-9]{1,2})?$`)

const FinanceFixedAccountsKey = "finance_fixed_cost_account_ids"

var ErrFinanceExpenseNotFound = infraerrors.NotFound("FINANCE_EXPENSE_NOT_FOUND", "支出记录不存在")

type FinanceRange struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Granularity string `json:"granularity"`
}

func ParseFinanceRange(start, end, granularity string, now time.Time) (FinanceRange, error) {
	today := now.In(AffiliateTeamLocation)
	if start == "" {
		start = today.Format("2006-01") + "-01"
	}
	if end == "" {
		end = today.Format("2006-01-02")
	}
	if granularity == "" {
		granularity = "day"
	}
	a, ea := time.ParseInLocation("2006-01-02", start, AffiliateTeamLocation)
	b, eb := time.ParseInLocation("2006-01-02", end, AffiliateTeamLocation)
	if ea != nil || eb != nil || b.Before(a) || (granularity != "day" && granularity != "month") {
		return FinanceRange{}, infraerrors.BadRequest("FINANCE_RANGE_INVALID", "日期范围或统计粒度无效")
	}
	if b.Sub(a) >= 732*24*time.Hour {
		return FinanceRange{}, infraerrors.BadRequest("FINANCE_RANGE_TOO_LARGE", "日期范围不能超过 732 天")
	}
	return FinanceRange{start, end, granularity}, nil
}
func (r FinanceRange) Bounds() (time.Time, time.Time) {
	a, _ := time.ParseInLocation("2006-01-02", r.Start, AffiliateTeamLocation)
	b, _ := time.ParseInLocation("2006-01-02", r.End, AffiliateTeamLocation)
	return a, b.AddDate(0, 0, 1)
}
func (r FinanceRange) Contains(t time.Time) bool {
	d := financeDay(t)
	return d >= r.Start && d <= r.End
}
func financeMoney(d decimal.Decimal) string { return d.Round(8).String() }

type FinanceUser struct {
	ID                                          int64
	Email, Role                                 string
	Deleted                                     bool
	Balance, Frozen, Available, AffiliateFrozen decimal.Decimal
}
type FinanceOrder struct {
	OutTradeNo                                             string // CUSTOM(fork): 商户订单号，用于精确跳转订单页。
	ID, UserID                                             int64
	Email, PaymentType, Status, OrderType                  string
	CompletedAt                                            time.Time
	RefundAt                                               *time.Time
	Pay, Principal, Bonus, FeeRate, Refund, RefundDeducted decimal.Decimal
	LegacyFee, LegacyRefund                                bool
}
type FinanceLedger struct {
	ID, UserID         int64
	At                 time.Time
	Action, RewardType string
	Amount             decimal.Decimal
}
type FinanceUserEvent struct {
	UserID int64
	Event  FinanceEvent
}
type FinanceCost struct {
	HasPerImagePricing                                   bool // CUSTOM(fork): 当前账号/所属分组命中含 image/per_request 定价的统计规则。
	Day                                                  string
	AccountID, GroupID, UserID                           int64
	AccountName, GroupName, UserEmail, Model             string
	AccountDeleted, GroupDeleted, UserDeleted, ModelRate bool
	Requests, Images                                     int64
	Consumption, Subscription, Cost                      decimal.Decimal
}
type FinanceAccount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Deleted  bool   `json:"deleted"`
}

// CUSTOM(fork): 支出关联账号的当前配置，独立于用量，供无请求账号补行。
type FinanceExpenseAccount struct {
	FinanceAccount
	ModelRate, HasPerImagePricing bool
}
type FinanceSettings struct {
	FixedCostAccountIDs []int64          `json:"fixed_cost_account_ids"`
	Accounts            []FinanceAccount `json:"accounts"`
}
type FinanceExpenseInput struct {
	OccurredOn   string `json:"occurred_on"`
	Category     string `json:"category"`
	Amount       string `json:"amount"`
	AmortizeDays int    `json:"amortize_days"`
	AccountID    *int64 `json:"account_id"`
	Note         string `json:"note"`
}
type FinanceExpense struct {
	FinanceExpenseInput
	ID             int64     `json:"id"`
	AccountName    *string   `json:"account_name"`
	DailyAmount    string    `json:"daily_amount"`
	CreatedByEmail string    `json:"created_by_email"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type FinanceData struct {
	ExpenseAccounts []FinanceExpenseAccount // CUSTOM(fork): 同一读取快照内的支出关联账号。
	Users           []FinanceUser
	Orders          []FinanceOrder
	Ledger          []FinanceLedger
	Events          []FinanceUserEvent
	Costs           []FinanceCost
	Expenses        []FinanceExpense
	FixedIDs        []int64
	ComputedAt      time.Time
}
type FinanceRepository interface {
	Load(context.Context, FinanceRange) (*FinanceData, error)
	Settings(context.Context) (FinanceSettings, error)
	SaveSettings(context.Context, []int64) error
	ListExpenses(context.Context, FinanceRange, string, int, int) ([]FinanceExpense, int64, error)
	SaveExpense(context.Context, int64, int64, FinanceExpenseInput) (*FinanceExpense, error)
	DeleteExpense(context.Context, int64) error
}
type FinanceService struct{ repo FinanceRepository }

func NewFinanceService(repo FinanceRepository) *FinanceService { return &FinanceService{repo: repo} }
func (s *FinanceService) Settings(ctx context.Context) (FinanceSettings, error) {
	return s.repo.Settings(ctx)
}
func (s *FinanceService) SaveSettings(ctx context.Context, ids []int64) (FinanceSettings, error) {
	if len(ids) > 10000 {
		return FinanceSettings{}, infraerrors.BadRequest("FINANCE_SETTINGS_INVALID", "固定成本账号过多")
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return FinanceSettings{}, infraerrors.BadRequest("FINANCE_SETTINGS_INVALID", "账号 ID 无效")
		}
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	if err := s.repo.SaveSettings(ctx, out); err != nil {
		return FinanceSettings{}, err
	}
	return s.repo.Settings(ctx)
}
func (s *FinanceService) ListExpenses(ctx context.Context, r FinanceRange, category string, page, size int) ([]FinanceExpense, int64, error) {
	if category != "" && !financeCategory(category) {
		return nil, 0, infraerrors.BadRequest("FINANCE_EXPENSE_INVALID", "支出类别无效")
	}
	return s.repo.ListExpenses(ctx, r, category, page, size)
}
func financeCategory(s string) bool {
	return s == "upstream_subscription" || s == "server" || s == "domain" || s == "other"
}
func (s *FinanceService) SaveExpense(ctx context.Context, id, actor int64, in FinanceExpenseInput) (*FinanceExpense, error) {
	_, err := time.ParseInLocation("2006-01-02", in.OccurredOn, AffiliateTeamLocation)
	if !financeExpenseAmountPattern.MatchString(in.Amount) {
		return nil, infraerrors.BadRequest("FINANCE_EXPENSE_INVALID", "金额须为正数，最多 18 位整数和 2 位小数")
	}
	v, e := decimal.NewFromString(in.Amount)
	if err != nil || !financeCategory(in.Category) || e != nil || !v.IsPositive() || !v.Equal(v.Round(2)) || v.GreaterThanOrEqual(decimal.New(1, 18)) || in.AmortizeDays < 1 || in.AmortizeDays > 3660 || utf8.RuneCountInString(in.Note) > 500 || (in.AccountID != nil && *in.AccountID <= 0) {
		return nil, infraerrors.BadRequest("FINANCE_EXPENSE_INVALID", "支出日期、类别、金额、摊销天数或备注无效")
	}
	in.Amount = v.StringFixed(2)
	return s.repo.SaveExpense(ctx, id, actor, in)
}
func (s *FinanceService) DeleteExpense(ctx context.Context, id int64) error {
	return s.repo.DeleteExpense(ctx, id)
}

// FinanceAmortization 按八位小数截断，最后一天吸收尾差，总和严格等于原金额。
func FinanceAmortization(amount decimal.Decimal, days int, index int) decimal.Decimal {
	if days < 1 || index < 0 || index >= days {
		return decimal.Zero
	}
	daily := amount.Div(decimal.NewFromInt(int64(days))).Truncate(8)
	if index == days-1 {
		return amount.Sub(daily.Mul(decimal.NewFromInt(int64(days - 1))))
	}
	return daily
}

// financeAmounts 的 JSON 只用于输出，所有中间加法保持 decimal。
type financeAmounts map[string]decimal.Decimal

func (m financeAmounts) add(k string, v decimal.Decimal) { m[k] = m[k].Add(v) }
func (m financeAmounts) MarshalJSON() ([]byte, error) {
	v := map[string]any{}
	for k, d := range m {
		v[k] = financeMoney(d)
	}
	return json.Marshal(v)
}

var financeTotalKeys = []string{"revenue", "revenue_paid", "revenue_commission", "free_consumed", "overdraft_consumed", "consumption", "account_cost", "account_cost_internal", "commission", "expenses", "profit"}

func financeTotals() financeAmounts {
	m := financeAmounts{}
	for _, k := range financeTotalKeys {
		m[k] = decimal.Zero
	}
	return m
}
func financeFinalize(m financeAmounts) map[string]any {
	m["revenue"] = m["revenue_paid"].Add(m["revenue_commission"])
	m["profit"] = m["revenue"].Sub(m["account_cost"]).Sub(m["commission"]).Sub(m["expenses"])
	out := map[string]any{}
	for k, v := range m {
		out[k] = financeMoney(v)
	}
	out["profit_margin"] = nil
	if !m["revenue"].IsZero() {
		out["profit_margin"] = m["profit"].Div(m["revenue"]).StringFixed(4)
	}
	return out
}
func financeReplays(data *FinanceData) map[int64]FinanceRevenue {
	events := map[int64][]FinanceEvent{}
	for _, e := range data.Events {
		events[e.UserID] = append(events[e.UserID], e.Event)
	}
	for _, o := range data.Orders {
		if o.OrderType != "balance" {
			continue
		}
		events[o.UserID] = append(events[o.UserID], FinanceEvent{At: o.CompletedAt, ID: o.ID, Kind: "paid", OrderID: o.ID, Amount: o.Principal, Cash: o.Pay.Mul(decimal.NewFromInt(1).Sub(o.FeeRate))}, FinanceEvent{At: o.CompletedAt, ID: o.ID, Kind: "free", Source: "recharge_bonus", Amount: o.Bonus})
		if o.RefundAt != nil {
			events[o.UserID] = append(events[o.UserID], FinanceEvent{At: *o.RefundAt, ID: o.ID, Kind: "refund", OrderID: o.ID, Amount: o.RefundDeducted, Cash: o.Refund})
		}
	}
	for _, l := range data.Ledger {
		kind := ""
		if l.Action == "transfer" {
			kind = "commission"
		}
		if l.Action == "invitee_bonus" {
			kind = "free"
		}
		if kind != "" {
			events[l.UserID] = append(events[l.UserID], FinanceEvent{At: l.At, ID: l.ID, Kind: kind, Source: l.Action, Amount: l.Amount, Cash: l.Amount})
		}
	}
	out := map[int64]FinanceRevenue{}
	for _, u := range data.Users {
		out[u.ID] = ReplayFinanceRevenue(events[u.ID], u.Balance)
	}
	return out
}
func financeWarnings(data *FinanceData, r FinanceRange, replays map[int64]FinanceRevenue) []string {
	w := []string{}
	subs, legacy := 0, 0
	for _, o := range data.Orders {
		if r.Contains(o.CompletedAt) {
			if o.OrderType != "balance" {
				subs++
			}
			if o.LegacyFee || o.LegacyRefund {
				legacy++
			}
		}
	}
	if subs > 0 {
		w = append(w, "范围内存在订阅订单，未计入利润")
	}
	if legacy > 0 {
		w = append(w, "历史订单使用支付实例当前通道费率或退款扣回兼容值")
	}
	for _, l := range data.Ledger {
		if l.Action == "accrue" && r.Contains(l.At) && l.RewardType != "agent_direct" && l.RewardType != "agent_diff" && l.RewardType != "invite_percent" && l.RewardType != "invite_fixed" && l.RewardType != "invitee_bonus" {
			w = append(w, "存在未知类型分润，已计入总支出，未归入四种分类")
			break
		}
	}
	for _, v := range replays {
		if v.Difference.Abs().GreaterThanOrEqual(decimal.RequireFromString("0.01")) {
			w = append(w, "存在未登记入账或缺失流水，请查看用户对账；推断期初仅按免费余额处理")
			break
		}
	}
	for _, v := range replays {
		if v.Overdraft.IsPositive() {
			w = append(w, "存在透支消耗，超额部分未确认收入")
			break
		}
	}
	w = append(w, "余额消耗按北京日重放；历史用量或兑换流水被清理后无法完整还原收入")
	return w
}
func financeLabel(label string, deleted bool) string {
	if deleted {
		return label + "（已删除）"
	}
	return label
}
func financeSearch(u FinanceUser, q string) bool {
	// CUSTOM(fork): 数字查询仍匹配邮箱；ID 只允许纯数字并精确比较，禁止前缀匹配。
	if q == "" || strings.Contains(strings.ToLower(u.Email), strings.ToLower(q)) {
		return true
	}
	for _, ch := range q {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	id, err := strconv.ParseInt(q, 10, 64)
	return err == nil && u.ID == id
}
