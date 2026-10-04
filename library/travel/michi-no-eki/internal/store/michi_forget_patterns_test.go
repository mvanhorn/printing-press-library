package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/store"
)

func seedMichiForgetFamilies(t *testing.T, s *store.Store) {
	t.Helper()
	for _, family := range []struct{ word, prefix, kind, venue string }{
		{"widget", "NORTH-", "widgets", "north"},
		{"widget", "SOUTH-", "widgets", "south"},
		{"widget", "OTHER-", "other", "north"},
		{"item", "ITEM-", "widgets", "north"},
	} {
		for _, entity := range []string{"Alpha", "Bravo"} {
			if _, err := s.DB().Exec(`INSERT INTO search_learnings(query_pattern,query_entities,resource_id,resource_type,venue,action,confidence,source) VALUES(?,?,?,?,?,'boost',2,'taught')`, strings.ToLower(entity)+" "+family.word, `["`+entity+`"]`, family.prefix+strings.ToLower(entity), family.kind, family.venue); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := patterns.Extract(s.DB(), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "widget {entity}", ResourceTemplate: "MANUAL-{entity:lowercase}", ResourceType: "widgets", Venue: "north", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
		t.Fatal(err)
	}
}

func TestMichiForgetInvalidatesOnlyAffectedInferredFamily(t *testing.T) {
	s := openLearnings(t)
	seedMichiForgetFamilies(t, s)
	ctx := context.Background()
	if n, err := s.ForgetLearnings(ctx, store.ForgetLearningsFilter{Query: "Alpha widget", ResourceID: "missing"}); err != nil || n != 0 {
		t.Fatalf("no-op delete=%d error=%v", n, err)
	}
	var before int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_patterns`).Scan(&before); err != nil || before != 5 {
		t.Fatalf("seed patterns=%d error=%v", before, err)
	}
	n, err := s.ForgetLearnings(ctx, store.ForgetLearningsFilter{Query: "Alpha widget", ResourceID: "NORTH-alpha"})
	if err != nil || n != 1 {
		t.Fatalf("forget=%d error=%v", n, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("patterns remaining=%d, want4: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Source == patterns.SourceInferred && row.Venue == "north" && row.ResourceType == "widgets" && row.QueryTemplate == "widget {entity}" {
			t.Fatalf("forgotten inferred family survived: %+v", row)
		}
	}
	var manual, teachings int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_patterns WHERE source='taught'`).Scan(&manual); err != nil || manual != 1 {
		t.Fatalf("manual patterns=%d error=%v", manual, err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings`).Scan(&teachings); err != nil || teachings != 7 {
		t.Fatalf("teachings=%d error=%v", teachings, err)
	}
}

func TestMichiForgetPatternFailureRollsBackTeachings(t *testing.T) {
	s := openLearnings(t)
	seedMichiForgetFamilies(t, s)
	if _, err := s.DB().Exec(`CREATE TRIGGER reject_pattern_delete BEFORE DELETE ON search_patterns WHEN OLD.source='inferred' BEGIN SELECT RAISE(ABORT,'test cleanup failure'); END`); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "Alpha widget", ResourceID: "NORTH-alpha"}); err == nil || n != 0 {
		t.Fatalf("failure not reported atomically: deleted=%d error=%v", n, err)
	}
	var teachings, rules int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings`).Scan(&teachings); err != nil || teachings != 8 {
		t.Fatalf("rollback teachings=%d error=%v", teachings, err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_patterns`).Scan(&rules); err != nil || rules != 5 {
		t.Fatalf("rollback rules=%d error=%v", rules, err)
	}
}
