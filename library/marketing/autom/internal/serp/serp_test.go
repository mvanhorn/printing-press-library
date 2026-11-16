// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

package serp

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestLookupEngine(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"google", "google", false},
		{" Bing ", "bing", false},
		{"ddg", "duckduckgo", false},
		{"g", "google", false},
		{"yandex", "", true},
	}
	for _, tt := range tests {
		e, err := LookupEngine(tt.in)
		if (err != nil) != tt.err {
			t.Fatalf("LookupEngine(%q) err=%v, want err=%v", tt.in, err, tt.err)
		}
		if e.Name != tt.want {
			t.Errorf("LookupEngine(%q) = %q, want %q", tt.in, e.Name, tt.want)
		}
	}
}

func TestParseEngineList(t *testing.T) {
	all, err := ParseEngineList("")
	if err != nil || len(all) != len(Engines) {
		t.Fatalf("empty list = %d engines, err %v; want all %d", len(all), err, len(Engines))
	}
	got, err := ParseEngineList("bing, google,bing")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "bing" || got[1].Name != "google" {
		t.Errorf("dedupe/order wrong: %+v", got)
	}
	if _, err := ParseEngineList("google,nope"); err == nil {
		t.Error("unknown engine accepted")
	}
}

func TestEngineParams(t *testing.T) {
	g, _ := LookupEngine("google")
	if got := g.Params("coffee", "FR", "FR", 2); !reflect.DeepEqual(got, map[string]string{"query": "coffee", "gl": "fr", "hl": "fr", "page": "2"}) {
		t.Errorf("google params = %v", got)
	}
	b, _ := LookupEngine("bing")
	got := b.Params("coffee", "fr", "fr", 3)
	want := map[string]string{"query": "coffee", "cc": "FR", "mkt": "fr-FR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bing params = %v, want %v (no page: Bing fails on page)", got, want)
	}
}

func TestParseOrganic(t *testing.T) {
	raw := json.RawMessage(`{"organic_results":[
		{"position":2,"title":" B ","link":"https://www.b.com/x","domain":"","snippet":"s2"},
		{"position":1,"title":"A","link":"https://a.com/","domain":"www.A.com","snippet":"s1"},
		{"position":3,"title":"no link","link":""}
	]}`)
	got, err := ParseOrganic(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 results (link-less dropped), got %d", len(got))
	}
	if got[0].Position != 1 || got[0].Domain != "a.com" || got[1].Domain != "b.com" || got[1].Title != "B" {
		t.Errorf("unexpected parse: %+v", got)
	}
	empty, err := ParseOrganic(json.RawMessage(`{"search_parameters":{}}`))
	if err != nil || len(empty) != 0 || empty == nil {
		t.Errorf("no organic_results should give empty non-nil slice, got %v %v", empty, err)
	}
	if _, err := ParseOrganic(json.RawMessage(`not json`)); err == nil {
		t.Error("invalid JSON accepted")
	}
}

func TestNormalizeDomainAndMatch(t *testing.T) {
	cases := map[string]string{
		"https://WWW.Example.com/path?q=1": "example.com",
		"blog.example.com":                 "blog.example.com",
		"example.com:443/a":                "example.com",
		"":                                 "",
	}
	for in, want := range cases {
		if got := NormalizeDomain(in); got != want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", in, got, want)
		}
	}
	if !DomainMatches("docs.autom.dev", "autom.dev") || DomainMatches("notautom.dev", "autom.dev") {
		t.Error("subdomain matching wrong")
	}
}

func TestRankOf(t *testing.T) {
	rs := []Result{{Position: 1, Domain: "a.com", URL: "https://a.com"}, {Position: 4, Domain: "docs.b.com", URL: "https://docs.b.com/x"}}
	if p, u := RankOf(rs, "b.com"); p != 4 || u != "https://docs.b.com/x" {
		t.Errorf("RankOf b.com = %d %s", p, u)
	}
	if p, _ := RankOf(rs, "c.com"); p != 0 {
		t.Errorf("absent domain should be 0, got %d", p)
	}
}

func TestComputeOverlap(t *testing.T) {
	sets := map[string][]Result{
		"google": {{Domain: "a.com"}, {Domain: "b.com"}, {Domain: "c.com"}},
		"bing":   {{Domain: "a.com"}, {Domain: "d.com"}},
	}
	ov := ComputeOverlap([]string{"google", "bing"}, sets)
	if !reflect.DeepEqual(ov.Shared, []string{"a.com"}) {
		t.Errorf("shared = %v", ov.Shared)
	}
	if !reflect.DeepEqual(ov.Unique["google"], []string{"b.com", "c.com"}) || !reflect.DeepEqual(ov.Unique["bing"], []string{"d.com"}) {
		t.Errorf("unique = %v", ov.Unique)
	}
	if ov.Jaccard["google|bing"] != 0.25 {
		t.Errorf("jaccard = %v, want 0.25", ov.Jaccard["google|bing"])
	}
	empty := ComputeOverlap([]string{"google", "bing"}, map[string][]Result{})
	if len(empty.Shared) != 0 || empty.Jaccard["google|bing"] != 0 {
		t.Errorf("empty sets should share nothing: %+v", empty)
	}
}

func TestDiff(t *testing.T) {
	before := []Result{{Position: 1, URL: "https://a.com/"}, {Position: 2, URL: "https://b.com"}, {Position: 3, URL: "https://c.com"}}
	after := []Result{{Position: 1, URL: "https://b.com/"}, {Position: 2, URL: "https://a.com"}, {Position: 3, URL: "https://d.com"}}
	got := Diff(before, after, 0)
	kinds := map[string]string{}
	for _, c := range got {
		kinds[c.URL] = c.Kind
	}
	want := map[string]string{"https://b.com/": "up", "https://a.com": "down", "https://d.com": "entered", "https://c.com": "left"}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("diff kinds = %v, want %v", kinds, want)
	}
	same := Diff(before, before, 0)
	for _, c := range same {
		if c.Kind != "same" || c.Delta != 0 {
			t.Errorf("identical snapshots must not report drift: %+v", c)
		}
	}
	if top := Diff(before, after, 1); len(top) != 2 {
		t.Errorf("top-1 diff should compare only first positions, got %+v", top)
	}
}

func TestMergeSources(t *testing.T) {
	lists := [][]Result{
		{{Position: 1, URL: "https://a.com/x?utm_source=g", Title: "A"}, {Position: 2, URL: "https://b.com", Title: "B"}},
		{{Position: 3, URL: "https://www.a.com/x/", Title: "A again"}, {Position: 1, URL: "https://c.com", Title: "C"}},
	}
	got := MergeSources([]string{"google", "bing"}, lists, 0)
	if len(got) != 3 {
		t.Fatalf("want 3 unique sources, got %d: %+v", len(got), got)
	}
	if got[0].URL != "https://a.com/x?utm_source=g" || !reflect.DeepEqual(got[0].FoundIn, []string{"google", "bing"}) || got[0].Best != 1 || got[0].N != 1 {
		t.Errorf("first source should be the cross-engine one: %+v", got[0])
	}
	if limited := MergeSources([]string{"google", "bing"}, lists, 2); len(limited) != 2 {
		t.Errorf("limit not applied: %d", len(limited))
	}
}

func TestNewsKeys(t *testing.T) {
	a := Result{URL: "https://news.com/story?utm_medium=x", Source: "News", Title: "Big Story"}
	b := Result{URL: "https://news.com/story", Source: "Other", Title: "Different"}
	c := Result{URL: "https://news.com/story-v2", Source: "news", Title: "big story"}
	ka, kb, kc := NewsKeys(a), NewsKeys(b), NewsKeys(c)
	if ka[0] != kb[0] {
		t.Errorf("same canonical URL should share a key: %v %v", ka, kb)
	}
	if ka[1] != kc[1] {
		t.Errorf("same source+title should share a key: %v %v", ka, kc)
	}
	if len(NewsKeys(Result{})) != 0 {
		t.Error("empty article should have no keys")
	}
}

func TestRedirectIdentity(t *testing.T) {
	a := Result{Position: 1, URL: "https://www.google.com/goto?url=CAESWAH&gsessionid=one", Domain: "serpapi.com", Title: "SerpApi: Plans and Pricing"}
	b := Result{Position: 2, URL: "https://www.google.com/goto?url=CAESXYZ&gsessionid=two", Domain: "serpapi.com", Title: "SerpApi:  Plans and Pricing"}
	if !IsRedirectLink(a.URL) || IsRedirectLink("https://serpapi.com/pricing") || !IsRedirectLink("https://www.bing.com/ck/a?u=x") {
		t.Fatal("redirect detection wrong")
	}
	if Identity(a) != Identity(b) {
		t.Error("the same page behind two Google redirect links must have one identity")
	}
	changes := Diff([]Result{a}, []Result{b}, 0)
	if len(changes) != 1 || changes[0].Kind != "down" || changes[0].From != 1 || changes[0].To != 2 {
		t.Errorf("a redirect link must diff as a move, not entered+left: %+v", changes)
	}
	merged := MergeSources([]string{"google", "bing"}, [][]Result{{a}, {{Position: 1, URL: "https://serpapi.com/pricing", Domain: "serpapi.com", Title: "SerpApi: Plans and Pricing"}}}, 0)
	if len(merged) != 2 {
		t.Errorf("a redirect and a direct URL stay separate sources (no target to compare): %+v", merged)
	}
	if keys := NewsKeys(Result{URL: a.URL, Source: "News", Title: "Story"}); len(keys) != 1 || keys[0][:3] != "st:" {
		t.Errorf("a redirect news link must be deduped by outlet and title only: %v", keys)
	}
}
