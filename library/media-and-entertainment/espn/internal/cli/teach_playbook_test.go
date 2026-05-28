// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
)

func writePlaybookFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestTeachPlaybook_HappyPath(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	playbookPath := writePlaybookFile(t, home, "p.json",
		`{"steps":[{"cmd":"teams basketball nba {team.id}"}],"entity_slots":["$TEAM"]}`)
	notesPath := writePlaybookFile(t, home, "n.md",
		"byathlete needs seasontype=2; categories has dup labels")

	stdout, stderr, err := runRootArgs(t,
		"teach-playbook",
		"--query", "how did Pistons end the season who led in ppg rpg spg",
		"--playbook-file", playbookPath,
		"--notes-file", notesPath,
		"--db", dbPath,
		"--agent",
	)
	if err != nil {
		t.Fatalf("teach-playbook: %v (stderr=%q)", err, stderr)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("decode: %v (stdout=%q)", err, stdout)
	}
	if v, _ := resp["recorded"].(bool); !v {
		t.Errorf("recorded should be true, got %v", resp)
	}
	if v, _ := resp["has_playbook"].(bool); !v {
		t.Errorf("has_playbook should be true, got %v", resp)
	}
	if v, _ := resp["has_notes"].(bool); !v {
		t.Errorf("has_notes should be true, got %v", resp)
	}

	// SQL-inspect
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	rows, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 playbook row, got %d", len(rows))
	}
	if !strings.Contains(rows[0].NotesText, "seasontype=2") {
		t.Errorf("notes_text not stored: %q", rows[0].NotesText)
	}
	if !strings.Contains(rows[0].PlaybookJSON, "teams basketball") {
		t.Errorf("playbook_json not stored: %q", rows[0].PlaybookJSON)
	}
}

func TestTeachPlaybook_NotesOnly(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "how to interpret division standings",
		"--notes", "always read both league_id and conference_id",
		"--db", dbPath,
		"--agent",
	)
	if err != nil {
		t.Fatalf("teach-playbook: %v", err)
	}
	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	rows, _ := s.ListPlaybooks()
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if rows[0].PlaybookJSON != "" {
		t.Errorf("playbook_json should be empty for notes-only, got %q", rows[0].PlaybookJSON)
	}
	if !strings.Contains(rows[0].NotesText, "league_id") {
		t.Errorf("notes_text content missing")
	}
}

func TestTeachPlaybook_MissingFile(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "any query",
		"--playbook-file", filepath.Join(home, "nonexistent.json"),
		"--db", dbPath,
	)
	if err == nil {
		t.Fatal("expected error for missing playbook file")
	}
}

func TestTeachPlaybook_MalformedJSON(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	badPath := writePlaybookFile(t, home, "bad.json", "{not valid json")

	_, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "any",
		"--playbook-file", badPath,
		"--db", dbPath,
	)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestTeachPlaybook_RequiresContent(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "x",
		"--db", dbPath,
	)
	if err == nil {
		t.Fatal("expected error when neither playbook nor notes provided")
	}
}

func TestTeach_WithPlaybookFile_LandsBothSurfaces(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	playbookPath := writePlaybookFile(t, home, "p.json",
		`{"steps":[{"cmd":"teams basketball nba {team.id}"}],"entity_slots":["$TEAM"]}`)

	_, _, err := runRootArgs(t,
		"teach",
		"--query", "how did Pistons end the season who led in ppg rpg spg",
		"--resource", "8",
		"--resource-type", "teams",
		"--playbook-file", playbookPath,
		"--playbook-notes", "byathlete needs seasontype=2",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach: %v", err)
	}

	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	// Resource learning landed.
	lrows, _ := listLearningsRows(context.Background(), s, store.ListLearningsFilter{})
	if len(lrows) != 1 {
		t.Errorf("want 1 learning row, got %d", len(lrows))
	}
	// Playbook row landed too.
	pbRows, _ := s.ListPlaybooks()
	if len(pbRows) != 1 {
		t.Errorf("want 1 playbook row, got %d", len(pbRows))
	} else if !strings.Contains(pbRows[0].NotesText, "seasontype=2") {
		t.Errorf("playbook notes missing seasontype=2")
	}
}

func TestTeach_GracefulDegrade_PlaybookFails(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	badPath := writePlaybookFile(t, home, "bad.json", "{not json")

	// teach should succeed (resource learning lands) even though
	// playbook file is malformed. The failure goes to teach.log.
	_, _, err := runRootArgs(t,
		"teach",
		"--query", "how to do X",
		"--resource", "x-1",
		"--resource-type", "things",
		"--playbook-file", badPath,
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("teach should not fail when only playbook side errors: %v", err)
	}
	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	lrows, _ := listLearningsRows(context.Background(), s, store.ListLearningsFilter{})
	if len(lrows) != 1 {
		t.Errorf("resource learning should land despite playbook fail; got %d rows", len(lrows))
	}
	pbRows, _ := s.ListPlaybooks()
	if len(pbRows) != 0 {
		t.Errorf("playbook should not land on parse failure; got %d rows", len(pbRows))
	}
}

func TestPlaybookAmend_HappyPath_ExistingPlaybook(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	pbPath := writePlaybookFile(t, home, "p.json", `{"steps":[{"cmd":"x"}],"query_family_examples":["foo bar baz"]}`)

	// Seed an existing playbook for "foo bar baz" family via teach-playbook
	if _, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "foo bar baz",
		"--playbook-file", pbPath,
		"--notes", "original note",
		"--db", dbPath,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Now amend
	if _, _, err := runRootArgs(t,
		"playbook", "amend",
		"--query", "foo bar baz",
		"--add-note", "a new correction",
		"--db", dbPath,
	); err != nil {
		t.Fatalf("amend: %v", err)
	}

	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	rows, _ := s.ListPlaybooks()
	if len(rows) != 1 {
		t.Fatalf("want 1 row (amend should append, not create new), got %d", len(rows))
	}
	if !strings.Contains(rows[0].NotesText, "original note") {
		t.Errorf("amend should preserve original notes; got %q", rows[0].NotesText)
	}
	if !strings.Contains(rows[0].NotesText, "a new correction") {
		t.Errorf("amend should add new note; got %q", rows[0].NotesText)
	}
	if !strings.Contains(rows[0].NotesText, "[amend ") {
		t.Errorf("amend should include timestamp marker; got %q", rows[0].NotesText)
	}
}

func TestPlaybookAmend_EmptyFamily_CreatesNotesOnly(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	// Amend a family with no existing playbook
	if _, _, err := runRootArgs(t,
		"playbook", "amend",
		"--query", "brand new query family",
		"--add-note", "from cold",
		"--db", dbPath,
	); err != nil {
		t.Fatalf("amend: %v", err)
	}

	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	rows, _ := s.ListPlaybooks()
	if len(rows) != 1 {
		t.Fatalf("want 1 row created from cold amend, got %d", len(rows))
	}
	if !strings.Contains(rows[0].NotesText, "from cold") {
		t.Errorf("notes should contain the amend text; got %q", rows[0].NotesText)
	}
	if !strings.Contains(rows[0].NotesText, "[amend ") {
		t.Errorf("cold amend should still carry the timestamp marker; got %q", rows[0].NotesText)
	}
}

func TestPlaybookAmend_RequiresQuery(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"playbook", "amend",
		"--add-note", "missing query",
		"--db", dbPath,
	)
	if err == nil {
		t.Fatal("amend without --query should exit nonzero")
	}
}

func TestPlaybookAmend_RequiresAddNote(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	_, _, err := runRootArgs(t,
		"playbook", "amend",
		"--query", "no add-note",
		"--db", dbPath,
	)
	if err == nil {
		t.Fatal("amend without --add-note should exit nonzero")
	}
}

func TestPlaybookAmend_MultipleAmendsTimestamped(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	for i, text := range []string{"first amend", "second amend", "third amend"} {
		if _, _, err := runRootArgs(t,
			"playbook", "amend",
			"--query", "multi-amend test",
			"--add-note", text,
			"--db", dbPath,
		); err != nil {
			t.Fatalf("amend %d: %v", i, err)
		}
	}

	s, _ := store.OpenWithContext(context.Background(), dbPath)
	defer s.Close()
	rows, _ := s.ListPlaybooks()
	if len(rows) != 1 {
		t.Fatalf("want 1 row after 3 amends to same family, got %d", len(rows))
	}
	for _, text := range []string{"first amend", "second amend", "third amend"} {
		if !strings.Contains(rows[0].NotesText, text) {
			t.Errorf("notes missing %q; full: %q", text, rows[0].NotesText)
		}
	}
}

func TestPlaybookAmend_RespectsNoLearn(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	t.Setenv(noLearnEnvVar, "true")

	_, _, err := runRootArgs(t,
		"playbook", "amend",
		"--query", "should noop",
		"--add-note", "should noop",
		"--db", dbPath,
	)
	if err != nil {
		t.Fatalf("amend with NO_LEARN should be silent noop: %v", err)
	}

	// DB shouldn't exist or should have no playbook rows.
	if _, statErr := os.Stat(dbPath); statErr == nil {
		s, _ := store.OpenWithContext(context.Background(), dbPath)
		defer s.Close()
		rows, _ := s.ListPlaybooks()
		if len(rows) != 0 {
			t.Errorf("NO_LEARN should suppress amend writes; got %d rows", len(rows))
		}
	}
}

func TestPlaybookList_Empty(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")

	stdout, _, err := runRootArgs(t,
		"playbook", "list",
		"--db", dbPath,
		"--json",
	)
	if err != nil {
		t.Fatalf("playbook list: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("empty list should be []; got %q", stdout)
	}
}

func TestPlaybookList_WithRows(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "data.db")
	pbPath := writePlaybookFile(t, home, "p.json", `{"steps":[{"cmd":"x"}]}`)

	if _, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "first query",
		"--playbook-file", pbPath,
		"--db", dbPath,
	); err != nil {
		t.Fatalf("seed first: %v", err)
	}
	if _, _, err := runRootArgs(t,
		"teach-playbook",
		"--query", "second query",
		"--notes", "remember the thing",
		"--db", dbPath,
	); err != nil {
		t.Fatalf("seed second: %v", err)
	}

	stdout, _, err := runRootArgs(t,
		"playbook", "list",
		"--db", dbPath,
		"--json",
	)
	if err != nil {
		t.Fatalf("playbook list: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("decode list: %v (stdout=%q)", err, stdout)
	}
	if len(rows) != 2 {
		t.Errorf("want 2 rows, got %d", len(rows))
	}
}
