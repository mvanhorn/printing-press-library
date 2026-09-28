package e2e

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestExplicitShowRouteCannotReuseAnotherRouteSnapshot(t *testing.T) {
	const otherRoute = "https://tabelog.com/en/tokyo/A1302/A130201/13294162/"
	detail := readFixture(t, "sushi-detail.html")
	r := newReplay(t, func(req *http.Request) response {
		if req.URL.Path == "/en/tokyo/A1301/A130103/13294162/" {
			return response{body: detail}
		}
		return response{status: http.StatusNotFound}
	})
	w := newWorkspace(t, r)
	first := mustSucceed(t, w.run(t, "show", sushiURL, "--agent"))
	original := restaurant(t, items(t, first), "13294162")
	equal(t, r.count(), 1)

	localMismatch := w.run(t, "show", otherRoute, "--data-source", "local", "--agent")
	mustFail(t, localMismatch)
	equal(t, localMismatch.code, 3)
	if !bytes.Contains(localMismatch.stderr, []byte("different canonical URL")) {
		t.Fatalf("route mismatch was not actionable: %.300s", localMismatch.stderr)
	}
	equal(t, r.count(), 1)

	for _, input := range []string{sushiURL, "13294162"} {
		cached := mustSucceed(t, w.run(t, "show", input, "--data-source", "local", "--agent"))
		value := restaurant(t, items(t, cached), "13294162")
		equal(t, value["url"], sushiURL)
		equal(t, value["fetched_at"], original["fetched_at"])
		equal(t, r.count(), 1)
	}

	mustFail(t, w.run(t, "show", otherRoute, "--data-source", "auto", "--agent"))
	equal(t, r.count(), 2)
	if got := r.seen()[1].path; got != "/en/tokyo/A1302/A130201/13294162/" {
		t.Fatalf("auto mode fetched %q rather than the requested route", got)
	}
	preserved := mustSucceed(t, w.run(t, "show", sushiURL, "--data-source", "local", "--agent"))
	equal(t, restaurant(t, items(t, preserved), "13294162")["fetched_at"], original["fetched_at"])
	equal(t, r.count(), 2)
}

func TestColdLocalAreasUseBundledMatchesButKeepUnknownMisses(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 500} })
	w := newWorkspace(t, r)
	p := mustSucceed(t, w.run(t, "areas", "Ginza", "--data-source", "local", "--agent"))
	found := false
	for _, choice := range items(t, p) {
		if choice["kind"] == "area" && choice["name"] == "Ginza" && choice["selector"] == ginzaURL {
			found = true
		}
	}
	if !found {
		t.Fatal("bundled Ginza area choice was missing from cold local output")
	}
	meta := object(t, p["meta"])
	equal(t, meta["source"], "local")
	equal(t, meta["requests"], float64(0))
	equal(t, r.count(), 0)

	for _, args := range [][]string{
		{"areas", "Ginza", "--kind", "station", "--data-source", "local", "--agent"},
		{"areas", "zzztabeloglocalunknown", "--data-source", "local", "--agent"},
	} {
		miss := w.run(t, args...)
		mustFail(t, miss)
		if !strings.Contains(string(miss.stderr), "cached source response") {
			t.Fatalf("unknown cold query did not report a cache miss: %.300s", miss.stderr)
		}
		equal(t, r.count(), 0)
	}
}
