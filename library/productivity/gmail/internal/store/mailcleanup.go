// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written data layer for the cleanup engine's durability tables
// (mail_nonces, mail_applies, mail_apply_chunks, mail_ledger,
// mail_ledger_entries). Schema lives in extras.go; this file owns every
// read/write so the engine never hand-rolls SQL against these tables.

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Nonce/authorization sentinel errors. The CLI maps every one of these to
// the typed "refused" exit (4); they are distinct so the refusal message
// can say exactly why the token was rejected.
var (
	ErrNonceUnknown  = errors.New("token not recognized")
	ErrNonceUsed     = errors.New("token already used")
	ErrNonceExpired  = errors.New("token expired")
	ErrNonceMismatch = errors.New("token is not bound to this plan and account")
)

// Apply lifecycle states.
const (
	MailApplyStateAuthorized = "authorized"
	MailApplyStateApplying   = "applying"
	MailApplyStateDone       = "done"
	MailApplyStatePartial    = "partial"
	MailApplyStateRefused    = "refused"
	MailApplyStateAbandoned  = "abandoned"
)

// Chunk lifecycle states.
const (
	MailChunkStatePending  = "pending"
	MailChunkStateApplying = "applying"
	MailChunkStateDone     = "done"
)

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// CreateMailNonce records a freshly minted one-time apply token bound to
// (planSha, account) with an absolute expiry.
func (s *Store) CreateMailNonce(nonce, planSha, account string, expiresAt time.Time) error {
	if nonce == "" || planSha == "" || account == "" {
		return fmt.Errorf("mail_nonces insert: nonce, plan_sha, and account are all required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO mail_nonces (nonce, plan_sha, account, expires_at, used, created_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		nonce, planSha, account, expiresAt.UTC().Format(time.RFC3339), nowRFC3339(),
	)
	return err
}

// MailApply is one row of mail_applies.
type MailApply struct {
	ID        int64  `json:"id"`
	PlanSha   string `json:"plan_sha"`
	Account   string `json:"account"`
	Action    string `json:"action"`
	State     string `json:"state"`
	LedgerID  string `json:"ledger_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// AuthorizeMailApply is the single atomic authorization step (grill R3-C2):
// in ONE SQLite transaction it (a) validates the nonce — exists, unused,
// unexpired, bound to exactly this plan_sha and account — (b) marks it
// used, and (c) inserts the mail_applies record in state 'authorized'.
// A crash after this transaction is recoverable: the burned nonce and the
// authorized apply row commit or roll back together, never separately.
func (s *Store) AuthorizeMailApply(nonce, planSha, account, action string, now time.Time) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var gotSha, gotAccount, expiresAt string
	var used int
	err = tx.QueryRow(
		`SELECT plan_sha, account, expires_at, used FROM mail_nonces WHERE nonce = ?`, nonce,
	).Scan(&gotSha, &gotAccount, &expiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNonceUnknown
	}
	if err != nil {
		return 0, err
	}
	if used != 0 {
		return 0, ErrNonceUsed
	}
	if exp, perr := time.Parse(time.RFC3339, expiresAt); perr != nil || now.UTC().After(exp) {
		return 0, ErrNonceExpired
	}
	if gotSha != planSha || gotAccount != account {
		return 0, ErrNonceMismatch
	}

	res, err := tx.Exec(`UPDATE mail_nonces SET used = 1 WHERE nonce = ? AND used = 0`, nonce)
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		// A concurrent authorization won the race between the SELECT above
		// and this UPDATE. Treat exactly like an already-used token.
		if err != nil {
			return 0, err
		}
		return 0, ErrNonceUsed
	}

	ts := now.UTC().Format(time.RFC3339)
	ins, err := tx.Exec(
		`INSERT INTO mail_applies (plan_sha, account, action, state, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		planSha, account, action, MailApplyStateAuthorized, ts, ts,
	)
	if err != nil {
		return 0, err
	}
	applyID, err := ins.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return applyID, nil
}

// GetMailApply returns one apply row. Propagates sql.ErrNoRows on a miss.
func (s *Store) GetMailApply(applyID int64) (MailApply, error) {
	var a MailApply
	err := s.db.QueryRow(
		`SELECT id, plan_sha, account, action, state, ledger_id, created_at, updated_at
		 FROM mail_applies WHERE id = ?`, applyID,
	).Scan(&a.ID, &a.PlanSha, &a.Account, &a.Action, &a.State, &a.LedgerID, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// SetMailApplyState transitions an apply row's lifecycle state.
func (s *Store) SetMailApplyState(applyID int64, state string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`UPDATE mail_applies SET state = ?, updated_at = ? WHERE id = ?`,
		state, nowRFC3339(), applyID,
	)
	return err
}

// SetMailApplyLedger records which ledger an apply wrote its deltas into.
func (s *Store) SetMailApplyLedger(applyID int64, ledgerID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`UPDATE mail_applies SET ledger_id = ?, updated_at = ? WHERE id = ?`,
		ledgerID, nowRFC3339(), applyID,
	)
	return err
}

// ListMailApplyLeftovers returns every apply row in a non-terminal state
// (authorized, applying, partial) — the set `cleanup recover` reconciles
// and a fresh apply refuses to run past.
func (s *Store) ListMailApplyLeftovers() ([]MailApply, error) {
	rows, err := s.db.Query(
		`SELECT id, plan_sha, account, action, state, ledger_id, created_at, updated_at
		 FROM mail_applies WHERE state IN (?, ?, ?) ORDER BY id ASC`,
		MailApplyStateAuthorized, MailApplyStateApplying, MailApplyStatePartial,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailApply
	for rows.Next() {
		var a MailApply
		if err := rows.Scan(&a.ID, &a.PlanSha, &a.Account, &a.Action, &a.State, &a.LedgerID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MailApplyChunk is one durable mutation-intent row.
type MailApplyChunk struct {
	ApplyID int64    `json:"apply_id"`
	ChunkNo int      `json:"chunk_no"`
	Kind    string   `json:"kind"` // trash | label
	IDs     []string `json:"ids"`
	Add     []string `json:"add"`
	Remove  []string `json:"remove"`
	State   string   `json:"state"`
}

// InsertMailApplyChunks durably records every chunk's intent (state
// 'pending') in one transaction, BEFORE any network mutation is issued.
func (s *Store) InsertMailApplyChunks(chunks []MailApplyChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(
		`INSERT INTO mail_apply_chunks (apply_id, chunk_no, kind, ids, add_labels, remove_labels, state, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	ts := nowRFC3339()
	for _, ch := range chunks {
		state := ch.State
		if state == "" {
			state = MailChunkStatePending
		}
		if _, err := stmt.Exec(ch.ApplyID, ch.ChunkNo, ch.Kind,
			marshalLabelIDs(ch.IDs), marshalLabelIDs(ch.Add), marshalLabelIDs(ch.Remove),
			state, ts); err != nil {
			return fmt.Errorf("mail_apply_chunks insert %d/%d: %w", ch.ApplyID, ch.ChunkNo, err)
		}
	}
	return tx.Commit()
}

// SetMailApplyChunkState flips one chunk's lifecycle state (durably, its
// own implicit transaction — this is the pre-/post-mutation intent stamp).
func (s *Store) SetMailApplyChunkState(applyID int64, chunkNo int, state string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`UPDATE mail_apply_chunks SET state = ?, updated_at = ? WHERE apply_id = ? AND chunk_no = ?`,
		state, nowRFC3339(), applyID, chunkNo,
	)
	return err
}

// ListMailApplyChunks returns an apply's chunks in chunk order.
func (s *Store) ListMailApplyChunks(applyID int64) ([]MailApplyChunk, error) {
	rows, err := s.db.Query(
		`SELECT apply_id, chunk_no, kind, ids, add_labels, remove_labels, state
		 FROM mail_apply_chunks WHERE apply_id = ? ORDER BY chunk_no ASC`, applyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailApplyChunk
	for rows.Next() {
		var ch MailApplyChunk
		var ids, add, remove string
		if err := rows.Scan(&ch.ApplyID, &ch.ChunkNo, &ch.Kind, &ids, &add, &remove, &ch.State); err != nil {
			return nil, err
		}
		ch.IDs = unmarshalLabelIDs(ids)
		ch.Add = unmarshalLabelIDs(add)
		ch.Remove = unmarshalLabelIDs(remove)
		out = append(out, ch)
	}
	return out, rows.Err()
}

// MailLedger is one undo-ledger group (one per apply / label rename).
type MailLedger struct {
	LedgerID  string `json:"ledger_id"`
	Account   string `json:"account"`
	PlanSha   string `json:"plan_sha"`
	ApplyID   int64  `json:"apply_id"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
}

// MailLedgerEntry is one per-id introduced-delta record.
type MailLedgerEntry struct {
	LedgerID     string   `json:"ledger_id"`
	ID           string   `json:"id"`
	Kind         string   `json:"kind"` // trash | label | label_rename
	DeltaAdd     []string `json:"delta_add"`
	DeltaRemove  []string `json:"delta_remove"`
	PrePlacement []string `json:"pre_placement"`
	OldName      string   `json:"old_name,omitempty"`
	NewName      string   `json:"new_name,omitempty"`
	Undone       string   `json:"undone,omitempty"` // '' | undone | conflict
}

// CreateMailLedger inserts the ledger group row.
func (s *Store) CreateMailLedger(l MailLedger) error {
	if l.LedgerID == "" || l.Account == "" {
		return fmt.Errorf("mail_ledger insert: ledger_id and account are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	created := l.CreatedAt
	if created == "" {
		created = nowRFC3339()
	}
	_, err := s.db.Exec(
		`INSERT INTO mail_ledger (ledger_id, account, plan_sha, apply_id, action, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		l.LedgerID, l.Account, l.PlanSha, l.ApplyID, l.Action, created,
	)
	return err
}

// InsertMailLedgerEntries appends delta records. INSERT OR IGNORE keyed on
// (ledger_id, id) makes crash-recovery re-inserts idempotent. Returns the
// number of rows actually inserted.
func (s *Store) InsertMailLedgerEntries(entries []MailLedgerEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO mail_ledger_entries
		 (ledger_id, id, kind, delta_add, delta_remove, pre_placement, old_name, new_name, undone, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	ts := nowRFC3339()
	inserted := 0
	for _, e := range entries {
		res, err := stmt.Exec(e.LedgerID, e.ID, e.Kind,
			marshalLabelIDs(e.DeltaAdd), marshalLabelIDs(e.DeltaRemove), marshalLabelIDs(e.PrePlacement),
			e.OldName, e.NewName, ts)
		if err != nil {
			return inserted, fmt.Errorf("mail_ledger_entries insert %s/%s: %w", e.LedgerID, e.ID, err)
		}
		if n, err := res.RowsAffected(); err == nil {
			inserted += int(n)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// GetMailLedger returns one ledger group row. Propagates sql.ErrNoRows.
func (s *Store) GetMailLedger(ledgerID string) (MailLedger, error) {
	var l MailLedger
	err := s.db.QueryRow(
		`SELECT ledger_id, account, plan_sha, apply_id, action, created_at
		 FROM mail_ledger WHERE ledger_id = ?`, ledgerID,
	).Scan(&l.LedgerID, &l.Account, &l.PlanSha, &l.ApplyID, &l.Action, &l.CreatedAt)
	return l, err
}

// ListMailLedgerEntries returns a ledger's entries in insertion (id) order.
func (s *Store) ListMailLedgerEntries(ledgerID string) ([]MailLedgerEntry, error) {
	rows, err := s.db.Query(
		`SELECT ledger_id, id, kind, delta_add, delta_remove, pre_placement, old_name, new_name, undone
		 FROM mail_ledger_entries WHERE ledger_id = ? ORDER BY id ASC`, ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailLedgerEntry
	for rows.Next() {
		var e MailLedgerEntry
		var add, remove, pre string
		if err := rows.Scan(&e.LedgerID, &e.ID, &e.Kind, &add, &remove, &pre, &e.OldName, &e.NewName, &e.Undone); err != nil {
			return nil, err
		}
		e.DeltaAdd = unmarshalLabelIDs(add)
		e.DeltaRemove = unmarshalLabelIDs(remove)
		e.PrePlacement = unmarshalLabelIDs(pre)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetMailLedgerEntryUndone stamps one entry's undo outcome ("undone" or
// "conflict") so a re-run of undo reports honestly instead of re-mutating.
func (s *Store) SetMailLedgerEntryUndone(ledgerID, id, status string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`UPDATE mail_ledger_entries SET undone = ? WHERE ledger_id = ? AND id = ?`,
		status, ledgerID, id,
	)
	return err
}
