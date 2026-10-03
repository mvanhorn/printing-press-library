package hostelworld

import (
	"database/sql"
	_ "modernc.org/sqlite"
	"testing"
)

func TestEmbeddedSQLiteHasWALResetFix(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version string
	if err := db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != "3.51.3" {
		t.Fatalf("embedded SQLite=%s; expected reviewed WAL-reset fix3.51.3", version)
	}
	t.Log("embedded SQLite", version)
}
