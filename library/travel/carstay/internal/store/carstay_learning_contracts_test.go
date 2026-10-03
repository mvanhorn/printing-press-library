// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/store"
)

func seedDerivedLearningFamily(t *testing.T) (*store.Store, int64) {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "learning.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for _, entity := range []string{"alpha", "beta"} {
		if _, _, err = s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: entity + " widgets", QueryEntities: []string{entity}, ResourceID: "spot/" + entity, ResourceType: "directory", Source: store.LearningSourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := patterns.Extract(s.DB(), nil); err != nil || n != 1 {
		t.Fatalf("extract n=%d err=%v", n, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{Source: patterns.SourceInferred})
	if err != nil || len(rows) != 1 {
		t.Fatalf("derived rows=%+v err=%v", rows, err)
	}
	return s, rows[0].ID
}

func TestForgetInvalidatesDerivedFamilyAndPreservesManualAndUnrelated(t *testing.T) {
	s, derivedID := seedDerivedLearningFamily(t)
	kept := []int64{}
	for _, p := range []patterns.Pattern{
		{QueryTemplate: "widgets {entity}", ResourceTemplate: "manual/{entity:lowercase}", ResourceType: "directory", Source: patterns.SourceTaught},
		{QueryTemplate: "onsen {entity}", ResourceTemplate: "onsen/{entity:lowercase}", ResourceType: "directory", Source: patterns.SourceInferred},
		{QueryTemplate: "widgets {entity}", ResourceTemplate: "camp/{entity:lowercase}", ResourceType: "camps", Source: patterns.SourceInferred},
		{QueryTemplate: "widgets {entity}", ResourceTemplate: "other/{entity:lowercase}", ResourceType: "directory", Venue: "elsewhere", Source: patterns.SourceInferred},
	} {
		p.Strategy = patterns.StrategySubstitute
		p.EntityKind = "lowercase"
		id, _, err := patterns.Upsert(s.DB(), p)
		if err != nil {
			t.Fatal(err)
		}
		kept = append(kept, id)
	}
	n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "alpha widgets", ResourceID: "spot/alpha"})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[int64]bool{}
	for _, p := range rows {
		ids[p.ID] = true
	}
	if ids[derivedID] {
		t.Fatal("forgotten teaching still has a derived recall pattern")
	}
	for _, id := range kept {
		if !ids[id] {
			t.Fatalf("manual/unrelated pattern%d deleted", id)
		}
	}
	hits, err := patterns.Apply(context.Background(), s.DB(), "gamma widgets", "widgets", []string{"gamma"}, patterns.Opts{NoVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range hits {
		if hit.ResourceID == "spot/gamma" {
			t.Fatal("derived behavior survived forget")
		}
	}
	learns, err := s.ListLearnings(context.Background(), store.ListLearningsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(learns) != 1 || learns[0].ResourceID != "spot/beta" {
		t.Fatalf("unrelated teaching changed: %+v", learns)
	}
}

func TestForgetDependencyFailureRollsBackTeachingDeletion(t *testing.T) {
	s, derivedID := seedDerivedLearningFamily(t)
	if _, err := s.DB().Exec(`CREATE TRIGGER reject_pattern_delete BEFORE DELETE ON search_patterns BEGIN SELECT RAISE(ABORT, 'test dependency failure'); END`); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "alpha widgets", All: true}); err == nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	learns, err := s.ListLearnings(context.Background(), store.ListLearningsFilter{})
	if err != nil || len(learns) != 2 {
		t.Fatalf("non-atomic teaching deletion: %+v %v", learns, err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != derivedID {
		t.Fatalf("dependency changed on failure: %+v %v", rows, err)
	}
}
