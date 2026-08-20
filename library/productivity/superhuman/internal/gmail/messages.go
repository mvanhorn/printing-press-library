// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ThreadRef is the minimal handle the Gmail messages.list endpoint returns
// per thread. id is the value to pass to GetMessage / GetAttachment;
// historyId rotates whenever the thread mutates so callers can detect
// staleness later.
type ThreadRef struct {
	ID        string `json:"id"`
	HistoryID string `json:"historyId"`
}

// ListInboxResult is the typed response from ListInboxThreads. NextPageToken
// is empty when there are no more pages; callers use it as the page-token
// for the next call.
type ListInboxResult struct {
	Threads            []ThreadRef
	NextPageToken      string
	ResultSizeEstimate int
}

// MessageRef is the minimal handle users.messages.list returns per message.
type MessageRef struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

// ListMessagesResult is the typed response from users.messages.list.
type ListMessagesResult struct {
	Messages           []MessageRef
	NextPageToken      string
	ResultSizeEstimate int
}

// ListInboxThreads calls Gmail's users.threads.list endpoint scoped to the
// INBOX label and returns the resulting thread refs. pageSize maps to
// Gmail's maxResults (capped at 500 by Gmail). pageToken continues a prior
// listing; empty fetches the first page.
//
// Why threads (not messages) for inbox listing: an inbox is a list of
// conversations, not individual messages. The MCP's list_threads tool also
// returns threads. Callers fetch per-thread bodies via GetThread or
// per-message bodies via GetMessage.
func (c *Client) ListInboxThreads(ctx context.Context, pageSize int, pageToken string) (*ListInboxResult, error) {
	return c.ListThreads(ctx, []string{"INBOX"}, "", pageSize, pageToken)
}

// ListThreads calls Gmail's users.threads.list endpoint with optional label
// IDs and Gmail search syntax.
func (c *Client) ListThreads(ctx context.Context, labelIDs []string, query string, pageSize int, pageToken string) (*ListInboxResult, error) {
	if pageSize <= 0 {
		pageSize = 25
	}
	if pageSize > 500 {
		pageSize = 500
	}
	q := url.Values{}
	q.Set("maxResults", strconv.Itoa(pageSize))
	for _, labelID := range labelIDs {
		if labelID != "" {
			q.Add("labelIds", labelID)
		}
	}
	if query != "" {
		q.Set("q", query)
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var raw struct {
		Threads            []ThreadRef `json:"threads"`
		NextPageToken      string      `json:"nextPageToken"`
		ResultSizeEstimate int         `json:"resultSizeEstimate"`
	}
	if err := c.GetJSON(ctx, "/users/me/threads?"+q.Encode(), &raw); err != nil {
		return nil, err
	}
	return &ListInboxResult{
		Threads:            raw.Threads,
		NextPageToken:      raw.NextPageToken,
		ResultSizeEstimate: raw.ResultSizeEstimate,
	}, nil
}

// ListMessages calls Gmail's users.messages.list endpoint with optional label
// and search-query filters. pageSize maps to Gmail maxResults and is capped at
// Gmail's documented 500-result limit.
func (c *Client) ListMessages(ctx context.Context, labelIDs []string, query string, pageSize int, pageToken string) (*ListMessagesResult, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	q := url.Values{}
	q.Set("maxResults", strconv.Itoa(pageSize))
	for _, labelID := range labelIDs {
		if labelID != "" {
			q.Add("labelIds", labelID)
		}
	}
	if query != "" {
		q.Set("q", query)
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var raw struct {
		Messages           []MessageRef `json:"messages"`
		NextPageToken      string       `json:"nextPageToken"`
		ResultSizeEstimate int          `json:"resultSizeEstimate"`
	}
	if err := c.GetJSON(ctx, "/users/me/messages?"+q.Encode(), &raw); err != nil {
		return nil, err
	}
	return &ListMessagesResult{
		Messages:           raw.Messages,
		NextPageToken:      raw.NextPageToken,
		ResultSizeEstimate: raw.ResultSizeEstimate,
	}, nil
}

// ListWithQuery is the Gmail-search passthrough convenience used by
// `messages list --query`.
func (c *Client) ListWithQuery(ctx context.Context, query string, pageSize int, pageToken string) (*ListMessagesResult, error) {
	return c.ListMessages(ctx, nil, query, pageSize, pageToken)
}

// Header is one entry from a Gmail message's parsed headers list.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AttachmentMeta is the per-attachment metadata GetMessage returns. The
// actual bytes are fetched separately via GetAttachment.
type AttachmentMeta struct {
	PartID       string `json:"partId"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mimeType"`
	AttachmentID string `json:"attachmentId"`
	Size         int    `json:"size"`
}

// Message is the typed view GetMessage returns. Body is the decoded
// text/plain body when present (Gmail's `format=full` response delivers it
// base64url-encoded inside MessagePart.Body.Data, which this helper decodes).
// HTMLBody is the decoded text/html alternative when present.
type Message struct {
	ID           string
	ThreadID     string
	LabelIDs     []string
	Snippet      string
	HistoryID    string
	InternalDate int64
	Headers      []Header
	Body         string // decoded text/plain
	HTMLBody     string // decoded text/html
	Attachments  []AttachmentMeta
}

// gmailMessagePart matches Gmail's MessagePart shape closely enough to
// decode the recursive body+attachment tree. Kept private — callers see
// the flattened Message struct above.
type gmailMessagePart struct {
	PartID   string               `json:"partId"`
	MimeType string               `json:"mimeType"`
	Filename string               `json:"filename"`
	Headers  []Header             `json:"headers"`
	Body     gmailMessagePartBody `json:"body"`
	Parts    []gmailMessagePart   `json:"parts"`
}

type gmailMessagePartBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int    `json:"size"`
	Data         string `json:"data"` // base64url
}

type gmailMessageRaw struct {
	ID           string           `json:"id"`
	ThreadID     string           `json:"threadId"`
	LabelIDs     []string         `json:"labelIds"`
	Snippet      string           `json:"snippet"`
	HistoryID    string           `json:"historyId"`
	InternalDate string           `json:"internalDate"`
	Payload      gmailMessagePart `json:"payload"`
}

// GetMessage fetches one message by id and returns its decoded body,
// headers, and attachment metadata. format defaults to "full" which is the
// shape every existing MCP tool expects.
func (c *Client) GetMessage(ctx context.Context, id, format string) (*Message, error) {
	if id == "" {
		return nil, fmt.Errorf("gmail: GetMessage: id is required")
	}
	if format == "" {
		format = "full"
	}
	q := url.Values{}
	q.Set("format", format)

	var raw gmailMessageRaw
	if err := c.GetJSON(ctx, "/users/me/messages/"+url.PathEscape(id)+"?"+q.Encode(), &raw); err != nil {
		return nil, err
	}

	msg := &Message{
		ID:        raw.ID,
		ThreadID:  raw.ThreadID,
		LabelIDs:  raw.LabelIDs,
		Snippet:   raw.Snippet,
		HistoryID: raw.HistoryID,
	}
	if raw.InternalDate != "" {
		if n, perr := strconv.ParseInt(raw.InternalDate, 10, 64); perr == nil {
			msg.InternalDate = n
		}
	}
	// Headers live on the top-level payload.
	msg.Headers = raw.Payload.Headers

	// Walk the payload to surface text/plain, text/html, and attachments.
	walkPart(raw.Payload, msg)
	return msg, nil
}

// walkPart recurses through the message-part tree, populating Body /
// HTMLBody / Attachments on the target Message. The first text/plain found
// wins for Body; the first text/html wins for HTMLBody. Attachments
// accumulate across the whole tree.
func walkPart(p gmailMessagePart, target *Message) {
	if p.Filename != "" && p.Body.AttachmentID != "" {
		target.Attachments = append(target.Attachments, AttachmentMeta{
			PartID:       p.PartID,
			Filename:     p.Filename,
			MimeType:     p.MimeType,
			AttachmentID: p.Body.AttachmentID,
			Size:         p.Body.Size,
		})
	}
	switch {
	case strings.HasPrefix(p.MimeType, "text/plain") && target.Body == "" && p.Body.Data != "":
		if decoded, err := decodeURL(p.Body.Data); err == nil {
			target.Body = string(decoded)
		}
	case strings.HasPrefix(p.MimeType, "text/html") && target.HTMLBody == "" && p.Body.Data != "":
		if decoded, err := decodeURL(p.Body.Data); err == nil {
			target.HTMLBody = string(decoded)
		}
	}
	for _, child := range p.Parts {
		walkPart(child, target)
	}
}

// Attachment is the typed response from GetAttachment. Data holds the
// decoded attachment bytes; Size matches what Gmail reported. A mismatch
// between len(Data) and Size returns an error from GetAttachment rather
// than silently truncating, so callers (CLI: messages get-attachment) can
// refuse to write a partial file.
type Attachment struct {
	Data []byte
	Size int
}

// GetAttachment fetches one attachment's bytes by message id + attachment
// id. The Gmail API returns the bytes base64url-encoded; this helper
// decodes them. A size mismatch surfaces as an error.
func (c *Client) GetAttachment(ctx context.Context, messageID, attachmentID string) (*Attachment, error) {
	if messageID == "" || attachmentID == "" {
		return nil, fmt.Errorf("gmail: GetAttachment: both messageID and attachmentID are required")
	}
	var raw struct {
		Size int    `json:"size"`
		Data string `json:"data"`
	}
	path := fmt.Sprintf("/users/me/messages/%s/attachments/%s", url.PathEscape(messageID), url.PathEscape(attachmentID))
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	data, err := decodeURL(raw.Data)
	if err != nil {
		return nil, fmt.Errorf("gmail: decode attachment data: %w", err)
	}
	if len(data) != raw.Size {
		return nil, fmt.Errorf("gmail: attachment size mismatch: got %d bytes, expected %d", len(data), raw.Size)
	}
	return &Attachment{Data: data, Size: raw.Size}, nil
}

// decodeURL handles Gmail's base64url-without-padding encoding (RFC 4648 §5)
// and silently re-pads as required by Go's base64.URLEncoding.
func decodeURL(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

// ModifyMessageLabels calls Gmail's users.messages.modify endpoint to add
// and/or remove labels on a single message. add and remove may each be nil
// or empty; at least one must be non-empty. Returns the updated message's
// label-id set.
//
// Why on messages, not threads, for the v1.1 use case: Gmail exposes both
// users.messages.modify and users.threads.modify. The MCP's update_thread
// applies labels at the thread level (every message in the thread gets the
// change), which matches user mental model. ModifyThreadLabels below is
// the thread-level mirror; per-message modify exists for the rare case
// where the caller needs single-message granularity.
func (c *Client) ModifyMessageLabels(ctx context.Context, messageID string, add, remove []string) ([]string, error) {
	if messageID == "" {
		return nil, fmt.Errorf("gmail: ModifyMessageLabels: messageID is required")
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil, fmt.Errorf("gmail: ModifyMessageLabels: at least one of add/remove must be non-empty")
	}
	reqBody := map[string]any{
		"addLabelIds":    add,
		"removeLabelIds": remove,
	}
	var raw struct {
		LabelIDs []string `json:"labelIds"`
	}
	path := "/users/me/messages/" + url.PathEscape(messageID) + "/modify"
	if err := c.PostJSON(ctx, path, reqBody, &raw); err != nil {
		return nil, err
	}
	return raw.LabelIDs, nil
}

// ModifyThreadLabels applies add/remove labels to every message in the
// thread. Returns the new label-id set on the thread (Gmail returns the
// whole-thread shape; we take the labels of the first message — they are
// uniform across the thread by Gmail's modify contract).
func (c *Client) ModifyThreadLabels(ctx context.Context, threadID string, add, remove []string) ([]string, error) {
	if threadID == "" {
		return nil, fmt.Errorf("gmail: ModifyThreadLabels: threadID is required")
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil, fmt.Errorf("gmail: ModifyThreadLabels: at least one of add/remove must be non-empty")
	}
	reqBody := map[string]any{
		"addLabelIds":    add,
		"removeLabelIds": remove,
	}
	var raw struct {
		Messages []struct {
			LabelIDs []string `json:"labelIds"`
		} `json:"messages"`
	}
	path := "/users/me/threads/" + url.PathEscape(threadID) + "/modify"
	if err := c.PostJSON(ctx, path, reqBody, &raw); err != nil {
		return nil, err
	}
	if len(raw.Messages) == 0 {
		return nil, nil
	}
	return raw.Messages[0].LabelIDs, nil
}
