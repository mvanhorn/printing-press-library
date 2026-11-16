// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

// Package serp holds the pure logic behind the Autom rank-tracking and
// comparison commands: which endpoint serves each engine, how organic results
// are parsed and normalized, and how result sets are compared.
package serp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Engine describes how one Autom search endpoint takes its locale parameters.
type Engine struct {
	Name         string
	Path         string
	CountryParam string // query parameter for a two-letter country code
	LangParam    string // query parameter for a language code ("" when unsupported)
	SupportsPage bool   // false when the endpoint fails on `page` (Bing answers 500)
}

// Engines lists the organic-search endpoints, in the default comparison order.
var Engines = []Engine{
	{Name: "google", Path: "/v1/google/search", CountryParam: "gl", LangParam: "hl", SupportsPage: true},
	{Name: "bing", Path: "/v1/bing/search", CountryParam: "cc", LangParam: "", SupportsPage: false},
	{Name: "brave", Path: "/v1/brave/search", CountryParam: "gl", LangParam: "hl", SupportsPage: true},
	{Name: "duckduckgo", Path: "/v1/duckduckgo/search", CountryParam: "gl", LangParam: "hl", SupportsPage: true},
	{Name: "yahoo", Path: "/v1/yahoo/search", CountryParam: "gl", LangParam: "hl", SupportsPage: true},
}

// EngineNames returns the engine names in default order.
func EngineNames() []string {
	out := make([]string, 0, len(Engines))
	for _, e := range Engines {
		out = append(out, e.Name)
	}
	return out
}

// LookupEngine resolves an engine name, accepting common aliases.
func LookupEngine(name string) (Engine, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "ddg", "duck", "duck duck go":
		n = "duckduckgo"
	case "g":
		n = "google"
	}
	for _, e := range Engines {
		if e.Name == n {
			return e, nil
		}
	}
	return Engine{}, fmt.Errorf("unknown engine %q (use one of: %s)", name, strings.Join(EngineNames(), ", "))
}

// ParseEngineList parses a comma-separated engine list; empty means all engines.
func ParseEngineList(csv string) ([]Engine, error) {
	if strings.TrimSpace(csv) == "" {
		return append([]Engine(nil), Engines...), nil
	}
	seen := map[string]bool{}
	out := make([]Engine, 0)
	for _, part := range strings.Split(csv, ",") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		e, err := LookupEngine(part)
		if err != nil {
			return nil, err
		}
		if !seen[e.Name] {
			seen[e.Name] = true
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no engine given")
	}
	return out, nil
}

// Params builds the query parameters for one search.
func (e Engine) Params(query, country, lang string, page int) map[string]string {
	p := map[string]string{"query": query}
	// Bing's mkt (market) and cc (country) cannot be combined: with both a
	// country and a language, send the market alone; cc only without one.
	if e.Name == "bing" && country != "" && lang != "" && e.LangParam == "" {
		p["mkt"] = strings.ToLower(lang) + "-" + strings.ToUpper(country)
		if page > 1 && e.SupportsPage {
			p["page"] = fmt.Sprintf("%d", page)
		}
		return p
	}
	if country != "" && e.CountryParam != "" {
		c := strings.ToLower(country)
		if e.CountryParam == "cc" {
			c = strings.ToUpper(country)
		}
		p[e.CountryParam] = c
	}
	if lang != "" && e.LangParam != "" {
		p[e.LangParam] = strings.ToLower(lang)
	}
	if page > 1 && e.SupportsPage {
		p["page"] = fmt.Sprintf("%d", page)
	}
	return p
}

// Result is one organic result, normalized across engines.
type Result struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Domain   string `json:"domain"`
	Snippet  string `json:"snippet,omitempty"`
	Source   string `json:"source,omitempty"`
	Date     string `json:"date,omitempty"`
}

type rawResult struct {
	Position json.Number `json:"position"`
	Title    string      `json:"title"`
	Link     string      `json:"link"`
	Domain   string      `json:"domain"`
	Snippet  string      `json:"snippet"`
	Source   string      `json:"source"`
	Date     string      `json:"date"`
}

// ParseOrganic extracts organic_results from an Autom search or news response.
// Results without a link are dropped. Missing positions are filled from order.
func ParseOrganic(raw json.RawMessage) ([]Result, error) {
	var body struct {
		Organic []rawResult `json:"organic_results"`
		Error   any         `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("parsing results: %w", err)
	}
	out := make([]Result, 0, len(body.Organic))
	for i, r := range body.Organic {
		link := strings.TrimSpace(r.Link)
		if link == "" {
			continue
		}
		pos := i + 1
		if n, err := r.Position.Int64(); err == nil && n > 0 {
			pos = int(n)
		}
		dom := NormalizeDomain(r.Domain)
		if dom == "" {
			dom = NormalizeDomain(link)
		}
		out = append(out, Result{
			Position: pos,
			Title:    strings.TrimSpace(r.Title),
			URL:      link,
			Domain:   dom,
			Snippet:  strings.TrimSpace(r.Snippet),
			Source:   strings.TrimSpace(r.Source),
			Date:     strings.TrimSpace(r.Date),
		})
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Position < out[b].Position })
	return out, nil
}

// NormalizeDomain turns a URL or host into a lowercase host without "www.".
func NormalizeDomain(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			s = u.Hostname()
		}
	} else {
		if i := strings.IndexAny(s, "/?#"); i >= 0 {
			s = s[:i]
		}
		if i := strings.LastIndex(s, ":"); i >= 0 && !strings.Contains(s[i:], "]") {
			s = s[:i]
		}
	}
	s = strings.TrimSuffix(s, ".")
	return strings.TrimPrefix(s, "www.")
}

// DomainMatches reports whether host belongs to domain (same host or a subdomain).
func DomainMatches(host, domain string) bool {
	host, domain = NormalizeDomain(host), NormalizeDomain(domain)
	if host == "" || domain == "" {
		return false
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// RankOf returns the best (lowest) position of domain in results and the
// matching URL, or 0 when the domain is absent.
func RankOf(results []Result, domain string) (int, string) {
	for _, r := range results {
		if DomainMatches(r.Domain, domain) {
			return r.Position, r.URL
		}
	}
	return 0, ""
}

// TopN returns the results whose position is within the first n (n<=0 means all).
func TopN(results []Result, n int) []Result {
	if n <= 0 {
		return results
	}
	out := make([]Result, 0, n)
	for _, r := range results {
		if r.Position <= n {
			out = append(out, r)
		}
	}
	return out
}

// Domains returns the distinct domains of results, in result order.
func Domains(results []Result) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(results))
	for _, r := range results {
		if r.Domain != "" && !seen[r.Domain] {
			seen[r.Domain] = true
			out = append(out, r.Domain)
		}
	}
	return out
}

// Overlap summarizes how several result sets (keyed by engine or country) share domains.
type Overlap struct {
	Shared  []string            `json:"shared"`  // domains present in every set
	Unique  map[string][]string `json:"unique"`  // domains present in only that set
	Counts  map[string]int      `json:"counts"`  // domain -> number of sets containing it
	Jaccard map[string]float64  `json:"jaccard"` // "a|b" -> |A∩B| / |A∪B| over domains
}

// ComputeOverlap compares the domain sets of each keyed result list. Keys are
// processed in the order given by keys.
func ComputeOverlap(keys []string, sets map[string][]Result) Overlap {
	domSets := map[string]map[string]bool{}
	counts := map[string]int{}
	order := make([]string, 0)
	for _, k := range keys {
		ds := map[string]bool{}
		for _, d := range Domains(sets[k]) {
			ds[d] = true
		}
		domSets[k] = ds
		for d := range ds {
			if counts[d] == 0 {
				order = append(order, d)
			}
			counts[d]++
		}
	}
	sort.Strings(order)
	ov := Overlap{Shared: make([]string, 0), Unique: map[string][]string{}, Counts: counts, Jaccard: map[string]float64{}}
	for _, d := range order {
		if counts[d] == len(keys) && len(keys) > 0 {
			ov.Shared = append(ov.Shared, d)
		}
	}
	for _, k := range keys {
		u := make([]string, 0)
		for _, d := range Domains(sets[k]) {
			if counts[d] == 1 {
				u = append(u, d)
			}
		}
		ov.Unique[k] = u
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			a, b := domSets[keys[i]], domSets[keys[j]]
			inter, union := 0, map[string]bool{}
			for d := range a {
				union[d] = true
				if b[d] {
					inter++
				}
			}
			for d := range b {
				union[d] = true
			}
			v := 0.0
			if len(union) > 0 {
				v = float64(inter) / float64(len(union))
			}
			ov.Jaccard[keys[i]+"|"+keys[j]] = roundTo(v, 3)
		}
	}
	return ov
}

func roundTo(v float64, places int) float64 {
	p := 1.0
	for i := 0; i < places; i++ {
		p *= 10
	}
	return float64(int64(v*p+0.5)) / p
}

// Change describes how one URL moved between two snapshots of the same query.
type Change struct {
	URL    string `json:"url"`
	Domain string `json:"domain"`
	Title  string `json:"title,omitempty"`
	From   int    `json:"from"`  // 0 = absent
	To     int    `json:"to"`    // 0 = absent
	Delta  int    `json:"delta"` // positive = moved up (From - To), 0 for entries and exits
	Kind   string `json:"kind"`  // entered, left, up, down, same
}

// IsRedirectLink reports engine redirect links, such as Google's
// google.com/goto?url=...&gsessionid=... or google.com/url?q=..., whose text
// changes on every call even when the target page is the same.
func IsRedirectLink(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	switch {
	case strings.HasPrefix(host, "google."):
		return u.Path == "/goto" || u.Path == "/url"
	case strings.HasPrefix(host, "bing."):
		return strings.HasPrefix(u.Path, "/ck/")
	}
	return false
}

// Identity is the key that says two results are the same page: the
// canonical URL, or domain plus title for redirect links, whose URL changes
// on every call.
func Identity(r Result) string {
	if IsRedirectLink(r.URL) {
		return "dt:" + NormalizeDomain(r.Domain) + "|" + strings.ToLower(strings.Join(strings.Fields(r.Title), " "))
	}
	return "u:" + CanonicalURL(r.URL)
}

// Diff compares two result lists by page Identity, restricted to the first
// top positions of each list (top<=0 means all).
func Diff(before, after []Result, top int) []Change {
	before, after = TopN(before, top), TopN(after, top)
	key := Identity
	prev := map[string]Result{}
	for _, r := range before {
		if _, ok := prev[key(r)]; !ok {
			prev[key(r)] = r
		}
	}
	seen := map[string]bool{}
	out := make([]Change, 0)
	for _, r := range after {
		k := key(r)
		if seen[k] {
			continue
		}
		seen[k] = true
		if p, ok := prev[k]; ok {
			c := Change{URL: r.URL, Domain: r.Domain, Title: r.Title, From: p.Position, To: r.Position, Delta: p.Position - r.Position}
			switch {
			case c.Delta > 0:
				c.Kind = "up"
			case c.Delta < 0:
				c.Kind = "down"
			default:
				c.Kind = "same"
			}
			out = append(out, c)
		} else {
			out = append(out, Change{URL: r.URL, Domain: r.Domain, Title: r.Title, To: r.Position, Kind: "entered"})
		}
	}
	for _, r := range before {
		k := key(r)
		if !seen[k] {
			seen[k] = true
			out = append(out, Change{URL: r.URL, Domain: r.Domain, Title: r.Title, From: r.Position, Kind: "left"})
		}
	}
	return out
}

// Source is one cited source in a grounding answer.
type Source struct {
	N       int      `json:"n"`
	Title   string   `json:"title"`
	URL     string   `json:"url"`
	Domain  string   `json:"domain"`
	Snippet string   `json:"snippet,omitempty"`
	Date    string   `json:"date,omitempty"`
	FoundIn []string `json:"found_in"`
	Best    int      `json:"best_position"`
}

// MergeSources deduplicates results from several origins by canonical URL,
// keeping first-seen order weighted by best position, and numbers them from 1.
// origins and lists are parallel; limit<=0 keeps everything.
func MergeSources(origins []string, lists [][]Result, limit int) []Source {
	type acc struct {
		src   Source
		score int
		first int
	}
	byKey := map[string]*acc{}
	order := 0
	for i, list := range lists {
		for _, r := range list {
			if strings.TrimSpace(r.URL) == "" {
				continue
			}
			k := Identity(r)
			if k == "" {
				continue
			}
			a, ok := byKey[k]
			if !ok {
				a = &acc{src: Source{Title: r.Title, URL: r.URL, Domain: r.Domain, Snippet: r.Snippet, Date: r.Date, Best: r.Position}, first: order}
				byKey[k] = a
				order++
			}
			if !contains(a.src.FoundIn, origins[i]) {
				a.src.FoundIn = append(a.src.FoundIn, origins[i])
			}
			if r.Position > 0 && (a.src.Best == 0 || r.Position < a.src.Best) {
				a.src.Best = r.Position
			}
			if a.src.Snippet == "" {
				a.src.Snippet = r.Snippet
			}
		}
	}
	all := make([]*acc, 0, len(byKey))
	for _, a := range byKey {
		all = append(all, a)
	}
	// More engines first, then better position, then first seen.
	sort.SliceStable(all, func(i, j int) bool {
		if len(all[i].src.FoundIn) != len(all[j].src.FoundIn) {
			return len(all[i].src.FoundIn) > len(all[j].src.FoundIn)
		}
		if all[i].src.Best != all[j].src.Best {
			return all[i].src.Best < all[j].src.Best
		}
		return all[i].first < all[j].first
	})
	out := make([]Source, 0, len(all))
	for _, a := range all {
		if limit > 0 && len(out) >= limit {
			break
		}
		s := a.src
		s.N = len(out) + 1
		out = append(out, s)
	}
	return out
}

// CanonicalURL lowercases scheme and host, drops "www.", fragments, common
// tracking parameters and the trailing slash, so the same page from two
// engines compares equal.
func CanonicalURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(strings.ToLower(raw), "/")
	}
	q := u.Query()
	for k := range q {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "utm_") || lk == "gclid" || lk == "fbclid" || lk == "ref" {
			q.Del(k)
		}
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	out := host + path
	if enc := q.Encode(); enc != "" {
		out += "?" + enc
	}
	return out
}

// NewsKeys returns the dedupe keys of a news article: its canonical URL and,
// when both are known, its source plus title. An article is a repeat when any
// of its keys was seen before, so the same story re-published under a new URL
// by the same outlet is not reported twice.
func NewsKeys(r Result) []string {
	keys := make([]string, 0, 2)
	if k := CanonicalURL(r.URL); k != "" && !IsRedirectLink(r.URL) {
		keys = append(keys, "url:"+k)
	}
	src, title := strings.ToLower(strings.TrimSpace(r.Source)), strings.ToLower(strings.TrimSpace(r.Title))
	if src == "" {
		src = r.Domain
	}
	if src != "" && title != "" {
		keys = append(keys, "st:"+src+"|"+title)
	}
	return keys
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
