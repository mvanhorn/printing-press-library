// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Regression tests for fix F1: the --agent/--compact projection must keep the
// sparse Steam demo fields that --with-demos adds to discover/similar rows.
// Every case runs against httptest servers; nothing here touches the network.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil/testenv"
)

// runDiscoverCompact executes discover with the same flags the real --agent
// path sets (asJSON + agent imply compact) and returns the decoded envelope.
func runDiscoverCompact(t *testing.T, args ...string) (map[string]any, string) {
	t.Helper()
	flags := &rootFlags{asJSON: true, agent: true, compact: true, noCache: true}
	cmd := newDiscoverCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("discover compact: %v stderr=%s", err, errOut.String())
	}
	var env map[string]any
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		t.Fatalf("stdout is not JSON (%v): %q", uerr, out.String())
	}
	return env, out.String()
}

// TestBulkWithDemosSurvivesCompact: five RAWG rows, two with a Steam store
// link, exactly one of those with a demo. Under --agent compaction every
// Steam-linked row must keep steam_app_id and has_demo, and the demo row must
// keep demo_app_ids. The 80% frequency rule would otherwise drop all three
// (present on at most 2 of 5 rows).
func TestBulkWithDemosSurvivesCompact(t *testing.T) {
	testenv.Isolate(t)
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/games":
			fmt.Fprint(w, `{"count":5,"results":[
				{"id":101,"name":"Alpha","released":"2020-01-01","rating":4.0,"ratings_count":120},
				{"id":102,"name":"Beta","released":"2021-01-01","rating":3.5,"ratings_count":80},
				{"id":103,"name":"Gamma","released":"2022-01-01","rating":3.0,"ratings_count":40},
				{"id":104,"name":"Delta","released":"2019-01-01","rating":2.5,"ratings_count":30},
				{"id":105,"name":"Epsilon","released":"2018-01-01","rating":2.0,"ratings_count":20}]}`)
		case "/games/101/stores":
			fmt.Fprint(w, `{"results":[{"store_id":1,"url":"https://store.steampowered.com/app/501/Alpha/"}]}`)
		case "/games/102/stores":
			fmt.Fprint(w, `{"results":[{"store_id":1,"url":"https://store.steampowered.com/app/502/Beta/"}]}`)
		default:
			// Non-Steam games resolve to an empty store list.
			fmt.Fprint(w, `{"results":[]}`)
		}
	}))
	defer rawgSrv.Close()
	withRAWGBaseURL(t, rawgSrv)

	steamLog := newDemosReqLog()
	steamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := steamLog.record(r)
		if r.URL.Path != demosGetItemsPath {
			t.Errorf("unexpected Steam path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		ids := getItemsIDs(t, p)
		items := make([]string, 0, len(ids))
		for _, id := range ids {
			switch id {
			case 501:
				items = append(items, `{"appid":501,"success":1,"visible":true,"name":"Alpha","related_items":{"demos":[{"appid":777,"description":""}]}}`)
			case 502:
				items = append(items, `{"appid":502,"success":1,"visible":true,"name":"Beta"}`)
			default:
				t.Errorf("unexpected appid %d in GetItems", id)
			}
		}
		fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(items, ","))
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, _ := runDiscoverCompact(t, "--limit", "5", "--with-demos")
	results, _ := env["results"].([]any)
	if len(results) != 5 {
		t.Fatalf("results = %d, want 5", len(results))
	}
	byID := map[float64]map[string]any{}
	for _, r := range results {
		row := r.(map[string]any)
		byID[row["id"].(float64)] = row
	}

	demoRow := byID[101]
	if demoRow["steam_app_id"] != float64(501) {
		t.Errorf("demo row steam_app_id = %v, want 501 (dropped by compaction?)", demoRow["steam_app_id"])
	}
	if demoRow["has_demo"] != true {
		t.Errorf("demo row has_demo = %v, want true (dropped by compaction?)", demoRow["has_demo"])
	}
	if !reflect.DeepEqual(demoRow["demo_app_ids"], []any{float64(777)}) {
		t.Errorf("demo row demo_app_ids = %v, want [777] (dropped by compaction?)", demoRow["demo_app_ids"])
	}

	noDemoRow := byID[102]
	if noDemoRow["steam_app_id"] != float64(502) {
		t.Errorf("second Steam row steam_app_id = %v, want 502 (dropped by compaction?)", noDemoRow["steam_app_id"])
	}
	if noDemoRow["has_demo"] != false {
		t.Errorf("second Steam row has_demo = %v, want false (dropped by compaction?)", noDemoRow["has_demo"])
	}

	for _, id := range []float64{103, 104, 105} {
		for _, key := range []string{"has_demo", "steam_app_id", "demo_app_ids"} {
			if _, ok := byID[id][key]; ok {
				t.Errorf("non-Steam results[%v] must not carry %s, got %v", id, key, byID[id][key])
			}
		}
	}
}

// TestSimilarRowsKeepDemoFieldsUnderCompact exercises the exact print helper
// similar calls (printJSONFiltered) with similarResult rows marshalled the same
// way. One of three rows is Steam-linked with a demo, so the 80% frequency
// rule would drop has_demo, steam_app_id, and demo_app_ids on that row.
func TestSimilarRowsKeepDemoFieldsUnderCompact(t *testing.T) {
	testenv.Isolate(t)
	yes := true
	view := similarView{
		Meta: similarMeta{Source: "live", DataOrigin: "tiered-join", Seed: "Portal 2", Count: 3, Limit: 5},
		Results: []similarResult{
			{ID: 1, Name: "A", Tier: "studio", SteamAppID: 501, HasDemo: &yes, DemoAppIDs: []int64{777}},
			{ID: 2, Name: "B", Tier: "mechanics"},
			{ID: 3, Name: "C", Tier: "genre"},
		},
	}
	flags := &rootFlags{asJSON: true, agent: true, compact: true, noCache: true}
	var out bytes.Buffer
	if err := printJSONFiltered(&out, view, flags); err != nil {
		t.Fatalf("similar print: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not JSON (%v): %q", err, out.String())
	}
	results, _ := env["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	first := results[0].(map[string]any)
	if first["steam_app_id"] != float64(501) {
		t.Errorf("results[0].steam_app_id = %v, want 501 (dropped by compaction?)", first["steam_app_id"])
	}
	if first["has_demo"] != true {
		t.Errorf("results[0].has_demo = %v, want true (dropped by compaction?)", first["has_demo"])
	}
	if !reflect.DeepEqual(first["demo_app_ids"], []any{float64(777)}) {
		t.Errorf("results[0].demo_app_ids = %v, want [777] (dropped by compaction?)", first["demo_app_ids"])
	}
}

// TestDiscoverDefaultCompactUnchanged guards the default path: without
// --with-demos the compaction keep-set extension must be a no-op. Approach:
// the compacted --agent output for the hermetic three-game fixture is compared
// byte-for-byte against a golden captured from HEAD 1a45fcf3d's behaviour
// (same fixture, same --limit 3, compaction keep-set extension absent).
func TestDiscoverDefaultCompactUnchanged(t *testing.T) {
	testenv.Isolate(t)
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/games" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, discoverGamesResponse())
	}))
	defer rawgSrv.Close()
	withRAWGBaseURL(t, rawgSrv)

	_, got := runDiscoverCompact(t, "--limit", "3")
	if got != discoverDefaultCompactGolden {
		t.Errorf("default discover --agent output changed:\n got: %q\nwant: %q", got, discoverDefaultCompactGolden)
	}
}

// discoverDefaultCompactGolden is the byte-for-byte --agent/--compact output of
// the three-game hermetic fixture captured at HEAD 1a45fcf3d, before the
// compaction keep-set extension existed.
const discoverDefaultCompactGolden = `{
  "meta": {
    "count": 3,
    "source": "live"
  },
  "results": [
    {
      "added": 0,
      "genres": [],
      "id": 101,
      "metacritic": null,
      "name": "Alpha",
      "platforms": [],
      "playtime": 0,
      "rating": 4,
      "ratings_count": 120,
      "released": "2020-01-01",
      "slug": ""
    },
    {
      "added": 0,
      "genres": [],
      "id": 102,
      "metacritic": null,
      "name": "Beta",
      "platforms": [],
      "playtime": 0,
      "rating": 3.5,
      "ratings_count": 80,
      "released": "2021-01-01",
      "slug": ""
    },
    {
      "added": 0,
      "genres": [],
      "id": 103,
      "metacritic": null,
      "name": "Gamma",
      "platforms": [],
      "playtime": 0,
      "rating": 3,
      "ratings_count": 40,
      "released": "2022-01-01",
      "slug": ""
    }
  ]
}
`
