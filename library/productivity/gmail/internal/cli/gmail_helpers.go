// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: shared plumbing for the Gmail-specific commands (find, read,
// send, reply, forward, triage, watch, pull, attachments, export, schedule).
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// gmailListIDs pages messages.list for up to limit ids matching query.
// Label scoping goes through the query string (`label:X`, `in:inbox`), which
// is what Gmail's own search box does.
func gmailListIDs(ctx context.Context, c *client.Client, query string, limit int) ([]string, error) {
	return gmailListIDsAt(ctx, c, "/gmail/v1/users/me/messages", query, limit)
}

// gmailListIDsAt is gmailListIDs against an explicit messages.list path.
// Callers that resolve the path through the emitted resource path map
// (export's batch mode) use this variant so the list endpoint has a single
// source of truth; {userId} placeholders are substituted by buildURL.
func gmailListIDsAt(ctx context.Context, c *client.Client, listPath, query string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive, got %d", limit)
	}
	var ids []string
	pageToken := ""
	for {
		params := map[string]string{}
		if query != "" {
			params["q"] = query
		}
		remaining := limit - len(ids)
		pageSize := 100
		if remaining < pageSize {
			pageSize = remaining
		}
		params["maxResults"] = fmt.Sprintf("%d", pageSize)
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.Get(ctx, listPath, params)
		if err != nil {
			return ids, err
		}
		var page struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return ids, fmt.Errorf("parsing messages.list response: %w", err)
		}
		for _, m := range page.Messages {
			ids = append(ids, m.ID)
		}
		if page.NextPageToken == "" || len(ids) >= limit {
			break
		}
		pageToken = page.NextPageToken
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

// gmailSearchMetadata is the shared live-search path: list ids, hydrate
// metadata via the batch endpoint. The skipped count is returned so callers
// can tell a short result set from a complete one.
func gmailSearchMetadata(ctx context.Context, c *client.Client, query string, limit int) ([]gmailmail.Message, int, error) {
	ids, err := gmailListIDs(ctx, c, query, limit)
	if err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return nil, 0, nil
	}
	return gmailmail.BatchGetMessages(ctx, c, ids, "metadata", gmailmail.DefaultMetadataHeaders)
}

// warnSkipped reports messages the batch endpoint could not return.
func warnSkipped(cmd *cobra.Command, skipped int) {
	if skipped > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d message(s) could not be fetched and are missing from these results\n", skipped)
	}
}

// gmailGetMessage fetches one message in the requested format.
func gmailGetMessage(ctx context.Context, c *client.Client, id, format string) (*gmailmail.Message, error) {
	params := map[string]string{}
	if format != "" {
		params["format"] = format
	}
	data, err := c.Get(ctx, "/gmail/v1/users/me/messages/"+id, params)
	if err != nil {
		return nil, err
	}
	var msg gmailmail.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parsing message %s: %w", id, err)
	}
	return &msg, nil
}

// gmailSendRaw posts a base64url RFC 2822 message; threadID keeps replies in
// their conversation. Returns the sent message's Gmail id.
func gmailSendRaw(ctx context.Context, c *client.Client, raw, threadID string) (string, error) {
	body := map[string]string{"raw": raw}
	if threadID != "" {
		body["threadId"] = threadID
	}
	data, _, err := c.Post(ctx, "/gmail/v1/users/me/messages/send", body)
	if err != nil {
		return "", err
	}
	var sent struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &sent); err != nil {
		return "", fmt.Errorf("parsing send response: %w", err)
	}
	return sent.ID, nil
}

// gmailProfile fetches the mailbox profile (email address, historyId, counts).
func gmailProfile(ctx context.Context, c *client.Client) (email, historyID string, err error) {
	data, err := c.Get(ctx, "/gmail/v1/users/me/profile", nil)
	if err != nil {
		return "", "", err
	}
	var p struct {
		EmailAddress string `json:"emailAddress"`
		HistoryID    string `json:"historyId"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return "", "", fmt.Errorf("parsing profile: %w", err)
	}
	return p.EmailAddress, p.HistoryID, nil
}

// messageRow is the compact listing shape find/triage print and emit as JSON.
type messageRow struct {
	ID           string `json:"id"`
	ThreadID     string `json:"thread_id"`
	Date         string `json:"date"`
	From         string `json:"from"`
	Subject      string `json:"subject"`
	Snippet      string `json:"snippet,omitempty"`
	Unread       bool   `json:"unread"`
	SizeEstimate int64  `json:"size_estimate,omitempty"`
}

func toMessageRows(msgs []gmailmail.Message) []messageRow {
	rows := make([]messageRow, 0, len(msgs))
	for i := range msgs {
		m := &msgs[i]
		date := ""
		if t, ok := m.InternalTime(); ok {
			date = t.Local().Format("2006-01-02 15:04")
		}
		rows = append(rows, messageRow{
			ID:           m.ID,
			ThreadID:     m.ThreadID,
			Date:         date,
			From:         m.Header("From"),
			Subject:      m.Header("Subject"),
			Snippet:      m.Snippet,
			Unread:       m.HasLabel("UNREAD"),
			SizeEstimate: m.SizeEstimate,
		})
	}
	return rows
}

func printMessageRows(w io.Writer, rows []messageRow) {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tDATE\tFROM\tSUBJECT")
	for _, r := range rows {
		marker := " "
		if r.Unread {
			marker = "*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s%s\n", r.ID, r.Date, truncateCell(r.From, 34), marker, truncateCell(r.Subject, 60))
	}
	tw.Flush()
}

// truncateCell shortens a display cell. It slices by rune so multi-byte
// subjects and sender names are never cut mid-character.
func truncateCell(s string, max int) string {
	s = strings.ReplaceAll(s, "\t", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// wantsJSONOutput mirrors the generated convention: machine flags or a piped
// stdout select JSON.
func wantsJSONOutput(cmd interface{ OutOrStdout() io.Writer }, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || flags.selectFields != "" || flags.csv || !isTerminal(cmd.OutOrStdout())
}

// openGmailStore opens (or creates) the local store at the resolved path.
func openGmailStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("gmail-pp-cli")
	}
	return store.OpenWithContext(ctx, dbPath)
}

// parseSendAt resolves --at (absolute) / --in (relative) into a UTC time.
func parseSendAt(at, in string, now time.Time) (time.Time, error) {
	switch {
	case at != "" && in != "":
		return time.Time{}, fmt.Errorf("use either --at or --in, not both")
	case in != "":
		d, err := cliutil.ParseDurationLoose(in)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --in duration %q: %w", in, err)
		}
		if d <= 0 {
			return time.Time{}, fmt.Errorf("--in must be a positive duration")
		}
		return now.Add(d).UTC(), nil
	case at != "":
		// RFC3339 carries its own offset; the bare layouts are interpreted in
		// the user's local zone, which is what someone typing "09:00" means.
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
		} {
			if t, err := time.ParseInLocation(layout, at, time.Local); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("invalid --at time %q: use RFC3339 or \"2006-01-02 15:04\" (local time)", at)
	default:
		return time.Time{}, fmt.Errorf("either --at or --in is required")
	}
}
