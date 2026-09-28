// Copyright 2026 googio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/serply/internal/platform"
)

func TestParseSerpResultsWeb(t *testing.T) {
	data := json.RawMessage(`{"results":[
		{"title":"A","link":"https://www.example.com/a","description":"first"},
		{"title":"no link"},
		{"title":"B","link":"https://docs.example.com/b"}]}`)
	got, err := parseSerpResults(data, "results")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 results, got %d", len(got))
	}
	if got[0].Position != 1 || got[0].Domain != "example.com" || got[0].Snippet != "first" {
		t.Errorf("first result wrong: %+v", got[0])
	}
	if got[1].Position != 2 || got[1].Domain != "docs.example.com" {
		t.Errorf("second result wrong: %+v", got[1])
	}
}

func TestParseSerpResultsNewsSourceAndMissingKey(t *testing.T) {
	data := json.RawMessage(`{"entries":[{"title":"N","link":"https://news.test/x","published":"Sat, 27 Sep 2026","source":{"title":"News Test"}}]}`)
	got, err := parseSerpResults(data, "entries")
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	if got[0].Author != "News Test" || got[0].Published == "" {
		t.Errorf("news fields not mapped: %+v", got[0])
	}
	redirect := json.RawMessage(`{"entries":[{"title":"N","link":"https://news.google.com/rss/articles/abc","source":{"href":"https://www.nature.com","title":"Nature"}}]}`)
	got, err = parseSerpResults(redirect, "entries")
	if err != nil || len(got) != 1 || got[0].Domain != "nature.com" {
		t.Errorf("news publisher domain should come from source.href: %+v, %v", got, err)
	}
	scholar := json.RawMessage(`{"articles":[{"title":"P","link":"https://dl.acm.org/doi/1","author":{"names":"A Salemi, H Zamani - SIGIR, 2024","authors":[{"name":"A Salemi"}]}}]}`)
	got, err = parseSerpResults(scholar, "articles")
	if err != nil || len(got) != 1 || got[0].Author != "A Salemi, H Zamani - SIGIR, 2024" {
		t.Errorf("scholar author object not mapped: %+v, %v", got, err)
	}
	empty, err := parseSerpResults(json.RawMessage(`{"feed":{}}`), "entries")
	if err != nil || len(empty) != 0 {
		t.Fatalf("missing key should be empty, got %v, %v", empty, err)
	}
}

func TestNormalizeDomainAndMatch(t *testing.T) {
	cases := map[string]string{
		"GitHub.com":                 "github.com",
		"www.github.com":             "github.com",
		"https://www.github.com/a/b": "github.com",
		"github.com/features":        "github.com",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Errorf("normalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
	if !domainMatches("docs.github.com", "github.com") {
		t.Error("subdomain should match")
	}
	if domainMatches("notgithub.com", "github.com") {
		t.Error("suffix without a dot must not match")
	}
}

func TestRankDomain(t *testing.T) {
	results := []serpResult{
		{Position: 1, Link: "https://a.test", Domain: "a.test"},
		{Position: 2, Link: "https://blog.b.test/x", Domain: "blog.b.test"},
		{Position: 3, Link: "https://b.test/y", Domain: "b.test"},
	}
	got := rankDomain(results, "b.test")
	if len(got) != 2 || got[0].Position != 2 || got[1].Position != 3 {
		t.Fatalf("unexpected matches: %+v", got)
	}
	if none := rankDomain(results, "c.test"); len(none) != 0 {
		t.Fatalf("want no matches, got %+v", none)
	}
}

func TestDiffSerps(t *testing.T) {
	baseline := []serpResult{
		{Position: 1, Link: "https://a.test/"},
		{Position: 2, Link: "https://b.test"},
		{Position: 3, Link: "https://c.test"},
	}
	current := []serpResult{
		{Position: 1, Link: "https://www.b.test"},
		{Position: 2, Link: "https://a.test"},
		{Position: 3, Link: "https://d.test"},
	}
	entered, left, moved, unchanged := diffSerps(baseline, current)
	if len(entered) != 1 || entered[0].Link != "https://d.test" {
		t.Errorf("entered: %+v", entered)
	}
	if len(left) != 1 || left[0].Link != "https://c.test" || left[0].Position != 3 {
		t.Errorf("left: %+v", left)
	}
	if len(moved) != 2 || moved[0].From != 2 || moved[0].To != 1 || moved[0].Delta != 1 {
		t.Errorf("moved: %+v", moved)
	}
	if unchanged != 0 {
		t.Errorf("unchanged: %d", unchanged)
	}
	e, l, m, u := diffSerps(baseline, baseline)
	if len(e)+len(l)+len(m) != 0 || u != 3 {
		t.Errorf("identical runs should be unchanged: %v %v %v %d", e, l, m, u)
	}
}

func TestSerpSnapshotRoundTripAndCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serp-snapshots", "k.json")
	f, err := loadSerpSnapshots(path)
	if err != nil || len(f.Snapshots) != 0 {
		t.Fatalf("missing file should load empty: %v %v", f, err)
	}
	for i := 0; i < maxSerpSnapshots+3; i++ {
		f.Snapshots = append(f.Snapshots, serpSnapshot{Query: "q", TakenAt: time.Unix(int64(i), 0).UTC()})
	}
	if err := saveSerpSnapshots(path, f); err != nil {
		t.Fatal(err)
	}
	back, err := loadSerpSnapshots(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Snapshots) != maxSerpSnapshots || back.Snapshots[0].TakenAt.Unix() != 3 {
		t.Fatalf("cap not applied: %d snapshots, first %v", len(back.Snapshots), back.Snapshots[0].TakenAt)
	}
}

func TestSerpSnapshotPathSeparatesLocation(t *testing.T) {
	t.Setenv("SERPLY_HOME", t.TempDir())
	us, err := serpSnapshotPath("Serp API", serpOptions{Location: "us"}, "")
	if err != nil {
		t.Fatal(err)
	}
	gb, _ := serpSnapshotPath("serp api", serpOptions{Location: "GB"}, "")
	us2, _ := serpSnapshotPath("serp api ", serpOptions{Location: "US"}, "")
	if us == gb {
		t.Error("different locations must not share a snapshot file")
	}
	if us != us2 {
		t.Error("query case, spacing and location case should not split history")
	}
}

func TestSerpSnapshotPathSeparatesDepth(t *testing.T) {
	t.Setenv("SERPLY_HOME", t.TempDir())
	shallow, err := serpSnapshotPath("serp api", serpOptions{Num: 5, Location: "US"}, "")
	if err != nil {
		t.Fatal(err)
	}
	deep, err := serpSnapshotPath("serp api", serpOptions{Num: 10, Location: "US"}, "")
	if err != nil {
		t.Fatal(err)
	}
	same, err := serpSnapshotPath("serp api", serpOptions{Num: 5, Location: "us"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if shallow == deep {
		t.Fatal("different --num depths must not share a snapshot file")
	}
	if shallow != same {
		t.Fatal("the same depth and location should share a snapshot file")
	}
}

func TestSerpSnapshotPathSeparatesClientProfile(t *testing.T) {
	t.Setenv("SERPLY_HOME", t.TempDir())
	opts := serpOptions{Num: 10, Location: "US"}
	tenantA, err := serpSnapshotPath("serp api", opts, "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	tenantB, err := serpSnapshotPath("serp api", opts, "tenant-b")
	if err != nil {
		t.Fatal(err)
	}
	shared, err := serpSnapshotPath("serp api", opts, "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := serpSnapshotPath("serp api", opts, " tenant-a ")
	if err != nil {
		t.Fatal(err)
	}
	if tenantA == tenantB || tenantA == shared {
		t.Fatal("client profiles must not share a serp snapshot file")
	}
	if tenantA != again {
		t.Fatal("profile whitespace should not split history")
	}
}

func TestActiveClientProfileResolution(t *testing.T) {
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "from-env")
	if got := activeClientProfile(&rootFlags{}); got != "from-env" {
		t.Fatalf("env profile = %q", got)
	}
	if got := activeClientProfile(&rootFlags{clientProfileName: "from-flag"}); got != "from-flag" {
		t.Fatalf("flag profile = %q", got)
	}
	flags := &rootFlags{
		clientProfileName: "from-flag",
		platformSession:   &platform.Session{ProfileName: "from-session"},
	}
	if got := activeClientProfile(flags); got != "from-session" {
		t.Fatalf("session profile = %q", got)
	}
}

func TestRecordSerpSnapshotConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serp-snapshots", "k.json")
	const n = 8
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := recordSerpSnapshot(path, serpSnapshot{
				Query:   "q",
				TakenAt: time.Unix(int64(i+1), 0).UTC(),
				Results: []serpResult{{Link: fmt.Sprintf("https://example.test/%d", i)}},
			})
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
	back, err := loadSerpSnapshots(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Snapshots) != n {
		t.Fatalf("concurrent appends kept %d snapshots, want %d", len(back.Snapshots), n)
	}
	seen := map[string]bool{}
	for _, snap := range back.Snapshots {
		if len(snap.Results) != 1 {
			t.Fatalf("snapshot missing its result: %+v", snap)
		}
		seen[snap.Results[0].Link] = true
	}
	if len(seen) != n {
		t.Fatalf("concurrent appends collapsed distinct runs: %v", seen)
	}
}

func TestParseResearchVerticals(t *testing.T) {
	got, err := parseResearchVerticals("")
	if err != nil || strings.Join(got, ",") != "web,news,scholar" {
		t.Fatalf("default: %v %v", got, err)
	}
	got, err = parseResearchVerticals(" Scholar,news,scholar ")
	if err != nil || strings.Join(got, ",") != "scholar,news" {
		t.Fatalf("subset: %v %v", got, err)
	}
	if _, err := parseResearchVerticals("web,maps"); err == nil {
		t.Fatal("unknown vertical should fail")
	}
}

func TestMergeResearchDedupesAcrossVerticals(t *testing.T) {
	by := map[string][]serpResult{
		"web":     {{Link: "https://x.test/p"}, {Link: "https://y.test"}},
		"news":    {{Link: "https://www.x.test/p/"}, {Link: "https://z.test"}},
		"scholar": {},
	}
	sources, counts := mergeResearch([]string{"web", "news", "scholar"}, by)
	if len(sources) != 3 {
		t.Fatalf("want 3 unique sources, got %d: %+v", len(sources), sources)
	}
	if sources[2].N != 3 || sources[2].Vertical != "news" || sources[2].Link != "https://z.test" {
		t.Errorf("numbering or order wrong: %+v", sources[2])
	}
	if counts["web"] != 2 || counts["news"] != 1 || counts["scholar"] != 0 {
		t.Errorf("counts: %v", counts)
	}
	md := renderResearchMarkdown(researchView{Topic: "t", Verticals: []string{"web", "news", "scholar"}, Sources: sources})
	for _, want := range []string{"# Research: t", "## News", "[3] https://z.test", "## Scholar\n\n- no results"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
