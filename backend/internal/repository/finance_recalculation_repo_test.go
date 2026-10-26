//go:build unit

package repository

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFinanceRecalculationEvidenceImageCacheSplit(t *testing.T) {
	for _, tt := range []struct {
		name, extra                        string
		rawImage, cacheRead, image, cached int
		known                              bool
	}{
		{"known", `"image_cache_read_tokens":40,"image_cache_split_present":true,`, 60, 50, 20, 40, true},
		{"explicit-zero", `"image_cache_read_tokens":0,"image_cache_split_present":true,`, 60, 50, 60, 0, true},
		{"missing", `"image_cache_read_tokens":null,"image_cache_split_present":false,`, 60, 50, 60, 0, false},
		{"invalid", `"image_cache_read_tokens":"secret-not-number","image_cache_split_present":true,`, 60, 50, 60, 0, false},
		{"oversized", `"image_cache_read_tokens":61,"image_cache_split_present":true,`, 60, 50, 60, 0, false},
		{"no-image-input", `"image_cache_read_tokens":null,"image_cache_split_present":false,`, 0, 50, 0, 0, true},
		{"no-cache-read", `"image_cache_read_tokens":null,"image_cache_split_present":false,`, 60, 0, 60, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{%s"tokens":{"ImageInputTokens":%d,"CacheReadTokens":%d}}`, tt.extra, tt.rawImage, tt.cacheRead))
			c, err := financeCandidate(service.FinanceRecalculationUsageCost, 1, time.Now(), raw, "")
			require.NoError(t, err)
			require.Equal(t, tt.image, c.Usage.Tokens.ImageInputTokens)
			require.Equal(t, tt.cached, c.Usage.Tokens.ImageCacheReadTokens)
			require.Equal(t, tt.known, c.Usage.ImageCacheSplitKnown)
			require.Nil(t, c.Usage.PricingAt)
			require.Nil(t, c.Usage.CacheStorageCost)
			require.JSONEq(t, string(raw), string(c.Raw))
		})
	}
}
func TestFinanceRecalculationAuditDTOHidesRawPayload(t *testing.T) {
	item := service.FinanceRecalculationItem{Candidate: service.FinanceRecalculationCandidate{Raw: json.RawMessage(`{"internal":"not-public"}`)}, UsageResult: &service.FinanceUsageCostResult{InputFingerprint: "private"}}
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "internal")
	require.NotContains(t, string(raw), "private")
}
func TestFinanceRecalculationSnapshotValidation(t *testing.T) {
	for _, value := range []string{"1e1000000", "NaN", "1.00000000001", "10000000000"} {
		_, err := financeDecimal(value, 10, "10000000000")
		require.Error(t, err)
	}
	d, err := financeDecimal("0.0000000001", 10, "10000000000")
	require.NoError(t, err)
	require.Equal(t, "0.0000000001", d.String())
	require.ErrorIs(t, financeNormalizeItem(service.FinanceRecalculationUsageCost, &service.FinanceRecalculationItem{Status: service.FinanceRecalculationChanged, Reason: "driver: credentials"}), service.ErrFinanceRecalculationInvalid)
}
