//go:build unit

package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func chatCitationGatewaySSE(t *testing.T) string {
	t.Helper()
	events := []string{
		`{"type":"message_start","message":{"id":"search","model":"claude-sonnet-4-5","content":[],"usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"先搜索。"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"server_tool_use","name":"web_search","id":"search1","input":{}}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"web_search_tool_result","tool_use_id":"search1","content":[]}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"事实甲。"}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://example.test/a","title":"来源甲","cited_text":"事实甲"}}}`,
		`{"type":"content_block_stop","index":3}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}`,
		`{"type":"message_stop"}`,
	}
	out := ""
	for _, raw := range events {
		var event struct {
			Type string `json:"type"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &event))
		out += "event: " + event.Type + "\ndata: " + raw + "\n\n"
	}
	return out
}
func TestChatGatewayBufferedCitations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		run  func(*http.Response, *gin.Context) error
	}{
		{"gateway chat", func(r *http.Response, c *gin.Context) error {
			result, err := (&GatewayService{}).handleCCBufferedFromAnthropic(r, c, "model", "claude-sonnet-4-5", nil, time.Now())
			if err == nil {
				require.Equal(t, 10, result.Usage.InputTokens)
				require.Equal(t, 20, result.Usage.OutputTokens)
			}
			return err
		}},
		{"gateway responses", func(r *http.Response, c *gin.Context) error {
			_, err := (&GatewayService{}).handleResponsesBufferedStreamingResponse(r, c, "model", "claude-sonnet-4-5", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return err
		}},
		{"openai native chat", func(r *http.Response, c *gin.Context) error {
			_, err := newNativeAnthropicHangTestService(5).handleCCBufferedFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now())
			return err
		}},
		{"openai native responses", func(r *http.Response, c *gin.Context) error {
			_, err := newNativeAnthropicHangTestService(5).handleResponsesBufferedFromNativeAnthropic(r, c, "model", "model", "model", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
			resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(strings.NewReader(chatCitationGatewaySSE(t)))}
			defer func() { require.NoError(t, resp.Body.Close()) }()
			require.NoError(t, tc.run(resp, c))
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), `"annotations":[{"type":"url_citation"`)
			require.Contains(t, rec.Body.String(), "https://example.test/a")
			if strings.Contains(tc.name, "chat") {
				var out apicompat.ChatCompletionsResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.JSONEq(t, `"先搜索。\n\n事实甲。"`, string(out.Choices[0].Message.Content))
				require.Equal(t, 6, out.Choices[0].Message.Annotations[0].URLCitation.StartIndex)
			}
		})
	}
}
func TestChatGatewayStreamingCitations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	resp := &http.Response{Header: make(http.Header), Body: io.NopCloser(strings.NewReader(chatCitationGatewaySSE(t)))}
	defer func() { require.NoError(t, resp.Body.Close()) }()
	result, err := (&GatewayService{}).handleCCStreamingFromAnthropic(resp, c, "model", "claude-sonnet-4-5", nil, time.Now())
	require.NoError(t, err)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 20, result.Usage.OutputTokens)
	require.Contains(t, rec.Body.String(), `"annotations":[{"type":"url_citation","url_citation":`)
	text := ""
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "data: {") {
			var chunk apicompat.ChatCompletionsChunk
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk))
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != nil {
					text += *choice.Delta.Content
				}
			}
		}
	}
	require.Equal(t, "先搜索。\n\n事实甲。", text)
}
