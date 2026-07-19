// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fixturePages serves numbered items out of a fixed-size archive, capping each
// page at serverCap regardless of the requested n — except the first page,
// which returns firstPageCap items. This mirrors the live behaviour observed on
// dougshapiro.substack.com (2026-07-19): 23 items for a 50-item request at
// offset 0, then 25-item pages deeper in, even though ~70 older posts existed.
func fixturePages(total, serverCap, firstPageCap int, offsets *[]int) func(n, offset int) ([]json.RawMessage, error) {
	return func(n, offset int) ([]json.RawMessage, error) {
		*offsets = append(*offsets, offset)
		if offset >= total {
			return nil, nil
		}
		pageCap := serverCap
		if offset == 0 {
			pageCap = firstPageCap
		}
		if n < pageCap {
			pageCap = n
		}
		end := offset + pageCap
		if end > total {
			end = total
		}
		items := make([]json.RawMessage, 0, end-offset)
		for i := offset; i < end; i++ {
			items = append(items, json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)))
		}
		return items, nil
	}
}

func storeAll(json.RawMessage) (bool, error) { return true, nil }

// TestArchiveWalkShortPageDoesNotEndWalk is the regression test for the
// premature-stop defect: a page shorter than requested must NOT terminate the
// walk (only an empty page does), and the offset must advance by the number of
// items actually returned so short pages never skip the posts behind them.
func TestArchiveWalkShortPageDoesNotEndWalk(t *testing.T) {
	var offsets []int
	archived, skipped, err := archiveWalk(120, 50, fixturePages(96, 25, 23, &offsets), storeAll)
	if err != nil {
		t.Fatalf("archiveWalk error = %v", err)
	}
	if archived != 96 || skipped != 0 {
		t.Fatalf("archived = %d, skipped = %d; want 96 archived (the whole fixture), 0 skipped", archived, skipped)
	}
	want := []int{0, 23, 48, 73, 96}
	if fmt.Sprint(offsets) != fmt.Sprint(want) {
		t.Fatalf("fetch offsets = %v, want %v (advance by items returned, not by requested page size)", offsets, want)
	}
}

// TestArchiveWalkRespectsLimit: --limit is a true cap even when the archive
// holds more posts.
func TestArchiveWalkRespectsLimit(t *testing.T) {
	var offsets []int
	archived, _, err := archiveWalk(30, 50, fixturePages(96, 25, 25, &offsets), storeAll)
	if err != nil {
		t.Fatalf("archiveWalk error = %v", err)
	}
	if archived != 30 {
		t.Fatalf("archived = %d, want exactly the 30-post limit", archived)
	}
	if len(offsets) != 2 {
		t.Fatalf("fetch calls = %d (%v), want 2 — the walk must stop at the limit", len(offsets), offsets)
	}
}

// TestArchiveWalkSkippedItemsDoNotCountTowardLimit: items the store callback
// rejects (missing id) are skipped without consuming the limit budget.
func TestArchiveWalkSkippedItemsDoNotCountTowardLimit(t *testing.T) {
	var offsets []int
	storeOdd := func(raw json.RawMessage) (bool, error) {
		var m struct {
			I int `json:"i"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return false, err
		}
		return m.I%2 == 1, nil
	}
	archived, skipped, err := archiveWalk(3, 50, fixturePages(6, 25, 25, &offsets), storeOdd)
	if err != nil {
		t.Fatalf("archiveWalk error = %v", err)
	}
	if archived != 3 || skipped != 3 {
		t.Fatalf("archived = %d, skipped = %d; want 3 and 3", archived, skipped)
	}
}

// TestArchiveWalkEmptyArchive: an empty first page terminates immediately.
func TestArchiveWalkEmptyArchive(t *testing.T) {
	var offsets []int
	archived, skipped, err := archiveWalk(50, 50, fixturePages(0, 25, 25, &offsets), storeAll)
	if err != nil {
		t.Fatalf("archiveWalk error = %v", err)
	}
	if archived != 0 || skipped != 0 || len(offsets) != 1 {
		t.Fatalf("archived = %d, skipped = %d, fetches = %d; want 0, 0, 1", archived, skipped, len(offsets))
	}
}

// TestArchiveWalkPropagatesFetchError: a fetch failure surfaces with the
// partial progress made so far.
func TestArchiveWalkPropagatesFetchError(t *testing.T) {
	calls := 0
	fetch := func(n, offset int) ([]json.RawMessage, error) {
		calls++
		if calls == 1 {
			items := make([]json.RawMessage, 10)
			for i := range items {
				items[i] = json.RawMessage(`{}`)
			}
			return items, nil
		}
		return nil, fmt.Errorf("boom")
	}
	archived, _, err := archiveWalk(50, 50, fetch, storeAll)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the fetch error propagated", err)
	}
	if archived != 10 {
		t.Fatalf("archived = %d, want the 10 posts stored before the failure", archived)
	}
}

// TestNovelArchiveHelpWires smoke-tests that the archive command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelArchiveHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"archive", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("archive --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "archive"} {
		if !strings.Contains(help, want) {
			t.Fatalf("archive --help missing %q in output:\n%s", want, help)
		}
	}
}
