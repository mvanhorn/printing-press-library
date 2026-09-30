// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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
	PRIMARY KEY (idempotency_key, server, sandbox, message_id)
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

// PostmarkLedgerPendingPrefix marks a reserved send whose delivery has not
// been confirmed. Each reservation gets a unique ID after the prefix, so two
// scopes or two attempts never share a row. A pending row blocks the key for
// the rest of its window: if the process died after Postmark accepted the
// message, sending again would deliver a second copy.
const PostmarkLedgerPendingPrefix = "pending:"

// Pending reports whether the entry is a reservation rather than a confirmed
// delivery.
func (e PostmarkLedgerEntry) Pending() bool {
	return strings.HasPrefix(e.MessageID, PostmarkLedgerPendingPrefix)
}

// PostmarkLedgerReserve atomically claims key on server before a send. When
// an entry sent or reserved within window exists it is returned and nothing
// is inserted (the caller must not send). Otherwise a pending row is inserted
// and its reservation ID returned for PostmarkLedgerComplete or
// PostmarkLedgerRelease. The read-write DSN opens transactions with BEGIN
// IMMEDIATE, so two processes racing on the same key serialize here and only
// one reserves it. The reservation time and window cutoff are taken after the
// write lock is held, so waiting for the lock cannot age the reservation out
// of a short window.
func (s *Store) PostmarkLedgerReserve(ctx context.Context, e PostmarkLedgerEntry, window time.Duration) (string, *PostmarkLedgerEntry, error) {
	if e.Key == "" || window <= 0 {
		return "", nil, errors.New("send ledger reservation needs a key and a positive window")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, fmt.Errorf("reserving send ledger key: %w", err)
	}
	reservation := PostmarkLedgerPendingPrefix + hex.EncodeToString(nonce[:])
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if err := s.ensurePostmarkLedger(ctx); err != nil {
		return "", nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("reserving send ledger key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	e.SentAt = time.Now()
	since := e.SentAt.Add(-window)
	row := tx.QueryRowContext(ctx, `SELECT idempotency_key, message_id, recipient, server, stream, sandbox, sent_at
		FROM postmark_send_ledger
		WHERE idempotency_key = ? AND server = ? AND sandbox = ? AND sent_at >= ?
		ORDER BY sent_at DESC LIMIT 1`,
		e.Key, e.Server, postmarkLedgerBool(e.Sandbox), since.UTC().Format(postmarkLedgerTimeLayout))
	var prior PostmarkLedgerEntry
	var sb int
	var sentAt string
	switch err := row.Scan(&prior.Key, &prior.MessageID, &prior.Recipient, &prior.Server, &prior.Stream, &sb, &sentAt); {
	case err == nil:
		prior.Sandbox = sb == 1
		if t, perr := time.Parse(postmarkLedgerTimeLayout, sentAt); perr == nil {
			prior.SentAt = t
		}
		return "", &prior, nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", nil, fmt.Errorf("reading send ledger: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO postmark_send_ledger
		(idempotency_key, message_id, recipient, server, stream, sandbox, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Key, reservation, e.Recipient, e.Server, e.Stream, postmarkLedgerBool(e.Sandbox), e.SentAt.UTC().Format(postmarkLedgerTimeLayout)); err != nil {
		return "", nil, fmt.Errorf("reserving send ledger key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("reserving send ledger key: %w", err)
	}
	return reservation, nil, nil
}

// PostmarkLedgerComplete replaces reservation with the MessageID Postmark
// returned. Only the attempt that made the reservation can complete it.
func (s *Store) PostmarkLedgerComplete(ctx context.Context, reservation, messageID string) error {
	if !strings.HasPrefix(reservation, PostmarkLedgerPendingPrefix) || messageID == "" {
		return errors.New("send ledger completion needs a reservation and a message ID")
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	res, err := s.db.ExecContext(ctx, `UPDATE postmark_send_ledger SET message_id = ? WHERE message_id = ?`, messageID, reservation)
	if err != nil {
		return fmt.Errorf("writing send ledger: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("writing send ledger: reservation %s not found", reservation)
	}
	return nil
}

// PostmarkLedgerRelease drops reservation after Postmark definitely refused
// the send, so a corrected retry is not blocked. Ambiguous outcomes (timeouts,
// 5xx, malformed responses) must keep the reservation instead.
func (s *Store) PostmarkLedgerRelease(ctx context.Context, reservation string) error {
	if !strings.HasPrefix(reservation, PostmarkLedgerPendingPrefix) {
		return errors.New("send ledger release needs a reservation")
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, `DELETE FROM postmark_send_ledger WHERE message_id = ?`, reservation); err != nil {
		return fmt.Errorf("releasing send ledger key: %w", err)
	}
	return nil
}

func postmarkLedgerBool(b bool) int {
	if b {
		return 1
	}
	return 0
}
