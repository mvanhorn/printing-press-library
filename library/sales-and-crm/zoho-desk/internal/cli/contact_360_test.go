// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/zoho-desk/internal/store"
)

// TestNovelContact360HelpWires smoke-tests that the contact-360 command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelContact360HelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"contact-360", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("contact-360 --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "contact-360"} {
		if !strings.Contains(help, want) {
			t.Fatalf("contact-360 --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelContact360Behavior(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	db.Upsert("contacts", "C1", mustJSON(t, map[string]any{
		"id": "C1", "email": "jane@acme.com", "firstName": "Jane", "lastName": "Doe", "accountId": "ACC1",
	}))
	db.Upsert("accounts", "ACC1", mustJSON(t, map[string]any{"id": "ACC1", "accountName": "Acme"}))
	db.Upsert("tickets", "1", mustJSON(t, map[string]any{
		"id": "1", "ticketNumber": "101", "subject": "open one", "status": "Open", "contactId": "C1",
	}))
	db.Upsert("tickets", "2", mustJSON(t, map[string]any{
		"id": "2", "ticketNumber": "102", "subject": "closed one", "status": "Closed", "contactId": "C1",
	}))
	db.Upsert("tickets", "3", mustJSON(t, map[string]any{"id": "3", "status": "Open", "contactId": "OTHER"}))
	db.Close()

	// Case-insensitive email match.
	out := runTranscendCmd(t, "contact-360", "JANE@ACME.COM", "--db", dbPath, "--json")
	var view struct {
		Contact     map[string]any `json:"contact"`
		Account     map[string]any `json:"account"`
		TicketCount int            `json:"ticketCount"`
		OpenCount   int            `json:"openCount"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if view.Contact == nil || view.Contact["id"] != "C1" {
		t.Fatalf("expected contact C1, got %+v", view.Contact)
	}
	if view.Account == nil || view.Account["accountName"] != "Acme" {
		t.Fatalf("expected account Acme, got %+v", view.Account)
	}
	if view.TicketCount != 2 || view.OpenCount != 1 {
		t.Fatalf("ticketCount=%d openCount=%d, want 2/1", view.TicketCount, view.OpenCount)
	}

	// No match -> contact null with a note, exit 0.
	out = runTranscendCmd(t, "contact-360", "nobody@nowhere.com", "--db", dbPath, "--json")
	var miss struct {
		Contact any    `json:"contact"`
		Note    string `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &miss); err != nil {
		t.Fatalf("unmarshal miss: %v\n%s", err, out)
	}
	if miss.Contact != nil || miss.Note == "" {
		t.Fatalf("expected null contact and a note, got %s", out)
	}
}
