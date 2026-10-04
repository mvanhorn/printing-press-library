// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hermetic tests for the opt-in Steam demo annotation on discover/similar and
// the has_demo field on the ratings card. Every case runs against httptest
// servers; nothing here touches the network.

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
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"
)

// withRAWGBaseURL points the CLI RAWG client at srv and gives it a throwaway
// key so config.Load does not treat the request as unauthenticated.
func withRAWGBaseURL(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("GAME_GOAT_BASE_URL", srv.URL)
	t.Setenv("GAME_GOAT_CONFIG", "")
	t.Setenv("RAWG_API_KEY", "test-key")
}

// rawgRequestPaths returns the recorded request paths of a demo request log.
func rawgRequestPaths(log *demosReqLog) []string {
	log.mu.Lock()
	defer log.mu.Unlock()
	paths := make([]string, 0, len(log.counts))
	for path, n := range log.counts {
		for i := 0; i < n; i++ {
			paths = append(paths, path)
		}
	}
	return paths
}

// countStoresRequests sums RAWG /games/{id}/stores requests.
func countStoresRequests(log *demosReqLog) int {
	n := 0
	for _, path := range rawgRequestPaths(log) {
		if strings.HasSuffix(path, "/stores") {
			n++
		}
	}
	return n
}

func runDiscoverJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()
	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := newDiscoverCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if out.Len() == 0 {
		return nil, errOut.String(), err
	}
	var env map[string]any
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		return nil, errOut.String(), fmt.Errorf("stdout is not JSON (%v): %q", uerr, out.String())
	}
	return env, errOut.String(), err
}

func discoverGamesResponse() string {
	return "{\"count\":3,\"results\":[" +
		"{\"id\":101,\"name\":\"Alpha\",\"released\":\"2020-01-01\",\"rating\":4.0,\"ratings_count\":120}," +
		"{\"id\":102,\"name\":\"Beta\",\"released\":\"2021-01-01\",\"rating\":3.5,\"ratings_count\":80}," +
		"{\"id\":103,\"name\":\"Gamma\",\"released\":\"2022-01-01\",\"rating\":3.0,\"ratings_count\":40}]}"
}

func TestRatingsCardHasDemo(t *testing.T) {
	yes, no := true, false
	game := rawgGame{ID: 379720, Name: "DOOM", Released: "2016-05-13"}

	cardJSON := func(t *testing.T, card ratingsCard) map[string]any {
		t.Helper()
		raw, err := json.Marshal(card)
		if err != nil {
			t.Fatalf("marshal ratings card: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal ratings card: %v", err)
		}
		return m
	}

	t.Run("known demo copies appids", func(t *testing.T) {
		card := buildRatingsCard(game, &steam.SteamReview{HasDemo: &yes, DemoAppIDs: []int64{479030}})
		if card.HasDemo == nil || !*card.HasDemo {
			t.Fatalf("HasDemo = %v, want true", card.HasDemo)
		}
		if !reflect.DeepEqual(card.DemoAppIDs, []int64{479030}) {
			t.Fatalf("DemoAppIDs = %v, want [479030]", card.DemoAppIDs)
		}
		m := cardJSON(t, card)
		if m["has_demo"] != true {
			t.Errorf("has_demo = %v, want true", m["has_demo"])
		}
		if !reflect.DeepEqual(m["demo_app_ids"], []any{float64(479030)}) {
			t.Errorf("demo_app_ids = %v, want [479030]", m["demo_app_ids"])
		}
	})

	t.Run("known no-demo is present and false", func(t *testing.T) {
		card := buildRatingsCard(game, &steam.SteamReview{HasDemo: &no})
		if card.HasDemo == nil || *card.HasDemo {
			t.Fatalf("HasDemo = %v, want false (known)", card.HasDemo)
		}
		m := cardJSON(t, card)
		if m["has_demo"] != false {
			t.Errorf("has_demo = %v, want false", m["has_demo"])
		}
		if _, ok := m["demo_app_ids"]; ok {
			t.Errorf("demo_app_ids must be absent when there are no demos, got %v", m["demo_app_ids"])
		}
	})

	t.Run("nil review omits the keys", func(t *testing.T) {
		card := buildRatingsCard(game, nil)
		m := cardJSON(t, card)
		if _, ok := m["has_demo"]; ok {
			t.Errorf("has_demo must be absent for a nil review, got %v", m["has_demo"])
		}
		if _, ok := m["demo_app_ids"]; ok {
			t.Errorf("demo_app_ids must be absent for a nil review, got %v", m["demo_app_ids"])
		}
	})

	t.Run("nil HasDemo omits the key", func(t *testing.T) {
		card := buildRatingsCard(game, &steam.SteamReview{})
		m := cardJSON(t, card)
		if _, ok := m["has_demo"]; ok {
			t.Errorf("has_demo must be absent when appdetails did not succeed, got %v", m["has_demo"])
		}
	})
}

func TestBulkDefaultHasNoDemoLookup(t *testing.T) {
	testenv.Isolate(t)
	rawgLog := newDemosReqLog()
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawgLog.record(r)
		if r.URL.Path != "/games" {
			t.Errorf("unexpected RAWG path %s (no --with-demos means no /stores lookup)", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, discoverGamesResponse())
	}))
	defer rawgSrv.Close()
	withRAWGBaseURL(t, rawgSrv)

	steamLog := newDemosReqLog()
	steamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		steamLog.record(r)
		t.Errorf("unexpected Steam request %s (no --with-demos means no Steam call)", r.URL.Path)
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, _, err := runDiscoverJSON(t, "--limit", "3")
	if err != nil {
		t.Fatalf("discover without --with-demos: %v", err)
	}
	if got := rawgLog.count("/games"); got != 1 {
		t.Errorf("RAWG /games requests = %d, want 1", got)
	}
	if got := countStoresRequests(rawgLog); got != 0 {
		t.Errorf("RAWG /stores requests = %d, want 0", got)
	}
	if got := steamLog.total(); got != 0 {
		t.Errorf("Steam requests = %d, want 0", got)
	}
	results, _ := env["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for i, r := range results {
		row := r.(map[string]any)
		for _, key := range []string{"has_demo", "steam_app_id", "demo_app_ids"} {
			if _, ok := row[key]; ok {
				t.Errorf("results[%d] must not carry %s without --with-demos, got %v", i, key, row[key])
			}
		}
	}
}

func TestBulkWithDemosBatchesLookups(t *testing.T) {
	testenv.Isolate(t)
	rawgLog := newDemosReqLog()
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawgLog.record(r)
		switch r.URL.Path {
		case "/games":
			fmt.Fprint(w, discoverGamesResponse())
		case "/games/101/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":1,\"url\":\"https://store.steampowered.com/app/501/Alpha/\"}]}")
		case "/games/102/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":1,\"url\":\"https://store.steampowered.com/app/502/Beta/\"}]}")
		case "/games/103/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":5,\"url\":\"https://www.gog.com/game/gamma\"}]}")
		default:
			t.Errorf("unexpected RAWG path %s", r.URL.Path)
			http.NotFound(w, r)
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
				items = append(items, "{\"appid\":501,\"success\":1,\"visible\":true,\"name\":\"Alpha\",\"related_items\":{\"demos\":[{\"appid\":777,\"description\":\"\"}]}}")
			case 502:
				items = append(items, "{\"appid\":502,\"success\":1,\"visible\":true,\"name\":\"Beta\"}")
			default:
				t.Errorf("unexpected appid %d in GetItems", id)
			}
		}
		fmt.Fprintf(w, "{\"response\":{\"store_items\":[%s]}}", strings.Join(items, ","))
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, _, err := runDiscoverJSON(t, "--limit", "3", "--with-demos")
	if err != nil {
		t.Fatalf("discover --with-demos: %v", err)
	}
	if got := rawgLog.count("/games"); got != 1 {
		t.Errorf("RAWG /games requests = %d, want 1", got)
	}
	if got := countStoresRequests(rawgLog); got != 3 {
		t.Errorf("RAWG /stores requests = %d, want 3 (one per result)", got)
	}
	if got := steamLog.count(demosGetItemsPath); got != 1 {
		t.Errorf("Steam GetItems requests = %d, want 1 (batched)", got)
	}
	if got := getItemsIDs(t, steamLog.last(t, demosGetItemsPath)); !reflect.DeepEqual(got, []int64{501, 502}) {
		t.Errorf("GetItems ids = %v, want [501 502] (the two Steam results)", got)
	}

	results, _ := env["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	first := results[0].(map[string]any)
	if first["steam_app_id"] != float64(501) {
		t.Errorf("results[0].steam_app_id = %v, want 501", first["steam_app_id"])
	}
	if first["has_demo"] != true {
		t.Errorf("results[0].has_demo = %v, want true", first["has_demo"])
	}
	if !reflect.DeepEqual(first["demo_app_ids"], []any{float64(777)}) {
		t.Errorf("results[0].demo_app_ids = %v, want [777]", first["demo_app_ids"])
	}

	second := results[1].(map[string]any)
	if second["steam_app_id"] != float64(502) {
		t.Errorf("results[1].steam_app_id = %v, want 502", second["steam_app_id"])
	}
	if second["has_demo"] != false {
		t.Errorf("results[1].has_demo = %v, want false", second["has_demo"])
	}

	third := results[2].(map[string]any)
	for _, key := range []string{"has_demo", "steam_app_id", "demo_app_ids"} {
		if _, ok := third[key]; ok {
			t.Errorf("results[2] is a non-Steam game and must not carry %s, got %v", key, third[key])
		}
	}
}

func TestSimilarWithDemosFlagWires(t *testing.T) {
	cmd := newSimilarCmd(&rootFlags{})
	flag := cmd.Flags().Lookup("with-demos")
	if flag == nil {
		t.Fatal("similar is missing the --with-demos flag")
	}
	if flag.DefValue != "false" {
		t.Errorf("--with-demos default = %q, want false", flag.DefValue)
	}
	for _, want := range []string{"has_demo", "demo_app_ids", "RAWG request per result", "Steam request per 200"} {
		if !strings.Contains(flag.Usage, want) {
			t.Errorf("--with-demos help %q must mention %q", flag.Usage, want)
		}
	}
}

// TestAnnotateCountsFailedStoreLookups: a failed RAWG /stores lookup is counted
// and warned about once, while a game whose store list has no Steam link is not
// a failure. Rows for failed lookups keep the demo fields absent.
func TestAnnotateCountsFailedStoreLookups(t *testing.T) {
	testenv.Isolate(t)
	rawgLog := newDemosReqLog()
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawgLog.record(r)
		switch r.URL.Path {
		case "/games":
			fmt.Fprint(w, discoverGamesResponse())
		case "/games/101/stores":
			// Malformed JSON (status 200) is a failed lookup without a 5xx retry.
			fmt.Fprint(w, "{\"results\":")
		case "/games/102/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":5,\"url\":\"https://www.gog.com/game/beta\"}]}")
		case "/games/103/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":1,\"url\":\"https://store.steampowered.com/app/503/Gamma/\"}]}")
		default:
			t.Errorf("unexpected RAWG path %s", r.URL.Path)
			http.NotFound(w, r)
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
		for range ids {
			items = append(items, "{\"appid\":503,\"success\":1,\"visible\":true,\"name\":\"Gamma\",\"related_items\":{\"demos\":[{\"appid\":999}]}}")
		}
		fmt.Fprintf(w, "{\"response\":{\"store_items\":[%s]}}", strings.Join(items, ","))
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, errOut, err := runDiscoverJSON(t, "--limit", "3", "--with-demos")
	if err != nil {
		t.Fatalf("discover --with-demos: %v", err)
	}
	if got := strings.Count(errOut, "warning: steam demo annotation:"); got != 1 {
		t.Errorf("stderr warning count = %d, want exactly 1; stderr=%q", got, errOut)
	}
	if !strings.Contains(errOut, "1 of 3") {
		t.Errorf("stderr = %q, want to mention 1 of 3", errOut)
	}
	results, _ := env["results"].([]any)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	for _, i := range []int{0, 1} {
		row := results[i].(map[string]any)
		if _, ok := row["has_demo"]; ok {
			t.Errorf("results[%d].has_demo = %v, want absent (lookup failed)", i, row["has_demo"])
		}
	}
	third := results[2].(map[string]any)
	if third["has_demo"] != true {
		t.Errorf("results[2].has_demo = %v, want true", third["has_demo"])
	}
}

// TestAnnotateNoLinkIsNotAFailure: games that list only non-Steam stores are
// simply not annotated; that is not a failed lookup and produces no warning.
func TestAnnotateNoLinkIsNotAFailure(t *testing.T) {
	testenv.Isolate(t)
	twoGames := "{\"count\":2,\"results\":[" +
		"{\"id\":101,\"name\":\"Alpha\",\"released\":\"2020-01-01\",\"rating\":4.0,\"ratings_count\":120}," +
		"{\"id\":102,\"name\":\"Beta\",\"released\":\"2021-01-01\",\"rating\":3.5,\"ratings_count\":80}]}"
	rawgLog := newDemosReqLog()
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawgLog.record(r)
		switch r.URL.Path {
		case "/games":
			fmt.Fprint(w, twoGames)
		case "/games/101/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":5,\"url\":\"https://www.gog.com/game/alpha\"}]}")
		case "/games/102/stores":
			fmt.Fprint(w, "{\"results\":[{\"store_id\":3,\"url\":\"https://store.playstation.com/game/beta\"}]}")
		default:
			t.Errorf("unexpected RAWG path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer rawgSrv.Close()
	withRAWGBaseURL(t, rawgSrv)

	steamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected Steam request %s (no Steam links means no GetItems)", r.URL.Path)
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, errOut, err := runDiscoverJSON(t, "--limit", "2", "--with-demos")
	if err != nil {
		t.Fatalf("discover --with-demos: %v", err)
	}
	if strings.Contains(errOut, "steam demo annotation") {
		t.Errorf("stderr = %q, want no demo-annotation warning for no-link games", errOut)
	}
	results, _ := env["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for i, r := range results {
		row := r.(map[string]any)
		if _, ok := row["has_demo"]; ok {
			t.Errorf("results[%d].has_demo = %v, want absent for a no-link game", i, row["has_demo"])
		}
	}
}
