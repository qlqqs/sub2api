package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// AccountExtraImagesURLToB64JSON 是账户 extra 中的开关键。开启后，Images 端点响应（含流式的完成事件）里
// b64_json 缺失或为空但 url 非空的图片项，由网关下载 url 内容并以 base64 回填到 b64_json。
const AccountExtraImagesURLToB64JSON = "images_url_to_b64_json"

// openAIImageURLDownloadTimeout 是单张图片 url 下载的超时上限。
const openAIImageURLDownloadTimeout = 60 * time.Second

// ImagesURLToB64JSONEnabled 返回账户是否开启了 url 转 b64_json 回填。
func ImagesURLToB64JSONEnabled(account *Account) bool {
	return account != nil && account.getExtraBool(AccountExtraImagesURLToB64JSON)
}

// backfillOpenAIImagesB64JSON 对 Images 端点的非流式响应做 url 转 b64_json 回填。
//
// 只处理 data[i].b64_json 缺失或为空且 url 非空的项；url 字段原样保留。
// 单项失败仅记日志并保留该项原样，响应整体照常返回。
// 客户端显式要求 response_format=url 时不做回填。
func (s *OpenAIGatewayService) backfillOpenAIImagesB64JSON(
	ctx context.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	body []byte,
) []byte {
	if !ImagesURLToB64JSONEnabled(account) {
		return body
	}
	if parsed != nil && parsed.ResponseFormat == "url" {
		return body
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}
	items := gjson.GetBytes(body, "data")
	if !items.IsArray() {
		return body
	}
	for index, item := range items.Array() {
		if !item.IsObject() {
			continue
		}
		if strings.TrimSpace(item.Get("b64_json").String()) != "" {
			continue
		}
		rawURL := strings.TrimSpace(item.Get("url").String())
		if rawURL == "" {
			continue
		}
		encoded, err := s.fetchOpenAIImageURLBase64(ctx, account, rawURL)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		updated, err := sjson.SetBytes(body, fmt.Sprintf("data.%d.b64_json", index), encoded)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		body = updated
	}
	return body
}

// openAIImagesStreamBackfill 是流式响应里 url 转 b64_json 回填所需的上下文；nil 表示不回填。
type openAIImagesStreamBackfill struct {
	ctx     context.Context
	account *Account
}

// newOpenAIImagesStreamBackfill 在账户开启回填且客户端未显式要求 url 时返回回填上下文。
func newOpenAIImagesStreamBackfill(ctx context.Context, account *Account, parsed *OpenAIImagesRequest) *openAIImagesStreamBackfill {
	if !ImagesURLToB64JSONEnabled(account) {
		return nil
	}
	if parsed != nil && parsed.ResponseFormat == "url" {
		return nil
	}
	return &openAIImagesStreamBackfill{ctx: ctx, account: account}
}

// backfillOpenAIImagesStreamEventB64JSON 对流式响应的单个事件做 url 转 b64_json 回填。
//
// 只处理完成事件（type 以 .completed 结尾或缺省 type）：顶层 b64_json 缺失或为空且 url 非空时回填顶层，
// data[i] 形式的图片项与非流式规则一致。返回值第二项表示事件是否被改写。
func (s *OpenAIGatewayService) backfillOpenAIImagesStreamEventB64JSON(backfill *openAIImagesStreamBackfill, payload []byte) ([]byte, bool) {
	if backfill == nil || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload, false
	}
	eventType := strings.TrimSpace(gjson.GetBytes(payload, "type").String())
	if eventType != "" && !strings.HasSuffix(eventType, ".completed") {
		return payload, false
	}
	body := payload
	if strings.TrimSpace(gjson.GetBytes(body, "b64_json").String()) == "" {
		if rawURL := strings.TrimSpace(gjson.GetBytes(body, "url").String()); rawURL != "" {
			encoded, err := s.fetchOpenAIImageURLBase64(backfill.ctx, backfill.account, rawURL)
			if err == nil {
				if updated, setErr := sjson.SetBytes(body, "b64_json", encoded); setErr == nil {
					body = updated
				} else {
					err = setErr
				}
			}
			if err != nil {
				logger.LegacyPrintf(
					"service.openai_gateway",
					"[OpenAI] Images stream b64_json backfill skipped account_id=%d err=%s",
					backfill.account.ID,
					sanitizeUpstreamErrorMessage(err.Error()),
				)
			}
		}
	}
	body = s.backfillOpenAIImagesB64JSON(backfill.ctx, backfill.account, nil, body)
	return body, !bytes.Equal(body, payload)
}

// fetchOpenAIImageURLBase64 取得图片 url 内容的标准 base64 编码。
// data: 形式的 url 直接取其 base64 载荷；其余 url 先沿用 base_url 的出站 URL 策略校验，
// 再无条件拒绝回环、私网、链路本地等目的地（含重定向的每一跳），经账户代理下载，
// 大小上限与 OAuth 路径的单图下载一致，且前 512 字节须嗅探为 png/jpeg/webp/gif。
func (s *OpenAIGatewayService) fetchOpenAIImageURLBase64(ctx context.Context, account *Account, rawURL string) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		if encoded := normalizeOpenAIImageBase64(rawURL); encoded != "" {
			return encoded, nil
		}
		return "", errors.New("data url payload is not valid base64")
	}
	if s == nil || s.httpUpstream == nil {
		return "", errors.New("http upstream is not configured")
	}
	downloadURL, err := s.validateOutboundURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid image url: %w", err)
	}
	if err := rejectPrivateImageHost(downloadURL); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(WithHTTPUpstreamPublicHostsOnly(ctx), openAIImageURLDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("build image download request: %w", err)
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download image: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, openAIImageMaxDownloadBytes+1))
	if err != nil {
		return "", fmt.Errorf("read image body: %w", err)
	}
	if int64(len(data)) > openAIImageMaxDownloadBytes {
		return "", fmt.Errorf("downloaded image exceeds %d bytes", openAIImageMaxDownloadBytes)
	}
	if len(data) == 0 {
		return "", errors.New("download image: empty body")
	}
	if !isBackfillImageContent(data) {
		return "", errors.New("download image: content is not an allowed image format")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// rejectPrivateImageHost 拒绝主机为 localhost 或回环、私网、链路本地、未指定地址字面量的下载 URL。
// 不受 security.url_allowlist 配置影响；域名的解析结果与重定向的每一跳由上游客户端按
// WithHTTPUpstreamPublicHostsOnly 标记校验。
func rejectPrivateImageHost(downloadURL string) error {
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("invalid image url: %w", err)
	}
	if host := parsed.Hostname(); urlvalidator.IsBlockedHost(host) {
		return fmt.Errorf("image url host is not allowed: %s", host)
	}
	return nil
}

// openAIImageBackfillContentTypes 是允许回填的图片格式，以字节嗅探结果为准，响应头不作为依据。
var openAIImageBackfillContentTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/gif":  {},
}

// isBackfillImageContent 报告 data 的前 512 字节是否嗅探为允许回填的图片格式。
func isBackfillImageContent(data []byte) bool {
	_, ok := openAIImageBackfillContentTypes[detectedImageContentType(data)]
	return ok
}
