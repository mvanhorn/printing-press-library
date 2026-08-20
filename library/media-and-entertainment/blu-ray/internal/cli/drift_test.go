package cli

// PATCH: Regression coverage for literal LIKE matching in drift filters.

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEscapeLikePatternTreatsUnderscoreLiterally(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE sitemap_snapshot (sitemap_name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sitemap_snapshot (sitemap_name) VALUES (?), (?)`, "sitemap_bluraymovies_0", "sitemap_bluraymoviesA"); err != nil {
		t.Fatal(err)
	}
	pattern := "%" + escapeLikePattern("bluraymovies_") + "%"
	rows, err := db.Query(`SELECT sitemap_name FROM sitemap_snapshot WHERE sitemap_name LIKE ? ESCAPE '\' ORDER BY sitemap_name`, pattern)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "sitemap_bluraymovies_0" {
		t.Fatalf("LIKE matches = %#v, want only sitemap_bluraymovies_0", got)
	}
}
