package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

type listingStateCall struct{ state, offset, path string }
type listingStateClient struct {
	shop    string
	calls   []listingStateCall
	respond func(string, string) (json.RawMessage, error)
}

func (c *listingStateClient) RateLimit() float64 { return 100000 }
func (c *listingStateClient) CurrentTemplateVars() map[string]string {
	return map[string]string{"shop_id": c.shop}
}
func (c *listingStateClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.calls = append(c.calls, listingStateCall{params["state"], params["offset"], path})
	if strings.HasSuffix(path, "/inventory") {
		return json.RawMessage(`{"products":[]}`), nil
	}
	return c.respond(params["state"], params["offset"])
}
func listingStatesFixture(t *testing.T) (*listingStateClient, *store.Store) {
	t.Helper()
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	t.Cleanup(func() { db.Close() })
	old := humanFriendly
	humanFriendly = false
	t.Cleanup(func() { humanFriendly = old })
	return &listingStateClient{shop: "9000001", respond: listingStateResponse}, db
}
func listingStateResponse(state, offset string) (json.RawMessage, error) {
	switch state {
	case "active":
		return json.RawMessage(`{"results":[{"listing_id":1000000001,"state":"active"}],"count":1}`), nil
	case "draft":
		return json.RawMessage(`{"results":[{"listing_id":1000000002,"state":"draft"}],"count":1}`), nil
	}
	return json.RawMessage(`{"results":[],"count":0}`), nil
}
func listingStatesRun(c *listingStateClient, db *store.Store, p *syncUserParams, full, prune bool, events *bytes.Buffer) syncResult {
	return syncResource(context.Background(), c, db, "listings", "", full, 0, false, prune, p, events)
}
func listingStatesRows(t *testing.T, db *store.Store) map[string]string {
	t.Helper()
	rows := map[string]string{}
	s2Must(t, db.ListScan("listings", func(id string, raw json.RawMessage) bool {
		var v struct {
			State string `json:"state"`
		}
		s2Must(t, json.Unmarshal(raw, &v))
		rows[id] = v.State
		return true
	}))
	return rows
}
func listingStatesOrder(t *testing.T, c *listingStateClient) {
	t.Helper()
	var got []string
	for _, call := range c.calls {
		got = append(got, call.state)
	}
	if !reflect.DeepEqual(got, []string{"active", "inactive", "draft", "expired", "sold_out"}) {
		t.Fatalf("states: %v", got)
	}
}
func TestListingStatesAllAndDependents(t *testing.T) {
	c, db := listingStatesFixture(t)
	var events bytes.Buffer
	p := &syncUserParams{flatGlobal: map[string]string{}, trueGlobal: map[string]string{}, perResource: map[string]map[string]string{"listings": {}}}
	r := listingStatesRun(c, db, p, true, false, &events)
	if r.Err != nil || r.Warn != nil || r.Count != 2 {
		t.Fatalf("result: %+v", r)
	}
	listingStatesOrder(t, c)
	if len(p.perResource["listings"]) != 0 {
		t.Fatal("mutated caller")
	}
	want := map[string]string{"1000000001": "active", "1000000002": "draft"}
	if got := listingStatesRows(t, db); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows: %v", got)
	}
	c.calls = nil
	r = syncDependentResource(context.Background(), c, db, shopScopeDep(t, "listings_inventory"), "", true, 0, false, false, nil, &events, 1)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	paths := map[string]bool{}
	for _, call := range c.calls {
		paths[call.path] = true
	}
	if len(c.calls) != 2 || !paths["/listings/1000000001/inventory"] || !paths["/listings/1000000002/inventory"] {
		t.Fatalf("inventory: %v", c.calls)
	}
}
func TestListingStatesOAuthSkip(t *testing.T) {
	for _, status := range []int{401, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, db := listingStatesFixture(t)
			var events bytes.Buffer
			c.respond = func(state, offset string) (json.RawMessage, error) {
				if state == "draft" {
					return nil, &client.APIError{StatusCode: status}
				}
				return listingStateResponse(state, offset)
			}
			r := listingStatesRun(c, db, nil, true, false, &events)
			if r.Err != nil || r.Warn != nil {
				t.Fatalf("result: %+v", r)
			}
			// etsy-ext: OAuth denial stops subsequent state requests.
			if len(c.calls) != 3 || c.calls[2].state != "draft" {
				t.Fatalf("calls: %v", c.calls)
			}
			if listingStatesRows(t, db)["1000000001"] != "active" {
				t.Fatal("active missing")
			}
			if strings.Count(events.String(), `"reason":"requires_oauth_listings_r"`) != 1 || !strings.Contains(events.String(), "state=draft requires OAuth listings_r") {
				t.Fatalf("events: %s", events.String())
			}
		})
	}
}
func TestListingStatesExplicitAndPrune(t *testing.T) {
	c, db := listingStatesFixture(t)
	var events bytes.Buffer
	draft, err := parseSyncUserParams(nil, []string{"listings:state=draft"}, nil)
	s2Must(t, err)
	r := listingStatesRun(c, db, draft, true, false, &events)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if len(c.calls) != 1 || c.calls[0].state != "draft" || len(listingStatesRows(t, db)) != 1 {
		t.Fatalf("explicit: %v", c.calls)
	}
	r = listingStatesRun(c, db, nil, true, false, &events)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	active, err := parseSyncUserParams(nil, []string{"listings:state=active"}, nil)
	s2Must(t, err)
	c.respond = func(_, offset string) (json.RawMessage, error) { return listingStateResponse("active", offset) }
	for _, p := range []*syncUserParams{active, nil} {
		r = listingStatesRun(c, db, p, true, true, &events)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		n, err := db.Count("listings")
		s2Must(t, err)
		// etsy-ext: Complete all-state union now prunes absent drafts; explicit selectors preserve them.
		want := 2
		if p == nil {
			want = 1
		}
		if n != want {
			t.Fatalf("stored %d want %d", n, want)
		}
	}
}
func TestListingStatesPagination(t *testing.T) {
	c, db := listingStatesFixture(t)
	var events bytes.Buffer
	c.respond = func(state, offset string) (json.RawMessage, error) {
		if state == "active" && (offset == "" || offset == "0") {
			var rows []map[string]any
			for i := 0; i < 100; i++ {
				rows = append(rows, map[string]any{"listing_id": 1000000100 + i, "state": "active"})
			}
			raw, err := json.Marshal(map[string]any{"results": rows, "count": 101})
			return raw, err
		}
		return listingStateResponse(state, offset)
	}
	// A capped active walk leaves a shared checkpoint; the next state's walk must restart.
	r := syncResource(context.Background(), c, db, "listings", "", false, 1, false, false, nil, &events)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	c.calls = nil
	r = listingStatesRun(c, db, nil, false, false, &events)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if determinePaginationDefaults("listings").cursorParam != "offset" {
		t.Fatal("unexpected cursor")
	}
	first := map[string]bool{}
	activeNext := false
	for _, call := range c.calls {
		if !first[call.state] {
			if call.offset != "" && call.offset != "0" {
				t.Fatalf("first offset: %+v", call)
			}
			first[call.state] = true
		}
		if call.state == "active" && call.offset == "100" {
			activeNext = true
		}
	}
	if len(first) != 5 || !activeNext {
		t.Fatalf("calls: %v", c.calls)
	}
}
func TestListingStatesMissingShop(t *testing.T) {
	c, db := listingStatesFixture(t)
	c.shop = ""
	var events bytes.Buffer
	r := listingStatesRun(c, db, nil, true, false, &events)
	if r.Err != nil || r.Warn == nil || len(c.calls) != 0 || strings.Count(events.String(), `"reason":"missing_shop_id"`) != 1 {
		t.Fatalf("result %+v calls %v events %s", r, c.calls, events.String())
	}
}
