//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSeedanceHandlerLifecycleAndOwnership(t *testing.T) {
	h, slots, bindings, upstream := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	var owner int64
	upstream.call = func(req *http.Request, id int64) (*http.Response, error) {
		body := `{"id":"task-ark","status":"queued"}`
		if req.Method == http.MethodPost {
			owner = id
		} else {
			require.Equal(t, owner, id)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	newContext := func(method string) (*gin.Context, *httptest.ResponseRecorder) {
		c, w := grokMediaSlotContext(context.Background(), method == http.MethodPost)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.Group.Platform = service.PlatformOpenAI
		body := ""
		if method == http.MethodPost {
			body = `{"model":"doubao-seedance","content":[{"type":"text","text":"waves"}]}`
		}
		c.Request = httptest.NewRequest(method, "/api/v3/contents/generations/tasks", strings.NewReader(body))
		c.Params = gin.Params{{Key: "task_id", Value: "task-ark"}}
		return c, w
	}
	c, w := newContext(http.MethodPost)
	h.SeedanceTasks(c)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Positive(t, owner)
	require.Len(t, bindings.pending, 1)
	slots.assertReleased(t)
	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		c, w = newContext(method)
		h.SeedanceTasks(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		slots.assertReleased(t)
	}
	for _, other := range []string{"user", "key", "group", "task", "provider"} {
		c, w = newContext(http.MethodGet)
		key, _ := middleware.GetAPIKeyFromContext(c)
		switch other {
		case "user":
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 11, Concurrency: 5})
		case "key":
			key.ID = 21
		case "group":
			group := int64(25)
			key.GroupID = &group
		case "task":
			c.Params = gin.Params{{Key: "task_id", Value: "other"}}
		case "provider":
			c.Params = gin.Params{{Key: "request_id", Value: "task-ark"}}
		}
		before := upstream.calls
		if other == "provider" {
			h.GrokVideoStatus(c)
		} else {
			h.SeedanceTasks(c)
		}
		require.Equal(t, 404, w.Code, other+": "+w.Body.String())
		require.Equal(t, before, upstream.calls)
		slots.assertReleased(t)
	}
	c, _ = newContext(http.MethodGet)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	result := &service.OpenAIForwardResult{Usage: service.OpenAIUsage{OutputTokens: 12345}, ResponseID: "seedance:task-ark", VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 5}
	for i := range 20 {
		billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, result.ResponseID, result)
		if i == 0 {
			require.NotNil(t, billed)
			require.Equal(t, "doubao-seedance", billed.BillingModel)
			require.Equal(t, 1, billed.VideoCount)
			require.Equal(t, 5, billed.VideoDurationSeconds)
		} else {
			require.Nil(t, billed)
		}
	}
	require.Len(t, bindings.billed, 1)
}

func TestSeedanceInflightEstimate(t *testing.T) {
	for _, tc := range []struct {
		duration  int
		reference bool
		want      int
	}{
		{5, false, 5}, {5, true, 9}, {15, true, 25}, {30, true, 50}, {60, true, 60},
	} {
		info := service.GrokMediaRequestInfo{Resolution: "720p", DurationSeconds: tc.duration, HasReferenceVideo: tc.reference}
		estimate := grokMediaInflightEstimate(service.SeedanceEndpointCreate, "seedance-2.5", info, []byte(`{}`))
		require.True(t, estimate.RequireFullBalance)
		require.Equal(t, tc.want, estimate.VideoDurationSeconds)
		require.Equal(t, service.VideoBillingTier("720p", tc.reference), estimate.VideoResolution)
		require.Equal(t, service.InflightEstimateVideo, estimate.Kind)
		require.Equal(t, service.InflightEstimateRequest{}, grokMediaInflightEstimate(service.SeedanceEndpointStatus, "seedance-2.5", info, nil))
	}
}

func TestSeedanceCompletionUsesPendingReferenceVideo(t *testing.T) {
	h, _, _, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	c, _ := grokMediaSlotContext(context.Background(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	pending := service.GrokVideoPendingBilling{Model: "seedance-2.5", VideoResolution: "720p", VideoDurationSeconds: 30, HasReferenceVideo: true}
	require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(context.Background(), "seedance:reference", subject.UserID, key.ID, pending))
	result := &service.OpenAIForwardResult{VideoCount: 1, VideoResolution: "720p", VideoRatio: "16:9", VideoDurationSeconds: 5, Usage: service.OpenAIUsage{OutputTokens: 1280 * 720 * (17*24 + 1) / 1024}}
	billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, "seedance:reference", result)
	require.NotNil(t, billed)
	require.Equal(t, 17, billed.VideoDurationSeconds)
	require.Equal(t, "720p-ref", billed.VideoResolution)
	require.Equal(t, result.Usage.OutputTokens, billed.Usage.OutputTokens)
}

func TestSeedance4KReferenceCompletionKeepsPhysicalResolutionForTokens(t *testing.T) {
	h, _, _, _ := newGrokMediaSlotHandler(t, false, false, service.PlatformOpenAI)
	c, _ := grokMediaSlotContext(context.Background(), true)
	key, _ := middleware.GetAPIKeyFromContext(c)
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	pending := service.GrokVideoPendingBilling{Model: "seedance-2.0", VideoResolution: "4K", HasReferenceVideo: true}
	require.NoError(t, h.gatewayService.StoreGrokVideoPendingBilling(context.Background(), "seedance:4k", subject.UserID, key.ID, pending))
	result := &service.OpenAIForwardResult{VideoCount: 1, VideoResolution: "2160p", VideoRatio: "9:16", VideoDurationSeconds: 5,
		Usage: service.OpenAIUsage{OutputTokens: 2160 * 3840 * (17*24 + 1) / 1024}}
	billed := prepareSeedanceCompletionBilling(context.Background(), h, key, subject, "seedance:4k", result)
	require.NotNil(t, billed)
	require.Equal(t, 17, billed.VideoDurationSeconds)
	require.Equal(t, "4k-ref", billed.VideoResolution)
	require.Equal(t, "2160p", result.VideoResolution)
}

func TestSeedanceAutomaticDurationAndReferenceInflightTier(t *testing.T) {
	for _, duration := range []string{"", `,"duration":-1`} {
		for _, content := range []string{
			`{"type":"text","text":"waves"}`,
			`{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}`,
			`{"type":"audio_url","audio_url":{"url":"https://example.com/a.mp3"}}`,
			`{"type":"video_url","video_url":{"url":"https://example.com/a.mp4"}}`,
		} {
			info, err := service.ParseSeedanceRequest([]byte(`{"model":"seedance-2.0","resolution":"4K","content":[` + content + `]` + duration + `}`))
			require.NoError(t, err)
			require.Equal(t, 15, info.DurationSeconds)
			estimate := grokMediaInflightEstimate(service.SeedanceEndpointCreate, info.Model, info, nil)
			if info.HasReferenceVideo {
				require.Equal(t, "4k-ref", estimate.VideoResolution)
				require.Equal(t, 25, estimate.VideoDurationSeconds)
			} else {
				require.Equal(t, "4k", estimate.VideoResolution)
				require.Equal(t, 15, estimate.VideoDurationSeconds)
			}
		}
	}
}
