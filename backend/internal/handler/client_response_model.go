package handler

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const clientResponseModelKey = "client_response_model"

// Capture once, before channel/account remapping; Gin's synchronized key store
// also allows keepalive goroutines to read without touching c.Request.
func setClientResponseModel(c *gin.Context, model string) {
	if model == "" {
		return
	}
	if _, exists := c.Get(clientResponseModelKey); !exists {
		c.Set(clientResponseModelKey, clientRequestedModel(c, model))
	}
}

// ClientResponseModelMiddleware restores the public request model at the HTTP
// output boundary, after upstream observation, protocol conversion and usage
// extraction. It must not change models used by routing or billing.
// WebSocket turns have their own forwarding path and are not HTTP bodies.
func ClientResponseModelMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		original := c.Writer
		w := &clientModelResponseWriter{ResponseWriter: original, context: c}
		c.Writer = w
		defer func() { c.Writer = original }()
		c.Next()
		if err := w.finish(); err != nil {
			c.Error(err)
		}
	}
}

type clientModelResponseWriter struct {
	gin.ResponseWriter
	context *gin.Context
	pending []byte
}

func (w *clientModelResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *clientModelResponseWriter) model() string {
	return w.context.GetString(clientResponseModelKey)
}

func (w *clientModelResponseWriter) format() string {
	if w.Status() < 200 || w.Status() >= 300 || w.model() == "" || w.Header().Get("Content-Encoding") != "" {
		return ""
	}
	contentType, _, _ := strings.Cut(w.Header().Get("Content-Type"), ";")
	switch strings.TrimSpace(contentType) {
	case "application/json":
		return "json"
	case "text/event-stream":
		return "sse"
	default:
		return ""
	}
}

func (w *clientModelResponseWriter) WriteHeaderNow() {
	if !w.Written() && w.format() != "" {
		// The public name can have a different byte length from the upstream ID.
		w.Header().Del("Content-Length")
	}
	w.ResponseWriter.WriteHeaderNow()
}

func (w *clientModelResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *clientModelResponseWriter) Write(p []byte) (int, error) {
	format := w.format()
	if format == "" {
		return w.ResponseWriter.Write(p)
	}
	w.WriteHeaderNow()
	if format == "json" {
		// Normal handlers write one complete JSON document. Avoid copying it;
		// retain fragments only for writers that split a document across writes.
		if len(w.pending) == 0 && (len(bytes.TrimSpace(p)) == 0 || gjson.ValidBytes(p)) {
			_, err := w.ResponseWriter.Write(restoreClientResponseModel(p, w.model()))
			if err != nil {
				return 0, err
			}
			return len(p), nil
		}
		w.pending = append(w.pending, p...)
		return len(p), nil
	}

	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := restoreClientSSEModel(w.pending[:i+1], w.model())
		if _, err := w.ResponseWriter.Write(line); err != nil {
			w.pending = nil
			return 0, err
		}
		w.pending = w.pending[i+1:]
	}
	if len(w.pending) == 0 {
		w.pending = nil
	}
	return len(p), nil
}

func (w *clientModelResponseWriter) Flush() {
	// Never emit a partial SSE data line: its model may straddle writes.
	// Complete lines (including comments/keepalives) are already forwarded.
	w.WriteHeaderNow()
	w.ResponseWriter.Flush()
}

func (w *clientModelResponseWriter) finish() error {
	if len(w.pending) == 0 {
		return nil
	}
	body := w.pending
	w.pending = nil
	switch w.format() {
	case "json":
		body = restoreClientResponseModel(body, w.model())
	case "sse":
		body = restoreClientSSEModel(body, w.model())
	}
	_, err := w.ResponseWriter.Write(body)
	return err
}

// Only protocol-owned fields are touched. In particular, never recurse into
// output content, tool arguments, metadata or user-defined JSON schemas.
func restoreClientResponseModel(body []byte, model string) []byte {
	if model == "" || !gjson.ValidBytes(body) {
		return body
	}
	original := body
	paths := []string{"model", "response.model", "message.model", "modelVersion", "response.modelVersion"}
	// Gemini streamGenerateContent without alt=sse returns an array of chunks.
	if root := gjson.ParseBytes(body); root.IsArray() {
		paths = nil
		root.ForEach(func(key, _ gjson.Result) bool {
			prefix := strconv.FormatInt(key.Int(), 10) + "."
			paths = append(paths, prefix+"modelVersion", prefix+"response.modelVersion")
			return true
		})
	}
	for _, path := range paths {
		value := gjson.GetBytes(body, path)
		if value.Type != gjson.String || value.String() == model {
			continue
		}
		updated, err := sjson.SetBytes(body, path, model)
		if err != nil {
			return original
		}
		body = updated
	}
	return body
}

func restoreClientSSEModel(line []byte, model string) []byte {
	if !bytes.HasPrefix(line, []byte("data:")) {
		return line
	}
	// Preserve the original data prefix, whitespace and CRLF/LF framing.
	payload := line[len("data:"):]
	updated := restoreClientResponseModel(payload, model)
	if bytes.Equal(payload, updated) {
		return line
	}
	return append([]byte("data:"), updated...)
}
