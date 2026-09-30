// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// The send-once ledger records every message `email send-once` delivered so
// a retry with the same idempotency key is answered locally before any API
// call. The table is created lazily by the commands that use it.
const postmarkLedgerDDL = `CREATE TABLE IF NOT EXISTS postmark_send_ledger (
	idempotency_key TEXT NOT NULL,
	message_id TEXT NOT NULL,
	recipient TEXT NOT NULL,
	server TEXT NOT NULL DEFAULT '',
	stream TEXT NOT NULL DEFAULT '',
	sandbox INTEGER NOT NULL DEFAULT 0,
	sent_at TEXT NOT NULL,
	PRIMARY KEY (idempotency_key, message_id)
)`

const postmarkLedgerIndexDDL = `CREATE INDEX IF NOT EXISTS idx_postmark_send_ledger_lookup
	ON postmark_send_ledger(idempotency_key, server, sandbox, sent_at)`

// postmarkLedgerTimeLayout is fixed-width UTC so sent_at sorts and compares
// lexically.
const postmarkLedgerTimeLayout = "2006-01-02T15:04:05.000000Z"

// PostmarkLedgerEntry is one delivered send.
type PostmarkLedgerEntry struct {
	Key       string    `json:"key"`
	MessageID string    `json:"message_id"`
	Recipient string    `json:"recipient"`
	Server    string    `json:"server"`
	Stream    string    `json:"stream"`
	Sandbox   bool      `json:"sandbox"`
	SentAt    time.Time `json:"sent_at"`
}

func (s *Store) ensurePostmarkLedger(ctx context.Context) error {
	for _, stmt := range []string{postmarkLedgerDDL, postmarkLedgerIndexDDL} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating send ledger: %w", err)
		}
	}
	return nil
}

// PostmarkLedgerLookup returns the newest ledger entry for key on server
// (sandbox sends only match sandbox sends) sent at or after since, or nil.
func (s *Store) PostmarkLedgerLookup(ctx context.Context, key, server string, sandbox bool, since time.Time) (*PostmarkLedgerEntry, error) {
	s.lockForWrite()
	err := s.ensurePostmarkLedger(ctx)
	s.unlockAfterWrite()
	if err != nil {
		return nil, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT idempotency_key, message_id, recipient, server, stream, sandbox, sent_at
		FROM postmark_send_ledger
		WHERE idempotency_key = ? AND server = ? AND sandbox = ? AND sent_at >= ?
		ORDER BY sent_at DESC LIMIT 1`,
		key, server, postmarkLedgerBool(sandbox), since.UTC().Format(postmarkLedgerTimeLayout))
	var e PostmarkLedgerEntry
	var sb int
	var sentAt string
	if err := row.Scan(&e.Key, &e.MessageID, &e.Recipient, &e.Server, &e.Stream, &sb, &sentAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading send ledger: %w", err)
	}
	e.Sandbox = sb == 1
	if t, perr := time.Parse(postmarkLedgerTimeLayout, sentAt); perr == nil {
		e.SentAt = t
	}
	return &e, nil
}

// PostmarkLedgerRecord stores a delivered send.
func (s *Store) PostmarkLedgerRecord(ctx context.Context, e PostmarkLedgerEntry) error {
	if e.Key == "" || e.MessageID == "" {
		return errors.New("send ledger entry needs a key and a message ID")
	}
	if e.SentAt.IsZero() {
		e.SentAt = time.Now()
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if err := s.ensurePostmarkLedger(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO postmark_send_ledger
		(idempotency_key, message_id, recipient, server, stream, sandbox, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Key, e.MessageID, e.Recipient, e.Server, e.Stream, postmarkLedgerBool(e.Sandbox), e.SentAt.UTC().Format(postmarkLedgerTimeLayout))
	if err != nil {
		return fmt.Errorf("writing send ledger: %w", err)
	}
	return nil
}

func postmarkLedgerBool(b bool) int {
	if b {
		return 1
	}
	return 0
}
