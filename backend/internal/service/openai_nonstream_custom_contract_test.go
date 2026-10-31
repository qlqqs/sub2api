package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAINonStreamTerminalPreservesCustomContracts(t *testing.T) {
	for _, status := range []string{"completed", "incomplete", "cancelled"} {
		for _, passthrough := range []bool{false, true} {
			name := status + "/legacy"
			if passthrough {
				name = status + "/passthrough"
			}
			t.Run(name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				cfg := &config.Config{}
				s := &OpenAIGatewayService{cfg: cfg, responseHeaderFilter: compileResponseHeaderFilter(cfg)}
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
				body := []byte("event: response." + status + "\ndata: " + `{"type":"response.` + status + `","response":{"id":"resp-contract","object":"response","status":"` + status + `","model":"upstream-model","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Source","annotations":[{"type":"url_citation","url":"https://example.com/source","title":"Source","start_index":0,"end_index":6}]}]},{"type":"image_generation_call","id":"img-1","status":"completed","result":"aW1hZ2U=","size":"1024x1024"}],"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}` + "\n\n")
				// The image completes before the enclosing response terminates.
				imageDone := "event: response.output_item.done\ndata: " + `{"type":"response.output_item.done","item":{"type":"image_generation_call","id":"img-1","status":"completed","result":"aW1hZ2U=","size":"1024x1024"}}` + "\n\n"
				body = append([]byte(imageDone), body...)
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
				var usage *OpenAIUsage
				var count int
				var sizes []string
				if passthrough {
					result, err := s.handlePassthroughSSEToJSON(resp, c, account, body, "client-alias", "upstream-model")
					require.NoError(t, err)
					usage, count, sizes = result.usage, result.imageCount, result.imageOutputSizes
				} else {
					result, err := s.handleSSEToJSON(resp, c, account, body, "client-alias", "upstream-model")
					require.NoError(t, err)
					usage, count, sizes = result.usage, result.imageCount, result.imageOutputSizes
				}
				require.Equal(t, http.StatusOK, rec.Code)
				require.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json"))
				require.True(t, gjson.ValidBytes(rec.Body.Bytes()))
				payload := gjson.ParseBytes(rec.Body.Bytes())
				require.Equal(t, status, payload.Get("status").String())
				require.Equal(t, "client-alias", payload.Get("model").String())
				require.Equal(t, "https://example.com/source", payload.Get("output.0.content.0.annotations.0.url").String())
				require.Equal(t, "aW1hZ2U=", payload.Get("output.1.result").String())
				require.Equal(t, 11, usage.InputTokens)
				require.Equal(t, 7, usage.OutputTokens)
				require.Equal(t, 1, count)
				require.Equal(t, []string{"1024x1024"}, sizes)
			})
		}
	}
}
