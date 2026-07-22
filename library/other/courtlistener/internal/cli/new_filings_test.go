// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/courtlistener/internal/store"
)

// TestNovelNewFilingsHelpWires smoke-tests that the new-filings command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNewFilingsHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"new-filings", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("new-filings --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "new-filings"} {
		if !strings.Contains(help, want) {
			t.Fatalf("new-filings --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestUpdateFilingWatchConcurrentProcessesPreserveBothObservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "watch.db")
	first, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	stores := []*store.Store{first, second}
	observations := []map[string]bool{{"id:90": true}, {"id:100": true}}
	errCh := make(chan error, len(stores))
	var wg sync.WaitGroup
	for i := range stores {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var added []string
			var baseline bool
			errCh <- updateFilingWatch(context.Background(), stores[i], "r|concurrent", observations[i], time.Now().UTC(), &added, &baseline)
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	raw, err := first.Get("courtlistener-search-watch", "r|concurrent")
	if err != nil {
		t.Fatal(err)
	}
	var state filingWatchState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"id:90", "id:100"} {
		if state.Seen[id] == "" {
			t.Fatalf("concurrent observation %q was lost: %#v", id, state.Seen)
		}
	}
}

func TestMergeSeenFilingsRetainsHistoryBeyondCurrentWindow(t *testing.T) {
	previous := map[string]string{"old": "2026-01-01T00:00:00Z"}
	added, next := mergeSeenFilings(previous, map[string]bool{"new": true}, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC), 5000)
	if len(added) != 1 || added[0] != "new" || next["old"] == "" {
		t.Fatalf("added=%v next=%v", added, next)
	}
	added, _ = mergeSeenFilings(next, map[string]bool{"old": true}, time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC), 5000)
	if len(added) != 0 {
		t.Fatalf("previously seen filing was reported again: %v", added)
	}
}
