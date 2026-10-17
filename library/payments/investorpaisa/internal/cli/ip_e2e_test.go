// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/investorpaisa/internal/cliutil/testenv"
)

// fakeSite serves a tiny InvestorPaisa: three stocks whose metrics shift
// between "days" so changes/breadth/sectors --trend have something to diff.
type fakeSite struct {
	day     atomic.Int32 // 0 = earlier session, 1 = later session
	unauth  atomic.Bool
	hits    atomic.Int32
	garbage atomic.Bool
	chartOK bool
}

func (f *fakeSite) screener() []ipRow {
	date := []string{"2026-10-15", "2026-10-16"}[f.day.Load()]
	rows := []ipRow{
		scr("TCS", "IT", 2465, 2241, 2466, 52, 4.9, nil),
		scr("INFY", "IT", 1161, 1100, 1000, 75, 4.6, nil),
		scr("ITC", "FMCG", 400, 410, 380, 40, -0.2, nil),
	}
	if f.day.Load() == 0 {
		rows[0] = scr("TCS", "IT", 2200, 2241, 2466, 45, -1, nil) // below 50dma yesterday
		rows[2] = scr("ITC", "FMCG", 420, 410, 380, 40, 0.5, nil)
	}
	for _, r := range rows {
		r["date"] = date
	}
	return rows
}

func (f *fakeSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	if f.unauth.Load() && r.URL.Path != "/api/health" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if f.garbage.Load() && (r.URL.Path == "/api/screener/rows" || r.URL.Path == "/api/live" || r.URL.Path == "/api/market/overview") {
		_, _ = w.Write([]byte(`{"error":"session expired"}`))
		return
	}
	switch {
	case r.URL.Path == "/api/health":
		_, _ = w.Write([]byte(`{"ok":true}`))
	case r.URL.Path == "/api/screener/rows":
		_ = json.NewEncoder(w).Encode(f.screener())
	case r.URL.Path == "/api/live":
		rows := []ipRow{
			{"symbol": "TCS", "name": "Tata Consultancy Services", "sector": "IT", "ltp": 2470.0, "chg": 5.1, "relVol": 2.4, "gap": 1.2, "valueCr": 2968.0, "hi52": 3350.0, "lo52": 1976.0,
				"signals": []any{map[string]any{"t": "brk", "tone": "up", "label": "Closed above its 20-day high", "why": "x", "weight": 4.0}}},
			{"symbol": "ITC", "name": "ITC", "sector": "FMCG", "ltp": 400.0, "chg": -0.2, "relVol": 0.7, "gap": 0.0, "valueCr": 341.0, "hi52": 500.0, "lo52": 380.0},
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "fetchedAt": "2026-10-16T10:00:00Z", "payload": map[string]any{
			"agg":       map[string]any{"n": 3, "up": 2, "down": 1, "avg": 0.4, "median": 0.3, "hi52": 1, "lo52": 0, "above200": 66.7, "valueCr": 3309.0, "unusualVolume": 1, "sectors": []any{map[string]any{"sector": "IT", "avg": 3.1, "up": 2, "down": 0}, map[string]any{"sector": "FMCG", "avg": -0.2, "up": 0, "down": 1}}},
			"rows":      rows,
			"indices":   map[string]any{"NIFTY50": map[string]any{"level": 23154.46, "chg": 157.1, "chgPct": 0.68, "prevClose": 22997.35}},
			"lastTrade": "15:43", "sessionDate": "2026-10-16"}})
	case r.URL.Path == "/api/market/universe":
		_, _ = w.Write([]byte(`[{"s":"TCS","n":"Tata Consultancy Services","i":"IT","close":2465,"chg1d":4.9,"asOf":"2026-10-16"},{"s":"ITC","n":"ITC","i":"FMCG","close":400,"chg1d":-0.2,"asOf":"2026-10-16"}]`))
	case r.URL.Path == "/api/market/overview":
		_, _ = w.Write([]byte(`{"asOf":"2026-10-16","signals":[{"s":"INDHOTEL","chg1d":4.1,"signal":{"tone":"up","type":"hi52","label":"Made a new 52-week high"}}],"sectors":[]}`))
	case r.URL.Path == "/api/watchlists":
		_, _ = w.Write([]byte(`[{"id":"w1","name":"My watchlist","symbols":["TCS","ITC"],"updatedAt":"2026-10-16T00:00:00Z"}]`))
	case r.URL.Path == "/api/news":
		_, _ = fmt.Fprintf(w, `{"items":[{"id":"n1","title":"%s wins deal","source":"Mint","publishedAt":"2099-01-01T00:00:00Z","link":"https://example.com","symbols":["%s"]}]}`, r.URL.Query().Get("symbol"), r.URL.Query().Get("symbol"))
	case r.URL.Path == "/api/me/layout":
		_, _ = w.Write([]byte(`{"v":1,"screener":{"expression":"rs >= 70","sector":null}}`))
	case strings.HasPrefix(r.URL.Path, "/chart/"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		sym := strings.TrimPrefix(r.URL.Path, "/chart/")
		view := fmt.Sprintf(`{"symbol":%q,"name":"Test Co","sector":"IT","d":["2026-10-15","2026-10-16"],"o":[1,2],"h":[2,3],"l":[1,1],"c":[1.5,2.5],"v":[100,200],"fac":[1,1],"adjustments":[],"events":[],"asOf":"2026-10-16"}`, sym)
		_, _ = w.Write(flightPage(view))
	default:
		http.NotFound(w, r)
	}
}

func runIP(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

func setupSite(t *testing.T) (*fakeSite, string) {
	t.Helper()
	testenv.Isolate(t)
	site := &fakeSite{}
	srv := httptest.NewServer(site)
	t.Cleanup(srv.Close)
	t.Setenv("INVESTORPAISA_BASE_URL", srv.URL)
	t.Setenv("INVESTORPAISA_SESSION_TOKEN", "test-session-token")
	return site, filepath.Join(t.TempDir(), "ip.db")
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
	if res, ok := m["results"].(map[string]any); ok { // --agent envelope
		return res
	}
	return m
}

func TestE2EScreenMoversQuoteBriefHistory(t *testing.T) {
	site, db := setupSite(t)
	site.day.Store(1)

	out, _, err := runIP(t, "screen", "close > sma50 and rs >= 50", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	res := decode(t, out)
	rows := res["rows"].([]any)
	if int(res["matched"].(float64)) != 3-1 || rows[0].(map[string]any)["symbol"] != "INFY" { // ITC fails close>sma50; sorted by rs desc
		t.Fatalf("screen = %s", out)
	}
	if _, _, err := runIP(t, "screen", "bogus > 1", "--json", "--db", db); err == nil {
		t.Error("unknown field must fail")
	}
	if _, _, err := runIP(t, "screen", "close >", "--json", "--db", db); err == nil {
		t.Error("bad expression must fail")
	}
	out, _, err = runIP(t, "screen", "--saved", "--json", "--db", db)
	if err != nil || int(decode(t, out)["matched"].(float64)) != 1 {
		t.Fatalf("saved screen = %s err=%v", out, err)
	}

	out, _, err = runIP(t, "movers", "--kind", "gainers", "--top", "5", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	mv := decode(t, out)["rows"].([]any)
	if len(mv) != 2 || mv[0].(map[string]any)["symbol"] != "TCS" || mv[0].(map[string]any)["rs"].(float64) != 52 {
		t.Fatalf("movers = %s", out)
	}

	out, _, err = runIP(t, "quote", "tcs", "NOPE", "--news", "1", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	qr := decode(t, out)
	q := qr["quotes"].([]any)[0].(map[string]any)
	if q["symbol"] != "TCS" || q["price"].(float64) != 2470 || len(q["news"].([]any)) != 1 || q["watchlists"].([]any)[0] != "My watchlist" {
		t.Fatalf("quote = %s", out)
	}
	if m := qr["missing"].([]any); len(m) != 1 || m[0] != "NOPE" {
		t.Errorf("missing = %v", m)
	}
	if _, _, err := runIP(t, "quote", "NOPE", "--json", "--db", db); err == nil {
		t.Error("all-unknown symbols must fail with not-found")
	}

	out, _, err = runIP(t, "brief", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	b := decode(t, out)
	if b["index"].(map[string]any)["nifty50"].(map[string]any)["level"].(float64) != 23154.46 {
		t.Fatalf("brief index = %s", out)
	}
	wl := b["watchlists"].([]any)[0].(map[string]any)["rows"].([]any)
	if len(wl) != 2 || len(b["headlines"].([]any)) == 0 {
		t.Fatalf("brief watchlist/headlines = %s", out)
	}

	out, _, err = runIP(t, "history", "TCS", "--years", "1", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	h := decode(t, out)
	if h["bars"].(float64) != 2 || h["source"] != "live" {
		t.Fatalf("history = %s", out)
	}
	site.unauth.Store(true)
	out, _, err = runIP(t, "history", "TCS", "--data-source", "local", "--json", "--db", db)
	if err != nil || decode(t, out)["source"] != "local" {
		t.Fatalf("cached history = %s err=%v", out, err)
	}
	if _, _, err := runIP(t, "history", "INFY", "--data-source", "local", "--json", "--db", db); err == nil {
		t.Error("uncached local history must fail")
	}
}

func TestE2EChangesBreadthSectorsAcrossDays(t *testing.T) {
	site, db := setupSite(t)

	// Single snapshot: honest empty result, exit 0.
	site.day.Store(0)
	out, _, err := runIP(t, "changes", "--since", "1d", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	c := decode(t, out)
	if c["note"] == nil || len(c["conditions"].(map[string]any)["above_50dma"].(map[string]any)["entered"].([]any)) != 0 {
		t.Fatalf("single-snapshot changes = %s", out)
	}
	if _, _, err := runIP(t, "breadth", "--days", "5", "--json", "--db", db); err != nil {
		t.Fatal(err)
	}

	// Next session: TCS crosses above its 50dma and RS 70 is untouched.
	site.day.Store(1)
	out, _, err = runIP(t, "changes", "--since", "1d", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	c = decode(t, out)
	if c["baseline_as_of"] != "2026-10-15" || c["as_of"] != "2026-10-16" {
		t.Fatalf("dates = %s", out)
	}
	entered := c["conditions"].(map[string]any)["above_50dma"].(map[string]any)["entered"].([]any)
	if len(entered) != 1 || entered[0] != "TCS" {
		t.Fatalf("above_50dma entered = %v", entered)
	}
	left := c["conditions"].(map[string]any)["above_50dma"].(map[string]any)["left"].([]any)
	if len(left) != 1 || left[0] != "ITC" {
		t.Fatalf("above_50dma left = %v", left)
	}

	out, _, err = runIP(t, "breadth", "--days", "5", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	b := decode(t, out)
	pts := b["points"].([]any)
	if len(pts) != 2 || pts[0].(map[string]any)["date"] != "2026-10-15" || b["trend"] == nil {
		t.Fatalf("breadth = %s", out)
	}

	out, _, err = runIP(t, "sectors", "--window", "1m", "--trend", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	s := decode(t, out)
	secs := s["sectors"].([]any)
	if s["prev_as_of"] != "2026-10-15" || secs[0].(map[string]any)["sector"] != "IT" || secs[0].(map[string]any)["rank_change"] == nil {
		t.Fatalf("sectors = %s", out)
	}
	if _, _, err := runIP(t, "sectors", "--window", "9y", "--json", "--db", db); err == nil {
		t.Error("bad window must fail")
	}
}

func TestE2EOfflineFallbackAndAuthFailure(t *testing.T) {
	site, db := setupSite(t)
	site.day.Store(1)
	if _, _, err := runIP(t, "screen", "rs >= 1", "--json", "--db", db); err != nil { // stores a snapshot
		t.Fatal(err)
	}
	// local mode never touches the network
	before := site.hits.Load()
	out, _, err := runIP(t, "screen", "rs >= 1", "--data-source", "local", "--json", "--db", db)
	if err != nil || decode(t, out)["source"] != "local" || site.hits.Load() != before {
		t.Fatalf("local mode hit the network or failed: %s %v", out, err)
	}
	// an expired session is an auth error, not silently masked by stale data
	site.unauth.Store(true)
	if _, _, err := runIP(t, "screen", "rs >= 1", "--json", "--db", db); err == nil {
		t.Fatal("401 must surface as an error even with a stored snapshot")
	}
	// local with nothing stored fails clearly
	_, _, err = runIP(t, "screen", "rs >= 1", "--data-source", "local", "--json", "--db", filepath.Join(t.TempDir(), "empty.db"))
	if err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("empty local store error = %v", err)
	}
}

func TestE2ESyncStoresSnapshots(t *testing.T) {
	site, db := setupSite(t)
	site.day.Store(0)
	if _, errOut, err := runIP(t, "sync", "--db", db); err != nil {
		t.Fatalf("sync failed: %v\n%s", err, errOut)
	}
	site.day.Store(1)
	if _, errOut, err := runIP(t, "sync", "--db", db); err != nil {
		t.Fatalf("second sync failed: %v\n%s", err, errOut)
	}
	out, _, err := runIP(t, "changes", "--since", "1d", "--data-source", "local", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	c := decode(t, out)
	if c["baseline_as_of"] != "2026-10-15" || c["source"] != "local" {
		t.Fatalf("changes after two syncs = %s", out)
	}
}

func TestE2EGarbageResponsesAreNeverStored(t *testing.T) {
	site, db := setupSite(t)
	site.day.Store(1)
	if _, _, err := runIP(t, "screen", "rs >= 1", "--json", "--db", db); err != nil {
		t.Fatal(err)
	}
	site.garbage.Store(true) // 200 with a JSON error envelope
	if _, _, err := runIP(t, "screen", "rs >= 1", "--json", "--db", db); err != nil {
		t.Fatalf("auto mode should fall back to the good stored snapshot: %v", err)
	}
	out, _, err := runIP(t, "screen", "rs >= 1", "--data-source", "local", "--json", "--db", db)
	if err != nil || decode(t, out)["source"] != "local" || decode(t, out)["as_of"] != "2026-10-16" {
		t.Fatalf("garbage must not shadow the good snapshot: %s %v", out, err)
	}
	if _, _, err := runIP(t, "screen", "rs >= 1", "--data-source", "live", "--json", "--db", db); err == nil {
		t.Fatal("live mode must surface the unexpected response")
	}
}

func TestE2ESavedScreenNeedsLiveAndSortValidation(t *testing.T) {
	site, db := setupSite(t)
	site.day.Store(1)
	if _, _, err := runIP(t, "screen", "--saved", "--data-source", "local", "--json", "--db", db); err == nil {
		t.Error("--saved with --data-source local must fail without touching the network")
	}
	if _, _, err := runIP(t, "screen", "rs >= 1", "--sort", "sector", "--json", "--db", db); err == nil {
		t.Error("sorting by a text column must be rejected")
	}
	if _, _, err := runIP(t, "history", "../x", "--json", "--db", db); err == nil {
		t.Error("invalid symbol must be rejected")
	}
	out, _, err := runIP(t, "history", "TCS", "--tail", "1", "--json", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	h := decode(t, out)
	if h["bars"].(float64) != 1 || h["from"] != "2026-10-16" || h["to"] != "2026-10-16" {
		t.Fatalf("--tail must refresh bars/from/to: %s", out)
	}
}
