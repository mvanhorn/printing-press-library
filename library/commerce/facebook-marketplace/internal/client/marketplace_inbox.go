package client

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
)

var (
	marketplaceInboxPreloaderRe = regexp.MustCompile(`"preloaderID":"([^"]*LSPlatformGraphQLLightspeedRequestQueryRelayPreloader_inbox[^"]*)","queryID":"(\d+)","variables":(\{.*?\}),"queryName":"LSPlatformGraphQLLightspeedRequestQuery"`)
	inboxThreadRe               = regexp.MustCompile(`deleteThenInsertThread",\[19,"([^"]+)"\],\[19,"[^"]+"\],"((?:\\.|[^"])*)",\[9\],"((?:\\.|[^"])*)",\[9\],\[19,"([^"]+)"\],\[19,"([^"]+)"\],\[19,"([^"]+)"\],\[19,"([^"]+)"\],"((?:\\.|[^"])*)"`)
	inboxContactRe              = regexp.MustCompile(`verifyContactRowExists",\[19,"([^"]+)"\],\[19,"([^"]+)"\],"((?:\\.|[^"])*)",\[19,"([^"]+)"\],[^[]*?"((?:\\.|[^"])*)",`)
)

type marketplaceInboxThread struct {
	ThreadRowID    string                   `json:"thread_row_id"`
	ThreadKey      string                   `json:"thread_key"`
	Snippet        string                   `json:"snippet"`
	ImageURL       string                   `json:"image_url,omitempty"`
	AuthorityLevel int                      `json:"authority_level"`
	FolderCode     int                      `json:"folder_code"`
	ThreadType     int                      `json:"thread_type"`
	FolderName     string                   `json:"folder_name"`
	Contact        *marketplaceInboxContact `json:"contact,omitempty"`
}

type marketplaceInboxContact struct {
	ContactID   string `json:"contact_id"`
	ContactType int    `json:"contact_type"`
	ProfileType string `json:"profile_type"`
	Name        string `json:"name"`
	ImageURL    string `json:"image_url,omitempty"`
}

type marketplaceInboxSnapshot struct {
	Mode        string                   `json:"mode"`
	Route       string                   `json:"route"`
	QueryName   string                   `json:"query_name"`
	QueryID     string                   `json:"query_id"`
	ThreadCount int                      `json:"thread_count"`
	Threads     []marketplaceInboxThread `json:"threads"`
}

// MarketplaceInboxOverviewFallback fetches the hidden Lightspeed inbox
// preloader from /marketplace/inbox and turns the resulting payload into a
// readable thread snapshot. Facebook's public inbox container query currently
// regresses to a seller-role stub on many buyer sessions, so this fallback is
// the only reliable read path we have when the generated GraphQL doc lies.
func (c *Client) MarketplaceInboxOverviewFallback() (json.RawMessage, int, error) {
	noCacheBefore := c.NoCache
	c.NoCache = true
	defer func() { c.NoCache = noCacheBefore }()

	shell, err := c.Get("/marketplace/inbox", nil)
	if err != nil {
		return nil, 0, err
	}
	queryID, variables, err := extractMarketplaceInboxPreloader(shell)
	if err != nil {
		return nil, 0, err
	}

	varsJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, 0, err
	}
	fields := url.Values{}
	fields.Set("fb_api_req_friendly_name", "LSPlatformGraphQLLightspeedRequestQuery")
	fields.Set("doc_id", queryID)
	fields.Set("variables", string(varsJSON))

	data, statusCode, err := c.PostForm("/api/graphql/", fields)
	if err != nil {
		return nil, statusCode, err
	}

	payload, err := extractMarketplaceLightspeedPayload(data)
	if err != nil {
		return nil, statusCode, err
	}
	threads := parseMarketplaceInboxThreads(payload)
	contacts := parseMarketplaceInboxContacts(payload)
	for i := range threads {
		if contact, ok := contacts[threads[i].ThreadKey]; ok {
			contactCopy := contact
			threads[i].Contact = &contactCopy
		}
	}

	snapshot := marketplaceInboxSnapshot{
		Mode:        "lightspeed_inbox_fallback",
		Route:       "/marketplace/inbox",
		QueryName:   "LSPlatformGraphQLLightspeedRequestQuery",
		QueryID:     queryID,
		ThreadCount: len(threads),
		Threads:     threads,
	}
	out, err := json.Marshal(snapshot)
	if err != nil {
		return nil, statusCode, err
	}
	return out, statusCode, nil
}

func extractMarketplaceInboxPreloader(shell []byte) (string, map[string]any, error) {
	match := marketplaceInboxPreloaderRe.FindSubmatch(shell)
	if len(match) != 4 {
		return "", nil, fmt.Errorf("could not find inbox Lightspeed preloader in /marketplace/inbox shell")
	}
	var variables map[string]any
	if err := json.Unmarshal(match[3], &variables); err != nil {
		return "", nil, fmt.Errorf("parse inbox preloader variables: %w", err)
	}
	return string(match[2]), variables, nil
}

func extractMarketplaceLightspeedPayload(body []byte) (string, error) {
	var response struct {
		Data struct {
			Viewer struct {
				LightspeedWebRequest struct {
					Payload string `json:"payload"`
				} `json:"lightspeed_web_request"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("parse inbox Lightspeed response: %w", err)
	}
	if response.Data.Viewer.LightspeedWebRequest.Payload == "" {
		return "", fmt.Errorf("marketplace inbox Lightspeed payload missing")
	}
	return response.Data.Viewer.LightspeedWebRequest.Payload, nil
}

func parseMarketplaceInboxThreads(payload string) []marketplaceInboxThread {
	out := make([]marketplaceInboxThread, 0)
	seen := map[string]struct{}{}
	for _, match := range inboxThreadRe.FindAllStringSubmatch(payload, -1) {
		if len(match) != 9 {
			continue
		}
		authorityLevel, _ := strconv.Atoi(match[4])
		folderCode, _ := strconv.Atoi(match[6])
		threadType, _ := strconv.Atoi(match[7])
		thread := marketplaceInboxThread{
			ThreadRowID:    match[1],
			ThreadKey:      match[5],
			Snippet:        mustUnescapeMarketplacePayloadString(match[2]),
			ImageURL:       mustUnescapeMarketplacePayloadString(match[3]),
			AuthorityLevel: authorityLevel,
			FolderCode:     folderCode,
			ThreadType:     threadType,
			FolderName:     mustUnescapeMarketplacePayloadString(match[8]),
		}
		key := thread.ThreadKey + ":" + thread.ThreadRowID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, thread)
	}
	return out
}

func parseMarketplaceInboxContacts(payload string) map[string]marketplaceInboxContact {
	out := map[string]marketplaceInboxContact{}
	for _, match := range inboxContactRe.FindAllStringSubmatch(payload, -1) {
		if len(match) != 6 {
			continue
		}
		contactType, _ := strconv.Atoi(match[2])
		out[match[1]] = marketplaceInboxContact{
			ContactID:   match[1],
			ContactType: contactType,
			ProfileType: match[4],
			Name:        mustUnescapeMarketplacePayloadString(match[5]),
			ImageURL:    mustUnescapeMarketplacePayloadString(match[3]),
		}
	}
	return out
}

func mustUnescapeMarketplacePayloadString(raw string) string {
	var out string
	if err := json.Unmarshal([]byte(`"`+raw+`"`), &out); err != nil {
		return raw
	}
	return out
}
