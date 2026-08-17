package client

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
)

var (
	marketplaceThreadTitleRe   = regexp.MustCompile(`<title>([^<]+)</title>`)
	marketplaceThreadSummaryRe = regexp.MustCompile(`deleteThenInsertThread",\[19,"(\d+)"\],\[19,"(\d+)"\],"((?:\\.|[^"])*)","((?:\\.|[^"])*)","((?:\\.|[^"])*)",false,\[19,"80"\],\[19,"([^"]+)".*?\[19,"(-?\d+)"\]`)
	marketplaceThreadMessageRe = regexp.MustCompile(`upsertMessage","((?:\\.|[^"])*)",\[9\],\[19,"80"\],\[19,"([^"]+)"\],\[19,"0"\],\[19,"(\d+)"\],\[19,"(\d+)"\],\[9\],"([^"]+)","([^"]+)",\[19,"([^"]+)"\]`)
	marketplaceThreadContactRe = regexp.MustCompile(`verifyContactRowExists",\[19,"([^"]+)"\],\[19,"([^"]+)"\],"((?:\\.|[^"])*)","((?:\\.|[^"])*)",\[19,"(\d+)"\]`)
)

type marketplaceThreadSummary struct {
	UpdatedAt         int64  `json:"updated_at"`
	PreviousUpdatedAt int64  `json:"previous_updated_at"`
	Snippet           string `json:"snippet"`
	Title             string `json:"title"`
	ImageURL          string `json:"image_url,omitempty"`
	FolderCode        int    `json:"folder_code"`
}

type marketplaceThreadMessage struct {
	Text               string `json:"text"`
	TimestampMS        int64  `json:"timestamp_ms"`
	SortKeyMS          int64  `json:"sort_key_ms"`
	MessageID          string `json:"message_id"`
	OfflineThreadingID string `json:"offline_threading_id"`
	SenderID           string `json:"sender_id"`
	SenderName         string `json:"sender_name,omitempty"`
}

type marketplaceThreadContact struct {
	ContactID   string
	ContactType int
	Name        string
	ImageURL    string
}

type marketplaceThreadSnapshot struct {
	Mode          string                     `json:"mode"`
	Route         string                     `json:"route"`
	RouteTitle    string                     `json:"route_title,omitempty"`
	ThreadID      string                     `json:"thread_id"`
	Summary       *marketplaceThreadSummary  `json:"summary,omitempty"`
	MessageCount  int                        `json:"message_count"`
	Messages      []marketplaceThreadMessage `json:"messages"`
	LatestMessage *marketplaceThreadMessage  `json:"latest_message,omitempty"`
}

// MarketplaceThreadSnapshot reads the direct Messenger thread route for a
// known Marketplace thread id. This route exposes the actual buyer-side thread
// payload even when /marketplace/inbox resolves to an unrelated top-level inbox store.
func (c *Client) MarketplaceThreadSnapshot(threadID string) (json.RawMessage, int, error) {
	noCacheBefore := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = noCacheBefore }()

	route := "/messages/t/" + threadID + "/"
	shell, statusCode, err := c.do("GET", route, nil, nil, nil)
	if err != nil {
		return nil, statusCode, err
	}

	contacts := parseMarketplaceThreadContacts(string(shell))
	messages := parseMarketplaceThreadMessages(string(shell), threadID, contacts)
	var latestMessage *marketplaceThreadMessage
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		latestMessage = &last
	}

	snapshot := marketplaceThreadSnapshot{
		Mode:          "messenger_thread_route",
		Route:         route,
		RouteTitle:    parseMarketplaceThreadTitle(string(shell)),
		ThreadID:      threadID,
		Summary:       parseMarketplaceThreadSummary(string(shell), threadID),
		MessageCount:  len(messages),
		Messages:      messages,
		LatestMessage: latestMessage,
	}

	out, err := json.Marshal(snapshot)
	if err != nil {
		return nil, statusCode, err
	}
	return out, statusCode, nil
}

func parseMarketplaceThreadTitle(shell string) string {
	match := marketplaceThreadTitleRe.FindStringSubmatch(shell)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func parseMarketplaceThreadSummary(shell string, threadID string) *marketplaceThreadSummary {
	for _, match := range marketplaceThreadSummaryRe.FindAllStringSubmatch(shell, -1) {
		if len(match) != 8 || match[6] != threadID {
			continue
		}
		return &marketplaceThreadSummary{
			UpdatedAt:         mustParseInt64(match[1]),
			PreviousUpdatedAt: mustParseInt64(match[2]),
			Snippet:           unquoteJSONString(match[3]),
			Title:             unquoteJSONString(match[4]),
			ImageURL:          unquoteJSONString(match[5]),
			FolderCode:        mustParseInt(match[7]),
		}
	}
	return nil
}

func parseMarketplaceThreadMessages(shell string, threadID string, contacts map[string]marketplaceThreadContact) []marketplaceThreadMessage {
	out := make([]marketplaceThreadMessage, 0)
	seen := map[string]struct{}{}
	for _, match := range marketplaceThreadMessageRe.FindAllStringSubmatch(shell, -1) {
		if len(match) != 8 || match[2] != threadID {
			continue
		}
		message := marketplaceThreadMessage{
			Text:               unquoteJSONString(match[1]),
			TimestampMS:        mustParseInt64(match[3]),
			SortKeyMS:          mustParseInt64(match[4]),
			MessageID:          match[5],
			OfflineThreadingID: match[6],
			SenderID:           match[7],
			SenderName:         contacts[match[7]].Name,
		}
		key := fmt.Sprintf("%s:%d", message.MessageID, message.TimestampMS)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, message)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TimestampMS != out[j].TimestampMS {
			return out[i].TimestampMS < out[j].TimestampMS
		}
		return out[i].MessageID < out[j].MessageID
	})
	return out
}

func parseMarketplaceThreadContacts(shell string) map[string]marketplaceThreadContact {
	out := map[string]marketplaceThreadContact{}
	for _, match := range marketplaceThreadContactRe.FindAllStringSubmatch(shell, -1) {
		if len(match) != 6 {
			continue
		}
		out[match[1]] = marketplaceThreadContact{
			ContactID:   match[1],
			ContactType: mustParseInt(match[2]),
			Name:        unquoteJSONString(match[4]),
			ImageURL:    unquoteJSONString(match[3]),
		}
	}
	return out
}

func mustParseInt(raw string) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func mustParseInt64(raw string) int64 {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
