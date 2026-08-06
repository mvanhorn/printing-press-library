// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/webflow/internal/store"
)

// TestStringMapToAnyOnlyCoercesSwitchFields guards against the boolean
// coercion bug: a plain-text field whose value is literally "true" or "false"
// must stay a string. Only a field the schema marks as a Webflow "Switch"
// (boolean) field should become a JSON bool.
func TestStringMapToAnyOnlyCoercesSwitchFields(t *testing.T) {
	fieldTypes := map[string]string{
		"featured": "Switch",
		"headline": "PlainText",
	}
	got := stringMapToAny(map[string]string{
		"featured": "true",
		"headline": "false", // a real headline that happens to be the word "false"
		"unknown":  "true",  // no schema entry at all
	}, fieldTypes)

	if v, ok := got["featured"].(bool); !ok || v != true {
		t.Fatalf("featured (Switch) = %#v, want JSON bool true", got["featured"])
	}
	if v, ok := got["headline"].(string); !ok || v != "false" {
		t.Fatalf("headline (PlainText) = %#v, want string \"false\", not a bool", got["headline"])
	}
	if v, ok := got["unknown"].(string); !ok || v != "true" {
		t.Fatalf("unknown (no schema) = %#v, want string \"true\", not a bool", got["unknown"])
	}
}

// TestBulkSetResumeRoundTrip verifies the resume-state sidecar file: saving
// under one signature and loading under the same signature returns the
// applied IDs, a different signature returns nothing (no cross-batch bleed),
// and clearing removes the file so a later run starts fresh.
func TestBulkSetResumeRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")

	sigA := bulkSetResumeSignature("c1", map[string]string{"status": "draft"}, map[string]string{"author": "editorial"})
	sigB := bulkSetResumeSignature("c1", map[string]string{"status": "live"}, map[string]string{"author": "editorial"})
	if sigA == sigB {
		t.Fatal("different --match values must not produce the same signature")
	}

	saveBulkSetResume(sigA, []string{"i1", "i2"})

	got := loadBulkSetResume(sigA)
	if !got["i1"] || !got["i2"] || len(got) != 2 {
		t.Fatalf("loadBulkSetResume(sigA) = %v, want {i1, i2}", got)
	}

	if other := loadBulkSetResume(sigB); len(other) != 0 {
		t.Fatalf("loadBulkSetResume(sigB) = %v, want empty (different batch signature)", other)
	}

	clearBulkSetResume(sigA)
	if got := loadBulkSetResume(sigA); len(got) != 0 {
		t.Fatalf("loadBulkSetResume after clear = %v, want empty", got)
	}
}

// erroringFetcher always fails, simulating a live GET that cannot reach the
// API (network down, auth revoked mid-run, etc.).
type erroringFetcher struct{}

func (erroringFetcher) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return nil, fmt.Errorf("simulated network failure")
}

// TestCollectionFieldTypesPropagatesFetchFailure guards the regression
// Greptile flagged: when the local mirror has no schema for this collection
// and the live fallback GET fails, collectionFieldTypes must return that
// error rather than silently returning an empty type map (which would make
// every Switch field look like a plain-text field to the caller).
func TestCollectionFieldTypesPropagatesFetchFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()
	lq := &localQuery{db: s, ctx: context.Background()}

	types, err := collectionFieldTypes(lq, erroringFetcher{}, "c1")
	if err == nil {
		t.Fatalf("expected an error when local schema is absent and the live fetch fails, got types=%v", types)
	}
}

// TestAmbiguousBooleanSetField confirms the guard that decides whether a
// schema-fetch failure actually matters: only a --set value that is
// literally "true"/"false" is ambiguous between a Switch field and text.
func TestAmbiguousBooleanSetField(t *testing.T) {
	if _, found := ambiguousBooleanSetField(map[string]string{"headline": "hello"}); found {
		t.Fatal("a non-boolean-looking value must not be reported as ambiguous")
	}
	field, found := ambiguousBooleanSetField(map[string]string{"headline": "hello", "featured": "True"})
	if !found || field != "featured" {
		t.Fatalf("ambiguousBooleanSetField = (%q, %v), want (\"featured\", true)", field, found)
	}
}

// TestBulkSetResumeClearIgnoresOtherSignature guards the fix's core promise:
// finishing one bulk-set batch must never wipe out a still-in-progress
// resume file left by a different --match/--set batch on the same collection.
func TestBulkSetResumeClearIgnoresOtherSignature(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")

	sigA := bulkSetResumeSignature("c1", map[string]string{"status": "draft"}, map[string]string{"author": "editorial"})
	sigB := bulkSetResumeSignature("c1", map[string]string{"status": "live"}, map[string]string{"author": "editorial"})

	saveBulkSetResume(sigA, []string{"i1"})
	clearBulkSetResume(sigB) // finishing an unrelated batch

	if got := loadBulkSetResume(sigA); !got["i1"] {
		t.Fatalf("clearing signature B wiped signature A's resume state: %v", got)
	}
}

// TestBulkSetResumeSaveDoesNotOverwriteOtherSignature guards the regression
// Greptile flagged: two different --match/--set batches on the same
// collection, both rate-limited before either resumes, must not clobber each
// other's saved progress. All batches share one sidecar file, so saving must
// be a read-modify-write keyed by signature rather than a full overwrite.
func TestBulkSetResumeSaveDoesNotOverwriteOtherSignature(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")

	sigA := bulkSetResumeSignature("c1", map[string]string{"status": "draft"}, map[string]string{"author": "editorial"})
	sigB := bulkSetResumeSignature("c1", map[string]string{"status": "live"}, map[string]string{"author": "editorial"})

	saveBulkSetResume(sigA, []string{"i1", "i2"})
	saveBulkSetResume(sigB, []string{"i3"}) // a second batch, rate-limited before batch A resumes

	gotA := loadBulkSetResume(sigA)
	if !gotA["i1"] || !gotA["i2"] || len(gotA) != 2 {
		t.Fatalf("batch A's resume state after batch B saved = %v, want {i1, i2} (batch B must not overwrite it)", gotA)
	}
	gotB := loadBulkSetResume(sigB)
	if !gotB["i3"] || len(gotB) != 1 {
		t.Fatalf("batch B's resume state = %v, want {i3}", gotB)
	}
}

// TestSelectBulkTargetsAdvancesPastSkippedWindow guards the regression
// Greptile flagged: when the total matched set is larger than --limit and
// some already-applied items fall inside the sorted head, the window must
// advance to the next untouched items instead of reselecting the same
// already-done head and silently completing without touching the tail.
func TestSelectBulkTargetsAdvancesPastSkippedWindow(t *testing.T) {
	rows := make([]rawRow, 0, 5)
	for _, name := range []string{"A", "B", "C", "D", "E"} {
		rows = append(rows, rowOf(t, name, "c1", wfItem{ID: name, FieldData: map[string]any{"name": name, "status": "draft"}}))
	}
	set := map[string]string{"author": "editorial"}
	match := map[string]string{"status": "draft"}

	// A and B were already applied by a previous run and are the sorted head;
	// without the fix a --limit 3 window would reselect them instead of
	// advancing to the untouched tail (C, D, E).
	skip := map[string]bool{"A": true, "B": true}

	got := selectBulkTargets(rows, "c1", match, set, 3, skip)
	if got.Matched != 5 {
		t.Fatalf("Matched = %d, want 5", got.Matched)
	}
	if got.Skipped != 2 {
		t.Fatalf("Skipped = %d, want 2", got.Skipped)
	}
	if len(got.Changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3 (the untouched tail, not the already-applied head)", len(got.Changes))
	}
	for _, ch := range got.Changes {
		if ch.Name == "A" || ch.Name == "B" {
			t.Fatalf("window reselected an already-applied item: %+v", ch)
		}
	}
	for _, want := range []string{"C", "D", "E"} {
		found := false
		for _, ch := range got.Changes {
			if ch.Name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("window missing untouched item %q: %+v", want, got.Changes)
		}
	}
}

// TestNovelItemsBulkSetHelpWires smoke-tests that the items bulk-set command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelItemsBulkSetHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"items", "bulk-set", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("items bulk-set --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "bulk-set"} {
		if !strings.Contains(help, want) {
			t.Fatalf("items bulk-set --help missing %q in output:\n%s", want, help)
		}
	}
}
