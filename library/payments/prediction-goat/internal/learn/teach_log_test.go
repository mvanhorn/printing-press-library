// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
)

// withTempHomeForLog isolates HOME so the test's teach-log writes
// land in a fresh per-test directory rather than the developer's real
// ~/.local/share tree.
func withTempHomeForLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// readRawTeachLog returns the raw bytes of teach.log under the
// supplied HOME, for tests that want to assert line-shape rather than
// going through the parser.
func readRawTeachLog(t *testing.T, home string) string {
	t.Helper()
	p := filepath.Join(home, ".local", "share", "prediction-goat-pp-cli", "teach.log")
	b, err := os.ReadFile(p) // #nosec G304 -- test-only path under temp HOME.
	if err != nil {
		t.Fatalf("read teach.log: %v", err)
	}
	return string(b)
}

func TestAppendTeachLogWarning_CreatesDirectoryAndFile(t *testing.T) {
	home := withTempHomeForLog(t)

	// Directory shouldn't exist before the first write.
	stateDir := filepath.Join(home, ".local", "share", "prediction-goat-pp-cli")
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("precondition: state dir should not exist yet, got err=%v", err)
	}

	w := learn.Warning{
		Code:      learn.WarningParentEventWhenChildExists,
		Resource:  "KXMENWORLDCUP-26",
		Detail:    "taught against parent event for USA query",
		Suggested: "KXMENWORLDCUP-26-US",
	}
	if err := learn.AppendTeachLogWarning("teach", "odds USA wins world cup", w); err != nil {
		t.Fatalf("append: %v", err)
	}

	if info, err := os.Stat(stateDir); err != nil {
		t.Fatalf("state dir should exist after first append: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("state dir path is not a directory: %s", stateDir)
	}

	raw := readRawTeachLog(t, home)
	if !strings.HasSuffix(raw, "\n") {
		t.Errorf("teach.log line should end with newline; got %q", raw)
	}
	// First non-blank character should be '{' -- structured JSONL.
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("structured entry should start with '{'; got %q", trimmed)
	}
}

func TestAppendTeachLogWarning_JSONShape(t *testing.T) {
	home := withTempHomeForLog(t)
	w := learn.Warning{
		Code:      learn.WarningParentEventWhenChildExists,
		Resource:  "KXMENWORLDCUP-26",
		Detail:    "child KXMENWORLDCUP-26-US matches",
		Suggested: "KXMENWORLDCUP-26-US",
	}
	if err := learn.AppendTeachLogWarning("teach", "odds USA wins world cup", w); err != nil {
		t.Fatalf("append: %v", err)
	}

	raw := readRawTeachLog(t, home)
	line := strings.TrimSpace(raw)
	var entry learn.TeachLogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("unmarshal teach.log line: %v (line=%q)", err, line)
	}
	if entry.Action != "teach" {
		t.Errorf("want action=teach, got %q", entry.Action)
	}
	if entry.Query != "odds USA wins world cup" {
		t.Errorf("query mismatch: %q", entry.Query)
	}
	if entry.Resource != "KXMENWORLDCUP-26" {
		t.Errorf("resource mismatch: %q", entry.Resource)
	}
	if entry.Warning != learn.WarningParentEventWhenChildExists {
		t.Errorf("warning mismatch: %q", entry.Warning)
	}
	if entry.Suggested != "KXMENWORLDCUP-26-US" {
		t.Errorf("suggested mismatch: %q", entry.Suggested)
	}
	if entry.TS == "" {
		t.Errorf("ts should be populated; got empty")
	}
}

func TestAppendTeachLogWarning_MultipleAppendsPreservePriorEntries(t *testing.T) {
	withTempHomeForLog(t)

	for i, code := range []string{"parent_event_when_child_exists", "no_entity_overlap"} {
		w := learn.Warning{
			Code:     code,
			Resource: "RESOURCE-" + string(rune('A'+i)),
			Detail:   "entry " + string(rune('A'+i)),
		}
		if err := learn.AppendTeachLogWarning("teach", "q", w); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	entries, err := learn.ReadTeachLogWarnings()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d (%+v)", len(entries), entries)
	}
	if entries[0].Resource != "RESOURCE-A" || entries[1].Resource != "RESOURCE-B" {
		t.Errorf("append order should be preserved; got %+v", entries)
	}
}

func TestAppendTeachLogWarning_EmptyCodeIsAnError(t *testing.T) {
	withTempHomeForLog(t)
	if err := learn.AppendTeachLogWarning("teach", "q", learn.Warning{}); err == nil {
		t.Errorf("want error for empty warning code; got nil")
	}
}

func TestReadTeachLogWarnings_MissingFileReturnsEmptyNotError(t *testing.T) {
	withTempHomeForLog(t)
	entries, err := learn.ReadTeachLogWarnings()
	if err != nil {
		t.Fatalf("want nil error for missing file; got %v", err)
	}
	if entries != nil {
		t.Errorf("want nil entries for missing file; got %+v", entries)
	}
}

func TestReadTeachLogWarnings_FilterByResourceID(t *testing.T) {
	withTempHomeForLog(t)
	for _, rid := range []string{"A", "B", "C"} {
		if err := learn.AppendTeachLogWarning("teach", "q", learn.Warning{
			Code: "no_entity_overlap", Resource: rid, Detail: rid,
		}); err != nil {
			t.Fatalf("append %s: %v", rid, err)
		}
	}

	got, err := learn.ReadTeachLogWarnings("B", "C")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 entries (B+C); got %d (%+v)", len(got), got)
	}
	gotIDs := map[string]bool{got[0].Resource: true, got[1].Resource: true}
	if !gotIDs["B"] || !gotIDs["C"] {
		t.Errorf("want resources B and C; got %v", gotIDs)
	}
}

// TestReadTeachLogWarnings_SkipsLegacyPlainTextLines documents that
// the JSONL reader is defensive about the legacy plain-text format
// the CLI helper in teach.go writes for backgrounded errors.
func TestReadTeachLogWarnings_SkipsLegacyPlainTextLines(t *testing.T) {
	home := withTempHomeForLog(t)

	// Seed via the real writer first so the directory exists.
	if err := learn.AppendTeachLogWarning("teach", "q", learn.Warning{
		Code: "no_entity_overlap", Resource: "STRUCTURED",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Now append a legacy plain-text line directly.
	p := filepath.Join(home, ".local", "share", "prediction-goat-pp-cli", "teach.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open teach.log: %v", err)
	}
	if _, err := f.WriteString("2026-05-23T20:01:23Z teach: open db: connection refused\n"); err != nil {
		f.Close()
		t.Fatalf("write legacy line: %v", err)
	}
	f.Close()

	got, err := learn.ReadTeachLogWarnings()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 structured entry (legacy line skipped); got %d (%+v)", len(got), got)
	}
	if got[0].Resource != "STRUCTURED" {
		t.Errorf("want structured entry to survive; got %+v", got[0])
	}
}

// TestReadTeachLogWarnings_SkipsMalformedJSON keeps a corrupted entry
// from blocking the whole inspection surface.
func TestReadTeachLogWarnings_SkipsMalformedJSON(t *testing.T) {
	home := withTempHomeForLog(t)

	if err := learn.AppendTeachLogWarning("teach", "q", learn.Warning{
		Code: "no_entity_overlap", Resource: "GOOD",
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	p := filepath.Join(home, ".local", "share", "prediction-goat-pp-cli", "teach.log")
	f, err := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open teach.log: %v", err)
	}
	if _, err := f.WriteString("{this is not valid json\n"); err != nil {
		f.Close()
		t.Fatalf("write malformed line: %v", err)
	}
	f.Close()

	got, err := learn.ReadTeachLogWarnings()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 1 || got[0].Resource != "GOOD" {
		t.Errorf("want only the good entry; got %+v", got)
	}
}

// TestAppendTeachLogWarning_ConcurrentAppendsSurviveAllEntries is a
// coarse sanity check that O_APPEND on small lines doesn't drop
// entries when two goroutines write at once. Lines are well under
// PIPE_BUF on every supported platform.
func TestAppendTeachLogWarning_ConcurrentAppendsSurviveAllEntries(t *testing.T) {
	withTempHomeForLog(t)
	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = learn.AppendTeachLogWarning("teach", "q", learn.Warning{
				Code: "no_entity_overlap", Resource: string(rune('A' + (i % 26))), Detail: "concurrent",
			})
		}(i)
	}
	wg.Wait()
	got, err := learn.ReadTeachLogWarnings()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != n {
		t.Errorf("want %d entries, got %d", n, len(got))
	}
}
