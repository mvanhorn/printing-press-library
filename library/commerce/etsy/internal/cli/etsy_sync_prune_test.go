package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
)

// etsy-ext: OAuth and empty unions must never authorize deletion.
func TestEtsySyncPruneSafety(t *testing.T) {
	for _, oauth := range []bool{false, true} {
		c, db := listingStatesFixture(t)
		var events bytes.Buffer
		if r := listingStatesRun(c, db, nil, true, false, &events); r.Err != nil {
			t.Fatal(r.Err)
		}
		c.calls = nil
		c.respond = func(state, offset string) (json.RawMessage, error) {
			if oauth && state == "inactive" {
				return nil, &client.APIError{StatusCode: 401}
			}
			return json.RawMessage(`{"results":[],"count":0}`), nil
		}
		events.Reset()
		r := listingStatesRun(c, db, nil, true, true, &events)
		if r.Err != nil || r.Warn != nil || r.Count != 2 {
			t.Fatalf("result %+v", r)
		}
		if oauth && (len(c.calls) != 2 || !strings.Contains(events.String(), "states_skipped_no_oauth")) {
			t.Fatalf("calls %v events %s", c.calls, events.String())
		}
		if !oauth && !strings.Contains(events.String(), "incomplete_walk") {
			t.Fatal(events.String())
		}
	}
}

// etsy-ext: A short natural-end page authorizes union pruning.
func TestEtsySyncPruneUnionShortPage(t *testing.T) {
	c, db := listingStatesFixture(t)
	n := determinePaginationDefaults("listings").limit - 1
	items := make([]map[string]any, n)
	for i := range items {
		id := 1000000001 + i
		// The page must exclude the stale listing, even with a limit of 100.
		if id >= 1000000099 {
			id++
		}
		items[i] = map[string]any{"listing_id": id, "state": "active"}
	}
	page, err := json.Marshal(map[string]any{"results": items, "total_count": n})
	if err != nil {
		t.Fatal(err)
	}
	c.respond = func(state, offset string) (json.RawMessage, error) {
		if state == "active" {
			return page, nil
		}
		return json.RawMessage(`{"results":[],"count":0}`), nil
	}
	var events bytes.Buffer
	params := &syncUserParams{perResource: map[string]map[string]string{"listings": {"state": "active"}}}
	walk := listingStatesRun(c, db, params, true, true, &events)
	if walk.Err != nil || walk.Warn != nil || !walk.complete || !walk.pruneOK {
		t.Fatalf("fixture did not produce natural completion: %+v", walk)
	}
	if err := db.Upsert("listings", "1000000099", json.RawMessage(`{"listing_id":1000000099,"title":"stale"}`)); err != nil {
		t.Fatal(err)
	}
	events.Reset()
	r := listingStatesRun(c, db, nil, true, true, &events)
	if r.Err != nil || r.Warn != nil {
		t.Fatalf("result %+v", r)
	}
	if _, survives := listingStatesRows(t, db)["1000000099"]; survives {
		t.Fatal("stale listing survived")
	}
	if strings.Count(events.String(), `"event":"reconcile","resource":"listings"`) != 1 || !strings.Contains(events.String(), `"deleted":1`) {
		t.Fatal(events.String())
	}
}

// etsy-ext: A complete union deletes a stale listing exactly once.
func TestEtsySyncPruneUnion(t *testing.T) {
	c, db := listingStatesFixture(t)
	if err := db.Upsert("listings", "1000000099", json.RawMessage(`{"listing_id":1000000099,"title":"stale"}`)); err != nil {
		t.Fatal(err)
	}
	var events bytes.Buffer
	r := listingStatesRun(c, db, nil, true, true, &events)
	if r.Err != nil || r.Warn != nil || r.Count != 2 {
		t.Fatalf("result %+v", r)
	}
	if _, ok := listingStatesRows(t, db)["1000000099"]; ok {
		t.Fatal("stale listing survived")
	}
	if strings.Count(events.String(), `"event":"reconcile","resource":"listings"`) != 1 || !strings.Contains(events.String(), `"deleted":1`) {
		t.Fatal(events.String())
	}
}

// etsy-ext: A count-proven walk is complete for the checkpoint but never authorizes the union prune.
// Offset-paged Etsy resources cannot produce a count-proven end through syncResource (the page-int
// fallback always supplies a next cursor), so the walk result is constructed directly.
func TestEtsySyncUnionPruneRequiresPruneAllowed(t *testing.T) {
	natural := syncResult{Resource: "listings", complete: true, pruneOK: true}
	countProven := syncResult{Resource: "listings", complete: true, pruneOK: false}
	warned := syncResult{Resource: "listings", complete: true, pruneOK: true, Warn: errors.New("incomplete")}
	if !etsyWalkAuthorizesUnionPrune(natural) {
		t.Fatal("natural completion must authorize the union prune")
	}
	if etsyWalkAuthorizesUnionPrune(countProven) {
		t.Fatal("count-proven completion authorized the union prune")
	}
	if etsyWalkAuthorizesUnionPrune(warned) {
		t.Fatal("a warned walk authorized the union prune")
	}
	// The real walk result carries the same rule: a natural end is prune-eligible.
	c, db := listingStatesFixture(t)
	params := &syncUserParams{perResource: map[string]map[string]string{"listings": {"state": "active"}}}
	var events bytes.Buffer
	walk := listingStatesRun(c, db, params, true, false, &events)
	if walk.Err != nil || !walk.complete || !walk.pruneOK {
		t.Fatalf("natural walk %+v", walk)
	}
}
