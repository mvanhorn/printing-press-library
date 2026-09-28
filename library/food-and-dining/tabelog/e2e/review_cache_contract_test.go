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

func TestColdLocalBareAreaCannotImplyUnambiguousGeography(t *testing.T) {
	r := newReplay(t, func(*http.Request) response { return response{status: 500} })
	w := newWorkspace(t, r)
	choices := mustSucceed(t, w.run(t, "areas", "Shinjuku", "--data-source", "local", "--agent"))
	areaURL := "https://tabelog.com/en/tokyo/A1304/A130401/rstLst/"
	foundArea := false
	for _, choice := range items(t, choices) {
		if choice["kind"] == "area" && choice["name"] == "Shinjuku" && choice["selector"] == areaURL {
			foundArea = true
		}
	}
	if !foundArea {
		t.Fatal("bundled Shinjuku area should remain visible for an explicit typed choice")
	}
	equal(t, object(t, choices["meta"])["source"], "local")
	equal(t, object(t, choices["meta"])["requests"], float64(0))
	equal(t, r.count(), 0)

	bare := w.run(t, "find", "--area", "Shinjuku", "--data-source", "local", "--agent")
	mustFail(t, bare)
	if !bytes.Contains(bare.stderr, []byte("offline location choices are incomplete")) {
		t.Fatalf("bare local location implied certainty: %.300s", bare.stderr)
	}
	if bare.payload == nil || len(items(t, bare.payload)) == 0 {
		t.Fatal("ambiguous local lookup should return the usable bundled choice")
	}
	equal(t, r.count(), 0)

	explicit := w.run(t, "find", "--area", areaURL, "--data-source", "local", "--agent")
	mustFail(t, explicit)
	if !bytes.Contains(explicit.stderr, []byte("cached source response")) {
		t.Fatalf("explicit area was not accepted before the listing cache miss: %.300s", explicit.stderr)
	}
	equal(t, r.count(), 0)
	prefecture := w.run(t, "find", "--area", "tokyo", "--data-source", "local", "--agent")
	mustFail(t, prefecture)
	if !bytes.Contains(prefecture.stderr, []byte("cached source response")) {
		t.Fatalf("verified prefecture slug did not reach the listing cache: %.300s", prefecture.stderr)
	}
	equal(t, r.count(), 0)
}

func TestColdLocalBareAreaCannotUseAnExistingAreaListingWithoutTypedChoice(t *testing.T) {
	const areaURL = "https://tabelog.com/en/tokyo/A1304/A130401/rstLst/"
	// Synthetic location rewrite of the independently captured Ginza listing is
	// cache setup only; this test makes no claim about Shinjuku restaurant facts.
	body := readFixture(t, "ginza-bars.html")
	body = bytes.ReplaceAll(body, []byte("A130101"), []byte("A130401"))
	body = bytes.ReplaceAll(body, []byte("A1301"), []byte("A1304"))
	body = bytes.ReplaceAll(body, []byte("Ginza"), []byte("Shinjuku"))
	body = bytes.ReplaceAll(body, []byte("ginza"), []byte("shinjuku"))
	r := newReplay(t, func(*http.Request) response { return response{body: body} })
	w := newWorkspace(t, r)
	criteria := []string{"--cuisine", "bar", "--meal", "dinner", "--budget-max", "5000", "--agent"}
	explicitArgs := append([]string{"find", "--area", areaURL, "--data-source", "live"}, criteria...)
	seeded := mustSucceed(t, w.run(t, explicitArgs...))
	if len(items(t, seeded)) == 0 {
		t.Fatal("explicit source-backed area did not seed a listing cache")
	}
	requests := r.count()

	bareArgs := append([]string{"find", "--area", "Shinjuku", "--data-source", "local"}, criteria...)
	bare := w.run(t, bareArgs...)
	mustFail(t, bare)
	if !bytes.Contains(bare.stderr, []byte("offline location choices are incomplete")) {
		t.Fatalf("cached area listing hid incomplete location evidence: %.300s", bare.stderr)
	}
	equal(t, r.count(), requests)

	localArgs := append([]string{"find", "--area", areaURL, "--data-source", "local"}, criteria...)
	local := mustSucceed(t, w.run(t, localArgs...))
	equal(t, len(items(t, local)), len(items(t, seeded)))
	equal(t, object(t, local["meta"])["source"], "local")
	equal(t, object(t, local["meta"])["requests"], float64(0))
	equal(t, r.count(), requests)
}
