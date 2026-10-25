//go:build unit

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type chatFakeBackend struct {
	keys       map[int64]*service.APIKey
	groups     []service.Group
	role, name string
	fail       bool
	created    bool
}

func newChatFake() *chatFakeBackend {
	g := service.Group{ID: 3, Name: "OpenAI", Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowImageGeneration: true}
	return &chatFakeBackend{keys: map[int64]*service.APIKey{12: {ID: 12, UserID: 42, Name: "对话", Key: "sk-chat-plaintext", Status: service.StatusActive, GroupID: &g.ID, Group: &g}}, groups: []service.Group{g}}
}
func (b *chatFakeBackend) Authenticate(_ context.Context, token string) (*service.User, time.Time, error) {
	if token != "valid" {
		return nil, time.Time{}, service.ErrInvalidToken
	}
	return &service.User{ID: 42, Username: b.name, Email: "alice@example.test", Role: b.role, Status: service.StatusActive}, time.Now().Add(time.Hour), nil
}
func (b *chatFakeBackend) GetKey(_ context.Context, id int64) (*service.APIKey, error) {
	k := b.keys[id]
	if k == nil {
		return nil, service.ErrAPIKeyNotFound
	}
	return k, nil
}
func (b *chatFakeBackend) ListKeys(context.Context, int64) ([]service.APIKey, error) {
	if b.fail {
		return nil, errors.New("secret database detail")
	}
	out := []service.APIKey{}
	for _, k := range b.keys {
		out = append(out, *k)
	}
	return out, nil
}
func (b *chatFakeBackend) Groups(context.Context, int64) ([]service.Group, error) {
	return b.groups, nil
}
func (b *chatFakeBackend) CreateKey(_ context.Context, uid int64, name string, gid int64) (*service.APIKey, error) {
	b.created = true
	return &service.APIKey{ID: 13, UserID: uid, Name: name, Key: "sk-created-secret", Status: service.StatusActive, GroupID: &gid}, nil
}
func (b *chatFakeBackend) ModelPrices(context.Context, int64, int64) ([]service.ChatModelPrice, error) {
	return []service.ChatModelPrice{{ID: "model", PriceText: "输入 ¥3.00 / 百万 tokens"}}, nil
}
func chatTestRouter(t *testing.T, target string, b chatBackend) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "global")
		c.Next()
	})
	installChatProxy(r, &config.Config{Chat: config.ChatConfig{WebUIURL: target}}, b)
	r.NoRoute(func(c *gin.Context) { c.String(200, "spa") })
	return r
}
func chatRequest(t *testing.T, r http.Handler, method, p, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, p, strings.NewReader(body))
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	req = req.WithContext(ctx)
	req.RemoteAddr = "198.51.100.20:9000"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestChatProxyDisabledAndUnavailable(t *testing.T) {
	for _, target := range []string{"", "ftp://bad", "http://127.0.0.1:1"} {
		r := chatTestRouter(t, target, newChatFake())
		w := chatRequest(t, r, "GET", "/_chat/api/config", "", map[string]string{"Cookie": "s2a_chat=valid"})
		require.Equal(t, 503, w.Code)
		if target == "" {
			for _, method := range []string{"POST", "DELETE"} {
				w = chatRequest(t, r, method, "/api/v1/chat/session", "", nil)
				require.Equal(t, 503, w.Code)
			}
		}
	}
	r := chatTestRouter(t, "", newChatFake())
	w := chatRequest(t, r, "GET", "/api/v1/chat/status", "", nil)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"enabled":false}}`, w.Body.String())
}
func TestChatProxySession(t *testing.T) {
	r := chatTestRouter(t, "http://127.0.0.1:1", newChatFake())
	w := chatRequest(t, r, "POST", "/api/v1/chat/session", "", map[string]string{"Authorization": "Bearer valid", "X-Forwarded-Proto": "https"})
	require.Equal(t, 200, w.Code)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	ck := cookies[0]
	require.Equal(t, "s2a_chat", ck.Name)
	require.Equal(t, "valid", ck.Value)
	require.Equal(t, "/_chat", ck.Path)
	require.True(t, ck.HttpOnly)
	require.True(t, ck.Secure)
	require.Equal(t, http.SameSiteStrictMode, ck.SameSite)
	require.Contains(t, w.Body.String(), "expires_at")
	w = chatRequest(t, r, "DELETE", "/api/v1/chat/session", "", nil)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Header().Get("Set-Cookie"), "Max-Age=0")
	for _, authorization := range []string{"", "Basic valid", "Bearer invalid"} {
		w = chatRequest(t, r, "POST", "/api/v1/chat/session", "", map[string]string{"Authorization": authorization})
		require.Equal(t, 401, w.Code)
	}
}
func TestChatProxyDeleteSessionClearsBothCookies(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		for _, auth := range []string{"", "invalid", "valid"} {
			for _, scheme := range []string{"http", "https", "forwarded_https"} {
				t.Run(fmt.Sprintf("enabled=%t/auth=%s/%s", enabled, auth, scheme), func(t *testing.T) {
					target, wantStatus := "http://127.0.0.1:1", http.StatusOK
					if !enabled {
						target, wantStatus = "", http.StatusServiceUnavailable
					}
					headers := map[string]string{}
					if auth != "" {
						headers["Authorization"] = "Bearer " + auth
						headers["Cookie"] = "s2a_chat=" + auth + "; token=old-webui-session"
					}
					requestURL := "http://console.test/api/v1/chat/session"
					if scheme == "https" {
						requestURL = "https://console.test/api/v1/chat/session"
					} else if scheme == "forwarded_https" {
						headers["X-Forwarded-Proto"] = "https"
					}
					w := chatRequest(t, chatTestRouter(t, target, newChatFake()), "DELETE", requestURL, "", headers)
					require.Equal(t, wantStatus, w.Code)
					require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
					cookies := w.Result().Cookies()
					require.Len(t, cookies, 2)
					names := make([]string, 0, 2)
					for _, cookie := range cookies {
						names = append(names, cookie.Name)
						require.Empty(t, cookie.Value)
						require.Empty(t, cookie.Domain)
						require.Equal(t, "/_chat", cookie.Path)
						require.Equal(t, -1, cookie.MaxAge)
						require.True(t, cookie.HttpOnly)
						require.Equal(t, scheme != "http", cookie.Secure)
						require.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
					}
					require.ElementsMatch(t, []string{"s2a_chat", "token"}, names)
					for _, raw := range w.Header().Values("Set-Cookie") {
						require.Contains(t, raw, "Max-Age=0")
					}
				})
			}
		}
	}
}

func TestChatProxyNameHeaderEncoding(t *testing.T) {
	for _, tc := range []struct{ label, name, want string }{
		{"chinese", "中文用户", "%E4%B8%AD%E6%96%87%E7%94%A8%E6%88%B7"},
		{"spaces", " Alice Smith ", "%20Alice%20Smith%20"},
		{"percent", "100%20", "100%2520"},
		{"emoji", "😀🚀", "%F0%9F%98%80%F0%9F%9A%80"},
		{"unreserved", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"},
		{"reserved", ":/?#[]@!$&'()*+,;=", "%3A%2F%3F%23%5B%5D%40%21%24%26%27%28%29%2A%2B%2C%3B%3D"},
		{"fallback", "", "alice"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			headers := make(chan http.Header, 1)
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headers <- r.Header.Clone()
				w.WriteHeader(http.StatusNoContent)
			}))
			defer up.Close()
			b := newChatFake()
			b.name = tc.name
			w := chatRequest(t, chatTestRouter(t, up.URL, b), "GET", "/_chat/api/config", "", map[string]string{"Cookie": "s2a_chat=valid", "X-Sub2API-Name": "forged"})
			require.Equal(t, http.StatusNoContent, w.Code)
			require.Equal(t, tc.want, (<-headers).Get("X-Sub2API-Name"))
		})
	}
}

func TestChatProxyInvalidSessionErrorCode(t *testing.T) {
	var hits atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer up.Close()
	r := chatTestRouter(t, up.URL, newChatFake())
	for _, cookie := range []string{"", "s2a_chat=", "s2a_chat=invalid", "token=webui-only"} {
		for _, path := range []string{"/_chat/api/config", "/_chat/ws/socket.io/", "/_chat/oauth/login", "/_chat/__sub2api/keys"} {
			t.Run(cookie+path, func(t *testing.T) {
				w := chatRequest(t, r, "GET", path, "", map[string]string{"Cookie": cookie, "Accept": "text/html"})
				require.Equal(t, http.StatusUnauthorized, w.Code)
				require.JSONEq(t, `{"error":{"code":"CHAT_SESSION_INVALID","message":"登录已失效"}}`, w.Body.String())
			})
		}
	}
	require.Zero(t, hits.Load(), "invalid sessions must not reach Open WebUI")
}

func TestChatProxyAuthenticationStaticAndHeaderIsolation(t *testing.T) {
	hits := make(chan *http.Request, 40)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits <- r.Clone(r.Context())
		w.Header().Set("Content-Security-Policy", "evil")
		w.Header().Set("X-Frame-Options", "DENY")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"path": r.URL.RequestURI()}))
	}))
	defer up.Close()
	b := newChatFake()
	b.role = "admin"
	r := chatTestRouter(t, up.URL, b)
	for _, p := range []string{"/_chat/api/config", "/_chat/ws/socket.io/", "/_chat/oauth/login", "/_chat/__sub2api/keys", "/_chat/api", "/_chat/ws"} {
		w := chatRequest(t, r, "GET", p, "", map[string]string{"Accept": "text/html", "X-Sub2API-User-Id": "42"})
		require.Equal(t, 401, w.Code, p)
	}
	require.Empty(t, hits)
	forged := map[string]string{"Accept": "text/html", "Cookie": "s2a_chat=valid; token=owui", "Authorization": "Bearer owui", "X-Sub2API-Key-Id": "12", "X-Sub2API-Key": "forged", "X-Sub2API-Image-Key": "forged", "X-Sub2API-Key-Platform": "forged", "X-Sub2API-Other": "evil", "X-Forwarded-For": "1.2.3.4", "X-Forwarded-Host": "evil", "X-Forwarded-Proto": "https", "X-Real-IP": "1.2.3.4", "Forwarded": "for=1.2.3.4"}
	for _, p := range []string{"/_chat/_app/a.js", "/_chat/static/a", "/_chat/assets/a", "/_chat/favicon.ico", "/_chat/manifest.json", "/_chat/embed/artifact-frame.html", "/_chat/", "/_chat/c/123"} {
		w := chatRequest(t, r, "GET", p, "", forged)
		require.Equal(t, 200, w.Code, p)
		got := <-hits
		for k := range got.Header {
			require.False(t, strings.HasPrefix(strings.ToLower(k), "x-sub2api-"), k)
		}
		require.Equal(t, "198.51.100.20", got.Header.Get("X-Forwarded-For"))
		require.Empty(t, got.Header.Get("Forwarded"))
		require.Empty(t, got.Header.Get("X-Forwarded-Host"))
		require.Empty(t, got.Header.Get("X-Forwarded-Proto"))
		require.NotContains(t, got.Header.Get("Cookie"), chatCookie)
		want := chatCSP
		if strings.HasSuffix(p, "artifact-frame.html") {
			want = chatArtifactCSP
		}
		require.Equal(t, []string{want}, w.Header().Values("Content-Security-Policy"))
		require.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	}
	w := chatRequest(t, r, "GET", "/_chat/api/config?q=a%2Fb", "", forged)
	require.Equal(t, 200, w.Code)
	got := <-hits
	require.Equal(t, "/api/config?q=a%2Fb", got.URL.RequestURI())
	require.Equal(t, "42", got.Header.Get("X-Sub2API-User-Id"))
	require.Equal(t, "u42@sub2api.internal", got.Header.Get("X-Sub2API-Email"))
	require.Equal(t, "alice", got.Header.Get("X-Sub2API-Name"))
	require.Equal(t, "admin", got.Header.Get("X-Sub2API-User-Role"))
	require.Equal(t, "sk-chat-plaintext", got.Header.Get("X-Sub2API-Key"))
	require.Equal(t, "openai", got.Header.Get("X-Sub2API-Key-Platform"))
	require.Empty(t, got.Header.Get("X-Sub2API-Image-Key"))
	require.Empty(t, got.Header.Get("X-Sub2API-Other"))
	require.Equal(t, "token=owui", got.Header.Get("Cookie"))
	require.Equal(t, "Bearer owui", got.Header.Get("Authorization"))
	for _, p := range []string{"/chat", "/_chatty", "/api/v1/other"} {
		w = chatRequest(t, r, "GET", p, "", nil)
		require.Equal(t, "global", w.Header().Get("Content-Security-Policy"))
		require.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	}
	w = chatRequest(t, r, "GET", "/_chat/static/../api/config", "", nil)
	require.Equal(t, 400, w.Code)
}
func TestChatProxyKeyValidation(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign", "disabled", "expired", "quota", "group", "no_group", "bad_id", "image_anthropic", "image_disabled"} {
		t.Run(scenario, func(t *testing.T) {
			b := newChatFake()
			key := b.keys[12]
			id := "12"
			switch scenario {
			case "foreign":
				key.UserID = 43
			case "disabled":
				key.Status = "inactive"
			case "expired":
				past := time.Now().Add(-time.Minute)
				key.ExpiresAt = &past
			case "quota":
				key.Quota = 1
				key.QuotaUsed = 1
			case "group":
				key.Group.Status = "inactive"
			case "no_group":
				key.Group = nil
			case "bad_id":
				id = "x"
			case "image_anthropic":
				key.Group.Platform = "anthropic"
			case "image_disabled":
				key.Group.AllowImageGeneration = false
			}
			hits := make(chan http.Header, 1)
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits <- r.Header.Clone(); w.WriteHeader(204) }))
			defer up.Close()
			w := chatRequest(t, chatTestRouter(t, up.URL, b), "POST", "/_chat/api/chat/completions", "{}", map[string]string{"Cookie": "s2a_chat=valid", "X-Sub2API-Key-Id": id, "X-Sub2API-Image-Key-Id": id, "X-Sub2API-Key": "fake", "X-Sub2API-Image-Key": "fake"})
			require.Equal(t, 204, w.Code)
			h := <-hits
			valid := scenario == "valid" || scenario == "image_anthropic" || scenario == "image_disabled"
			if valid {
				require.Equal(t, key.Key, h.Get("X-Sub2API-Key"))
				require.Equal(t, key.Group.Platform, h.Get("X-Sub2API-Key-Platform"))
			} else {
				require.Empty(t, h.Get("X-Sub2API-Key"))
				require.Empty(t, h.Get("X-Sub2API-Key-Platform"))
			}
			if scenario == "valid" {
				require.Equal(t, key.Key, h.Get("X-Sub2API-Image-Key"))
			} else {
				require.Empty(t, h.Get("X-Sub2API-Image-Key"))
			}
		})
	}
}
func TestChatProxyOwnAPI(t *testing.T) {
	b := newChatFake()
	r := chatTestRouter(t, "http://127.0.0.1:1", b)
	h := map[string]string{"Cookie": "s2a_chat=valid", "Content-Type": "application/json"}
	w := chatRequest(t, r, "GET", chatAPI+"keys", "", h)
	require.Equal(t, 200, w.Code)
	require.NotContains(t, w.Body.String(), "sk-")
	require.NotContains(t, w.Body.String(), `"code"`)
	require.Contains(t, w.Body.String(), `"supports_images":true`)
	require.Contains(t, w.Body.String(), `"platform":"openai"`)
	w = chatRequest(t, r, "GET", chatAPI+"groups", "", h)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"groups"`)
	w = chatRequest(t, r, "POST", chatAPI+"keys", `{"name":"新密钥","group_id":3}`, h)
	require.Equal(t, 200, w.Code)
	require.True(t, b.created)
	require.NotContains(t, w.Body.String(), "secret")
	require.Contains(t, w.Body.String(), `"supports_images":true`)
	for _, body := range []string{`{"name":"x","group_id":9}`, `{"name":"x","group_id":3,"user_id":43}`, `{"name":"x","group_id":3} {}`} {
		b.created = false
		w = chatRequest(t, r, "POST", chatAPI+"keys", body, h)
		require.Contains(t, []int{400, 403}, w.Code)
		require.False(t, b.created)
	}
	w = chatRequest(t, r, "POST", chatAPI+"keys", "name=x", map[string]string{"Cookie": "s2a_chat=valid", "Content-Type": "application/x-www-form-urlencoded"})
	require.Equal(t, 415, w.Code)
	w = chatRequest(t, r, "GET", chatAPI+"model-prices?key_id=12", "", h)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "price_text")
	b.keys[12].UserID = 43
	w = chatRequest(t, r, "GET", chatAPI+"model-prices?key_id=12", "", h)
	require.Equal(t, 404, w.Code)
	b.fail = true
	w = chatRequest(t, r, "GET", chatAPI+"keys", "", h)
	require.Equal(t, 500, w.Code)
	require.NotContains(t, w.Body.String(), "database")
}
func TestChatProxyLocationCookiesAndBodyLimits(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Location", r.URL.Query().Get("location"))
		w.Header().Add("Set-Cookie", "token=owui; Path=/; HttpOnly")
		w.Header().Add("Set-Cookie", "other=x; Path=/already; SameSite=Lax")
		w.WriteHeader(200)
	}))
	defer up.Close()
	r := chatTestRouter(t, up.URL, newChatFake())
	h := map[string]string{"Cookie": "s2a_chat=valid"}
	w := chatRequest(t, r, "GET", "/_chat/api/x?location=/static/logo.png", "", h)
	require.Equal(t, "/_chat/static/logo.png", w.Header().Get("Location"))
	require.Equal(t, []string{"token=owui; Path=/_chat; HttpOnly", "other=x; Path=/already; SameSite=Lax"}, w.Header().Values("Set-Cookie"))
	for _, loc := range []string{"//external.example/x", "https://external.example/x", "/_chat/api/x"} {
		w = chatRequest(t, r, "GET", "/_chat/api/x?location="+loc, "", h)
		require.Equal(t, loc, w.Header().Get("Location"))
	}
	for _, tc := range []struct {
		p    string
		size int
		code int
	}{{"/_chat/api/x", 5 << 20, 200}, {"/_chat/api/x", (5 << 20) + 1, 413}, {"/_chat/api/v1/files/", (5 << 20) + 1, 200}, {"/_chat/api/v1/files/", (25 << 20) + 1, 200}, {"/_chat/api/v1/files/", 26 << 20, 200}, {"/_chat/api/v1/files/", (26 << 20) + 1, 413}} {
		before := hits.Load()
		w = chatRequest(t, r, "POST", tc.p, strings.Repeat("x", tc.size), h)
		require.Equal(t, tc.code, w.Code)
		if tc.code == 413 {
			require.Equal(t, before, hits.Load())
		}
	}
	req := httptest.NewRequest("POST", "/_chat/api/x", strings.NewReader(strings.Repeat("x", (5<<20)+1)))
	req.ContentLength = -1
	req.Header.Set("Cookie", "s2a_chat=valid")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 413, w.Code)
}
func TestChatProxyWebSocketNoKeys(t *testing.T) {
	headers := make(chan http.Header, 1)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				t.Error(err)
			}
		}()
		typ, data, err := conn.ReadMessage()
		if err != nil {
			t.Error(err)
			return
		}
		if err := conn.WriteMessage(typ, data); err != nil {
			t.Error(err)
		}
	}))
	defer up.Close()
	b := newChatFake()
	b.name = "中文 100% 😀"
	proxy := httptest.NewServer(chatTestRouter(t, up.URL, b))
	defer proxy.Close()
	h := http.Header{"Cookie": {"s2a_chat=valid"}, "X-Sub2api-Key-Id": {"12"}, "X-Sub2api-Image-Key-Id": {"12"}, "X-Sub2api-Key": {"fake"}}
	conn, res, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(proxy.URL, "http")+"/_chat/ws/socket.io/?transport=websocket", h)
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	require.Equal(t, 101, res.StatusCode)
	got := <-headers
	require.Equal(t, "%E4%B8%AD%E6%96%87%20100%25%20%F0%9F%98%80", got.Get("X-Sub2API-Name"))
	require.Equal(t, "42", got.Get("X-Sub2API-User-Id"))
	for _, k := range []string{"X-Sub2API-Key", "X-Sub2API-Key-Platform", "X-Sub2API-Image-Key", "X-Sub2API-Key-Id", "X-Sub2API-Image-Key-Id"} {
		require.Empty(t, got.Get(k), k)
	}
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("echo")))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, "echo", string(data))
}
func TestChatGatewayIPTrust(t *testing.T) {
	// These are the same helpers used by usage creation and API-key IP ACL.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies([]string{"192.168.241.0/24"}))
	r.Use(func(c *gin.Context) { ip.SetForwardedIPSettings(c, false, nil); c.Next() })
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(200, gin.H{"usage_ip": ip.GetClientIP(c), "acl_ip": ip.GetSecurityClientIP(c, false)})
	})
	for _, tc := range []struct{ remote, want string }{{"192.168.241.2:8080", "203.0.113.7"}, {"198.51.100.8:1234", "198.51.100.8"}} {
		req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.JSONEq(t, `{"usage_ip":"`+tc.want+`","acl_ip":"`+tc.want+`"}`, w.Body.String())
	}
}

func TestChatSupportsImagesPlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok, service.PlatformComposite, service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity} {
		g := &service.Group{Platform: platform, AllowImageGeneration: true}
		want := platform == service.PlatformOpenAI || platform == service.PlatformGrok || platform == service.PlatformComposite
		require.Equal(t, want, chatSupportsImages(g), platform)
		g.AllowImageGeneration = false
		require.False(t, chatSupportsImages(g), platform)
	}
	require.False(t, chatSupportsImages(nil))
}
func TestChatKeyUnusableReasons(t *testing.T) {
	b := newChatFake()
	key := b.keys[12]
	require.Empty(t, chatKey(key).UnusableReason)
	for _, tc := range []struct{ status, reason string }{{"inactive", "disabled"}, {"expired", "expired"}, {"quota_exhausted", "quota_exhausted"}} {
		key.Status = tc.status
		out := chatKey(key)
		require.False(t, out.Usable)
		require.Equal(t, tc.reason, out.UnusableReason)
	}
	key.Status = "active"
	key.Group = nil
	require.Equal(t, "group_unavailable", chatKey(key).UnusableReason)
}
func TestChatProxyInternalTransportDoesNotUseEnvironmentProxy(t *testing.T) {
	proxy := newChatReverseProxy("http://127.0.0.1:8080")
	transport, ok := proxy.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy)
}

func TestChatProxyStaticWithoutCookieAndTrustedIP(t *testing.T) {
	hits := make(chan http.Header, 2)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits <- r.Header.Clone(); w.WriteHeader(204) }))
	defer up.Close()
	b := newChatFake()
	b.name = "Alice"
	r := chatTestRouter(t, up.URL, b)
	w := chatRequest(t, r, "GET", "/_chat/_app/index.js", "", nil)
	require.Equal(t, 204, w.Code)
	require.Empty(t, (<-hits).Get("X-Sub2API-User-Id"))
	require.NoError(t, r.SetTrustedProxies([]string{"198.51.100.0/24"}))
	w = chatRequest(t, r, "GET", "/_chat/api/config", "", map[string]string{"Cookie": "s2a_chat=valid", "X-Forwarded-For": "203.0.113.9", "X-Sub2API-User-Id": "999", "Connection": "X-Sub2API-User-Id, X-Forwarded-For"})
	require.Equal(t, 204, w.Code)
	got := <-hits
	require.Equal(t, "203.0.113.9", got.Get("X-Forwarded-For"))
	require.Equal(t, "203.0.113.9", got.Get("X-Real-IP"))
	require.Equal(t, "42", got.Get("X-Sub2API-User-Id"))
	require.Equal(t, "user", got.Get("X-Sub2API-User-Role"))
	require.Equal(t, "Alice", got.Get("X-Sub2API-Name"))
}

// A legal maximum-size file must not be rejected because of its multipart envelope.
func TestChatProxyMultipartFileLimit(t *testing.T) {
	var hits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		mr, err := r.MultipartReader()
		require.NoError(t, err)
		part, err := mr.NextPart()
		require.NoError(t, err)
		n, err := io.Copy(io.Discard, part)
		require.NoError(t, err)
		require.EqualValues(t, 25<<20, n)
		w.WriteHeader(http.StatusOK)
	}))
	defer up.Close()
	r := chatTestRouter(t, up.URL, newChatFake())
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "image.png")
	require.NoError(t, err)
	_, err = io.Copy(part, strings.NewReader(strings.Repeat("x", 25<<20)))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	for _, chunked := range []bool{false, true} {
		req := httptest.NewRequest("POST", "/_chat/api/v1/files/", bytes.NewReader(body.Bytes()))
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		req = req.WithContext(ctx)
		req.Header.Set("Cookie", "s2a_chat=valid")
		req.Header.Set("Content-Type", mw.FormDataContentType())
		if chunked {
			req.ContentLength = -1
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
	}
	require.EqualValues(t, 2, hits.Load())
	req := httptest.NewRequest("POST", "/_chat/api/v1/files/", strings.NewReader(strings.Repeat("x", (26<<20)+1)))
	req.ContentLength = -1
	req.Header.Set("Cookie", "s2a_chat=valid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	require.EqualValues(t, 2, hits.Load())
}
