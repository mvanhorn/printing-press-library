package cli

import (
	"reflect"
	"testing"
)

// assertEmptyListKeys fails unless every named key is present in the marshaled
// form of v and encodes exactly as [] (never null, never absent).
func assertEmptyListKeys(t *testing.T, label string, v any, keys ...string) {
	t.Helper()
	raw := marshalToRawMap(t, v)
	for _, k := range keys {
		got, ok := raw[k]
		if !ok {
			t.Errorf("%s marshaled without a %q key: %v", label, k, keysOf(raw))
			continue
		}
		if string(got) != "[]" {
			t.Errorf("%s %s = %s, want [] (never null, never absent)", label, k, got)
		}
	}
}

// TestDigestListKeysAlwaysPresent guards the digest contract: newest and
// recently_terminated are always arrays, [] when empty. The views are built
// through buildDigest, the code the command runs.
func TestDigestListKeysAlwaysPresent(t *testing.T) {
	active := []Trial{
		{NCTID: "NCT00000001", Status: "RECRUITING", LastUpdate: "2026-01-02"},
		{NCTID: "NCT00000002", Status: "COMPLETED", LastUpdate: "2026-01-01"},
	}

	t.Run("no terminated trial", func(t *testing.T) {
		view := buildDigest("rare", active, 5)
		assertEmptyListKeys(t, "digestView", view, "recently_terminated")
		raw := marshalToRawMap(t, view)
		if string(raw["newest"]) == "[]" {
			t.Errorf("newest = [], want the two fixture trials")
		}
	})

	t.Run("no trials", func(t *testing.T) {
		view := buildDigest("rare", nil, 5)
		assertEmptyListKeys(t, "digestView", view, "newest", "recently_terminated")
	})
}

// TestWatchListKeysAlwaysPresent guards the watch contract: new, changed and
// terminated are always arrays, [] when empty, on a first run and on a rerun
// with no changes. The views are built through diffWatch.
func TestWatchListKeysAlwaysPresent(t *testing.T) {
	current := watchSnapshot{Term: "rare", Trials: map[string]watchEntry{
		"NCT00000001": {Title: "A", Status: "RECRUITING", LastUpdate: "2026-01-02"},
		"NCT00000002": {Title: "B", Status: "COMPLETED", LastUpdate: "2026-01-01"},
	}}

	t.Run("first run", func(t *testing.T) {
		view := diffWatch("rare", watchSnapshot{}, false, current)
		if !view.FirstRun {
			t.Fatalf("FirstRun = false, want true")
		}
		assertEmptyListKeys(t, "watchView", view, "new", "changed", "terminated")
	})

	t.Run("rerun with no changes", func(t *testing.T) {
		view := diffWatch("rare", current, true, current)
		if view.FirstRun {
			t.Fatalf("FirstRun = true, want false")
		}
		assertEmptyListKeys(t, "watchView", view, "new", "changed", "terminated")
	})
}

// TestDigestWatchKeepValues checks that making the keys always present left the
// values and their order alone: each trial still lands in its own list.
func TestDigestWatchKeepValues(t *testing.T) {
	t.Run("digest", func(t *testing.T) {
		trials := []Trial{
			{NCTID: "NCT00000001", Status: "RECRUITING", LastUpdate: "2026-01-03"},
			{NCTID: "NCT00000002", Status: "TERMINATED", LastUpdate: "2026-01-02"},
			{NCTID: "NCT00000003", Status: "COMPLETED", LastUpdate: "2026-01-01"},
		}
		view := buildDigest("rare", trials, 5)

		var newest, stopped []string
		for _, d := range view.Newest {
			newest = append(newest, d.NCTID)
		}
		for _, d := range view.RecentlyTerminated {
			stopped = append(stopped, d.NCTID)
		}
		if want := []string{"NCT00000001", "NCT00000002", "NCT00000003"}; !reflect.DeepEqual(newest, want) {
			t.Errorf("newest = %v, want %v", newest, want)
		}
		if want := []string{"NCT00000002"}; !reflect.DeepEqual(stopped, want) {
			t.Errorf("recently_terminated = %v, want %v", stopped, want)
		}
	})

	t.Run("watch", func(t *testing.T) {
		prev := watchSnapshot{Term: "rare", Trials: map[string]watchEntry{
			"NCT00000001": {Status: "RECRUITING", LastUpdate: "2026-01-01"},
			"NCT00000002": {Status: "RECRUITING", LastUpdate: "2026-01-01"},
			"NCT00000003": {Status: "RECRUITING", LastUpdate: "2026-01-01"},
		}}
		current := watchSnapshot{Term: "rare", Trials: map[string]watchEntry{
			"NCT00000001": {Status: "RECRUITING", LastUpdate: "2026-01-01"}, // unchanged
			"NCT00000002": {Status: "RECRUITING", LastUpdate: "2026-02-01"}, // changed
			"NCT00000003": {Status: "TERMINATED", LastUpdate: "2026-02-01"}, // newly terminated
			"NCT00000004": {Status: "RECRUITING", LastUpdate: "2026-02-01"}, // new
		}}
		view := diffWatch("rare", prev, true, current)

		ids := func(cs []watchChange) []string {
			var out []string
			for _, c := range cs {
				out = append(out, c.NCTID)
			}
			return out
		}
		if got, want := ids(view.New), []string{"NCT00000004"}; !reflect.DeepEqual(got, want) {
			t.Errorf("new = %v, want %v", got, want)
		}
		if got, want := ids(view.Changed), []string{"NCT00000002"}; !reflect.DeepEqual(got, want) {
			t.Errorf("changed = %v, want %v", got, want)
		}
		if got, want := ids(view.Terminated), []string{"NCT00000003"}; !reflect.DeepEqual(got, want) {
			t.Errorf("terminated = %v, want %v", got, want)
		}
		if len(view.Terminated) == 1 && view.Terminated[0].WasStatus != "RECRUITING" {
			t.Errorf("terminated was_status = %q, want RECRUITING", view.Terminated[0].WasStatus)
		}
	})
}
