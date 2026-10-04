// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// steam_hidden_test.go - the command-level contract for Steam apps hidden from
// anonymous store requests (age or region gate): `steam app` is a not-found, and
// bulk --with-demos leaves the demo state unknown instead of false. Offline httptest only.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil/testenv"
)

// hiddenEldenRingRecord is the verbatim IStoreBrowseService/GetItems record a
// real anonymous request returns for Elden Ring: success:15 (not OK),
// visible:false, an empty name, and appid 0 with the real id in id.
const hiddenEldenRingRecord = `{"item_type":0,"id":1245690,"success":15,"visible":false,"name":"","store_url_path":"app/0/","store_url_slug":"","appid":0}`

func TestSteamAppHiddenIsNotFound(t *testing.T) {
	demosIsolateEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != demosGetItemsPath {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, hiddenEldenRingRecord)
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	flags := &rootFlags{asJSON: true}
	cmd := newSteamAppCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"1245690"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("steam app on a hidden app must fail; stdout %s", out.String())
	}
	if got := ExitCode(err); got != 3 {
		t.Errorf("exit code = %d, want 3 (not found)", got)
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Errorf("error = %v, want it to mention hidden", err)
	}
	if strings.Contains(out.String(), `"name":""`) {
		t.Errorf("stdout must not carry a result row with an empty name: %s", out.String())
	}
}

func TestBulkWithDemosHiddenAppIsUnknown(t *testing.T) {
	testenv.Isolate(t)
	rawgLog := newDemosReqLog()
	rawgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawgLog.record(r)
		switch r.URL.Path {
		case "/games":
			fmt.Fprint(w, `{"count":2,"results":[
				{"id":101,"name":"Hidden","released":"2022-01-01","rating":4.0,"ratings_count":10},
				{"id":102,"name":"Visible","released":"2016-01-01","rating":4.0,"ratings_count":10}]}`)
		case "/games/101/stores":
			fmt.Fprint(w, `{"results":[{"store_id":1,"url":"https://store.steampowered.com/app/1245690/Elden_Ring/"}]}`)
		case "/games/102/stores":
			fmt.Fprint(w, `{"results":[{"store_id":1,"url":"https://store.steampowered.com/app/379720/DOOM/"}]}`)
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
			case 1245690:
				items = append(items, hiddenEldenRingRecord)
			case 379720:
				items = append(items, `{"appid":379720,"success":1,"visible":true,"name":"DOOM","related_items":{"demos":[{"appid":479030}]}}`)
			default:
				t.Errorf("unexpected appid %d in GetItems", id)
			}
		}
		fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(items, ","))
	}))
	defer steamSrv.Close()
	withSteamHook(t, steamSrv)

	env, _, err := runDiscoverJSON(t, "--limit", "2", "--with-demos")
	if err != nil {
		t.Fatalf("discover --with-demos: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	hidden := results[0].(map[string]any)
	for _, key := range []string{"steam_app_id", "has_demo", "demo_app_ids"} {
		if _, ok := hidden[key]; ok {
			t.Errorf("hidden row must not carry %s, got %v", key, hidden[key])
		}
	}
	visible := results[1].(map[string]any)
	if visible["has_demo"] != true {
		t.Errorf("visible row has_demo = %v, want true", visible["has_demo"])
	}
	if visible["steam_app_id"] != float64(379720) {
		t.Errorf("visible row steam_app_id = %v, want 379720", visible["steam_app_id"])
	}
}
