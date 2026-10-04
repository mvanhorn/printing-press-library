// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
)

func negativeSupportTeaching(t *testing.T, s *store.Store, country, resource, action, source string) int64 {
	t.Helper()
	in := store.UpsertLearningInput{Query: "pitch " + country, QueryEntities: []string{country}, ResourceID: resource, ResourceType: "source", Action: action, Source: source}
	if action == store.LearningActionAlias {
		in.AliasTarget = "pitch-DE"
	}
	id, _, err := s.UpsertLearning(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestForgetOnlyPositiveBindingsSupportPattern(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, action := range []string{store.LearningActionHide, store.LearningActionAlias} {
			t.Run(map[bool]string{false: "exact", true: "prefix"}[prefix]+"/"+action, func(t *testing.T) {
				s := openLearnings(t)
				id := seedSurvivingPattern(t, s, prefix)
				resource := "pitch-FR"
				if prefix {
					resource += "-session-fr"
				}
				negativeID := negativeSupportTeaching(t, s, "France", resource, action, store.LearningSourceTaught)
				// Normal API rows coexist by action; remove the positive contributor only.
				n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch France", Action: store.LearningActionBoost, All: true})
				if err != nil || n != 1 {
					t.Fatal(n, err)
				}
				rows, err := patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 1 || rows[0].ID != id || !generalizedItaly(t, s) {
					t.Fatalf("two positive supporters lost: %#v %v", rows, err)
				}
				s.DB().SetMaxOpenConns(1)
				forgetJapan(t, s)
				s.DB().SetMaxOpenConns(4)
				if generalizedItaly(t, s) {
					t.Fatal("one boost plus a non-positive teaching retained generalization")
				}
				rows, err = patterns.List(s.DB(), patterns.ListFilter{})
				if err != nil || len(rows) != 0 {
					t.Fatalf("non-positive support survived: %#v %v", rows, err)
				}
				if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != 0 {
					t.Fatalf("extract recreated unsupported inference: n=%d err=%v", n, err)
				}
				if generalizedItaly(t, s) {
					t.Fatal("extraction resurrected unsupported pattern")
				}
				var kept int
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE id=? AND action=?`, negativeID, action).Scan(&kept); err != nil || kept != 1 {
					t.Fatalf("unrelated negative rule changed: %d %v", kept, err)
				}
			})
		}
	}
}

func TestExtractIgnoresNonPositiveBindings(t *testing.T) {
	for _, action := range []string{store.LearningActionHide, store.LearningActionAlias} {
		t.Run(action, func(t *testing.T) {
			s := openLearnings(t)
			for _, country := range []string{"Germany", "France", "Italy"} {
				code := map[string]string{"Germany": "DE", "France": "FR", "Italy": "IT"}[country]
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country',?,?,'seeded')`, country, code); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,'{}')`, "pitch-"+code); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch Germany", QueryEntities: []string{"Germany"}, ResourceID: "pitch-DE", ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
				t.Fatal(err)
			}
			negativeSupportTeaching(t, s, "France", "pitch-FR", action, store.LearningSourceTaught)
			if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != 0 {
				t.Fatalf("negative teaching inferred a positive rule: n=%d err=%v", n, err)
			}
			if generalizedItaly(t, s) {
				t.Fatal("non-positive teaching generalized")
			}
		})
	}
}

func TestForgetPreservesPositiveCohortAlongsideNonPositiveRows(t *testing.T) {
	for _, action := range []string{store.LearningActionHide, store.LearningActionAlias} {
		t.Run(action, func(t *testing.T) {
			s := openLearnings(t)
			id := seedSurvivingPattern(t, s, false)
			if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country','Spain','ES','seeded')`); err != nil {
				t.Fatal(err)
			}
			negativeSupportTeaching(t, s, "Spain", "pitch-ES", action, "inferred-followup")
			s.DB().SetMaxOpenConns(1)
			forgetJapan(t, s)
			s.DB().SetMaxOpenConns(4)
			rows, err := patterns.List(s.DB(), patterns.ListFilter{})
			if err != nil || len(rows) != 1 || rows[0].ID != id || !generalizedItaly(t, s) {
				t.Fatalf("negative row vetoed two positive supporters: %#v %v", rows, err)
			}
			if strings.Contains(strings.ToLower(rows[0].ExampleQuery), "spain") || strings.Contains(rows[0].ExampleResource, "ES") {
				t.Fatalf("negative row became positive provenance: %#v", rows[0])
			}
		})
	}
}
