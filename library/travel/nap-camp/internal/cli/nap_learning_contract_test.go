// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
)

func TestManualTeachPatternReportedScopeMatchesVerifiedApplication(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "manual.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	original := patterns.Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity}", ResourceType: "old", Venue: "old", EntityKind: "lowercase", Strategy: patterns.StrategySubstitute, Source: patterns.SourceInferred, ExampleQuery: "old", ExampleResource: "old"}
	id, _, err := patterns.Upsert(s.DB(), original)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','pitch-JAPAN','{}')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	stdout, stderr, err := runRootArgs(t, "teach-pattern", "--query-template", "pitch {entity}", "--resource-template", "pitch-{entity}", "--resource-type", "source", "--venue", "kanto", "--entity-kind", "uppercase", "--db", path, "--agent")
	if err != nil {
		t.Fatalf("actual teach-pattern %v %s", err, stderr)
	}
	var reported map[string]any
	unmarshalAgentResults(t, stdout, &reported)
	if reported["recorded"] != true || reported["resource_type"] != "source" || reported["venue"] != "kanto" || reported["entity_kind"] != "uppercase" {
		t.Fatalf("unexpected CLI response %#v", reported)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// A subsequent inferred write with stale data must preserve explicit scope.
	if _, _, err := patterns.Upsert(s.DB(), original); err != nil {
		t.Fatal(err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Source != patterns.SourceTaught || rows[0].ResourceType != "source" || rows[0].Venue != "kanto" || rows[0].EntityKind != "uppercase" || rows[0].ExampleQuery != "" || rows[0].ExampleResource != "" {
		t.Fatalf("stored manual declaration mismatch %#v %v", rows, err)
	}
	hits, err := patterns.Apply(context.Background(), s.DB(), "pitch Japan", "pitch", []string{"Japan"}, patterns.Opts{})
	if err != nil || len(hits) != 1 || hits[0].ResourceID != "pitch-JAPAN" || hits[0].ResourceType != "source" || hits[0].Venue != "kanto" {
		t.Fatalf("reported declaration did not govern verified recall %#v %v", hits, err)
	}
}
