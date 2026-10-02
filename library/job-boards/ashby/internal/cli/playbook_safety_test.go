// Copyright 2026 klubieniecki and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/job-boards/ashby/internal/store"
)

func TestValidateAshbyPlaybookAllowsOnlyReadOnlyArgvCommands(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{name: "list", command: "ashby-pp-cli postings list {board.name} --remote --limit 5"},
		{name: "get", command: "postings get ashby {posting.id} --json"},
		{name: "search", command: "search engineer --board ashby --limit=5"},
		{name: "flag before positional", command: "postings list --limit 5 ashby"},
		{name: "synthesized list", command: "postings list {board.name} --limit <int> --location <str>"},
		{name: "synthesized get", command: "postings get {board.name} --json {posting.id}"},
		{name: "synthesized search", command: "search --board <str> {query}"},
		{name: "arbitrary executable", command: "rm -rf data", wantErr: true},
		{name: "write command", command: "ashby-pp-cli sync ashby", wantErr: true},
		{name: "semicolon", command: "postings list ashby; touch owned", wantErr: true},
		{name: "substitution", command: "postings list $(touch owned)", wantErr: true},
		{name: "pipe", command: "postings list ashby | sh", wantErr: true},
		{name: "output file", command: "postings list ashby --deliver file:owned", wantErr: true},
		{name: "config injection", command: "postings list ashby --config malicious.toml", wantErr: true},
		{name: "flag smuggled as positional", command: "postings get --home /tmp", wantErr: true},
		{name: "synthesized hint as positional", command: "postings list <str>", wantErr: true},
		{name: "redacted hint", command: "postings list ashby --query <redacted>", wantErr: true},
		{name: "extra positional", command: "postings list ashby other", wantErr: true},
		{name: "missing positional after flag", command: "postings list --limit 5", wantErr: true},
		{name: "flag without value", command: "postings list ashby --limit", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAshbyPlaybook(learn.Playbook{Steps: []learn.PlaybookStep{{Cmd: test.command}}})
			if (err != nil) != test.wantErr {
				t.Fatalf("validate %q error=%v, wantErr=%v", test.command, err, test.wantErr)
			}
		})
	}
}

func TestTeachPlaybookRejectsPersistentCommandInjection(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := home + "/data.db"
	_, _, err := runRootArgs(t,
		"teach-playbook", "--query", "find jobs", "--db", dbPath,
		"--playbook-json", `{"steps":[{"cmd":"postings list ashby; touch owned"}]}`,
	)
	if err == nil || !strings.Contains(err.Error(), "unsafe playbook") {
		t.Fatalf("teach-playbook error=%v, want unsafe-playbook rejection", err)
	}
	s, openErr := store.OpenWithContext(context.Background(), dbPath)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer s.Close()
	rows, listErr := s.ListPlaybooks()
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(rows) != 0 {
		t.Fatalf("unsafe playbook persisted: %#v", rows)
	}
}

func TestRecallRejectsUnsafePersistedPlaybookAndMarksSafeRowsUntrusted(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := home + "/data.db"
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	query := "find available jobs"
	family := learn.QueryFamily(learn.Normalize(query, newLearnConfig()))
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  family,
		PlaybookJSON: `{"steps":[{"cmd":"postings list ashby; touch owned"}]}`,
	}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	stdout, _, err := runRootArgs(t, "recall", query, "--db", dbPath, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var rejected recallEnvelope
	unmarshalAgentResults(t, stdout, &rejected)
	if rejected.Playbook != nil {
		t.Fatalf("unsafe stored playbook surfaced: %#v", rejected.Playbook)
	}
	if !slices.Contains(rejected.Warnings, learn.TopWarningUnsafePlaybookRejected) {
		t.Fatalf("warnings=%v, want unsafe rejection", rejected.Warnings)
	}

	s, err = store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpsertPlaybook(store.UpsertPlaybookInput{
		QueryFamily:  family,
		PlaybookJSON: `{"steps":[{"cmd":"postings list ashby --limit 5"}]}`,
	}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	stdout, _, err = runRootArgs(t, "recall", query, "--db", dbPath, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var safe recallEnvelope
	unmarshalAgentResults(t, stdout, &safe)
	if safe.Playbook == nil || safe.Playbook.Trust != "untrusted" || !safe.Playbook.ReviewRequired {
		t.Fatalf("safe persisted playbook lacks trust metadata: %#v", safe.Playbook)
	}
}
