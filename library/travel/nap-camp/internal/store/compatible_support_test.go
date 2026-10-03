// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
)

func TestUndoKeepsCompatibleDistinctSupportDespiteUnusableRetainedRows(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, variant := range []string{"stale-lookup", "conflicting-resource", "multiple-entities"} {
			t.Run(map[bool]string{false: "exact", true: "prefix"}[prefix]+"/"+variant, func(t *testing.T) {
				s := openLearnings(t)
				id := seedSurvivingPattern(t, s, prefix)
				resource := "pitch-IS"
				if prefix {
					resource += "-session-is"
				}
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country','Iceland','IS','taught')`); err != nil {
					t.Fatal(err)
				}
				query, entities := "pitch Iceland", []string{"Iceland"}
				if variant == "conflicting-resource" {
					resource = "unrelated-IS"
				}
				if variant == "multiple-entities" {
					query, entities = "pitch Iceland Germany", []string{"Iceland", "Germany"}
				}
				if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: query, QueryEntities: entities, ResourceID: resource, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
					t.Fatal(err)
				}
				if variant == "stale-lookup" {
					if _, err := s.DB().Exec(`DELETE FROM entity_lookups WHERE canonical='Iceland'`); err != nil {
						t.Fatal(err)
					}
				}
				if !generalizedItaly(t, s) {
					t.Fatal("actual extracted baseline rule cannot generalize")
				}
				// Two valid retained bindings (Germany, France) must survive the
				// newest invalid family member. This also forces tx-owned reads.
				s.DB().SetMaxOpenConns(1)
				forgetJapan(t, s)
				s.DB().SetMaxOpenConns(4)
				if !generalizedItaly(t, s) {
					t.Fatal("unusable retained row poisoned two valid supports")
				}
				rows, err := patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Source != patterns.SourceInferred {
					t.Fatalf("supported rule identity/provenance lost: %#v %v", rows, err)
				}
				if strings.Contains(strings.ToLower(rows[0].ExampleQuery), "iceland") || strings.Contains(rows[0].ExampleResource, "IS") || strings.Contains(strings.ToLower(rows[0].ExampleQuery), "japan") {
					t.Fatalf("example came from unusable/deleted member: %#v", rows[0])
				}
				// The invalid row cannot supply the second support after France is
				// removed. Germany plus unusable evidence must invalidate the rule.
				if n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch France", All: true}); err != nil || n != 1 {
					t.Fatal(n, err)
				}
				if generalizedItaly(t, s) {
					t.Fatal("one compatible support plus unusable evidence retained inference")
				}
			})
		}
	}
}

func TestUndoCompatibleSupportLookupFailureRollsBack(t *testing.T) {
	s := openLearnings(t)
	id := seedSurvivingPattern(t, s, false)
	// An infrastructure failure is distinct from a missing/stale lookup row.
	if _, err := s.DB().Exec(`DROP TABLE entity_lookups`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", All: true}); err == nil {
		t.Fatal("lookup query failure reported a successful undo")
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE query_pattern='pitch japan'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("teaching changed on lookup failure: %d %v", count, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].ExampleResource != "pitch-JP" {
		t.Fatalf("rule/provenance changed on lookup failure: %#v %v", rows, err)
	}
}
