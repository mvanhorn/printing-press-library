package store

import (
	"database/sql"
	"fmt"
)

// PPFixture is a Linear issue this CLI created (tracked locally so pp-cleanup
// can archive only its own fixtures, never pre-existing tickets).
type PPFixture struct {
	IssueID    string `json:"issue_id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Session    string `json:"session"`
	CreatedAt  string `json:"created_at"`
	ArchivedAt string `json:"archived_at,omitempty"`
}

// RecordPPFixture writes an issues-create result into the pp_created ledger.
func (s *Store) RecordPPFixture(issueID, identifier, title, session string) error {
	if issueID == "" || session == "" {
		return fmt.Errorf("RecordPPFixture: issueID and session required")
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO pp_created (issue_id, identifier, title, session) VALUES (?, ?, ?, ?)`,
		issueID, identifier, title, session)
	return err
}

// IsPPCreated returns true if issueID was created by this CLI in any session
// and is not yet archived. Used by trust-mode strict to gate mutations.
func (s *Store) IsPPCreated(issueID string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pp_created WHERE issue_id = ? AND archived_at IS NULL`, issueID).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListPPFixtures returns active (non-archived) fixtures for a given session,
// or for all sessions if session is empty.
func (s *Store) ListPPFixtures(session string) ([]PPFixture, error) {
	var rows *sql.Rows
	var err error
	if session == "" {
		rows, err = s.db.Query(`SELECT issue_id, IFNULL(identifier, ''), IFNULL(title, ''), session, created_at, IFNULL(archived_at, '') FROM pp_created WHERE archived_at IS NULL ORDER BY created_at DESC`)
	} else {
		rows, err = s.db.Query(`SELECT issue_id, IFNULL(identifier, ''), IFNULL(title, ''), session, created_at, IFNULL(archived_at, '') FROM pp_created WHERE session = ? AND archived_at IS NULL ORDER BY created_at DESC`, session)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PPFixture
	for rows.Next() {
		var f PPFixture
		if err := rows.Scan(&f.IssueID, &f.Identifier, &f.Title, &f.Session, &f.CreatedAt, &f.ArchivedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// MarkPPFixtureArchived flips archived_at for an issueID (called after the API archive succeeds).
func (s *Store) MarkPPFixtureArchived(issueID string) error {
	_, err := s.db.Exec(`UPDATE pp_created SET archived_at = CURRENT_TIMESTAMP WHERE issue_id = ?`, issueID)
	return err
}

// ListPPSessions returns distinct session tags that still have active fixtures.
func (s *Store) ListPPSessions() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT session FROM pp_created WHERE archived_at IS NULL ORDER BY session DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
