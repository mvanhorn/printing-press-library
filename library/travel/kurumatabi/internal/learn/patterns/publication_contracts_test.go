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
