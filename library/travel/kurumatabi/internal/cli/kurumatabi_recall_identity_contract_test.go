// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/learn/patterns"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/store"
)

func TestKurumatabiActualRecallCachedIdentityGatesBothPaths(t *testing.T) {
	cases := []struct {
		name, query, payload, wantMatch     string
		direct, pattern, missing, wantFound bool
	}{
		{"direct_literal_conflict", "Alpha park", `{"name":"Beta"}`, "", true, false, false, false},
		{"direct_canonical_alias_conflict", "A1 park", `{"name":"Beta"}`, "", true, false, false, false},
		{"pattern_conflict", "A1 park", `{"name":"Beta"}`, "", false, true, false, false},
		{"pattern_unrelated_query_alias_conflict", "A1 B1 park", `{"name":"Beta"}`, "", false, true, false, false},
		{"pattern_unrelated_literal_conflict", "A1 B1 park", `{"name":"B1"}`, "", false, true, false, false},
		{"pattern_reversed_unrelated_alias_conflict", "B1 A1 park", `{"name":"Beta"}`, "", false, true, false, false},
		{"pattern_bound_literal_matches", "A1 B1 park", `{"name":"A1"}`, learn.EntityMatchExact, false, true, false, true},
		{"pattern_bound_query_alias_matches", "A1 B1 park", `{"name":"Alpha"}`, learn.EntityMatchExact, false, true, false, true},
		{"direct_and_pattern_conflict", "A1 park", `{"name":"Beta"}`, "", true, true, false, false},
		{"direct_cached_alias", "A1 park", `{"name":"Alpha"}`, learn.EntityMatchExact, true, false, false, true},
		{"pattern_cached_alias", "A1 park", `{"name":"Alpha"}`, learn.EntityMatchExact, false, true, false, true},
		{"direct_and_pattern_cached_alias", "A1 park", `{"name":"Alpha"}`, learn.EntityMatchExact, true, true, false, true},
		{"warned_missing_resource", "A1 park", "", learn.EntityMatchExact, true, false, true, true},
		{"direct_unknown_identity", "A1 park", `{}`, learn.EntityMatchPartial, true, false, false, true},
		{"pattern_unknown_identity", "A1 park", `{}`, learn.EntityMatchExact, false, true, false, true},
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
			// Self-contained synthetic aliases exercise the real configured source identity map.
			if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('synthetic-place','Alpha','A1','taught'),('synthetic-place','Beta','B1','taught')`); err != nil {
				t.Fatal(err)
			}
			id := "park-A1"
			if !tc.missing {
				if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source',?,?)`, id, tc.payload); err != nil {
					t.Fatal(err)
				}
			}
			if tc.pattern {
				_, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "park {entity}", ResourceTemplate: "park-{entity:uppercase}", ResourceType: "source", EntityKind: "uppercase", Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if tc.direct {
				_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha park", "--resource", id, "--resource-type", "source", "--db", path)
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
				if !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") {
					t.Fatalf("ordinary warning hides actual cached alternative: %#v", result.Warnings)
				}
				var cached struct {
					Name string `json:"name"`
				}
				if err := json.Unmarshal([]byte(tc.payload), &cached); err != nil {
					t.Fatal(err)
				}
				for _, mismatch := range result.Mismatches {
					if mismatch.EntityMatch != learn.EntityMatchMismatch || !slices.Contains(mismatch.ResourceEntities, cached.Name) {
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

func TestKurumatabiActualRecallAmbiguousAliasNeedsSharedResourceCanonical(t *testing.T) {
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
				if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('synthetic-place',?,'Z1','taught')`, canonical); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-Z1',?)`, `{"name":"`+name+`"}`); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha park", "--resource", "park-Z1", "--resource-type", "source", "--db", path)
			if err != nil {
				t.Fatalf("teach: %v %s", err, stderr)
			}
			stdout, stderr, err := runRootArgs(t, "recall", "Z1 park", "--db", path, "--agent", "--debug-mismatches")
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

func TestKurumatabiRejectedPatternDoesNotSuppressLaterValidBinding(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "pattern-dedup.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := initLearn(context.Background(), s.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES
 ('synthetic-place','Alpha','A1','taught'),('synthetic-place','Beta','B1','taught'),
 ('alpha-binding','A1','shared','taught'),('beta-binding','B1','shared','taught'),('beta-duplicate','B1','shared','taught')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-shared','{"name":"Beta"}')`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"alpha-binding", "beta-binding", "beta-duplicate"} {
		if _, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "park {entity}", ResourceTemplate: "park-{entity:" + kind + "}", ResourceType: "source", EntityKind: kind, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	// Force the known-conflicting Alpha binding before both valid Beta bindings.
	if _, err := s.DB().Exec(`UPDATE search_patterns SET confidence=10 WHERE entity_kind='alpha-binding'`); err != nil {
		t.Fatal(err)
	}
	bound, err := patterns.Apply(context.Background(), s.DB(), "A1 B1 park", "park", []string{"A1", "B1"}, patterns.Opts{})
	if err != nil || len(bound) != 3 || bound[0].BoundEntity != "A1" {
		t.Fatalf("fixture does not exercise rejected-first ordering: %#v %v", bound, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runRootArgs(t, "recall", "A1 B1 park", "--db", path, "--agent", "--debug-mismatches", "--limit", "1")
	if err != nil {
		t.Fatalf("recall: %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, stdout, &result)
	if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "park-shared" || result.Results[0].EntityMatch != learn.EntityMatchExact {
		t.Fatalf("rejected binding hid later valid hit or accepted duplicate survived: %#v", result)
	}
	if len(result.Mismatches) != 0 || slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") {
		t.Fatalf("accepted typed target retained contradictory diagnostics: %#v", result)
	}
}

func seedKurumatabiBindingDB(t *testing.T) (*store.Store, string) {
	t.Helper()
	home := withTempLearnHome(t)
	path := filepath.Join(home, "binding-ranking.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := initLearn(context.Background(), s.DB()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('synthetic-place','Alpha','A1','taught'),('synthetic-place','Beta','B1','taught')`); err != nil {
		t.Fatal(err)
	}
	return s, path
}
func putKurumatabiBindingPattern(t *testing.T, s *store.Store, kind, template string, confidence int) {
	t.Helper()
	id, _, err := patterns.Upsert(s.DB(), patterns.Pattern{QueryTemplate: "park {entity}", ResourceTemplate: template, ResourceType: "source", EntityKind: kind, Strategy: patterns.StrategySubstitute, Source: patterns.SourceTaught})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE search_patterns SET confidence=? WHERE id=?`, confidence, id); err != nil {
		t.Fatal(err)
	}
}
func recallKurumatabiBindingDB(t *testing.T, s *store.Store, path string) learn.Result {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runRootArgs(t, "recall", "A1 B1 park", "--db", path, "--agent", "--debug-mismatches", "--limit", "10")
	if err != nil {
		t.Fatalf("recall %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, out, &result)
	return result
}
func TestKurumatabiSamePatternTriesLaterCompatibleBinding(t *testing.T) {
	for _, payload := range []string{`{"name":"Beta"}`, `{"name":"B1"}`} {
		t.Run(payload, func(t *testing.T) {
			s, path := seedKurumatabiBindingDB(t)
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-A1',?),('source','park-B1','{"name":"Beta"}')`, payload); err != nil {
				t.Fatal(err)
			}
			putKurumatabiBindingPattern(t, s, "uppercase", "park-{entity:uppercase}", 2)
			// Standalone Apply still chooses its original first existing binding.
			raw, err := patterns.Apply(context.Background(), s.DB(), "A1 B1 park", "park", []string{"A1", "B1"}, patterns.Opts{})
			if err != nil || len(raw) != 1 || raw[0].BoundEntity != "A1" {
				t.Fatalf("standalone changed %#v %v", raw, err)
			}
			result := recallKurumatabiBindingDB(t, s, path)
			if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "park-B1" || result.Results[0].EntityMatch != learn.EntityMatchExact {
				t.Fatalf("same-pattern rejection hid compatible B1: %#v", result)
			}
			if len(result.Mismatches) != 1 || result.Mismatches[0].ResourceID != "park-A1" {
				t.Fatalf("rejected-only evidence lost %#v", result)
			}
		})
	}
}
func TestKurumatabiFirstCompatibleDuplicateConsumesPatternOnlyAfterValidation(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "compatible-duplicate", true: "conflicting-duplicate"}[conflict], func(t *testing.T) {
			s, path := seedKurumatabiBindingDB(t)
			firstKind, firstEntity, payload := "alpha-only", "A1", `{"name":"Alpha"}`
			if conflict {
				firstKind, firstEntity, payload = "beta-only", "B1", `{"name":"Beta"}`
			}
			if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES(?,?,'shared','taught'),('pair-binding','A1','shared','taught'),('pair-binding','B1','new','taught')`, firstKind, firstEntity); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-shared',?),('source','park-new','{"name":"Beta"}')`, payload); err != nil {
				t.Fatal(err)
			}
			putKurumatabiBindingPattern(t, s, firstKind, "park-{entity:"+firstKind+"}", 5)
			putKurumatabiBindingPattern(t, s, "pair-binding", "park-{entity:pair-binding}", 2)
			result := recallKurumatabiBindingDB(t, s, path)
			want := 1
			if conflict {
				want = 2
			}
			if !result.Found || len(result.Results) != want || len(result.Mismatches) != 0 {
				t.Fatalf("pattern consumed before validated compatibility, or valid duplicate selected later binding: %#v", result)
			}
			if slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") {
				t.Fatalf("accepted shared target retained contradictory alternative %#v", result)
			}
		})
	}
}
func TestKurumatabiFinalTypedDedupAllowsExactUpgradeAndPreservesDirectExact(t *testing.T) {
	for _, payload := range []string{`{}`, `{"name":"Alpha"}`} {
		t.Run(payload, func(t *testing.T) {
			s, path := seedKurumatabiBindingDB(t)
			if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-A1',?)`, payload); err != nil {
				t.Fatal(err)
			}
			if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "Alpha park", QueryEntities: []string{"Alpha"}, ResourceID: "park-A1", ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`UPDATE search_learnings SET confidence=10`); err != nil {
				t.Fatal(err)
			}
			putKurumatabiBindingPattern(t, s, "uppercase", "park-{entity:uppercase}", 2)
			result := recallKurumatabiBindingDB(t, s, path)
			if !result.Found || len(result.Results) != 1 || result.Results[0].EntityMatch != learn.EntityMatchExact {
				t.Fatalf("best typed-ID classification lost %#v", result)
			}
			wantSource := learn.SourcePattern
			if payload != `{}` {
				wantSource = store.LearningSourceTaught
			}
			if result.Results[0].Source != wantSource {
				t.Fatalf("source priority or exact upgrade lost got=%s want=%s", result.Results[0].Source, wantSource)
			}
		})
	}
}

func TestKurumatabiAcceptedIDsArePrunedBeforeDiagnosticLimit(t *testing.T) {
	s, path := seedKurumatabiBindingDB(t)
	if _, err := s.DB().Exec(`INSERT INTO entity_lookups(kind,canonical,value,source) VALUES('bad-shared','A1','shared','taught'),('bad-other','A1','other','taught'),('good-shared','B1','shared','taught')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`INSERT INTO resources(resource_type,id,data) VALUES('source','park-shared','{"name":"Beta"}'),('source','park-other','{"name":"Gamma"}')`); err != nil {
		t.Fatal(err)
	}
	putKurumatabiBindingPattern(t, s, "bad-shared", "park-{entity:bad-shared}", 10)
	putKurumatabiBindingPattern(t, s, "bad-other", "park-{entity:bad-other}", 9)
	putKurumatabiBindingPattern(t, s, "good-shared", "park-{entity:good-shared}", 2)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runRootArgs(t, "recall", "A1 B1 park", "--db", path, "--agent", "--debug-mismatches", "--limit", "1")
	if err != nil {
		t.Fatalf("recall %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, out, &result)
	if !result.Found || len(result.Results) != 1 || result.Results[0].ResourceID != "park-shared" || len(result.Mismatches) != 1 || result.Mismatches[0].ResourceID != "park-other" {
		t.Fatalf("accepted-key filter or diagnostic limit lost rejected-only row %#v", result)
	}
	if slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Beta") || !slices.Contains(result.Warnings, learn.WarningSimilarShapeDifferentEntity+":Gamma") || !slices.Contains(result.Warnings, learn.WarningAmbiguousAlias) {
		t.Fatalf("contradictory alternative or independent warning changed %#v", result)
	}
}

func TestKurumatabiActualRecallPayloadReadFailureIsNotMissingRowSuccess(t *testing.T) {
	for _, mode := range []string{"direct", "pattern"} {
		t.Run(mode, func(t *testing.T) {
			s, path := seedKurumatabiBindingDB(t)
			// A legacy-compatible nullable table reaches the actual payload read.
			if _, err := s.DB().Exec(`DROP TABLE resources; CREATE TABLE resources(resource_type TEXT NOT NULL,id TEXT NOT NULL,data JSON,synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(resource_type,id)); INSERT INTO resources(resource_type,id,data) VALUES('source','park-A1',NULL)`); err != nil {
				t.Fatal(err)
			}
			if mode == "direct" {
				if _, _, err := s.UpsertLearning(context.Background(), store.UpsertLearningInput{Query: "Alpha park", QueryEntities: []string{"Alpha"}, ResourceID: "park-A1", ResourceType: "source", Source: store.LearningSourceTaught}); err != nil {
					t.Fatal(err)
				}
			} else {
				putKurumatabiBindingPattern(t, s, "uppercase", "park-{entity:uppercase}", 2)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := runRootArgs(t, "recall", "A1 park", "--db", path, "--agent")
			if err == nil || ExitCode(err) != 1 {
				t.Fatalf("payload read failure became successful missing fallback: exit%d err=%v out=%s stderr=%s", ExitCode(err), err, out, stderr)
			}
			if out != "" {
				t.Fatalf("failed recall emitted a successful factual envelope: %s", out)
			}
		})
	}
}

func TestKurumatabiRecallKeepsDistinctTypedTuplesWithDelimiter(t *testing.T) {
	home := withTempLearnHome(t)
	path := filepath.Join(home, "typed-tuples.db")
	tuples := []struct{ kind, id string }{{"a|b", "c"}, {"a", "b|c"}}
	for _, row := range tuples {
		_, stderr, err := runRootArgs(t, "teach", "--query", "Alpha park", "--resource", row.id, "--resource-type", row.kind, "--db", path)
		if err != nil {
			t.Fatalf("teach %q/%q %v %s", row.kind, row.id, err, stderr)
		}
	}
	out, stderr, err := runRootArgs(t, "recall", "Alpha park", "--db", path, "--agent", "--limit", "2")
	if err != nil {
		t.Fatalf("recall %v %s", err, stderr)
	}
	var result learn.Result
	unmarshalAgentResults(t, out, &result)
	seen := make(map[struct{ kind, id string }]bool)
	for _, hit := range result.Results {
		seen[struct{ kind, id string }{hit.ResourceType, hit.ResourceID}] = true
	}
	if !result.Found || len(result.Results) != 2 || !seen[tuples[0]] || !seen[tuples[1]] {
		t.Fatalf("distinct typed tuples collapsed %#v", result)
	}
}
