// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

type topOutput struct {
	Meta    topMeta  `json:"meta"`
	Results []topRow `json:"results"`
}

// usChartsHandler serves the US iPhone chart fixture for every chart page,
// synthetic Lookups, and the US grossing RSS fixture.
func usChartsHandler(t *testing.T) func(r *http.Request) fakeResponse {
	charts := fixtureBytes(t, "charts_us_iphone_36.html")
	grossing := fixtureBytes(t, "rss_topgrossing_us_6014.json")
	return func(r *http.Request) fakeResponse {
		switch {
		case strings.Contains(r.URL.Path, "/charts/"):
			return okBody(charts)
		case r.URL.Path == "/lookup":
			return synthLookup(r, nil)
		case strings.Contains(r.URL.Path, "/rss/topgrossing"):
			return okBody(grossing)
		}
		return notFoundResp()
	}
}

func TestTopWebChartReturns200RanksAndRecordsSnapshot(t *testing.T) {
	env := newS2bEnv(t, true, usChartsHandler(t))
	out, stderr, err := env.run("top", "--chart", "top-free", "--genre", "36", "--limit", "200", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 200 {
		t.Fatalf("rows = %d, want 200", len(got.Results))
	}
	nonZero := 0
	for i, r := range got.Results {
		if r.Rank != i+1 || r.AppID == "" || r.Title == "" {
			t.Fatalf("row %d = %+v", i, r)
		}
		if r.RatingCount != nil && *r.RatingCount > 0 {
			nonZero++
		}
	}
	if nonZero <= 190 {
		t.Fatalf("non-zero ratingCount rows = %d, want > 190", nonZero)
	}
	// The first 25 ranks come with chart-page lockup fields; later ranks
	// must not invent them.
	if got.Results[0].EditorsChoice == nil || got.Results[0].Subtitle == "" {
		t.Fatalf("rank 1 lacks lockup fields: %+v", got.Results[0])
	}
	if got.Results[150].EditorsChoice != nil || got.Results[150].Subtitle != "" {
		t.Fatalf("rank 151 has invented lockup fields: %+v", got.Results[150])
	}
	m := got.Meta
	if m.Surface != "web" || m.Chart != "top-free" || m.GenreID != "36" || m.RanksAvailable != 200 || m.SnapshotRows != 200 || m.RowLimit != 200 || m.Hydrated != 200 || m.RatingsStored != 200 {
		t.Fatalf("meta = %+v", m)
	}
	if n := env.fake.count("/charts/36"); n != 1 {
		t.Fatalf("chart page requests = %d, want 1", n)
	}
	if n := env.fake.count("/lookup"); n != 2 {
		t.Fatalf("lookup requests = %d, want 2 (150 + 50 ids)", n)
	}
	st := env.store()
	key := store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free", Source: "web"}
	caps, err := store.ChartCaptures(context.Background(), st, key)
	if err != nil || len(caps) != 1 || caps[0].Rows != 200 || caps[0].RowLimit != 200 {
		t.Fatalf("captures = %+v %v", caps, err)
	}
	first := got.Results[0].AppID
	series, _ := store.RatingsSeries(context.Background(), st, first, "us", store.SourceLookup)
	if len(series) != 1 {
		t.Fatalf("ratings snapshots for %s = %d, want 1", first, len(series))
	}

	// A rerun inside the cache window is served locally: no new network
	// requests, no second chart capture and no duplicated ratings points.
	before := len(env.fake.urls())
	out, stderr, err = env.run("top", "--chart", "top-free", "--limit", "200", "--json")
	if err != nil {
		t.Fatalf("rerun: %v\n%s", err, stderr)
	}
	decodeJSON(t, out, &got)
	if len(env.fake.urls()) != before {
		t.Fatalf("rerun sent %d new requests, want 0", len(env.fake.urls())-before)
	}
	if !got.Meta.FromCache || got.Meta.RatingsStored != 0 {
		t.Fatalf("rerun meta = %+v", got.Meta)
	}
	caps, _ = store.ChartCaptures(context.Background(), st, key)
	series, _ = store.RatingsSeries(context.Background(), st, first, "us", store.SourceLookup)
	if len(caps) != 1 || len(series) != 1 {
		t.Fatalf("rerun duplicated snapshots: captures %d ratings %d", len(caps), len(series))
	}
}

func TestTopLimitStillStoresEveryRank(t *testing.T) {
	env := newS2bEnv(t, false, usChartsHandler(t))
	out, stderr, err := env.run("top", "--chart", "top-paid", "--limit", "10", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 10 || got.Meta.SnapshotRows != 200 {
		t.Fatalf("rows %d snapshot %d", len(got.Results), got.Meta.SnapshotRows)
	}
	if n := env.fake.count("/lookup"); n != 1 {
		t.Fatalf("lookup requests = %d, want 1", n)
	}
	if !strings.Contains(env.fake.urls()[0], "chart=top-paid") {
		t.Fatalf("chart URL = %s", env.fake.urls()[0])
	}
}

func TestTopGrossingCapsAt100WithNote(t *testing.T) {
	env := newS2bEnv(t, false, usChartsHandler(t))
	out, stderr, err := env.run("top", "--chart", "top-grossing", "--limit", "150", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 100 {
		t.Fatalf("rows = %d, want 100", len(got.Results))
	}
	notes := strings.Join(got.Meta.Notes, " | ")
	if !strings.Contains(notes, "at most 100") || !strings.Contains(notes, "--limit 150 was capped at 100") {
		t.Fatalf("notes = %q", notes)
	}
	if got.Meta.Surface != "rss" || got.Meta.RowLimit != 100 || got.Meta.SnapshotRows != 100 {
		t.Fatalf("meta = %+v", got.Meta)
	}
	if got.Results[0].Price != "Free" || got.Results[0].RatingCount == nil {
		t.Fatalf("row 1 = %+v", got.Results[0])
	}
	u := env.fake.urls()[0]
	if !strings.Contains(u, "/us/rss/topgrossingapplications/limit=100/json") || strings.Contains(u, "genre=") {
		t.Fatalf("grossing URL = %s", u)
	}
	caps, _ := store.ChartCaptures(context.Background(), env.store(), store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-grossing", Source: "rss"})
	if len(caps) != 1 || caps[0].RowLimit != 100 {
		t.Fatalf("captures = %+v", caps)
	}
}

func TestTopPKPaidPricesInRupees(t *testing.T) {
	charts := fixtureBytes(t, "charts_pk_iphone_6014.html")
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		switch {
		case strings.HasPrefix(r.URL.Path, "/pk/iphone/charts/6014"):
			return okBody(charts)
		case r.URL.Path == "/lookup":
			// Leave formattedPrice to the page so the storefront's own
			// price text is what the rows show.
			return synthLookup(r, func(id string, m map[string]any) bool {
				delete(m, "formattedPrice")
				m["currency"] = "PKR"
				return true
			})
		}
		return notFoundResp()
	})
	out, stderr, err := env.run("top", "--country", "pk", "--genre", "6014", "--chart", "top-paid", "--limit", "25", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 25 {
		t.Fatalf("rows = %d", len(got.Results))
	}
	for _, r := range got.Results {
		if !strings.Contains(r.Price, "Rs") {
			t.Fatalf("rank %d price %q lacks Rs", r.Rank, r.Price)
		}
	}
	if got.Meta.Storefront != "pk" || got.Meta.GenreID != "6014" || got.Meta.Genre != "Games" {
		t.Fatalf("meta = %+v", got.Meta)
	}
}

func TestTopEmptyChartIsHonestAndRecordsNothing(t *testing.T) {
	empty := pageModelHTML(
		map[string]any{"$kind": "ChartsPageIntent", "storefront": "us", "platform": "iphone", "genreId": "36", "chart": "top-free"},
		map[string]any{"segments": []any{map[string]any{"chart": "top-free", "shelves": []any{}}}},
	)
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.Contains(r.URL.Path, "/charts/") {
			return okBody(empty)
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	out, stderr, err := env.run("top", "--chart", "top-free", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if got.Results == nil || len(got.Results) != 0 {
		t.Fatalf("results = %#v, want []", got.Results)
	}
	if !strings.Contains(strings.Join(got.Meta.Notes, " "), "verified empty") {
		t.Fatalf("notes = %v", got.Meta.Notes)
	}
	if !strings.Contains(out, `"results": []`) {
		t.Fatalf("empty results must print as [] not null:\n%s", out)
	}
	caps, _ := store.ChartCaptures(context.Background(), env.store(), store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
	if len(caps) != 0 {
		t.Fatalf("an empty chart was recorded: %+v", caps)
	}
}

func TestTopMacGrossingEmptyFeed(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.Contains(r.URL.Path, "/rss/topgrossingmacapps") {
			return okBody([]byte(`{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`))
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	out, _, err := env.run("top", "--chart", "top-grossing", "--platform", "mac", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	notes := strings.Join(got.Meta.Notes, " ")
	if len(got.Results) != 0 || !strings.Contains(notes, "verified empty") || !strings.Contains(notes, "Mac grossing") {
		t.Fatalf("results %d notes %q", len(got.Results), notes)
	}
}

func TestTopKidsAgeBandUsesItsOwnSnapshotKey(t *testing.T) {
	env := newS2bEnv(t, false, usChartsHandler(t))
	out, stderr, err := env.run("top", "--age-band", "6-8", "--limit", "5", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	// The series key is genre 36 plus the age band, never the interim
	// composite genre string "36-kids-10001".
	want := store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", AgeBand: "10001", Chart: "top-free", Source: "web"}
	if got.Meta.AgeBandID != "10001" || got.Meta.SnapshotKey != want {
		t.Fatalf("meta = %+v, want snapshotKey %+v", got.Meta, want)
	}
	if strings.Contains(out, "36-kids-") {
		t.Fatalf("output names the composite genre key:\n%s", out)
	}
	if !strings.Contains(env.fake.urls()[0], "ageBandId=10001") {
		t.Fatalf("chart URL = %s", env.fake.urls()[0])
	}
	st := env.store()
	kids, _ := store.ChartCaptures(context.Background(), st, store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", AgeBand: "10001", Chart: "top-free"})
	overall, _ := store.ChartCaptures(context.Background(), st, store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
	if len(kids) != 1 || len(overall) != 0 {
		t.Fatalf("kids captures %d, overall captures %d (a Kids chart must not land in the 36 series)", len(kids), len(overall))
	}
	var composite int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM chart_snapshots WHERE genre_id != '36' OR age_band != '10001'`).Scan(&composite); err != nil || composite != 0 {
		t.Fatalf("rows outside genre 36 + band 10001: %d (%v)", composite, err)
	}
}

// An empty --age-band is the overall chart, never the All Kids chart
// (ageBandId 0): resolving an unset flag must not change the chart.
func TestTopEmptyAgeBandIsTheOverallChart(t *testing.T) {
	env := newS2bEnv(t, false, usChartsHandler(t))
	out, stderr, err := env.run("top", "--age-band", "", "--limit", "5", "--json")
	if err != nil {
		t.Fatalf("top: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if got.Meta.AgeBandID != "" || got.Meta.AgeBand != "" {
		t.Fatalf("an empty --age-band selected a Kids band: %+v", got.Meta)
	}
	if u := env.fake.urls()[0]; strings.Contains(u, "ageBandId") {
		t.Fatalf("chart URL = %s, want the overall chart", u)
	}
	st := env.store()
	overall, _ := store.ChartCaptures(context.Background(), st, store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
	kids, _ := store.ChartRows(context.Background(), st, store.ChartQuery{AgeBand: "0"})
	if len(overall) != 1 || len(kids) != 0 {
		t.Fatalf("overall captures %d, All Kids rows %d; an empty band must store the overall chart", len(overall), len(kids))
	}
}

func TestTopRejectsBadInputsWithoutNetwork(t *testing.T) {
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	cases := [][]string{
		{"top", "--chart", "top-bogus"},
		{"top", "--limit", "0"},
		{"top", "--limit", "201"},
		{"top", "--platform", "watch"},
		{"top", "--genre", "not-a-genre"},
		{"top", "--age-band", "6-8", "--genre", "6014"},
		{"top", "--age-band", "6-8", "--chart", "top-grossing"},
		{"top", "--age-band", "6-8", "--platform", "mac"},
		{"top", "--age-band", "teens"},
		{"top", "games"},
		{"top", "--country", "zz"},
	}
	for _, args := range cases {
		_, _, err := env.run(append(args, "--json")...)
		if ExitCode(err) != 2 {
			t.Errorf("%v: exit %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	if n := len(env.fake.urls()); n != 0 {
		t.Fatalf("validation failures sent %d requests", n)
	}
}

func TestTopThrottleExits7(t *testing.T) {
	t.Run("chart page throttled", func(t *testing.T) {
		env := newS2bEnv(t, false, func(r *http.Request) fakeResponse { return throttled() })
		_, _, err := env.run("top", "--json")
		if ExitCode(err) != 7 || !isRateLimited(err) {
			t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
		}
		caps, _ := store.ChartCaptures(context.Background(), env.store(), store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
		if len(caps) != 0 {
			t.Fatal("a throttled run recorded a chart")
		}
	})
	t.Run("lookup throttled after the page", func(t *testing.T) {
		charts := fixtureBytes(t, "charts_us_iphone_36.html")
		env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
			if strings.Contains(r.URL.Path, "/charts/") {
				return okBody(charts)
			}
			return throttled()
		})
		out, _, err := env.run("top", "--limit", "200", "--json")
		if ExitCode(err) != 7 {
			t.Fatalf("exit %d (%v), want 7", ExitCode(err), err)
		}
		// The ranks are real and print, with a note that the rating counts
		// stopped; exit 7 says the run is partial.
		if !strings.Contains(out, `"rank": 1`) || !strings.Contains(out, "hydration stopped early") {
			t.Fatalf("a hydration throttle must keep the ranks and say why counts are missing:\n%s", out)
		}
		if n := env.fake.count("/lookup"); n != 1 {
			t.Fatalf("lookup requests after the throttle = %d, want 1 (governor stops)", n)
		}
		// The chart ranks themselves were real and are kept.
		caps, _ := store.ChartCaptures(context.Background(), env.store(), store.ChartKey{Storefront: "us", Platform: "iphone", GenreID: "36", Chart: "top-free"})
		if len(caps) != 1 || caps[0].Rows != 200 {
			t.Fatalf("captures = %+v", caps)
		}
	})
}

func TestTopLookupMissingIDsAreNotZeroed(t *testing.T) {
	charts := fixtureBytes(t, "charts_us_iphone_36.html")
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		if strings.Contains(r.URL.Path, "/charts/") {
			return okBody(charts)
		}
		n := 0
		return synthLookup(r, func(id string, m map[string]any) bool {
			n++
			return n != 30 // Apple leaves one charted app out
		})
	})
	out, _, err := env.run("top", "--limit", "40", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	missing := got.Results[29]
	if missing.RatingCount != nil || missing.Genre != "" {
		t.Fatalf("missing app got invented Lookup fields: %+v", missing)
	}
	if len(got.Meta.LookupMissing) != 1 || got.Meta.Hydrated != 39 {
		t.Fatalf("meta = %+v", got.Meta)
	}
}

func TestTopAgentModeKeepsSparseRowFields(t *testing.T) {
	env := newS2bEnv(t, false, usChartsHandler(t))
	out, _, err := env.run("top", "--limit", "100", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 100 || got.Results[0].EditorsChoice == nil || got.Results[0].Subtitle == "" || got.Results[99].RatingCount == nil {
		t.Fatalf("--agent dropped row fields: first %+v last %+v", got.Results[0], got.Results[99])
	}
}

func TestTopMacChartAndGenreNames(t *testing.T) {
	mac := fixtureBytes(t, "charts_us_mac_36.html")
	env := newS2bEnv(t, false, func(r *http.Request) fakeResponse {
		switch {
		case strings.HasPrefix(r.URL.Path, "/us/mac/charts/"):
			return okBody(mac)
		case r.URL.Path == "/lookup":
			return synthLookup(r, nil)
		}
		t.Errorf("unexpected request %s", r.URL)
		return notFoundResp()
	})
	out, stderr, err := env.run("top", "--platform", "macos", "--chart", "paid", "--genre", "all apps", "--limit", "5", "--json")
	if err != nil {
		t.Fatalf("top mac: %v\n%s", err, stderr)
	}
	var got topOutput
	decodeJSON(t, out, &got)
	if len(got.Results) != 5 || got.Meta.Platform != "mac" || got.Meta.Chart != "top-paid" || got.Meta.GenreID != "36" {
		t.Fatalf("rows %d meta %+v", len(got.Results), got.Meta)
	}
	if u := env.fake.urls()[0]; u != "https://apps.apple.com/us/mac/charts/36?chart=top-paid" {
		t.Fatalf("mac chart URL = %s", u)
	}
	caps, _ := store.ChartCaptures(context.Background(), env.store(), store.ChartKey{Storefront: "us", Platform: "mac", GenreID: "36", Chart: "top-paid", Source: "web"})
	if len(caps) != 1 || caps[0].Rows != 200 {
		t.Fatalf("mac captures = %+v", caps)
	}
	// Genre names resolve through genres.go.
	if _, _, err := env.run("top", "--platform", "mac", "--genre", "developer tools", "--limit", "1", "--json"); err != nil {
		t.Fatalf("genre name: %v", err)
	}
	if u := env.fake.urls()[len(env.fake.urls())-2]; !strings.Contains(u, "/us/mac/charts/6026?chart=top-free") {
		t.Fatalf("developer tools chart URL = %s", u)
	}
}
