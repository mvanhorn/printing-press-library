package store

import (
	"encoding/json"
	"fmt"
)

// Issue-relation persistence.
//
// `issue_relations` ships as one of the bare id/data/synced_at shell tables
// that nothing ever populated, which is why every relation-aware command had
// to go live. The helpers below give it typed columns and a per-issue
// replace, so a relation read can be written through and served again from
// the local snapshot.
//
// The typed columns are added lazily by ensureIssueRelationColumns rather
// than in migrate(): ALTER TABLE ADD COLUMN is idempotent here (ensureColumns
// inspects PRAGMA table_info first) and keeping the migration untouched
// avoids a shared-file edit for a table only this code path writes.

// issueRelationColumns are the typed columns the relation helpers need on
// top of the shell table's id/data/synced_at.
var issueRelationColumns = map[string]string{
	"issue_id":         "TEXT",
	"related_issue_id": "TEXT",
	"type":             "TEXT",
}

// IssueRelationRecord is the denormalized row written per relation. Data is
// the full API node, so a local read returns the same shape a live read does.
type IssueRelationRecord struct {
	ID             string
	IssueID        string
	RelatedIssueID string
	Type           string
	Data           json.RawMessage
}

func (s *Store) ensureIssueRelationColumns() error {
	return s.ensureColumns("issue_relations", issueRelationColumns)
}

// UpsertIssueRelation writes one relation row.
func (s *Store) UpsertIssueRelation(rec IssueRelationRecord) error {
	if rec.ID == "" {
		return fmt.Errorf("upsert issue_relation: empty relation id")
	}
	if err := s.ensureIssueRelationColumns(); err != nil {
		return err
	}
	return s.upsertEntity("issue_relations", []string{"id", "data", "issue_id", "related_issue_id", "type"},
		rec.ID, string(rec.Data), rec.IssueID, rec.RelatedIssueID, rec.Type,
	)
}

// ReplaceIssueRelationsForIssue rewrites every relation row that touches
// issueID, then inserts the supplied set. Replace rather than upsert because
// a relation deleted upstream leaves no tombstone to chase: rewriting the
// issue's whole neighbourhood is the only way a write-through read can also
// reconcile a deletion. Rows touching other issues are untouched.
func (s *Store) ReplaceIssueRelationsForIssue(issueID string, records []IssueRelationRecord) error {
	if issueID == "" {
		return fmt.Errorf("replace issue_relations: empty issue id")
	}
	if err := s.ensureIssueRelationColumns(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM issue_relations WHERE issue_id = ? OR related_issue_id = ?`, issueID, issueID); err != nil {
		return fmt.Errorf("replace issue_relations: %w", err)
	}
	for _, rec := range records {
		if rec.ID == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO issue_relations (id, data, issue_id, related_issue_id, type, synced_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			rec.ID, string(rec.Data), rec.IssueID, rec.RelatedIssueID, rec.Type,
		); err != nil {
			return fmt.Errorf("replace issue_relations: %w", err)
		}
	}
	return tx.Commit()
}

// DeleteIssueRelation removes one relation row by id. Returns the number of
// rows removed so a caller can distinguish a real delete from a no-op.
func (s *Store) DeleteIssueRelation(id string) (int64, error) {
	if id == "" {
		return 0, fmt.Errorf("delete issue_relation: empty relation id")
	}
	res, err := s.db.Exec(`DELETE FROM issue_relations WHERE id = ?`, id)
	if err != nil {
		return 0, fmt.Errorf("delete issue_relation: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return affected, nil
}

// ListIssueRelationsForIssue returns every stored relation node touching
// issueID, both directions, newest first.
func (s *Store) ListIssueRelationsForIssue(issueID string) ([]json.RawMessage, error) {
	if issueID == "" {
		return nil, fmt.Errorf("list issue_relations: empty issue id")
	}
	if err := s.ensureIssueRelationColumns(); err != nil {
		return nil, err
	}
	return s.queryJSON(
		`SELECT data FROM issue_relations WHERE issue_id = ? OR related_issue_id = ? ORDER BY synced_at DESC, id`,
		issueID, issueID,
	)
}

// CountIssueRelations reports how many relation rows the snapshot holds.
func (s *Store) CountIssueRelations() (int, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM issue_relations`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
