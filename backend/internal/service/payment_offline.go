package service

// CUSTOM(fork): Offline receipts share payment accounting/rewards, but never a provider.
import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AdminOfflineRechargeInput also carries backfill receipt metadata; Amount is ignored on backfill.
type AdminOfflineRechargeInput struct {
	UserID         int64
	AdminID        int64
	Amount         float64
	PaidAmount     float64 `json:"paid_amount"`
	OfflineChannel string  `json:"offline_channel"`
	TradeNo        string  `json:"trade_no"`
	Notes          string  `json:"notes"`
}

// Offline amounts are CNY cents, capped independently of public checkout settings.
const maxOfflineRechargeAmount = 100000000.0

func validateOfflineAmount(value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > maxOfflineRechargeAmount || math.Abs(value*100-math.Round(value*100)) > 0.000001 {
		return infraerrors.BadRequest("OFFLINE_INVALID_AMOUNT", "amount must be positive, at most 100000000 and have no more than two decimal places")
	}
	return nil
}

func validateOfflineReceipt(in *AdminOfflineRechargeInput) error {
	if err := validateOfflineAmount(in.PaidAmount); err != nil {
		return err
	}
	switch in.OfflineChannel {
	case "wechat", "alipay", "bank", "other":
	default:
		return infraerrors.BadRequest("OFFLINE_INVALID_CHANNEL", "select a valid offline receipt channel")
	}
	in.TradeNo = strings.TrimSpace(in.TradeNo)
	if utf8.RuneCountInString(in.TradeNo) > 128 || len(in.Notes) > 10000 {
		return infraerrors.BadRequest("OFFLINE_INVALID_RECEIPT", "trade number or notes are too long")
	}
	return nil
}

func ValidateAdminBalanceKind(kind, operation string) error {
	if (kind != "" && kind != "gift" && kind != "offline") || (kind == "offline" && operation != "add") {
		return infraerrors.BadRequest("OFFLINE_INVALID_KIND", "offline recharge only supports adding balance; kind must be gift or offline")
	}
	return nil
}

func (s *PaymentService) AdminCreateOfflineRecharge(ctx context.Context, in AdminOfflineRechargeInput) (*User, error) {
	if err := validateOfflineAmount(in.Amount); err != nil {
		return nil, err
	}
	if err := validateOfflineReceipt(&in); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	u, err := s.userRepo.GetByID(txCtx, in.UserID)
	if err != nil {
		return nil, err
	}
	if err := s.lockOfflineTrade(txCtx, tx, in.TradeNo); err != nil {
		return nil, err
	}
	code, err := GenerateRedeemCode()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rc, err := tx.RedeemCode.Create().SetCode(code).SetType(AdjustmentTypeAdminBalance).
		SetValue(in.Amount).SetStatus(StatusUsed).SetUsedBy(u.ID).SetUsedAt(now).SetNotes(in.Notes).Save(txCtx)
	if err != nil {
		return nil, err
	}
	order, err := s.createOfflineOrder(txCtx, tx, in, u, rc, "OFFLINE_RECHARGE")
	if err != nil {
		return nil, err
	}
	change, err := s.userRepo.AdjustBalance(txCtx, u.ID, in.Amount)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	u.Balance = change.New
	if s.redeemService != nil {
		s.redeemService.invalidateRedeemCaches(ctx, u.ID, &RedeemCode{Type: RedeemTypeBalance})
	}
	// An already committed receipt must not be retried because a reward failed.
	// The shared helpers record failures and deduplicate by source order/user.
	_ = s.applyAffiliateRebateForOrder(ctx, order)
	s.applyFirstRechargeRewards(ctx, order)
	if s.affiliateService != nil {
		s.affiliateService.EvaluateInviterAfterRecharge(ctx, u.ID)
	}
	return u, nil
}

func (s *PaymentService) AdminBackfillOfflineRecharge(ctx context.Context, redeemID int64, in AdminOfflineRechargeInput) (*OfflineOrderSummary, error) {
	if err := validateOfflineReceipt(&in); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	query := tx.RedeemCode.Query().Where(redeemcode.IDEQ(redeemID), redeemcode.UsedByEQ(in.UserID))
	if s.entClient.Driver().Dialect() == dialect.Postgres {
		query.ForUpdate()
	}
	rc, err := query.Only(txCtx)
	if dbent.IsNotFound(err) {
		return nil, ErrRedeemCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	if rc.Type != AdjustmentTypeAdminBalance || rc.Value <= 0 || rc.UsedAt == nil || rc.Status != StatusUsed {
		return nil, infraerrors.BadRequest("OFFLINE_INVALID_HISTORY", "only used positive admin balance records can be backfilled")
	}
	if err := validateOfflineAmount(rc.Value); err != nil {
		return nil, err
	}
	exists, err := tx.PaymentOrder.Query().Where(paymentorder.RechargeCodeEQ(rc.Code)).Exist(txCtx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, infraerrors.Conflict("OFFLINE_ALREADY_LINKED", "this balance record is already linked to a payment order")
	}
	if err := s.lockOfflineTrade(txCtx, tx, in.TradeNo); err != nil {
		return nil, err
	}
	u, err := s.userRepo.GetByID(txCtx, in.UserID)
	if err != nil {
		return nil, err
	}
	in.Amount = rc.Value
	o, err := s.createOfflineOrder(txCtx, tx, in, u, rc, "OFFLINE_RECHARGE_BACKFILL")
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	// Deliberately no balance mutation, cache invalidation or rewards/upgrade evaluation.
	return offlineOrderSummary(o), nil
}

// Serialize check + insert across processes without adding a schema/index. All offline
// writers take this transaction-scoped advisory lock; blank trade numbers are exempt.
func (s *PaymentService) lockOfflineTrade(ctx context.Context, tx *dbent.Tx, trade string) error {
	if trade == "" {
		return nil
	}
	if s.entClient.Driver().Dialect() == dialect.Postgres {
		if _, err := tx.Client().ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "offline-receipt:"+trade); err != nil {
			return err
		}
	}
	exists, err := tx.PaymentOrder.Query().Where(paymentorder.PaymentTypeEQ(payment.TypeOffline), paymentorder.PaymentTradeNoEQ(trade)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		return infraerrors.Conflict("OFFLINE_DUPLICATE_TRADE", "this offline trade number already exists; possible duplicate receipt")
	}
	return nil
}

func (s *PaymentService) createOfflineOrder(ctx context.Context, tx *dbent.Tx, in AdminOfflineRechargeInput, u *User, rc *dbent.RedeemCode, action string) (*dbent.PaymentOrder, error) {
	number, err := s.allocateOutTradeNo(ctx, tx)
	if err != nil {
		return nil, err
	}
	at := *rc.UsedAt
	o, err := tx.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName(u.Username).
		SetAmount(in.Amount).SetPayAmount(in.PaidAmount).SetBonusAmount(math.Max(0, math.Round((in.Amount-in.PaidAmount)*100)/100)).
		SetFeeRate(0).SetChannelFeeRate("0").SetRechargeCode(rc.Code).SetOutTradeNo(number).
		SetPaymentType(payment.TypeOffline).SetProviderKey(payment.TypeOffline).SetPaymentTradeNo(in.TradeNo).
		SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusCompleted).
		SetPaidAt(at).SetCompletedAt(at).SetCreatedAt(at).SetExpiresAt(at).SetClientIP("").SetSrcHost("").Save(ctx)
	if err != nil {
		return nil, err
	}
	// Receipt metadata is essential accounting evidence: persist with the receipt, not best-effort.
	detail, err := json.Marshal(map[string]any{"offline_channel": in.OfflineChannel, "admin_id": in.AdminID, "notes": in.Notes, "redeem_id": rc.ID})
	if err != nil {
		return nil, err
	}
	_, err = tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(o.ID, 10)).SetAction(action).
		SetOperator(fmt.Sprintf("admin:%d", in.AdminID)).SetDetail(string(detail)).Save(ctx)
	return o, err
}

// CUSTOM(fork): History and backfill share receipt status and balance-denominated refunds.
type OfflineOrderSummary struct {
	ID           int64   `json:"id"`
	OutTradeNo   string  `json:"out_trade_no"`
	PaidAmount   float64 `json:"paid_amount"`
	Amount       float64 `json:"amount"`
	Status       string  `json:"status"`
	RefundAmount float64 `json:"refund_amount"`
}

func offlineOrderSummary(o *dbent.PaymentOrder) *OfflineOrderSummary {
	return &OfflineOrderSummary{
		ID: o.ID, OutTradeNo: o.OutTradeNo, PaidAmount: o.PayAmount,
		Amount: o.Amount, Status: o.Status, RefundAmount: o.RefundAmount,
	}
}

// Batch query avoids an N+1 query per history row. Any linked order prevents backfill.
func (s *PaymentService) AdminBalanceHistoryOrders(ctx context.Context, codes []RedeemCode) (map[string]*OfflineOrderSummary, map[string]bool, error) {
	summaries := make(map[string]*OfflineOrderSummary)
	linked := make(map[string]bool)
	keys := make([]string, 0, len(codes))
	for _, code := range codes {
		if code.Type == AdjustmentTypeAdminBalance {
			keys = append(keys, code.Code)
		}
	}
	if len(keys) == 0 {
		return summaries, linked, nil
	}
	orders, err := s.entClient.PaymentOrder.Query().Where(paymentorder.RechargeCodeIn(keys...)).All(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, o := range orders {
		linked[o.RechargeCode] = true
		if o.PaymentType == payment.TypeOffline {
			summaries[o.RechargeCode] = offlineOrderSummary(o)
		}
	}
	return summaries, linked, nil
}
