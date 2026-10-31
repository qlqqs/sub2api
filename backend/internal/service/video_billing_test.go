package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanonicalGrokImagineVideoPriceFamily(t *testing.T) {
	t.Parallel()
	require.Equal(t, VideoPriceFamilyGrokImagineVideo, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-1.5"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-1.5-preview"))
	require.Equal(t, VideoPriceFamilyGrokImagineVideo15, CanonicalGrokImagineVideoPriceFamily("xai/grok-video-1.5"))
	require.Equal(t, "grok-imagine-video-2", CanonicalGrokImagineVideoPriceFamily("grok-imagine-video-2"))
	require.Equal(t, "grok-imagine-video-2", CanonicalGrokImagineVideoPriceFamily("xai/grok-imagine-video-2"))
}

func TestNormalizeAndLookupVideoModelPrices(t *testing.T) {
	t.Parallel()
	raw := map[string]map[string]float64{
		"grok-imagine-video-1.5-preview": {"480p": 0.08, "720p": 0.14},
		"grok-imagine-video":             {"480p": 0.05},
		"grok-imagine-video-2":           {"1080p": 0.4},
	}
	norm := NormalizeVideoModelPrices(raw)
	require.NotNil(t, norm)
	require.Contains(t, norm, VideoPriceFamilyGrokImagineVideo15)
	require.Contains(t, norm, VideoPriceFamilyGrokImagineVideo)
	require.Contains(t, norm, "grok-imagine-video-2")

	p15 := LookupVideoModelPrice(norm, "grok-imagine-video-1.5", "480p")
	require.NotNil(t, p15)
	require.InDelta(t, 0.08, *p15, 1e-9)

	pBase := LookupVideoModelPrice(norm, "grok-imagine-video", "480p")
	require.NotNil(t, pBase)
	require.InDelta(t, 0.05, *pBase, 1e-9)
	// A missing model-specific tier must fall back to the flat tier price,
	// rather than borrowing another model-specific resolution.
	require.Nil(t, LookupVideoModelPrice(norm, "grok-imagine-video", "720p"))

	p2 := LookupVideoModelPrice(norm, "grok-imagine-video-2", "1080p")
	require.NotNil(t, p2)
	require.InDelta(t, 0.4, *p2, 1e-9)

	// Unmatched model → nil (caller falls back to flat columns / defaults).
	require.Nil(t, LookupVideoModelPrice(norm, "unknown-model", "480p"))
}

func TestNormalizeVideoModelPricesDropsUnknownResolutions(t *testing.T) {
	t.Parallel()
	// Unknown tiers must not overwrite a recognized resolution price.
	norm := NormalizeVideoModelPrices(map[string]map[string]float64{
		"grok-imagine-video": {"480p": 0.05, "4k": 0.50, "1080i": 0.30},
	})
	require.NotNil(t, norm)
	require.Equal(t, map[string]float64{VideoBillingResolution480P: 0.05, VideoBillingResolution4K: 0.50}, norm[VideoPriceFamilyGrokImagineVideo])

	// A model whose tiers are all unrecognized contributes no family at all.
	require.Nil(t, NormalizeVideoModelPrices(map[string]map[string]float64{
		"grok-imagine-video": {"1080i": 0.50},
	}))
}

func TestNormalizeVideoModelPricesIsDeterministicAcrossAliasConflicts(t *testing.T) {
	t.Parallel()
	// Both keys canonicalize onto grok-imagine-video-1.5 and disagree on 480p.
	// Whichever price wins, it must be the same one on every run — a Go map walk
	// would let two processes bill the same request differently.
	raw := map[string]map[string]float64{
		"grok-imagine-video-1.5":         {"480p": 0.08},
		"grok-imagine-video-1.5-preview": {"480p": 0.11},
		"grok-video-1.5":                 {"480p": 0.09},
	}
	first := NormalizeVideoModelPrices(raw)
	require.NotNil(t, first)
	for i := 0; i < 50; i++ {
		require.Equal(t, first, NormalizeVideoModelPrices(raw), "run %d diverged", i)
	}

	// Aliases within one model key normalize to the same tier deterministically too.
	aliased := map[string]map[string]float64{
		"grok-imagine-video": {"720": 0.12, "720p": 0.13, "hd": 0.14},
	}
	firstAliased := NormalizeVideoModelPrices(aliased)
	require.NotNil(t, firstAliased)
	for i := 0; i < 50; i++ {
		require.Equal(t, firstAliased, NormalizeVideoModelPrices(aliased), "run %d diverged", i)
	}
}

func TestLookupVideoBillingResolutionReportsUnknownTiers(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"480", "480p", "SD", "720", "hd", "1080", "full-hd", " fhd ", "4k", "4K", "2160p", "2160", "uhd", "720p-ref", "4K-REF"} {
		normalized, ok := LookupVideoBillingResolution(in)
		require.True(t, ok, "input=%q", in)
		require.NotEmpty(t, normalized)
	}
	for _, in := range []string{"", "1080i", "potato", "8k-ref"} {
		normalized, ok := LookupVideoBillingResolution(in)
		require.False(t, ok, "input=%q", in)
		require.Empty(t, normalized)
	}
	// Runtime billing still needs a tier for unrecognized upstream values.
	require.Equal(t, VideoBillingResolution4K, NormalizeVideoBillingResolutionOrDefault("4k"))
	require.Equal(t, VideoBillingResolution1080P, NormalizeVideoBillingResolutionOrDefault("full_hd"))
}

func TestVideoModelPriceMissingTierFallsBackToFlatTierPrice(t *testing.T) {
	t.Parallel()
	flat720P := 0.7
	service := &BillingService{}

	result := service.CalculateVideoCost("grok-imagine-video", "720p", 1, 1, &VideoPriceConfig{
		Price720P: &flat720P,
		ModelPrices: map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo: {VideoBillingResolution480P: 0.05},
		},
	}, 1)

	require.InDelta(t, flat720P, result.TotalCost, 1e-9)
}

func TestVideoBillingReferenceTierNormalizationAndModelPrices(t *testing.T) {
	for input, want := range map[string]string{
		"4K": "4k", "2160p": "4k", "2160": "4k", "UHD": "4k",
		" 2160P-REF ": "4k-ref", "HD-ref": "720p-ref", "1080p-REF": "1080p-ref", "sd-ref": "480p-ref",
	} {
		require.Equal(t, want, NormalizeVideoBillingResolutionOrDefault(input))
	}
	prices := NormalizeVideoModelPrices(map[string]map[string]float64{"seedance-2.0": {
		"480p": 0.5318, "720p": 0.75, "720p-ref": 0.9075, "4K": 3.75, "2160p-ref": 4.665,
	}})
	for tier, want := range map[string]float64{"480p-ref": 0.5318, "720p-ref": 0.9075, "4k": 3.75, "4k-ref": 4.665} {
		price := LookupVideoModelPrice(prices, "seedance-2.0", tier)
		require.NotNil(t, price)
		require.Equal(t, want, *price)
	}
	flat := 0.75
	group := &Group{VideoPrice480P: &flat, VideoPrice720P: &flat}
	require.Equal(t, &flat, group.GetVideoPrice("720p-ref"))
	require.Nil(t, group.GetVideoPrice("4k"), "4k must not borrow the flat 480p override")
	bill := (&BillingService{}).CalculateVideoCost("seedance-2.0", "720p-ref", 1, 9, &VideoPriceConfig{Price720P: &flat}, 1)
	require.InDelta(t, 6.75, bill.TotalCost, 1e-9)
}
