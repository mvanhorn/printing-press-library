// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package patterns

import (
	"context"
	"fmt"
	"testing"
)

func TestVerifiedPrefixIsUniqueLiteralAndTypeScoped(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		ids          []string
		want         string
	}{
		{"unique", "spot/108", []string{"spot/1086"}, "spot/1086"},
		{"carstay-unique-id", "632c59b82b614b99a252d1", []string{"632c59b82b614b99a252d1b2"}, "632c59b82b614b99a252d1b2"},
		{"carstay-ambiguous-id", "632c59b82b614b99a252d1", []string{"632c59b82b614b99a252d1b2", "632c59b82b614b99a252d1b3"}, ""},
		{"ambiguous", "spot/108", []string{"spot/1086", "spot/1087"}, ""},
		{"literal-percent", "spot/%", []string{"spot/%one", "spot/other"}, "spot/%one"},
		{"literal-underscore", "spot/_", []string{"spot/_one", "spot/xone"}, "spot/_one"},
		{"case-sensitive", "Spot/", []string{"Spot/one", "spot/two"}, "Spot/one"},
		{"no-match", "spot/missing", []string{"spot/one"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openApplyTestDB(t)
			for _, id := range tc.ids {
				seedResource(t, db, "directory", id, `{}`)
			}
			seedResource(t, db, "other", tc.prefix+"not-a-directory-entry", `{}`)
			hit, ok := verifyCandidate(context.Background(), db, tc.prefix+"*", "directory", StrategySubstituteThenSearchPrefix, false)
			if ok != (tc.want != "") || (ok && hit.ResourceID != tc.want) {
				t.Fatalf("hit=%+v ok=%v want=%q", hit, ok, tc.want)
			}
		})
	}
}

func TestManualPatternPromotionNeverDowngrades(t *testing.T) {
	db := openApplyTestDB(t)
	p := Pattern{QueryTemplate: "widgets {entity}", ResourceTemplate: "spot/{entity:lowercase}", ResourceType: "directory", Strategy: StrategySubstitute, EntityKind: "lowercase", Source: SourceInferred}
	id, _, err := Upsert(db, p)
	if err != nil {
		t.Fatal(err)
	}
	p.Source = SourceTaught
	if _, _, err = Upsert(db, p); err != nil {
		t.Fatal(err)
	}
	p.Source = SourceInferred
	if _, _, err = Upsert(db, p); err != nil {
		t.Fatal(err)
	}
	rows, err := List(db, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != id || rows[0].Source != SourceTaught {
		t.Fatalf("manual source was lost: %+v", rows)
	}
}

func TestExplicitTeachingReplacesScopeAndInferencePreservesPayload(t *testing.T) {
	for _, venue := range []string{"new-venue", ""} {
		t.Run("venue="+venue, func(t *testing.T) {
			db := openApplyTestDB(t)
			inferred := Pattern{QueryTemplate: "widgets {entity}", ResourceTemplate: "spot/{entity:lowercase}", ResourceType: "activities", Venue: "old-venue", Strategy: StrategySubstitute, EntityKind: "uppercase", Source: SourceInferred, ExampleQuery: "old query", ExampleResource: "old resource"}
			id, _, err := Upsert(db, inferred)
			if err != nil {
				t.Fatal(err)
			}
			manual := inferred
			manual.Source, manual.ResourceType, manual.Venue, manual.EntityKind = SourceTaught, "directory", venue, "lowercase"
			manual.ExampleQuery, manual.ExampleResource = "new query", "new resource"
			updatedID, inserted, err := Upsert(db, manual)
			if err != nil || inserted || updatedID != id {
				t.Fatalf("update id=%d inserted=%v err=%v", updatedID, inserted, err)
			}
			if _, _, err := Upsert(db, inferred); err != nil {
				t.Fatal(err)
			}
			rows, err := List(db, ListFilter{})
			if err != nil || len(rows) != 1 {
				t.Fatalf("rows=%+v err=%v", rows, err)
			}
			got := rows[0]
			if got.ID != id || got.Source != SourceTaught || got.ResourceType != manual.ResourceType || got.Venue != venue || got.EntityKind != manual.EntityKind || got.ExampleQuery != manual.ExampleQuery || got.ExampleResource != manual.ExampleResource {
				t.Fatalf("recorded scope/payload was lost: %+v", got)
			}
			manual.ExampleQuery, manual.ExampleResource = "", ""
			if _, _, err := Upsert(db, manual); err != nil {
				t.Fatal(err)
			}
			rows, err = List(db, ListFilter{})
			if err != nil || rows[0].ExampleQuery != "" || rows[0].ExampleResource != "" {
				t.Fatalf("explicit cleared examples not recorded: %+v %v", rows, err)
			}
		})
	}
}

func TestCarstayStandaloneApplyKeepsDefaultAndExplicitCaps(t *testing.T) {
	db := openApplyTestDB(t)
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("fixture-%02d-alpha", i)
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{}')`, id); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Upsert(db, Pattern{QueryTemplate: "widget {entity}", ResourceTemplate: fmt.Sprintf("fixture-%02d-{entity:lowercase}", i), ResourceType: "widgets", Strategy: StrategySubstitute, EntityKind: "lowercase", Source: SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		opts Opts
		want int
	}{{Opts{}, 10}, {Opts{Limit: 1}, 1}, {Opts{NoLimit: true, Limit: 1}, 12}} {
		hits, err := Apply(context.Background(), db, "Alpha widget", "widget", []string{"Alpha"}, tc.opts)
		if err != nil || len(hits) != tc.want {
			t.Fatalf("standalone/candidate bounds: count%d want%d err%v", len(hits), tc.want, err)
		}
	}
}
