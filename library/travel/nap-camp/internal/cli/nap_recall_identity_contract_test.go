// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/store"
)

func TestActualRecallCachedIdentityGatesBothPaths(t *testing.T) {
	cases := []struct {
		name, query, payload, wantMatch     string
		direct, pattern, missing, wantFound bool
	}{
		{"direct_literal_conflict", "Nagatoro pitch", `{"name":"Bravo"}`, "", true, false, false, false},
		{"direct_canonical_alias_conflict", "NGT pitch", `{"name":"Bravo"}`, "", true, false, false, false},
		{"pattern_conflict", "NGT pitch", `{"name":"Bravo"}`, "", false, true, false, false},
		{"direct_and_pattern_conflict", "NGT pitch", `{"name":"Bravo"}`, "", true, true, false, false},
		{"direct_cached_alias", "NGT pitch", `{"name":"Nagatoro"}`, learn.EntityMatchExact, true, false, false, true},
		{"pattern_cached_alias", "NGT pitch", `{"name":"Nagatoro"}`, learn.EntityMatchExact, false, true, false, true},
		{"direct_and_pattern_cached_alias", "NGT pitch", `{"name":"Nagatoro"}`, learn.EntityMatchExact, true, true, false, true},
		{"warned_missing_resource", "NGT pitch", "", learn.EntityMatchExact, true, false, true, true},
		{"direct_unknown_identity", "NGT pitch", `{}`, learn.EntityMatchPartial, true, false, false, true},
		{"pattern_unknown_identity", "NGT pitch", `{}`, learn.EntityMatchExact, false, true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := withTempLearnHome(t)
			path := filepath.Join(home, "recall-identity.db")
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := initLearn(context.Background(), s.DB()); err != nil {
				t.Fatal(err)
			}
			// Nagatoro is a seeded public spelling; NGT is a synthetic alias.
			if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('campsite','長瀞オートキャンプ場','NGT','taught')`); err != nil {
				t.Fatal(err)
			}
			id := "pitch-NGT"
			if !tc.missing {
				if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,?)`, id, tc.payload); err != nil {
					t.Fatal(err)
				}
			}
			if tc.pattern {
				_, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "pitch {entity}", ResourceTemplate: "pitch-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if tc.direct {
				_, stderr, err := runRootArgs(t, "teach", "--query", "Nagatoro pitch", "--resource", id, "--resource-type", "source", "--db", path)
				if err != nil {
					t.Fatalf("teach: %v %s", err, stderr)
				}
			}
			stdout, stderr, err := runRootArgs(t, "recall", tc.query, "--db", path, "--agent", "--debug-mismatches")
			if err != nil {
				t.Fatalf("recall: %v %s", err, stderr)
			}
			var result learn.Result
			unmarshalAgentResults(t, stdout, &result)
			if result.Found != tc.wantFound {
				t.Fatalf("found=%v want=%v: %#v", result.Found, tc.wantFound, result)
			}
			if !tc.wantFound {
				if len(result.Results) != 0 || len(result.Mismatches) == 0 {
					t.Fatalf("conflict lost mismatch evidence: %#v", result)
				}
				if !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Bravo") {
					t.Fatalf("ordinary warning hides actual cached alternative: %#v", result.Warnings)
				}
				for _, mismatch := range result.Mismatches {
					if mismatch.EntityMatch != learn.EntityMatchMismatch || !slices.Contains(mismatch.ResourceEntities, "Bravo") {
						t.Fatalf("conflict hidden: %#v", mismatch)
					}
				}
				return
			}
			if len(result.Results) != 1 || result.Results[0].EntityMatch != tc.wantMatch {
				t.Fatalf("classification changed: %#v", result.Results)
			}
			if tc.missing && !slices.Contains(result.Results[0].Warnings, learn.WarningResourceNotInStore) {
				t.Fatalf("fallback lacks warning: %#v", result.Results[0])
			}
			if tc.payload == `{}` && len(result.Results[0].ResourceEntities) != 0 {
				t.Fatalf("unknown identity invented: %#v", result.Results[0])
			}
		})
	}
}

func TestActualRecallAmbiguousAliasNeedsSharedResourceCanonical(t *testing.T) {
	for _, name := range []string{"Alpha", "Beta"} {
		t.Run(name, func(t *testing.T) {
			home := withTempLearnHome(t)
			path := filepath.Join(home, "ambiguous-alias.db")
			s, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := initLearn(context.Background(), s.DB()); err != nil {
				t.Fatal(err)
			}
			for _, canonical := range []string{"Alpha", "Beta"} {
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('campsite',?,'Z1','taught')`, canonical); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','pitch-Z1',?)`, `{"name":"`+name+`"}`); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha pitch", "--resource", "pitch-Z1", "--resource-type", "source", "--db", path)
			if err != nil {
				t.Fatalf("teach: %v %s", err, stderr)
			}
			stdout, stderr, err := runRootArgs(t, "recall", "Z1 pitch", "--db", path, "--agent", "--debug-mismatches")
			if err != nil {
				t.Fatalf("recall: %v %s", err, stderr)
			}
			var result learn.Result
			unmarshalAgentResults(t, stdout, &result)
			wantFound := name == "Alpha"
			if result.Found != wantFound {
				t.Fatalf("ambiguous alias promoted different canonical: %#v", result)
			}
			if !slices.Contains(result.Warnings, learn.WarningAmbiguousAlias) {
				t.Fatalf("ambiguity warning absent: %#v", result.Warnings)
			}
			if wantFound && (len(result.Results) != 1 || result.Results[0].EntityMatch != learn.EntityMatchExact) {
				t.Fatalf("genuine common alias lost: %#v", result)
			}
			if !wantFound && !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") {
				t.Fatalf("cached alternative missing: %#v", result.Warnings)
			}
		})
	}
}
