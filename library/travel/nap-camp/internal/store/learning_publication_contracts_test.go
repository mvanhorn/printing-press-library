// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store_test

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
)

func seedUndoFixture(t *testing.T, s *store.Store) {
	t.Helper()
	for _, row := range []struct{ country, id string }{{"Japan", "pitch-JP"}, {"Germany", "pitch-DE"}} {
		if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "pitch " + row.country, QueryEntities: []string{row.country}, ResourceID: row.id, ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,?)`, row.id, `{"name":"fixture"}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('country','Japan','JP','seeded'),('country','Germany','DE','seeded')`); err != nil {
		t.Fatal(err)
	}
	// Exercise the actual extractor so undo is bound to its template and
	// entity-slot contracts, including a non-example contributing teaching.
	if n, err := patterns.Extract(s.DB(), []string{"country"}); err != nil || n != 1 {
		t.Fatalf("extract fixture: created=%d err=%v", n, err)
	}
}

func TestForgottenTeachingCannotRecallDerivedPattern(t *testing.T) {
	s := openLearnings(t)
	seedUndoFixture(t, s)
	before, err := learn.Recall(context.Background(), s.DB(), "pitch Japan", learn.Opts{})
	if err != nil || len(before.Results) == 0 {
		t.Fatalf("fixture had no initial recall: %v %#v", err, before)
	}
	n, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", ResourceID: "pitch-JP"})
	if err != nil || n != 1 {
		t.Fatalf("forget: n=%d err=%v", n, err)
	}
	after, err := learn.Recall(context.Background(), s.DB(), "pitch Japan", learn.Opts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, hit := range after.Results {
		if hit.ResourceID == "pitch-JP" {
			t.Fatalf("forgotten non-example contributor still recalled: %#v", after)
		}
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_id='pitch-DE'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unrelated teaching removed: count=%d err=%v", count, err)
	}
}

func TestForgetRetainsManualAndUnrelatedPatternFamilies(t *testing.T) {
	s := openLearnings(t)
	seedUndoFixture(t, s)
	variants := []patterns.Pattern{
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "manual-{entity}", ResourceType: "source", Strategy: patterns.StrategySubstitute, EntityKind: "country", Source: patterns.SourceTaught},
		{QueryTemplate: "other {entity}", ResourceTemplate: "other-{entity}", ResourceType: "source", Strategy: patterns.StrategySubstitute, EntityKind: "country", Source: patterns.SourceInferred},
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "venue-{entity}", ResourceType: "source", Venue: "elsewhere", Strategy: patterns.StrategySubstitute, EntityKind: "country", Source: patterns.SourceInferred},
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "resource-{entity}", ResourceType: "different", Strategy: patterns.StrategySubstitute, EntityKind: "country", Source: patterns.SourceInferred},
	}
	for _, p := range variants {
		if _, _, err := patterns.Upsert(s.DB(), p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", ResourceID: "pitch-JP"}); err != nil {
		t.Fatal(err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 4 {
		t.Fatalf("manual/unrelated scope changed: %v %#v", err, rows)
	}
	for _, p := range rows {
		if p.ResourceTemplate == "pitch-{entity:country}" {
			t.Fatal("affected inferred family retained")
		}
	}
}

func TestForgetRollbackPreservesTeachingAndPattern(t *testing.T) {
	s := openLearnings(t)
	seedUndoFixture(t, s)
	if _, err := s.DB().Exec(`CREATE TRIGGER prevent_pattern_delete BEFORE DELETE ON search_patterns BEGIN SELECT RAISE(ABORT,'fixture rollback'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", ResourceID: "pitch-JP"}); err == nil {
		t.Fatal("failed cascade reported success")
	}
	for _, q := range []string{`SELECT COUNT(*) FROM search_learnings WHERE resource_id='pitch-JP'`, `SELECT COUNT(*) FROM search_patterns WHERE resource_template='pitch-{entity:country}'`} {
		var count int
		if err := s.DB().QueryRow(q).Scan(&count); err != nil || count != 1 {
			t.Fatalf("transaction did not roll back: count=%d err=%v", count, err)
		}
	}
}

func TestExplicitPatternPromotionSurvivesUndoAndInference(t *testing.T) {
	s := openLearnings(t)
	seedUndoFixture(t, s)
	p := patterns.Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:country}", ResourceType: "source", Strategy: patterns.StrategySubstitute, EntityKind: "country", Source: patterns.SourceTaught}
	if _, _, err := patterns.Upsert(s.DB(), p); err != nil {
		t.Fatal(err)
	}
	p.Source = patterns.SourceInferred
	if _, _, err := patterns.Upsert(s.DB(), p); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForgetLearnings(context.Background(), store.ForgetLearningsFilter{Query: "pitch Japan", ResourceID: "pitch-JP"}); err != nil {
		t.Fatal(err)
	}
	var source string
	if err := s.DB().QueryRow(`SELECT source FROM search_patterns WHERE resource_template='pitch-{entity:country}'`).Scan(&source); err != nil || source != patterns.SourceTaught {
		t.Fatalf("manual intent lost: source=%q err=%v", source, err)
	}
}
