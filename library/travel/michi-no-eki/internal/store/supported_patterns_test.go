// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/store"
)

func seedSurvivingPattern(t *testing.T, s *store.Store, prefix bool) int64 {
	t.Helper()
	for i, row := range []struct{ country, code string }{{"Germany", "DE"}, {"France", "FR"}, {"Japan", "JP"}, {"Italy", "IT"}} {
		resource := "pitch-" + row.code
		if prefix {
			resource += "-session-" + strings.ToLower(row.code)
		}
		if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country',?,?,'seeded')`, row.country, row.code); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,'{}')`, resource); err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch " + row.country, QueryEntities: []string{row.country}, ResourceID: resource, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != 1 {
		t.Fatalf("actual extract n=%d err=%v", n, err)
	}
	listed, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(listed) != 1 {
		t.Fatalf("fixture pattern %#v %v", listed, err)
	}
	if listed[0].ExampleResource != map[bool]string{false: "pitch-JP", true: "pitch-JP-session-jp"}[prefix] {
		t.Fatalf("deleted example fixture missing %#v", listed[0])
	}
	return listed[0].ID
}

func generalizedItaly(t *testing.T, s *store.Store) bool {
	t.Helper()
	r, err := learn.Recall(context.Background(), s.DB(), "pitch Italy", learn.Opts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range r.Results {
		if hit.Source == "pattern" && strings.HasPrefix(hit.ResourceID, "pitch-IT") {
			return true
		}
	}
	return false
}

func forgetJapan(t *testing.T, s *store.Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := s.ForgetLearnings(ctx, store.ForgetLearningsFilter{Query: "pitch Japan", All: true})
	if err != nil || n != 1 {
		t.Fatalf("forget Japan n=%d err=%v", n, err)
	}
}

func TestForgetPreservesActuallySupportedGeneralization(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact", true: "prefix"}[prefix], func(t *testing.T) {
			s := openLearnings(t)
			id := seedSurvivingPattern(t, s, prefix)
			if !generalizedItaly(t, s) {
				t.Fatal("actual initial pattern did not generalize")
			}
			// A DB lookup outside the owning transaction would deadlock here.
			s.DB().SetMaxOpenConns(1)
			forgetJapan(t, s)
			s.DB().SetMaxOpenConns(4)
			if !generalizedItaly(t, s) {
				t.Fatal("two retained distinct bindings lost generalization")
			}
			rows, err := patterns.List(s.DB(), patterns.ListFilter{})
			if err != nil || len(rows) != 1 || rows[0].ID != id {
				t.Fatalf("supported identity lost %#v %v", rows, err)
			}
			if strings.Contains(strings.ToLower(rows[0].ExampleQuery), "japan") || strings.Contains(rows[0].ExampleResource, "JP") {
				t.Fatalf("deleted example provenance retained %#v", rows[0])
			}
			n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch France", All: true})
			if err != nil || n != 1 {
				t.Fatal(n, err)
			}
			if generalizedItaly(t, s) {
				t.Fatal("one remaining supporter retained unsupported inference")
			}
		})
	}
}

func TestForgetSupportIsNotJustAFamilyRowCount(t *testing.T) {
	for _, variant := range []string{"conflicting-resource", "missing-lookup", "duplicate-entity", "multi-entity"} {
		t.Run(variant, func(t *testing.T) {
			s := openLearnings(t)
			seedSurvivingPattern(t, s, false)
			var mutationErr error
			switch variant {
			case "conflicting-resource":
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET resource_id='unrelated-FR' WHERE query_pattern='pitch france'`)
			case "missing-lookup":
				_, mutationErr = s.DB().Exec(`DELETE FROM entity_lookups WHERE canonical='France'`)
			case "duplicate-entity":
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET query_pattern='pitch germany',query_entities='["Germany"]',resource_id='pitch-DE',action='hide' WHERE query_pattern='pitch france'`)
			case "multi-entity":
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET query_pattern='pitch france germany',query_entities='["France","Germany"]' WHERE query_pattern='pitch france'`)
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			forgetJapan(t, s)
			if generalizedItaly(t, s) {
				t.Fatalf("%s rows falsely prove a surviving pattern", variant)
			}
			rows, err := patterns.List(s.DB(), patterns.ListFilter{})
			if err != nil || len(rows) != 0 {
				t.Fatalf("unsupported rule survived %#v %v", rows, err)
			}
		})
	}
}

func TestForgetRetainedSupportOutsideExtractionWindow(t *testing.T) {
	s := openLearnings(t)
	seedSurvivingPattern(t, s, false)
	for i := 0; i < 60; i++ {
		query := "unrelated " + strings.Repeat("x", i+1)
		if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: query, ResourceID: query, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	forgetJapan(t, s)
	if !generalizedItaly(t, s) {
		t.Fatal("valid old retained support was lost to newest-50 truncation")
	}
}

func TestForgetSupportedPatternUpdateFailureRollsBack(t *testing.T) {
	s := openLearnings(t)
	id := seedSurvivingPattern(t, s, false)
	if _, err := s.DB().Exec(`CREATE TRIGGER prevent_example_update BEFORE UPDATE ON search_patterns BEGIN SELECT RAISE(ABORT,'fixture update rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", All: true}); err == nil {
		t.Fatal("failed supported-rule update reported success")
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE query_pattern='pitch japan'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("teaching rollback count=%d err=%v", count, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].ExampleResource != "pitch-JP" {
		t.Fatalf("pattern rollback lost provenance %#v %v", rows, err)
	}
}

func TestForgetIgnoresUnusableCohortRowsWhenTwoSupportersRemain(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, variant := range []string{"missing-lookup", "conflicting-resource", "multi-entity"} {
			t.Run(map[bool]string{false: "exact", true: "prefix"}[prefix]+"/"+variant, func(t *testing.T) {
				s := openLearnings(t)
				id := seedSurvivingPattern(t, s, prefix)
				query := "pitch Spain"
				entities := []string{"Spain"}
				resource := "pitch-ES"
				if variant == "conflicting-resource" {
					if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country','Spain','ES','seeded')`); err != nil {
						t.Fatal(err)
					}
					resource = "unrelated-ES"
				}
				if variant == "multi-entity" {
					query = "pitch Spain Germany"
					entities = []string{"Spain", "Germany"}
				}
				if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: query, QueryEntities: entities, ResourceID: resource, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
					t.Fatal(err)
				}
				s.DB().SetMaxOpenConns(1)
				forgetJapan(t, s)
				s.DB().SetMaxOpenConns(4)
				if !generalizedItaly(t, s) {
					t.Fatal("unusable cohort row vetoed two compatible supporting teachings")
				}
				rows, err := patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 1 || rows[0].ID != id {
					t.Fatalf("supported identity lost %#v %v", rows, err)
				}
				if strings.Contains(rows[0].ExampleQuery, "spain") || strings.Contains(rows[0].ExampleResource, "ES") || strings.Contains(rows[0].ExampleResource, "JP") {
					t.Fatalf("example must be an actual surviving supporter: %#v", rows[0])
				}
			})
		}
	}
}

func TestForgetRetainedLookupFailureRollsBack(t *testing.T) {
	s := openLearnings(t)
	id := seedSurvivingPattern(t, s, false)
	if _, err := s.DB().Exec(`DROP TABLE entity_lookups`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", All: true}); err == nil {
		t.Fatal("lookup SQL error must abort, not count as unusable evidence")
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE query_pattern='pitch japan'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("teaching was not rolled back: %d %v", n, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].ExampleResource != "pitch-JP" {
		t.Fatalf("pattern/provenance was not rolled back: %#v %v", rows, err)
	}
}

func TestForgetCannotRetainOrRevivePatternFromNegativeTeachings(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, action := range []string{store.LearningActionHide, store.LearningActionAlias} {
			t.Run(map[bool]string{false: "exact", true: "prefix"}[prefix]+"/"+action, func(t *testing.T) {
				s := openLearnings(t)
				seedSurvivingPattern(t, s, prefix)
				if n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch France", Action: store.LearningActionBoost, All: true}); err != nil || n != 1 {
					t.Fatal(n, err)
				}
				resource := "pitch-FR"
				if prefix {
					resource += "-session-fr"
				}
				if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch France", QueryEntities: []string{"France"}, ResourceID: resource, ResourceType: "source", Action: action, AliasTarget: "replacement-FR", Source: store.LearningSourceTaught}); err != nil {
					t.Fatal(err)
				}
				forgetJapan(t, s)
				if generalizedItaly(t, s) {
					t.Fatalf("%s counted as positive retained support", action)
				}
				if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != 0 {
					t.Fatalf("negative teaching revived inference: n=%d err=%v", n, err)
				}
				if generalizedItaly(t, s) {
					t.Fatal("negative teaching re-extraction revived pattern")
				}
			})
		}
	}
}

func TestExtractUsesOnlyPositiveEligibleTeachingSources(t *testing.T) {
	for _, tc := range []struct {
		name, action, source string
		want                 int
	}{
		{"hide", store.LearningActionHide, store.LearningSourceTaught, 0},
		{"alias", store.LearningActionAlias, store.LearningSourceTaught, 0},
		{"ineligible positive source", store.LearningActionBoost, store.LearningSourceManual, 0},
		{"taught boost", store.LearningActionBoost, store.LearningSourceTaught, 1},
		{"followup boost", store.LearningActionBoost, "inferred-followup", 1},
		{"reach boost", store.LearningActionBoost, "inferred-reach", 1},
		{"pair boost", store.LearningActionBoost, "inferred-pair", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := openLearnings(t)
			for _, r := range []struct{ country, code string }{{"Germany", "DE"}, {"France", "FR"}} {
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country',?,?,'seeded')`, r.country, r.code); err != nil {
					t.Fatal(err)
				}
				if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch " + r.country, QueryEntities: []string{r.country}, ResourceID: "pitch-" + r.code, ResourceType: "source", Action: tc.action, AliasTarget: "replacement-" + r.code, Source: tc.source}); err != nil {
					t.Fatal(err)
				}
			}
			if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != tc.want {
				t.Fatalf("effect/source extraction n=%d want=%d err=%v", n, tc.want, err)
			}
		})
	}
}

func TestForgetPositiveSupportIgnoresNewestNegativeProvenance(t *testing.T) {
	for _, action := range []string{store.LearningActionHide, store.LearningActionAlias} {
		t.Run(action, func(t *testing.T) {
			s := openLearnings(t)
			id := seedSurvivingPattern(t, s, false)
			if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "France pitch", QueryEntities: []string{"France"}, ResourceID: "pitch-FR", ResourceType: "source", Action: action, AliasTarget: "replacement-FR", Source: store.LearningSourceTaught}); err != nil {
				t.Fatal(err)
			}
			forgetJapan(t, s)
			if !generalizedItaly(t, s) {
				t.Fatal("two positive supporters were lost")
			}
			rows, err := patterns.List(s.DB(), patterns.ListFilter{})
			if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].ExampleQuery == "france pitch" {
				t.Fatalf("negative row replaced positive provenance: %#v %v", rows, err)
			}
		})
	}
}

func TestForgettingNegativeRowReconcilesLegacyInference(t *testing.T) {
	s := openLearnings(t)
	seedSurvivingPattern(t, s, false)
	for _, r := range []struct{ country, code string }{{"Germany", "DE"}, {"France", "FR"}} {
		if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch " + r.country, QueryEntities: []string{r.country}, ResourceID: "pitch-" + r.code, ResourceType: "source", Action: store.LearningActionHide, Source: store.LearningSourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	// Model the invalid inference left by the previous action-agnostic extractor.
	if _, err := s.DB().Exec(`DELETE FROM search_learnings WHERE action='boost'`); err != nil {
		t.Fatal(err)
	}
	if !generalizedItaly(t, s) {
		t.Fatal("legacy inference fixture missing")
	}
	if n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch France", Action: store.LearningActionHide, All: true}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if generalizedItaly(t, s) {
		t.Fatal("negative-only family capture did not invalidate legacy inference")
	}
}
