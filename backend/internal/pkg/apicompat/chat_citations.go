package apicompat

// CUSTOM: additive native web-search citation mapping. Responses offsets refer
// to their output_text part; Chat offsets refer to the flattened message. Count
// Unicode characters (not UTF-8 bytes), including injected paragraph separators.
import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type AnthropicCitation struct {
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	CitedText string `json:"cited_text,omitempty"`
}
type ResponsesAnnotation struct {
	Type       string `json:"type"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}
type ChatURLCitation struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}
type ChatAnnotation struct {
	Type        string          `json:"type"`
	URLCitation ChatURLCitation `json:"url_citation"`
}

func citationTextLength(s string) int { return utf8.RuneCountInString(s) }
func mapAnthropicCitation(c AnthropicCitation, text string) *ResponsesAnnotation {
	if c.Type != "web_search_result_location" || c.URL == "" {
		return nil
	}
	start, end := 0, citationTextLength(text)
	if c.CitedText != "" {
		if idx := strings.Index(text, c.CitedText); idx >= 0 {
			start = citationTextLength(text[:idx])
			end = start + citationTextLength(c.CitedText)
		}
	}
	return &ResponsesAnnotation{Type: "url_citation", URL: c.URL, Title: c.Title, StartIndex: start, EndIndex: end}
}
func chatAnnotations(in []ResponsesAnnotation, offset int) []ChatAnnotation {
	var out []ChatAnnotation
	for _, a := range in {
		if a.Type == "url_citation" && a.URL != "" {
			out = append(out, ChatAnnotation{"url_citation", ChatURLCitation{a.URL, a.Title, a.StartIndex + offset, a.EndIndex + offset}})
		}
	}
	return out
}

type anthropicCitationResponseState struct{ enabled, search, sawText bool }

func newAnthropicCitationResponseState(blocks []AnthropicContentBlock) anthropicCitationResponseState {
	state := anthropicCitationResponseState{}
	for _, b := range blocks {
		for _, c := range b.Citations {
			if c.Type == "web_search_result_location" && c.URL != "" {
				state.enabled = true
			}
		}
	}
	return state
}
func (s *anthropicCitationResponseState) textPart(b AnthropicContentBlock) ResponsesContentPart {
	text := b.Text
	if text != "" && s.enabled && s.search && s.sawText {
		text = "\n\n" + text
	}
	if text != "" {
		s.sawText = true
		s.search = false
	}
	part := ResponsesContentPart{Type: "output_text", Text: text}
	for _, c := range b.Citations {
		if a := mapAnthropicCitation(c, text); a != nil {
			part.Annotations = append(part.Annotations, *a)
		}
	}
	return part
}

type anthropicCitationStreamState struct {
	search, sawText, pendingBreak bool
	pending                       []string
	annotations                   []ResponsesAnnotation
}

func observeAnthropicSearchEvent(e *AnthropicStreamEvent, s *AnthropicEventToResponsesState) {
	if e.Type == "content_block_start" && e.ContentBlock != nil && (e.ContentBlock.Type == "web_search_tool_result" || (e.ContentBlock.Type == "server_tool_use" && e.ContentBlock.Name == "web_search")) {
		s.citations.search = true
	}
	// Keep legacy events (including empty parts) unchanged when citations are absent.
}
func anthropicCitationTextDelta(s *AnthropicEventToResponsesState, text string) []ResponsesStreamEvent {
	if text == "" {
		return nil
	}
	if s.citations.pendingBreak {
		s.citations.pending = append(s.citations.pending, text)
		return nil
	}
	s.citations.sawText = true
	s.TextAccum += text
	return []ResponsesStreamEvent{makeResponsesEvent(s, "response.output_text.delta", &ResponsesStreamEvent{OutputIndex: s.OutputIndex, ContentIndex: s.ContentIndex, ItemID: s.CurrentItemID, Delta: text})}
}
func flushAnthropicCitationText(s *AnthropicEventToResponsesState, separator bool) []ResponsesStreamEvent {
	if !s.citations.pendingBreak {
		return nil
	}
	s.citations.pendingBreak = false
	var out []ResponsesStreamEvent
	if separator {
		out = append(out, anthropicCitationTextDelta(s, "\n\n")...)
	}
	for _, text := range s.citations.pending {
		out = append(out, anthropicCitationTextDelta(s, text)...)
	}
	s.citations.pending = nil
	return out
}
func anthropicCitationDelta(s *AnthropicEventToResponsesState, c *AnthropicCitation) []ResponsesStreamEvent {
	if c == nil || c.Type != "web_search_result_location" || c.URL == "" || s.CurrentItemType != "message" {
		return nil
	}
	out := flushAnthropicCitationText(s, true)
	a := mapAnthropicCitation(*c, s.TextAccum)
	idx := len(s.citations.annotations)
	s.citations.annotations = append(s.citations.annotations, *a)
	return append(out, makeResponsesEvent(s, "response.output_text.annotation.added", &ResponsesStreamEvent{OutputIndex: s.OutputIndex, ContentIndex: s.ContentIndex, ItemID: s.CurrentItemID, AnnotationIndex: idx, Annotation: a}))
}

type citationPartKey struct{ output, content int }
type responsesCitationTracker struct {
	total   int
	offsets map[citationPartKey]int
	seen    map[string]bool
}

func (s *responsesCitationTracker) process(e *ResponsesStreamEvent) []ChatAnnotation {
	if s.offsets == nil {
		s.offsets = make(map[citationPartKey]int)
		s.seen = make(map[string]bool)
	}
	key := citationPartKey{e.OutputIndex, e.ContentIndex}
	switch e.Type {
	case "response.output_text.delta":
		if _, ok := s.offsets[key]; !ok {
			s.offsets[key] = s.total
		}
		s.total += citationTextLength(e.Delta)
	case "response.output_text.annotation.added":
		if e.Annotation != nil {
			return s.add(key, e.AnnotationIndex, *e.Annotation)
		}
	case "response.output_item.done":
		if e.Item != nil {
			return s.item(e.OutputIndex, e.Item)
		}
	case "response.completed", "response.done", "response.incomplete", "response.failed":
		var out []ChatAnnotation
		if e.Response != nil {
			offset := 0
			for i := range e.Response.Output {
				item := &e.Response.Output[i]
				for j, part := range item.Content {
					key := citationPartKey{i, j}
					if _, ok := s.offsets[key]; !ok {
						s.offsets[key] = offset
					}
					if part.Type == "output_text" {
						offset += citationTextLength(part.Text)
					}
				}
				out = append(out, s.item(i, item)...)
			}
		}
		return out
	}
	return nil
}
func (s *responsesCitationTracker) add(key citationPartKey, index int, a ResponsesAnnotation) []ChatAnnotation {
	if a.Type != "url_citation" || a.URL == "" {
		return nil
	}
	id := fmt.Sprintf("%d:%d:%d", key.output, key.content, index)
	if s.seen[id] {
		return nil
	}
	s.seen[id] = true
	offset, ok := s.offsets[key]
	if !ok {
		offset = s.total
		s.offsets[key] = offset
	}
	return chatAnnotations([]ResponsesAnnotation{a}, offset)
}
func (s *responsesCitationTracker) item(output int, item *ResponsesOutput) []ChatAnnotation {
	var out []ChatAnnotation
	offset := s.total
	for i, part := range item.Content {
		key := citationPartKey{output, i}
		if known, ok := s.offsets[key]; ok {
			offset = known
		} else {
			s.offsets[key] = offset
		}
		for j, a := range part.Annotations {
			out = append(out, s.add(citationPartKey{output, i}, j, a)...)
		}
		if part.Type == "output_text" {
			offset += citationTextLength(part.Text)
		}
	}
	return out
}
func (a *BufferedResponseAccumulator) supplementCitations(resp *ResponsesResponse) {
	if len(a.annotations) == 0 {
		return
	}
	for _, item := range resp.Output {
		for _, p := range item.Content {
			if len(p.Annotations) > 0 {
				return
			}
		}
	}
	// Accumulated annotations use flattened offsets. Rebase onto terminal parts.
	offset := 0
	for i := range resp.Output {
		for j := range resp.Output[i].Content {
			part := &resp.Output[i].Content[j]
			if part.Type != "output_text" {
				continue
			}
			end := offset + citationTextLength(part.Text)
			for _, ann := range a.annotations {
				if ann.StartIndex >= offset && ann.EndIndex <= end {
					ann.StartIndex -= offset
					ann.EndIndex -= offset
					part.Annotations = append(part.Annotations, ann)
				}
			}
			offset = end
		}
	}
}

func responsesHaveURLCitations(output []ResponsesOutput) bool {
	for _, item := range output {
		for _, part := range item.Content {
			for _, a := range part.Annotations {
				if a.Type == "url_citation" && a.URL != "" {
					return true
				}
			}
		}
	}
	return false
}

// Wait for the first citation after a web_search_call before adding a paragraph.
// Without citations the original event sequence and text are emitted unchanged.
// The synthetic separator uses a distinct part key: native annotation offsets
// remain local to the original part, while Chat's cumulative offsets include it.
type responsesChatSearchState struct {
	boundary bool
	pending  []ResponsesStreamEvent
}

func (s *responsesChatSearchState) events(e ResponsesStreamEvent, hasText bool) []ResponsesStreamEvent {
	if (e.Type == "response.output_item.added" || e.Type == "response.output_item.done") && e.Item != nil && e.Item.Type == "web_search_call" {
		s.boundary = true
	}
	if len(s.pending) == 0 {
		if e.Type != "response.output_text.delta" || !s.boundary || !hasText || e.Delta == "" {
			if e.Type == "response.output_text.delta" && e.Delta != "" {
				s.boundary = false
			}
			return []ResponsesStreamEvent{e}
		}
		s.boundary = false
	}
	s.pending = append(s.pending, e)
	hasCitation := e.Type == "response.output_text.annotation.added" && e.Annotation != nil && e.Annotation.Type == "url_citation" && e.Annotation.URL != ""
	if e.Item != nil {
		hasCitation = hasCitation || responsesHaveURLCitations([]ResponsesOutput{*e.Item})
	}
	if e.Response != nil {
		hasCitation = hasCitation || responsesHaveURLCitations(e.Response.Output)
	}
	if hasCitation {
		return s.flush(true)
	}
	switch e.Type {
	case "response.output_item.done", "response.completed", "response.done", "response.incomplete", "response.failed":
		return s.flush(false)
	}
	return nil
}
func (s *responsesChatSearchState) flush(separator bool) []ResponsesStreamEvent {
	if len(s.pending) == 0 {
		return nil
	}
	out := s.pending
	s.pending = nil
	if separator {
		prefix := ""
		for _, e := range out {
			if e.Type == "response.output_text.delta" {
				prefix += e.Delta
				if len(prefix) >= 2 {
					break
				}
			}
		}
		if !strings.HasPrefix(prefix, "\n\n") {
			out = append([]ResponsesStreamEvent{{Type: "response.output_text.delta", OutputIndex: -1, Delta: "\n\n"}}, out...)
		}
	}
	return out
}

// AccumulateAnthropicCitation is the additive hook for SSE-to-JSON aggregators.
// Those paths do not run the streaming converter, so citations_delta must be
// retained on the source block before AnthropicToResponsesResponse is called.
func AccumulateAnthropicCitation(response *AnthropicResponse, event *AnthropicStreamEvent) {
	if response == nil || event == nil || event.Type != "content_block_delta" || event.Index == nil || event.Delta == nil || event.Delta.Type != "citations_delta" || event.Delta.Citation == nil {
		return
	}
	index := *event.Index
	if index < 0 || index >= len(response.Content) || response.Content[index].Type != "text" {
		return
	}
	response.Content[index].Citations = append(response.Content[index].Citations, *event.Delta.Citation)
}
