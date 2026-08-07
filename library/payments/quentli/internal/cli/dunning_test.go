// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
)

// TestNovelDunningHelpWires smoke-tests that the dunning command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDunningHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"dunning", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dunning --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dunning"} {
		if !strings.Contains(help, want) {
			t.Fatalf("dunning --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestNovelDunningCollapsesUnspecifiedCurrency guards the footer against the
// bucket-splitting bug: an invoice with no currency and one explicitly in MXN
// must aggregate into a single MXN total. Grouping on the raw currency emits two
// footer lines that both render as "MXN", which reads as contradictory totals.
func TestNovelDunningCollapsesUnspecifiedCurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dunning.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	due := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	seed := func(resource, id string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %s/%s: %v", resource, id, err)
		}
		if err := db.Upsert(resource, id, raw); err != nil {
			t.Fatalf("upsert %s/%s: %v", resource, id, err)
		}
	}
	seed("customers", "cus_1", map[string]any{"id": "cus_1", "name": "Acme", "email": "ap@acme.mx"})
	// Same customer, same real currency — one row omits it, one states it.
	seed("invoices", "inv_1", map[string]any{"id": "inv_1", "customerId": "cus_1", "totalAmount": 10000, "amountPaid": 0, "currency": "", "dueDate": due})
	seed("invoices", "inv_2", map[string]any{"id": "inv_2", "customerId": "cus_1", "totalAmount": 5000, "amountPaid": 0, "currency": "MXN", "dueDate": due})
	// A genuinely different currency must still stay in its own bucket.
	seed("invoices", "inv_3", map[string]any{"id": "inv_3", "customerId": "cus_1", "totalAmount": 2000, "amountPaid": 0, "currency": "USD", "dueDate": due})
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// --human-friendly selects the table+footer path; the flag registration in
	// RootCmd resets the package-level toggle, so pass it as an arg and restore.
	t.Cleanup(func() { humanFriendly = false })

	cmd := RootCmd()
	cmd.SetArgs([]string{"dunning", "--db", dbPath, "--human-friendly"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dunning error = %v", err)
	}

	got := out.String()
	marker := "Total outstanding across"
	idx := strings.Index(got, marker)
	if idx < 0 {
		t.Fatalf("missing %q footer in output:\n%s", marker, got)
	}
	footer := got[idx:]
	var mxn []string
	for _, line := range strings.Split(footer, "\n") {
		if strings.Contains(line, "MXN") {
			mxn = append(mxn, strings.TrimSpace(line))
		}
	}
	if len(mxn) != 1 {
		t.Fatalf("want exactly 1 MXN footer line, got %d: %v\nfooter:\n%s", len(mxn), mxn, footer)
	}
	if want := fmt.Sprintf("MXN %.2f", 150.00); mxn[0] != want {
		t.Fatalf("MXN footer = %q, want %q (unspecified + explicit MXN must sum)", mxn[0], want)
	}
	if !strings.Contains(footer, "USD 20.00") {
		t.Fatalf("USD bucket missing or wrong in footer:\n%s", footer)
	}
	if !strings.Contains(footer, "across 1 customers") {
		t.Fatalf("want 1 distinct customer in footer:\n%s", footer)
	}
}
