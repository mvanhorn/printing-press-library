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
