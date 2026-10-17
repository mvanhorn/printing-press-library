// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func scr(sym, sector string, close, sma50, sma200, rs, chg float64, extra ipRow) ipRow {
	r := ipRow{"symbol": sym, "name": sym + " Ltd", "sector": sector, "date": "2026-10-16",
		"close": close, "sma50": sma50, "sma200": sma200, "rs": rs, "chg1d": chg,
		"ret1w": chg * 2, "ret1m": chg * 3, "ret3m": chg * 4, "ret6m": 1.0, "ret1y": 2.0,
		"newhi": 0.0, "fromlo52": 10.0, "volratio": 1.0, "pos52": 50.0}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func TestRankMovers(t *testing.T) {
	rows := []ipRow{
		{"symbol": "AAA", "sector": "IT", "ltp": 100.0, "chg": 5.0, "relVol": 1.0, "gap": 0.5, "valueCr": 10.0},
		{"symbol": "BBB", "sector": "IT", "ltp": 50.0, "chg": -3.0, "relVol": 3.5, "gap": -2.5, "valueCr": 40.0},
		{"symbol": "CCC", "sector": "FMCG", "ltp": 80.0, "chg": 1.0, "relVol": 2.0, "gap": 1.0, "valueCr": 5.0, "hi52": 80.0},
		{"symbol": "DDD", "sector": "FMCG", "ltp": 10.0, "chg": -1.0, "relVol": 0.5, "gap": 0.1, "valueCr": 1.0, "lo52": 10.0},
	}
	rs := map[string]float64{"AAA": 80, "BBB": 40, "CCC": 90}
	top := func(kind string, minRS float64, sector string) []string {
		got, err := rankMovers(rows, kind, rs, minRS, sector, 0)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, m := range got {
			out = append(out, m.Symbol)
		}
		return out
	}
	if got := strings.Join(top("gainers", 0, ""), ","); got != "AAA,CCC,DDD,BBB" {
		t.Errorf("gainers = %s", got)
	}
	if got := strings.Join(top("losers", 0, ""), ","); got != "BBB,DDD,CCC,AAA" {
		t.Errorf("losers = %s", got)
	}
	if got := strings.Join(top("volume", 0, ""), ","); got != "BBB,CCC,AAA,DDD" {
		t.Errorf("volume = %s", got)
	}
	if got := strings.Join(top("gap", 0, ""), ","); got != "BBB,CCC,AAA,DDD" {
		t.Errorf("gap = %s", got)
	}
	if got := strings.Join(top("high52", 0, ""), ","); got != "CCC" {
		t.Errorf("high52 = %s", got)
	}
	if got := strings.Join(top("low52", 0, ""), ","); got != "DDD" {
		t.Errorf("low52 = %s", got)
	}
	if got := strings.Join(top("gainers", 70, ""), ","); got != "AAA,CCC" {
		t.Errorf("min-rs 70 = %s (DDD has no RS and must be excluded)", got)
	}
	if got := strings.Join(top("gainers", 0, "fmcg"), ","); got != "CCC,DDD" {
		t.Errorf("sector filter = %s", got)
	}
	if got := top("gainers", 99, ""); len(got) != 0 {
		t.Errorf("min-rs 99 should be empty, got %v", got)
	}
	if _, err := rankMovers(rows, "bogus", rs, 0, "", 0); err == nil {
		t.Error("unknown kind must error")
	}
}

func TestRankSectors(t *testing.T) {
	rows := []ipRow{
		scr("A", "IT", 120, 100, 90, 80, 2, nil),
		scr("B", "IT", 90, 100, 95, 60, 4, nil),
		scr("C", "FMCG", 50, 40, 30, 50, 1, nil),
	}
	got := rankSectors(rows, "ret1m")
	if len(got) != 2 || got[0].Sector != "IT" || got[0].Rank != 1 {
		t.Fatalf("ranking = %+v", got)
	}
	it := got[0]
	if it.Stocks != 2 || it.ReturnPct != 9 || it.PctAbove50 != 50 || it.PctAbove200 != 50 || it.AvgRS != 70 || it.Leader != "B" {
		t.Errorf("IT = %+v", it)
	}
	if got[1].Sector != "FMCG" || got[1].Rank != 2 {
		t.Errorf("FMCG = %+v", got[1])
	}
}

func TestDiffSnapshots(t *testing.T) {
	base := []ipRow{
		scr("UP", "IT", 90, 100, 80, 60, 0, nil),    // below 50dma, rs<70
		scr("DOWN", "IT", 110, 100, 80, 75, 0, nil), // above 50dma, rs>=70
		scr("SAME", "IT", 110, 100, 80, 75, 0, nil),
		scr("GONE", "IT", 110, 100, 80, 75, 0, nil),
	}
	cur := []ipRow{
		scr("UP", "IT", 110, 100, 80, 72, 0, ipRow{"newhi": 1.0}),
		scr("DOWN", "IT", 90, 100, 80, 65, 0, nil),
		scr("SAME", "IT", 110, 100, 80, 75, 0, nil),
		scr("NEW", "FMCG", 110, 100, 80, 90, 0, nil), // not in baseline: ignored
	}
	d := diffSnapshots(base, cur, 70, "")
	if got := strings.Join(d["above_50dma"].Entered, ","); got != "UP" {
		t.Errorf("above_50dma entered = %s", got)
	}
	if got := strings.Join(d["above_50dma"].Left, ","); got != "DOWN" {
		t.Errorf("above_50dma left = %s", got)
	}
	if got := strings.Join(d["rs_above_threshold"].Entered, ","); got != "UP" {
		t.Errorf("rs entered = %s", got)
	}
	if got := strings.Join(d["new_52w_high"].Entered, ","); got != "UP" {
		t.Errorf("new high entered = %s", got)
	}
	if n := len(d["above_200dma"].Entered) + len(d["above_200dma"].Left); n != 0 {
		t.Errorf("no 200dma change expected, got %d", n)
	}
	if got := diffSnapshots(base, cur, 70, "FMCG"); len(got["above_50dma"].Entered) != 0 {
		t.Errorf("sector filter should exclude IT changes: %+v", got["above_50dma"])
	}
	// identical snapshots (single-snapshot case) produce empty, non-nil slices
	same := diffSnapshots(cur, cur, 70, "")
	if same["above_50dma"].Entered == nil || len(same["above_50dma"].Entered) != 0 {
		t.Errorf("identical snapshots must give empty lists: %+v", same["above_50dma"])
	}
}

func TestParseSinceDays(t *testing.T) {
	cases := map[string]int{"1d": 1, "3d": 3, "1w": 7, "24h": 1, "48h": 2, "2W": 14}
	for in, want := range cases {
		got, err := parseSinceDays(in)
		if err != nil || got != want {
			t.Errorf("%s = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "x", "1m", "-1d", "1.5d"} {
		if _, err := parseSinceDays(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestBreadthOf(t *testing.T) {
	rows := []ipRow{
		scr("A", "IT", 120, 100, 90, 80, 2, ipRow{"newhi": 1.0}),
		scr("B", "IT", 90, 100, 95, 60, -4, ipRow{"fromlo52": 0.0}),
		scr("C", "IT", 110, 100, 105, 50, 0, nil),
		scr("D", "IT", 80, 100, 95, 50, -1, nil),
	}
	p := breadthOf("2026-10-16", rows)
	if p.Stocks != 4 || p.Advancers != 1 || p.Decliners != 2 || p.AdRatio == nil || *p.AdRatio != 0.5 {
		t.Errorf("A/D = %+v", p)
	}
	if p.PctAbove50 != 50 || p.PctAbove200 != 50 || p.New52wHighs != 1 || p.At52wLows != 1 {
		t.Errorf("breadth = %+v", p)
	}
	if p.AvgChgPct != -0.75 {
		t.Errorf("avg = %v", p.AvgChgPct)
	}
	empty := breadthOf("x", nil)
	if empty.Stocks != 0 || empty.AdRatio != nil {
		t.Errorf("empty = %+v", empty)
	}
}

func TestBuildQuote(t *testing.T) {
	screener := []ipRow{
		scr("A", "IT", 120, 100, 90, 80, 2, nil),
		scr("B", "IT", 90, 100, 95, 60, -4, nil),
		scr("C", "IT", 110, 100, 105, 40, 0, nil),
		scr("Z", "FMCG", 10, 10, 10, 99, 0, nil),
	}
	live := ipRow{"ltp": 121.5, "chg": 3.0, "relVol": 1.8, "signals": []any{map[string]any{"t": "brk", "tone": "up", "label": "Closed above its 20-day high", "why": "x", "weight": 4.0}}}
	q, ok := buildQuote("A", screener, live)
	if !ok || q.Price != 121.5 || q.ChgPct != 3 || q.Sector != "IT" {
		t.Fatalf("quote = %+v ok=%v", q, ok)
	}
	if q.SectorPercentile["rs"] != 67 { // 2 of 3 IT peers below
		t.Errorf("rs percentile = %v", q.SectorPercentile["rs"])
	}
	if len(q.Signals) != 1 || q.Signals[0].Type != "brk" || q.Volume["rel_vol"] != 1.8 {
		t.Errorf("live fields = %+v %+v", q.Signals, q.Volume)
	}
	if _, ok := buildQuote("NOPE", screener, nil); ok {
		t.Error("unknown symbol must not be found")
	}
	// live-only fallback when the screener lacks the symbol
	q2, ok := buildQuote("LIVEONLY", nil, ipRow{"name": "Live Co", "sector": "Power", "ltp": 5.0, "chg": 1.0})
	if !ok || q2.Name != "Live Co" || q2.Price != 5 {
		t.Errorf("live-only = %+v", q2)
	}
}

func TestSummarizeWatchlistOrdersByAbsMove(t *testing.T) {
	live := map[string]ipRow{
		"A": {"name": "A", "ltp": 10.0, "chg": 1.0},
		"B": {"name": "B", "ltp": 20.0, "chg": -5.0},
		"C": {"name": "C", "ltp": 30.0, "chg": 3.0},
	}
	w := summarizeWatchlist(ipWatchlist{Name: "w", Symbols: []string{"A", "B", "C", "MISSING"}}, live, map[string]float64{"B": 61})
	if len(w.Rows) != 3 || w.Rows[0].Symbol != "B" || w.Rows[1].Symbol != "C" {
		t.Fatalf("rows = %+v", w.Rows)
	}
	if w.Rows[0].RS == nil || *w.Rows[0].RS != 61 || w.Rows[1].RS != nil {
		t.Errorf("RS join wrong: %+v", w.Rows)
	}
}

func flightPage(view string) []byte {
	enc, _ := json.Marshal(`1:{"x":1,"initialView":` + view + `,"y":"}"}`)
	return []byte(fmt.Sprintf(`<html><script>self.__next_f.push([1,%s])</script></html>`, enc))
}

func TestParseChartHTMLAndBuildHistory(t *testing.T) {
	view := `{"symbol":"RELIANCE","name":"Reliance Industries","sector":"Oil & Gas",` +
		`"d":["2024-01-01","2024-06-03","2026-10-16"],"o":[2000,1500,1215],"h":[2100,1550,1215.4],"l":[1900,1480,1193],"c":[2000,1520,1211.2],"v":[1000,2000,3000],` +
		`"fac":[0.5,0.5,1],"adjustments":[{"date":"2024-10-28","factor":0.5,"source":"confirmed","label":"1:1 bonus"}],"events":[],"gaps":[],"metrics":{"close":1211.2},"asOf":"2026-10-16"}`
	v, err := parseChartHTML(flightPage(view))
	if err != nil {
		t.Fatal(err)
	}
	if v.Symbol != "RELIANCE" || len(v.D) != 3 || v.AsOf != "2026-10-16" {
		t.Fatalf("view = %+v", v)
	}
	adj := buildHistory(v, 0, false, "live")
	if adj.Rows[0].Close != 1000 || adj.Rows[0].Volume != 2000 || adj.Rows[2].Close != 1211.2 {
		t.Errorf("adjusted rows = %+v", adj.Rows)
	}
	if !adj.Adjusted || adj.Bars != 3 || adj.From != "2024-01-01" || adj.To != "2026-10-16" || len(adj.Adjustments) != 1 {
		t.Errorf("adjusted meta = %+v", adj)
	}
	raw := buildHistory(v, 0, true, "live")
	if raw.Rows[0].Close != 2000 || raw.Adjusted {
		t.Errorf("raw rows = %+v", raw.Rows)
	}
	one := buildHistory(v, 1, false, "local")
	if one.Bars != 1 || one.Rows[0].Date != "2026-10-16" {
		t.Errorf("1y window = %+v", one.Rows)
	}
	if adj.Events == nil {
		t.Error("events must be [] not null")
	}
}

func TestParseChartHTMLErrors(t *testing.T) {
	if _, err := parseChartHTML([]byte("<html>Sign in with Google</html>")); err == nil {
		t.Error("login page must error")
	}
	bad := `{"symbol":"X","d":["2024-01-01","2024-01-02"],"o":[1],"h":[1],"l":[1],"c":[1],"v":[1],"fac":[1]}`
	if _, err := parseChartHTML(flightPage(bad)); err == nil {
		t.Error("inconsistent array lengths must error")
	}
	trunc := []byte(`<script>self.__next_f.push([1,"1:{\"initialView\":{\"symbol\":\"X\""])</script>`)
	if _, err := parseChartHTML(trunc); err == nil {
		t.Error("truncated payload must error")
	}
}

func TestNormalizeSessionCookie(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"  abc-123 \n":                      "__Secure-authjs.session-token=abc-123",
		"Cookie: a=1; b=2":                  "a=1; b=2",
		"__Secure-authjs.session-token=zzz": "__Secure-authjs.session-token=zzz",
	}
	for in, want := range cases {
		if got := normalizeSessionCookie(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}
