// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
)

// newUUID returns a random RFC-4122 v4 UUID, used as the per-message idempotency
// key Airbnb expects (uniqueIdentifier).
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// CurrentUserID returns the signed-in user's numeric ID, parsed from the
// `_user_attributes` cookie the Airbnb frontend sets on login. Returns an error
// when no session is present.
func (c *Client) CurrentUserID() (string, error) {
	raw := c.session.Cookies["_user_attributes"]
	if raw == "" {
		return "", fmt.Errorf("not authenticated: run `airbnb-outreach-pp-cli auth login --chrome`")
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	var attrs struct {
		IDStr string          `json:"id_str"`
		ID    json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal([]byte(decoded), &attrs); err != nil {
		return "", fmt.Errorf("parsing _user_attributes cookie: %w", err)
	}
	if attrs.IDStr != "" {
		return attrs.IDStr, nil
	}
	if len(attrs.ID) > 0 {
		return string(trimJSONQuotes(attrs.ID)), nil
	}
	return "", fmt.Errorf("no user id in _user_attributes cookie")
}

// Inbox returns message threads for the signed-in user, newest first. cursor is
// an opaque pagination token from a previous page's endCursor ("" for page 1).
func (c *Client) Inbox(limit int, cursor string) (json.RawMessage, error) {
	uid, err := c.CurrentUserID()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 15
	}
	vars := map[string]any{
		"getParticipants":          true,
		"numRequestedThreads":      limit,
		"numPriorityThreads":       20,
		"getPriorityInbox":         false,
		"useUserThreadTag":         true,
		"userId":                   NormalizeViewerID(uid),
		"originType":               "USER_INBOX",
		"threadVisibility":         "UNARCHIVED",
		"threadTagFilters":         []any{},
		"priorityThreadTagFilters": []map[string]any{{"userThreadTagName": "priority"}},
		"query":                    nil,
		"getLastReads":             false,
		"getThreadState":           true,
		"getInboxFields":           true,
		"getInboxOnlyFields":       true,
		"getMessageFields":         false,
		"getThreadOnlyFields":      true,
		"skipOldMessagePreviewFields": false,
	}
	if cursor != "" {
		vars["beforeCursor"] = cursor
	}
	data, err := c.Query("ViaductInboxData", vars)
	if err != nil {
		return nil, err
	}
	return rawAtPath(data, "node", "messagingInbox", "inboxItems"), nil
}

// Thread returns a single conversation with its messages, newest last. id may be
// numeric or an encoded MessageThread global ID.
func (c *Client) Thread(id string, numMessages int) (json.RawMessage, error) {
	if numMessages <= 0 {
		numMessages = 50
	}
	vars := map[string]any{
		"numRequestedMessages":       numMessages,
		"getThreadState":             true,
		"getParticipants":            true,
		"mockThreadIdentifier":       nil,
		"mockMessageTestIdentifier":  nil,
		"getLastReads":               true,
		"forceUgcTranslation":        false,
		"isNovaLite":                 false,
		"globalThreadId":             NormalizeThreadID(id),
		"mockListFooterSlot":         nil,
		"forceReturnAllReadReceipts": false,
		"originType":                 "USER_INBOX",
		"getInboxFields":             true,
		"getInboxOnlyFields":         false,
		"getMessageFields":           true,
		"getThreadOnlyFields":        true,
		"skipOldMessagePreviewFields": false,
	}
	return c.Query("ViaductGetThreadAndDataQuery", vars)
}

// MarkRead marks a thread's latest message as read.
func (c *Client) MarkRead(threadID string) (json.RawMessage, error) {
	vars := map[string]any{
		"request": map[string]any{
			"threadId": NumericID(threadID),
		},
	}
	return c.Mutation("CreateLastMessageReadViaductMutation", vars)
}

// SendMessage sends a text message to a thread. mediaItemIDs, when non-empty,
// attaches previously-uploaded photos (see UploadImage). The variables shape is
// the one Airbnb's web client sends, captured from live traffic.
func (c *Client) SendMessage(threadID, text string, mediaItemIDs []string) (json.RawMessage, error) {
	tid := NumericID(threadID)
	content := map[string]any{}
	if text != "" {
		content["textContent"] = map[string]any{"body": text}
	}
	if len(mediaItemIDs) > 0 {
		imgs := make([]map[string]any, 0, len(mediaItemIDs))
		for _, id := range mediaItemIDs {
			imgs = append(imgs, map[string]any{"mediaItemId": id})
		}
		content["imageContent"] = map[string]any{"images": imgs}
	}
	message := map[string]any{
		"content":          content,
		"loggingSource":    map[string]any{"sourceType": "", "sourceId": ""},
		"loggingExtras":    map[string]any{"entries": []any{}},
		"uniqueIdentifier": newUUID(),
	}
	vars := map[string]any{
		"actAs":                 "PARTICIPANT",
		"businessJustification": map[string]any{"feature": "USER_INBOX"},
		"originType":            "USER_INBOX",
		"senderPlatform":        "WEB",
		"messages":              []map[string]any{message},
		"messageThreadId":       tid,
		"skipOldMessagePreviewFields": false,
	}
	return c.Mutation("CreateBulkMessagesMutation", vars)
}

func trimJSONQuotes(b json.RawMessage) json.RawMessage {
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return b[1 : len(b)-1]
	}
	return b
}
