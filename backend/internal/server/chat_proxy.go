package server

// CUSTOM: Open WebUI embedding. Contract: tasks/10-24-chat-openwebui/contract.md.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	chatPrefix      = "/_chat"
	chatCookie      = "s2a_chat"
	chatAPI         = chatPrefix + "/__sub2api/"
	chatCSP         = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' blob: data:; worker-src 'self' blob:; frame-src 'self' blob: data:; frame-ancestors 'self'; base-uri 'self'; form-action 'self'"
	chatArtifactCSP = "default-src 'none'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob: data:; style-src 'self' 'unsafe-inline' blob: data:; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' data: blob:; connect-src 'none'; frame-src 'none'; worker-src 'none'; form-action 'none'; base-uri 'none'; frame-ancestors 'self'"
)

type chatBackend interface {
	Authenticate(context.Context, string) (*service.User, time.Time, error)
	GetKey(context.Context, int64) (*service.APIKey, error)
	ListKeys(context.Context, int64) ([]service.APIKey, error)
	Groups(context.Context, int64) ([]service.Group, error)
	CreateKey(context.Context, int64, string, int64) (*service.APIKey, error)
	ModelPrices(context.Context, int64, int64) ([]service.ChatModelPrice, error)
	ModelReasoning(context.Context, int64, int64, []string) (map[string]*service.ChatModelReasoning, error)
}
type chatProxyHeadersKey struct{}

func installChatProxy(r *gin.Engine, cfg *config.Config, b chatBackend) {
	proxy := newChatReverseProxy(cfg.Chat.WebUIURL)
	r.Use(func(c *gin.Context) {
		if c.Request.URL.Path != chatPrefix && !strings.HasPrefix(c.Request.URL.Path, chatPrefix+"/") {
			c.Next()
			return
		}
		defer c.Abort()
		setChatSecurityHeaders(c.Writer.Header(), c.Request.URL.Path)
		if proxy == nil {
			chatError(c, 503, "CHAT_UNAVAILABLE", "对话工作台未启用或暂不可用")
			return
		}
		req := c.Request
		// Reject ambiguous paths rather than authenticating one path and forwarding another.
		if path.Clean(req.URL.Path) != strings.TrimSuffix(req.URL.Path, "/") && req.URL.Path != chatPrefix+"/" {
			chatError(c, 400, "INVALID_PATH", "请求路径无效")
			return
		}
		static := chatStaticRequest(req)
		var user *service.User
		if !static || strings.HasPrefix(req.URL.Path, chatAPI) {
			cookie, err := req.Cookie(chatCookie)
			if err != nil {
				chatError(c, 401, "CHAT_SESSION_INVALID", "登录已失效")
				return
			}
			user, _, err = b.Authenticate(req.Context(), cookie.Value)
			if err != nil || user == nil {
				chatError(c, 401, "CHAT_SESSION_INVALID", "登录已失效")
				return
			}
			c.Header("Cache-Control", "no-store")
		}
		if !limitChatBody(c) {
			return
		}
		if strings.HasPrefix(req.URL.Path, chatAPI) {
			serveChatAPI(c, b, user)
			return
		}
		headers := make(http.Header)
		clientIP := ip.GetSecurityClientIP(c, cfg.TrustForwardedIPForAPIKeyACL())
		headers.Set("X-Forwarded-For", clientIP)
		headers.Set("X-Real-IP", clientIP)
		if user != nil {
			headers.Set("X-Sub2API-User-Id", strconv.FormatInt(user.ID, 10))
			headers.Set("X-Sub2API-Email", "u"+strconv.FormatInt(user.ID, 10)+"@sub2api.internal")
			name := user.Username
			if name == "" {
				name = strings.SplitN(user.Email, "@", 2)[0]
			}
			// QueryEscape encodes UTF-8 bytes outside RFC 3986 unreserved; use %20, not +, for spaces.
			headers.Set("X-Sub2API-Name", strings.ReplaceAll(url.QueryEscape(name), "+", "%20"))
			role := "user"
			if user.Role == "admin" {
				role = "admin"
			}
			headers.Set("X-Sub2API-User-Role", role)
			if !strings.EqualFold(req.Header.Get("Upgrade"), "websocket") {
				if key := chatSelectedKey(req.Context(), b, user.ID, req.Header.Get("X-Sub2API-Key-Id")); key != nil {
					headers.Set("X-Sub2API-Key", key.Key)
					headers.Set("X-Sub2API-Key-Platform", key.Group.Platform)
				}
				if key := chatSelectedKey(req.Context(), b, user.ID, req.Header.Get("X-Sub2API-Image-Key-Id")); key != nil && chatSupportsImages(key.Group) {
					headers.Set("X-Sub2API-Image-Key", key.Key)
				}
			}
		}
		// Do not forward the console JWT to Open WebUI; retain its own cookies/token.
		cookies := req.Cookies()
		req.Header.Del("Cookie")
		for _, cookie := range cookies {
			if cookie.Name != chatCookie {
				req.AddCookie(cookie)
			}
		}
		ctx := context.WithValue(req.Context(), chatProxyHeadersKey{}, headers)
		proxy.ServeHTTP(c.Writer, req.WithContext(ctx))
	})
	r.GET("/api/v1/chat/status", func(c *gin.Context) {
		response.Success(c, gin.H{"enabled": strings.TrimSpace(cfg.Chat.WebUIURL) != ""})
	})
	r.POST("/api/v1/chat/session", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if proxy == nil {
			response.Error(c, 503, "对话工作台未启用或暂不可用")
			return
		}
		parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Unauthorized(c, "请重新登录")
			return
		}
		token := strings.TrimSpace(parts[1])
		_, expires, err := b.Authenticate(c.Request.Context(), token)
		if err != nil {
			response.Unauthorized(c, "登录已失效，请重新登录")
			return
		}
		http.SetCookie(c.Writer, &http.Cookie{Name: chatCookie, Value: token, Path: chatPrefix, HttpOnly: true, Secure: chatRequestHTTPS(c), SameSite: http.SameSiteStrictMode, Expires: expires})
		response.Success(c, gin.H{"expires_at": expires.UTC().Format(time.RFC3339)})
	})
	r.DELETE("/api/v1/chat/session", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		// Clear Open WebUI's HttpOnly login too, so switching accounts cannot retain its old session.
		for _, name := range []string{chatCookie, "token"} {
			http.SetCookie(c.Writer, &http.Cookie{Name: name, Path: chatPrefix, HttpOnly: true, Secure: chatRequestHTTPS(c), SameSite: http.SameSiteStrictMode, MaxAge: -1})
		}
		if proxy == nil {
			response.Error(c, 503, "对话工作台未启用或暂不可用")
			return
		}
		response.Success(c, gin.H{})
	})
}
func chatRequestHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
}
func chatStaticRequest(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	p := strings.TrimPrefix(r.URL.Path, chatPrefix)
	for _, prefix := range []string{"/_app/", "/static/", "/assets/", "/embed/", "/favicon"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	if p == "/manifest.json" {
		return true
	}
	if r.Method != http.MethodGet || !strings.Contains(r.Header.Get("Accept"), "text/html") {
		return false
	}
	for _, prefix := range []string{"/api", "/ws", "/oauth", "/__sub2api"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return false
		}
	}
	return true
}
func setChatSecurityHeaders(h http.Header, p string) {
	h.Set("X-Frame-Options", "SAMEORIGIN")
	policy := chatCSP
	if p == chatPrefix+"/embed/artifact-frame.html" {
		policy = chatArtifactCSP
	}
	h.Set("Content-Security-Policy", policy)
}
func newChatReverseProxy(raw string) *httputil.ReverseProxy {
	target, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil
	}
	transport := &http.Transport{}
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = base.Clone()
	}
	transport.Proxy = nil // internal credentials must never traverse an environment HTTP proxy
	return &httputil.ReverseProxy{
		Transport: transport,
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = strings.TrimRight(target.Path, "/") + strings.TrimPrefix(pr.In.URL.Path, chatPrefix)
			if pr.Out.URL.Path == "" {
				pr.Out.URL.Path = "/"
			}
			pr.Out.URL.RawPath = ""
			for k := range pr.Out.Header {
				lower := strings.ToLower(k)
				if strings.HasPrefix(lower, "x-sub2api-") || strings.HasPrefix(lower, "x-forwarded-") || lower == "x-real-ip" || lower == "forwarded" {
					delete(pr.Out.Header, k)
				}
			}
			if headers, ok := pr.In.Context().Value(chatProxyHeadersKey{}).(http.Header); ok {
				for k, v := range headers {
					pr.Out.Header[k] = v
				}
			}
		},
		ModifyResponse: func(res *http.Response) error {
			res.Header.Del("X-Frame-Options")
			res.Header.Del("Content-Security-Policy")
			if headers, ok := res.Request.Context().Value(chatProxyHeadersKey{}).(http.Header); ok && headers.Get("X-Sub2API-User-Id") != "" {
				res.Header.Del("Cache-Control")
			}
			loc := res.Header.Get("Location")
			if strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && loc != chatPrefix && !strings.HasPrefix(loc, chatPrefix+"/") {
				res.Header.Set("Location", chatPrefix+loc)
			}
			cookies := res.Header.Values("Set-Cookie")
			res.Header.Del("Set-Cookie")
			for _, value := range cookies {
				parts := strings.Split(value, ";")
				for i := 1; i < len(parts); i++ {
					kv := strings.SplitN(strings.TrimSpace(parts[i]), "=", 2)
					if len(kv) == 2 && strings.EqualFold(kv[0], "Path") && kv[1] == "/" {
						parts[i] = " Path=" + chatPrefix
					}
				}
				res.Header.Add("Set-Cookie", strings.Join(parts, ";"))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Never log URL/query/body/headers: they can contain tokens or user data.
			slog.Warn("chat.proxy_unavailable")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(503)
			if _, writeErr := io.WriteString(w, `{"error":{"code":"CHAT_UNAVAILABLE","message":"对话工作台暂不可用"}}`); writeErr != nil {
				slog.Debug("chat.error_response_write_failed")
			}
		},
	}
}
func limitChatBody(c *gin.Context) bool {
	req := c.Request
	limit := int64(5 << 20)
	if strings.HasPrefix(req.URL.Path, chatPrefix+"/api/v1/files/") {
		// Allow a 25 MiB file plus multipart headers/metadata. The fork still
		// enforces the per-file 25 MiB limit. Other requests remain at 5 MiB.
		limit = 26 << 20
	}
	if req.ContentLength > limit {
		chatError(c, 413, "BODY_TOO_LARGE", "请求体超过大小限制")
		return false
	}
	if req.Body == nil {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, limit+1))
	if closeErr := req.Body.Close(); closeErr != nil {
		slog.Debug("chat.request_body_close_failed")
	}
	if int64(len(body)) > limit {
		chatError(c, 413, "BODY_TOO_LARGE", "请求体超过大小限制")
		return false
	}
	if err != nil {
		chatError(c, 400, "INVALID_BODY", "无法读取请求体")
		return false
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	return true
}
func chatSelectedKey(ctx context.Context, b chatBackend, userID int64, raw string) *service.APIKey {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	key, err := b.GetKey(ctx, id)
	if err != nil || key == nil || key.UserID != userID || chatKeyReason(key) != "" {
		return nil
	}
	return key
}
func chatKeyReason(k *service.APIKey) string {
	if k.Status == "expired" || k.IsExpired() {
		return "expired"
	}
	if k.Status == "quota_exhausted" || k.IsQuotaExhausted() {
		return "quota_exhausted"
	}
	if !k.IsActive() {
		return "disabled"
	}
	if k.Group == nil || !k.Group.IsActive() {
		return "group_unavailable"
	}
	return ""
}
func chatSupportsImages(g *service.Group) bool {
	// Composite images are routed to OpenAI/Grok by compositeTargetPlatformMiddleware.
	// This is an endpoint capability, not a promise of a schedulable account/model.
	return g != nil && service.GroupAllowsImageGeneration(g) && (g.Platform == service.PlatformOpenAI || g.Platform == service.PlatformGrok || g.Platform == service.PlatformComposite)
}

type chatGroupDTO struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Platform       string `json:"platform"`
	Description    string `json:"description"`
	SupportsImages bool   `json:"supports_images"`
}
type chatKeyGroupDTO struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}
type chatKeyDTO struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	Status         string           `json:"status"`
	Usable         bool             `json:"usable"`
	UnusableReason string           `json:"unusable_reason"`
	Group          *chatKeyGroupDTO `json:"group"`
	SupportsChat   bool             `json:"supports_chat"`
	SupportsImages bool             `json:"supports_images"`
	Quota          float64          `json:"quota"`
	QuotaUsed      float64          `json:"quota_used"`
	ExpiresAt      *time.Time       `json:"expires_at"`
}

func chatGroup(g *service.Group) *chatGroupDTO {
	if g == nil {
		return nil
	}
	return &chatGroupDTO{g.ID, g.Name, g.Platform, g.Description, chatSupportsImages(g)}
}
func chatKey(k *service.APIKey) chatKeyDTO {
	reason := chatKeyReason(k)
	var group *chatKeyGroupDTO
	if k.Group != nil {
		group = &chatKeyGroupDTO{k.Group.ID, k.Group.Name, k.Group.Platform}
	}
	return chatKeyDTO{k.ID, k.Name, k.Status, reason == "", reason, group, k.Group != nil, chatSupportsImages(k.Group), k.Quota, k.QuotaUsed, k.ExpiresAt}
}
func serveChatAPI(c *gin.Context, b chatBackend, u *service.User) {
	endpoint := strings.TrimPrefix(c.Request.URL.Path, chatAPI)
	if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
		media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || media != "application/json" {
			chatError(c, 415, "JSON_REQUIRED", "请使用 application/json")
			return
		}
	}
	switch {
	case endpoint == "workbench" && c.Request.Method == "GET":
		wb, ok := b.(interface {
			WorkbenchState(context.Context, int64, string) (*service.WorkbenchState, error)
		})
		if !ok {
			chatError(c, 503, "WORKBENCH_UNAVAILABLE", "工作台配置暂不可用")
			return
		}
		state, err := wb.WorkbenchState(c.Request.Context(), u.ID, "chat")
		if err != nil {
			chatServiceError(c, err)
			return
		}
		c.JSON(200, state)
	case endpoint == "keys" && c.Request.Method == "GET":
		keys, err := b.ListKeys(c.Request.Context(), u.ID)
		if err != nil {
			chatServiceError(c, err)
			return
		}
		out := make([]chatKeyDTO, 0, len(keys))
		for i := range keys {
			if keys[i].UserID == u.ID {
				out = append(out, chatKey(&keys[i]))
			}
		}
		c.JSON(200, gin.H{"keys": out})
	case endpoint == "groups" && c.Request.Method == "GET":
		groups, err := b.Groups(c.Request.Context(), u.ID)
		if err != nil {
			chatServiceError(c, err)
			return
		}
		out := make([]*chatGroupDTO, 0, len(groups))
		for i := range groups {
			out = append(out, chatGroup(&groups[i]))
		}
		c.JSON(200, gin.H{"groups": out})
	case endpoint == "keys" && c.Request.Method == "POST":
		var input struct {
			Name    string `json:"name"`
			GroupID int64  `json:"group_id"`
		}
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 || input.GroupID <= 0 {
			chatError(c, 400, "INVALID_INPUT", "请输入名称和有效分组")
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			chatError(c, 400, "INVALID_INPUT", "请求必须为单个 JSON 对象")
			return
		}
		groups, err := b.Groups(c.Request.Context(), u.ID)
		if err != nil {
			chatServiceError(c, err)
			return
		}
		allowed := false
		for _, g := range groups {
			if g.ID == input.GroupID {
				allowed = true
				break
			}
		}
		if !allowed {
			chatError(c, 403, "GROUP_NOT_ALLOWED", "该分组不可用")
			return
		}
		key, err := b.CreateKey(c.Request.Context(), u.ID, strings.TrimSpace(input.Name), input.GroupID)
		if err != nil {
			chatServiceError(c, err)
			return
		}
		// Create may not preload the group relationship; use the already authorized group.
		for i := range groups {
			if groups[i].ID == input.GroupID {
				key.Group = &groups[i]
				break
			}
		}
		c.JSON(200, gin.H{"key": chatKey(key)})
	case endpoint == "model-prices" && c.Request.Method == "GET":
		key := chatOwnedKey(c, b, u)
		if key == nil {
			return
		}
		out := []service.ChatModelPrice{}
		if key.GroupID != nil {
			var err error
			out, err = b.ModelPrices(c.Request.Context(), u.ID, *key.GroupID)
			if err != nil {
				chatServiceError(c, err)
				return
			}
		}
		c.JSON(200, gin.H{"models": out})
	case endpoint == "model-reasoning" && c.Request.Method == "GET":
		key := chatOwnedKey(c, b, u)
		if key == nil {
			return
		}
		models := c.QueryArray("model")
		if len(models) > chatMaxReasoningModels {
			chatError(c, 400, "TOO_MANY_MODELS", "模型数量过多")
			return
		}
		ids := make([]string, 0, len(models))
		for _, m := range models {
			if m = strings.TrimSpace(m); m != "" && len(m) <= 200 {
				ids = append(ids, m)
			}
		}
		out := map[string]*service.ChatModelReasoning{}
		if key.GroupID != nil && len(ids) > 0 {
			var err error
			out, err = b.ModelReasoning(c.Request.Context(), u.ID, *key.GroupID, ids)
			if err != nil {
				chatServiceError(c, err)
				return
			}
		}
		c.JSON(200, gin.H{"models": out})
	default:
		chatError(c, 404, "NOT_FOUND", "接口不存在")
	}
}

// chatMaxReasoningModels bounds the model-reasoning query (Open WebUI lists every gateway model).
const chatMaxReasoningModels = 300

// chatOwnedKey resolves ?key_id= to one of the user's keys, writing the error response otherwise.
func chatOwnedKey(c *gin.Context, b chatBackend, u *service.User) *service.APIKey {
	id, err := strconv.ParseInt(c.Query("key_id"), 10, 64)
	if err != nil || id <= 0 {
		chatError(c, 400, "INVALID_KEY_ID", "密钥编号无效")
		return nil
	}
	key, err := b.GetKey(c.Request.Context(), id)
	if err != nil || key == nil || key.UserID != u.ID {
		chatError(c, 404, "KEY_NOT_FOUND", "密钥不存在")
		return nil
	}
	return key
}
func chatError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
func chatServiceError(c *gin.Context, err error) {
	var app *infraerrors.ApplicationError
	if errors.As(err, &app) && app.Code >= 400 && app.Code < 500 {
		chatError(c, int(app.Code), app.Reason, "操作未完成，请检查分组权限或密钥数量限制")
		return
	}
	chatError(c, 500, "INTERNAL_ERROR", "服务暂不可用，请稍后重试")
}
