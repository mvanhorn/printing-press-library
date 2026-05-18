// Hand-built (Phase 3): shared helpers for the offline-store novel commands
// (followup, senders, since, flagged, stale-unread, waiting, conversations,
// quiet, digest, attachments-stale, dedup, bulk-archive). Every helper here
// assumes `outlook-email-pp-cli sync` has populated the local SQLite store;
// commands surface a clear "run sync first" hint via the messages table when
// the row count is zero. SQL predicates push the time-window into the WHERE
// clause per the PR #408 P2 lesson — Go-side filtering is the precise gate,
// not the scan boundary.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-email/internal/store"
)

// messageRow is the typed projection of the messages table that every novel
// command consumes. Fields not needed for a given command are simply left at
// their zero value — keeping a single struct avoids reflection-y casts
// across the dozen commands that share it.
type messageRow struct {
	ID                      string    `json:"id"`
	ConversationID          string    `json:"conversation_id,omitempty"`
	InternetMessageID       string    `json:"internet_message_id,omitempty"`
	ParentFolderID          string    `json:"parent_folder_id,omitempty"`
	Subject                 string    `json:"subject,omitempty"`
	BodyPreview             string    `json:"body_preview,omitempty"`
	Importance              string    `json:"importance,omitempty"`
	InferenceClassification string    `json:"inference_classification,omitempty"`
	IsRead                  bool      `json:"is_read"`
	IsDraft                 bool      `json:"is_draft"`
	HasAttachments          bool      `json:"has_attachments"`
	ReceivedAt              time.Time `json:"received_at,omitempty"`
	SentAt                  time.Time `json:"sent_at,omitempty"`
	FromEmail               string    `json:"from_email,omitempty"`
	FromName                string    `json:"from_name,omitempty"`
	ToEmails                []string  `json:"to_emails,omitempty"`
	CcEmails                []string  `json:"cc_emails,omitempty"`
	Categories              []string  `json:"categories,omitempty"`
	FlagStatus              string    `json:"flag_status,omitempty"`
	FlagDueAt               time.Time `json:"flag_due_at,omitempty"`
	FlagStartAt             time.Time `json:"flag_start_at,omitempty"`
	FlagCompletedAt         time.Time `json:"flag_completed_at,omitempty"`
	WebLink                 string    `json:"web_link,omitempty"`
}

// loadMessagesFilter is what callers pass to loadMessages. Zero-value times
// disable the bound; an empty slice disables that filter dimension entirely.
// The SQL builder pushes every set predicate into the WHERE clause, so a
// command that asks for `ReceivedAfter = now - 30d` will not touch a single
// row outside that window — even on a 500k-message mailbox.
type loadMessagesFilter struct {
	ReceivedAfter  time.Time
	ReceivedBefore time.Time
	SentAfter      time.Time
	SentBefore     time.Time
	IsRead         *bool // nil = either
	IsDraft        *bool
	HasAttachments *bool
	Folders        []string // parent_folder_id IN (...)
	Senders        []string // matches data->>'from.emailAddress.address' (case-insensitive)
	Conversations  []string
	ExcludeDrafts  bool
	FlaggedOnly    bool
	IncompleteFlag bool   // flag_status = 'flagged' AND completedDateTime IS NULL
	Inference      string // focused | other | ""
	Limit          int    // 0 = no LIMIT (caller is expected to bound by time)
	OrderBy        string // default: received_date_time DESC
}

// myAddress returns the authenticated user's email address. We resolve it
// from local store evidence; we never want the answer to depend on which
// external sender happens to be loudest in the inbox.
//
// Heuristic 1 (best): the `from` address of any message in a sentitems-shaped
// folder. Only ever the mailbox owner.
//
// Heuristic 2 (fallback when no sentitems messages have synced yet): the
// address that appears most often in `toRecipients` across messages from
// many distinct senders. Inbound mail to the mailbox owner shows that owner
// as a `to` recipient repeatedly; external senders don't. We require the
// address to appear with at least two distinct from-addresses, which is
// enough to rule out per-sender mailing-list quirks.
//
// As a last resort we return "" and the caller handles the empty result.
func myAddress(db *sql.DB) (string, error) {
	// Heuristic 1: from-address of any message in a sentitems-shaped folder.
	row := db.QueryRow(`
		SELECT json_extract(data, '$.from.emailAddress.address')
		FROM messages
		WHERE parent_folder_id IN (
			SELECT id FROM folders WHERE LOWER(well_known_name) = 'sentitems' OR LOWER(display_name) = 'sent items'
		)
		AND json_extract(data, '$.from.emailAddress.address') IS NOT NULL
		LIMIT 1`)
	var addr sql.NullString
	if err := row.Scan(&addr); err == nil && addr.Valid && addr.String != "" {
		return strings.ToLower(addr.String), nil
	}

	// Heuristic 2: top `toRecipients` address across distinct senders. The
	// mailbox owner is the consistent recipient on inbound mail; external
	// senders never appear in `toRecipients` repeatedly across unrelated
	// senders.
	row = db.QueryRow(`
		WITH recipients AS (
			SELECT
				LOWER(json_extract(r.value, '$.emailAddress.address')) AS to_addr,
				LOWER(json_extract(m.data, '$.from.emailAddress.address')) AS from_addr
			FROM messages m, json_each(m.data, '$.toRecipients') r
			WHERE json_extract(r.value, '$.emailAddress.address') IS NOT NULL
			  AND json_extract(m.data, '$.from.emailAddress.address') IS NOT NULL
		)
		SELECT to_addr, COUNT(*) AS n
		FROM recipients
		WHERE to_addr IS NOT NULL AND to_addr != ''
		GROUP BY to_addr
		HAVING COUNT(DISTINCT from_addr) >= 2
		ORDER BY n DESC
		LIMIT 1`)
	if err := row.Scan(&addr, new(int)); err == nil && addr.Valid {
		return addr.String, nil
	}
	return "", nil
}

// safeOrderByExprs is the closed allowlist of ORDER BY expressions loadMessages
// may emit. Every novel-command caller picks from this set; an empty or
// unrecognized value falls back to received_date_time DESC. Keeping this list
// explicit blocks any future caller from passing user- or config-derived
// strings into the concatenated SQL.
var safeOrderByExprs = map[string]struct{}{
	"received_date_time DESC": {},
	"received_date_time ASC":  {},
	"sent_date_time DESC":     {},
	"sent_date_time ASC":      {},
}

func safeOrderBy(order string) string {
	if _, ok := safeOrderByExprs[order]; ok {
		return order
	}
	return "received_date_time DESC"
}

// loadMessages runs a parameterized SQL query against the messages table,
// pushing every set field of `f` into the WHERE clause. Time predicates are
// rendered as `received_date_time >= ?` using ISO-8601 strings — that's the
// shape Graph stores in the JSON payload and the shape we persist into the
// typed column, so string comparison is correct without parsing per row.
func loadMessages(ctx context.Context, db *sql.DB, f loadMessagesFilter) ([]messageRow, error) {
	clauses := []string{"1=1"}
	args := []any{}

	if !f.ReceivedAfter.IsZero() {
		clauses = append(clauses, "received_date_time >= ?")
		args = append(args, f.ReceivedAfter.UTC().Format(time.RFC3339))
	}
	if !f.ReceivedBefore.IsZero() {
		clauses = append(clauses, "received_date_time < ?")
		args = append(args, f.ReceivedBefore.UTC().Format(time.RFC3339))
	}
	if !f.SentAfter.IsZero() {
		clauses = append(clauses, "sent_date_time >= ?")
		args = append(args, f.SentAfter.UTC().Format(time.RFC3339))
	}
	if !f.SentBefore.IsZero() {
		clauses = append(clauses, "sent_date_time < ?")
		args = append(args, f.SentBefore.UTC().Format(time.RFC3339))
	}
	if f.IsRead != nil {
		if *f.IsRead {
			clauses = append(clauses, "is_read = 1")
		} else {
			clauses = append(clauses, "is_read = 0")
		}
	}
	if f.IsDraft != nil {
		if *f.IsDraft {
			clauses = append(clauses, "is_draft = 1")
		} else {
			clauses = append(clauses, "is_draft = 0")
		}
	}
	if f.ExcludeDrafts {
		clauses = append(clauses, "(is_draft = 0 OR is_draft IS NULL)")
	}
	if f.HasAttachments != nil {
		if *f.HasAttachments {
			clauses = append(clauses, "has_attachments = 1")
		} else {
			clauses = append(clauses, "(has_attachments = 0 OR has_attachments IS NULL)")
		}
	}
	if f.Inference != "" {
		clauses = append(clauses, "LOWER(inference_classification) = LOWER(?)")
		args = append(args, f.Inference)
	}
	if f.FlaggedOnly {
		clauses = append(clauses, "LOWER(json_extract(data, '$.flag.flagStatus')) = 'flagged'")
	}
	if f.IncompleteFlag {
		clauses = append(clauses, "LOWER(json_extract(data, '$.flag.flagStatus')) = 'flagged'")
		clauses = append(clauses, "(json_extract(data, '$.flag.completedDateTime') IS NULL OR json_extract(data, '$.flag.completedDateTime') = '')")
	}
	if len(f.Folders) > 0 {
		ph, fargs := inPlaceholders(f.Folders)
		clauses = append(clauses, "parent_folder_id IN ("+ph+")")
		args = append(args, fargs...)
	}
	if len(f.Senders) > 0 {
		// LOWER() both sides; Graph payload casing is unreliable.
		lows := make([]string, len(f.Senders))
		for i, s := range f.Senders {
			lows[i] = strings.ToLower(strings.TrimSpace(s))
		}
		ph, fargs := inPlaceholders(lows)
		clauses = append(clauses, "LOWER(json_extract(data, '$.from.emailAddress.address')) IN ("+ph+")")
		args = append(args, fargs...)
	}
	if len(f.Conversations) > 0 {
		ph, fargs := inPlaceholders(f.Conversations)
		clauses = append(clauses, "conversation_id IN ("+ph+")")
		args = append(args, fargs...)
	}

	// PATCH: validate OrderBy against an allowlist before concatenating into SQL — every current caller passes a hardcoded literal, but the allowlist closes the future risk of a config- or user-derived value reaching this string.
	order := safeOrderBy(f.OrderBy)
	limit := ""
	if f.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	q := "SELECT id, COALESCE(conversation_id,''), COALESCE(internet_message_id,''), COALESCE(parent_folder_id,''), COALESCE(subject,''), COALESCE(body_preview,''), COALESCE(importance,''), COALESCE(inference_classification,''), COALESCE(is_read,0), COALESCE(is_draft,0), COALESCE(has_attachments,0), COALESCE(received_date_time,''), COALESCE(sent_date_time,''), COALESCE(data,'{}') FROM messages WHERE " +
		strings.Join(clauses, " AND ") + " ORDER BY " + order + limit
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	out := []messageRow{}
	for rows.Next() {
		var (
			r        messageRow
			isRead   int64
			isDraft  int64
			hasAtt   int64
			recv     string
			sent     string
			dataBlob string
		)
		if err := rows.Scan(&r.ID, &r.ConversationID, &r.InternetMessageID, &r.ParentFolderID, &r.Subject, &r.BodyPreview, &r.Importance, &r.InferenceClassification, &isRead, &isDraft, &hasAtt, &recv, &sent, &dataBlob); err != nil {
			return nil, fmt.Errorf("scan messages: %w", err)
		}
		r.IsRead = isRead != 0
		r.IsDraft = isDraft != 0
		r.HasAttachments = hasAtt != 0
		if recv != "" {
			r.ReceivedAt, _ = time.Parse(time.RFC3339, recv)
		}
		if sent != "" {
			r.SentAt, _ = time.Parse(time.RFC3339, sent)
		}
		// Parse from/to/cc/categories/flag-times from the JSON blob. The typed
		// columns don't capture nested-object fields; the JSON blob is the
		// source of truth for everything else the novel commands need.
		var blob map[string]any
		if err := json.Unmarshal([]byte(dataBlob), &blob); err == nil {
			r.FromEmail, r.FromName = extractFrom(blob)
			r.ToEmails = extractRecipients(blob, "toRecipients")
			r.CcEmails = extractRecipients(blob, "ccRecipients")
			r.Categories = extractStringSlice(blob, "categories")
			r.FlagStatus, r.FlagDueAt, r.FlagStartAt, r.FlagCompletedAt = extractFlag(blob)
			if v, ok := blob["webLink"].(string); ok {
				r.WebLink = v
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func inPlaceholders(vals []string) (string, []any) {
	if len(vals) == 0 {
		return "", nil
	}
	parts := make([]string, len(vals))
	args := make([]any, len(vals))
	for i, v := range vals {
		parts[i] = "?"
		args[i] = v
	}
	return strings.Join(parts, ","), args
}

func extractFrom(blob map[string]any) (string, string) {
	from, ok := blob["from"].(map[string]any)
	if !ok {
		return "", ""
	}
	ea, ok := from["emailAddress"].(map[string]any)
	if !ok {
		return "", ""
	}
	addr, _ := ea["address"].(string)
	name, _ := ea["name"].(string)
	return strings.ToLower(addr), name
}

func extractRecipients(blob map[string]any, key string) []string {
	arr, ok := blob[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ea, ok := m["emailAddress"].(map[string]any)
		if !ok {
			continue
		}
		if a, _ := ea["address"].(string); a != "" {
			out = append(out, strings.ToLower(a))
		}
	}
	return out
}

func extractStringSlice(blob map[string]any, key string) []string {
	arr, ok := blob[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func extractFlag(blob map[string]any) (status string, due, start, completed time.Time) {
	flag, ok := blob["flag"].(map[string]any)
	if !ok {
		return
	}
	status, _ = flag["flagStatus"].(string)
	due = parseGraphDateTime(flag["dueDateTime"])
	start = parseGraphDateTime(flag["startDateTime"])
	completed = parseGraphDateTime(flag["completedDateTime"])
	return
}

// parseGraphDateTime reads either a plain ISO string or Graph's
// {dateTime, timeZone} struct. Returns zero time on any miss.
func parseGraphDateTime(v any) time.Time {
	switch x := v.(type) {
	case string:
		t, _ := time.Parse(time.RFC3339, x)
		return t
	case map[string]any:
		if s, ok := x["dateTime"].(string); ok && s != "" {
			// Graph sometimes returns without timezone offset; treat as UTC.
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				return t
			}
			if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
				return t.UTC()
			}
			if t, err := time.Parse("2006-01-02T15:04:05.0000000", s); err == nil {
				return t.UTC()
			}
		}
	}
	return time.Time{}
}

// resolveSinceWindow returns the cutoff time for a `--since` style flag.
// Accepts: RFC3339 timestamps, relative durations ("2h", "3d", "30d"),
// "last-sync" (literal — resolves via store.GetLastSyncedAt), or a date
// (YYYY-MM-DD). When the value is empty, returns (zero, nil) so callers can
// branch.
//
// PR #408 P1 lesson: never hardcode `now - 24h` for "last-sync". The
// fallback only fires when the store has no sync record at all.
func resolveSinceWindow(value string, st *store.Store, resourceType string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	now := time.Now().UTC()
	if value == "last-sync" {
		if st != nil {
			if iso := st.GetLastSyncedAt(resourceType); iso != "" {
				if t, err := time.Parse(time.RFC3339, iso); err == nil {
					return t.UTC(), nil
				}
				// fall-through to absolute parsing
			}
		}
		// fallback: 24h is honest when no sync has run; we tell the user.
		return now.Add(-24 * time.Hour), nil
	}
	// Relative duration with day/week suffix support.
	if d, ok := parseRelativeDuration(value); ok {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("--since: cannot parse %q (try RFC3339, YYYY-MM-DD, '3d', '12h', 'last-sync', or 'N hours ago')", value)
}

// parseRelativeDuration handles "30d", "12h", "45m", "3d", "2 hours ago",
// "30 days ago". Returns ok=false if it can't parse anything sensible.
func parseRelativeDuration(value string) (time.Duration, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	// strip a trailing "ago" so "2 hours ago" → "2 hours"
	if strings.HasSuffix(value, " ago") {
		value = strings.TrimSpace(strings.TrimSuffix(value, " ago"))
	}
	// "12h", "30d", "1w"
	if d, err := time.ParseDuration(value); err == nil {
		return d, true
	}
	// "30 days", "2 weeks", "5 hours", "10 minutes"
	parts := strings.Fields(value)
	if len(parts) != 2 {
		// "30d" / "2w" / "1mo"
		return parseShortDuration(value)
	}
	var n int
	if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil || n < 0 {
		return 0, false
	}
	switch parts[1] {
	case "second", "seconds", "sec", "secs":
		return time.Duration(n) * time.Second, true
	case "minute", "minutes", "min", "mins":
		return time.Duration(n) * time.Minute, true
	case "hour", "hours", "hr", "hrs":
		return time.Duration(n) * time.Hour, true
	case "day", "days":
		return time.Duration(n) * 24 * time.Hour, true
	case "week", "weeks":
		return time.Duration(n) * 7 * 24 * time.Hour, true
	}
	return 0, false
}

func parseShortDuration(s string) (time.Duration, bool) {
	if len(s) < 2 {
		return 0, false
	}
	// last 1-2 chars is unit
	var n int
	unit := ""
	for i := len(s) - 1; i > 0; i-- {
		if s[i] >= '0' && s[i] <= '9' {
			unit = s[i+1:]
			if _, err := fmt.Sscanf(s[:i+1], "%d", &n); err != nil {
				return 0, false
			}
			break
		}
	}
	if unit == "" {
		return 0, false
	}
	switch unit {
	case "s":
		return time.Duration(n) * time.Second, true
	case "m":
		return time.Duration(n) * time.Minute, true
	case "h":
		return time.Duration(n) * time.Hour, true
	case "d":
		return time.Duration(n) * 24 * time.Hour, true
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour, true
	}
	return 0, false
}

// openLocalStore opens the SQLite-backed local store. Returns a helpful error
// when no DB exists yet (the user hasn't run `sync` for the first time).
func openLocalStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("outlook-email-pp-cli")
	}
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store at %s: %w (run `outlook-email-pp-cli sync` first?)", dbPath, err)
	}
	return st, nil
}
