// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: schema and accessors for the scheduled-send queue. The Gmail
// API has no schedule-send endpoint; this local table is the whole feature.
// Lazy init via EnsureGmailSchema keeps this out of the generated migration
// slice so regeneration cannot clobber it.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ScheduledSend is one queued outbound message.
type ScheduledSend struct {
	ID          int64           `json:"id"`
	To          string          `json:"to"`
	Cc          string          `json:"cc,omitempty"`
	Bcc         string          `json:"bcc,omitempty"`
	Subject     string          `json:"subject"`
	BodyText    string          `json:"body_text,omitempty"`
	BodyHTML    string          `json:"body_html,omitempty"`
	Attachments json.RawMessage `json:"attachments,omitempty"` // JSON array of local paths
	ThreadID    string          `json:"thread_id,omitempty"`
	InReplyTo   string          `json:"in_reply_to,omitempty"`
	References  string          `json:"references,omitempty"`
	SendAt      time.Time       `json:"send_at"`
	CreatedAt   time.Time       `json:"created_at"`
	Status      string          `json:"status"` // pending | sent | canceled | failed
	SentAt      sql.NullTime    `json:"-"`
	SentAtJSON  string          `json:"sent_at,omitempty"`
	GmailID     string          `json:"gmail_message_id,omitempty"`
	// MessageIDHeader is the RFC 5322 Message-ID stamped on the outbound
	// message. Gmail indexes it, so it doubles as a crash-safe idempotency
	// key: after an interrupted attempt we can ask the server whether the
	// message actually went out instead of guessing.
	MessageIDHeader string `json:"message_id_header,omitempty"`
	Attempts        int    `json:"attempts,omitempty"`
	LastError       string `json:"error,omitempty"`
}

// EnsureGmailSchema creates the hand-authored tables if absent. Safe to call
// from every command that touches them.
func (s *Store) EnsureGmailSchema() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS scheduled_sends (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		to_addrs TEXT NOT NULL,
		cc TEXT NOT NULL DEFAULT '',
		bcc TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		body_text TEXT NOT NULL DEFAULT '',
		body_html TEXT NOT NULL DEFAULT '',
		attachments TEXT NOT NULL DEFAULT '[]',
		thread_id TEXT NOT NULL DEFAULT '',
		in_reply_to TEXT NOT NULL DEFAULT '',
		refs TEXT NOT NULL DEFAULT '',
		send_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'pending',
		sent_at DATETIME,
		gmail_message_id TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return fmt.Errorf("creating scheduled_sends table: %w", err)
	}
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_scheduled_sends_due
		ON scheduled_sends(status, send_at)`)
	if err != nil {
		return fmt.Errorf("indexing scheduled_sends: %w", err)
	}
	// claimed_at backs stale-claim recovery. Added after the initial schema,
	// so tolerate the duplicate-column error on existing databases.
	for _, stmt := range []string{
		`ALTER TABLE scheduled_sends ADD COLUMN claimed_at DATETIME`,
		`ALTER TABLE scheduled_sends ADD COLUMN message_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE scheduled_sends ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrating scheduled_sends: %w", err)
		}
	}
	return nil
}

// StaleClaimTimeout bounds how long an item may sit in 'sending' before a
// later run reclaims it. A process killed between claim and finish (cron
// timeout, crash, power loss) would otherwise strand the message forever:
// 'sending' is not 'pending', so nothing would ever retry or report it.
const StaleClaimTimeout = 15 * time.Minute

// RequeueStaleClaims returns interrupted 'sending' items to 'pending' and
// reports how many were recovered.
func (s *Store) RequeueStaleClaims(now time.Time) (int, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return 0, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE scheduled_sends
		SET status = 'pending',
		    last_error = 'previous send attempt was interrupted; requeued'
		WHERE status = 'sending'
		  AND (claimed_at IS NULL OR claimed_at <= ?)`, now.Add(-StaleClaimTimeout).UTC())
	if err != nil {
		return 0, fmt.Errorf("requeueing stale claims: %w", err)
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// CreateScheduledSend queues a message and returns its queue id.
func (s *Store) CreateScheduledSend(item ScheduledSend) (int64, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return 0, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	atts := string(item.Attachments)
	if atts == "" {
		atts = "[]"
	}
	res, err := s.db.Exec(`INSERT INTO scheduled_sends
		(to_addrs, cc, bcc, subject, body_text, body_html, attachments, thread_id, in_reply_to, refs, send_at, message_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		item.To, item.Cc, item.Bcc, item.Subject, item.BodyText, item.BodyHTML,
		atts, item.ThreadID, item.InReplyTo, item.References, item.SendAt.UTC(), item.MessageIDHeader)
	if err != nil {
		return 0, fmt.Errorf("queueing scheduled send: %w", err)
	}
	return res.LastInsertId()
}

func scanScheduledSend(rows *sql.Rows) (ScheduledSend, error) {
	var it ScheduledSend
	var atts string
	var sentAt sql.NullTime
	err := rows.Scan(&it.ID, &it.To, &it.Cc, &it.Bcc, &it.Subject, &it.BodyText,
		&it.BodyHTML, &atts, &it.ThreadID, &it.InReplyTo, &it.References,
		&it.SendAt, &it.CreatedAt, &it.Status, &sentAt, &it.GmailID, &it.LastError,
		&it.MessageIDHeader, &it.Attempts)
	if err != nil {
		return it, err
	}
	it.Attachments = json.RawMessage(atts)
	it.SentAt = sentAt
	if sentAt.Valid {
		it.SentAtJSON = sentAt.Time.UTC().Format(time.RFC3339)
	}
	return it, nil
}

const scheduledSendCols = `id, to_addrs, cc, bcc, subject, body_text, body_html,
	attachments, thread_id, in_reply_to, refs, send_at, created_at, status,
	sent_at, gmail_message_id, last_error, message_id, attempts`

// ListScheduledSends returns queue items, newest-scheduled first. status ""
// means all statuses.
func (s *Store) ListScheduledSends(status string, limit int) ([]ScheduledSend, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT ` + scheduledSendCols + ` FROM scheduled_sends`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY send_at ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing scheduled sends: %w", err)
	}
	defer rows.Close()
	var out []ScheduledSend
	for rows.Next() {
		it, err := scanScheduledSend(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning scheduled send: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// DueScheduledSends returns pending items whose send_at is at or before now.
func (s *Store) DueScheduledSends(now time.Time) ([]ScheduledSend, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT `+scheduledSendCols+` FROM scheduled_sends
		WHERE status = 'pending' AND send_at <= ? ORDER BY send_at ASC`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("querying due sends: %w", err)
	}
	defer rows.Close()
	var out []ScheduledSend
	for rows.Next() {
		it, err := scanScheduledSend(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning due send: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ClaimScheduledSend transitions pending -> sending atomically so two
// concurrent `schedule run` invocations can never double-send one item.
func (s *Store) ClaimScheduledSend(id int64) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE scheduled_sends
		SET status = 'sending', claimed_at = ?, attempts = attempts + 1
		WHERE id = ? AND status = 'pending'`, time.Now().UTC(), id)
	if err != nil {
		return false, fmt.Errorf("claiming scheduled send %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// FinishScheduledSend records the terminal outcome of a claimed item.
func (s *Store) FinishScheduledSend(id int64, gmailID string, sendErr error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if sendErr != nil {
		_, err := s.db.Exec(`UPDATE scheduled_sends
			SET status = 'failed', last_error = ? WHERE id = ?`, sendErr.Error(), id)
		return err
	}
	_, err := s.db.Exec(`UPDATE scheduled_sends
		SET status = 'sent', sent_at = ?, gmail_message_id = ?, last_error = ''
		WHERE id = ?`, time.Now().UTC(), gmailID, id)
	return err
}

// CancelScheduledSend cancels a pending item. Returns false when the item was
// not pending (already sent, failed, or missing).
func (s *Store) CancelScheduledSend(id int64) (bool, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return false, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE scheduled_sends SET status = 'canceled'
		WHERE id = ? AND status = 'pending'`, id)
	if err != nil {
		return false, fmt.Errorf("canceling scheduled send %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// UpdateScheduledSendTime moves a pending item's due time.
func (s *Store) UpdateScheduledSendTime(id int64, sendAt time.Time) (bool, error) {
	if err := s.EnsureGmailSchema(); err != nil {
		return false, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`UPDATE scheduled_sends SET send_at = ?
		WHERE id = ? AND status = 'pending'`, sendAt.UTC(), id)
	if err != nil {
		return false, fmt.Errorf("rescheduling send %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// DeferScheduledSend returns a claimed item to pending without sending it,
// recording why. Used when delivery cannot be proven safe — an inconclusive
// duplicate check must never fall through to a send.
func (s *Store) DeferScheduledSend(id int64, reason string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE scheduled_sends
		SET status = 'pending', last_error = ?
		WHERE id = ? AND status = 'sending'`, reason, id)
	if err != nil {
		return fmt.Errorf("deferring scheduled send %d: %w", id, err)
	}
	return nil
}
