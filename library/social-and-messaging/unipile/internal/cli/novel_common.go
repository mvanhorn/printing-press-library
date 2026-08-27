// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// novelStore opens the local mirror for a read-only novel command.
//
// Returns ok=false when the mirror does not exist yet: the caller has already
// been handed an empty machine-readable result (or a human hint on stderr) and
// should return nil. A missing mirror is an empty local-cache state, not an
// error.
func novelStore(cmd *cobra.Command, flags *rootFlags, dbPath string, emptyValue any) (*sql.DB, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("unipile-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unipile-pp-cli sync --db %s\n", dbPath, dbPath)
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return nil, false, printJSONFiltered(cmd.OutOrStdout(), emptyValue, flags)
		}
		return nil, false, nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local mirror: %w", err)
	}
	return db, true, nil
}

// relationRow is one LinkedIn connection from the local mirror.
type relationRow struct {
	MemberID   string `json:"member_id"`
	Name       string `json:"name"`
	Headline   string `json:"headline,omitempty"`
	PublicID   string `json:"public_identifier,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`
	CreatedAt  string `json:"connected_at,omitempty"`
}

// loadRelations reads every synced connection keyed by LinkedIn member id.
// Drains fully before returning so callers may issue follow-up queries.
func loadRelations(ctx context.Context, db *sql.DB) (map[string]relationRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(json_extract(data,'$.member_id'), ''),
		       COALESCE(json_extract(data,'$.first_name'), ''),
		       COALESCE(json_extract(data,'$.last_name'), ''),
		       COALESCE(json_extract(data,'$.headline'), ''),
		       COALESCE(json_extract(data,'$.public_identifier'), ''),
		       COALESCE(json_extract(data,'$.public_profile_url'), ''),
		       COALESCE(json_extract(data,'$.created_at'), 0)
		FROM resources WHERE resource_type = 'users-relations'`)
	if err != nil {
		return nil, fmt.Errorf("reading connections: %w", err)
	}
	out := make(map[string]relationRow)
	for rows.Next() {
		var member, first, last, headline, publicID, profileURL sql.NullString
		var createdMS sql.NullInt64
		if err := rows.Scan(&member, &first, &last, &headline, &publicID, &profileURL, &createdMS); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning connection: %w", err)
		}
		if member.String == "" {
			continue
		}
		r := relationRow{
			MemberID:   member.String,
			Name:       strings.TrimSpace(first.String + " " + last.String),
			Headline:   headline.String,
			PublicID:   publicID.String,
			ProfileURL: profileURL.String,
		}
		if createdMS.Int64 > 0 {
			r.CreatedAt = time.UnixMilli(createdMS.Int64).UTC().Format(time.RFC3339)
		}
		out[r.MemberID] = r
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating connections: %w", err)
	}
	return out, rows.Close()
}

// lastMessage is the most recent message in one chat.
type lastMessage struct {
	ChatID    string
	Text      string
	Timestamp time.Time
	FromMe    bool
	// SenderAttendeeID and SenderID identify who actually sent the message.
	// Chat titles are not senders: group chats carry their own name and
	// sponsored InMail carries a subject line, so callers that want a person
	// must resolve these rather than reusing the chat name.
	SenderAttendeeID string
	SenderID         string
}

// loadLastMessages returns the newest message per chat plus per-chat inbound
// and outbound counts. One pass over the local messages table; drained before
// return so callers may issue follow-up queries.
func loadLastMessages(ctx context.Context, db *sql.DB) (map[string]lastMessage, map[string]int, map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(chat_id,''), COALESCE(text,''), COALESCE(timestamp,''), COALESCE(is_sender,'0'),
		       COALESCE(sender_attendee_id,''), COALESCE(sender_id,'')
		FROM messages WHERE chat_id IS NOT NULL AND chat_id != ''`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading messages: %w", err)
	}
	last := make(map[string]lastMessage)
	inbound := make(map[string]int)
	outbound := make(map[string]int)
	for rows.Next() {
		var chatID, text, ts, isSender, senderAttendeeID, senderID sql.NullString
		if err := rows.Scan(&chatID, &text, &ts, &isSender, &senderAttendeeID, &senderID); err != nil {
			_ = rows.Close()
			return nil, nil, nil, fmt.Errorf("scanning message: %w", err)
		}
		fromMe := isSender.String == "1" || strings.EqualFold(isSender.String, "true")
		if fromMe {
			outbound[chatID.String]++
		} else {
			inbound[chatID.String]++
		}
		when, parseErr := time.Parse(time.RFC3339, ts.String)
		if parseErr != nil {
			continue
		}
		if prev, ok := last[chatID.String]; !ok || when.After(prev.Timestamp) {
			last[chatID.String] = lastMessage{
				ChatID:           chatID.String,
				Text:             text.String,
				Timestamp:        when,
				FromMe:           fromMe,
				SenderAttendeeID: senderAttendeeID.String,
				SenderID:         senderID.String,
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, nil, nil, fmt.Errorf("iterating messages: %w", err)
	}
	return last, inbound, outbound, rows.Close()
}

// chatRow is one conversation from the local mirror.
type chatRow struct {
	ID          string
	Name        string
	AccountType string
	AttendeeID  string
	UnreadCount int
	Timestamp   time.Time
}

// loadChats reads every synced conversation. onlyUnread restricts to chats the
// provider still reports as unread.
func loadChats(ctx context.Context, db *sql.DB, onlyUnread bool) ([]chatRow, error) {
	q := `SELECT COALESCE(id,''),
	             COALESCE(json_extract(data,'$.name'),''),
	             COALESCE(account_type,''),
	             COALESCE(attendee_provider_id,''),
	             COALESCE(json_extract(data,'$.unread_count'),0),
	             COALESCE(timestamp,'')
	      FROM chats`
	if onlyUnread {
		q += ` WHERE json_extract(data,'$.unread') = 1`
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading chats: %w", err)
	}
	out := make([]chatRow, 0)
	for rows.Next() {
		var id, name, accountType, attendee, ts sql.NullString
		var unread sql.NullInt64
		if err := rows.Scan(&id, &name, &accountType, &attendee, &unread, &ts); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning chat: %w", err)
		}
		c := chatRow{
			ID:          id.String,
			Name:        name.String,
			AccountType: accountType.String,
			AttendeeID:  attendee.String,
			UnreadCount: int(unread.Int64),
		}
		if when, parseErr := time.Parse(time.RFC3339, ts.String); parseErr == nil {
			c.Timestamp = when
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating chats: %w", err)
	}
	return out, rows.Close()
}

// attendeeIndex resolves the display names Unipile stores for chat participants.
// Chats key their counterpart by provider id; messages key their sender by the
// attendee's own id, so both directions are indexed.
type attendeeIndex struct {
	byProviderID map[string]string
	byAttendeeID map[string]string
}

// loadAttendees reads every synced chat participant. Returns an empty index
// (never nil) when chat-attendees have not been synced, so callers can look up
// unconditionally and fall back to ids.
func loadAttendees(ctx context.Context, db *sql.DB) (attendeeIndex, error) {
	idx := attendeeIndex{byProviderID: map[string]string{}, byAttendeeID: map[string]string{}}
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(id,''), COALESCE(provider_id,''), COALESCE(name,''), COALESCE(is_self,'0')
		FROM chat_attendees`)
	if err != nil {
		// chat-attendees is optional: an un-synced mirror is not an error.
		return idx, nil
	}
	for rows.Next() {
		var id, providerID, name, isSelf sql.NullString
		if err := rows.Scan(&id, &providerID, &name, &isSelf); err != nil {
			_ = rows.Close()
			return idx, fmt.Errorf("scanning attendee: %w", err)
		}
		if name.String == "" {
			continue
		}
		if providerID.String != "" {
			idx.byProviderID[providerID.String] = name.String
		}
		if id.String != "" {
			idx.byAttendeeID[id.String] = name.String
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return idx, fmt.Errorf("iterating attendees: %w", err)
	}
	return idx, rows.Close()
}

// counterpartName resolves the human on the other side of a chat. Group chats
// carry their own name; one-to-one chats resolve through the attendee index,
// then the connection list, and only then fall back to the raw provider id.
func counterpartName(c chatRow, relations map[string]relationRow, attendees attendeeIndex) string {
	if c.Name != "" {
		return c.Name
	}
	if n, ok := attendees.byProviderID[c.AttendeeID]; ok && n != "" {
		return n
	}
	if r, ok := relations[c.AttendeeID]; ok && r.Name != "" {
		return r.Name
	}
	if c.AttendeeID != "" {
		return c.AttendeeID
	}
	return "(unknown)"
}

// senderLabel names who sent a message. Prefers the real attendee behind the
// message over the conversation title: a chat's name is the thread's subject,
// not a person, so sponsored InMail and group threads would otherwise report a
// subject line where a sender belongs. Falls back to "them" when the mirror has
// no attendee record, which stays truthful without inventing a name.
func senderLabel(m lastMessage, relations map[string]relationRow, attendees attendeeIndex) string {
	if m.FromMe {
		return "you"
	}
	if n, ok := attendees.byAttendeeID[m.SenderAttendeeID]; ok && n != "" {
		return n
	}
	if n, ok := attendees.byProviderID[m.SenderID]; ok && n != "" {
		return n
	}
	if r, ok := relations[m.SenderID]; ok && r.Name != "" {
		return r.Name
	}
	return "them"
}

// emptyMirrorHint explains an empty result on a mirror that was never synced.
// Written to stderr so the stdout JSON contract stays a bare array, matching
// how novelStore reports a missing database file.
func emptyMirrorHint(ctx context.Context, cmd *cobra.Command, db *sql.DB, resultCount int) {
	if resultCount > 0 || mirrorPopulated(ctx, db) {
		return
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "hint: the local mirror is empty; run 'unipile-pp-cli sync' before trusting this result.")
}

// parseWindow accepts the same loose duration vocabulary as sync --since
// (7d, 2w, 24h) and returns the cutoff instant.
func parseWindow(now time.Time, raw string) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, nil
	}
	d, err := parseLooseDuration(raw)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-d), nil
}

// parseLooseDuration extends time.ParseDuration with the d and w suffixes that
// the framework's sync --since already accepts.
func parseLooseDuration(raw string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	mult := time.Duration(0)
	switch {
	case strings.HasSuffix(s, "d"):
		mult = 24 * time.Hour
		s = strings.TrimSuffix(s, "d")
	case strings.HasSuffix(s, "w"):
		mult = 7 * 24 * time.Hour
		s = strings.TrimSuffix(s, "w")
	}
	if mult > 0 {
		var n float64
		if _, err := fmt.Sscanf(s, "%f", &n); err != nil {
			return 0, fmt.Errorf("invalid duration %q", raw)
		}
		return time.Duration(n * float64(mult)), nil
	}
	return time.ParseDuration(s)
}

// mirrorPopulated reports whether the local mirror holds any synced rows.
//
// "nothing matched your query" and "you have not synced yet" are different
// answers and deserve different exit codes: the first is a genuine not-found
// (exit 3), the second is an empty cache that a sync fixes (exit 0 plus a
// hint). Without this split a fresh install looks like a failed lookup.
func mirrorPopulated(ctx context.Context, db *sql.DB) bool {
	var n sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		return false
	}
	return n.Int64 > 0
}
