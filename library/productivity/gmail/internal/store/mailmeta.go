// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written data layer for the mailbox-cleanup intelligence tables
// (mail_meta, mail_sync_state, mail_meta_fts). Schema lives in extras.go;
// this file owns every read/write so the cleanup engine and novel commands
// never hand-roll SQL against these tables.

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MailMeta is one message's headers-first metadata for one account.
// InternalDate is Gmail's internalDate in milliseconds since epoch.
// LabelIDs round-trips as a JSON array in the label_ids TEXT column.
type MailMeta struct {
	Account             string   `json:"account"`
	ID                  string   `json:"id"`
	ThreadID            string   `json:"thread_id"`
	FromEmail           string   `json:"from_email"`
	FromName            string   `json:"from_name"`
	Subject             string   `json:"subject"`
	Snippet             string   `json:"snippet"`
	InternalDate        int64    `json:"internal_date"`
	SizeEstimate        int64    `json:"size_estimate"`
	LabelIDs            []string `json:"label_ids"`
	Category            string   `json:"category"`
	ListUnsubscribe     string   `json:"list_unsubscribe"`
	ListUnsubscribePost string   `json:"list_unsubscribe_post"`
	Unread              bool     `json:"unread"`
	// AuthResults holds the raw Authentication-Results header value ('' when
	// absent); the unsubscribe engine reads it for DKIM-pass verification.
	AuthResults string `json:"auth_results"`
	// ListUnsubDomain is reserved for the unsubscribe engine (extracted
	// unsubscribe-target domain); sync always writes ''.
	ListUnsubDomain string `json:"list_unsub_domain"`
}

func marshalLabelIDs(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalLabelIDs(s string) []string {
	var ids []string
	if err := json.Unmarshal([]byte(s), &ids); err != nil || ids == nil {
		return []string{}
	}
	return ids
}

// mailFTSUpsertTx replaces the FTS row for (account, id), mirroring the
// resources_fts house pattern: manual delete+insert keyed by a hashed rowid.
func mailFTSUpsertTx(tx *sql.Tx, m *MailMeta) error {
	rowid := ftsRowID(m.Account, m.ID)
	if _, err := tx.Exec(`DELETE FROM mail_meta_fts WHERE rowid = ?`, rowid); err != nil {
		return err
	}
	_, err := tx.Exec(
		`INSERT INTO mail_meta_fts (rowid, account, id, subject, from_email, from_name, snippet)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rowid, m.Account, m.ID, m.Subject, m.FromEmail, m.FromName, m.Snippet,
	)
	return err
}

// UpsertMailMeta inserts or fully replaces rows keyed by (account, id) and
// keeps mail_meta_fts in step. Returns the number of rows written.
func (s *Store) UpsertMailMeta(rows []MailMeta) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO mail_meta (
			account, id, thread_id, from_email, from_name, subject, snippet,
			internal_date, size_estimate, label_ids, category,
			list_unsubscribe, list_unsubscribe_post, unread,
			auth_results, list_unsub_domain
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, id) DO UPDATE SET
			thread_id = excluded.thread_id,
			from_email = excluded.from_email,
			from_name = excluded.from_name,
			subject = excluded.subject,
			snippet = excluded.snippet,
			internal_date = excluded.internal_date,
			size_estimate = excluded.size_estimate,
			label_ids = excluded.label_ids,
			category = excluded.category,
			list_unsubscribe = excluded.list_unsubscribe,
			list_unsubscribe_post = excluded.list_unsubscribe_post,
			unread = excluded.unread,
			auth_results = excluded.auth_results,
			list_unsub_domain = excluded.list_unsub_domain`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	written := 0
	for i := range rows {
		m := &rows[i]
		if m.Account == "" || m.ID == "" {
			return written, fmt.Errorf("mail_meta upsert: row %d missing account or id", i)
		}
		unread := 0
		if m.Unread {
			unread = 1
		}
		if _, err := stmt.Exec(
			m.Account, m.ID, m.ThreadID, m.FromEmail, m.FromName, m.Subject, m.Snippet,
			m.InternalDate, m.SizeEstimate, marshalLabelIDs(m.LabelIDs), m.Category,
			m.ListUnsubscribe, m.ListUnsubscribePost, unread,
			m.AuthResults, m.ListUnsubDomain,
		); err != nil {
			return written, fmt.Errorf("mail_meta upsert %s/%s: %w", m.Account, m.ID, err)
		}
		if err := mailFTSUpsertTx(tx, m); err != nil {
			return written, fmt.Errorf("mail_meta_fts upsert %s/%s: %w", m.Account, m.ID, err)
		}
		written++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return written, nil
}

// GetMailMeta returns one row. Propagates sql.ErrNoRows on a miss so callers
// can distinguish absence via errors.Is.
func (s *Store) GetMailMeta(account, id string) (MailMeta, error) {
	var m MailMeta
	var labelJSON string
	var unread int
	err := s.db.QueryRow(
		`SELECT account, id, thread_id, from_email, from_name, subject, snippet,
			internal_date, size_estimate, label_ids, category,
			list_unsubscribe, list_unsubscribe_post, unread,
			auth_results, list_unsub_domain
		 FROM mail_meta WHERE account = ? AND id = ?`, account, id,
	).Scan(&m.Account, &m.ID, &m.ThreadID, &m.FromEmail, &m.FromName, &m.Subject, &m.Snippet,
		&m.InternalDate, &m.SizeEstimate, &labelJSON, &m.Category,
		&m.ListUnsubscribe, &m.ListUnsubscribePost, &unread,
		&m.AuthResults, &m.ListUnsubDomain)
	if err != nil {
		return MailMeta{}, err
	}
	m.LabelIDs = unmarshalLabelIDs(labelJSON)
	m.Unread = unread != 0
	return m, nil
}

// UpdateMailLabels replaces the label set (and the label-derived columns
// unread + category) on an existing row — the incremental-sync path for
// history labelsAdded/labelsRemoved records. Returns false when no local row
// exists for (account, id); callers decide whether that warrants a fetch.
func (s *Store) UpdateMailLabels(account, id string, labelIDs []string, unread bool, category string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	unreadInt := 0
	if unread {
		unreadInt = 1
	}
	res, err := s.db.Exec(
		`UPDATE mail_meta SET label_ids = ?, unread = ?, category = ?
		 WHERE account = ? AND id = ?`,
		marshalLabelIDs(labelIDs), unreadInt, category, account, id,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// DeleteMailMeta removes rows (and their FTS shadows) for the given message
// ids. Returns how many mail_meta rows were actually deleted.
func (s *Store) DeleteMailMeta(account string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	deleted := 0
	for _, id := range ids {
		res, err := tx.Exec(`DELETE FROM mail_meta WHERE account = ? AND id = ?`, account, id)
		if err != nil {
			return 0, err
		}
		if n, err := res.RowsAffected(); err == nil {
			deleted += int(n)
		}
		if _, err := tx.Exec(`DELETE FROM mail_meta_fts WHERE rowid = ?`, ftsRowID(account, id)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deleted, nil
}

// SearchMailMeta runs an FTS match over subject/from/snippet for one account
// and returns matching rows, newest first. Foundation-level primitive for
// later offline-search commands; also exercises the FTS table in tests.
func (s *Store) SearchMailMeta(account, query string, limit int) ([]MailMeta, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT m.account, m.id, m.thread_id, m.from_email, m.from_name, m.subject, m.snippet,
			m.internal_date, m.size_estimate, m.label_ids, m.category,
			m.list_unsubscribe, m.list_unsubscribe_post, m.unread,
			m.auth_results, m.list_unsub_domain
		 FROM mail_meta m
		 JOIN mail_meta_fts f ON f.account = m.account AND f.id = m.id
		 WHERE mail_meta_fts MATCH ? AND m.account = ?
		 ORDER BY m.internal_date DESC LIMIT ?`,
		ftsMatchQuery(query), account, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailMeta
	for rows.Next() {
		var m MailMeta
		var labelJSON string
		var unread int
		if err := rows.Scan(&m.Account, &m.ID, &m.ThreadID, &m.FromEmail, &m.FromName, &m.Subject, &m.Snippet,
			&m.InternalDate, &m.SizeEstimate, &labelJSON, &m.Category,
			&m.ListUnsubscribe, &m.ListUnsubscribePost, &unread,
			&m.AuthResults, &m.ListUnsubDomain); err != nil {
			return nil, err
		}
		m.LabelIDs = unmarshalLabelIDs(labelJSON)
		m.Unread = unread != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// MailSyncState is the per-account incremental-sync cursor row.
type MailSyncState struct {
	Account         string `json:"account"`
	HistoryID       string `json:"history_id"`
	LastFullSync    string `json:"last_full_sync"`
	LastIncremental string `json:"last_incremental"`
}

// GetMailSyncState returns the cursor row for an account. A missing row is
// not an error: it returns a zero-valued state with the account filled in.
func (s *Store) GetMailSyncState(account string) (MailSyncState, error) {
	st := MailSyncState{Account: account}
	err := s.db.QueryRow(
		`SELECT history_id, last_full_sync, last_incremental FROM mail_sync_state WHERE account = ?`,
		account,
	).Scan(&st.HistoryID, &st.LastFullSync, &st.LastIncremental)
	if errors.Is(err, sql.ErrNoRows) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	return st, nil
}

// SaveMailSyncState records the post-sync historyId cursor and stamps the
// timestamp column matching mode ("full" or "incremental") with now (RFC3339
// UTC). The other timestamp is preserved.
func (s *Store) SaveMailSyncState(account, historyID, mode string) error {
	var col string
	switch mode {
	case "full":
		col = "last_full_sync"
	case "incremental":
		col = "last_incremental"
	default:
		return fmt.Errorf("unknown sync mode %q (want full|incremental)", mode)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	// col comes from the closed switch above, never from input.
	_, err := s.db.Exec(fmt.Sprintf(
		`INSERT INTO mail_sync_state (account, history_id, %s) VALUES (?, ?, ?)
		 ON CONFLICT(account) DO UPDATE SET history_id = excluded.history_id, %s = excluded.%s`,
		col, col, col), account, historyID, now)
	return err
}

// SenderStat is one from_email aggregate row for the senders command.
type SenderStat struct {
	FromEmail      string  `json:"from_email"`
	FromName       string  `json:"from_name"`
	Count          int     `json:"count"`
	TotalSize      int64   `json:"total_size"`
	UnreadCount    int     `json:"unread_count"`
	UnreadRate     float64 `json:"unread_rate"`
	FirstSeen      int64   `json:"first_seen_ms"`
	LastSeen       int64   `json:"last_seen_ms"`
	HasUnsubscribe bool    `json:"has_unsubscribe"`
}

// SenderStats aggregates mail_meta per from_email for one account, ordered by
// message count descending. category "" means all categories; sinceMs 0 means
// no lower time bound; top <= 0 falls back to 25.
func (s *Store) SenderStats(account, category string, sinceMs int64, top int) ([]SenderStat, error) {
	if top <= 0 {
		top = 25
	}
	q := strings.Builder{}
	q.WriteString(`SELECT from_email,
			MAX(from_name) AS from_name,
			COUNT(*) AS cnt,
			COALESCE(SUM(size_estimate), 0) AS total_size,
			COALESCE(SUM(unread), 0) AS unread_count,
			MIN(internal_date) AS first_seen,
			MAX(internal_date) AS last_seen,
			MAX(CASE WHEN list_unsubscribe != '' THEN 1 ELSE 0 END) AS has_unsub
		FROM mail_meta WHERE account = ?`)
	args := []any{account}
	if category != "" {
		q.WriteString(` AND category = ?`)
		args = append(args, category)
	}
	if sinceMs > 0 {
		q.WriteString(` AND internal_date >= ?`)
		args = append(args, sinceMs)
	}
	q.WriteString(` GROUP BY from_email ORDER BY cnt DESC, from_email ASC LIMIT ?`)
	args = append(args, top)

	rows, err := s.db.Query(q.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SenderStat
	for rows.Next() {
		var st SenderStat
		var hasUnsub int
		if err := rows.Scan(&st.FromEmail, &st.FromName, &st.Count, &st.TotalSize,
			&st.UnreadCount, &st.FirstSeen, &st.LastSeen, &hasUnsub); err != nil {
			return nil, err
		}
		if st.Count > 0 {
			st.UnreadRate = float64(st.UnreadCount) / float64(st.Count)
		}
		st.HasUnsubscribe = hasUnsub != 0
		out = append(out, st)
	}
	return out, rows.Err()
}

// CountMailMeta reports how many messages are stored for an account.
func (s *Store) CountMailMeta(account string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mail_meta WHERE account = ?`, account).Scan(&n)
	return n, err
}
