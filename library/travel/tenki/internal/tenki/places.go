package tenki

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

var prefectureRE = regexp.MustCompile(`^(北海道|東京都|京都府|大阪府|.{2,3}県)`)
var municipalityNameRE = regexp.MustCompile(`^(.+?市(?:[^（(]*?区)?|.+?区|.+?町|.+?村)`)
var postalRE = regexp.MustCompile(`^[0-9]{3}-?[0-9]{4}$`)

func bounds(limit, maxPages int) (int, int, error) {
	if limit == 0 {
		limit = 10
	}
	if maxPages == 0 {
		maxPages = 1
	}
	if limit < 1 || limit > 50 || maxPages < 1 || maxPages > 2 {
		return 0, 0, errors.New("search limit must be 1–50 and max-pages 1–2")
	}
	return limit, maxPages, nil
}

func nextPage(doc *html.Node, current string) string {
	pager := class(doc, "pager-entries")
	if pager == nil {
		return ""
	}
	cur, _ := url.Parse(current)
	cp := atoi(cur.Query().Get("p"))
	if cp == 0 {
		cp = 1
	}
	for _, a := range all(pager, func(n *html.Node) bool { return n.Data == "a" }) {
		raw := absoluteURL(attr(a, "href"))
		u, err := url.Parse(raw)
		if err != nil || u.Path != cur.Path {
			continue
		}
		p := atoi(u.Query().Get("p"))
		if p == 0 {
			p = atoi(u.Query().Get("page"))
		}
		if p == cp+1 && u.Query().Get("keyword") == cur.Query().Get("keyword") && u.Query().Get("search_type") == cur.Query().Get("search_type") {
			return raw
		}
	}
	return ""
}

func municipalityCandidates(doc *html.Node, query string) ([]Place, int) {
	result := []Place{}
	seen := map[string]bool{}
	scanned := 0
	for _, entry := range all(doc, func(n *html.Node) bool { return hasClass(n, "search-entry-data") }) {
		scanned++
		a := find(entry, func(n *html.Node) bool { return n.Data == "a" })
		raw, err := canonicalPlaceURL(absoluteURL(attr(a, "href")))
		if err != nil {
			continue
		}
		address := nodeText(class(a, "address"))
		zip := nodeText(class(a, "zipcode"))
		match := strings.Contains(address, query)
		if postalRE.MatchString(query) {
			match = strings.ReplaceAll(zip, "-", "") == strings.ReplaceAll(query, "-", "")
		}
		if !match || seen[raw] {
			continue
		}
		pref := ""
		if m := prefectureRE.FindStringSubmatch(address); m != nil {
			pref = m[1]
		}
		name := strings.TrimPrefix(address, pref)
		if m := municipalityNameRE.FindStringSubmatch(name); m != nil {
			name = m[1]
		}
		result = append(result, Place{ID: strings.TrimPrefix(raw, "https://tenki.jp/"), URL: raw, Kind: "municipality", Name: name, Prefecture: pref, Address: address, PostalCode: zip, NameSource: "search_address", ForecastReferenceURL: raw, Scope: "municipal"})
		seen[raw] = true
	}
	return result, scanned
}

func directoryCandidates(doc *html.Node, query, kind string) ([]Place, int) {
	result := []Place{}
	seen := map[string]bool{}
	scanned := 0
	main := id(doc, "main-column")
	if main == nil {
		main = doc
	}
	for _, a := range all(main, func(n *html.Node) bool { return n.Data == "a" }) {
		raw, err := canonicalPlaceURL(absoluteURL(attr(a, "href")))
		if err != nil || !strings.HasPrefix(raw, "https://tenki.jp/"+kind+"/") || seen[raw] {
			continue
		}
		seen[raw] = true
		scanned++
		name := nodeText(a)
		if box := class(a, "text-box"); box != nil {
			name = nodeText(box)
		}
		if box := class(a, "name"); box != nil {
			name = nodeText(box)
		}
		if name == "" {
			name = imageAlt(a)
		}
		if query != "" && !strings.Contains(name, query) {
			continue
		}
		scope := "municipal"
		if kind == "mountain" {
			scope = "foothill"
		}
		result = append(result, Place{ID: strings.TrimPrefix(raw, "https://tenki.jp/"), URL: raw, Kind: kind, Name: name, Scope: scope})
	}
	return result, scanned
}

func (c *Client) Search(ctx context.Context, query, kind string, limit, maxPages int) (SearchResult, error) {
	limit, maxPages, err := bounds(limit, maxPages)
	if err != nil {
		return SearchResult{}, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResult{}, errors.New("search query is required")
	}
	if kind == "" || kind == "all" {
		kind = "municipality"
	}
	if kind != "municipality" && kind != "leisure" && kind != "mountain" {
		return SearchResult{}, fmt.Errorf("unknown place kind %q", kind)
	}
	result := SearchResult{Places: []Place{}, Warnings: []string{}, Status: "ok", SearchScope: "bounded_municipality_search"}
	if strings.HasPrefix(query, "https://") {
		r, err := c.Resolve(ctx, query)
		if err != nil {
			return result, err
		}
		if r.Place.Kind != kind {
			return result, errors.New("canonical URL kind differs from selected search kind")
		}
		result.Places = append(result.Places, r.Place)
		result.Source = r.Source
		result.Pages = 1
		result.Scanned = 1
		result.SearchScope = "canonical_url"
		return result, nil
	}
	raw := "https://tenki.jp/search/?" + url.Values{"keyword": {query}}.Encode()
	if kind == "mountain" {
		raw = "https://tenki.jp/mountain/"
		result.SearchScope = "mountain_directory"
	}
	if kind == "leisure" {
		raw = "https://tenki.jp/leisure/"
		if c.cfg.SearchDirectory != "" {
			raw = c.cfg.SearchDirectory
		}
		u, e := url.Parse(raw)
		if e != nil || !regexp.MustCompile(`^/leisure/(?:[0-9]+/){0,3}$`).MatchString(u.Path) {
			return result, errors.New("--directory must be a canonical tenki.jp leisure directory")
		}
		if _, e = validateURL(raw); e != nil {
			return result, e
		}
		result.SearchScope = "selected_leisure_directory"
		result.DirectoryURL = raw
		result.Warnings = append(result.Warnings, "Leisure name matching scans only the selected directory's linked destinations, not a nationwide catalog. Keyword search is unavailable from the source.")
	}
	seen := map[string]bool{}
	for result.Pages < maxPages {
		body, source, err := c.fetch(ctx, raw, catalogTTL)
		if err != nil {
			return result, err
		}
		result.Source = source
		result.Pages++
		doc := parseHTML(body)
		var candidates []Place
		var count int
		if kind == "municipality" {
			candidates, count = municipalityCandidates(doc, query)
		} else {
			candidates, count = directoryCandidates(doc, query, kind)
		}
		result.Scanned += count
		for _, p := range candidates {
			if !seen[p.URL] {
				seen[p.URL] = true
				result.Places = append(result.Places, p)
			}
		}
		next := nextPage(doc, raw)
		if len(result.Places) > limit {
			result.Truncated = true
			result.Places = result.Places[:limit]
			break
		}
		if next == "" {
			break
		}
		if result.Pages >= maxPages || len(result.Places) >= limit {
			result.Truncated = true
			break
		}
		raw = next
	}
	result.Ambiguous = len(result.Places) > 1 || result.Truncated
	if len(result.Places) == 0 {
		result.Status = "no_results"
	}
	if result.Ambiguous {
		result.Warnings = append(result.Warnings, "Select a canonical place URL; this query has multiple candidates or truncated coverage.")
	}
	return result, nil
}

func kindFromURL(raw string) string {
	path := strings.TrimPrefix(raw, "https://tenki.jp/")
	return strings.Split(path, "/")[0]
}

func (c *Client) Resolve(ctx context.Context, target string) (PlaceResult, error) {
	result := PlaceResult{Warnings: []string{}, Status: "ok"}
	raw := ""
	var err error
	if strings.HasPrefix(target, "https://") || strings.Contains(target, "://") {
		raw, err = canonicalPlaceURL(target)
		if err != nil {
			return result, err
		}
	} else {
		search, err := c.Search(ctx, target, "municipality", 10, 2)
		if err != nil {
			return result, err
		}
		if len(search.Places) == 0 {
			return result, fmt.Errorf("no municipality found for %q", target)
		}
		if len(search.Places) != 1 || search.Truncated {
			urls := []string{}
			for _, p := range search.Places {
				urls = append(urls, p.URL)
			}
			return result, fmt.Errorf("ambiguous place %q; select a canonical URL: %s", target, strings.Join(urls, ", "))
		}
		raw = search.Places[0].URL
	}
	body, source, err := c.fetch(ctx, raw, forecastTTL)
	if err != nil {
		return result, err
	}
	doc := parseHTML(body)
	kind := kindFromURL(raw)
	if kind == "forecast" {
		kind = "municipality"
	}
	result.Place = bodyPlace(doc, raw, kind)
	if kind == "municipality" || kind == "leisure" {
		issue, ir := forecastIssue(doc)
		applyIssue(&source, issue, ir, 2*time.Hour, c.cfg.Now())
	}
	if kind == "sakura" || kind == "kouyou" {
		issue, ir := seasonalIssue(doc)
		applyIssue(&source, issue, ir, 36*time.Hour, c.cfg.Now())
	}
	result.Source = source
	result.Warnings = append(result.Warnings, sourceWarnings(source)...)
	if result.Place.Name == "" {
		return result, errors.New("place identity missing from source markup")
	}
	if kind != "municipality" && result.Place.ForecastReferenceURL == "" {
		result.Warnings = append(result.Warnings, "No municipal weather reference could be verified from this destination page.")
	}
	if kind == "mountain" {
		result.Warnings = append(result.Warnings, "The linked weather describes the named foothill municipality; destination elevation is separate.")
	}
	return result, nil
}
