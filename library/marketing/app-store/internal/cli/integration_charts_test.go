// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Integration: the live chart writer (top) and the local chart readers
// (movers, rank-history, breakouts, traction) agree on the chart series key.
// top runs against an httptest Apple that serves the real chart-page fixture
// (and a re-ranked copy of it for the second capture) through the shared
// client seam; the readers then run on the store top wrote, for the overall
// chart and for a Kids age-band chart, which must stay separate series.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil/testenv"
)

// chartPageReranked rewrites one segment's ranks 26-200 (the
// nextPage.remainingContent ids) of a chart page fixture.
func chartPageReranked(t *testing.T, raw []byte, chart string, mutate func(ids []string) []string) []byte {
	t.Helper()
	re := regexp.MustCompile(`(?s)(<script type="application/json" id="serialized-server-data">)(.*?)(</script>)`)
	m := re.FindSubmatchIndex(raw)
	if m == nil {
		t.Fatal("chart fixture has no serialized-server-data")
	}
	dec := json.NewDecoder(bytes.NewReader(raw[m[4]:m[5]]))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	var entries []any
	switch v := doc.(type) {
	case map[string]any:
		entries, _ = v["data"].([]any)
	case []any:
		entries = v
	}
	data, _ := entries[0].(map[string]any)["data"].(map[string]any)
	segments, _ := data["segments"].([]any)
	found := false
	for _, s := range segments {
		seg, _ := s.(map[string]any)
		if seg["chart"] != chart {
			continue
		}
		next, _ := seg["nextPage"].(map[string]any)
		items, _ := next["remainingContent"].([]any)
		ids := make([]string, 0, len(items))
		for _, it := range items {
			id, _ := it.(map[string]any)["id"].(string)
			ids = append(ids, id)
		}
		ids = mutate(ids)
		out := make([]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, map[string]any{"id": id, "type": "apps"})
		}
		next["remainingContent"] = out
		found = true
	}
	if !found {
		t.Fatalf("chart fixture has no %s segment", chart)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	page := append([]byte{}, raw[:m[4]]...)
	page = append(page, body...)
	return append(page, raw[m[5]:]...)
}

// moveRank moves the id at rank from to rank to (ranks 26-200 map to
// remainingContent indexes 0-174).
func moveRank(ids []string, from, to int) []string {
	out := append([]string{}, ids...)
	id := out[from-26]
	out = append(out[:from-26], out[from-25:]...)
	idx := to - 26
	out = append(out[:idx], append([]string{id}, out[idx:]...)...)
	return out
}

type integrationRun struct {
	stdout, stderr string
	err            error
}

func TestIntegrationTopToMoversRankHistoryBreakouts(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	db := filepath.Join(dir, "store.db")
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	base := fixtureBytes(t, "charts_us_iphone_36.html")
	baseIDs := func() []string {
		var ids []string
		chartPageReranked(t, base, "top-free", func(in []string) []string { ids = in; return in })
		return ids
	}()
	// Overall chart, second capture: rank 150 climbs to 26 (everything from
	// 26 to 149 slips one place), the rank-200 app exits and a new app enters
	// at 200. Kids 6-8 chart, second capture: rank 100 climbs to 30.
	climber, exiter, entrant := baseIDs[150-26], baseIDs[200-26], "6999999901"
	overall2 := chartPageReranked(t, base, "top-free", func(ids []string) []string {
		ids = moveRank(ids, 150, 26)
		return append(ids[:len(ids)-1], entrant)
	})
	kidsClimber := baseIDs[100-26]
	kids2 := chartPageReranked(t, base, "top-free", func(ids []string) []string { return moveRank(ids, 100, 30) })

	t0 := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	now, phase := t0, 1
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		p := phase
		mu.Unlock()
		switch host := r.Header.Get("X-Test-Orig-Host"); {
		case host == "apps.apple.com" && regexp.MustCompile(`^/[a-z]{2}/iphone/charts/36$`).MatchString(r.URL.Path):
			body := base
			switch {
			case p == 2 && r.URL.Query().Get("ageBandId") == "10001":
				body = kids2
			case p == 2 && r.URL.Query().Get("ageBandId") == "":
				body = overall2
			}
			w.Header().Set("Cache-Control", "public, max-age=900")
			_, _ = w.Write(body)
		case host == "itunes.apple.com" && r.URL.Path == "/lookup":
			resp := synthLookup(r, nil)
			_, _ = w.Write(resp.Body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	target, _ := url.Parse(srv.URL)
	useFakeAppStore(t, fakeAppStore{
		Transport: s2aRewrite{target: target},
		CacheDir:  filepath.Join(dir, "cache"),
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	})
	run := func(args ...string) integrationRun {
		t.Helper()
		root := RootCmd()
		var out, errb bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errb)
		root.SetArgs(append(append([]string{}, args...), "--db", db, "--home", home))
		err := root.Execute()
		return integrationRun{stdout: out.String(), stderr: errb.String(), err: err}
	}
	mustRun := func(args ...string) string {
		t.Helper()
		res := run(args...)
		if res.err != nil {
			t.Fatalf("%v: %v\n%s", args, res.err, res.stderr)
		}
		return res.stdout
	}
	capture := func() {
		t.Helper()
		for _, cc := range []string{"us", "gb"} {
			mustRun("top", "--country", cc, "--limit", "200", "--json")
			mustRun("top", "--country", cc, "--age-band", "6-8", "--limit", "10", "--json")
		}
	}
	capture()
	mu.Lock()
	now, phase = t0.Add(26*time.Hour), 2
	mu.Unlock()
	capture()

	// movers: the overall chart shows the overall movement only.
	var movers []moversRow
	decodeJSON(t, mustRun("movers", "--country", "us", "--min-change", "5", "--json"), &movers)
	got := map[string]moversRow{}
	for _, r := range movers {
		got[r.Movement+":"+r.AppID] = r
	}
	if c, ok := got["climbed:"+climber]; !ok || *c.Rank != 26 || *c.PreviousRank != 150 || *c.Delta != 124 {
		t.Fatalf("overall movers: climber %s missing or wrong: %+v (all %d rows)", climber, c, len(movers))
	}
	if _, ok := got["entered:"+entrant]; !ok {
		t.Fatalf("overall movers: entrant %s missing: %+v", entrant, movers)
	}
	if _, ok := got["exited:"+exiter]; !ok {
		t.Fatalf("overall movers: exiter %s missing: %+v", exiter, movers)
	}
	if len(movers) != 3 {
		t.Fatalf("overall movers = %d rows, want 3 (climbed, entered, exited): %+v", len(movers), movers)
	}
	// movers --age-band reads the Kids series: only the Kids climber moved.
	decodeJSON(t, mustRun("movers", "--country", "us", "--age-band", "6-8", "--min-change", "5", "--json"), &movers)
	if len(movers) != 1 || movers[0].AppID != kidsClimber || movers[0].Movement != "climbed" || *movers[0].Delta != 70 {
		t.Fatalf("Kids movers = %+v, want only %s climbing 70", movers, kidsClimber)
	}

	// rank-history: one app's series per chart key.
	var hist rankHistoryView
	decodeJSON(t, mustRun("rank-history", climber, "--country", "us", "--json"), &hist)
	if hist.Captures != 2 || hist.Charted != 2 || hist.Peak == nil || hist.Peak.Rank != 26 || hist.FirstSeen.Rank != 150 || hist.AgeBand != "" {
		t.Fatalf("overall rank-history of %s = %+v", climber, hist)
	}
	decodeJSON(t, mustRun("rank-history", kidsClimber, "--country", "us", "--age-band", "6-8", "--json"), &hist)
	if hist.Captures != 2 || hist.Peak == nil || hist.Peak.Rank != 30 || hist.FirstSeen.Rank != 100 || hist.AgeBand != "10001" {
		t.Fatalf("Kids rank-history of %s = %+v", kidsClimber, hist)
	}
	decodeJSON(t, mustRun("rank-history", kidsClimber, "--country", "us", "--json"), &hist)
	if hist.Peak == nil || hist.Peak.Rank != 100 || hist.LastSeen.Rank != 101 {
		t.Fatalf("overall rank-history of %s must not see its Kids ranks: %+v", kidsClimber, hist)
	}

	// breakouts: both storefronts were captured twice per chart.
	var br breakoutsView
	decodeJSON(t, mustRun("breakouts", "--since", "all", "--min-markets", "2", "--json"), &br)
	if len(br.Meta.StorefrontsCompared) != 2 || len(br.Results) != 2 || br.Results[0].AppID != climber || br.Results[0].Markets != 2 || br.Results[0].MedianGain != 124 || br.Results[1].AppID != entrant {
		t.Fatalf("overall breakouts = %+v", br)
	}
	decodeJSON(t, mustRun("breakouts", "--since", "all", "--min-markets", "2", "--age-band", "6-8", "--json"), &br)
	if br.Meta.AgeBand != "10001" || len(br.Results) != 1 || br.Results[0].AppID != kidsClimber || br.Results[0].MedianGain != 70 {
		t.Fatalf("Kids breakouts = %+v", br)
	}

	// traction reads every chart the app is in: the Kids chart is its own,
	// named series, never folded into genre 36.
	var tr struct {
		Results []tractionRow `json:"results"`
	}
	decodeJSON(t, mustRun("traction", kidsClimber, "--countries", "us", "--period", "all-time", "--json"), &tr)
	if len(tr.Results) != 1 || tr.Results[0].BestRank == nil || *tr.Results[0].BestRank != 30 || tr.Results[0].BestRankChart != "iphone/36/top-free/kids-10001" {
		t.Fatalf("traction of %s = %+v", kidsClimber, tr.Results)
	}
}
