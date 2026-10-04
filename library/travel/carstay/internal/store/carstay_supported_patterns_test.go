// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/store"
)

func seedSurvivingPattern(t *testing.T, s *store.Store, prefix bool) int64 {
	t.Helper()
	for i, row := range []struct{ country, code string }{{"Germany", "DE"}, {"France", "FR"}, {"Japan", "JP"}, {"Italy", "IT"}} {
		resource := "stop-" + row.code
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
			if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "stop " + row.country, QueryEntities: []string{row.country}, ResourceID: resource, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
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
	if listed[0].ExampleResource != map[bool]string{false: "stop-JP", true: "stop-JP-session-jp"}[prefix] {
		t.Fatalf("deleted example fixture missing %#v", listed[0])
	}
	return listed[0].ID
}

func generalizedItaly(t *testing.T, s *store.Store) bool {
	t.Helper()
	r, err := learn.Recall(context.Background(), s.DB(), "stop Italy", learn.Opts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range r.Results {
		if hit.Source == "pattern" && strings.HasPrefix(hit.ResourceID, "stop-IT") {
			return true
		}
	}
	return false
}

func forgetJapan(t *testing.T, s *store.Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := s.ForgetLearnings(ctx, store.ForgetLearningsFilter{Query: "stop Japan", All: true})
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
			n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "stop France", All: true})
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
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET resource_id='unrelated-FR' WHERE query_pattern='stop france'`)
			case "missing-lookup":
				_, mutationErr = s.DB().Exec(`DELETE FROM entity_lookups WHERE canonical='France'`)
			case "duplicate-entity":
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET query_pattern='stop germany',query_entities='["Germany"]',resource_id='stop-DE',action='hide' WHERE query_pattern='stop france'`)
			case "multi-entity":
				_, mutationErr = s.DB().Exec(`UPDATE search_learnings SET query_pattern='stop france germany',query_entities='["France","Germany"]' WHERE query_pattern='stop france'`)
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
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "stop Japan", All: true}); err == nil {
		t.Fatal("failed supported-rule update reported success")
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE query_pattern='stop japan'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("teaching rollback count=%d err=%v", count, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].ExampleResource != "stop-JP" {
		t.Fatalf("pattern rollback lost provenance %#v %v", rows, err)
	}
}

func TestForgetRetainsCompatibleCohortDespiteOutliers(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, variant := range []string{"conflicting-resource", "missing-lookup", "multi-entity"} {
			t.Run(map[bool]string{false: "exact/", true: "prefix/"}[prefix]+variant, func(t *testing.T) {
				s := openLearnings(t)
				id := seedSurvivingPattern(t, s, prefix)
				spain := "stop-ES"
				if prefix {
					spain += "-session-es"
				}
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country','Spain','ES','seeded')`); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,'{}')`, spain); err != nil {
					t.Fatal(err)
				}
				query, entities, resource := "stop italy", `["Italy"]`, "stale-IT"
				if variant == "missing-lookup" {
					resource = "stop-IT"
					if prefix {
						resource += "-session-it"
					}
					if _, err := s.DB().Exec(`DELETE FROM entity_lookups WHERE canonical='Italy'`); err != nil {
						t.Fatal(err)
					}
				}
				if variant == "multi-entity" {
					query, entities = "stop italy austria", `["Italy","Austria"]`
				}
				if _, err := s.DB().Exec(`INSERT INTO search_learnings(query_pattern,query_entities,resource_id,resource_type,action,source,confidence) VALUES(?,?,?,'source','boost','taught',2)`, query, entities, resource); err != nil {
					t.Fatal(err)
				}
				s.DB().SetMaxOpenConns(1)
				forgetJapan(t, s)
				s.DB().SetMaxOpenConns(4)
				rows, err := patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 1 || rows[0].ID != id {
					t.Fatalf("two compatible supporters lost rule: %#v %v", rows, err)
				}
				if strings.Contains(strings.ToLower(rows[0].ExampleQuery), "italy") || strings.Contains(rows[0].ExampleResource, "stale") {
					t.Fatalf("example selected a non-supporter: %#v", rows[0])
				}
				result, err := learn.Recall(context.Background(), s.DB(), "stop Spain", learn.Opts{})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, hit := range result.Results {
					if hit.Source == "pattern" && hit.ResourceID == spain {
						found = true
					}
				}
				if !found {
					t.Fatalf("compatible cohort failed actual generalized recall: %+v", result)
				}
				n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "stop France", All: true})
				if err != nil || n != 1 {
					t.Fatalf("forget final compatible supporter: %d %v", n, err)
				}
				rows, err = patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 0 {
					t.Fatalf("outlier counted as a second compatible binding: %#v %v", rows, err)
				}
			})
		}
	}
}
