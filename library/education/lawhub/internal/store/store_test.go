package store

import "testing"

func TestOpenCreatesSchemaAndMigrations(t *testing.T) {
	db, path, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if path == "" {
		t.Fatal("empty db path")
	}
	for _, table := range []string{"tests", "attempts", "sections", "questions", "sync_log"} {
		if _, err := Count(db, table); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}
	for _, col := range []string{"answered", "answer_state_json", "difficulty"} {
		var found int
		rows, err := db.Query(`PRAGMA table_info(questions)`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var cid int
			var name, typ string
			var notnull int
			var dflt any
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
				t.Fatal(err)
			}
			if name == col {
				found = 1
			}
		}
		rows.Close()
		if found == 0 {
			t.Fatalf("missing questions.%s", col)
		}
	}
}
