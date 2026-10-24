package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientResponseModelPreservesOtherFields(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"chat", ` {"model":"upstream-v2", "choices":[{"message":{"content":"upstream-v2","model":"do not touch"}}],"usage":{"total_tokens":9}} `, ` {"model":"public", "choices":[{"message":{"content":"upstream-v2","model":"do not touch"}}],"usage":{"total_tokens":9}} `},
		{"anthropic", `{"type":"message_start","message":{"model":"vendor/alias","content":[],"usage":{"input_tokens":2}}}`, `{"type":"message_start","message":{"model":"public","content":[],"usage":{"input_tokens":2}}}`},
		{"responses", `{"type":"response.completed","response":{"model":"alias","output":[{"arguments":"{\"model\":\"alias\"}"}]},"metadata":{"model":"alias"}}`, `{"type":"response.completed","response":{"model":"public","output":[{"arguments":"{\"model\":\"alias\"}"}]},"metadata":{"model":"alias"}}`},
		{"gemini", `{"modelVersion":"version","candidates":[{"content":{"parts":[{"text":"version"}]}}]}`, `{"modelVersion":"public","candidates":[{"content":{"parts":[{"text":"version"}]}}]}`},
		{"gemini array", `[{"modelVersion":"v1"}, {"response":{"modelVersion":"v2"}}]`, `[{"modelVersion":"public"}, {"response":{"modelVersion":"public"}}]`},
		{"absent", `{"choices":[],"usage":{"total_tokens":1}}`, `{"choices":[],"usage":{"total_tokens":1}}`},
		{"wrong types", `{"model":null,"response":{"model":42},"message":{"model":{}},"modelVersion":true}`, `{"model":null,"response":{"model":42},"message":{"model":{}},"modelVersion":true}`},
		{"malformed", `{"model":"alias"`, `{"model":"alias"`},
		{"trailing garbage", `{"model":"alias"}garbage`, `{"model":"alias"}garbage`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, string(restoreClientResponseModel([]byte(tc.body), "public")))
		})
	}
	body := []byte(`{"model":"upstream"}`)
	require.Equal(t, body, restoreClientResponseModel(body, ""))
	require.JSONEq(t, `{"model":"a\"b\\c"}`, string(restoreClientResponseModel(body, "a\"b\\c")))
}

func TestClientResponseModelMiddlewareJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, fragmented := range []bool{false, true} {
		for _, composite := range []bool{false, true} {
			r := gin.New()
			r.Use(ClientResponseModelMiddleware())
			r.POST("/v1/responses", func(c *gin.Context) {
				if composite {
					ctx := service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{
						Matched: true, PublicModel: "public", UpstreamModel: "routed", TargetPlatform: service.PlatformOpenAI,
					})
					c.Request = c.Request.WithContext(ctx)
					setOpsRequestContext(c, "routed", false)
				} else {
					setOpsRequestContext(c, "public", false)
				}
				// A later fallback/ops update must not change the client-facing name.
				setOpsRequestContext(c, "billing-model", false)
				c.Header("Content-Type", "application/json")
				c.Header("Content-Length", "20")
				c.Header("X-Request-Id", "request-123")
				body := `{"model":"alias","usage":{"input_tokens":7},"text":"alias"}`
				if fragmented {
					for i := range body {
						n, err := c.Writer.Write([]byte(body[i : i+1]))
						require.NoError(t, err)
						require.Equal(t, 1, n)
					}
				} else {
					n, err := c.Writer.WriteString(body)
					require.NoError(t, err)
					require.Equal(t, len(body), n)
				}
				require.True(t, c.Writer.Written())
				require.Equal(t, "billing-model", c.Request.Context().Value(ctxkey.Model))
				require.Equal(t, "billing-model", c.GetString(opsModelKey))
			})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/responses", nil))
			require.Equal(t, 200, rec.Code)
			require.Equal(t, `{"model":"public","usage":{"input_tokens":7},"text":"alias"}`, rec.Body.String())
			require.Empty(t, rec.Header().Get("Content-Length"))
			require.Equal(t, "request-123", rec.Header().Get("X-Request-Id"))
		}
	}
}

func TestClientResponseModelMiddlewareSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := ": keepalive\r\nevent: message_start\r\ndata:{\"message\":{\"model\":\"alias\"}}\r\n\r\n" +
		"data: {\"response\":{\"model\":\"alias\"},\"delta\":\"alias\"}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"alias\"}}]}\n\ndata: [DONE]\n\n" +
		"data: {\"model\":\"alias\"}" // EOF without newline
	want := strings.ReplaceAll(body, `"model":"alias"`, `"model":"public"`)
	for _, chunkSize := range []int{1, 7, len(body)} {
		r := gin.New()
		r.Use(ClientResponseModelMiddleware())
		r.POST("/v1/messages", func(c *gin.Context) {
			setOpsRequestContext(c, "public", true)
			c.Header("Content-Type", "text/event-stream; charset=utf-8")
			for i := 0; i < len(body); i += chunkSize {
				chunk := body[i:min(i+chunkSize, len(body))]
				n, err := io.WriteString(c.Writer, chunk)
				require.NoError(t, err)
				require.Equal(t, len(chunk), n)
				c.Writer.Flush()
			}
		})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", nil))
		require.Equal(t, want, rec.Body.String())
		require.True(t, rec.Flushed)
	}
}

func TestClientResponseModelMiddlewareBypassesNonInference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, method, contentType, model, encoding string
		status                                     int
	}{
		{"error", "POST", "application/json", "public", "", 400},
		{"models listing", "GET", "application/json", "public", "", 200},
		{"no model", "POST", "application/json", "", "", 200},
		{"binary", "POST", "audio/mpeg", "public", "", 200},
		{"compressed", "POST", "application/json", "public", "gzip", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(ClientResponseModelMiddleware())
			r.Handle(tc.method, "/test", func(c *gin.Context) {
				setOpsRequestContext(c, tc.model, false)
				c.Header("Content-Encoding", tc.encoding)
				c.Data(tc.status, tc.contentType, []byte(`{"model":"alias"}`))
			})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, "/test", nil))
			require.Equal(t, tc.status, rec.Code)
			require.Equal(t, `{"model":"alias"}`, rec.Body.String())
		})
	}
}

func TestClientResponseModelKeepalivesFlushImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, contentType := range []string{"application/json", "text/event-stream"} {
		rec := httptest.NewRecorder()
		r := gin.New()
		r.Use(ClientResponseModelMiddleware())
		r.POST("/test", func(c *gin.Context) {
			setOpsRequestContext(c, "public", true)
			c.Header("Content-Type", contentType)
			keepalive := " \n"
			if contentType == "text/event-stream" {
				keepalive = ": ping\n\n"
			}
			_, err := c.Writer.WriteString(keepalive)
			require.NoError(t, err)
			c.Writer.Flush()
			require.Equal(t, keepalive, rec.Body.String())
			require.True(t, rec.Flushed)
		})
		r.ServeHTTP(rec, httptest.NewRequest("POST", "/test", nil))
	}
}

type failingClientModelWriter struct{ gin.ResponseWriter }

func (w failingClientModelWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestClientResponseModelWriteError(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/test", nil).WithContext(context.Background())
	setOpsRequestContext(c, "public", false)
	c.Header("Content-Type", "application/json")
	w := &clientModelResponseWriter{ResponseWriter: failingClientModelWriter{c.Writer}, context: c}
	_, err := w.Write([]byte(`{"model":"alias"}`))
	require.True(t, errors.Is(err, io.ErrClosedPipe))
}

func TestClientResponseModelHTTPContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ClientResponseModelMiddleware())
	r.POST("/test", func(c *gin.Context) {
		setOpsRequestContext(c, "a-much-longer-public-model", false)
		c.Header("Content-Length", "13")
		c.Data(200, "application/json", []byte(`{"model":"x"}`))
	})
	s := httptest.NewServer(r)
	defer s.Close()
	resp, err := s.Client().Post(s.URL+"/test", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"model":"a-much-longer-public-model"}`, string(body))
}
