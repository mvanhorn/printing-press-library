// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package learn

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn/entities"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/learn/patterns"
	"modernc.org/sqlite"
)

const carstayAlphaID = "000000000000000000000abc"
const carstayBetaID = "000000000000000000000def"

func carstayRankedFixture(t *testing.T) (*sql.DB, Opts) {
	t.Helper()
	carstayRegisterProbe(t)
	db := openRecallCanonicalTestDB(t)
	seedCanonicalLookup(t, db, "alpha_alias", "Alpha-Canonical", []string{"Alpha", "A1"})
	seedCanonicalLookup(t, db, "beta_alias", "Beta-Canonical", []string{"Beta", "BetaAlias"})
	seedCanonicalLookup(t, db, "target", "Alpha", []string{"abc"})
	seedCanonicalLookup(t, db, "target", "BetaAlias", []string{"def"})
	cfg := entities.NewConfig()
	cfg.RegisterStopwords("today")
	return db, Opts{EntityConfig: cfg, ResourceTypeFields: map[string][]string{"widgets": {"name"}, "other": {"name"}}, Limit: 1, DebugMismatches: true}
}
func carstayRankedResource(t *testing.T, db *sql.DB, kind, id, payload string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES(?,?,?)`, kind, id, payload); err != nil {
		t.Fatal(err)
	}
}
func carstayRankedPattern(t *testing.T, db *sql.DB, queryTemplate, template, kind, lookup string, confidence int, prefix bool) {
	t.Helper()
	strategy := patterns.StrategySubstitute
	if prefix {
		strategy = patterns.StrategySubstituteThenSearchPrefix
		template += "*"
	}
	if _, err := db.Exec(`INSERT INTO search_patterns(query_template,resource_template,resource_type,strategy,entity_kind,confidence,source) VALUES(?,?,?,?,?,?,'taught')`, queryTemplate, template, kind, strategy, lookup, confidence); err != nil {
		t.Fatal(err)
	}
}
func carstayRankedRecall(t *testing.T, db *sql.DB, query string, opts Opts) Result {
	t.Helper()
	result, err := Recall(context.Background(), db, query, opts)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCarstayRecallTriesLaterExistingBindingWithinPattern(t *testing.T) {
	for _, prefix := range []bool{false, true} {
		t.Run(fmt.Sprintf("prefix=%v", prefix), func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			carstayRankedResource(t, db, "widgets", carstayAlphaID, `{"name":"Gamma"}`)
			carstayRankedResource(t, db, "widgets", carstayBetaID, `{"name":"Beta"}`)
			carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:target}", "widgets", "target", 2, prefix)
			got := carstayRankedRecall(t, db, "Alpha BetaAlias widget today", opts)
			if len(got.Results) != 1 || got.Results[0].ResourceID != carstayBetaID || got.Results[0].EntityMatch != EntityMatchExact || len(got.Mismatches) != 1 || got.Mismatches[0].ResourceID != carstayAlphaID {
				t.Fatalf("later verified identity hidden: %+v", got)
			}
			if !strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity+":Gamma") {
				t.Fatalf("rejected-only evidence lost: %+v", got)
			}
			// Standalone Apply keeps its existing first ID-verified binding contract.
			standalone, err := patterns.Apply(context.Background(), db, "Alpha BetaAlias widget today", "widget", []string{"Alpha", "BetaAlias"}, patterns.Opts{Limit: 1})
			if err != nil || len(standalone) != 1 || standalone[0].ResourceID != carstayAlphaID || standalone[0].BoundEntity != "Alpha" {
				t.Fatalf("standalone changed: %+v %v", standalone, err)
			}
		})
	}
}

func TestCarstayRecallValidDuplicateConsumesPatternButConflictDoesNot(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflict=%v", conflict), func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			opts.Limit = 3
			payload, teaching, entity := `{"name":"Alpha"}`, "alpha widget today", `["Alpha"]`
			if conflict {
				payload, teaching, entity = `{"name":"Beta"}`, "betaalias widget today", `["BetaAlias"]`
			}
			carstayRankedResource(t, db, "widgets", carstayAlphaID, payload)
			carstayRankedResource(t, db, "widgets", carstayBetaID, `{"name":"Beta"}`)
			seedCanonicalLearning(t, db, teaching, entity, carstayAlphaID, "widgets")
			carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:target}", "widgets", "target", 2, false)
			got := carstayRankedRecall(t, db, "Alpha BetaAlias widget today", opts)
			want := 1
			if conflict {
				want = 2
			}
			if len(got.Results) != want || len(got.Mismatches) != 0 {
				t.Fatalf("duplicate pattern bookkeeping: %+v", got)
			}
			if !conflict && got.Results[0].Source == SourcePattern {
				t.Fatalf("valid direct target lost: %+v", got)
			}
			if strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity) {
				t.Fatalf("accepted duplicate still rejected: %+v", got)
			}
		})
	}
}

func TestCarstayRecallBestTypedIdentityRetainsExactUpgrade(t *testing.T) {
	for _, tc := range []struct {
		name        string
		same, exact bool
	}{{"same target partial upgrades", true, false}, {"different target exact outranks partial", false, false}, {"exact direct retains priority", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			opts.DebugMismatches = false
			directID, patternID := "direct-abc", "pattern-abc"
			if tc.same {
				patternID = directID
			}
			payload := `{}`
			if tc.exact {
				payload = `{"name":"Alpha"}`
			}
			carstayRankedResource(t, db, "widgets", directID, payload)
			if !tc.same {
				carstayRankedResource(t, db, "widgets", patternID, `{"name":"Alpha"}`)
			}
			seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, directID, "widgets")
			if _, err := db.Exec(`UPDATE search_learnings SET confidence=99`); err != nil {
				t.Fatal(err)
			}
			prefix := "pattern-"
			if tc.same {
				prefix = "direct-"
			}
			carstayRankedPattern(t, db, "widget {entity}", prefix+"{entity:target}", "widgets", "target", 2, false)
			got := carstayRankedRecall(t, db, "Alpha widget today", opts)
			source := SourcePattern
			id := patternID
			if tc.exact {
				source = "taught"
				id = directID
			}
			if len(got.Results) != 1 || got.Results[0].ResourceID != id || got.Results[0].Source != source || got.Results[0].EntityMatch != EntityMatchExact {
				t.Fatalf("best typed evidence/rank lost: %+v", got)
			}
		})
	}
}

func TestCarstayRecallDiagnosticsExcludeOnlyFinalAcceptedTypedIDs(t *testing.T) {
	db, opts := carstayRankedFixture(t)
	seedCanonicalLookup(t, db, "wrong_target", "Alpha", []string{"abc"})
	seedCanonicalLookup(t, db, "right_target", "BetaAlias", []string{"abc"})
	carstayRankedResource(t, db, "widgets", carstayAlphaID, `{"name":"Beta"}`)
	carstayRankedResource(t, db, "other", carstayAlphaID, `{"name":"Gamma"}`)
	for _, p := range []struct {
		lookup, kind string
		confidence   int
	}{{"wrong_target", "widgets", 99}, {"wrong_target", "other", 90}, {"right_target", "widgets", 2}} {
		carstayRankedPattern(t, db, "widget {entity}", "000000000000000000000{entity:"+p.lookup+"}", p.kind, p.lookup, p.confidence, false)
	}
	got := carstayRankedRecall(t, db, "Alpha BetaAlias widget today", opts)
	if len(got.Results) != 1 || got.Results[0].ResourceType != "widgets" || len(got.Mismatches) != 1 || got.Mismatches[0].ResourceType != "other" {
		t.Fatalf("accepted filtering/cap/type boundary: %+v", got)
	}
	// A missing cache row keeps its teaching-only alternative evidence.
	seedCanonicalLearning(t, db, "delta widget today", `["Delta"]`, "missing-delta", "widgets")
	opts.Limit = 2
	withMissing := carstayRankedRecall(t, db, "Alpha BetaAlias widget today", opts)
	warnings := strings.Join(withMissing.Warnings, " ")
	if strings.Contains(warnings, WarningSimilarShapeDifferentEntity+":Beta") || !strings.Contains(warnings, WarningSimilarShapeDifferentEntity+":Gamma") || !strings.Contains(warnings, WarningSimilarShapeDifferentEntity+":Delta") {
		t.Fatalf("contradictory/lost alternative warnings: %+v", got)
	}
}

var carstayIdentityReads atomic.Int64
var carstayProbeOnce sync.Once
var carstayProbeError error

func carstayRegisterProbe(t *testing.T) {
	t.Helper()
	carstayProbeOnce.Do(func() {
		carstayProbeError = sqlite.RegisterScalarFunction("carstay_ranked_identity_payload", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			carstayIdentityReads.Add(1)
			return args[0], nil
		})
	})
	if carstayProbeError != nil {
		t.Fatal(carstayProbeError)
	}
}

func carstayIdentityProbe(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`ALTER TABLE resources RENAME TO ranked_identity_rows`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE VIEW resources AS SELECT resource_type,id,carstay_ranked_identity_payload(data) AS data FROM ranked_identity_rows`); err != nil {
		t.Fatal(err)
	}
	carstayIdentityReads.Store(0)
}
func TestCarstayRecallRankedLimitStopsActualIdentityReads(t *testing.T) {
	for _, count := range []int{12, 30} {
		t.Run(fmt.Sprintf("patterns=%d", count), func(t *testing.T) {
			db, opts := carstayRankedFixture(t)
			opts.DebugMismatches = false
			for i := 0; i < count; i++ {
				id := fmt.Sprintf("fixture-%02d-alpha", i)
				carstayRankedResource(t, db, "widgets", id, `{"name":"Alpha"}`)
				carstayRankedPattern(t, db, "widget {entity}", fmt.Sprintf("fixture-%02d-{entity:lowercase}", i), "widgets", "lowercase", count-i, false)
			}
			carstayIdentityProbe(t, db)
			got := carstayRankedRecall(t, db, "Alpha widget today", opts)
			if len(got.Results) != 1 || got.Results[0].ResourceID != "fixture-00-alpha" || carstayIdentityReads.Load() != 1 {
				t.Fatalf("unbounded/incorrect accepted work: reads=%d result=%+v", carstayIdentityReads.Load(), got)
			}
		})
	}
}
func TestCarstayRecallRankedLimitPreservesFinalConfidenceOrder(t *testing.T) {
	db, opts := carstayRankedFixture(t)
	opts.DebugMismatches = false
	carstayRankedResource(t, db, "widgets", "weak-alpha", `{"name":"Alpha"}`)
	carstayRankedResource(t, db, "widgets", "strong-alpha", `{"name":"Alpha"}`)
	carstayRankedPattern(t, db, "check widget {entity}", "weak-{entity:lowercase}", "widgets", "lowercase", 2, false)
	carstayRankedPattern(t, db, "check extra widget {entity}", "strong-{entity:lowercase}", "widgets", "lowercase", 9, false)
	carstayIdentityProbe(t, db)
	got := carstayRankedRecall(t, db, "Alpha widget check today", opts)
	if len(got.Results) != 1 || got.Results[0].ResourceID != "strong-alpha" || got.Results[0].MatchScore != 2.0/3.0 || carstayIdentityReads.Load() != 1 {
		t.Fatalf("raw-score stopping hid stronger final rank: reads%d %+v", carstayIdentityReads.Load(), got)
	}
	standalone, err := patterns.Apply(context.Background(), db, "Alpha widget check today", "check widget", []string{"Alpha"}, patterns.Opts{Limit: 1})
	if err != nil || len(standalone) != 1 || standalone[0].ResourceID != "weak-alpha" {
		t.Fatalf("standalone score order changed: %+v %v", standalone, err)
	}
}

func TestCarstayRecallDelimiterTuplesDoNotEraseRejectedEvidence(t *testing.T) {
	db, opts := carstayRankedFixture(t)
	opts.ResourceTypeFields = map[string][]string{"a|b": {"name"}, "a": {"name"}}
	seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, "c", "a|b")
	seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, "b|c", "a")
	carstayRankedResource(t, db, "a|b", "c", `{"name":"Alpha"}`)
	carstayRankedResource(t, db, "a", "b|c", `{"name":"Beta"}`)
	got := carstayRankedRecall(t, db, "Alpha widget today", opts)
	if len(got.Results) != 1 || got.Results[0].ResourceType != "a|b" || got.Results[0].ResourceID != "c" || len(got.Mismatches) != 1 || got.Mismatches[0].ResourceType != "a" || got.Mismatches[0].ResourceID != "b|c" {
		t.Fatalf("accepted tuple erased distinct rejected tuple: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Warnings, " "), WarningSimilarShapeDifferentEntity) {
		t.Fatalf("distinct rejected alternatives lost: %+v", got)
	}
}
