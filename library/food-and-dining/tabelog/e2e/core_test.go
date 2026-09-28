package e2e

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

const tokyoURL = "https://tabelog.com/en/tokyo/rstLst/"
const ginzaURL = "https://tabelog.com/en/tokyo/A1301/A130101/rstLst/"
const sushiURL = "https://tabelog.com/en/tokyo/A1301/A130103/13294162/"

func tokyoReplay(t *testing.T) *replay {
	body := readFixture(t, "tokyo-ranked.html")
	return newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/rstLst/" && req.URL.Query().Get("SrtT") == "rt" {
			return response{body: body}
		}
		return response{status: 404, body: []byte("unexpected replay request")}
	})
}

func assertMeta(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	meta := object(t, payload["meta"])
	for _, field := range []string{"source", "source_url", "fetched_at", "age_seconds", "stale", "source_sort", "returned", "scanned", "pages", "has_more", "requests", "coverage"} {
		if _, ok := meta[field]; !ok {
			t.Fatalf("essential metadata %s missing", field)
		}
	}
	if !strings.HasPrefix(meta["source_url"].(string), "https://tabelog.com/en/") {
		t.Fatal("transport override leaked into source provenance")
	}
	return meta
}

func assertBudget(t *testing.T, item map[string]any, field, raw string, min, max any) {
	t.Helper()
	b := object(t, item[field])
	equal(t, b["raw"], raw)
	equal(t, b["min_jpy"], min)
	equal(t, b["max_jpy"], max)
	if b["source"] == nil || b["source"] == "" {
		t.Fatal("budget provenance missing")
	}
}

func TestAgentFindPreservesFactsAndUnknownsWithoutDetailFanout(t *testing.T) {
	r := tokyoReplay(t)
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--limit", "20", "--agent"))
	values := items(t, p)
	equal(t, len(values), 20)
	hoshino := restaurant(t, values, "13136847")
	equal(t, hoshino["name"], "Shimbashi Hoshino")
	equal(t, hoshino["url"], "https://tabelog.com/en/tokyo/A1314/A131401/13136847/")
	equal(t, hoshino["rating"], 4.66)
	equal(t, hoshino["review_count"], float64(331))
	equal(t, hoshino["nearest_station"], "Onarimon Sta.")
	equal(t, hoshino["nearest_station_distance_m"], float64(416))
	assertBudget(t, hoshino, "dinner_budget", "JPY 50,000 - JPY 59,999", float64(50000), float64(59999))
	assertBudget(t, hoshino, "lunch_budget", "-", nil, nil)
	equal(t, hoshino["closures"], "Sunday, Public Holiday")
	equal(t, hoshino["source_surface"], "listing")
	if hoshino["fetched_at"] == "" || hoshino["fetched_at"] == nil {
		t.Fatal("snapshot fetch time missing")
	}
	awards := hoshino["awards"].([]any)
	equal(t, len(awards), 2)
	equal(t, awards[0], "The Tabelog Award 2026 Gold winner")
	equal(t, awards[1], "Selected for Tabelog Japanese cuisine TOKYO \"Tabelog 100\" 2025")
	trace := restaurant(t, values, "13246316")
	equal(t, trace["nearest_station_distance_m"], nil)
	equal(t, trace["nearest_station"], "")
	equal(t, trace["closures"], nil)
	assertBudget(t, trace, "lunch_budget", "-", nil, nil)
	sawada := restaurant(t, values, "13001043")
	equal(t, sawada["rating"], 4.5)
	equal(t, sawada["review_count"], float64(614))
	assertBudget(t, sawada, "dinner_budget", "JPY 50,000 - JPY 59,999", float64(50000), float64(59999))
	assertBudget(t, sawada, "lunch_budget", "JPY 40,000 - JPY 49,999", float64(40000), float64(49999))
	meta := assertMeta(t, p)
	equal(t, meta["returned"], float64(20))
	equal(t, meta["scanned"], float64(20))
	equal(t, meta["pages"], float64(1))
	equal(t, r.count(), 1)
}

func TestDefaultFindProjectionAndOfflineCache(t *testing.T) {
	r := tokyoReplay(t)
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--agent"))
	values := items(t, p)
	want := []string{"13136847", "13243463", "13249117", "13018162", "13300481"}
	equal(t, len(values), len(want))
	for i, id := range want {
		equal(t, values[i]["id"], id)
	}
	meta := assertMeta(t, p)
	equal(t, meta["returned"], float64(5))
	equal(t, meta["scanned"], float64(20))
	equal(t, meta["pages"], float64(1))
	if meta["coverage"] == "" {
		t.Fatal("omitted fetched results need explicit coverage")
	}
	equal(t, meta["coverage"], "source_pages")
	equal(t, meta["omitted_from_scanned"], float64(15))
	equal(t, meta["next_url_scope"], "after_scanned_pages")
	equal(t, meta["budget_source"], "listing")
	for _, key := range []string{"lunch_budget", "dinner_budget"} {
		if _, duplicate := object(t, rawItems(t, p)[0][key])["source"]; duplicate {
			t.Fatal("default find repeated shared budget provenance")
		}
		equal(t, object(t, values[0][key])["source"], "listing")
	}
	equal(t, r.count(), 1)
	for _, mode := range []string{"auto", "local"} {
		cached := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--data-source", mode, "--agent"))
		if !reflect.DeepEqual(items(t, cached), values) {
			t.Fatalf("%s changed cached source facts", mode)
		}
		equal(t, assertMeta(t, cached)["requests"], float64(0))
		equal(t, r.count(), 1)
	}
	selected := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--select", "items.id,items.name,items.rating,items.url", "--agent"))
	assertMeta(t, selected)
	for _, item := range rawItems(t, selected) {
		equal(t, len(item), 4)
		for _, key := range []string{"id", "name", "rating", "url"} {
			if _, ok := item[key]; !ok {
				t.Fatalf("projected %s missing", key)
			}
		}
	}
	equal(t, r.count(), 1)
	fullProjection := mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--select", "items.id,items.fetched_at,items.source_surface,items.area,items.dinner_budget", "--agent"))
	projected := restaurant(t, rawItems(t, fullProjection), "13136847")
	equal(t, projected["fetched_at"], values[0]["fetched_at"])
	equal(t, projected["source_surface"], "listing")
	equal(t, object(t, projected["area"])["area1"], "A1314")
	assertBudget(t, projected, "dinner_budget", "JPY 50,000 - JPY 59,999", float64(50000), float64(59999))
	equal(t, r.count(), 1)
	mustSucceed(t, w.run(t, "find", "--area", tokyoURL, "--data-source", "live", "--agent"))
	equal(t, r.count(), 2)
	missing := newWorkspace(t, r)
	mustFail(t, missing.run(t, "find", "--area", tokyoURL, "--data-source", "local", "--agent"))
	equal(t, r.count(), 2)
}

func TestShowKeepsBudgetSourcesAndStationReferencesSeparate(t *testing.T) {
	body := readFixture(t, "sushi-detail.html")
	r := newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/A1301/A130103/13294162/" {
			return response{body: body}
		}
		return response{status: 404}
	})
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "show", sushiURL, "--agent"))
	value := restaurant(t, items(t, p), "13294162")
	equal(t, value["name"], "Sushi Dokoro Isseki Sanchou")
	equal(t, value["rating"], 3.47)
	equal(t, value["review_count"], float64(529))
	equal(t, value["nearest_station"], "Shiodome Sta.")
	equal(t, value["nearest_station_distance_m"], float64(250))
	equal(t, value["address"], "東京都港区新橋4-20-2 新橋フォーワンビル 1F")
	assertBudget(t, value, "dinner_budget", "JPY 10,000 - JPY 14,999", float64(10000), float64(14999))
	assertBudget(t, value, "lunch_budget", "JPY 8,000 - JPY 9,999", float64(8000), float64(9999))
	assertBudget(t, value, "review_dinner_budget", "JPY 15,000 - JPY 19,999", float64(15000), float64(19999))
	assertBudget(t, value, "review_lunch_budget", "JPY 15,000 - JPY 19,999", float64(15000), float64(19999))
	if object(t, value["dinner_budget"])["source"] == object(t, value["review_dinner_budget"])["source"] {
		t.Fatal("listed and review budget sources were merged")
	}
	transport, _ := value["transportation"].(string)
	if !strings.Contains(transport, "270 meters from Shimbashi Station") {
		t.Fatal("second station access fact missing")
	}
	for _, field := range []string{"hours", "payment", "reservation"} {
		if value[field] == nil || value[field] == "" {
			t.Fatalf("practical detail %s missing", field)
		}
	}
	equal(t, value["source_surface"], "detail")
	equal(t, r.count(), 1)
	cached := mustSucceed(t, w.run(t, "show", "13294162", "--data-source", "local", "--agent"))
	equal(t, r.count(), 1)
	equal(t, restaurant(t, items(t, cached), "13294162")["fetched_at"], value["fetched_at"])
	mustFail(t, w.run(t, "show", "99999999", "--agent"))
	equal(t, r.count(), 1)
}

func TestBudgetRequestsAndNativePaginationPreserveSourceRanking(t *testing.T) {
	first, second := readFixture(t, "ginza-bars.html"), readFixture(t, "ginza-bars-page2.html")
	r := newReplay(t, func(req *http.Request) response {
		if strings.Contains(req.URL.Path, "/bar/2/") {
			return response{body: second}
		}
		if strings.Contains(req.URL.Path, "rstLst") {
			return response{body: first}
		}
		return response{status: 404}
	})
	w := newWorkspace(t, r)
	args := []string{"find", "--area", ginzaURL, "--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--limit", "25", "--max-pages", "2", "--agent"}
	p := mustSucceed(t, w.run(t, args...))
	values := items(t, p)
	equal(t, len(values), 25)
	// The first two bars have equal ratings: upstream order remains decisive.
	equal(t, values[0]["id"], "13005012")
	equal(t, values[1]["id"], "13002248")
	equal(t, values[20]["id"], "13224293")
	equal(t, values[24]["id"], "13127154")
	seen := r.seen()
	listingRequests := 0
	for _, req := range seen {
		if !strings.Contains(req.path, "rstLst") {
			continue
		}
		listingRequests++
		q, _ := url.ParseQuery(req.query)
		equal(t, q.Get("SrtT"), "rt")
		equal(t, q.Get("RdoCosTp"), "2")
		equal(t, q.Get("LstCosT"), "5")
		if listingRequests == 1 {
			if !strings.Contains(req.path, "/tokyo/A1301/A130101/") {
				equal(t, q.Get("LstPrf"), "A1301")
				equal(t, q.Get("LstAre"), "A130101")
			}
		}
		if listingRequests == 2 {
			equal(t, req.path, "/en/tokyo/A1301/A130101/rstLst/bar/2/")
		}
	}
	equal(t, listingRequests, 2)
	meta := assertMeta(t, p)
	equal(t, meta["returned"], float64(25))
	equal(t, meta["scanned"], float64(40))
	equal(t, meta["pages"], float64(2))
}
