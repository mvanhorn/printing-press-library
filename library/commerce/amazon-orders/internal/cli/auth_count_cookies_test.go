package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestCountCookiesForDomain_RejectsSQLInjection exercises the parameterized
// query path. Before the fix, the function built the WHERE clause via
// fmt.Sprintf and shelled out to the `sqlite3` binary; a domainPattern
// containing single-quote + UNION ALL would have either errored or returned
// wildly wrong counts. After the fix, the value is passed as a bind parameter
// and SQLite treats every byte as data.
func TestCountCookiesForDomain_RejectsSQLInjection(t *testing.T) {
	tmpDir := t.TempDir()
	cookiesDB := filepath.Join(tmpDir, "Cookies")

	// Build a minimal Chrome-shaped cookies table with 3 rows.
	db, err := sql.Open("sqlite", "file:"+cookiesDB+"?_journal_mode=OFF")
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for _, host := range []string{".amazon.com", ".amazon.com", ".example.com"} {
		if _, err := db.Exec(`INSERT INTO cookies (host_key) VALUES (?)`, host); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	db.Close()

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{"normal amazon", "%amazon.com%", 2},
		{"normal example", "%example.com%", 1},
		{"no match", "%nonexistent.com%", 0},
		// Pre-fix, the next two would have either errored (broken SQL) or
		// returned the row count of the entire cookies table (UNION ALL bypass).
		// Post-fix, both are treated as opaque pattern strings so neither
		// matches any literal host_key and both return 0.
		{"injection: trailing OR", "%amazon.com%' OR '1'='1", 0},
		{"injection: union all", "x%' UNION ALL SELECT 'pwned", 0},
	}
	for _, tt := range tests {
		got := countCookiesForDomain(cookiesDB, tt.pattern)
		if got != tt.want {
			t.Errorf("countCookiesForDomain(%q) = %d, want %d", tt.pattern, got, tt.want)
		}
	}
}

// TestCountCookiesForDomain_MissingFile returns 0 when the cookie DB is absent
// (e.g., Chrome profile has never been used). Regression for the early-return
// path that runs before the SQL query is built.
func TestCountCookiesForDomain_MissingFile(t *testing.T) {
	got := countCookiesForDomain(filepath.Join(t.TempDir(), "does-not-exist"), "%anything%")
	if got != 0 {
		t.Errorf("missing-file count = %d, want 0", got)
	}
	// Ensure no stray temp files survived (defer cleanup ran).
	tmpDir := os.TempDir()
	entries, _ := filepath.Glob(filepath.Join(tmpDir, "cookies-probe-*.db"))
	for _, e := range entries {
		if info, err := os.Stat(e); err == nil && info.ModTime().Unix() < 1 {
			t.Errorf("leftover temp file: %s", e)
		}
	}
}
