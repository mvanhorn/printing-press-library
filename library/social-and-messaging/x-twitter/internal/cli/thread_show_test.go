// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

// mkSyncedTweet builds a syncedTweet from JSON so the anonymous
// referenced_tweets struct doesn't have to be spelled out in each case.
func mkSyncedTweet(t *testing.T, jsonStr string) syncedTweet {
	t.Helper()
	var st syncedTweet
	if err := json.Unmarshal([]byte(jsonStr), &st); err != nil {
		t.Fatalf("unmarshal %s: %v", jsonStr, err)
	}
	return st
}

func threadOrder(thread []threadPost) []string {
	ids := make([]string, len(thread))
	for i, p := range thread {
		ids[i] = p.ID
	}
	return ids
}

func depthByID(thread []threadPost) map[string]int {
	m := make(map[string]int, len(thread))
	for _, p := range thread {
		m[p.ID] = p.Depth
	}
	return m
}

func TestReconstructThreadLinearDepthAndOrder(t *testing.T) {
	// root <- reply1 <- reply2, supplied out of chronological order.
	byID := map[string]syncedTweet{
		"3": mkSyncedTweet(t, `{"id":"3","created_at":"2026-01-01T00:02:00Z","referenced_tweets":[{"type":"replied_to","id":"2"}]}`),
		"1": mkSyncedTweet(t, `{"id":"1","created_at":"2026-01-01T00:00:00Z"}`),
		"2": mkSyncedTweet(t, `{"id":"2","created_at":"2026-01-01T00:01:00Z","referenced_tweets":[{"type":"replied_to","id":"1"}]}`),
	}
	thread := reconstructThread(byID)

	if got := threadOrder(thread); !equalStrings(got, []string{"1", "2", "3"}) {
		t.Errorf("chronological order = %v, want [1 2 3]", got)
	}
	depths := depthByID(thread)
	for id, want := range map[string]int{"1": 0, "2": 1, "3": 2} {
		if depths[id] != want {
			t.Errorf("depth[%s] = %d, want %d", id, depths[id], want)
		}
	}
	for _, p := range thread {
		if p.ID == "2" && p.InReplyTo != "1" {
			t.Errorf("post 2 InReplyTo = %q, want 1", p.InReplyTo)
		}
	}
}

func TestReconstructThreadParentOutsideSet(t *testing.T) {
	// Reply whose replied_to parent was never synced terminates at depth 0.
	byID := map[string]syncedTweet{
		"2": mkSyncedTweet(t, `{"id":"2","created_at":"2026-01-01T00:01:00Z","referenced_tweets":[{"type":"replied_to","id":"99"}]}`),
	}
	thread := reconstructThread(byID)
	if len(thread) != 1 || thread[0].Depth != 0 {
		t.Fatalf("parent-outside-set: got %+v, want one post at depth 0", thread)
	}
	if thread[0].InReplyTo != "99" {
		t.Errorf("InReplyTo = %q, want 99 (edge preserved even when parent unsynced)", thread[0].InReplyTo)
	}
}

func TestReconstructThreadCycleGuard(t *testing.T) {
	// Malformed replied_to cycle (1 -> 2 -> 1) must terminate, not recurse forever.
	byID := map[string]syncedTweet{
		"1": mkSyncedTweet(t, `{"id":"1","created_at":"2026-01-01T00:00:00Z","referenced_tweets":[{"type":"replied_to","id":"2"}]}`),
		"2": mkSyncedTweet(t, `{"id":"2","created_at":"2026-01-01T00:01:00Z","referenced_tweets":[{"type":"replied_to","id":"1"}]}`),
	}
	thread := reconstructThread(byID) // would hang/overflow without the cycle guard
	if len(thread) != 2 {
		t.Fatalf("cycle: got %d posts, want 2", len(thread))
	}
	for _, p := range thread {
		if p.Depth < 0 {
			t.Errorf("cycle produced negative depth for %s", p.ID)
		}
	}
}

func TestReconstructThreadSortTiebreakers(t *testing.T) {
	// Same created_at -> id breaks the tie; missing created_at sorts first.
	byID := map[string]syncedTweet{
		"b": mkSyncedTweet(t, `{"id":"b","created_at":"2026-01-01T00:00:00Z"}`),
		"a": mkSyncedTweet(t, `{"id":"a","created_at":"2026-01-01T00:00:00Z"}`),
		"z": mkSyncedTweet(t, `{"id":"z"}`),
	}
	if got := threadOrder(reconstructThread(byID)); !equalStrings(got, []string{"z", "a", "b"}) {
		t.Errorf("sort order = %v, want [z a b] (no-created_at first, then id)", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
