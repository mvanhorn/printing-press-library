// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCountCookiesForDomain verifies the cookie probe counts matching rows and,
// critically, that a domain pattern containing SQL metacharacters is bound as a
// literal LIKE argument rather than interpolated — so it cannot break out of the
// query or execute injected statements.
func TestCountCookiesForDomain(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "Cookies")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{".ankiweb.net", "ankiweb.net", "other.com", ".ankiuser.net"} {
		if _, err := db.Exec(`INSERT INTO cookies (host_key) VALUES (?)`, h); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	if got := countCookiesForDomain(dbPath, "%ankiweb.net%"); got != 2 {
		t.Errorf("ankiweb.net count = %d, want 2", got)
	}

	// Injection attempt: must be treated as a literal LIKE pattern (matches
	// nothing) and must not execute the embedded DROP TABLE.
	if got := countCookiesForDomain(dbPath, "%'; DROP TABLE cookies; --"); got != 0 {
		t.Errorf("injection pattern count = %d, want 0", got)
	}
	// Table is intact (the DROP did not run): "%" matches all 4 rows.
	if got := countCookiesForDomain(dbPath, "%"); got != 4 {
		t.Errorf("post-injection total = %d, want 4 (table intact)", got)
	}
}
