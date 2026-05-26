// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(messages-readonly-chatdb): opens ~/Library/Messages/chat.db read-only
// via the file: URI prefix on modernc.org/sqlite. Without that prefix the
// driver silently drops mode=ro and returns a writable handle.

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// cocoaEpoch is the Cocoa reference date (2001-01-01 00:00:00 UTC) offset from
// Unix epoch in seconds.
const cocoaEpoch int64 = 978307200

// nanosecondMagnitude is the boundary above which a chat.db timestamp is
// nanoseconds (since macOS 10.13 / iOS 11) rather than plain seconds. The
// magnitude check is per-row, not per-database — chat_message_join.message_date
// is not uniformly backfilled to nanoseconds on every row even on modern DBs.
const nanosecondMagnitude int64 = 1_000_000_000_000

// errFDADenied is the sentinel returned when chat.db access fails with EPERM,
// which on macOS indicates Full Disk Access has not been granted to the
// running process. The doctor command checks for this with errors.Is.
var errFDADenied = errors.New("chat.db cannot be read: Full Disk Access not granted")

// Tapback associated_message_type values. Rows whose associated_message_guid
// is non-null and type falls in this range are reactions, not real messages,
// and are filtered from list/search output by default. The "removed" variants
// live at the upper end of the range.
const (
	tapbackTypeMin = 2000
	tapbackTypeMax = 3005
)

// ── types ─────────────────────────────────────────────────────────────────────

// ChatRow is one row from the chat table augmented with participant + message
// statistics that the subcommands surface together.
type ChatRow struct {
	ROWID            int64      `json:"rowid"`
	GUID             string     `json:"guid"`
	ChatIdentifier   string     `json:"chat_identifier"`
	DisplayName      string     `json:"display_name,omitempty"`
	Style            int        `json:"style"`
	ParticipantCount int        `json:"participants"`
	MessageCount     int64      `json:"message_count"`
	LastMessageDate  *time.Time `json:"last_message_date,omitempty"`
	LastPreview      string     `json:"last_preview,omitempty"`
	IsGroup          bool       `json:"is_group"`
}

// MessageRow is one row from the message table with text decoded from
// attributedBody when message.text is NULL.
type MessageRow struct {
	ROWID            int64      `json:"rowid"`
	GUID             string     `json:"guid"`
	ChatGUID         string     `json:"chat_guid,omitempty"`
	ChatDisplayName  string     `json:"chat_display_name,omitempty"`
	HandleID         *int64     `json:"handle_id,omitempty"`
	HandleAddress    string     `json:"handle,omitempty"`
	IsFromMe         bool       `json:"is_from_me"`
	Date             time.Time  `json:"date"`
	DateEdited       *time.Time `json:"date_edited,omitempty"`
	Text             string     `json:"text"`
	TextSource       string     `json:"text_source"`
	HasAttachments   bool       `json:"has_attachments"`
	AssociatedType   *int       `json:"associated_type,omitempty"`
}

// HandleRow is one row from the handle table.
type HandleRow struct {
	ROWID   int64  `json:"rowid"`
	Address string `json:"address"`
	Service string `json:"service"`
}

// AttachmentRow is one row from the attachment table joined with the message.
type AttachmentRow struct {
	ROWID         int64  `json:"rowid"`
	Filename      string `json:"filename,omitempty"`
	ResolvedPath  string `json:"resolved_path,omitempty"`
	MIMEType      string `json:"mime_type,omitempty"`
	TransferState int    `json:"transfer_state"`
	Missing       bool   `json:"missing,omitempty"`
}

// YearStats is one row of the by-year stats breakdown.
type YearStats struct {
	Year         string `json:"year"`
	MessageCount int64  `json:"message_count"`
}

// HandleStats is one row of the top-handles stats breakdown.
type HandleStats struct {
	Handle       string `json:"handle"`
	MessageCount int64  `json:"message_count"`
}

// MessagesTotals summarises the corpus.
type MessagesTotals struct {
	TotalMessages int64 `json:"total_messages"`
	TotalChats    int64 `json:"total_chats"`
	TotalHandles  int64 `json:"total_handles"`
}

// ── options ───────────────────────────────────────────────────────────────────

// ChatListOpts controls listChats query behavior.
type ChatListOpts struct {
	Limit        int
	Since        *time.Time
	IncludeEmpty bool
}

// SearchOpts controls searchMessages query behavior.
type SearchOpts struct {
	Query           string
	ChatFilter      string // GUID or chat_identifier
	HandleFilter    string // address (phone/email)
	FromMe          *bool
	Since           *time.Time
	Until           *time.Time
	Limit           int
	IncludeTapbacks bool
}

// MessageWindowOpts controls messagesForChat query behavior.
type MessageWindowOpts struct {
	Since           *time.Time
	Until           *time.Time
	Limit           int
	IncludeTapbacks bool
}

// ── open + helpers ────────────────────────────────────────────────────────────

func defaultMessagesDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Messages", "chat.db")
}

func defaultAttachmentsRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Messages", "Attachments")
}

// openMessagesDB opens chat.db read-only. Returns errFDADenied (wrapped) when
// the OS reports EPERM, which on macOS indicates Full Disk Access has not been
// granted to the running process.
func openMessagesDB(dbPath string) (*sql.DB, error) {
	if runtime.GOOS != "darwin" {
		return nil, configErr(fmt.Errorf(
			"icloud-pp-cli messages requires macOS — this is %s", runtime.GOOS,
		))
	}
	if dbPath == "" {
		dbPath = defaultMessagesDBPath()
	}
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf(
				"chat.db not found at %s\n\nOpen Messages.app once to initialize the database, or use --messages-db to specify a path.",
				dbPath,
			)
		}
		if isPermissionError(err) {
			return nil, configErr(fmt.Errorf("%w: %s", errFDADenied, dbPath))
		}
		return nil, err
	}
	u := &url.URL{
		Scheme:   "file",
		Path:     dbPath,
		RawQuery: "mode=ro&_busy_timeout=5000&_query_only=1",
	}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("cannot open chat.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		if isPermissionError(err) {
			return nil, configErr(fmt.Errorf("%w: %s", errFDADenied, dbPath))
		}
		return nil, fmt.Errorf("cannot read chat.db: %w", err)
	}
	return db, nil
}

// isPermissionError detects access denials surfaced by the OS or the SQLite
// driver. macOS TCC denials don't always reach Go as EPERM — modernc.org/sqlite
// often reports them as SQLITE_CANTOPEN ("unable to open database file") or a
// surrogate code like "out of memory" because the OS withholds enough state
// from the driver to distinguish the cause. Treat any open-side failure as a
// likely permission issue when chat.db itself exists.
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPERM) || errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"operation not permitted",
		"permission denied",
		"unable to open database file",
		"out of memory (14)", // SQLITE_CANTOPEN surrogate
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// cocoaToUnix converts a chat.db timestamp to time.Time. The encoding is
// either plain seconds or nanoseconds since the Cocoa epoch (2001-01-01 UTC),
// determined per-row by magnitude — chat_message_join.message_date is not
// uniformly backfilled to nanoseconds even on modern databases (bagoup #40).
func cocoaToUnix(raw int64) time.Time {
	if raw == 0 {
		return time.Time{}
	}
	seconds := raw
	if raw >= nanosecondMagnitude {
		seconds = raw / 1_000_000_000
	}
	return time.Unix(seconds+cocoaEpoch, 0).UTC()
}

// nullStringToValue returns the dereferenced string or empty.
func nullStringToValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullInt64ToPtr returns a pointer to the int64 value or nil.
func nullInt64ToPtr(ni sql.NullInt64) *int64 {
	if ni.Valid {
		v := ni.Int64
		return &v
	}
	return nil
}

// nullIntToPtr returns a pointer to int or nil.
func nullIntToPtr(ni sql.NullInt64) *int {
	if ni.Valid {
		v := int(ni.Int64)
		return &v
	}
	return nil
}

// extractMessageText returns the message body from the text column when
// non-null, falling back to the attributedBody decoder, with the source
// classification surfaced to callers.
func extractMessageText(text sql.NullString, attributedBody []byte) (string, string) {
	if text.Valid && text.String != "" {
		return text.String, textSourceTextColumn
	}
	if len(attributedBody) > 0 {
		decoded, source := decodeAttributedBody(attributedBody)
		if source == textSourceDecoded {
			return decoded, textSourceDecoded
		}
	}
	if text.Valid {
		// text column existed but was empty string
		return text.String, textSourceTextColumn
	}
	return "", textSourceUnrecoverable
}

// ── queries ───────────────────────────────────────────────────────────────────

// listChats returns chats ordered by most-recent activity. Group vs DM is
// determined by participant count (chat_handle_join), not chat.style, because
// chat.style has been observed to drift across schema generations.
func listChats(db *sql.DB, opts ChatListOpts) ([]ChatRow, error) {
	var whereLastDate string
	args := []any{}
	if opts.Since != nil {
		// PATCH(messages-list-chats-since-nanosecond-conversion): last_msg.last_date
		// is MAX(m.date) which is nanoseconds-since-Cocoa-epoch on modern macOS.
		// Convert the user-supplied seconds-domain threshold to nanoseconds so
		// the comparison meets in the same unit. Matches searchMessages's pattern.
		whereLastDate = " AND last_msg.last_date >= ?"
		args = append(args, (opts.Since.Unix()-cocoaEpoch)*1_000_000_000)
	}
	if !opts.IncludeEmpty {
		whereLastDate += " AND last_msg.last_date IS NOT NULL"
	}

	q := fmt.Sprintf(`
		SELECT
			c.ROWID,
			COALESCE(c.guid, ''),
			COALESCE(c.chat_identifier, ''),
			c.display_name,
			COALESCE(c.style, 0),
			COALESCE(participants.cnt, 0),
			COALESCE(last_msg.msg_count, 0),
			last_msg.last_date
		FROM chat c
		LEFT JOIN (
			SELECT chat_id, COUNT(handle_id) AS cnt
			FROM chat_handle_join
			GROUP BY chat_id
		) participants ON participants.chat_id = c.ROWID
		LEFT JOIN (
			SELECT cmj.chat_id,
				COUNT(*) AS msg_count,
				MAX(m.date) AS last_date
			FROM chat_message_join cmj
			JOIN message m ON m.ROWID = cmj.message_id
			WHERE m.associated_message_guid IS NULL
			GROUP BY cmj.chat_id
		) last_msg ON last_msg.chat_id = c.ROWID
		WHERE 1 = 1%s
		ORDER BY last_msg.last_date DESC NULLS LAST
	`, whereLastDate)

	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()

	var out []ChatRow
	for rows.Next() {
		var c ChatRow
		var displayName sql.NullString
		var lastDate sql.NullInt64
		if err := rows.Scan(
			&c.ROWID,
			&c.GUID,
			&c.ChatIdentifier,
			&displayName,
			&c.Style,
			&c.ParticipantCount,
			&c.MessageCount,
			&lastDate,
		); err != nil {
			return nil, fmt.Errorf("scan chat row: %w", err)
		}
		c.DisplayName = nullStringToValue(displayName)
		if lastDate.Valid {
			t := cocoaToUnix(lastDate.Int64)
			c.LastMessageDate = &t
		}
		c.IsGroup = c.ParticipantCount > 1
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fillLastPreviews populates the LastPreview field on each chat by fetching
// the most recent non-tapback message and decoding its body. Done in a
// follow-up pass so listChats stays a single aggregate query rather than a
// correlated subquery per row.
func fillLastPreviews(db *sql.DB, chats []ChatRow, maxLen int) error {
	for i := range chats {
		if chats[i].MessageCount == 0 {
			continue
		}
		text, source, err := fetchLastMessageText(db, chats[i].ROWID)
		if err != nil {
			return err
		}
		_ = source
		if text == "" {
			continue
		}
		// PATCH(messages-preview-rune-aware-truncate): slice at rune boundaries
		// so emoji and CJK previews don't end on a half rune and emit �.
		if maxLen > 0 {
			if runes := []rune(text); len(runes) > maxLen {
				text = string(runes[:maxLen]) + "..."
			}
		}
		chats[i].LastPreview = text
	}
	return nil
}

func fetchLastMessageText(db *sql.DB, chatROWID int64) (string, string, error) {
	q := `
		SELECT m.text, m.attributedBody
		FROM chat_message_join cmj
		JOIN message m ON m.ROWID = cmj.message_id
		WHERE cmj.chat_id = ?
		  AND m.associated_message_guid IS NULL
		ORDER BY m.date DESC
		LIMIT 1
	`
	row := db.QueryRow(q, chatROWID)
	var text sql.NullString
	var attrBody []byte
	if err := row.Scan(&text, &attrBody); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", textSourceUnrecoverable, nil
		}
		return "", textSourceUnrecoverable, err
	}
	body, source := extractMessageText(text, attrBody)
	return body, source, nil
}

// searchMessages performs a text-content search with optional filters. Returns
// at most opts.Limit rows. Two-phase: SQL LIKE matches text-column hits
// efficiently, then a second pass scans attributedBody-only rows when
// opts.Query is set so post-Big-Sur messages aren't silently missed.
func searchMessages(db *sql.DB, opts SearchOpts) ([]MessageRow, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	conds := []string{"1 = 1"}
	args := []any{}

	if !opts.IncludeTapbacks {
		conds = append(conds, "m.associated_message_guid IS NULL")
	}

	if opts.ChatFilter != "" {
		conds = append(conds, "(c.guid = ? OR c.chat_identifier = ?)")
		args = append(args, opts.ChatFilter, opts.ChatFilter)
	}
	if opts.HandleFilter != "" {
		conds = append(conds, "h.id = ?")
		args = append(args, opts.HandleFilter)
	}
	if opts.FromMe != nil {
		v := 0
		if *opts.FromMe {
			v = 1
		}
		conds = append(conds, "m.is_from_me = ?")
		args = append(args, v)
	}
	if opts.Since != nil {
		conds = append(conds, "m.date >= ?")
		args = append(args, (opts.Since.Unix()-cocoaEpoch)*1_000_000_000)
	}
	if opts.Until != nil {
		conds = append(conds, "m.date <= ?")
		args = append(args, (opts.Until.Unix()-cocoaEpoch)*1_000_000_000)
	}

	// SQL LIKE phase. Match against text column only; attributedBody hits
	// are caught in the post-scan filter below.
	//
	// PATCH(messages-like-escape-clause): escapeLike writes \% and \_ into the
	// pattern but the LIKE expression must carry an explicit ESCAPE clause for
	// SQLite to honor the backslash as an escape character. Without it, queries
	// containing literal % or _ silently miss rows from the SQL phase.
	textCond := ""
	if opts.Query != "" {
		textCond = ` AND (m.text LIKE ? COLLATE NOCASE ESCAPE '\' OR m.text IS NULL)`
		args = append(args, "%"+escapeLike(opts.Query)+"%")
	}

	q := fmt.Sprintf(`
		SELECT
			m.ROWID, COALESCE(m.guid, ''),
			COALESCE(c.guid, ''), c.display_name,
			m.handle_id, h.id,
			COALESCE(m.is_from_me, 0),
			COALESCE(m.date, 0),
			m.date_edited,
			m.text, m.attributedBody,
			COALESCE(m.cache_has_attachments, 0),
			m.associated_message_type
		FROM message m
		LEFT JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
		LEFT JOIN chat c ON c.ROWID = cmj.chat_id
		LEFT JOIN handle h ON h.ROWID = m.handle_id
		WHERE %s%s
		ORDER BY m.date DESC
		LIMIT %d
	`, strings.Join(conds, " AND "), textCond, opts.Limit*4) // overscan to allow filtering

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()

	var out []MessageRow
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		if opts.Query != "" && !strings.Contains(strings.ToLower(m.Text), strings.ToLower(opts.Query)) {
			continue
		}
		out = append(out, m)
		if len(out) >= opts.Limit {
			break
		}
	}
	return out, rows.Err()
}

// escapeLike escapes the SQL LIKE metacharacters in a user query. The returned
// string contains backslash-prefixed `\%`, `\_`, and `\\` sequences and MUST
// be paired with an explicit `ESCAPE '\'` clause in the LIKE expression for
// SQLite to honor the escapes. Without the ESCAPE clause SQLite treats the
// backslash as a literal character and the intended escaping does not fire.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)
	return s
}

// scanMessageRow scans a single row from the search/window query into a
// MessageRow with text decoded.
func scanMessageRow(rows *sql.Rows) (MessageRow, error) {
	var m MessageRow
	var chatDisplay sql.NullString
	var handleID sql.NullInt64
	var handleAddr sql.NullString
	var isFromMe int
	var dateRaw int64
	var dateEdited sql.NullInt64
	var text sql.NullString
	var attrBody []byte
	var hasAttach int
	var assocType sql.NullInt64

	if err := rows.Scan(
		&m.ROWID, &m.GUID,
		&m.ChatGUID, &chatDisplay,
		&handleID, &handleAddr,
		&isFromMe,
		&dateRaw,
		&dateEdited,
		&text, &attrBody,
		&hasAttach,
		&assocType,
	); err != nil {
		return m, fmt.Errorf("scan message row: %w", err)
	}
	m.ChatDisplayName = nullStringToValue(chatDisplay)
	m.HandleID = nullInt64ToPtr(handleID)
	m.HandleAddress = nullStringToValue(handleAddr)
	m.IsFromMe = isFromMe != 0
	m.Date = cocoaToUnix(dateRaw)
	if dateEdited.Valid && dateEdited.Int64 != 0 {
		t := cocoaToUnix(dateEdited.Int64)
		m.DateEdited = &t
	}
	m.HasAttachments = hasAttach != 0
	m.AssociatedType = nullIntToPtr(assocType)
	m.Text, m.TextSource = extractMessageText(text, attrBody)
	return m, nil
}

// messagesForChat returns the message history for a chat in chronological order.
func messagesForChat(db *sql.DB, chatROWID int64, opts MessageWindowOpts) ([]MessageRow, error) {
	conds := []string{"cmj.chat_id = ?"}
	args := []any{chatROWID}
	if !opts.IncludeTapbacks {
		conds = append(conds, "m.associated_message_guid IS NULL")
	}
	if opts.Since != nil {
		conds = append(conds, "m.date >= ?")
		args = append(args, (opts.Since.Unix()-cocoaEpoch)*1_000_000_000)
	}
	if opts.Until != nil {
		conds = append(conds, "m.date <= ?")
		args = append(args, (opts.Until.Unix()-cocoaEpoch)*1_000_000_000)
	}

	limit := ""
	if opts.Limit > 0 {
		limit = fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	q := fmt.Sprintf(`
		SELECT
			m.ROWID, COALESCE(m.guid, ''),
			COALESCE(c.guid, ''), c.display_name,
			m.handle_id, h.id,
			COALESCE(m.is_from_me, 0),
			COALESCE(m.date, 0),
			m.date_edited,
			m.text, m.attributedBody,
			COALESCE(m.cache_has_attachments, 0),
			m.associated_message_type
		FROM message m
		JOIN chat_message_join cmj ON cmj.message_id = m.ROWID
		LEFT JOIN chat c ON c.ROWID = cmj.chat_id
		LEFT JOIN handle h ON h.ROWID = m.handle_id
		WHERE %s
		ORDER BY m.date ASC%s
	`, strings.Join(conds, " AND "), limit)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("messages for chat: %w", err)
	}
	defer rows.Close()

	var out []MessageRow
	for rows.Next() {
		m, err := scanMessageRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// chatByIdentifier resolves a chat by GUID or chat_identifier (which is the
// underlying phone/email for DMs or "chat<n>" for groups).
func chatByIdentifier(db *sql.DB, ident string) (ChatRow, error) {
	q := `
		SELECT c.ROWID, COALESCE(c.guid, ''), COALESCE(c.chat_identifier, ''),
		       c.display_name, COALESCE(c.style, 0),
		       COALESCE((SELECT COUNT(*) FROM chat_handle_join chj WHERE chj.chat_id = c.ROWID), 0)
		FROM chat c
		WHERE c.guid = ? OR c.chat_identifier = ?
		LIMIT 1
	`
	row := db.QueryRow(q, ident, ident)
	var c ChatRow
	var displayName sql.NullString
	if err := row.Scan(&c.ROWID, &c.GUID, &c.ChatIdentifier, &displayName, &c.Style, &c.ParticipantCount); err != nil {
		return c, err
	}
	c.DisplayName = nullStringToValue(displayName)
	c.IsGroup = c.ParticipantCount > 1
	return c, nil
}

// attachmentsForMessage returns attachment rows for a message with resolved
// filesystem paths and a missing flag.
func attachmentsForMessage(db *sql.DB, messageROWID int64) ([]AttachmentRow, error) {
	q := `
		SELECT a.ROWID, a.filename, a.mime_type, COALESCE(a.transfer_state, 0)
		FROM attachment a
		JOIN message_attachment_join maj ON maj.attachment_id = a.ROWID
		WHERE maj.message_id = ?
	`
	rows, err := db.Query(q, messageROWID)
	if err != nil {
		return nil, fmt.Errorf("attachments for message: %w", err)
	}
	defer rows.Close()

	home, _ := os.UserHomeDir()
	var out []AttachmentRow
	for rows.Next() {
		var a AttachmentRow
		var filename, mime sql.NullString
		if err := rows.Scan(&a.ROWID, &filename, &mime, &a.TransferState); err != nil {
			return nil, fmt.Errorf("scan attachment row: %w", err)
		}
		a.Filename = nullStringToValue(filename)
		a.MIMEType = nullStringToValue(mime)
		if a.Filename != "" {
			path := a.Filename
			if strings.HasPrefix(path, "~/") && home != "" {
				path = filepath.Join(home, path[2:])
			}
			a.ResolvedPath = path
			if _, err := os.Stat(path); err != nil {
				a.Missing = true
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// statsTotals returns the top-level row counts for the corpus.
func statsTotals(db *sql.DB, includeTapbacks bool) (MessagesTotals, error) {
	var t MessagesTotals
	cond := ""
	if !includeTapbacks {
		cond = " WHERE associated_message_guid IS NULL"
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM message" + cond).Scan(&t.TotalMessages); err != nil {
		return t, fmt.Errorf("stats totals (messages): %w", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM chat").Scan(&t.TotalChats); err != nil {
		return t, fmt.Errorf("stats totals (chats): %w", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM handle").Scan(&t.TotalHandles); err != nil {
		return t, fmt.Errorf("stats totals (handles): %w", err)
	}
	return t, nil
}

// statsByYear groups message counts by year. Cocoa-epoch dates are converted
// to Unix seconds in SQL via the magnitude check (nanoseconds vs seconds).
func statsByYear(db *sql.DB, includeTapbacks bool) ([]YearStats, error) {
	cond := ""
	if !includeTapbacks {
		cond = " AND associated_message_guid IS NULL"
	}
	q := fmt.Sprintf(`
		SELECT
			strftime('%%Y', datetime(
				CASE WHEN date >= %d
					 THEN date / 1000000000 + %d
					 ELSE date + %d
				END,
				'unixepoch'
			)) AS yr,
			COUNT(*)
		FROM message
		WHERE date > 0%s
		GROUP BY yr
		ORDER BY yr DESC
	`, nanosecondMagnitude, cocoaEpoch, cocoaEpoch, cond)

	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("stats by year: %w", err)
	}
	defer rows.Close()

	var out []YearStats
	for rows.Next() {
		var y YearStats
		if err := rows.Scan(&y.Year, &y.MessageCount); err != nil {
			return nil, fmt.Errorf("scan year row: %w", err)
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

// statsByHandle returns the top N handles by message count.
func statsByHandle(db *sql.DB, top int, includeTapbacks bool) ([]HandleStats, error) {
	if top <= 0 {
		top = 10
	}
	cond := ""
	if !includeTapbacks {
		cond = " AND m.associated_message_guid IS NULL"
	}
	q := fmt.Sprintf(`
		SELECT COALESCE(h.id, '(me)'), COUNT(*) AS cnt
		FROM message m
		LEFT JOIN handle h ON h.ROWID = m.handle_id
		WHERE 1 = 1%s
		GROUP BY h.id
		ORDER BY cnt DESC
		LIMIT %d
	`, cond, top)

	rows, err := db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("stats by handle: %w", err)
	}
	defer rows.Close()

	var out []HandleStats
	for rows.Next() {
		var h HandleStats
		if err := rows.Scan(&h.Handle, &h.MessageCount); err != nil {
			return nil, fmt.Errorf("scan handle row: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// tapbackTypeRange returns the documented inclusive range for tapback rows.
// v1 queries filter tapbacks via `associated_message_guid IS NULL`, which is
// the cheaper shortcut; this helper exists so future surfaces that want to
// distinguish tapbacks from regular associated messages have a single source
// of truth for the range.
func tapbackTypeRange() (int, int) { return tapbackTypeMin, tapbackTypeMax }
