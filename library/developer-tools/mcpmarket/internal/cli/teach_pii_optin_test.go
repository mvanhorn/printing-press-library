// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/store"
)

func TestRejectPIIOptInAcrossTeachWrites(t *testing.T) {
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	t.Setenv("MCPMARKET_NO_LEARN", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, tc := range []struct {
		name, sensitive string
		args            []string
	}{
		{"teach query", "qa@example.invalid", []string{"teach", "--query", "orders for qa@example.invalid", "--resource-type", "server", "--resource", "one"}},
		{"teach pattern", "202-555-0199", []string{"teach-pattern", "--query-template", "tools for {entity}", "--resource-template", "server-{entity:category}", "--resource-type", "server", "--entity-kind", "category", "--venue", "202-555-0199"}},
		{"teach lookup", "qa@example.invalid", []string{"teach-lookup", "--kind", "team", "--canonical", "qa@example.invalid", "--value", "qa"}},
		{"teach playbook", "qa@example.invalid", []string{"teach-playbook", "--query", "find a server", "--notes", "email qa@example.invalid"}},
		{"playbook amend", "202-555-0199", []string{"playbook", "amend", "--query", "find a server", "--add-note", "call 202-555-0199"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "learn.db")
			cmd := RootCmd()
			cmd.SetArgs(append(tc.args, "--db", dbPath))
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			if err := cmd.Execute(); err == nil {
				t.Fatal("detected personal text was accepted")
			}
			if strings.Contains(output.String(), tc.sensitive) || !strings.Contains(output.String(), "PII rule") {
				t.Fatalf("rejection did not name the rule without echoing input: %q", output.String())
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("rejected teaching created a database: %v", err)
			}
		})
	}
}

func TestRejectPIIFlagReadsNotesFileBeforeWriting(t *testing.T) {
	t.Setenv("MCPMARKET_REJECT_PII", "")
	t.Setenv("MCPMARKET_NO_LEARN", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dir := t.TempDir()
	notesPath := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(notesPath, []byte("contact qa@example.invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "learn.db")
	cmd := RootCmd()
	cmd.SetArgs([]string{"teach", "--query", "find a server", "--resource-type", "server", "--resource", "one", "--playbook-notes-file", notesPath, "--reject-pii", "--db", dbPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err == nil {
		t.Fatal("personal text from notes file was accepted")
	}
	if strings.Contains(output.String(), "qa@example.invalid") || !strings.Contains(output.String(), "email PII rule") {
		t.Fatalf("file-based rejection was not safely reported: %q", output.String())
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("rejected file-based teaching created a database: %v", err)
	}
}

func TestRejectPIIOptInAllowsCleanIntegratedTeach(t *testing.T) {
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	t.Setenv("MCPMARKET_NO_LEARN", "")
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "learn.db")
	playbookPath := writePlaybookFile(t, home, "playbook.json", `{"steps":[{"cmd":"items list {category.id}"}],"entity_slots":["$CATEGORY"]}`)
	notesPath := writePlaybookFile(t, home, "notes.md", "Use the items list endpoint.")
	_, stderr, err := runRootArgs(t,
		"teach", "--query", "list inventory widgets", "--resource-type", "items", "--resource", "widget-42",
		"--playbook-file", playbookPath, "--playbook-notes-file", notesPath, "--db", dbPath,
	)
	if err != nil {
		t.Fatalf("clean opt-in teach failed: %v (stderr=%q)", err, stderr)
	}
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := listLearningsRows(context.Background(), s, store.ListLearningsFilter{})
	if err != nil || len(rows) != 1 || rows[0].ResourceID != "widget-42" {
		t.Fatalf("clean resource learning missing: rows=%#v error=%v", rows, err)
	}
	playbooks, err := s.ListPlaybooks()
	if err != nil || len(playbooks) != 1 || !strings.Contains(playbooks[0].PlaybookJSON, "items list") {
		t.Fatalf("clean playbook missing: rows=%#v error=%v", playbooks, err)
	}
}

func TestRejectPIIEnvironmentOverridesProfileAndFlagFalse(t *testing.T) {
	withTempLearnHome(t)
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	if err := saveProfileStore(&profileStore{Profiles: map[string]Profile{
		"weak": {Name: "weak", Values: map[string]string{"reject-pii": "false"}},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--profile", "weak"},
		{"--reject-pii=false"},
	} {
		dbPath := filepath.Join(t.TempDir(), "learn.db")
		cmdArgs := append([]string{"teach", "--query", "orders for qa@example.invalid", "--resource-type", "server", "--resource", "one", "--db", dbPath}, args...)
		_, stderr, err := runRootArgs(t, cmdArgs...)
		if err == nil || !strings.Contains(stderr, "email PII rule") {
			t.Fatalf("environment rejection was bypassed by %v: err=%v stderr=%q", args, err, stderr)
		}
		if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
			t.Fatalf("rejected teaching created explicit database: %v", err)
		}
	}
}

func TestRejectPIIOptionalPlaybookFailurePreservesCleanTeach(t *testing.T) {
	home := withTempLearnHome(t)
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing notes file", args: []string{"--playbook-notes-file", filepath.Join(home, "missing-notes.md")}},
		{name: "malformed inline JSON", args: []string{"--playbook-json", `{not-json}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "learn.db")
			args := append([]string{"teach", "--query", "list clean servers", "--resource-type", "server", "--resource", "one", "--db", dbPath}, tc.args...)
			_, _, err := runRootArgs(t, args...)
			if err != nil {
				t.Fatalf("optional playbook failure discarded a clean teach: %v", err)
			}
			s, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			rows, err := listLearningsRows(context.Background(), s, store.ListLearningsFilter{})
			if err != nil || len(rows) != 1 || rows[0].ResourceID != "one" {
				t.Fatalf("clean resource learning missing: rows=%#v error=%v", rows, err)
			}
			playbooks, err := s.ListPlaybooks()
			if err != nil || len(playbooks) != 0 {
				t.Fatalf("invalid optional playbook was stored: rows=%#v error=%v", playbooks, err)
			}
		})
	}
}

func TestRejectPIIDefaultDatabaseContainsNoRejectedLearning(t *testing.T) {
	withTempLearnHome(t)
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	_, stderr, err := runRootArgs(t,
		"teach", "--query", "orders for qa@example.invalid", "--resource-type", "server", "--resource", "one",
	)
	if err == nil || !strings.Contains(stderr, "email PII rule") {
		t.Fatalf("default database rejected input unexpectedly: err=%v stderr=%q", err, stderr)
	}
	s, err := store.OpenWithContext(context.Background(), defaultDBPath("mcpmarket-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := listLearningsRows(context.Background(), s, store.ListLearningsFilter{})
	if err != nil || len(rows) != 0 {
		t.Fatalf("rejected learning reached default database: rows=%#v error=%v", rows, err)
	}
}

func TestRejectPIIPlaybookReadErrorNamesFlagWithoutPath(t *testing.T) {
	home := withTempLearnHome(t)
	t.Setenv("MCPMARKET_REJECT_PII", "1")
	missingNotes := filepath.Join(home, "qa@example.invalid-notes.md")
	_, _, err := runRootArgs(t,
		"teach-playbook", "--query", "list clean servers", "--notes-file", missingNotes,
	)
	if err == nil || !strings.Contains(err.Error(), "--notes-file: file not found") || strings.Contains(err.Error(), missingNotes) {
		t.Fatalf("unsafe or unhelpful notes error: %v", err)
	}
}
