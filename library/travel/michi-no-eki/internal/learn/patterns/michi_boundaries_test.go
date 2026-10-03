package patterns

import (
	"context"
	"testing"
)

func TestMichiPrefixVerificationRequiresOneLiteralMatch(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, want string
		ids                []string
	}{
		{"absent", "ABC", "", []string{"other"}},
		{"unique", "ABC", "ABC-1", []string{"ABC-1"}},
		{"ambiguous", "ABC", "", []string{"ABC-1", "ABC-2"}},
		{"underscore is literal", "AB_", "AB_1", []string{"AB_1", "ABX1"}},
		{"percent is literal", "AB%", "AB%1", []string{"AB%1", "ABX1"}},
		{"case is literal", "ab", "ab1", []string{"ab1", "AB1"}},
		{"empty is not discovery", "", "", []string{"anything"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openApplyTestDB(t)
			for _, id := range tc.ids {
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{}')`, id); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('other',?,'{}')`, tc.prefix+"-other"); err != nil {
				t.Fatal(err)
			}
			hit, ok := verifyCandidate(context.Background(), db, tc.prefix+"*", "widgets", StrategySubstituteThenSearchPrefix, false)
			if ok != (tc.want != "") || hit.ResourceID != tc.want {
				t.Fatalf("candidate=%+v ok=%v, want=%q", hit, ok, tc.want)
			}
		})
	}
}

func TestMichiExplicitPatternPromotesAndKeepsManualSource(t *testing.T) {
	db := openApplyTestDB(t)
	p := Pattern{QueryTemplate: "{entity} widget", ResourceTemplate: "PREFIX-{entity:lowercase}", ResourceType: "widgets", Strategy: StrategySubstitute, EntityKind: "lowercase", Source: SourceInferred}
	id, _, err := Upsert(db, p)
	if err != nil {
		t.Fatal(err)
	}
	p.Source = SourceTaught
	if got, inserted, err := Upsert(db, p); err != nil || inserted || got != id {
		t.Fatalf("promotion id=%d inserted=%v error=%v", got, inserted, err)
	}
	p.Source = SourceInferred
	if _, _, err := Upsert(db, p); err != nil {
		t.Fatal(err)
	}
	var source string
	if err := db.QueryRow(`SELECT source FROM search_patterns WHERE id=?`, id).Scan(&source); err != nil {
		t.Fatal(err)
	}
	if source != SourceTaught {
		t.Fatalf("explicit pattern downgraded to %q", source)
	}
}
