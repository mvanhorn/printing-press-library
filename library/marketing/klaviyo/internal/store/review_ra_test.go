package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRAQueryRejectsUnsafeStatements(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	other := filepath.Join(t.TempDir(), "other.db")
	for _, tc := range []struct{ query, want string }{
		{"ATTACH DATABASE '" + other + "' AS b; SELECT * FROM b.resources", "only SELECT or WITH queries are allowed"},
		{"-- comment\nDELETE FROM resources", "only SELECT or WITH queries are allowed"},
		{"/* comment */ UPDATE resources SET data = '{}'", "only SELECT or WITH queries are allowed"},
		{"DETACH DATABASE b", "only SELECT or WITH queries are allowed"},
		{"PRAGMA query_only=OFF", "only SELECT or WITH queries are allowed"},
		{"VACUUM INTO '" + other + "'", "only SELECT or WITH queries are allowed"},
		{"SELECT 1; ATTACH DATABASE '" + other + "' AS b", "only one SQL statement is allowed"},
		{"SELECT 1; /* comment */ DELETE FROM resources", "only one SQL statement is allowed"},
		{"SELECT 1; SELECT 2", "only one SQL statement is allowed"},
		{"SELECT 1;;", "only one SQL statement is allowed"},
		{"/* nothing */", "only SELECT or WITH queries are allowed"},
		{"SELECT 'unfinished", "unterminated SQL quote"},
		{"SELECT 1 /* unfinished", "unterminated SQL comment"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			rows, err := s.Query(tc.query)
			if rows != nil {
				rows.Close()
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRAVerifiedReadOnlyOpenDoesNotCreateOrAdoptStores(t *testing.T) {
	owner := klaviyoStoreSession(time.Unix(1, 0).UTC())
	missing := filepath.Join(t.TempDir(), "missing", "data.db")
	reader, err := OpenVerifiedReadOnlyWithContext(context.Background(), missing, owner)
	if reader != nil {
		reader.Close()
	}
	if err == nil {
		t.Fatal("read-only SQL created a missing store")
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatalf("read-only open created directories: %v", err)
	}
	filename := filepath.Join(t.TempDir(), "data.db")
	writable, err := OpenVerifiedWithContext(context.Background(), filename, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer writable.Close()
	other := *owner
	other.ProfileName = "tenant-b"
	if reader, err := OpenVerifiedReadOnlyWithContext(context.Background(), filename, &other); err == nil || err.Error() != "verifying database tenant metadata: database tenant metadata does not match the verified client profile" {
		if reader != nil {
			reader.Close()
		}
		t.Fatalf("cross-tenant read-only open error = %v", err)
	}
	reader, err = OpenVerifiedReadOnlyWithContext(context.Background(), filename, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	// Bypass the prefix filter to prove that SQLite itself enforces read-only mode.
	if _, err := reader.DB().Exec(`CREATE TABLE forbidden(value TEXT)`); err == nil || err.Error() != "attempt to write a readonly database (8)" {
		t.Fatalf("read-only connection write error = %v", err)
	}
}

func TestRAQueryReadOnlyConnectionGuardsWithWrites(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type, id, data) VALUES ('profiles', 'p1', '{"keep":true}')`); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Query(`WITH chosen AS (SELECT 'p1') DELETE FROM resources WHERE id IN chosen RETURNING id`)
	if rows != nil {
		for rows.Next() {
			t.Error("read-only query returned a deleted row")
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	if err == nil || err.Error() != "attempt to write a readonly database (8)" {
		t.Fatalf("write error = %v", err)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE id='p1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("protected rows = %d, %v", count, err)
	}
	// A read query must not leave the store's ordinary connection in query_only mode.
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type, id, data) VALUES ('profiles', 'p2', '{}')`); err != nil {
		t.Fatalf("read-only query poisoned writable pool: %v", err)
	}
}

func TestRAQueryAllowsSingleSelectAndWith(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, query := range []string{
		"-- leading comment\n/* another */ SELECT ? AS value; -- trailing comment",
		"WITH value(v) AS (SELECT ?) SELECT v FROM value",
		"SELECT ? AS \"semi;colon\" /* ; DELETE */",
		"SELECT ? AS [semi;colon]",
		"SELECT ? AS `semi;colon`",
		"SELECT 'semi;''colon' || ?",
	} {
		rows, err := s.Query(query, "fixture")
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		if !rows.Next() {
			t.Fatalf("%s: no row: %v", query, rows.Err())
		}
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		want := "fixture"
		if query == "SELECT 'semi;''colon' || ?" {
			want = "semi;'colonfixture"
		}
		if value != want || rows.Next() || rows.Err() != nil {
			t.Fatalf("%s: result = %q, error %v", query, value, rows.Err())
		}
		rows.Close()
	}
}
