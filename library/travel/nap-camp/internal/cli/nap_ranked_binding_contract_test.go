// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
	"path/filepath"
	"slices"
	"testing"
)

func TestActualRecallTriesSamePatternAfterCachedIdentityRejection(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "same-pattern.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := initLearn(context.Background(), s.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','pitch-ALPHA','{"name":"Gamma"}'),('source','pitch-BETA','{"name":"Beta"}')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runRootArgs(t, "recall", "Alpha Beta pitch", "--db", path, "--agent", "--debug-mismatches", "--limit", "1")
	if err != nil {
		t.Fatalf("recall: %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, stdout, &result)
	if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "pitch-BETA" || result.Results[0].EntityMatch != learn.EntityMatchExact {
		t.Fatalf("same pattern's valid later binding lost: %#v", result)
	}
	if len(result.Mismatches) != 1 || result.Mismatches[0].ResourceID != "pitch-ALPHA" || !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Gamma") {
		t.Fatalf("distinct rejected-only diagnostics lost: %#v", result)
	}
}

func TestAcceptedTargetFilterPrecedesMismatchLimit(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "diagnostic-limit.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := initLearn(context.Background(), s.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('primary','Alpha','A','taught'),('secondary','Beta','A','taught')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','pitch-A','{"name":"Beta"}'),('source','other-ALPHA','{"name":"Gamma"}')`); err != nil {
		t.Fatal(err)
	}
	for _, p := range []patterns.Pattern{
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:primary}", ResourceType: "source", EntityKind: "primary", Confidence: 9, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught},
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:secondary}", ResourceType: "source", EntityKind: "secondary", Confidence: 8, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught},
		{QueryTemplate: "pitch {entity}", ResourceTemplate: "other-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Confidence: 7, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught},
	} {
		if _, _, err := patterns.Upsert(s.DB(), p); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runRootArgs(t, "recall", "Alpha Beta pitch", "--db", path, "--agent", "--debug-mismatches", "--limit", "1")
	if err != nil {
		t.Fatalf("recall: %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, stdout, &result)
	if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "pitch-A" {
		t.Fatalf("valid binding lost: %#v", result)
	}
	if len(result.Mismatches) != 1 || result.Mismatches[0].ResourceID != "other-ALPHA" || !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Gamma") || slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") {
		t.Fatalf("accepted ID crowded out rejected-only diagnostics: %#v", result)
	}
}

func TestBestAcceptedTypedHitPromotesPartialAndRetainsExactDirect(t *testing.T) {
	for _, tc := range []struct{ name, payload, wantSource string }{{"partial-improved", `{}`, learn.SourcePattern}, {"exact-direct", `{"name":"Alpha"}`, "taught"}} {
		t.Run(tc.name, func(t *testing.T) {
			home := withTempLearnHome(t)
			path := filepath.Join(home, "typed-priority.db")
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := initLearn(context.Background(), s.DB()); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','pitch-ALPHA',?)`, tc.payload); err != nil {
				t.Fatal(err)
			}
			if _, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Confidence: 9, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught}); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha pitch", "--resource", "pitch-ALPHA", "--resource-type", "source", "--db", path)
			if err != nil {
				t.Fatalf("teach: %v %s", err, stderr)
			}
			stdout, stderr, err := runRootArgs(t, "recall", "Alpha pitch", "--db", path, "--agent", "--debug-mismatches", "--limit", "1")
			if err != nil {
				t.Fatalf("recall: %v %s", err, stderr)
			}
			var result learn.Result
			unmarshalAgentResults(t, stdout, &result)
			if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "pitch-ALPHA" || result.Results[0].EntityMatch != learn.EntityMatchExact || result.Results[0].Source != tc.wantSource {
				t.Fatalf("typed hit priority changed: %#v", result)
			}
		})
	}
}

func TestRecallLimitPreservesLaterConfidenceBeforeScore(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "ranked-score.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := initLearn(context.Background(), s.DB()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("pitch%d-ALPHA", i)
		if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?, '{"name":"Alpha"}')`, id); err != nil {
			t.Fatal(err)
		}
		query, confidence := "pitch check service {entity}", 2
		if i == 11 {
			query, confidence = "pitch check {entity}", 9
		}
		if _, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: query,
			ResourceTemplate: fmt.Sprintf("pitch%d-{entity:uppercase}", i), ResourceType: "source", EntityKind: "uppercase",
			Confidence: confidence, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runRootArgs(t, "recall", "Alpha pitch check service", "--db", path, "--agent", "--limit", "1")
	if err != nil {
		t.Fatalf("recall: %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, stdout, &result)
	if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "pitch11-ALPHA" || result.Results[0].Confidence != 9 || result.Results[0].MatchScore != 2.0/3.0 {
		t.Fatalf("later higher-confidence result lost to score or pre-validation cap: %#v", result)
	}
}

func TestRecallKeepsDistinctTypeIDTuplesContainingDelimiter(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "typed-tuples.db")
	mappings := []struct{ kind, id string }{{"a|b", "c"}, {"a", "b|c"}}
	for _, row := range mappings {
		_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha pitch", "--resource", row.id, "--resource-type", row.kind, "--db", path)
		if err != nil {
			t.Fatalf("teach %q/%q: %v %s", row.kind, row.id, err, stderr)
		}
	}
	stdout, stderr, err := runRootArgs(t, "recall", "Alpha pitch", "--db", path, "--agent", "--limit", "2")
	if err != nil {
		t.Fatalf("recall: %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, stdout, &result)
	pairs := make(map[struct{ kind, id string }]bool)
	for _, hit := range result.Results {
		pairs[struct{ kind, id string }{hit.ResourceType, hit.ResourceID}] = true
	}
	if !result.Found || len(result.Results) != 2 || !pairs[mappings[0]] || !pairs[mappings[1]] {
		t.Fatalf("distinct typed tuples collapsed: %#v", result)
	}
}
