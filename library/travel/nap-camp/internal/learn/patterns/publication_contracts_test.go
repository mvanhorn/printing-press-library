// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package patterns

import (
	"context"
	"database/sql"
	"fmt"
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

func TestApplyCarriesActuallyVerifiedEntityAfterEarlierMiss(t *testing.T) {
	for _, strategy := range []string{StrategySubstitute, StrategySubstituteThenSearchPrefix} {
		t.Run(strategy, func(t *testing.T) {
			db := openApplyTestDB(t)
			resource, template := "pitch-BRAVO", "pitch-{entity:uppercase}"
			if strategy == StrategySubstituteThenSearchPrefix {
				resource += "-session"
				template += "*"
			}
			seedResource(t, db, "source", resource, `{}`)
			if _, _, err := Upsert(db, Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: template, ResourceType: "source", Strategy: strategy, EntityKind: "uppercase", Source: SourceTaught}); err != nil {
				t.Fatal(err)
			}
			hits, err := Apply(context.Background(), db, "pitch Alpha Bravo", "pitch", []string{"Alpha", "Bravo"}, Opts{})
			if err != nil || len(hits) != 1 || hits[0].ResourceID != resource || hits[0].BoundEntity != "Bravo" {
				t.Fatalf("binding did not follow verified candidate: %#v %v", hits, err)
			}
		})
	}
}

func TestApplyRetainsStandaloneCapsWhenRecallRequestsAllCandidates(t *testing.T) {
	db := openApplyTestDB(t)
	for i := 0; i < 12; i++ {
		seedResource(t, db, "source", fmt.Sprintf("pitch-ALPHA-%d", i), `{}`)
		if _, _, err := Upsert(db, Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: fmt.Sprintf("pitch-{entity:uppercase}-%d", i), ResourceType: "source", Strategy: StrategySubstitute, EntityKind: "uppercase", Source: SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name  string
		opts  Opts
		count int
	}{{"default", Opts{}, 10}, {"explicit", Opts{Limit: 2}, 2}, {"caller-filtering", Opts{NoLimit: true}, 12}} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := Apply(context.Background(), db, "pitch Alpha", "pitch", []string{"Alpha"}, tc.opts)
			if err != nil || len(hits) != tc.count {
				t.Fatalf("candidate cap contract changed: %d want%d %v", len(hits), tc.count, err)
			}
		})
	}
}

func TestApplyAllBindingsIsOptInAndKeepsStandaloneFirstBinding(t *testing.T) {
	db := openApplyTestDB(t)
	seedResource(t, db, "source", "pitch-ALPHA", `{}`)
	seedResource(t, db, "source", "pitch-BETA", `{}`)
	if _, _, err := Upsert(db, Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:uppercase}",
		ResourceType: "source", Strategy: StrategySubstitute, EntityKind: "uppercase", Source: SourceTaught}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		opts  Opts
		count int
	}{
		{"standalone", Opts{}, 1}, {"standalone-explicit-limit", Opts{Limit: 1}, 1},
		{"all-bindings", Opts{AllBindings: true, NoLimit: true}, 2}, {"all-bindings-result-limit", Opts{AllBindings: true, Limit: 1}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := Apply(context.Background(), db, "Alpha Beta pitch", "pitch", []string{"Alpha", "Beta"}, tc.opts)
			if err != nil || len(hits) != tc.count || hits[0].ResourceID != "pitch-ALPHA" || hits[0].BoundEntity != "Alpha" {
				t.Fatalf("binding contract: %#v %v", hits, err)
			}
			if tc.count == 2 && (hits[1].ResourceID != "pitch-BETA" || hits[1].BoundEntity != "Beta") {
				t.Fatalf("second binding not carried: %#v", hits)
			}
		})
	}
}
