package apicompat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func citationFixture() AnthropicCitation {
	return AnthropicCitation{Type: "web_search_result_location", URL: "https://example.test/a", Title: "来源 A", CitedText: "事实甲"}
}
func TestChatCitationsAnthropicNonStreaming(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-5", "claude-opus-4-5", "claude-opus-4-6", "claude-opus-5-5"} {
		t.Run(model, func(t *testing.T) {
			source := citationFixture()
			sourceB := source
			sourceB.URL = "https://example.test/b"
			sourceB.CitedText = "事实乙"
			response := AnthropicToResponsesResponse(&AnthropicResponse{ID: "fixture", Model: model, Content: []AnthropicContentBlock{{Type: "text", Text: "先搜索。"}, {Type: "server_tool_use", Name: "web_search"}, {Type: "web_search_tool_result"}, {Type: "text", Text: "事实甲与事实乙。", Citations: []AnthropicCitation{source, sourceB}}}, Usage: AnthropicUsage{InputTokens: 10, OutputTokens: 20}})
			chat := ResponsesToChatCompletions(response, model)
			require.JSONEq(t, `"先搜索。\n\n事实甲与事实乙。"`, string(chat.Choices[0].Message.Content))
			annotations := chat.Choices[0].Message.Annotations
			require.Len(t, annotations, 2)
			require.Equal(t, 6, annotations[0].URLCitation.StartIndex)
			require.Equal(t, 9, annotations[0].URLCitation.EndIndex)
			require.Equal(t, 10, annotations[1].URLCitation.StartIndex)
			require.Equal(t, 13, annotations[1].URLCitation.EndIndex)
			require.Equal(t, 10, chat.Usage.PromptTokens)
			require.Equal(t, 20, chat.Usage.CompletionTokens)
			bytes, err := json.Marshal(chat)
			require.NoError(t, err)
			require.Contains(t, string(bytes), `"annotations":[{"type":"url_citation","url_citation":`)
		})
	}
}
func anthropicSearchStream(citations bool) []AnthropicStreamEvent {
	raw := []string{
		`{"type":"message_start","message":{"id":"fixture","model":"claude-sonnet-4-5","usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"先搜索。"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"web_search_tool_result","tool_use_id":"srvtoolu_1","content":[]}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"事实甲"}}`,
	}
	if citations {
		raw = append(raw, `{"type":"content_block_delta","index":3,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://example.test/a","title":"来源 A","cited_text":"事实甲"}}}`)
	}
	raw = append(raw, `{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"与事实乙。"}}`)
	if citations {
		raw = append(raw, `{"type":"content_block_delta","index":3,"delta":{"type":"citations_delta","citation":{"type":"web_search_result_location","url":"https://example.test/b","title":"来源 B","cited_text":"事实乙"}}}`)
	}
	raw = append(raw, `{"type":"content_block_stop","index":3}`, `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":20}}`, `{"type":"message_stop"}`)
	events := make([]AnthropicStreamEvent, len(raw))
	for i, v := range raw {
		if err := json.Unmarshal([]byte(v), &events[i]); err != nil {
			panic(err)
		}
	}
	return events
}
func TestChatCitationsAnthropicStreaming(t *testing.T) {
	for _, withCitations := range []bool{true, false} {
		ast := NewAnthropicEventToResponsesState()
		cst := NewResponsesEventToChatState()
		cst.IncludeUsage = true
		var text string
		var annotations []ChatAnnotation
		var completed *ResponsesResponse
		var annotationsEvents int
		for _, event := range anthropicSearchStream(withCitations) {
			for _, res := range AnthropicEventToResponsesEvents(&event, ast) {
				data, err := json.Marshal(res)
				require.NoError(t, err)
				// Exercise the actual custom wire marshaler, not only in-memory structs.
				var wire ResponsesStreamEvent
				require.NoError(t, json.Unmarshal(data, &wire))
				if wire.Type == "response.output_text.annotation.added" {
					annotationsEvents++
					require.Contains(t, string(data), `"annotation_index":`)
					require.Contains(t, string(data), `"content_index":3`)
				}
				if wire.Type == "response.completed" {
					completed = wire.Response
				}
				for _, chunk := range ResponsesEventToChatChunks(&wire, cst) {
					if len(chunk.Choices) > 0 {
						delta := chunk.Choices[0].Delta
						if delta.Content != nil {
							text += *delta.Content
						}
						annotations = append(annotations, delta.Annotations...)
					}
				}
			}
		}
		require.NotNil(t, completed)
		require.Equal(t, 10, completed.Usage.InputTokens)
		require.Equal(t, 20, completed.Usage.OutputTokens)
		if withCitations {
			require.Equal(t, "先搜索。\n\n事实甲与事实乙。", text)
			require.Len(t, annotations, 2)
			require.Equal(t, 2, annotationsEvents)
			require.Equal(t, 6, annotations[0].URLCitation.StartIndex)
			require.Equal(t, 9, annotations[0].URLCitation.EndIndex)
			require.Equal(t, 10, annotations[1].URLCitation.StartIndex)
			nonstream := ResponsesToChatCompletions(completed, "claude-sonnet-4-5")
			require.Equal(t, annotations, nonstream.Choices[0].Message.Annotations)
			// output_item.done and terminal output must contain annotations as well.
			require.Len(t, completed.Output[0].Content[3].Annotations, 2)
		} else {
			require.Equal(t, "先搜索。事实甲与事实乙。", text)
			require.Empty(t, annotations)
		}
	}
}
func TestChatCitationsResponsesMappingAndDedup(t *testing.T) {
	a := ResponsesAnnotation{Type: "url_citation", URL: "https://example.test", Title: "title", StartIndex: 0, EndIndex: 2}
	part := ResponsesContentPart{Type: "output_text", Text: "你好", Annotations: []ResponsesAnnotation{a}}
	item := ResponsesOutput{Type: "message", Content: []ResponsesContentPart{part}}
	state := NewResponsesEventToChatState()
	acc := NewBufferedResponseAccumulator()
	events := []ResponsesStreamEvent{{Type: "response.output_text.delta", Delta: "前"}, {Type: "response.output_text.delta", OutputIndex: 1, Delta: "你好"}, {Type: "response.output_text.annotation.added", OutputIndex: 1, Annotation: &a}, {Type: "response.output_item.done", OutputIndex: 1, Item: &item}, {Type: "response.completed", Response: &ResponsesResponse{Output: []ResponsesOutput{{Type: "message", Content: []ResponsesContentPart{{Type: "output_text", Text: "前"}}}, item}}}}
	var annotations []ChatAnnotation
	for i := range events {
		acc.ProcessEvent(&events[i])
		for _, chunk := range ResponsesEventToChatChunks(&events[i], state) {
			for _, choice := range chunk.Choices {
				annotations = append(annotations, choice.Delta.Annotations...)
			}
		}
	}
	require.Len(t, annotations, 1)
	require.Equal(t, 1, annotations[0].URLCitation.StartIndex)
	require.Equal(t, 3, annotations[0].URLCitation.EndIndex)
	buffered := &ResponsesResponse{}
	acc.SupplementResponseOutput(buffered)
	chat := ResponsesToChatCompletions(buffered, "model")
	require.JSONEq(t, `"前你好"`, string(chat.Choices[0].Message.Content))
	require.Equal(t, annotations, chat.Choices[0].Message.Annotations)
	// Some upstreams only supply annotations on output_item.done.
	state = NewResponsesEventToChatState()
	ResponsesEventToChatChunks(&events[0], state)
	ResponsesEventToChatChunks(&events[1], state)
	chunks := ResponsesEventToChatChunks(&events[3], state)
	require.Len(t, chunks, 1)
	require.Equal(t, annotations, chunks[0].Choices[0].Delta.Annotations)
	// Do not add empty annotations or document citations unsupported by URL schema.
	require.Nil(t, mapAnthropicCitation(AnthropicCitation{Type: "char_location"}, "x"))
	data, err := json.Marshal(ResponsesToChatCompletions(&ResponsesResponse{Output: []ResponsesOutput{{Type: "message", Content: []ResponsesContentPart{{Type: "output_text", Text: "plain"}}}}}, "model"))
	require.NoError(t, err)
	require.False(t, strings.Contains(string(data), "annotations"))
}

func TestChatCitationsResponsesSearchParagraphs(t *testing.T) {
	for _, withCitations := range []bool{false, true} {
		ann := ResponsesAnnotation{Type: "url_citation", URL: "https://example.test", Title: "source", StartIndex: 0, EndIndex: 2}
		post := ResponsesOutput{Type: "message", Content: []ResponsesContentPart{{Type: "output_text", Text: "回答"}}}
		if withCitations {
			post.Content[0].Annotations = []ResponsesAnnotation{ann}
		}
		output := []ResponsesOutput{{Type: "message", Content: []ResponsesContentPart{{Type: "output_text", Text: "先搜索"}}}, {Type: "web_search_call"}, post}
		want := "先搜索回答"
		if withCitations {
			want = "先搜索\n\n回答"
		}
		nonstream := ResponsesToChatCompletions(&ResponsesResponse{Output: output}, "model")
		var got string
		require.NoError(t, json.Unmarshal(nonstream.Choices[0].Message.Content, &got))
		require.Equal(t, want, got)
		events := []ResponsesStreamEvent{{Type: "response.output_text.delta", Delta: "先搜索"}, {Type: "response.output_item.added", OutputIndex: 1, Item: &output[1]}, {Type: "response.output_text.delta", OutputIndex: 2, Delta: "回答"}}
		if withCitations {
			events = append(events, ResponsesStreamEvent{Type: "response.output_text.annotation.added", OutputIndex: 2, Annotation: &ann})
		}
		events = append(events, ResponsesStreamEvent{Type: "response.output_item.done", OutputIndex: 2, Item: &post}, ResponsesStreamEvent{Type: "response.completed", Response: &ResponsesResponse{Output: output}})
		state := NewResponsesEventToChatState()
		acc := NewBufferedResponseAccumulator()
		text := ""
		var anns []ChatAnnotation
		for _, e := range events {
			acc.ProcessEvent(&e)
			for _, c := range ResponsesEventToChatChunks(&e, state) {
				for _, choice := range c.Choices {
					if choice.Delta.Content != nil {
						text += *choice.Delta.Content
					}
					anns = append(anns, choice.Delta.Annotations...)
				}
			}
		}
		require.Equal(t, want, text)
		require.Equal(t, nonstream.Choices[0].Message.Annotations, anns)
		terminal := &ResponsesResponse{}
		acc.SupplementResponseOutput(terminal)
		buffered := ResponsesToChatCompletions(terminal, "model")
		require.Equal(t, nonstream.Choices[0].Message.Content, buffered.Choices[0].Message.Content)
		require.Equal(t, anns, buffered.Choices[0].Message.Annotations)
		if withCitations {
			require.Equal(t, 5, anns[0].URLCitation.StartIndex)
			require.Equal(t, 7, anns[0].URLCitation.EndIndex)
		}
	}
}

func TestChatCitationsDoNotLeakAcrossSearchBlocks(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	chatState := NewResponsesEventToChatState()
	source := citationFixture()
	source.CitedText = "a"
	var got []ChatAnnotation
	events := []AnthropicStreamEvent{
		{Type: "message_start", Message: &AnthropicResponse{ID: "test", Model: "claude-sonnet-4-5"}},
		{Type: "content_block_start", ContentBlock: &AnthropicContentBlock{Type: "text"}},
		{Type: "content_block_delta", Delta: &AnthropicDelta{Type: "text_delta", Text: "a"}},
		{Type: "content_block_delta", Delta: &AnthropicDelta{Type: "citations_delta", Citation: &source}},
		{Type: "content_block_stop"},
		{Type: "content_block_start", ContentBlock: &AnthropicContentBlock{Type: "server_tool_use", Name: "web_search"}},
		{Type: "content_block_stop"},
		{Type: "content_block_start", ContentBlock: &AnthropicContentBlock{Type: "web_search_tool_result"}},
		{Type: "content_block_stop"},
		{Type: "message_stop"},
	}
	for _, event := range events {
		for _, res := range AnthropicEventToResponsesEvents(&event, state) {
			for _, chunk := range ResponsesEventToChatChunks(&res, chatState) {
				for _, choice := range chunk.Choices {
					got = append(got, choice.Delta.Annotations...)
				}
			}
		}
	}
	require.Len(t, got, 1)
}
