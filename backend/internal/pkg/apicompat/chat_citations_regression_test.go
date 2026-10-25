package apicompat

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Golden captured from the pre-citation converters, not generated from new
// behavior. Only random item IDs / wall-clock timestamps are normalized.
func TestChatCitationsAbsentWireUnchanged(t *testing.T) {
	cases := []string{
		`{"id":"msg_fixture","model":"claude-sonnet-4-5","stop_reason":"end_turn","content":[{"type":"text","text":"你好 "},{"type":"text","text":"world"}],"usage":{"input_tokens":10,"output_tokens":5}}`,
		`{"id":"msg_fixture","model":"claude-opus-4-5","stop_reason":"tool_use","content":[{"type":"thinking","thinking":"reason"},{"type":"text","text":"Before tool"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"test"}}],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":3}}`,
	}
	var wire []string
	ids := regexp.MustCompile(`item_[a-f0-9]+`)
	record := func(v any) {
		data, err := json.Marshal(v)
		require.NoError(t, err)
		wire = append(wire, ids.ReplaceAllString(string(data), "item_fixture"))
	}
	for _, input := range cases {
		var a AnthropicResponse
		require.NoError(t, json.Unmarshal([]byte(input), &a))
		res := AnthropicToResponsesResponse(&a)
		res.CreatedAt = 1
		record(res)
		chat := ResponsesToChatCompletions(res, "client-model")
		chat.Created = 1
		record(chat)
	}
	astate := NewAnthropicEventToResponsesState()
	astate.Created = 1
	cstate := NewResponsesEventToChatState()
	cstate.Created = 1
	cstate.ID = "chat_fixture"
	cstate.IncludeUsage = true
	events := []string{
		`{"type":"message_start","message":{"id":"msg_fixture","model":"claude-sonnet-4-5","usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好 "}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"!"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	}
	for _, raw := range events {
		var e AnthropicStreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &e))
		for _, r := range AnthropicEventToResponsesEvents(&e, astate) {
			record(r)
			for _, c := range ResponsesEventToChatChunks(&r, cstate) {
				record(c)
			}
		}
	}

	// Search without citations must also retain the exact old serialized events.
	ast2 := NewAnthropicEventToResponsesState()
	ast2.Created = 1
	cst2 := NewResponsesEventToChatState()
	cst2.Created = 1
	cst2.ID = "chat_fixture"
	cst2.IncludeUsage = true
	searchEvents := []string{
		`{"type":"message_start","message":{"id":"msg_fixture","model":"claude-sonnet-4-5","usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Searching."}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"server_tool_use","name":"web_search","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"web_search_tool_result"}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"No citations"}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":" here."}}`,
		`{"type":"content_block_stop","index":3}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	}
	for _, raw := range searchEvents {
		var e AnthropicStreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &e))
		for _, r := range AnthropicEventToResponsesEvents(&e, ast2) {
			record(r)
			for _, c := range ResponsesEventToChatChunks(&r, cst2) {
				record(c)
			}
		}
	}
	got := strings.Join(wire, "\n") + "\n"
	const file = "testdata/chat_no_citations.golden"

	want, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, string(want), got)
}
