// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package learn

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn/entities"
)

func TestCarstayAliasRecallRespectsCachedResourceIdentity(t *testing.T) {
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

func TestCarstayPatternRecallValidatesOnlyBoundEntity(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		for _, tc := range []struct {
			name, payload, query string
			wantFound, wantCross bool
		}{
			{"single conflicting target", `{"name":"Beta"}`, "Alpha widget today", false, false},
			{"unrelated query alias cannot validate target", `{"name":"Beta"}`, "Alpha BetaAlias widget today", false, false},
			{"literal bound target", `{"name":"Alpha"}`, "Alpha BetaAlias widget today", true, false},
			{"real bound alias", `{"name":"A1"}`, "Alpha BetaAlias widget today", true, true},
			{"unknown cached identity", `{}`, "Alpha BetaAlias widget today", true, false},
		} {
			t.Run(map[bool]string{false: "exact/", true: "prefix/"}[prefix]+tc.name, func(t *testing.T) {
				db := openRecallCanonicalTestDB(t)
				seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
				seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "BetaAlias"})
				seedCanonicalLookup(t, db, "target", "Alpha", []string{"abc"})
				seedCanonicalLookup(t, db, "target", "A1", []string{"abc"})
				id, template, strategy := "000000000000000000000abc", "000000000000000000000{entity:target}", "substitute"
				if prefix {
					template, strategy = template+"*", "substitute-then-search-prefix"
				}
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, id, tc.payload); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,venue,strategy,entity_kind,confidence,source) VALUES('widget {entity}',?,'widgets','north',?,'target',2,'taught')`, template, strategy); err != nil {
					t.Fatal(err)
				}
				cfg := entities.NewConfig()
				cfg.RegisterStopwords("today", "versus", "vs")
				got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: cfg, ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
				if err != nil {
					t.Fatal(err)
				}
				if got.Found != tc.wantFound {
					t.Fatalf("found=%v want%v: %+v", got.Found, tc.wantFound, got)
				}
				var hit Hit
				if tc.wantFound {
					if len(got.Results) != 1 || len(got.Mismatches) != 0 {
						t.Fatalf("unexpected positive envelope: %+v", got)
					}
					hit = got.Results[0]
				} else {
					if len(got.Results) != 0 || len(got.Mismatches) != 1 {
						t.Fatalf("known conflicting pattern target admitted: %+v", got)
					}
					hit = got.Mismatches[0]
				}
				if hit.Source != SourcePattern || hit.Venue != "north" || hit.ResourceID != id || hit.ResourceType != "widgets" {
					t.Fatalf("lost pattern scope: %+v", hit)
				}
				if strings.Contains(strings.Join(hit.Warnings, " "), WarningCrossAliasMatch) != tc.wantCross {
					t.Fatalf("alias warning mismatch: %+v", hit)
				}
				if !tc.wantFound && hit.EntityMatch != EntityMatchMismatch {
					t.Fatalf("conflict confidence: %+v", hit)
				}
			})
		}
	}
}

func TestCarstayRecallLimitAppliesAfterPatternIdentityValidation(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
	seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "BetaAlias"})
	seedCanonicalLookup(t, db, "wrong_target", "Alpha", []string{"abc"})
	seedCanonicalLookup(t, db, "right_target", "BetaAlias", []string{"abc"})
	const id = "000000000000000000000abc"
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{"name":"Beta"}')`, id); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		kind       string
		confidence int
	}{{"wrong_target", 99}, {"right_target", 2}} {
		if _, err := db.Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,strategy,entity_kind,confidence,source) VALUES('widget {entity}',?,'widgets','substitute',?,?,'taught')`, "000000000000000000000{entity:"+p.kind+"}", p.kind, p.confidence); err != nil {
			t.Fatal(err)
		}
	}
	cfg := entities.NewConfig()
	cfg.RegisterStopwords("today", "versus", "vs")
	got, err := Recall(context.Background(), db, "Alpha BetaAlias widget today", Opts{EntityConfig: cfg, ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != id || got.Results[0].Confidence != 2 || got.Results[0].EntityMatch != EntityMatchExact || len(got.Mismatches) != 0 {
		t.Fatalf("candidate cap hid validated lower-ranked binding: %+v", got)
	}
}
