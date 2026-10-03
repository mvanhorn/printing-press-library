// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
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
