package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2AuthoritativeSnapshots(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, raw := range []string{`{"id":"d","title":"old","tags":{"a":{},"b":{}}}`, `{"id":"d","title":"","tags":{"a":{}}}`} {
		if err := s.UpsertDocuments(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	var removed any
	var title string
	if err := s.db.QueryRow(`SELECT json_extract(data,'$.tags.b'),title FROM documents WHERE id='d'`).Scan(&removed, &title); err != nil {
		t.Fatal(err)
	}
	if removed != nil || title != "" {
		t.Fatalf("stale snapshot: b=%v title=%q", removed, title)
	}
	for _, raw := range []string{`{"id":1,"text":"passage","note":"obsoleteword"}`, `{"id":1,"text":"passage","note":""}`} {
		if err := s.UpsertHighlights(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Search("obsoleteword", 50, 0)
	if err != nil || len(rows) != 0 {
		t.Fatalf("cleared note remains searchable: %s %v", rows, err)
	}
	for _, raw := range []string{`{"id":2,"title":"book","tags":{"a":{},"b":{}},"author":"old"}`, `{"id":2,"tags":{"a":{}},"author":null}`} {
		if err := s.UpsertBooks(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	var b, author any
	if err := s.db.QueryRow(`SELECT json_extract(data,'$.tags.b'),json_extract(data,'$.author'),title FROM books WHERE id='2'`).Scan(&b, &author, &title); err != nil {
		t.Fatal(err)
	}
	if b != nil || author != nil || title != "book" {
		t.Fatalf("assembled book mismatch b=%v author=%v title=%s", b, author, title)
	}
}
func TestE2SearchIgnoresOpaqueID(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Upsert("documents", "gaix", json.RawMessage(`{"title":"ordinary prose"}`)); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Search("AI", 50, 0)
	if err != nil || len(rows) != 0 {
		t.Fatalf("opaque ID matched: %s %v", rows, err)
	}
	count, err := s.SearchCountContext(context.Background(), "AI")
	if err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
func TestE2MigrationBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE resources(id TEXT PRIMARY KEY,resource_type TEXT,data JSON,synced_at DATETIME,updated_at DATETIME); INSERT INTO resources VALUES('keep','documents','{"id":"keep"}',NULL,NULL); PRAGMA user_version=4;`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	info, err := os.Stat(path + ".v4.bak")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup mode: %v %v", info, err)
	}
	// Simulate a second opener observing an already-created backup.
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`PRAGMA user_version=4`); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
	backup, err := sql.Open("sqlite", "file:"+path+".v4.bak?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var version, count int
	if err := backup.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := backup.QueryRow(`SELECT count(*) FROM resources WHERE id='keep'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if version != 4 || count != 1 {
		t.Fatalf("backup version=%d count=%d", version, count)
	}
}

func TestE2SearchPageSnapshot(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"b", "a", "c"} {
		if err := s.Upsert("documents", id, json.RawMessage(`{"title":"AI systems"}`)); err != nil {
			t.Fatal(err)
		}
	}
	rows, count, err := s.SearchPageContext(context.Background(), "AI", 1, 1)
	if err != nil || len(rows) != 1 || count != 3 {
		t.Fatalf("rows=%s count=%d err=%v", rows, count, err)
	}
}

func TestE2BackfillsTypedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE highlights(id TEXT PRIMARY KEY,data JSON NOT NULL,synced_at DATETIME); INSERT INTO highlights VALUES('7','{"id":7,"external_id":"external","end_location":42,"note":"memo"}',NULL); CREATE TABLE books(id TEXT PRIMARY KEY,data JSON NOT NULL,synced_at DATETIME); INSERT INTO books VALUES('1','{"id":1,"title":"Backfilled book"}',NULL); CREATE TABLE documents(id TEXT PRIMARY KEY,data JSON NOT NULL,synced_at DATETIME); INSERT INTO documents VALUES('d','{"id":"d","title":"Backfilled document"}',NULL); PRAGMA user_version=4;`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for table, want := range map[string]string{"books": "Backfilled book", "documents": "Backfilled document"} {
		var title string
		if err := s.db.QueryRow(`SELECT title FROM ` + table).Scan(&title); err != nil || title != want {
			t.Fatalf("%s title=%q err=%v", table, title, err)
		}
	}
	var external, note string
	var end int
	if err := s.db.QueryRow(`SELECT external_id,end_location,note FROM highlights WHERE id='7'`).Scan(&external, &end, &note); err != nil {
		t.Fatal(err)
	}
	if external != "external" || end != 42 || note != "memo" {
		t.Fatalf("backfill external=%q end=%d note=%q", external, end, note)
	}
}
func TestE2PageTombstonesAtomic(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.UpsertBooks(json.RawMessage(`{"id":1,"title":"book"}`)); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertHighlights(json.RawMessage(`{"id":2,"text":"vanishingword"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteReadwiseTombstones(context.Background(), []string{"1"}, []string{"2", ""}); err == nil {
		t.Fatal("invalid page succeeded")
	}
	for _, resource := range []string{"books", "highlights"} {
		count, err := s.Count(resource)
		if err != nil || count != 1 {
			t.Fatalf("partial delete resource=%s count=%d err=%v", resource, count, err)
		}
	}
	if _, err := s.DeleteReadwiseTombstones(context.Background(), []string{"1"}, []string{"2"}); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"books", "highlights"} {
		count, err := s.Count(resource)
		if err != nil || count != 0 {
			t.Fatalf("tombstone remains resource=%s count=%d err=%v", resource, count, err)
		}
	}
	rows, err := s.Search("vanishingword", 10, 0)
	if err != nil || len(rows) != 0 {
		t.Fatalf("FTS tombstone remains: %s %v", rows, err)
	}
}

func TestE2SearchStableOrder(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"b", "a", "c"} {
		raw, _ := json.Marshal(map[string]string{"id": id, "title": "AI systems"})
		if err := s.Upsert("documents", id, raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"AI", "systems"} {
		rows, err := s.Search(query, 3, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 3 {
			t.Fatalf("query=%s rows=%s", query, rows)
		}
		for i, raw := range rows {
			var obj map[string]string
			if err := json.Unmarshal(raw, &obj); err != nil {
				t.Fatal(err)
			}
			if obj["id"] != []string{"a", "b", "c"}[i] {
				t.Fatalf("query=%s unstable order=%s", query, rows)
			}
		}
	}
}

func TestFStoreRejectsURI(t *testing.T) {
	for _, suffix := range []string{"?mode=ro", "#fragment"} {
		path := filepath.Join(t.TempDir(), "data.db") + suffix
		for _, open := range []func(string) (*Store, error){Open, OpenReadOnly} {
			s, err := open(path)
			if s != nil {
				s.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "URI delimiters") {
				t.Fatalf("accepted path: %s %v", path, err)
			}
		}
	}
}
