// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLedgerSQLReadOnlyBypass replays the payloads that once escaped the
// read-only guard (a quote inside a comment hid the statement separator)
// and checks that each is rejected with a usage error, writes no file, and
// leaves the mirror intact.
func TestLedgerSQLReadOnlyBypass(t *testing.T) {
	s3Fixture(t)
	out := filepath.Join(t.TempDir(), "pwn.db")
	payloads := []struct {
		q     string
		extra []string
	}{
		{"SELECT 1 WHERE 0) /* ' */ ; PRAGMA query_only=0; VACUUM INTO '" + out + "'; SELECT * FROM (SELECT 1 /* ' */", nil},
		{"SELECT 1 WHERE 0 /* ' */ ; PRAGMA query_only=0; ATTACH '" + out + "' AS z; CREATE TABLE z.t AS SELECT * FROM main.transactions /* ' */", []string{"--limit", "0"}},
		{"SELECT 1 WHERE 0 /* ' */ ; PRAGMA query_only=0; DELETE FROM transactions /* ' */", []string{"--limit", "0"}},
		{"SELECT 1 WHERE 0 -- '\n; PRAGMA query_only=0; VACUUM INTO '" + out + "'", nil},
	}
	for _, p := range payloads {
		_, _, err := s3Run(t, append([]string{"ledger", "sql", p.q}, p.extra...)...)
		if code := s3ExitCode(err); code != 2 {
			t.Errorf("payload %q: exit %d (err %v), want usage error 2", p.q, code, err)
		}
		if _, statErr := os.Stat(out); statErr == nil {
			t.Fatalf("payload %q wrote %s", p.q, out)
		}
	}
	var rows []map[string]any
	s3RunJSON(t, &rows, "ledger", "sql", "SELECT COUNT(*) AS n FROM transactions")
	if len(rows) != 1 || rows[0]["n"] == float64(0) {
		t.Fatalf("mirror transactions changed after bypass attempts: %v", rows)
	}
}
