// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadBulkInput_JSONArray(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rows.json")
	content := `[{"merchant":"Cartesia","amount":4900,"created":"2025-11-17","tag":"101 G&A"},
	            {"merchant":"Cursor","amount":2204,"created":"2025-11-07"}]`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readBulkInput(p)
	if err != nil {
		t.Fatalf("readBulkInput: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].Merchant != "Cartesia" || got[0].Amount != 4900 || got[0].Created != "2025-11-17" || got[0].Tag != "101 G&A" {
		t.Errorf("row 0 mismatch: %+v", got[0])
	}
}

func TestReadBulkInput_JSONL(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "rows.jsonl")
	content := "# a comment\n" +
		`{"merchant":"Render","amount":4897,"date":"2025-12-17"}` + "\n" +
		"\n" +
		`{"merchant":"Linear","amount":3086,"created":"2025-12-09"}` + "\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readBulkInput(p)
	if err != nil {
		t.Fatalf("readBulkInput: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows (comment + blank skipped), got %d", len(got))
	}
	// `date` is tolerated as an alias for `created` on input.
	if got[0].Date != "2025-12-17" {
		t.Errorf("want date alias preserved, got %+v", got[0])
	}
}

func TestReadBulkInput_InvalidLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(p, []byte("{not json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBulkInput(p); err == nil {
		t.Fatal("expected error on invalid JSON line")
	}
}

func TestFirstNonEmptyStr(t *testing.T) {
	if got := firstNonEmptyStr("", "", "USD"); got != "USD" {
		t.Errorf("want USD, got %q", got)
	}
	if got := firstNonEmptyStr("EUR", "USD"); got != "EUR" {
		t.Errorf("want EUR, got %q", got)
	}
	if got := firstNonEmptyStr("", ""); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}
