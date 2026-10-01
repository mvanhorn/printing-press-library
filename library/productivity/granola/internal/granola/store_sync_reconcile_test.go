package granola

import (
	"context"
	"testing"
)

func TestReconcileMissingAPINotesRemovesOnlyAPIDependents(t *testing.T) {
	db := openTestDB(t)
	statements := []string{
		`INSERT INTO meetings(id, row_source) VALUES ('missing', 'api'), ('kept', 'api')`,
		`INSERT INTO transcript_segments(meeting_id, idx, row_source) VALUES ('missing', 0, 'api'), ('missing', 1, 'cache'), ('kept', 0, 'api')`,
		`INSERT INTO attendees(meeting_id, email, row_source) VALUES ('missing', 'api@invalid.test', 'api'), ('missing', 'cache@invalid.test', 'cache'), ('kept', 'kept@invalid.test', 'api')`,
		`INSERT INTO folder_memberships(folder_id, meeting_id, row_source) VALUES ('api-folder', 'missing', 'api'), ('cache-folder', 'missing', 'cache'), ('kept-folder', 'kept', 'api')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := ReconcileMissingAPINotes(context.Background(), db, map[string]struct{}{"kept": {}})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='missing' AND deleted_at IS NOT NULL`, 1, "missing meeting was not marked deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='missing' AND row_source='api'`, 0, "missing API transcript remains")
	assertCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='missing' AND row_source='api'`, 0, "missing API attendees remain")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='missing' AND row_source='cache'`, 1, "cache transcript was deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='missing' AND row_source='cache'`, 1, "cache attendee was deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='missing' AND row_source='api'`, 0, "missing API folder membership remains")
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='missing' AND row_source='cache'`, 1, "cache folder membership was deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='kept' AND deleted_at IS NULL`, 1, "seen meeting was deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='kept'`, 1, "seen meeting transcript was deleted")
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='kept'`, 1, "seen meeting membership was deleted")
}

func TestReconcileMissingAPINotesRollsBackDependentFailure(t *testing.T) {
	db := openTestDB(t)
	for _, statement := range []string{
		`INSERT INTO meetings(id, row_source) VALUES ('missing', 'api')`,
		`INSERT INTO transcript_segments(meeting_id, idx, row_source) VALUES ('missing', 0, 'api')`,
		`INSERT INTO attendees(meeting_id, email, row_source) VALUES ('missing', 'api@invalid.test', 'api')`,
		`CREATE TRIGGER fail_api_attendee_delete BEFORE DELETE ON attendees WHEN OLD.row_source='api' BEGIN SELECT RAISE(ABORT, 'synthetic failure'); END`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ReconcileMissingAPINotes(context.Background(), db, map[string]struct{}{}); err == nil {
		t.Fatal("expected dependent-row deletion to fail")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='missing' AND COALESCE(deleted_at, '')=''`, 1, "meeting tombstone was not rolled back")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='missing' AND row_source='api'`, 1, "transcript deletion was not rolled back")
	assertCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='missing' AND row_source='api'`, 1, "attendee deletion was not rolled back")
}
