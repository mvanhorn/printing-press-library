package cli

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCountCookiesForDomainUsesParameterizedQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "Cookies")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite fixture: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT)`); err != nil {
		t.Fatalf("create cookies table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO cookies (host_key) VALUES (?), (?)`, ".pointhound.com", "evil' OR 1=1 --"); err != nil {
		t.Fatalf("insert cookies: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite fixture: %v", err)
	}

	got := countCookiesForDomain(dbPath, "%evil' OR 1=1 --%")
	if got != 1 {
		t.Fatalf("cookie count with quoted domain pattern: want 1, got %d", got)
	}
}
