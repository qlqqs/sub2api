package service

// CUSTOM(fork): 合伙人区域、利润结算及管理端口。
import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrPartnerUserNotFound   = infraerrors.NotFound("USER_NOT_FOUND", "用户不存在")
	ErrPartnerNotFound       = infraerrors.NotFound("PARTNER_NOT_FOUND", "合伙人不存在")
	ErrPartnerForbidden      = infraerrors.Forbidden("PARTNER_FORBIDDEN", "无权访问该合伙人区域")
	ErrPartnerExists         = infraerrors.Conflict("PARTNER_EXISTS", "该用户已是合伙人")
	ErrPartnerAttached       = infraerrors.Conflict("PARTNER_AGENT_ATTACHED", "该渠道经理已挂靠合伙人")
	ErrPartnerAgentIsPartner = infraerrors.Conflict("PARTNER_AGENT_IS_PARTNER", "合伙人不能挂靠其他合伙人")
	ErrPartnerAgentLevel     = infraerrors.BadRequest("PARTNER_AGENT_NOT_LEVEL1", "只能挂靠现有渠道经理")
	ErrPartnerLevelFixed     = infraerrors.Conflict("PARTNER_LEVEL_FIXED", "合伙人的渠道经理档位已锁定")
	ErrPartnerCannotAppoint  = infraerrors.Forbidden("PARTNER_CANNOT_APPOINT", "合伙人不能任命或取消代理")
	ErrPartnerMonthRange     = infraerrors.BadRequest("PARTNER_MONTH_OUT_OF_RANGE", "月份不在合伙人结算范围内")
	ErrPartnerMonthConfirmed = infraerrors.Conflict("PARTNER_MONTH_CONFIRMED", "该月份已确认，不能修改")
)

type PartnerUserRef struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Deleted  bool   `json:"deleted"`
}
type Partner struct {
	UserID     int64
	RegionName string
	CreatedAt  time.Time
}
type PartnerRate struct {
	PartnerID int64
	Month     string
	Rate      decimal.Decimal
}
type PartnerAttachment struct {
	AgentID, PartnerID int64
	AttachedAt         time.Time
	AttachedByEmail    *string
	Source             string
}
type PartnerSnapshot struct {
	User              PartnerUserRef
	AgentLevel        int
	OwnerID, UplineID *int64
	BoundAt           *time.Time
	CreatedAt         time.Time
}
type PartnerMember struct {
	PartnerID, LineAgentID int64
	StartAt                time.Time
}
type PartnerInfo struct {
	RegionName    string    `json:"region_name"`
	ShareRate     string    `json:"share_rate"`
	NextShareRate *string   `json:"next_share_rate"`
	NextRateMonth *string   `json:"next_rate_month"`
	Since         time.Time `json:"since"`
}
type PartnerRegionRef struct {
	UserID     int64  `json:"user_id"`
	RegionName string `json:"region_name"`
}
type PartnerCurrentRegion struct {
	Partner    PartnerUserRef `json:"partner"`
	RegionName string         `json:"region_name"`
}
type PartnerTeamCount struct {
	Owned     int `json:"owned"`
	ViaUpline int `json:"via_upline"`
}
type PartnerPreview struct {
	User          PartnerUserRef        `json:"user"`
	AgentLevel    int                   `json:"agent_level"`
	IsPartner     bool                  `json:"is_partner"`
	IsAttached    bool                  `json:"is_attached"`
	Team          PartnerTeamCount      `json:"team"`
	CurrentRegion *PartnerCurrentRegion `json:"current_region"`
	CanCreate     bool                  `json:"can_create"`
	Reason        *string               `json:"reason"`
}
type PartnerAttachPreview struct {
	Agent         PartnerUserRef        `json:"agent"`
	AgentLevel    int                   `json:"agent_level"`
	Team          PartnerTeamCount      `json:"team"`
	CurrentRegion *PartnerCurrentRegion `json:"current_region"`
	CanAttach     bool                  `json:"can_attach"`
	Reason        *string               `json:"reason"`
}
type PartnerAutoAttachment struct {
	AgentUserID int64          `json:"agent_user_id"`
	Partner     PartnerUserRef `json:"partner"`
	RegionName  string         `json:"region_name"`
}
type PartnerMonthTotals struct {
	Month         string     `json:"month"`
	Status        string     `json:"status"`
	ShareRate     string     `json:"share_rate"`
	Consumption   string     `json:"consumption"`
	Revenue       string     `json:"revenue"`
	AccountCost   string     `json:"account_cost"`
	Commission    string     `json:"commission"`
	RegionExpense string     `json:"region_expense"`
	Adjustment    string     `json:"adjustment"`
	Profit        string     `json:"profit"`
	LossBefore    string     `json:"loss_before"`
	LossAfter     string     `json:"loss_after"`
	ShareBase     string     `json:"share_base"`
	ShareAmount   string     `json:"share_amount"`
	ConfirmedAt   *time.Time `json:"confirmed_at"`
	PaidOn        *string    `json:"paid_on"`
	PaidNote      *string    `json:"paid_note"`
}
type PartnerLineAmounts struct {
	Consumption string `json:"consumption"`
	Revenue     string `json:"revenue"`
	AccountCost string `json:"account_cost"`
	Commission  string `json:"commission"`
	Profit      string `json:"profit"`
}
type PartnerLine struct {
	LineAgent *PartnerUserRef `json:"line_agent"`
	Members   int             `json:"members"`
	PartnerLineAmounts
}
type PartnerCostGroup struct {
	GroupID     int64  `json:"group_id"`
	GroupName   string `json:"group_name"`
	Consumption string `json:"consumption"`
	AccountCost string `json:"account_cost"`
}
type PartnerCostModel struct {
	Model       string `json:"model"`
	Consumption string `json:"consumption"`
	AccountCost string `json:"account_cost"`
}
type PartnerAdjustment struct {
	ID             int64     `json:"id"`
	PartnerID      int64     `json:"-"`
	Month          string    `json:"-"`
	Amount         string    `json:"amount"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedByEmail string    `json:"created_by_email,omitempty"`
}
type PartnerStatementDetail struct {
	Totals      PartnerMonthTotals  `json:"totals"`
	Lines       []PartnerLine       `json:"lines"`
	CostByGroup []PartnerCostGroup  `json:"cost_by_group"`
	CostByModel []PartnerCostModel  `json:"cost_by_model"`
	Adjustments []PartnerAdjustment `json:"adjustments"`
	Warnings    []string            `json:"warnings"`
}
type PartnerItem struct {
	User PartnerUserRef `json:"user"`
	PartnerInfo
	AgentCount        int                `json:"agent_count"`
	MemberCount       int                `json:"member_count"`
	LossCarried       string             `json:"loss_carried"`
	UnconfirmedMonths int                `json:"unconfirmed_months"`
	CurrentMonth      PartnerMonthTotals `json:"current_month"`
}
type PartnerAgentItem struct {
	User            PartnerUserRef     `json:"user"`
	AttachedAt      time.Time          `json:"attached_at"`
	Source          string             `json:"source"`
	AttachedByEmail *string            `json:"attached_by_email"`
	AgentLevel      int                `json:"agent_level"`
	Team            PartnerTeamCount   `json:"team"`
	CurrentMonth    PartnerLineAmounts `json:"current_month"`
}
type PartnerAdminDetail struct {
	PartnerItem
	Agents []PartnerAgentItem `json:"agents"`
}
type PartnerOverview struct {
	PartnerInfo
	LossCarried  string             `json:"loss_carried"`
	AgentCount   int                `json:"agent_count"`
	MemberCount  int                `json:"member_count"`
	CurrentMonth PartnerMonthTotals `json:"current_month"`
	Lines        []PartnerLine      `json:"lines"`
}
type PartnerState struct {
	Partners    map[int64]Partner
	Rates       []PartnerRate
	Attachments map[int64]PartnerAttachment
	Snapshots   map[int64]PartnerSnapshot
	Adjustments []PartnerAdjustment
	Statements  map[int64]map[string]PartnerStatementDetail
}
type PartnerUsage struct {
	UserID, GroupID       int64
	Day, GroupName, Model string
	Cost, Consumption     decimal.Decimal
	FixedRequests         int64
}
type PartnerStartDay struct {
	Total, After decimal.Decimal
	Requests     int64
}
type PartnerMonthsData struct {
	State     *PartnerState
	Finance   *FinanceData
	Members   map[int64]PartnerMember
	Usage     []PartnerUsage
	StartDays map[int64]PartnerStartDay
}
type PartnerRepository interface {
	Read(context.Context) (*PartnerState, error)
	LoadMonths(context.Context, []int64, string, string) (*PartnerMonthsData, error)
	Create(context.Context, int64, string, decimal.Decimal, int64) error
	Attach(context.Context, int64, int64, int64) error
	Update(context.Context, int64, *string, *decimal.Decimal, int64, string) error
	WithPartnerLock(context.Context, int64, func(context.Context) error) error
	SaveStatement(context.Context, int64, int64, PartnerStatementDetail) error
	PayStatement(context.Context, int64, string, int64, string, string) error
	AddAdjustment(context.Context, int64, string, string, string, int64) (*PartnerAdjustment, error)
	DeleteAdjustment(context.Context, int64, int64) error
}
type PartnerService struct {
	repo      PartnerRepository
	affiliate *AffiliateService
	team      *AffiliateTeamService
}

func NewPartnerService(repo PartnerRepository, affiliate *AffiliateService, team *AffiliateTeamService) *PartnerService {
	return &PartnerService{repo: repo, affiliate: affiliate, team: team}
}
