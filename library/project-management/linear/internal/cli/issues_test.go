package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
	"github.com/spf13/cobra"
)

func TestParseIssueIdentifier(t *testing.T) {
	cases := []struct {
		in       string
		wantTeam string
		wantNum  float64
		wantOK   bool
	}{
		{"ESP-1155", "ESP", 1155, true},
		{"MULTI-WORD-42", "MULTI-WORD", 42, true}, // unusual but valid: splits on last hyphen
		{"ESP", "", 0, false},
		{"ESP-", "", 0, false},
		{"-1155", "", 0, false},
		{"ESP-abc", "", 0, false},
		{"ESP-0", "", 0, false},
		{"", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			team, num, ok := parseIssueIdentifier(tc.in)
			if team != tc.wantTeam || num != tc.wantNum || ok != tc.wantOK {
				t.Fatalf("parseIssueIdentifier(%q) = (%q, %v, %v); want (%q, %v, %v)",
					tc.in, team, num, ok, tc.wantTeam, tc.wantNum, tc.wantOK)
			}
		})
	}
}

func TestMatchesStateFilter(t *testing.T) {
	cases := []struct {
		stateType string
		flag      string
		want      bool
	}{
		{"started", "active", true},
		{"backlog", "active", true},
		{"unstarted", "active", true},
		{"completed", "active", false},
		{"canceled", "active", false},
		{"completed", "all", true},
		{"canceled", "", true},
		{"started", "started", true},
		{"started", "completed", false},
		{"Started", "started", true}, // case insensitive
	}
	for _, tc := range cases {
		if got := matchesStateFilter(tc.stateType, tc.flag); got != tc.want {
			t.Errorf("matchesStateFilter(%q, %q) = %v; want %v", tc.stateType, tc.flag, got, tc.want)
		}
	}
}

func seedTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	issues := []map[string]any{
		{
			"id": "uuid-1", "identifier": "ESP-1", "title": "Urgent thing", "priority": 1,
			"state": map[string]any{"name": "In Progress", "type": "started"},
			"team":  map[string]any{"id": "team-esp", "key": "ESP", "name": "Esper"},
			"assignee": map[string]any{
				"id": "user-1", "name": "Matt", "displayName": "mvanhorn", "email": "mvanhorn@gmail.com",
			},
			"updatedAt": "2026-04-20T00:00:00.000Z",
		},
		{
			"id": "uuid-2", "identifier": "ESP-2", "title": "Medium thing", "priority": 3,
			"state":     map[string]any{"name": "Todo", "type": "backlog"},
			"team":      map[string]any{"id": "team-esp", "key": "ESP", "name": "Esper"},
			"updatedAt": "2026-04-19T00:00:00.000Z",
		},
		{
			"id": "uuid-3", "identifier": "ESP-3", "title": "Shipped thing", "priority": 2,
			"state":     map[string]any{"name": "Done", "type": "completed"},
			"team":      map[string]any{"id": "team-esp", "key": "ESP", "name": "Esper"},
			"updatedAt": "2026-04-18T00:00:00.000Z",
		},
		{
			"id": "uuid-4", "identifier": "OTHER-9", "title": "Other team work", "priority": 2,
			"state":     map[string]any{"name": "Todo", "type": "backlog"},
			"team":      map[string]any{"id": "team-other", "key": "OTHER", "name": "Other"},
			"updatedAt": "2026-04-20T01:00:00.000Z",
		},
	}
	for _, i := range issues {
		raw, _ := json.Marshal(i)
		if err := db.UpsertIssue(i["id"].(string), i["identifier"].(string), i["title"].(string), raw); err != nil {
			t.Fatalf("seed issue %v: %v", i["identifier"], err)
		}
	}

	teams := []map[string]any{
		{"id": "team-esp", "key": "ESP", "name": "Esper"},
		{"id": "team-other", "key": "OTHER", "name": "Other"},
	}
	for _, tm := range teams {
		raw, _ := json.Marshal(tm)
		if err := db.UpsertTeam(tm["id"].(string), raw); err != nil {
			t.Fatalf("seed team: %v", err)
		}
	}

	user := map[string]any{
		"id": "user-1", "name": "Matt", "displayName": "mvanhorn", "email": "mvanhorn@gmail.com",
	}
	raw, _ := json.Marshal(user)
	if err := db.UpsertUser("user-1", raw); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	return db, dbPath
}

func TestListIssuesByIdentifier(t *testing.T) {
	db, _ := seedTestStore(t)
	rows, err := db.ListIssues(map[string]string{"identifier": "ESP-2"}, 10)
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListIssues by identifier: got %d rows; want 1", len(rows))
	}
	var got struct{ Identifier string }
	json.Unmarshal(rows[0], &got)
	if got.Identifier != "ESP-2" {
		t.Fatalf("Identifier = %q; want ESP-2", got.Identifier)
	}
}

func TestResolveTeamFilter(t *testing.T) {
	db, _ := seedTestStore(t)
	id, err := resolveTeamFilter(db, "ESP")
	if err != nil {
		t.Fatalf("resolveTeamFilter(ESP): %v", err)
	}
	if id != "team-esp" {
		t.Fatalf("got %q; want team-esp", id)
	}
	if _, err := resolveTeamFilter(db, "NOPE"); err == nil {
		t.Fatal("expected error for unknown team key")
	}
}

func TestResolveAssigneeFilter_LocalLookup(t *testing.T) {
	db, _ := seedTestStore(t)
	flags := &rootFlags{}
	id, err := resolveAssigneeFilter(flags, db, "mvanhorn@gmail.com")
	if err != nil {
		t.Fatalf("resolveAssigneeFilter by email: %v", err)
	}
	if id != "user-1" {
		t.Fatalf("got %q; want user-1", id)
	}
	id, err = resolveAssigneeFilter(flags, db, "mvanhorn")
	if err != nil {
		t.Fatalf("resolveAssigneeFilter by displayName: %v", err)
	}
	if id != "user-1" {
		t.Fatalf("got %q; want user-1", id)
	}
	if _, err := resolveAssigneeFilter(flags, db, "nobody"); err == nil {
		t.Fatal("expected error for unknown assignee")
	}
}

// runListForTest invokes runIssuesList with a captured stdout. --json is forced so
// output is deterministic across environments.
func runListForTest(t *testing.T, dbPath, assignee, stateFlag, team, project string, limit int) string {
	t.Helper()
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	flags := &rootFlags{asJSON: true}
	if err := runIssuesList(cmd, flags, dbPath, assignee, stateFlag, team, project, limit); err != nil {
		t.Fatalf("runIssuesList: %v", err)
	}
	return buf.String()
}

func TestRunIssuesList_DefaultExcludesCompleted(t *testing.T) {
	_, path := seedTestStore(t)
	out := runListForTest(t, path, "", "active", "", "", 200)
	var rows []issueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out)
	}
	for _, r := range rows {
		if r.State.Type == "completed" || r.State.Type == "canceled" {
			t.Errorf("unexpected %q issue in active list: %s", r.State.Type, r.Identifier)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("active list: got %d rows; want 3 (ESP-1, ESP-2, OTHER-9)", len(rows))
	}
}

func TestRunIssuesList_StateAllIncludesCompleted(t *testing.T) {
	_, path := seedTestStore(t)
	out := runListForTest(t, path, "", "all", "", "", 200)
	var rows []issueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var sawCompleted bool
	for _, r := range rows {
		if r.State.Type == "completed" {
			sawCompleted = true
		}
	}
	if !sawCompleted {
		t.Fatalf("expected --state all to include completed; got: %s", out)
	}
}

func TestRunIssuesList_TeamFilter(t *testing.T) {
	_, path := seedTestStore(t)
	out := runListForTest(t, path, "", "all", "OTHER", "", 200)
	var rows []issueRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 1 || rows[0].Identifier != "OTHER-9" {
		t.Fatalf("team filter: got %+v; want only OTHER-9", rows)
	}
}

func TestRunIssuesGet_LocalHit(t *testing.T) {
	_, path := seedTestStore(t)
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{})
	flags := &rootFlags{asJSON: true, dataSource: "local"}
	if err := runIssuesGet(cmd, flags, path, "ESP-2"); err != nil {
		t.Fatalf("runIssuesGet: %v", err)
	}
	var wrapped struct {
		Results json.RawMessage `json:"results"`
		Meta    struct {
			Source string `json:"source"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(buf.Bytes(), &wrapped); err != nil {
		t.Fatalf("unmarshal wrapped: %v\nraw: %s", err, buf.String())
	}
	if wrapped.Meta.Source != "local" {
		t.Fatalf("meta.source = %q; want local", wrapped.Meta.Source)
	}
	var inner struct {
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(wrapped.Results, &inner); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if inner.Identifier != "ESP-2" {
		t.Fatalf("identifier = %q; want ESP-2", inner.Identifier)
	}
}

func TestRunIssuesGet_LocalMiss(t *testing.T) {
	_, path := seedTestStore(t)
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	flags := &rootFlags{dataSource: "local"}
	err := runIssuesGet(cmd, flags, path, "NOPE-999")
	if err == nil {
		t.Fatal("expected not-found error; got nil")
	}
	if !strings.Contains(err.Error(), "NOPE-999") {
		t.Fatalf("error should mention identifier: %v", err)
	}
}
