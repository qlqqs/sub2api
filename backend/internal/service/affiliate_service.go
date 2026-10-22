package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid     = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken       = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound    = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateSelfInvite      = infraerrors.BadRequest("AFFILIATE_SELF_INVITE", "user cannot invite themselves")
	ErrAffiliateBindReason      = infraerrors.BadRequest("AFFILIATE_BIND_REASON_INVALID", "binding reason must be between 1 and 500 characters")
	ErrAffiliateQuotaEmpty      = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")

	ErrAffiliateQuotaInsufficient     = infraerrors.BadRequest("AFFILIATE_QUOTA_INSUFFICIENT", "insufficient available affiliate quota")
	ErrAffiliateWithdrawAmountInvalid = infraerrors.BadRequest("AFFILIATE_WITHDRAW_AMOUNT_INVALID", "invalid offline withdrawal amount")
	// ErrAffiliateCycle：绑定后邀请链或归属链会经过被绑定人本人。
	ErrAffiliateCycle = infraerrors.BadRequest("AFFILIATE_CYCLE", "binding would create an affiliate cycle")
)

const (
	affiliateInviteesLimit = 100
	// AffiliateCodeMinLength / AffiliateCodeMaxLength bound both system-generated
	// 12-char codes and admin-customized codes (e.g. "VIP2026").
	AffiliateCodeMinLength = 4
	AffiliateCodeMaxLength = 32
)

// affiliateCodeValidChar accepts uppercase letters, digits, underscore and dash.
// All input passes through strings.ToUpper before validation, so lowercase from
// users is normalized — admins may supply mixed case in their UI.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for c := byte('A'); c <= 'Z'; c++ {
		tbl[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		tbl[c] = true
	}
	tbl['_'] = true
	tbl['-'] = true
	return tbl
}()

// isValidAffiliateCodeFormat validates code format for both binding (user input)
// and admin updates. Caller is expected to upper-case the input first.
func isValidAffiliateCodeFormat(code string) bool {
	if len(code) < AffiliateCodeMinLength || len(code) > AffiliateCodeMaxLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID               int64      `json:"user_id"`
	AffCode              string     `json:"aff_code"`
	AffCodeCustom        bool       `json:"aff_code_custom"`
	AffRebateRatePercent *float64   `json:"aff_rebate_rate_percent,omitempty"`
	InviterID            *int64     `json:"inviter_id,omitempty"`
	InviterBoundAt       *time.Time `json:"inviter_bound_at,omitempty"`
	InviterBoundSource   *string    `json:"inviter_bound_source,omitempty"`
	InviterBoundBy       *int64     `json:"inviter_bound_by,omitempty"`
	InviterBindReason    *string    `json:"inviter_bind_reason,omitempty"`
	AffCount             int        `json:"aff_count"`
	AffQuota             float64    `json:"aff_quota"`
	AffFrozenQuota       float64    `json:"aff_frozen_quota"`
	AffHistoryQuota      float64    `json:"aff_history_quota"`
	// AgentLevel：1 一级代理，2 二级代理，3 普通用户（默认）。
	AgentLevel          int        `json:"agent_level"`
	AgentLevelLocked    bool       `json:"agent_level_locked"`
	AgentLevelSource    string     `json:"agent_level_source"`
	AgentLevelUpdatedAt *time.Time `json:"agent_level_updated_at,omitempty"`
	AgentLevelUpdatedBy *int64     `json:"agent_level_updated_by,omitempty"`
	// 绑定时写死的分润结构快照（见 AffiliateOwnerSnapshot）。
	AgentOwnerID      *int64     `json:"agent_owner_id,omitempty"`
	AgentOwnerLevel   *int       `json:"agent_owner_level,omitempty"`
	AgentUplineID     *int64     `json:"agent_upline_id,omitempty"`
	AgentOwnerBoundAt *time.Time `json:"agent_owner_bound_at,omitempty"`
	// RebateSettledAt / RebateSettledConsumption 是按消费结算返利的水位。
	RebateSettledAt          time.Time `json:"-"`
	RebateSettledConsumption float64   `json:"-"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// AffiliateAccrueSource 标识一笔返利的来源：充值订单（OrderID）或消费结算
// （UsageUntil 为本次结算覆盖到的 usage_logs.created_at 上界）。两者都为 nil
// 时来自兑换码或管理员加款。
type AffiliateAccrueSource struct {
	OrderID    *int64
	UsageUntil *time.Time
}

// AffiliatePendingConsumption 是一个用户待结算的余额消费快照（行锁内读取）。
type AffiliatePendingConsumption struct {
	UserID int64
	// SettledAt 是上次结算的水位；UntilAt 是本次可推进到的水位（当前时间减去滞后）。
	SettledAt time.Time
	UntilAt   time.Time
	// DeltaCost 是 (SettledAt, UntilAt] 区间内的余额消费合计。
	DeltaCost float64
	// PaidTotal 是真实支付的充值总额（扣除赠送与已退款）；SettledConsumption 是
	// 之前已计入返利的累计消费。两者之差是本次最多可计入返利的额度。
	PaidTotal          float64
	SettledConsumption float64
}

type AffiliateInviterBinding struct {
	Source  string
	BoundBy *int64
	Reason  *string
	// Owner 是绑定时解析出的分润结构快照，与邀请人一起写入。
	Owner AffiliateOwnerSnapshot
}

// AffiliateOwnerSnapshot 是一条邀请关系在绑定时写死的分润结构：归属代理、归属代理
// 当时的档位，以及归属代理为二级时其上方最近的一级代理。之后任何人升降级都不改它。
type AffiliateOwnerSnapshot struct {
	OwnerID    *int64
	OwnerLevel *int
	UplineID   *int64
}

// AffiliateNode 是解析归属链与计算分润时需要的单个用户信息。Active 为 false 表示
// 账号已删除（结算时跳过其份额，但仍可沿其快照继续向上找）。
type AffiliateNode struct {
	UserID       int64
	AgentLevel   int
	AgentOwnerID *int64
	InviterID    *int64
	CustomRate   *float64
	Active       bool
	Email        string
}

// AffiliateAgentRef 是列表与总览里展示的归属代理 / 上方一级信息（绑定时快照）。
type AffiliateAgentRef struct {
	AgentOwnerID       *int64 `json:"agent_owner_id"`
	AgentOwnerEmail    string `json:"agent_owner_email"`
	AgentOwnerLevel    *int   `json:"agent_owner_level"`
	AgentOwnerDeleted  bool   `json:"agent_owner_deleted"`
	AgentUplineID      *int64 `json:"agent_upline_id"`
	AgentUplineEmail   string `json:"agent_upline_email"`
	AgentUplineDeleted bool   `json:"agent_upline_deleted"`
}

// AffiliateRewardTotals 是某用户作为受益人收到的奖励按类别的合计。
type AffiliateRewardTotals struct {
	Agent        float64 `json:"agent"`
	Invite       float64 `json:"invite"`
	InviteeBonus float64 `json:"invitee_bonus"`
}

type AffiliateInviterBindingResult struct {
	InviteeID int64     `json:"invitee_id"`
	InviterID int64     `json:"inviter_id"`
	BoundAt   time.Time `json:"bound_at"`
	Source    string    `json:"source"`
}

type AffiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
	AgentLevel  int        `json:"agent_level"`
	// OwnedByMe：该好友的归属代理是我；AppointedByMe：该好友是我任命的二级；
	// Appointable：我现在可以把该好友设为二级。
	OwnedByMe     bool `json:"owned_by_me"`
	AppointedByMe bool `json:"appointed_by_me"`
	Appointable   bool `json:"appointable"`

	AgentOwnerID        *int64 `json:"-"`
	AgentLevelSource    string `json:"-"`
	AgentLevelLocked    bool   `json:"-"`
	AgentLevelUpdatedBy *int64 `json:"-"`
}

// AffiliateInviteRewardInfo 是普通用户邀请好友后自己能得的奖励（展示用）。
type AffiliateInviteRewardInfo struct {
	Mode          string  `json:"mode"`
	RatePercent   float64 `json:"rate_percent"`
	DurationDays  int     `json:"duration_days"`
	PerInviteeCap float64 `json:"per_invitee_cap"`
	FixedAmount   float64 `json:"fixed_amount"`
	MinRecharge   float64 `json:"min_recharge"`
}

// AffiliateInviteeBonusInfo 是被邀请人首充奖励（展示用）。
type AffiliateInviteeBonusInfo struct {
	Mode        string  `json:"mode"`
	Value       float64 `json:"value"`
	MinRecharge float64 `json:"min_recharge"`
	Max         float64 `json:"max"`
}

// AffiliateAppointInfo 是一级代理任命二级的额度（Max=0 表示不限）。
type AffiliateAppointInfo struct {
	Max  int `json:"max"`
	Used int `json:"used"`
}

type AffiliateDetail struct {
	UserID          int64   `json:"user_id"`
	AffCode         string  `json:"aff_code"`
	InviterID       *int64  `json:"inviter_id,omitempty"`
	AffCount        int     `json:"aff_count"`
	AffQuota        float64 `json:"aff_quota"`
	AffFrozenQuota  float64 `json:"aff_frozen_quota"`
	AffHistoryQuota float64 `json:"aff_history_quota"`
	RebateBase      string  `json:"rebate_base"`
	// AgentLevel：1 一级代理，2 二级代理，3 普通用户。
	AgentLevel       int  `json:"agent_level"`
	AgentLevelLocked bool `json:"agent_level_locked"`
	// 自动升级模式下的进度：NextAgentLevel/NextLevelThreshold 为 nil 表示没有可自动升到的等级。
	AgentUpgradeMode   string `json:"agent_upgrade_mode"`
	NextAgentLevel     *int   `json:"next_agent_level,omitempty"`
	NextLevelThreshold *int   `json:"next_level_threshold,omitempty"`
	CurrentInviteCount int    `json:"current_invite_count"`
	// AgentRatePercent 仅代理可见：已套专属比例的本档比例。普通用户不下发任何代理比例。
	AgentRatePercent   *float64 `json:"agent_rate_percent,omitempty"`
	OwnedCustomerCount int      `json:"owned_customer_count"`
	// InviteReward 仅普通用户且模式非 off 时下发；代理邀请的人只产生分润。
	InviteReward *AffiliateInviteRewardInfo `json:"invite_reward,omitempty"`
	// InviteeBonus 是好友（被邀请人）首充能得什么；FirstRechargeBonus 是我自己尚可领取的首充奖励。
	InviteeBonus       *AffiliateInviteeBonusInfo `json:"invitee_bonus,omitempty"`
	FirstRechargeBonus *AffiliateInviteeBonusInfo `json:"first_recharge_bonus,omitempty"`
	RewardTotals       AffiliateRewardTotals      `json:"reward_totals"`
	Appoint            *AffiliateAppointInfo      `json:"appoint,omitempty"`
	Invitees           []AffiliateInvitee         `json:"invitees"`
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	// BindInviter 绑定邀请人并写入分润结构快照，同时把消费结算水位重置为绑定时刻。
	BindInviter(ctx context.Context, userID, inviterID int64, binding AffiliateInviterBinding) (*AffiliateInviterBindingResult, bool, error)
	// AccrueQuota 给受益人返利额度入账并写流水。首充类奖励（invite_fixed）命中唯一索引时
	// 视为已发放，返回 false。
	AccrueQuota(ctx context.Context, beneficiaryID, sourceUserID int64, amount float64, freezeHours int, source AffiliateAccrueSource, rewardType string) (bool, error)
	// GetAccruedRebateFromInvitee 汇总受益人从某来源用户已获得的指定类型奖励。
	GetAccruedRebateFromInvitee(ctx context.Context, beneficiaryID, sourceUserID int64, rewardTypes []string) (float64, error)
	// GetAffiliateNodes 批量读取用户的等级、快照与专属比例；不存在 user_affiliates 行的用户不在结果中。
	GetAffiliateNodes(ctx context.Context, userIDs []int64) (map[int64]AffiliateNode, error)
	// CreditInviteeBonus 把被邀请人首充奖励直接加到用户余额并写流水；已发过返回 false。
	CreditInviteeBonus(ctx context.Context, userID int64, amount float64, sourceOrderID *int64) (bool, error)
	// HasRewardForSource 判断某来源用户是否已产生过指定类型的奖励流水。
	HasRewardForSource(ctx context.Context, sourceUserID int64, rewardType string) (bool, error)
	CountOwnedCustomers(ctx context.Context, agentID int64) (int, error)
	SumRewardTotals(ctx context.Context, beneficiaryID int64) (AffiliateRewardTotals, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	WithdrawQuota(ctx context.Context, userID int64, amount float64, operationID string) (*AffiliateWithdrawResult, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error)

	// 代理等级
	// CountQualifiedInvitees 统计直属邀请人数；rechargedOnly 时只算有过真实支付充值的。
	CountQualifiedInvitees(ctx context.Context, inviterID int64, rechargedOnly bool) (int, error)
	// PromoteAgentLevelAuto 把未锁定且等级低于 level 的用户自动升到 level，返回是否升级。
	PromoteAgentLevelAuto(ctx context.Context, userID int64, level int) (bool, error)
	// SetAgentLevelManual 管理员设置等级：降级时锁定，升级时解锁，相同等级只改来源。
	SetAgentLevelManual(ctx context.Context, userIDs []int64, level int, operatorID int64) error
	// GetAffiliateForUpdate 在事务内锁定并读取用户的 user_affiliates 行。
	GetAffiliateForUpdate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	// SetAgentLevelByAgent 一级代理任命 / 取消二级：来源记为 agent，locked 控制是否锁定自动升级。
	SetAgentLevelByAgent(ctx context.Context, targetID int64, level int, appointerID int64, locked bool) error
	// CountAgentAppointments 统计某一级代理当前任命的二级人数。
	CountAgentAppointments(ctx context.Context, appointerID int64) (int, error)
	SetAgentLevelLocked(ctx context.Context, userIDs []int64, locked bool) error
	// ListInviterIDs 按 user_id 升序分页列出有直属下级的用户，用于重新评估等级。
	ListInviterIDs(ctx context.Context, afterUserID int64, limit int) ([]int64, error)

	// 按消费结算
	WithTx(ctx context.Context, fn func(txCtx context.Context) error) error
	// ListUsersWithPendingConsumption 列出有邀请人且水位之后有新余额消费的用户。
	ListUsersWithPendingConsumption(ctx context.Context, lag time.Duration, limit int) ([]int64, error)
	// LockPendingConsumption 在事务内锁定用户行并读取待结算消费快照。
	LockPendingConsumption(ctx context.Context, userID int64, lag time.Duration) (*AffiliatePendingConsumption, error)
	AdvanceConsumptionWatermark(ctx context.Context, userID int64, until time.Time, settledConsumption float64) error

	// 管理端：用户级专属配置
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAgentCustomers(ctx context.Context, agentID int64, page, pageSize int) ([]AffiliateAgentCustomer, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

// AffiliateAdminFilter 代理管理列表筛选条件。Search、AgentLevel、OwnerID 都为空时只列
// 代理与有专属设置的用户；任一存在时按条件查全部用户。
type AffiliateAdminFilter struct {
	Search     string
	AgentLevel int   // 0 = 不限
	OwnerID    int64 // 0 = 不限
	Page       int
	PageSize   int
}

// AffiliateAdminEntry 代理管理列表条目
type AffiliateAdminEntry struct {
	UserID                   int64    `json:"user_id"`
	Email                    string   `json:"email"`
	Username                 string   `json:"username"`
	AffCode                  string   `json:"aff_code"`
	AffCodeCustom            bool     `json:"aff_code_custom"`
	AffRebateRatePercent     *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount                 int      `json:"aff_count"`
	AgentLevel               int      `json:"agent_level"`
	AgentLevelLocked         bool     `json:"agent_level_locked"`
	AgentLevelSource         string   `json:"agent_level_source"`
	AgentLevelUpdatedBy      *int64   `json:"agent_level_updated_by"`
	AgentLevelUpdatedByEmail string   `json:"agent_level_updated_by_email"`
	AffiliateAgentRef
	OwnedCustomerCount int     `json:"owned_customer_count"`
	AgentProfitTotal   float64 `json:"agent_profit_total"`
}

// AffiliateAgentCustomer 是归属某代理（owner）或以其为上方一级（upline）的客户。
type AffiliateAgentCustomer struct {
	UserID       int64      `json:"user_id"`
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	AgentLevel   int        `json:"agent_level"`
	Relation     string     `json:"relation"`
	BoundAt      *time.Time `json:"bound_at"`
	Contribution float64    `json:"contribution"`
}

type AffiliateRecordFilter struct {
	Search     string
	Page       int
	PageSize   int
	StartAt    *time.Time
	EndAt      *time.Time
	SortBy     string
	SortDesc   bool
	RewardType string // 仅返利记录使用
}

type AffiliateInviteRecord struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
	AffiliateAgentRef
}

// AffiliateRebateRecord 是一笔返利入账流水。非订单来源（兑换码、管理员充值）
// 的流水没有订单信息，OrderID/OrderAmount/PayAmount 为 nil；被邀请人账号被删除时
// InviteeID 为 nil。
type AffiliateRebateRecord struct {
	OrderID         *int64    `json:"order_id"`
	OutTradeNo      string    `json:"out_trade_no"`
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       *int64    `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	OrderAmount     *float64  `json:"order_amount"`
	PayAmount       *float64  `json:"pay_amount"`
	RebateAmount    float64   `json:"rebate_amount"`
	RewardType      *string   `json:"reward_type"`
	PaymentType     string    `json:"payment_type"`
	OrderStatus     string    `json:"order_status"`
	CreatedAt       time.Time `json:"created_at"`
}

// AffiliateTransferRecord 是一笔返利额度流出：Action 为 transfer（用户转入余额）
// 或 withdraw（管理员登记的线下提现）。
type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	Action              string    `json:"action"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

// AffiliateWithdrawResult 是线下提现登记完成后的流水 ID 与额度快照。
// Replayed 表示本次请求命中了同一幂等标识的已有登记，返回的是首次登记的结果。
type AffiliateWithdrawResult struct {
	LedgerID            int64   `json:"ledger_id"`
	UserID              int64   `json:"user_id"`
	Amount              float64 `json:"amount"`
	AvailableQuotaAfter float64 `json:"available_quota_after"`
	FrozenQuotaAfter    float64 `json:"frozen_quota_after"`
	HistoryQuotaAfter   float64 `json:"history_quota_after"`
	Replayed            bool    `json:"-"`
}

type AffiliateUserOverview struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	AffCode             string  `json:"aff_code"`
	InvitedCount        int     `json:"invited_count"`
	RebatedInviteeCount int     `json:"rebated_invitee_count"`
	AvailableQuota      float64 `json:"available_quota"`
	HistoryQuota        float64 `json:"history_quota"`
	AgentLevel          int     `json:"agent_level"`
	AgentLevelLocked    bool    `json:"agent_level_locked"`
	AgentLevelSource    string  `json:"agent_level_source"`
	// AgentRatePercent 仅代理有值：已套专属比例的本档比例。
	AgentRatePercent *float64 `json:"agent_rate_percent"`
	CustomRate       *float64 `json:"-"`
	AffiliateAgentRef
	OwnedCustomerCount int                   `json:"owned_customer_count"`
	RewardTotals       AffiliateRewardTotals `json:"reward_totals"`
}

type AffiliateService struct {
	repo                 AffiliateRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingService *SettingService, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	return &AffiliateService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

// IsEnabled reports whether the affiliate (邀请返利) feature is turned on.
func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return AffiliateEnabledDefault
	}
	return s.settingService.IsAffiliateEnabled(ctx)
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

// rewardConfig 返回当前奖励配置；SettingService 不可用时使用默认配置。
func (s *AffiliateService) rewardConfig(ctx context.Context) AffiliateRewardConfig {
	if s == nil || s.settingService == nil {
		return defaultAffiliateRewardConfig()
	}
	return s.settingService.GetAffiliateRewardConfig(ctx)
}

// rebateBase 返回返利基数（充值 / 消费）；SettingService 不可用时按充值处理，
// 与历史行为一致。
func (s *AffiliateService) rebateBase(ctx context.Context) string {
	if s == nil || s.settingService == nil {
		return AffiliateRebateBaseRecharge
	}
	return s.settingService.GetAffiliateRebateBase(ctx)
}

// AccrueInviteRebate 按充值入账分润与邀请奖励（兑换码、管理员充值）。仅在返利基数为
// 「充值」时生效；基数为「消费」时由 SettleConsumptionRebates 定时按消费结算。
func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, nil)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	// 总开关关闭时，新充值不再产生返利
	if !s.IsEnabled(ctx) {
		return 0, nil
	}
	if s.rebateBase(ctx) != AffiliateRebateBaseRecharge {
		return 0, nil
	}

	consumer, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	return s.distributeRewards(ctx, consumer, baseRechargeAmount, AffiliateAccrueSource{OrderID: sourceOrderID})
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, affiliateInviteesLimit)
	if err != nil {
		return nil, err
	}
	for i := range invitees {
		invitees[i].Email = maskEmail(invitees[i].Email)
	}
	return invitees, nil
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

// =========================
// Admin: 专属配置管理
// =========================

// validateExclusiveRate ensures a per-user override is finite and within
// [Min, Max]. nil is always valid (means "clear / fall back to global").
func validateExclusiveRate(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	v := *ratePercent
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return infraerrors.BadRequest("INVALID_RATE", "invalid rebate rate")
	}
	if v < AffiliateRebateRateMin || v > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	return nil
}

// AdminUpdateUserAffCode 管理员改写用户的邀请码（专属邀请码）。
func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, rawCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	return s.repo.UpdateUserAffCode(ctx, userID, code)
}

// AdminResetUserAffCode 重置用户邀请码为系统随机码。
func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ResetUserAffCode(ctx, userID)
}

// AdminSetUserRebateRate 设置/清除用户专属返利比例。ratePercent==nil 表示清除。
func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	return s.repo.SetUserRebateRate(ctx, userID, ratePercent)
}

// AdminBatchSetUserRebateRate 批量设置/清除用户专属返利比例。
func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserRebateRate(ctx, cleaned, ratePercent)
}

// AdminWithdrawQuota 登记一笔已在站外打款的线下提现：从用户可提取返利额度
// 中扣除 amount，并写入 action=withdraw 的流水。冻结期内的额度不可扣除。
// 金额按返利流水精度保留 8 位小数，舍入后必须为正。
//
// idempotencyKey 标识一次登记且必填：同一个键只扣减一次，重复请求在用户与金额
// 一致时返回首次登记的结果，不一致时返回 ErrIdempotencyKeyConflict。
func (s *AffiliateService) AdminWithdrawQuota(ctx context.Context, userID int64, amount float64, idempotencyKey string) (*AffiliateWithdrawResult, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	key, err := NormalizeIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	amount = roundTo(amount, 8)
	if amount <= 0 || math.IsInf(amount, 0) {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	return s.repo.WithdrawQuota(ctx, userID, amount, affiliateWithdrawOperationID(key))
}

// affiliateWithdrawOperationID 把线下提现登记的幂等键映射为流水的 operation_id，
// 同一个键始终得到同一个值。
func affiliateWithdrawOperationID(idempotencyKey string) string {
	sum := sha256.Sum256([]byte("admin.affiliates.withdraw\x00" + idempotencyKey))
	return hex.EncodeToString(sum[:])
}

// AdminListCustomUsers 列出有专属配置的用户。
func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListUsersWithCustomSettings(ctx, filter)
}

// AdminListAgentCustomers 列出归属该代理或以其为上方一级的客户。
func (s *AffiliateService) AdminListAgentCustomers(ctx context.Context, agentID int64, page, pageSize int) ([]AffiliateAgentCustomer, int64, error) {
	if agentID <= 0 {
		return nil, 0, errInvalidUser()
	}
	if s == nil || s.repo == nil {
		return nil, 0, errAffiliateUnavailable()
	}
	return s.repo.ListAgentCustomers(ctx, agentID, page, pageSize)
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateInviteRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	filter = normalizeAffiliateRecordFilter(filter)
	filter.RewardType = normalizeAffiliateRewardTypeFilter(filter.RewardType)
	return s.repo.ListAffiliateRebateRecords(ctx, filter)
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	overview, err := s.repo.GetAffiliateUserOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	if overview == nil {
		return nil, nil
	}
	overview.AgentLevel = normalizeAgentLevel(overview.AgentLevel)
	if overview.AgentLevel != AffiliateAgentLevelDefault {
		cfg := s.rewardConfig(ctx)
		rate := agentTierRate(cfg, overview.AgentLevel, overview.CustomRate)
		overview.AgentRatePercent = &rate
	}
	if overview.OwnedCustomerCount, err = s.repo.CountOwnedCustomers(ctx, userID); err != nil {
		return nil, err
	}
	if overview.RewardTotals, err = s.repo.SumRewardTotals(ctx, userID); err != nil {
		return nil, err
	}
	return overview, nil
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}
