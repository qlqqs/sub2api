package server

// CUSTOM（fork）：创作工作台（Nova Studio）的同源转发。
//
// 浏览器只访问本站；/_nova/*（静态资源）与 /api/nova/*（含 WebSocket /api/nova/ws）
// 转给 studio.nova_url 指向的 Nova 容器。/api/nova/* 先经用户 JWT 校验，再注入
// 用户 ID / 角色 / 真实 IP；客户端自带的同名头一律丢弃。
// 双方契约见 qlqqs/nova-image-studio 的 docs/sub2api-embed.md。

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

const (
	studioStaticPrefix = "/_nova"
	studioAPIPrefix    = "/api/nova"
	studioWSPath       = "/api/nova/ws"
	studioStatusPath   = "/api/v1/studio/status"

	studioHeaderUserID   = "X-Sub2API-User-Id"
	studioHeaderUserRole = "X-Sub2API-User-Role"
	studioHeaderPrefix   = "x-sub2api-" // 小写比较，用于剥离客户端自带的同名头

	// Nova 是 Next 静态导出，含内联脚本，无法使用本站的 nonce 策略；该策略只作用于 /_nova/*。
	// connect-src 只允许同源：浏览器端也无法直连任何外部上游。
	studioContentSecurityPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; " +
		"connect-src 'self'; worker-src 'self' blob:; frame-src 'self' blob: data:; " +
		"frame-ancestors 'self'; base-uri 'self'; form-action 'self'"
)

type studioClientIPKey struct{}

// installStudioProxy 在 SetupRouter 中、前端 SPA 中间件之前调用：
// /_nova/* 走全局中间件（否则会被 SPA 回退成首页），/api/nova/* 注册为路由以串上 JWT 校验。
func installStudioProxy(r *gin.Engine, cfg *config.Config, jwtAuth gin.HandlerFunc) {
	proxy := newStudioReverseProxy(cfg.Studio.NovaURL)
	// 转给 Nova 的 IP 会被网关用于 Key 的 IP 白名单，按网关校验白名单时的同一规则解析，
	// 使经工作台发出的请求与浏览器直接调用 API 的判定一致。
	trustForwarded := cfg.TrustForwardedIPForAPIKeyACL()

	r.Use(func(c *gin.Context) {
		if !isStudioStaticPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		serveStudio(c, proxy, true, trustForwarded)
	})

	// 前端据此决定是否显示「创作工作台」入口；只反映是否配置，不探测 Nova 是否在线。
	enabled := proxy != nil
	r.GET(studioStatusPath, func(c *gin.Context) {
		response.Success(c, gin.H{"enabled": enabled})
	})

	r.Any(studioAPIPrefix+"/*path", func(c *gin.Context) {
		if c.Request.URL.Path == studioWSPath {
			// 浏览器 WebSocket 无法带 Authorization；由 Nova 校验一次性 ticket。
			c.Next()
			return
		}
		jwtAuth(c)
	}, func(c *gin.Context) {
		serveStudio(c, proxy, false, trustForwarded)
	})
}

func isStudioStaticPath(path string) bool {
	return path == studioStaticPrefix || strings.HasPrefix(path, studioStaticPrefix+"/")
}

func newStudioReverseProxy(rawURL string) *httputil.ReverseProxy {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	target, err := url.Parse(rawURL)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		log.Printf("[Studio] invalid studio.nova_url %q, studio proxy disabled", rawURL)
		return nil
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Rewrite 调用前 X-Forwarded-* 已被删除；只写入本站解析出的真实 IP。
			pr.SetURL(target)
			if clientIP, _ := pr.In.Context().Value(studioClientIPKey{}).(string); clientIP != "" {
				pr.Out.Header.Set("X-Forwarded-For", clientIP)
				pr.Out.Header.Set("X-Real-IP", clientIP)
			}
		},
		ModifyResponse: func(res *http.Response) error {
			// 以本站设置的嵌入策略为准，避免与 Nova 自带的同名头叠加。
			res.Header.Del("X-Frame-Options")
			res.Header.Del("Content-Security-Policy")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("[Studio] proxy %s failed: %v", r.URL.Path, err)
			writeStudioUnavailable(w)
		},
	}
}

func serveStudio(c *gin.Context, proxy *httputil.ReverseProxy, static, trustForwarded bool) {
	defer c.Abort()
	if proxy == nil {
		writeStudioUnavailable(c.Writer)
		return
	}

	req := c.Request
	for name := range req.Header {
		if strings.HasPrefix(strings.ToLower(name), studioHeaderPrefix) {
			req.Header.Del(name)
		}
	}
	// Nova 不需要本站 JWT 与 Cookie。
	req.Header.Del("Authorization")
	req.Header.Del("Cookie")

	if !static && req.URL.Path != studioWSPath {
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			middleware2.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
			return
		}
		role, _ := middleware2.GetUserRoleFromContext(c)
		if role != "admin" {
			role = "user"
		}
		req.Header.Set(studioHeaderUserID, strconv.FormatInt(subject.UserID, 10))
		req.Header.Set(studioHeaderUserRole, role)
	}

	// 全局安全头为整站设置了 DENY / nonce CSP，这里换成允许同源嵌入的专用策略。
	h := c.Writer.Header()
	h.Del("Content-Security-Policy")
	h.Set("X-Frame-Options", "SAMEORIGIN")
	if static {
		h.Set("Content-Security-Policy", studioContentSecurityPolicy)
	}

	ctx := context.WithValue(req.Context(), studioClientIPKey{}, ip.GetSecurityClientIP(c, trustForwarded))
	proxy.ServeHTTP(c.Writer, req.WithContext(ctx))
}

func writeStudioUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(`{"code":503,"message":"创作工作台未启用或暂不可用","reason":"STUDIO_UNAVAILABLE"}`))
}
