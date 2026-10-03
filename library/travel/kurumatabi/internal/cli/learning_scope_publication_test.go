package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/store"
	"path/filepath"
	"testing"
)

func TestTeachPatternRecordedScopeMatchesStoredRecall(t *testing.T) {
	home := withTempLearnHome(t)
	dbPath := filepath.Join(home, "manual-scope.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	inferred := patterns.Pattern{QueryTemplate: "station {entity}", ResourceTemplate: "000000000000000000000{entity:lowercase}", ResourceType: "activities", Venue: "old-venue", Strategy: patterns.StrategySubstitute, EntityKind: "uppercase", Source: patterns.SourceInferred}
	id, _, err := patterns.Upsert(s.DB(), inferred)
	if err != nil {
		t.Fatal(err)
	}
	const resourceID = "000000000000000000000abc"
	if err := s.Upsert("parks", resourceID, json.RawMessage(`{"id":"000000000000000000000abc","name":"Synthetic park"}`)); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "teach-pattern", "--query-template", inferred.QueryTemplate, "--resource-template", inferred.ResourceTemplate, "--resource-type", "parks", "--venue", "new-venue", "--entity-kind", "lowercase", "--db", dbPath, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var reported map[string]any
	unmarshalAgentResults(t, stdout, &reported)
	if reported["recorded"] != true || reported["resource_type"] != "parks" || reported["venue"] != "new-venue" || reported["entity_kind"] != "lowercase" {
		t.Fatalf("reported=%+v", reported)
	}
	if _, _, err := patterns.Upsert(s.DB(), inferred); err != nil {
		t.Fatal(err)
	}
	rows, err := patterns.List(s.DB(), patterns.ListFilter{})
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Source != patterns.SourceTaught || rows[0].ResourceType != reported["resource_type"] || rows[0].Venue != reported["venue"] || rows[0].EntityKind != reported["entity_kind"] {
		t.Fatalf("recorded scope differs from store: %+v %v", rows, err)
	}
	hits, err := patterns.Apply(context.Background(), s.DB(), "Abc station", "station", []string{"Abc"}, patterns.Opts{})
	if err != nil || len(hits) != 1 || hits[0].ResourceID != resourceID || hits[0].ResourceType != "parks" || hits[0].Venue != "new-venue" {
		t.Fatalf("recorded scope differs from recall: %+v %v", hits, err)
	}
}
