// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestRefdomainFromURL(t *testing.T) {
	tests := map[string]string{
		"https://www.example.com/path": "example.com",
		"http://Sub.Example.com/a":     "sub.example.com",
		"blog.example.com/page":        "blog.example.com",
		"":                             "",
	}
	for input, want := range tests {
		if got := refdomainFromURL(input); got != want {
			t.Fatalf("refdomainFromURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSnapshotCompactResult(t *testing.T) {
	result := snapshotResult{
		Authority: map[string]any{"domain_rating": 76.5},
		Backlinks: map[string]any{"live_refdomains": 1234},
		Organic:   map[string]any{"org_traffic": 98765},
	}
	compact := compactSnapshotResult(result)
	if compact["domain_rating"] != 76.5 {
		t.Fatalf("domain_rating = %v, want 76.5", compact["domain_rating"])
	}
	if compact["live_refdomains"] != 1234 {
		t.Fatalf("live_refdomains = %v, want 1234", compact["live_refdomains"])
	}
	if compact["org_traffic"] != 98765 {
		t.Fatalf("org_traffic = %v, want 98765", compact["org_traffic"])
	}
}

func TestCompositeCommandsRegisteredReadOnly(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"keyword-gap", "striking-distance", "link-intersect", "snapshot"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("finding %s: %v", name, err)
		}
		if cmd == nil || cmd.Use != name {
			t.Fatalf("root.Find(%s) returned %#v", name, cmd)
		}
		if got := cmd.Annotations["mcp:read-only"]; got != "true" {
			t.Fatalf("%s mcp:read-only annotation = %q, want true", name, got)
		}
	}
}

func TestKeywordGapCommandComposesOrganicKeywordRows(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		if r.URL.Path != "/site-explorer/organic-keywords" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("select"); got != organicKeywordsCompositeSelect {
			t.Fatalf("select = %q, want %q", got, organicKeywordsCompositeSelect)
		}
		switch r.URL.Query().Get("target") {
		case "ours.example":
			fmt.Fprint(w, `[
				{"keyword":"owned","volume":900,"keyword_difficulty":20,"best_position":5,"sum_traffic":80},
				{"keyword":"weak","volume":500,"keyword_difficulty":30,"best_position":15,"sum_traffic":20}
			]`)
		case "comp.example":
			fmt.Fprint(w, `[
				{"keyword":"owned","volume":900,"keyword_difficulty":20,"best_position":3,"best_position_url":"https://comp.example/owned","sum_traffic":200,"cpc":4},
				{"keyword":"weak","volume":500,"keyword_difficulty":30,"best_position":4,"best_position_url":"https://comp.example/weak","sum_traffic":90,"cpc":7},
				{"keyword":"new","volume":700,"keyword_difficulty":35,"best_position":2,"best_position_url":"https://comp.example/new","sum_traffic":130,"cpc":9},
				{"keyword":"too-low","volume":10,"keyword_difficulty":5,"best_position":1,"sum_traffic":1}
			]`)
		default:
			t.Fatalf("unexpected target: %s", r.URL.Query().Get("target"))
		}
	}))
	defer srv.Close()

	out := runCompositeCommand(t, srv.URL, "keyword-gap", "--target", "ours.example", "--competitor", "comp.example", "--min-volume", "100", "--max-difficulty", "40", "--json", "--no-cache", "--data-source", "live")
	var env struct {
		Results []keywordGapResult `json:"results"`
	}
	mustUnmarshal(t, out, &env)
	if len(env.Results) != 2 {
		t.Fatalf("expected 2 gap rows, got %d: %s", len(env.Results), out)
	}
	if env.Results[0].Keyword != "new" || env.Results[0].BestCompetitor != "comp.example" {
		t.Fatalf("top row = %+v, want new keyword from comp.example", env.Results[0])
	}
	if env.Results[1].Keyword != "weak" || env.Results[1].YourPosition == nil || *env.Results[1].YourPosition != 15 {
		t.Fatalf("second row = %+v, want weak with your_position 15", env.Results[1])
	}
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d: %v", len(requests), requests)
	}
}

func TestStrikingDistanceCommandSortsOpportunity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/site-explorer/organic-keywords" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		where := r.URL.Query().Get("where")
		for _, want := range []string{"best_position >= 4", "best_position <= 15", "volume >= 100"} {
			if !strings.Contains(where, want) {
				t.Fatalf("where %q missing %q", where, want)
			}
		}
		fmt.Fprint(w, `[
			{"keyword":"pos4","volume":100,"keyword_difficulty":10,"best_position":4,"sum_traffic":20},
			{"keyword":"pos10","volume":1000,"keyword_difficulty":15,"best_position":10,"sum_traffic":50},
			{"keyword":"pos16","volume":5000,"keyword_difficulty":5,"best_position":16,"sum_traffic":1},
			{"keyword":"low-volume","volume":10,"keyword_difficulty":5,"best_position":5,"sum_traffic":1}
		]`)
	}))
	defer srv.Close()

	out := runCompositeCommand(t, srv.URL, "striking-distance", "--target", "ours.example", "--min-position", "4", "--max-position", "15", "--min-volume", "100", "--json", "--no-cache", "--data-source", "live")
	var env struct {
		Results []strikingDistanceResult `json:"results"`
	}
	mustUnmarshal(t, out, &env)
	if got := len(env.Results); got != 2 {
		t.Fatalf("expected 2 striking-distance rows, got %d: %s", got, out)
	}
	if env.Results[0].Keyword != "pos10" {
		t.Fatalf("first keyword = %s, want pos10 by opportunity", env.Results[0].Keyword)
	}
	if env.Results[1].Keyword != "pos4" {
		t.Fatalf("second keyword = %s, want pos4", env.Results[1].Keyword)
	}
}

func TestLinkIntersectCommandDerivesRefdomains(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/site-explorer/all-backlinks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("aggregation"); got != "1_per_domain" {
			t.Fatalf("aggregation = %q, want 1_per_domain", got)
		}
		switch r.URL.Query().Get("target") {
		case "ours.example":
			fmt.Fprint(w, `[{"url_from":"https://already-links.example/post","domain_rating_source":70}]`)
		case "comp-a.example":
			fmt.Fprint(w, `[
				{"url_from":"https://www.gap-source.example/a","domain_rating_source":40,"first_seen":"2026-01-01","traffic_domain":100},
				{"url_from":"https://already-links.example/competitor","domain_rating_source":80}
			]`)
		case "comp-b.example":
			fmt.Fprint(w, `[
				{"url_from":"http://gap-source.example/b","domain_rating_source":55,"first_seen":"2026-02-01","traffic_domain":200},
				{"url_from":"https://one-off.example/page","domain_rating_source":90}
			]`)
		default:
			t.Fatalf("unexpected target: %s", r.URL.Query().Get("target"))
		}
	}))
	defer srv.Close()

	out := runCompositeCommand(t, srv.URL, "link-intersect", "--target", "ours.example", "--competitor", "comp-a.example", "--competitor", "comp-b.example", "--min-competitors", "2", "--min-dr", "30", "--json", "--no-cache", "--data-source", "live")
	var env struct {
		Results []linkIntersectResult `json:"results"`
	}
	mustUnmarshal(t, out, &env)
	if len(env.Results) != 1 {
		t.Fatalf("expected one intersect row, got %d: %s", len(env.Results), out)
	}
	got := env.Results[0]
	if got.Refdomain != "gap-source.example" || got.DomainRatingSource != 55 || got.ExampleURL != "http://gap-source.example/b" {
		t.Fatalf("intersect row = %+v", got)
	}
	if !slices.Equal(got.CompetitorsLinking, []string{"comp-a.example", "comp-b.example"}) {
		t.Fatalf("competitors = %v", got.CompetitorsLinking)
	}
}

func TestSnapshotCommandReturnsWarningsOnPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/site-explorer/domain-rating":
			fmt.Fprint(w, `{"domain_rating":76.5,"ahrefs_rank":12345}`)
		case "/site-explorer/backlinks-stats":
			http.Error(w, `{"error":"bad date"}`, http.StatusBadRequest)
		case "/site-explorer/metrics":
			fmt.Fprint(w, `{"org_keywords":1000,"org_traffic":20000,"org_cost":3000,"paid_keywords":40,"paid_traffic":500}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	out := runCompositeCommand(t, srv.URL, "snapshot", "--target", "ours.example", "--country", "us", "--date", "2026-06-03", "--json", "--no-cache", "--data-source", "live")
	var env struct {
		Results snapshotResult `json:"results"`
	}
	mustUnmarshal(t, out, &env)
	if env.Results.Authority["domain_rating"] != 76.5 {
		t.Fatalf("authority = %+v", env.Results.Authority)
	}
	if env.Results.Backlinks != nil {
		t.Fatalf("backlinks = %+v, want nil after partial failure", env.Results.Backlinks)
	}
	if env.Results.Organic["org_traffic"] != float64(20000) {
		t.Fatalf("organic = %+v", env.Results.Organic)
	}
	if len(env.Results.Warnings) != 1 || !strings.Contains(env.Results.Warnings[0], "backlinks-stats") {
		t.Fatalf("warnings = %+v", env.Results.Warnings)
	}
}

func runCompositeCommand(t *testing.T, baseURL string, args ...string) []byte {
	t.Helper()
	t.Setenv("AHREFS_BASE_URL", baseURL)
	t.Setenv("AHREFS_API_KEY", "test-key")
	t.Setenv("AHREFS_CONFIG", t.TempDir()+"/missing.toml")
	root := RootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\nstderr:\n%s\nstdout:\n%s", args, err, stderr.String(), stdout.String())
	}
	return stdout.Bytes()
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
}
