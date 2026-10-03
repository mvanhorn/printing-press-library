// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package patterns

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
	"path/filepath"
	"testing"
)

func TestPrefixVerificationIsLiteralUniqueAndResourceScoped(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "prefix.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE resources(resource_type TEXT,id TEXT,data TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ kind, id string }{{"source", "alpha-a"}, {"source", "alpha-b"}, {"source", "unique-a"}, {"other", "unique-b"}, {"source", "literal%a"}, {"source", "literal_a"}, {"source", "literalXa"}} {
		if _, err := db.Exec(`INSERT INTO resources VALUES(?,?, '{}')`, row.kind, row.id); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		prefix, want string
		ok           bool
	}{{"alpha*", "", false}, {"missing*", "", false}, {"unique*", "unique-a", true}, {"literal%*", "literal%a", true}, {"literal_*", "literal_a", true}}
	for _, tc := range cases {
		hit, ok := verifyCandidate(context.Background(), db, tc.prefix, "source", StrategySubstituteThenSearchPrefix, false)
		if ok != tc.ok || hit.ResourceID != tc.want {
			t.Fatalf("prefix %q got=%#v ok=%v", tc.prefix, hit, ok)
		}
	}
}

func TestExplicitTeachingUpdatesScopeAndInferencePreservesPayload(t *testing.T) {
	db := openTestDB(t)
	original := samplePattern()
	original.Source = SourceInferred
	id, _, err := Upsert(db, original)
	if err != nil {
		t.Fatal(err)
	}
	taught := original
	taught.Source = SourceTaught
	taught.ResourceType = "pitches"
	taught.EntityKind = "country"
	taught.Venue = "kanto"
	taught.ExampleQuery = "pitch Japan"
	taught.ExampleResource = "pitch-JP"
	for _, clear := range []bool{false, true} {
		if clear {
			taught.Venue = ""
			taught.ExampleQuery = ""
			taught.ExampleResource = ""
		}
		got, inserted, err := Upsert(db, taught)
		if err != nil || inserted || got != id {
			t.Fatal(got, inserted, err)
		}
		if _, _, err := Upsert(db, original); err != nil {
			t.Fatal(err)
		}
		rows, err := List(db, ListFilter{})
		if err != nil || len(rows) != 1 {
			t.Fatal(rows, err)
		}
		r := rows[0]
		if r.ID != id || r.Source != SourceTaught || r.ResourceType != taught.ResourceType || r.EntityKind != taught.EntityKind || r.Venue != taught.Venue || r.ExampleQuery != taught.ExampleQuery || r.ExampleResource != taught.ExampleResource {
			t.Fatalf("explicit payload differs after inference %#v want %#v", r, taught)
		}
	}
}
