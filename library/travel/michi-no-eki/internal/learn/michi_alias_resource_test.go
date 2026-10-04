package learn

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

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

func TestMichiPatternUsesOnlyItsActualBoundEntity(t *testing.T) {
	for _, tc := range []struct {
		name, query, payload, id string
		wantFound                bool
	}{
		{"unrelated alias cannot promote", "find Alpha B1 details", `{"name":"Beta"}`, "resource-alpha", false},
		{"unrelated literal cannot match", "find Alpha Beta details", `{"name":"Beta"}`, "resource-alpha", false},
		{"true bound literal still matches", "find Alpha B1 details", `{"name":"Alpha"}`, "resource-alpha", true},
		{"true bound alias still matches", "find A1 Beta details", `{"name":"Alpha"}`, "resource-a1", true},
		{"unknown identity retains verified ID", "find Alpha B1 details", `{}`, "resource-alpha", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "B1"})
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, tc.id, tc.payload); err != nil {
				t.Fatal(err)
			}
			if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
				t.Fatal(err)
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("wrong bound entity result: %+v", got)
			}
			if tc.wantFound {
				if len(got.Results) != 1 || got.Results[0].Source != SourcePattern || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("valid bound entity regressed: %+v", got)
				}
			} else if len(got.Results) != 0 || len(got.Mismatches) != 1 || got.Mismatches[0].EntityMatch != EntityMatchMismatch {
				t.Fatalf("unrelated query entity admitted conflicting bound resource: %+v", got)
			}
		})
	}
}

func TestMichiRejectedPatternDoesNotHideLaterValidBinding(t *testing.T) {
	for _, invalidFirst := range []bool{true, false} {
		for _, resultLimit := range []int{1, 10} {
			t.Run(map[bool]string{true: "invalid-first", false: "valid-first"}[invalidFirst]+map[int]string{1: "/limit-one", 10: "/default"}[resultLimit], func(t *testing.T) {
				db := openRecallCanonicalTestDB(t)
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-target','{"name":"Beta"}')`); err != nil {
					t.Fatal(err)
				}
				seedCanonicalLookup(t, db, "alpha-map", "Alpha", []string{"target"})
				seedCanonicalLookup(t, db, "beta-map", "Beta", []string{"target"})
				patternsToInsert := []patterns.Pattern{
					{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:alpha-map}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "alpha-map", Source: patterns.SourceTaught},
					{QueryTemplate: "details {entity} find", ResourceTemplate: "resource-{entity:beta-map}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "beta-map", Source: patterns.SourceTaught},
				}
				for _, pattern := range patternsToInsert {
					if _, _, err := patterns.Upsert(db, pattern); err != nil {
						t.Fatal(err)
					}
				}
				alphaTime, betaTime := "2026-01-01 00:00:00", "2026-01-02 00:00:00"
				if invalidFirst {
					alphaTime, betaTime = betaTime, alphaTime
				}
				if _, err := db.Exec(`UPDATE search_patterns SET last_observed_at=CASE entity_kind WHEN 'alpha-map' THEN ? ELSE ? END`, alphaTime, betaTime); err != nil {
					t.Fatal(err)
				}
				got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, PatternKinds: []string{"alpha-map", "beta-map"}, DebugMismatches: true, Limit: resultLimit})
				if err != nil {
					t.Fatal(err)
				}
				if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-target" || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("rejected pattern hid later valid binding: %+v", got)
				}
				if len(got.Mismatches) != 0 || strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity) {
					t.Fatalf("accepted target retained contradictory mismatch diagnostics: %+v", got)
				}
			})
		}
	}
}

func TestMichiAcceptedPatternHitsRemainDeduplicated(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-beta','{"name":"Beta"}')`); err != nil {
		t.Fatal(err)
	}
	for _, template := range []string{"find {entity} details", "details {entity} find"} {
		if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: template, ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Recall(context.Background(), db, "find Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 {
		t.Fatalf("accepted duplicate pattern IDs leaked: %+v", got)
	}
}

func TestMichiStandalonePatternApplyRetainsCandidateCaps(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	for i := 0; i < 12; i++ {
		suffix := strconv.Itoa(i)
		id := "resource-" + suffix + "-beta"
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{"name":"Beta"}')`, id); err != nil {
			t.Fatal(err)
		}
		if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-" + suffix + "-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		opts patterns.Opts
		want int
	}{{"default", patterns.Opts{}, 10}, {"explicit", patterns.Opts{Limit: 2}, 2}, {"internal filtering candidates", patterns.Opts{NoLimit: true}, 12}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := patterns.Apply(context.Background(), db, "find Beta details", "details find", []string{"Beta"}, tc.opts)
			if err != nil || len(got) != tc.want {
				t.Fatalf("standalone candidate cap got=%d want=%d err=%v", len(got), tc.want, err)
			}
		})
	}
}

func TestMichiSamePatternConsidersLaterValidEntityBinding(t *testing.T) {
	for _, tc := range []struct {
		name, query, alphaPayload, betaPayload, wantID string
	}{
		{"rejected first binding", "find Alpha Beta details", `{"name":"Gamma"}`, `{"name":"Beta"}`, "resource-beta"},
		{"first binding valid", "find Alpha Beta details", `{"name":"Alpha"}`, `{"name":"Gamma"}`, "resource-alpha"},
		{"later binding true alias", "find Alpha B1 details", `{"name":"Gamma"}`, `{"name":"Beta"}`, "resource-b1"},
		{"later unknown identity", "find Alpha Beta details", `{"name":"Gamma"}`, `{}`, "resource-beta"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "beta_kind", "Beta", []string{"B1"})
			secondID := "resource-beta"
			if strings.Contains(tc.query, "B1") {
				secondID = "resource-b1"
			}
			for id, data := range map[string]string{"resource-alpha": tc.alphaPayload, secondID: tc.betaPayload} {
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, id, data); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
				t.Fatal(err)
			}
			standalone, err := patterns.Apply(context.Background(), db, tc.query, "details find", []string{"Alpha", "Beta"}, patterns.Opts{})
			if err != nil || len(standalone) != 1 || standalone[0].ResourceID != "resource-alpha" {
				t.Fatalf("standalone first-binding behavior changed: %+v err=%v", standalone, err)
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true, Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != tc.wantID {
				t.Fatalf("same-pattern alternate binding hidden: %+v", got)
			}
		})
	}
}

func TestMichiAcceptedTargetDoesNotEraseUnrelatedMismatch(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	for id, data := range map[string]string{"resource-target": `{"name":"Beta"}`, "resource-other": `{"name":"Gamma"}`} {
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, id, data); err != nil {
			t.Fatal(err)
		}
	}
	seedCanonicalLookup(t, db, "alpha-map", "Alpha", []string{"target"})
	seedCanonicalLookup(t, db, "beta-map", "Beta", []string{"target"})
	seedCanonicalLookup(t, db, "other-map", "Alpha", []string{"other"})
	for _, p := range []patterns.Pattern{
		{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:alpha-map}", EntityKind: "alpha-map"},
		{QueryTemplate: "details {entity} find", ResourceTemplate: "resource-{entity:beta-map}", EntityKind: "beta-map"},
		{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:other-map}", EntityKind: "other-map"},
	} {
		p.ResourceType, p.Strategy, p.Source = "widgets", patterns.StrategySubstitute, patterns.SourceTaught
		if _, _, err := patterns.Upsert(db, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, PatternKinds: []string{"alpha-map", "beta-map", "other-map"}, DebugMismatches: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-target" {
		t.Fatalf("valid result missing: %+v", got)
	}
	if len(got.Mismatches) != 1 || got.Mismatches[0].ResourceID != "resource-other" {
		t.Fatalf("unrelated mismatch lost or accepted mismatch retained: %+v", got)
	}
	warnings := strings.Join(got.Warnings, " ")
	if !strings.Contains(warnings, WarningSimilarShapeDifferentEntity+":Gamma") || strings.Contains(warnings, WarningSimilarShapeDifferentEntity+":Beta") {
		t.Fatalf("mismatch warning evidence was not reconciled: %+v", got)
	}
}

func TestMichiRecallLimitPreservesFinalRankingAcrossLocalStoreSizes(t *testing.T) {
	for _, size := range []int{100, 1000} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			for i := 0; i < size; i++ {
				id := "resource-" + strconv.Itoa(i) + "-alpha"
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{"name":"Alpha"}')`, id); err != nil {
					t.Fatal(err)
				}
				queryTemplate := "find {entity} details"
				confidence := 2
				if i == size-1 {
					queryTemplate = "find {entity} details summary"
					confidence = 10
				}
				if _, err := db.Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,strategy,entity_kind,source,confidence) VALUES(?,?,'widgets',?,'lowercase','taught',?)`, queryTemplate, "resource-"+strconv.Itoa(i)+"-{entity:lowercase}", patterns.StrategySubstitute, confidence); err != nil {
					t.Fatal(err)
				}
			}
			for rep := 1; rep <= 3; rep++ {
				started := time.Now()
				got, err := Recall(context.Background(), db, "find Alpha details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, Limit: 1})
				elapsed := time.Since(started)
				if err != nil {
					t.Fatal(err)
				}
				want := "resource-" + strconv.Itoa(size-1) + "-alpha"
				if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != want {
					t.Fatalf("Apply order replaced final Recall ranking: %+v", got)
				}
				t.Logf("local_recall_patterns=%d limit=1 rep=%d elapsed_ms=%.3f", size, rep, float64(elapsed.Microseconds())/1000)
			}
		})
	}
}

func TestMichiSamePatternAcceptedIDRemainsUniqueAndConsistent(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-target','{"name":"Beta"}')`); err != nil {
		t.Fatal(err)
	}
	for _, entity := range []string{"Alpha", "Beta"} {
		seedCanonicalLookup(t, db, "shared-map", entity, []string{"target"})
	}
	if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:shared-map}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "shared-map", Source: patterns.SourceTaught}); err != nil {
		t.Fatal(err)
	}
	got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, PatternKinds: []string{"shared-map"}, DebugMismatches: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-target" || len(got.Mismatches) != 0 || strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity) {
		t.Fatalf("same-pattern duplicate accepted ID has contradictory result: %+v", got)
	}
}

func TestMichiSamePatternRetainsFirstAcceptedBinding(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	for _, entity := range []string{"Alpha", "Beta"} {
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, "resource-"+strings.ToLower(entity), `{"name":"`+entity+`"}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
		t.Fatal(err)
	}
	got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-alpha" || len(got.Mismatches) != 0 {
		t.Fatalf("one accepted binding per pattern changed: %+v", got)
	}
}

func TestMichiSamePatternDuplicateBindingConsumesOnlyAcceptedIdentity(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid duplicate stops pattern", true: "conflicting duplicate tries later binding"}[conflicting], func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			firstEntity, wantCount := "Alpha", 1
			if conflicting {
				firstEntity, wantCount = "Beta", 2
			}
			for id, entity := range map[string]string{"resource-first-target": firstEntity, "resource-second-target": "Beta"} {
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, id, `{"name":"`+entity+`"}`); err != nil {
					t.Fatal(err)
				}
			}
			seedCanonicalLookup(t, db, "prior-map", firstEntity, []string{"first-target"})
			seedCanonicalLookup(t, db, "next-map", "Alpha", []string{"first-target"})
			seedCanonicalLookup(t, db, "next-map", "Beta", []string{"second-target"})
			for _, kind := range []string{"prior-map", "next-map"} {
				id, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:" + kind + "}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: kind, Source: patterns.SourceTaught})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "prior-map" {
					if _, err := db.Exec(`UPDATE search_patterns SET confidence=10 WHERE id=?`, id); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, PatternKinds: []string{"prior-map", "next-map"}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Found || len(got.Results) != wantCount || len(got.Mismatches) != 0 || strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity) {
				t.Fatalf("duplicate binding consumed the wrong pattern state: %+v", got)
			}
		})
	}
}

func TestMichiFinalTypedIDDedupKeepsBestValidatedHit(t *testing.T) {
	for _, tc := range []struct {
		name, payload         string
		teaching, highPattern bool
		wantSource, wantMatch string
		wantConfidence        int
	}{
		{"direct partial yields to pattern exact", `{}`, true, false, SourcePattern, EntityMatchExact, 2},
		{"pattern confidence precedes match score", `{"name":"Alpha"}`, false, true, SourcePattern, EntityMatchExact, 10},
		{"direct exact retains source priority", `{"name":"Alpha"}`, true, true, "taught", EntityMatchExact, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-alpha',?)`, tc.payload); err != nil {
				t.Fatal(err)
			}
			if tc.teaching {
				seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, "resource-alpha", "widgets")
			}
			if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
				t.Fatal(err)
			}
			if tc.highPattern {
				id, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details summary", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec(`UPDATE search_patterns SET confidence=10 WHERE id=?`, id); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Recall(context.Background(), db, "find Alpha details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true, Limit: 1})
			if err != nil {
				t.Fatal(err)
			}
			if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-alpha" || got.Results[0].Source != tc.wantSource || got.Results[0].EntityMatch != tc.wantMatch || got.Results[0].Confidence != tc.wantConfidence || len(got.Mismatches) != 0 {
				t.Fatalf("typed-ID dedup retained lower-ranked evidence: %+v", got)
			}
		})
	}
}

func TestMichiFinalDedupPreservesDifferentTypedResources(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	for _, resourceType := range []string{"widgets", "other-widgets"} {
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES(?,'resource-alpha','{"name":"Alpha"}')`, resourceType); err != nil {
			t.Fatal(err)
		}
	}
	seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, "resource-alpha", "widgets")
	if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "other-widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
		t.Fatal(err)
	}
	got, err := Recall(context.Background(), db, "find Alpha details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}, "other-widgets": {"name"}}, DebugMismatches: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 || got.Results[0].ResourceType != "widgets" || got.Results[1].ResourceType != "other-widgets" || len(got.Mismatches) != 0 {
		t.Fatalf("same IDs of different types collapsed: %+v", got)
	}
}

func TestMichiRecallResourceReadFailuresDoNotBecomeFallbackHits(t *testing.T) {
	for _, mode := range []string{"direct", "pattern"} {
		for _, failure := range []string{"missing column", "null payload"} {
			t.Run(mode+"/"+failure, func(t *testing.T) {
				db := openRecallCanonicalTestDB(t)
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-alpha','{"name":"Alpha"}')`); err != nil {
					t.Fatal(err)
				}
				if mode == "direct" {
					seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, "resource-alpha", "widgets")
				} else {
					if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
						t.Fatal(err)
					}
				}
				if failure == "missing column" {
					if _, err := db.Exec(`ALTER TABLE resources RENAME COLUMN data TO unreadable_data`); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := db.Exec(`ALTER TABLE resources RENAME TO readable_ids`); err != nil {
						t.Fatal(err)
					}
					if _, err := db.Exec(`CREATE VIEW resources AS SELECT resource_type,id,NULL AS data,synced_at,updated_at FROM readable_ids`); err != nil {
						t.Fatal(err)
					}
				}
				got, err := Recall(context.Background(), db, "find Alpha details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}})
				if err == nil || got.Found || len(got.Results) != 0 {
					t.Fatalf("failed cached identity read became successful fallback: result=%+v err=%v", got, err)
				}
			})
		}
	}
}

func TestMichiCancelledPayloadReadDoesNotBecomeMissingResource(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-alpha','{"name":"Alpha"}')`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hit := Hit{ResourceID: "resource-alpha", ResourceType: "widgets", Confidence: 2}
	present, err := validateResource(ctx, db, canonicalTestConfig(), &hit, []string{"Alpha"}, []string{"Alpha"}, map[string][]string{"widgets": {"name"}})
	if present || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was not preserved: present=%v err=%v", present, err)
	}
	if hit.EntityMatch == EntityMatchExact || strings.Contains(strings.Join(hit.Warnings, " "), WarningResourceNotInStore) {
		t.Fatalf("cancelled read was represented as missing/exact: %+v", hit)
	}
}

func TestMichiDelimiterTuplesRemainDistinctInResultsAndDiagnostics(t *testing.T) {
	for _, conflicting := range []bool{false, true} {
		t.Run(map[bool]string{false: "two accepted tuples", true: "accepted and different rejected tuple"}[conflicting], func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			for _, tuple := range [][2]string{{"a|b", "c"}, {"a", "b|c"}} {
				name := "Alpha"
				if conflicting && tuple[0] == "a" {
					name = "Beta"
				}
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES(?,?,?)`, tuple[0], tuple[1], `{"name":"`+name+`"}`); err != nil {
					t.Fatal(err)
				}
				seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, tuple[1], tuple[0])
			}
			got, err := Recall(context.Background(), db, "find Alpha details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"a|b": {"name"}, "a": {"name"}}, DebugMismatches: true, Limit: 2})
			if err != nil {
				t.Fatal(err)
			}
			wantResults, wantMismatches := 2, 0
			if conflicting {
				wantResults, wantMismatches = 1, 1
			}
			if len(got.Results) != wantResults || len(got.Mismatches) != wantMismatches {
				t.Fatalf("distinct tuple keys collided: %+v", got)
			}
			if conflicting && (got.Mismatches[0].ResourceType != "a" || got.Mismatches[0].ResourceID != "b|c" || !strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity)) {
				t.Fatalf("unrelated rejected tuple diagnostics suppressed: %+v", got)
			}
		})
	}
}
