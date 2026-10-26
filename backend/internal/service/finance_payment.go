package service

// CUSTOM(fork): 财务字段只供报表，不参与支付网关金额和余额变更。
import (
	"regexp"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var financeChannelFeePattern = regexp.MustCompile(`^[0-9](\.[0-9]{1,4})?$`)

func ValidateChannelFeeRate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "0"
	}
	if !financeChannelFeePattern.MatchString(raw) {
		return "", infraerrors.BadRequest("CHANNEL_FEE_RATE_INVALID", "通道费率须为普通小数，最多 4 位小数")
	}
	d, err := decimal.NewFromString(raw)
	if err != nil || d.IsNegative() || d.GreaterThan(decimal.RequireFromString("0.2")) || !d.Equal(d.Round(4)) {
		return "", infraerrors.BadRequest("CHANNEL_FEE_RATE_INVALID", "通道费率须为 0–0.2 之间、最多 4 位小数")
	}
	return d.StringFixed(4), nil
}

func financeRefundDeducted(amount float64) string { return decimal.NewFromFloat(amount).StringFixed(8) }
