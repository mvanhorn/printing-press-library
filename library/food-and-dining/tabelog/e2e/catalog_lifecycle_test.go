package e2e

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestTypedCatalogChoicesAndAmbiguityDoNotBroadenDiscovery(t *testing.T) {
	home := readFixture(t, "english-home.html")
	ginza, shinjuku, bar := readFixture(t, "suggest-ginza.json"), readFixture(t, "suggest-shinjuku.json"), readFixture(t, "suggest-bar.json")
	station := readFixture(t, "station-shinjuku.html")
	r := newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/" {
			return response{body: home}
		}
		if req.URL.Path == "/en/suggest/keyword_suggest" {
			switch strings.ToLower(req.URL.Query().Get("keyword")) {
			case "ginza":
				return response{contentType: "application/json", body: ginza}
			case "shinjuku":
				return response{contentType: "application/json", body: shinjuku}
			case "bar":
				return response{contentType: "application/json", body: bar}
			}
		}
		if strings.Contains(req.URL.Path, "rstLst") {
			return response{body: station}
		}
		return response{status: 404}
	})
	w := newWorkspace(t, r)
	areas := mustSucceed(t, w.run(t, "areas", "Ginza", "--kind", "station", "--agent"))
	var selected map[string]any
	for _, choice := range items(t, areas) {
		if choice["station_id"] == "3368" {
			selected = choice
		}
		if choice["kind"] == "restaurant" {
			t.Fatal("area catalog admitted a restaurant-name suggestion")
		}
	}
	if selected == nil {
		t.Fatal("verified Ginza3368 station choice missing")
	}
	equal(t, selected["name"], "Ginza Sta.（Tokyo）")
	equal(t, selected["area1"], "A1301")
	equal(t, selected["area2"], "A130101")
	if selected["selector"] == nil || selected["selector"] == "" {
		t.Fatal("typed choice cannot be reused")
	}
	before := r.count()
	mustSucceed(t, w.run(t, "areas", "Ginza", "--kind", "station", "--data-source", "local", "--agent"))
	equal(t, r.count(), before)
	cuisines := mustSucceed(t, w.run(t, "cuisines", "Bar", "--agent"))
	encoded := mustJSON(cuisines)
	for _, value := range []string{"bar", "BC01"} {
		if !bytes.Contains(encoded, []byte(value)) {
			t.Fatalf("broad/child source taxonomy %s missing", value)
		}
	}
	before = r.count()
	mustSucceed(t, w.run(t, "cuisines", "Bar", "--data-source", "local", "--agent"))
	equal(t, r.count(), before)
	ambiguous := w.run(t, "find", "--area", "Shinjuku", "--agent")
	mustFail(t, ambiguous)
	if ambiguous.payload == nil || len(items(t, ambiguous.payload)) < 2 {
		t.Fatal("ambiguous location did not return typed usable choices")
	}
	before = r.count()
	offlineAmbiguous := w.run(t, "find", "--area", "Shinjuku", "--data-source", "local", "--agent")
	mustFail(t, offlineAmbiguous)
	if offlineAmbiguous.payload == nil || len(items(t, offlineAmbiguous.payload)) < 2 {
		t.Fatal("cached source choices lost Shinjuku area/station ambiguity offline")
	}
	equal(t, r.count(), before)
	for _, req := range r.seen() {
		if strings.Contains(req.path, "rstLst") {
			t.Fatal("ambiguous name silently launched a source-wide listing")
		}
	}
	var stationSelector string
	for _, choice := range items(t, ambiguous.payload) {
		if choice["station_id"] == "5172" {
			stationSelector, _ = choice["selector"].(string)
		}
	}
	if stationSelector == "" {
		t.Fatal("ambiguity response lost station5172 selector")
	}
	mustSucceed(t, w.run(t, "find", "--area", stationSelector, "--agent"))
	before = r.count()
	mustSucceed(t, w.run(t, "find", "--area", stationSelector, "--data-source", "local", "--agent"))
	equal(t, r.count(), before)
	seen := r.seen()
	listings := 0
	for _, req := range seen {
		if req.path == "/en/suggest/keyword_suggest" {
			if !strings.Contains(req.accept, "application/json") || req.requestedWith != "XMLHttpRequest" || req.referer != "https://tabelog.com/en/" {
				t.Fatal("public suggestion AJAX contract not honored")
			}
		}
		if strings.Contains(req.path, "rstLst") {
			listings++
			if !strings.Contains(req.path, "R5172") && !strings.Contains(req.query, "station_id=5172") {
				t.Fatal("chosen station vicinity was lost")
			}
		}
	}
	equal(t, listings, 1)
}

func TestGenuineSourceEmptyResultsAreDistinctFromDrift(t *testing.T) {
	body := readFixture(t, "ginza-empty.html")
	r := newReplay(t, func(*http.Request) response { return response{body: body} })
	w := newWorkspace(t, r)
	keyword := "zzztabeloge2ezerocandidates20260927"
	p := mustSucceed(t, w.run(t, "find", "--area", ginzaURL, "--keyword", keyword, "--agent"))
	equal(t, len(items(t, p)), 0)
	meta := assertMeta(t, p)
	equal(t, meta["returned"], float64(0))
	equal(t, meta["scanned"], float64(0))
	equal(t, meta["has_more"], false)
	equal(t, r.count(), 1)
	if !strings.Contains(r.seen()[0].query, "sw="+keyword) {
		t.Fatal("free-text keyword did not reach the source")
	}
}

func TestRelocatedAndCurrentSameNameRecordsRemainDistinctWhenSaved(t *testing.T) {
	old, current := readFixture(t, "miyakawa-old.html"), readFixture(t, "miyakawa-current.html")
	r := newReplay(t, func(req *http.Request) response {
		if strings.Contains(req.URL.Path, "1046463") {
			return response{body: old}
		}
		if strings.Contains(req.URL.Path, "1073214") {
			return response{body: current}
		}
		return response{status: 404}
	})
	w := newWorkspace(t, r)
	for _, id := range []string{"1046463", "1073214"} {
		url := "https://tabelog.com/en/hokkaido/A0101/A010105/" + id + "/"
		mustSucceed(t, w.run(t, "show", url, "--agent"))
		mustSucceed(t, w.run(t, "lists", "add", "hokkaido", id, "--note", "record "+id, "--agent"))
	}
	requests := r.count()
	p := mustSucceed(t, w.run(t, "lists", "compare", "hokkaido", "--data-source", "local", "--agent"))
	values := items(t, p)
	equal(t, len(values), 2)
	previous := restaurant(t, values, "1046463")
	present := restaurant(t, values, "1073214")
	equal(t, previous["name"], "Sushi Miyakawa")
	equal(t, present["name"], "Sushi Miyakawa")
	status, _ := previous["status"].(string)
	if !strings.EqualFold(status, "relocated") {
		t.Fatal("saved old record lost explicit relocated status")
	}
	if !bytes.Contains(mustJSON(previous["source_warnings"]), []byte("This is information from before the relocation.")) {
		t.Fatal("saved old record lost historical lifecycle warning")
	}
	if present["status"] != nil {
		t.Fatal("absent lifecycle notice was converted into an open/closed claim")
	}
	equal(t, previous["rating"], 4.46)
	equal(t, present["rating"], 4.48)
	equal(t, r.count(), requests)
}

func TestDryRunsAndHelpRemainNetworkFreeAcrossApprovedLeaves(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 500} })
	w := newWorkspace(t, r)
	leaves := [][]string{{"areas"}, {"cuisines"}, {"find"}, {"show"}, {"lists", "add"}, {"lists", "show"}, {"lists", "note"}, {"lists", "remove"}, {"lists", "compare"}, {"lists", "refresh"}, {"lists", "alternatives"}, {"lists", "audit"}}
	for _, leaf := range leaves {
		t.Run(strings.Join(leaf, "-"), func(t *testing.T) {
			help := w.run(t, append(append([]string{}, leaf...), "--help")...)
			equal(t, help.code, 0)
			if len(help.stdout) == 0 {
				t.Fatal("help empty")
			}
			planned := w.run(t, append(append([]string{}, leaf...), "--dry-run", "--agent")...)
			p := mustSucceed(t, planned)
			coverage := object(t, p["meta"])["coverage"]
			if coverage != "dry_run" && coverage != "planned" {
				t.Fatalf("dry run did not identify a plan: %v", coverage)
			}
			equal(t, r.count(), 0)
		})
	}
}
