package service

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	SeedanceEndpointCreate           GrokMediaEndpoint        = "seedance_create"
	SeedanceEndpointStatus           GrokMediaEndpoint        = "seedance_status"
	SeedanceEndpointDelete           GrokMediaEndpoint        = "seedance_delete"
	OpenAIEndpointCapabilitySeedance OpenAIEndpointCapability = "seedance"
)

func (e GrokMediaEndpoint) IsSeedance() bool {
	return e == SeedanceEndpointCreate || e == SeedanceEndpointStatus || e == SeedanceEndpointDelete
}

// SeedanceTaskKey isolates ownership and billing keys from other video providers.
func SeedanceTaskKey(id string) string { return "seedance:" + strings.TrimSpace(id) }

func ParseSeedanceRequest(body []byte) (GrokMediaRequestInfo, error) {
	var info GrokMediaRequestInfo
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("request body must be a JSON object")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.String()) == "" {
		return info, fmt.Errorf("model is required")
	}
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() || len(content.Array()) == 0 {
		return info, fmt.Errorf("content must be a non-empty array")
	}
	// Billing happens when the task is polled through the gateway; an upstream
	// callback would hand the video to the client without that poll.
	if callback := gjson.GetBytes(body, "callback_url"); callback.Exists() && callback.Type != gjson.Null {
		return info, fmt.Errorf("callback_url is not supported; poll GET /contents/generations/tasks/{id} for the result")
	}
	info.Model = strings.TrimSpace(model.String())
	info.Resolution = gjson.GetBytes(body, "resolution").String()
	if info.Resolution == "" {
		info.Resolution = VideoBillingResolution720P
	}
	info.Ratio = gjson.GetBytes(body, "ratio").String()
	info.DurationSeconds = int(gjson.GetBytes(body, "duration").Int())
	if info.DurationSeconds <= 0 {
		info.DurationSeconds = 15
		model := strings.ToLower(info.Model)
		if strings.Contains(model, "2-5") || strings.Contains(model, "2.5") {
			info.DurationSeconds = 30
		}
	}
	var texts []string
	for _, item := range content.Array() {
		switch item.Get("type").String() {
		case "text":
			texts = append(texts, item.Get("text").String())
		case "video_url":
			info.HasReferenceVideo = true
		case "image_url":
			info.InputImageURLs = append(info.InputImageURLs, item.Get("image_url.url").String())
		}
	}
	info.Prompt = strings.Join(texts, "\n")
	return info, nil
}

func buildSeedanceURL(base string, endpoint GrokMediaEndpoint, taskID string) (string, error) {
	base = strings.TrimRight(base, "/")
	// Accept an origin, a proxy prefix, or the full Ark API base.
	if !strings.HasSuffix(base, "/api/v3") && !strings.HasSuffix(base, "/v3") {
		base += "/api/v3"
	}
	base += "/contents/generations/tasks"
	if endpoint != SeedanceEndpointCreate {
		if err := validateUpstreamPathSegment("Seedance task ID", taskID); err != nil || strings.TrimSpace(taskID) == "" {
			return "", fmt.Errorf("invalid Seedance task ID")
		}
		base += "/" + taskID
	}
	return base, nil
}

// ForwardSeedance preserves the Ark protocol, including multimodal content and
// future fields. Only model is rewritten using the account's configured mapping.
func (s *OpenAIGatewayService) ForwardSeedance(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error) {
	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance) || !endpoint.IsSeedance() {
		return nil, fmt.Errorf("seedance requires an OpenAI API key account with a custom base URL")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	target, err := buildSeedanceURL(base, endpoint, strings.TrimPrefix(taskID, "seedance:"))
	if err != nil {
		return nil, err
	}
	model, upstreamModel := "", ""
	var requestInfo GrokMediaRequestInfo
	method := http.MethodGet
	switch endpoint {
	case SeedanceEndpointCreate:
		info, parseErr := ParseSeedanceRequest(body)
		if parseErr != nil {
			return nil, parseErr
		}
		requestInfo = info
		model = info.Model
		upstreamModel = account.GetMappedModel(model)
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
		method = http.MethodPost
	case SeedanceEndpointDelete:
		method = http.MethodDelete
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("seedance account missing api_key")
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	// Do not retry ambiguous asynchronous creates: the upstream may already have
	// accepted a billable job. Preserve native error codes and response bodies.
	if resp.StatusCode >= 300 {
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("seedance upstream status %d", resp.StatusCode)
	}
	result := &OpenAIForwardResult{Model: model, BillingModel: model, UpstreamModel: upstreamModel, Duration: time.Since(started), ResponseHeaders: resp.Header.Clone()}
	if endpoint == SeedanceEndpointCreate {
		id := strings.TrimSpace(gjson.GetBytes(responseBody, "id").String())
		if id == "" {
			return nil, fmt.Errorf("seedance create response missing task ID")
		}
		result.ResponseID = SeedanceTaskKey(id)
		result.VideoResolution = requestInfo.Resolution
		result.VideoDurationSeconds = requestInfo.DurationSeconds
	}
	if endpoint == SeedanceEndpointStatus {
		result.ResponseID = taskID
		result.UpstreamModel = gjson.GetBytes(responseBody, "model").String()
		// duration is the actual output length, including auto-duration requests.
		// Reference-video billing is derived from tokens after loading pending.
		if gjson.GetBytes(responseBody, "status").String() == "succeeded" {
			result.Usage.OutputTokens = max(0, int(gjson.GetBytes(responseBody, "usage.completion_tokens").Int()))
			result.VideoCount = 1
			result.VideoResolution = gjson.GetBytes(responseBody, "resolution").String()
			result.VideoRatio = gjson.GetBytes(responseBody, "ratio").String()
			result.VideoDurationSeconds = int(gjson.GetBytes(responseBody, "duration").Int())
		}
	}
	writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
	return result, nil
}

// Standard Ark dimensions; 4k approximates twice the 1080p dimensions.
// mmgate /v1/models has no token_frames; verify against real tasks after launch.
var seedanceVideoPixels = map[string]map[string]int{
	"480p":  {"16:9": 864 * 496, "4:3": 752 * 560, "1:1": 640 * 640, "3:4": 560 * 752, "9:16": 496 * 864, "21:9": 992 * 432},
	"720p":  {"16:9": 1280 * 720, "4:3": 1112 * 834, "1:1": 960 * 960, "3:4": 834 * 1112, "9:16": 720 * 1280, "21:9": 1470 * 630},
	"1080p": {"16:9": 1920 * 1080, "4:3": 1664 * 1248, "1:1": 1440 * 1440, "3:4": 1248 * 1664, "9:16": 1080 * 1920, "21:9": 2206 * 946},
	"4k":    {"16:9": 3840 * 2160, "4:3": 3328 * 2496, "1:1": 2880 * 2880, "3:4": 2496 * 3328, "9:16": 2160 * 3840, "21:9": 4412 * 1892},
}

// SeedanceBillingDurationSeconds includes reference-video usage in billed seconds.
func SeedanceBillingDurationSeconds(duration, completionTokens int, resolution, ratio string, hasReferenceVideo bool) int {
	billed := duration
	if hasReferenceVideo {
		physicalResolution, _ := LookupVideoBillingResolution(resolution)
		if pixels := seedanceVideoPixels[strings.TrimSuffix(physicalResolution, "-ref")][ratio]; pixels > 0 {
			frames := float64(completionTokens) * 1024 / float64(pixels)
			billed = max(duration, int(math.Round((frames-1)/24)))
		} else {
			billed = duration + int(math.Ceil(float64(duration)*2/3))
		}
	}
	return min(billed, VideoBillingMaxDurationSeconds)
}
