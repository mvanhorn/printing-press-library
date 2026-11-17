// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/store"
)

// seedSerpDB writes saved pages: for each keyword, one page 10 days ago and one now.
func seedSerpDB(t *testing.T, pages map[string][2][]serp.Result) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "serp.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Now().Add(-10 * 24 * time.Hour)
	for q, pair := range pages {
		if _, err := db.SaveSerpSnapshot(context.Background(), store.SerpSnapshot{Query: q, Engine: "google", Country: "fr", TakenAt: old}, pair[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := db.SaveSerpSnapshot(context.Background(), store.SerpSnapshot{Query: q, Engine: "google", Country: "fr", TakenAt: time.Now()}, pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func res(pairs ...string) []serp.Result {
	out := make([]serp.Result, 0)
	for i, d := range pairs {
		out = append(out, serp.Result{Position: i + 1, URL: "https://" + d + "/", Domain: d, Title: d})
	}
	return out
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	return out.String(), err
}

func TestRankMoversDomain(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{
		"serp api pricing":      {res("a.com", "b.com", "c.com", "autom.dev"), res("autom.dev", "a.com", "b.com")},
		"google search scraper": {res("autom.dev", "x.com"), res("x.com", "y.com", "z.com")},
		"steady keyword":        {res("autom.dev", "x.com"), res("autom.dev", "x.com")},
	})
	out, err := runCLI(t, "rank", "movers", "--since", "7d", "--domain", "autom.dev", "--db", db, "--json")
	if err != nil {
		t.Fatalf("rank movers: %v\n%s", err, out)
	}
	var v moversView
	mustJSON(t, out, &v)
	if v.KeywordsCompared != 3 {
		t.Errorf("compared %d keywords, want 3", v.KeywordsCompared)
	}
	if len(v.Gains) != 1 || v.Gains[0].Query != "serp api pricing" || v.Gains[0].From != 4 || v.Gains[0].To != 1 || v.Gains[0].Delta != 3 {
		t.Errorf("gains = %+v, want serp api pricing 4 -> 1", v.Gains)
	}
	if len(v.Left) != 1 || v.Left[0].Query != "google search scraper" {
		t.Errorf("left = %+v, want google search scraper", v.Left)
	}
	if len(v.Losses) != 0 || len(v.Entered) != 0 {
		t.Errorf("a steady keyword must not report drift: losses %+v entered %+v", v.Losses, v.Entered)
	}
}

func TestRankMoversNoBaseline(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{"k": {res("a.com"), res("b.com")}})
	out, err := runCLI(t, "rank", "movers", "--since", "30d", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v moversView
	mustJSON(t, out, &v)
	if v.KeywordsCompared != 0 || v.KeywordsNoBaseline != 1 || v.Note == "" || len(v.Gains)+len(v.Entered) != 0 {
		t.Errorf("a cutoff older than every page must compare nothing, got %+v", v)
	}
}

func TestVisibility(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{
		"k1": {res("x.com"), res("autom.dev", "serpapi.com")},
		"k2": {res("x.com"), res("serpapi.com", "a.com", "b.com", "autom.dev")},
		"k3": {res("x.com"), res("serpapi.com")},
	})
	out, err := runCLI(t, "visibility", "autom.dev", "--vs", "serpapi.com", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v visibilityView
	mustJSON(t, out, &v)
	if v.KeywordsTracked != 3 || len(v.Domains) != 2 {
		t.Fatalf("unexpected view: %+v", v)
	}
	a, s := v.Domains[0], v.Domains[1]
	if a.Domain != "autom.dev" || a.KeywordsFound != 2 || a.AvgPosition == nil || *a.AvgPosition != 2.5 || a.BestPosition == nil || *a.BestPosition != 1 || a.Top3 != 1 {
		t.Errorf("autom.dev stats = %+v", a)
	}
	if s.KeywordsFound != 3 || s.Top3 != 3 || s.BestPosition == nil || *s.BestPosition != 1 {
		t.Errorf("serpapi.com stats = %+v", s)
	}
	out, err = runCLI(t, "visibility", "nowhere.example", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var none visibilityView
	mustJSON(t, out, &none)
	if len(none.Domains) != 1 {
		t.Fatalf("want one domain row, got %+v", none)
	}
	if none.Domains[0].AvgPosition != nil || none.Domains[0].BestPosition != nil {
		t.Errorf("a domain never found must report null positions, got %+v", none.Domains[0])
	}
}

func TestSerpDiffLatestTwo(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{"k": {res("a.com", "b.com"), res("b.com", "c.com")}})
	out, err := runCLI(t, "serp", "diff", "--query", "k", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v serpDiffView
	mustJSON(t, out, &v)
	kinds := map[string]string{}
	for _, c := range v.Changes {
		kinds[c.Domain] = c.Kind
	}
	if kinds["b.com"] != "up" || kinds["c.com"] != "entered" || kinds["a.com"] != "left" {
		t.Errorf("diff kinds = %v", kinds)
	}
	none, err := runCLI(t, "serp", "diff", "--query", "missing keyword", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var empty serpDiffView
	mustJSON(t, none, &empty)
	if len(empty.Changes) != 0 || empty.Note == "" {
		t.Errorf("an unknown keyword must give no changes and a note: %+v", empty)
	}
}

func TestRankHistoryRequiresDomain(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{"k": {res("a.com"), res("b.com")}})
	if _, err := runCLI(t, "rank", "history", "--db", db, "--json"); err == nil || ExitCode(err) != 2 {
		t.Errorf("missing domain should be a usage error (exit 2), got %v", err)
	}
	out, err := runCLI(t, "rank", "history", "b.com", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []rankHistoryRow
	mustJSON(t, out, &rows)
	if len(rows) != 2 || rows[0].Position != 1 || rows[1].Position != 0 {
		t.Errorf("history newest-first = %+v, want #1 then absent", rows)
	}
}

func TestParseBatchInput(t *testing.T) {
	def := batchItem{Engine: "google", Domain: "autom.dev"}
	lines, err := parseBatchInput(strings.NewReader("# comment\nserp api pricing\n\ngoogle search scraper\n"), def)
	if err != nil || len(lines) != 2 || lines[1].Query != "google search scraper" || lines[1].Domain != "autom.dev" {
		t.Fatalf("line format = %+v %v", lines, err)
	}
	csvIn := "keyword,domain,engine,country\nserp api pricing,autom.dev,bing,fr\nother,,,\n"
	rows, err := parseBatchInput(strings.NewReader(csvIn), def)
	if err != nil || len(rows) != 2 {
		t.Fatalf("csv = %+v %v", rows, err)
	}
	if rows[0].Engine != "bing" || rows[0].Country != "fr" || rows[1].Engine != "google" || rows[1].Domain != "autom.dev" {
		t.Errorf("csv defaults not applied: %+v", rows)
	}
	empty, err := parseBatchInput(strings.NewReader("  \n"), def)
	if err != nil || len(empty) != 0 {
		t.Errorf("blank input = %+v %v", empty, err)
	}
}

func TestLedger(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("AUTOM_LEDGER_PATH", filepath.Join(t.TempDir(), "ledger.jsonl"))
	t.Setenv(cliutil.VerifyEnvVar, "")
	recordLedger("GET", "/v1/google/search", map[string]string{"query": "serp api"}, 200)
	recordLedger("GET", "/v1/usage", nil, 200)
	recordLedger("GET", "/v1/bing/search", map[string]string{"query": "x"}, 202)
	entries, err := readLedger(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	if entries[0].Credits != 1 || entries[0].Engine != "google" || entries[0].Query != "serp api" {
		t.Errorf("billed search entry = %+v", entries[0])
	}
	if entries[1].Credits != 0 {
		t.Errorf("usage is free, got %+v", entries[1])
	}
	if entries[2].Credits != 0 || !entries[2].Pending {
		t.Errorf("async 202 must be pending, not billed: %+v", entries[2])
	}
	if later, _ := readLedger(time.Now().Add(time.Hour)); len(later) != 0 {
		t.Errorf("since filter ignored: %d entries", len(later))
	}
	if _, err := os.Stat(os.Getenv("AUTOM_LEDGER_PATH")); err != nil {
		t.Errorf("ledger file not written: %v", err)
	}
}

func TestAutomRateLimit401Classification(t *testing.T) {
	err := classifyAPIErrorOnly(errString(`GET /v1/google/search returned HTTP 401: {"error":"Rate limit exceeded for the API key: quota monthly"}`))
	if ExitCode(err) != 7 || !strings.Contains(err.Error(), "quota") {
		t.Errorf("quota 401 should exit 7 with a quota hint, got %d: %v", ExitCode(err), err)
	}
	bad := classifyAPIErrorOnly(errString(`GET /v1/google/search returned HTTP 401: {"error":"Invalid API Key"}`))
	if ExitCode(bad) == 7 {
		t.Errorf("an invalid key must stay an auth error, got exit %d", ExitCode(bad))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func mustJSON(t *testing.T, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
}

func TestAllFailedErr(t *testing.T) {
	fails := []fetchFailure{{Source: "google", Error: "x"}, {Source: "bing", Error: "y"}}
	if got := allFailedErr("engine", fails, []error{authErr(errString("a")), authErr(errString("b"))}); ExitCode(got) != ExitCode(authErr(nil)) {
		t.Errorf("same-code failures should keep the auth exit, got %d", ExitCode(got))
	}
	if got := allFailedErr("engine", fails, []error{authErr(errString("a")), rateLimitErr(errString("b"))}); ExitCode(got) != ExitCode(apiErr(nil)) {
		t.Errorf("mixed failures should be a generic API error, got %d", ExitCode(got))
	}
	if got := allFailedErr("engine", nil, nil); ExitCode(got) != ExitCode(apiErr(nil)) {
		t.Errorf("no errors should be a generic API error, got %d", ExitCode(got))
	}
}

func TestRateLimit401NotQuota(t *testing.T) {
	err := classifyAPIErrorOnly(errString(`GET /v1/google/search returned HTTP 401: {"error":"Rate limit exceeded: You've exceeded the per second rate limit on your subscription"}`))
	if ExitCode(err) != 7 || strings.Contains(err.Error(), "quota") {
		t.Errorf("per-second rate limit should exit 7 with the retry hint, got %d: %v", ExitCode(err), err)
	}
}

func TestBatchKeysAndHeader(t *testing.T) {
	a := batchItem{Query: "Serp API", Engine: "google", Country: "FR", Lang: "fr", Domain: "a.com"}
	b := batchItem{Query: "serp api", Engine: "google", Country: "fr", Lang: "fr", Domain: "b.com"}
	c := batchItem{Query: "serp api", Engine: "google", Country: "fr", Lang: "en"}
	if a.key() != b.key() {
		t.Error("rows differing only by domain or case must share one search")
	}
	if a.key() == c.key() {
		t.Error("rows differing by lang must not share a search")
	}
	for line, want := range map[string]bool{"keyword,domain": true, `"Query", engine`: true, "best keyword tool, free": false, "plain keyword": false} {
		if got := isBatchCSVHeader(line); got != want {
			t.Errorf("isBatchCSVHeader(%q) = %v, want %v", line, got, want)
		}
	}
	rows, err := parseBatchInput(strings.NewReader("best keyword tool, free\nother\n"), batchItem{Engine: "google"})
	if err != nil || len(rows) != 2 || rows[0].Query != "best keyword tool, free" {
		t.Errorf("a keyword containing a comma must stay a keyword: %+v %v", rows, err)
	}
	if _, err := parseBatchInput(strings.NewReader(strings.Repeat("k\n", maxBatchInput)), batchItem{}); err == nil {
		t.Error("input over the size cap must be refused")
	}
}

func TestNewsScopes(t *testing.T) {
	art := serp.Result{URL: "https://news.com/a", Source: "News", Title: "Story"}
	k1 := scopedKeys(newsScope("Autom", "fr", ""), serp.NewsKeys(art))
	k2 := scopedKeys(newsScope("serp api", "fr", ""), serp.NewsKeys(art))
	k3 := scopedKeys(newsScope(" autom ", "FR", ""), serp.NewsKeys(art))
	if k1[0] == k2[0] {
		t.Error("two watches must keep separate memories")
	}
	if k1[0] != k3[0] {
		t.Error("the same watch spelled differently must share its memory")
	}
}

func TestJobIDPattern(t *testing.T) {
	for id, ok := range map[string]bool{"6f1c2b8e-1d2a-4c7f-9a51-0f3e2b7d9c10": true, "../google/search?query=x": false, "a/b": false, "": false} {
		if jobIDPattern.MatchString(id) != ok {
			t.Errorf("job id %q accepted=%v, want %v", id, !ok, ok)
		}
	}
	if _, err := runCLI(t, "jobs", "get", "../usage"); err == nil || ExitCode(err) != 2 {
		t.Errorf("a path-like job id must be a usage error, got %v", err)
	}
}

func TestSerpDiffFlagsAndKeys(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{"k": {res("a.com"), res("b.com")}})
	if _, err := runCLI(t, "serp", "diff", "--from", "1", "--db", db, "--json"); err == nil || ExitCode(err) != 2 {
		t.Errorf("--from without --to must be a usage error, got %v", err)
	}
	// A newer page of the same query on another engine must not be diffed
	// against the google pages.
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sdb.SaveSerpSnapshot(context.Background(), store.SerpSnapshot{Query: "k", Engine: "bing", TakenAt: time.Now().Add(time.Hour)}, res("z.com")); err != nil {
		t.Fatal(err)
	}
	_ = sdb.Close()
	out, err := runCLI(t, "serp", "diff", "--query", "k", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v serpDiffView
	mustJSON(t, out, &v)
	if len(v.Changes) != 0 || v.Note == "" || v.Engine == "google" {
		t.Errorf("a single bing page must not be diffed against google pages: %+v", v)
	}
	out, err = runCLI(t, "serp", "diff", "--query", "k", "--engine", "g", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var g serpDiffView
	mustJSON(t, out, &g)
	if g.Engine != "google" || len(g.Changes) == 0 {
		t.Errorf("--engine alias g should diff the two google pages: %+v", g)
	}
}

func TestRankCompetitorsSameKey(t *testing.T) {
	db := seedSerpDB(t, map[string][2][]serp.Result{"k": {res("a.com", "b.com"), res("b.com", "c.com")}})
	// A newer page of the same query without a country must not be compared
	// with the fr pages.
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sdb.SaveSerpSnapshot(context.Background(), store.SerpSnapshot{Query: "k", Engine: "google", TakenAt: time.Now().Add(time.Hour)}, res("z.com")); err != nil {
		t.Fatal(err)
	}
	_ = sdb.Close()
	out, err := runCLI(t, "rank", "competitors", "--query", "k", "--country", "fr", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v competitorsView
	mustJSON(t, out, &v)
	if v.Country != "fr" || v.Latest == nil || v.Previous == nil || len(v.Top) != 2 {
		t.Fatalf("fr competitors should compare the two fr pages: %+v", v)
	}
	if v.Top[0].Domain != "b.com" || v.Top[0].Kind != "up" || v.Top[1].Kind != "entered" || len(v.Left) != 1 {
		t.Errorf("unexpected moves: top %+v left %+v", v.Top, v.Left)
	}
	out, err = runCLI(t, "rank", "competitors", "--query", "k", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var single competitorsView
	mustJSON(t, out, &single)
	if single.Country != "" || single.Previous != nil || single.Note == "" || single.Top[0].Kind != "" {
		t.Errorf("the lone country-less page has no previous page and no moves: %+v", single)
	}
}

func TestPrintThenFail(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	flags := &rootFlags{asJSON: true}
	err := printThenFail(cmd, flags, map[string]any{"fetch_failures": []string{"google"}}, authErr(errString("no key")))
	if ExitCode(err) != ExitCode(authErr(nil)) {
		t.Errorf("error code not kept: %v", err)
	}
	var v map[string]any
	mustJSON(t, out.String(), &v)
	if _, ok := v["fetch_failures"]; !ok {
		t.Errorf("JSON view not printed before failing: %s", out.String())
	}
}

func TestDomainArg(t *testing.T) {
	for in, want := range map[string]string{"autom.dev": "autom.dev", "https://www.Autom.dev/pricing": "autom.dev", "docs.example.co.uk": "docs.example.co.uk"} {
		if got, err := domainArg(in, false); err != nil || got != want {
			t.Errorf("domainArg(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"__printing_press_invalid__", "localhost", "a..b", "-bad.com"} {
		if _, err := domainArg(bad, false); err == nil || ExitCode(err) != 2 {
			t.Errorf("domainArg(%q) should be a usage error, got %v", bad, err)
		}
	}
	if d, err := domainArg("", true); err != nil || d != "" {
		t.Errorf("empty optional domain = %q, %v", d, err)
	}
	if _, err := runCLI(t, "visibility", "__printing_press_invalid__", "--json"); ExitCode(err) != 2 {
		t.Errorf("visibility with an invalid domain must exit 2, got %v", err)
	}
}
