// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Ask sends a chat query and returns the answer with citations.
func (c *Client) Ask(ctx context.Context, notebookID, question string, sourceIDs []string) (ChatResult, error) {
	if strings.TrimSpace(question) == "" {
		return ChatResult{}, fmt.Errorf("question is required")
	}
	if len(sourceIDs) == 0 {
		ids, err := c.sourceIDs(ctx, notebookID)
		if err != nil {
			return ChatResult{}, err
		}
		sourceIDs = ids
	}
	u, body, err := c.buildChatRequest(notebookID, question, sourceIDs, nil)
	if err != nil {
		return ChatResult{}, err
	}
	respBody, err := c.Session.postForm(ctx, u, body, http.Header{
		"User-Agent": {chromeUserAgent},
		"Origin":     {BaseURL},
		"Referer":    {BaseURL + notebookPath(notebookID)},
	})
	if err != nil {
		return ChatResult{}, err
	}
	answer, citations, err := parseStreamingChat(string(respBody))
	if err != nil {
		return ChatResult{}, err
	}
	return ChatResult{Answer: answer, Citations: citations, NotebookID: notebookID}, nil
}

func (c *Client) sourceIDs(ctx context.Context, notebookID string) ([]string, error) {
	sources, err := c.ListSources(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("notebook has no sources")
	}
	ids := make([]string, len(sources))
	for i, s := range sources {
		ids[i] = s.ID
	}
	return ids, nil
}

func (c *Client) buildChatRequest(notebookID, question string, sourceIDs []string, conversationID *string) (string, string, error) {
	params := []any{
		NestSourceIDs(sourceIDs, 2),
		question,
		nil,
		[]any{2, nil, []any{1}, []any{1}},
		conversationID,
		nil,
		nil,
		notebookID,
		1,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return "", "", err
	}
	freqJSON, err := json.Marshal([]any{nil, string(paramsJSON)})
	if err != nil {
		return "", "", err
	}
	body := "f.req=" + url.QueryEscape(string(freqJSON))
	if c.Session.AT != "" {
		body += "&at=" + url.QueryEscape(c.Session.AT)
	}
	body += "&"
	q := url.Values{}
	if c.Session.BL != "" {
		q.Set("bl", c.Session.BL)
	}
	q.Set("hl", "en")
	q.Set("_reqid", c.Session.nextReqID())
	q.Set("rt", "c")
	if c.Session.SID != "" {
		q.Set("f.sid", c.Session.SID)
	}
	q.Set("authuser", "0")
	u := BaseURL + QueryEndpointPath + "?" + q.Encode()
	return u, body, nil
}

func parseStreamingChat(body string) (string, []ChatCitation, error) {
	body = StripXSSIPrefix(body)
	lines := strings.Split(body, "\n")
	var bestAnswer string
	var citations []ChatCitation
	parseable := 0
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		i++
		if line == "" {
			continue
		}
		if _, err := parseIntLine(line); err == nil {
			if i < len(lines) {
				processChatChunk(lines[i], &bestAnswer, &citations, &parseable)
			}
			i++
			continue
		}
		processChatChunk(line, &bestAnswer, &citations, &parseable)
	}
	if parseable == 0 {
		return "", nil, fmt.Errorf("no parseable chat response chunks")
	}
	return bestAnswer, citations, nil
}

func parseIntLine(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func processChatChunk(line string, best *string, citations *[]ChatCitation, parseable *int) {
	var outer []any
	if err := json.Unmarshal([]byte(line), &outer); err != nil {
		return
	}
	for _, el := range outer {
		row, ok := el.([]any)
		if !ok || len(row) < 3 {
			continue
		}
		tag, _ := row[0].(string)
		if tag != "wrb.fr" {
			continue
		}
		inner, _ := row[2].(string)
		if inner == "" {
			continue
		}
		*parseable++
		ans, cites := extractChatInner(inner)
		if len(ans) > len(*best) {
			*best = ans
			if len(cites) > 0 {
				*citations = cites
			}
		}
	}
}

func extractChatInner(innerJSON string) (string, []ChatCitation) {
	var data any
	if err := json.Unmarshal([]byte(innerJSON), &data); err != nil {
		return "", nil
	}
	answer := findLongestString(data)
	citations := extractCitations(data)
	return answer, citations
}

func findLongestString(v any) string {
	var best string
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case string:
			if len(t) > len(best) && len(t) > 20 && !strings.HasPrefix(t, "[[") {
				best = t
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return best
}

func extractCitations(v any) []ChatCitation {
	var out []ChatCitation
	var walk func(any, int)
	walk = func(node any, depth int) {
		row, ok := node.([]any)
		if !ok {
			return
		}
		if len(row) >= 2 {
			if id := extractUUIDFromNested(row); id != "" {
				cite := ChatCitation{SourceID: id, Number: len(out) + 1}
				if text := findCitationText(row); text != "" {
					cite.CitedText = text
				}
				out = append(out, cite)
				return
			}
		}
		if depth > 12 {
			return
		}
		for _, item := range row {
			walk(item, depth+1)
		}
	}
	walk(v, 0)
	return out
}

func extractUUIDFromNested(v any) string {
	switch t := v.(type) {
	case string:
		if looksLikeUUID(t) {
			return t
		}
	case []any:
		for _, item := range t {
			if id := extractUUIDFromNested(item); id != "" {
				return id
			}
		}
	}
	return ""
}

func findCitationText(row []any) string {
	for _, item := range row {
		if s, ok := item.(string); ok && len(s) > 10 && len(s) < 500 {
			return s
		}
	}
	return ""
}

// ListConversationTurns returns chat history for the latest conversation.
func (c *Client) ListConversationTurns(ctx context.Context, notebookID string) ([]ConversationTurn, error) {
	convRaw, err := c.Call(ctx, RPCGetLastConversationID, notebookPath(notebookID), BuildLastConversationParams(notebookID))
	if err != nil {
		if strings.Contains(err.Error(), "null result") {
			return nil, nil
		}
		return nil, err
	}
	convID := extractConversationID(convRaw)
	if convID == "" {
		return nil, nil
	}
	raw, err := c.Call(ctx, RPCGetConversationTurns, notebookPath(notebookID), BuildConversationTurnsParams(notebookID, convID))
	if err != nil {
		return nil, err
	}
	return parseConversationTurns(raw)
}

func extractConversationID(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if row, ok := v.([]any); ok && len(row) > 0 {
		if s, ok := row[0].(string); ok {
			return s
		}
	}
	return findUUIDString(v)
}

func findUUIDString(v any) string {
	switch t := v.(type) {
	case string:
		if looksLikeUUID(t) {
			return t
		}
	case []any:
		for _, item := range t {
			if id := findUUIDString(item); id != "" {
				return id
			}
		}
	}
	return ""
}

func parseConversationTurns(raw json.RawMessage) ([]ConversationTurn, error) {
	var outer []any
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, err
	}
	var turns []ConversationTurn
	var walk func(any)
	walk = func(node any) {
		row, ok := node.([]any)
		if !ok {
			return
		}
		if len(row) >= 2 {
			q, _ := row[0].(string)
			a, _ := row[1].(string)
			if q != "" && a != "" && len(q) > 3 {
				turns = append(turns, ConversationTurn{Question: q, Answer: a})
				return
			}
		}
		for _, item := range row {
			walk(item)
		}
	}
	for _, chunk := range outer {
		walk(chunk)
	}
	return turns, nil
}
