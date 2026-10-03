package patterns

import (
	"context"
	"testing"
)

func TestVerifiedPrefixIsUniqueLiteralAndTypeScoped(t *testing.T) {
	for _, tc := range []struct {
		name, prefix string
		ids          []string
		want         string
	}{
		{"unique", "park/108", []string{"park/1086"}, "park/1086"},
		{"ambiguous", "park/108", []string{"park/1086", "park/1087"}, ""},
		{"literal-percent", "park/%", []string{"park/%one", "park/other"}, "park/%one"},
		{"literal-underscore", "park/_", []string{"park/_one", "park/xone"}, "park/_one"},
		{"case-sensitive", "Park/", []string{"Park/one", "park/two"}, "Park/one"},
		{"no-match", "park/missing", []string{"park/one"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openApplyTestDB(t)
			for _, id := range tc.ids {
				seedResource(t, db, "parks", id, `{}`)
			}
			seedResource(t, db, "other", tc.prefix+"not-a-park", `{}`)
			hit, ok := verifyCandidate(context.Background(), db, tc.prefix+"*", "parks", StrategySubstituteThenSearchPrefix, false)
			if ok != (tc.want != "") || (ok && hit.ResourceID != tc.want) {
				t.Fatalf("hit=%+v ok=%v want=%q", hit, ok, tc.want)
			}
		})
	}
}

func TestManualPatternPromotionNeverDowngrades(t *testing.T) {
	db := openApplyTestDB(t)
	p := Pattern{QueryTemplate: "widgets {entity}", ResourceTemplate: "park/{entity:lowercase}", ResourceType: "parks", Strategy: StrategySubstitute, EntityKind: "lowercase", Source: SourceInferred}
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

func TestApplyStandaloneCapsAndRecallCandidatePassRemainDistinct(t *testing.T) {
	db := openApplyTestDB(t)
	for i := 0; i < 12; i++ {
		prefix := "park-" + string(rune('a'+i)) + "-"
		seedResource(t, db, "source", prefix+"A1", `{}`)
		if _, _, err := Upsert(db, Pattern{QueryTemplate: "park {entity}", ResourceTemplate: prefix + "{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Strategy: StrategySubstitute, Source: SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		opts Opts
		want int
	}{
		{"default", Opts{}, 10}, {"explicit", Opts{Limit: 1}, 1}, {"recall-candidates", Opts{Limit: 1, NoLimit: true}, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := Apply(context.Background(), db, "A1 park", "park", []string{"A1"}, tc.opts)
			if err != nil || len(hits) != tc.want {
				t.Fatalf("hits=%d want=%d err=%v", len(hits), tc.want, err)
			}
		})
	}
}

func TestApplyAllBindingsIsAnExplicitRecallCandidateOption(t *testing.T) {
	db := openApplyTestDB(t)
	for _, id := range []string{"park-A1", "park-B1"} {
		seedResource(t, db, "source", id, `{}`)
	}
	if _, _, err := Upsert(db, Pattern{QueryTemplate: "park {entity}", ResourceTemplate: "park-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Strategy: StrategySubstitute, Source: SourceTaught}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		opts Opts
		want int
	}{
		{"standalone-first", Opts{}, 1}, {"recall-candidates", Opts{NoLimit: true, AllBindings: true}, 2}, {"explicit-standalone-cap", Opts{Limit: 1, AllBindings: true}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits, err := Apply(context.Background(), db, "A1 B1 park", "park", []string{"A1", "B1"}, tc.opts)
			if err != nil || len(hits) != tc.want || hits[0].BoundEntity != "A1" {
				t.Fatalf("binding/cap changed %#v %v", hits, err)
			}
			if tc.want == 2 && hits[1].BoundEntity != "B1" {
				t.Fatalf("later bound entity lost %#v", hits)
			}
		})
	}
}
