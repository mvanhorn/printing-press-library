package learn

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/learn/patterns"
)

func TestMichiAliasRecallRespectsCachedResourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, payload, wantMatch, query        string
		wantFound, wantCrossAlias, wantMissing bool
	}{
		{"conflicting cached resource", `{"name":"Beta"}`, EntityMatchMismatch, "A1 widget today", false, false, false},
		{"cached true alias", `{"name":"Alpha"}`, EntityMatchExact, "A1 widget today", true, true, false},
		{"missing cached resource", "", EntityMatchExact, "A1 widget today", true, true, true},
		{"unknown cached identity", `{}`, EntityMatchPartial, "A1 widget today", true, false, false},
		{"ambiguous query cannot switch teaching meaning", `{"name":"Beta"}`, EntityMatchMismatch, "Z1 widget today", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1", "Z1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "Z1"})
			seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, "resource-1", "widgets")
			if tc.payload != "" {
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-1',?)`, tc.payload); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("found=%v want %v; result=%+v", got.Found, tc.wantFound, got)
			}
			var hit Hit
			if tc.wantFound {
				if len(got.Results) != 1 || len(got.Mismatches) != 0 {
					t.Fatalf("unexpected results/mismatches: %+v", got)
				}
				hit = got.Results[0]
			} else {
				if len(got.Results) != 0 || len(got.Mismatches) != 1 {
					t.Fatalf("conflicting cached identity was admitted: %+v", got)
				}
				hit = got.Mismatches[0]
			}
			if hit.EntityMatch != tc.wantMatch {
				t.Fatalf("match=%q want %q; hit=%+v", hit.EntityMatch, tc.wantMatch, hit)
			}
			warnings := strings.Join(hit.Warnings, " ")
			if strings.Contains(warnings, WarningCrossAliasMatch) != tc.wantCrossAlias {
				t.Fatalf("cross alias warning: %+v", hit)
			}
			if strings.Contains(warnings, WarningResourceNotInStore) != tc.wantMissing {
				t.Fatalf("missing resource warning: %+v", hit)
			}
		})
	}
}

func TestMichiPatternRecallRespectsCachedResourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, query, payload, resourceID string
		withTeaching, wantFound          bool
	}{
		{"pattern conflicting cached resource", "find Alpha details", `{"name":"Beta"}`, "resource-alpha", false, false},
		{"true cached pattern alias", "find A1 details", `{"name":"Alpha"}`, "resource-a1", false, true},
		{"unknown identity keeps verified identifier", "find Alpha details", `{}`, "resource-alpha", false, true},
		{"pattern cannot re-admit conflicting teaching", "find Alpha details", `{"name":"Beta"}`, "resource-alpha", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta"})
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, tc.resourceID, tc.payload); err != nil {
				t.Fatal(err)
			}
			_, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught})
			if err != nil {
				t.Fatal(err)
			}
			if tc.withTeaching {
				seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, tc.resourceID, "widgets")
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("found=%v want %v; result=%+v", got.Found, tc.wantFound, got)
			}
			if tc.wantFound {
				if len(got.Results) != 1 || got.Results[0].Source != SourcePattern || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("valid pattern contract regressed: %+v", got)
				}
			} else {
				if len(got.Results) != 0 || len(got.Mismatches) == 0 {
					t.Fatalf("known resource mismatch must be surfaced: %+v", got)
				}
				for _, h := range got.Mismatches {
					if h.EntityMatch != EntityMatchMismatch {
						t.Fatalf("false mismatch evidence: %+v", h)
					}
				}
			}
		})
	}
}
