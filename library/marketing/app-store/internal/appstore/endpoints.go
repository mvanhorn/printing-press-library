// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	// LookupBatchSize is the ids per Lookup request (176 verified in one call
	// on 2026-10-23; Apple answered 502 above 200 in 2021).
	LookupBatchSize = 150
	// SearchMaxLimit is the Search API's documented maximum.
	SearchMaxLimit = 200
	// ChartFeedMaxLimit is what the legacy RSS charts return at most.
	ChartFeedMaxLimit = 100
	// ReviewsMaxPage is the deepest customer-reviews RSS page (page 11 is 400).
	ReviewsMaxPage = 10
	// HintsPlatformID is the X-Apple-Store-Front platform id that makes the
	// search-hints endpoint answer its plist.
	HintsPlatformID = 12

	acceptJSON = "application/json, text/javascript, */*;q=0.1"
	acceptHTML = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
)

func (c *Client) resolveCountry(cc string) (string, error) {
	if strings.TrimSpace(cc) == "" {
		return c.country, nil
	}
	return NormalizeCountry(cc)
}

// Lookup calls the iTunes Lookup API. IDs are sent in batches of at most
// LookupBatchSize per request, results keep the input order and Missing lists
// ids Apple returned nothing for in the storefront. Bundle ids are looked up
// one per request (comma-separated bundle ids are not documented). With
// ArtistID it runs a developer lookup (entity=software, limit=200) and
// returns the artist wrapper row separately.
//
// When a batch fails (a throttle, an HTTP error or a wrong-shaped body),
// Lookup stops and returns the error together with a response that holds
// every batch that completed: Results and Missing cover those batches only,
// and Unresolved lists the ids (or bundle ids) that were not looked up.
func (c *Client) Lookup(ctx context.Context, q LookupQuery) (*LookupResponse, error) {
	cc, err := c.resolveCountry(q.Country)
	if err != nil {
		return nil, err
	}
	resp := &LookupResponse{Country: cc, Results: make([]LookupResult, 0), Missing: make([]string, 0)}
	lang := lookupLang(firstNonEmpty(q.Lang, c.lang))
	base := func() url.Values {
		v := url.Values{}
		v.Set("country", cc)
		if q.Entity != "" {
			v.Set("entity", q.Entity)
		}
		if q.Limit > 0 {
			v.Set("limit", strconv.Itoa(q.Limit))
		}
		if lang != "" {
			v.Set("lang", lang)
		}
		return v
	}
	switch {
	case strings.TrimSpace(q.ArtistID) != "":
		artistID := strings.TrimPrefix(strings.TrimSpace(q.ArtistID), "id")
		if !IsAppID(artistID) {
			return nil, fmt.Errorf("invalid developer id %q (want digits)", q.ArtistID)
		}
		v := base()
		v.Set("id", artistID)
		if q.Entity == "" {
			v.Set("entity", "software")
		}
		if q.Limit <= 0 {
			v.Set("limit", "200")
		}
		results, artist, meta, err := c.lookupOnce(ctx, itunesBase+"/lookup?"+v.Encode())
		if err != nil {
			return nil, err
		}
		resp.Requests++
		resp.Meta = append(resp.Meta, meta)
		resp.observe(meta)
		resp.Artist = artist
		resp.Results = append(resp.Results, results...)
		if artist == nil && len(results) == 0 {
			resp.Missing = append(resp.Missing, artistID)
		}
		return resp, nil
	case len(q.BundleIDs) > 0:
		bundles := dedupe(q.BundleIDs)
		for i, bundle := range bundles {
			v := base()
			v.Set("bundleId", bundle)
			results, _, meta, err := c.lookupOnce(ctx, itunesBase+"/lookup?"+v.Encode())
			if err != nil {
				resp.Unresolved = append(resp.Unresolved, bundles[i:]...)
				return resp, err
			}
			resp.Requests++
			resp.Meta = append(resp.Meta, meta)
			resp.observe(meta)
			if len(results) == 0 {
				resp.Missing = append(resp.Missing, bundle)
				continue
			}
			resp.Results = append(resp.Results, results...)
		}
		return resp, nil
	}
	// Normalize first ("id324684580", " 324684580 " and "324684580" are one
	// app), then de-duplicate, so an id is never sent or returned twice.
	normalized := make([]string, 0, len(q.IDs))
	for _, raw := range q.IDs {
		id := strings.TrimSpace(raw)
		if len(id) > 2 && strings.EqualFold(id[:2], "id") {
			id = strings.TrimSpace(id[2:])
		}
		if id == "" {
			continue
		}
		if !IsAppID(id) {
			// One bad id makes Apple fail the whole batch with HTTP 400.
			return nil, fmt.Errorf("invalid App Store id %q (want digits, e.g. 324684580)", raw)
		}
		normalized = append(normalized, id)
	}
	ids := dedupe(normalized)
	if len(ids) == 0 {
		return nil, fmt.Errorf("lookup needs at least one app id, bundle id or developer id")
	}
	byID := make(map[string]LookupResult, len(ids))
	// collect fills Results and Missing, in input order, for the ids of the
	// batches that completed (ids[:done]).
	collect := func(done int) {
		for _, id := range ids[:done] {
			if r, ok := byID[id]; ok {
				resp.Results = append(resp.Results, r)
			} else {
				resp.Missing = append(resp.Missing, id)
			}
		}
	}
	for start := 0; start < len(ids); start += LookupBatchSize {
		end := start + LookupBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		v := base()
		v.Set("id", strings.Join(ids[start:end], ","))
		results, artist, meta, err := c.lookupOnce(ctx, itunesBase+"/lookup?"+v.Encode())
		if err != nil {
			// Keep the batches that completed; the rest is unresolved, not
			// missing (Apple was never asked about them).
			collect(start)
			resp.Unresolved = append(resp.Unresolved, ids[start:]...)
			return resp, err
		}
		resp.Requests++
		resp.Meta = append(resp.Meta, meta)
		resp.observe(meta)
		if artist != nil && resp.Artist == nil {
			resp.Artist = artist
		}
		for _, r := range results {
			byID[r.AppID()] = r
		}
	}
	collect(len(ids))
	return resp, nil
}

func (c *Client) lookupOnce(ctx context.Context, rawURL string) ([]LookupResult, *LookupArtist, FetchMeta, error) {
	res, err := c.fetch(ctx, fetchRequest{URL: rawURL, Accept: acceptJSON})
	if err != nil {
		return nil, nil, FetchMeta{URL: rawURL}, err
	}
	results, artist, _, perr := ParseLookupBody(res.Body)
	outcome := OutcomeOK
	count := len(results)
	if artist != nil {
		count++
	}
	if perr == nil && count == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	c.finish(ctx, res, outcome, count, "", "", perr)
	if perr != nil {
		return nil, nil, res.meta(OutcomeFailed, 0), perr
	}
	stampResults(results, res)
	return results, artist, res.meta(outcome, count), nil
}

// stampResults gives every result its response's observation time.
func stampResults(results []LookupResult, res *fetchResult) {
	asOf := res.dataAsOf()
	for i := range results {
		results[i].FetchedAt = res.FetchedAt
		results[i].FromCache = res.FromCache
		results[i].DataAsOf = asOf
	}
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Search calls the iTunes Search API for software. Order is Apple's API order
// (it differs from the web store's order).
func (c *Client) Search(ctx context.Context, q SearchQuery) (*SearchResponse, error) {
	cc, err := c.resolveCountry(q.Country)
	if err != nil {
		return nil, err
	}
	term := strings.TrimSpace(q.Term)
	if term == "" {
		return nil, fmt.Errorf("search term is empty")
	}
	entity := q.Entity
	switch strings.ToLower(entity) {
	case "", "software", "iphone":
		entity = "software"
	case "ipadsoftware", "ipad":
		entity = "iPadSoftware"
	case "desktopsoftware", "mac":
		entity = "desktopSoftware"
	case "macsoftware":
		entity = "macSoftware"
	default:
		return nil, fmt.Errorf("unknown search entity %q; use software, iPadSoftware or desktopSoftware", q.Entity)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > SearchMaxLimit {
		limit = SearchMaxLimit
	}
	v := url.Values{}
	v.Set("term", term)
	v.Set("country", cc)
	v.Set("media", "software")
	v.Set("entity", entity)
	v.Set("limit", strconv.Itoa(limit))
	if lang := lookupLang(firstNonEmpty(q.Lang, c.lang)); lang != "" {
		v.Set("lang", lang)
	}
	res, err := c.fetch(ctx, fetchRequest{URL: itunesBase + "/search?" + v.Encode(), Accept: acceptJSON})
	if err != nil {
		return nil, err
	}
	results, _, count, perr := ParseLookupBody(res.Body)
	outcome := OutcomeOK
	if perr == nil && len(results) == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	c.finish(ctx, res, outcome, len(results), "", "", perr)
	if perr != nil {
		return nil, perr
	}
	stampResults(results, res)
	return &SearchResponse{
		Term: term, Country: cc, Entity: entity, ResultCount: count, Results: results,
		Meta: res.meta(outcome, len(results)), FetchedAt: res.FetchedAt, FromCache: res.FromCache,
	}, nil
}

// ChartRSS fetches a legacy RSS chart. limit is capped at 100 (Apple returns
// at most 100 rows even for limit=200); genreID is optional.
func (c *Client) ChartRSS(ctx context.Context, country, feed, genreID string, limit int) (*ChartFeed, error) {
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	if !LegacyChartFeeds[feed] {
		return nil, fmt.Errorf("unknown legacy RSS chart feed %q", feed)
	}
	if genreID != "" && !IsAppID(genreID) {
		return nil, fmt.Errorf("invalid genre id %q", genreID)
	}
	if limit <= 0 || limit > ChartFeedMaxLimit {
		limit = ChartFeedMaxLimit
	}
	res, err := c.fetch(ctx, fetchRequest{URL: ChartFeedURL(cc, feed, genreID, limit), Accept: acceptJSON})
	if err != nil {
		return nil, err
	}
	updated, entries, perr := parseChartFeed(res.Body, feed)
	outcome := OutcomeOK
	if perr == nil && len(entries) == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	c.finish(ctx, res, outcome, len(entries), "", updated, perr)
	if perr != nil {
		return nil, perr
	}
	if entries == nil {
		entries = make([]ChartEntry, 0)
	}
	return &ChartFeed{
		Country: cc, Feed: feed, GenreID: genreID, Limit: limit, Updated: updated, Entries: entries, Outcome: outcome,
		Meta: res.meta(outcome, len(entries)), FetchedAt: res.FetchedAt, FromCache: res.FromCache,
	}, nil
}

// ReviewsRSS fetches one customer-reviews RSS page (1..10, 50 reviews each).
// A 200 with no reviews is reported as verified-empty and never cached.
func (c *Client) ReviewsRSS(ctx context.Context, country, appID string, page int, sort string) (*ReviewsFeed, error) {
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	appID = strings.TrimPrefix(strings.TrimSpace(appID), "id")
	if !IsAppID(appID) {
		return nil, fmt.Errorf("invalid App Store id %q (want digits)", appID)
	}
	if page < 1 || page > ReviewsMaxPage {
		return nil, fmt.Errorf("review page %d out of range 1..%d", page, ReviewsMaxPage)
	}
	s, err := NormalizeReviewSort(sort)
	if err != nil {
		return nil, err
	}
	res, err := c.fetch(ctx, fetchRequest{URL: ReviewsFeedURL(cc, appID, page, s), Accept: acceptJSON})
	if err != nil {
		return nil, err
	}
	reviews, perr := parseReviewsFeed(res.Body, reviewsFeedRequest{AppID: appID, Page: page, Sort: s})
	outcome := OutcomeOK
	if perr == nil && len(reviews) == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	oldest, newest := reviewSpan(reviews)
	c.finish(ctx, res, outcome, len(reviews), oldest, newest, perr)
	if perr != nil {
		return nil, perr
	}
	return &ReviewsFeed{
		Country: cc, AppID: appID, Page: page, Sort: s, Reviews: reviews,
		Outcome: outcome, Oldest: oldest, Newest: newest, EntryCount: len(reviews),
		Meta: res.meta(outcome, len(reviews)), FetchedAt: res.FetchedAt, FromCache: res.FromCache,
	}, nil
}

// Hints fetches App Store search suggestions for a term. The request carries
// X-Apple-Store-Front ({storefrontId},12); without it Apple answers nothing.
func (c *Client) Hints(ctx context.Context, country, term string) (*HintsResult, error) {
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("suggest term is empty")
	}
	sf, err := StoreFrontHeader(cc, HintsPlatformID)
	if err != nil {
		return nil, err
	}
	v := url.Values{}
	v.Set("clientApplication", "Software")
	v.Set("term", term)
	res, err := c.fetch(ctx, fetchRequest{
		URL:     hintsURL + "?" + v.Encode(),
		Accept:  "*/*",
		Headers: map[string]string{"X-Apple-Store-Front": sf},
		VaryKey: "X-Apple-Store-Front: " + sf,
	})
	if err != nil {
		return nil, err
	}
	hints, perr := ParseHintsPlist(res.Body)
	outcome := OutcomeOK
	if perr == nil && len(hints) == 0 {
		outcome = OutcomeVerifiedEmpty
	}
	c.finish(ctx, res, outcome, len(hints), "", "", perr)
	if perr != nil {
		return nil, perr
	}
	if hints == nil {
		hints = make([]string, 0)
	}
	return &HintsResult{Term: term, Country: cc, Suggestions: hints, Meta: res.meta(outcome, len(hints)), FetchedAt: res.FetchedAt, FromCache: res.FromCache}, nil
}

// Page fetches an apps.apple.com page and returns its embedded page model.
// A model of a known kind is parsed with that kind's typed parser before the
// capture is logged or cached, exactly as PageOfKind does.
func (c *Client) Page(ctx context.Context, pageURL string) (*PageModel, error) {
	return c.PageOfKind(ctx, pageURL, "")
}

// PageOfKind is Page with an intent-kind check: a model of another kind is a
// *PageKindError (and a failed capture). Pass "today_Intent" for Today pages.
//
// The model is also run through the typed parser for its kind (product,
// charts, developer, Today, see-all, search) before the capture-log row is
// written and before the page enters the cache: a page that fails that parse
// is a failed capture and is never cached, a page that parses to no rows is
// verified-empty and is not cached, and the row's entry count is the parsed
// row count (versions, chart ranks, apps, stories, reviews or lockups).
func (c *Client) PageOfKind(ctx context.Context, pageURL, wantKind string) (*PageModel, error) {
	m, _, err := c.fetchPage(ctx, pageURL, wantKind)
	return m, err
}

// pageParse is the typed parse of one page model.
type pageParse struct {
	value   any // ProductPage, ChartsPage, DeveloperPage, TodayPage, SeeAllPage, SearchPage or nil
	count   int
	outcome string
}

func countOutcome(n int) string {
	if n > 0 {
		return OutcomeOK
	}
	return OutcomeVerifiedEmpty
}

// parsePageModel runs the typed parser for the model's intent kind. Kinds
// without a typed parser count as one ok entry.
func parsePageModel(m *PageModel) (pageParse, error) {
	switch {
	case m.Kind == KindProductPage:
		p, err := ParseProductPage(m)
		if err != nil {
			return pageParse{}, err
		}
		// A product page is an observation of the app even without version
		// rows; the count is its version-history entries.
		return pageParse{value: p, count: len(p.Versions), outcome: OutcomeOK}, nil
	case m.Kind == KindChartsPage:
		p, err := ParseChartsPage(m)
		if err != nil {
			return pageParse{}, err
		}
		n := p.rankCount()
		return pageParse{value: p, count: n, outcome: countOutcome(n)}, nil
	case m.Kind == KindDeveloperPage:
		p, err := ParseDeveloperPage(m)
		if err != nil {
			return pageParse{}, err
		}
		n := len(p.AppIDs())
		return pageParse{value: p, count: n, outcome: countOutcome(n)}, nil
	case IsTodayKind(m.Kind):
		p, err := ParseTodayPage(m)
		if err != nil {
			return pageParse{}, err
		}
		return pageParse{value: p, count: len(p.Stories), outcome: countOutcome(len(p.Stories))}, nil
	case m.Kind == KindSeeAllPage:
		p, err := ParseSeeAllPage(m)
		if err != nil {
			return pageParse{}, err
		}
		n := len(p.Reviews) + len(p.Lockups)
		return pageParse{value: p, count: n, outcome: countOutcome(n)}, nil
	case isSearchKind(m.Kind):
		p, err := ParseSearchPage(m)
		if err != nil {
			return pageParse{}, err
		}
		n := len(p.RankedIDs())
		return pageParse{value: p, count: n, outcome: countOutcome(n)}, nil
	}
	return pageParse{count: 1, outcome: OutcomeOK}, nil
}

// fetchPage is the one page path: fetch (opt-in, governor, redirects and
// cache as in fetch), extract the model, check its kind, run its typed
// parser, and only then write the capture-log row and cache the body.
func (c *Client) fetchPage(ctx context.Context, pageURL, wantKind string) (*PageModel, pageParse, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, pageParse{}, fmt.Errorf("invalid App Store page URL %q: %w", pageURL, err)
	}
	if !strings.EqualFold(u.Hostname(), "apps.apple.com") {
		return nil, pageParse{}, fmt.Errorf("not an apps.apple.com page URL: %q", pageURL)
	}
	pageURL = withPageLang(pageURL, c.lang)
	res, err := c.fetch(ctx, fetchRequest{URL: pageURL, Accept: acceptHTML})
	if err != nil {
		return nil, pageParse{}, err
	}
	m, perr := ExtractPageModel(res.Body)
	var parsed pageParse
	if perr == nil {
		m.URL = res.RequestedURL
		m.FinalURL = res.FinalURL
		m.FromCache = res.FromCache
		m.FetchedAt = res.FetchedAt
		m.DataAsOf = res.dataAsOf()
		perr = expectKind(m, wantKind)
	}
	if perr == nil {
		parsed, perr = parsePageModel(m)
	}
	if perr != nil {
		c.finish(ctx, res, OutcomeFailed, 0, "", "", perr)
		return nil, pageParse{}, perr
	}
	c.finish(ctx, res, parsed.outcome, parsed.count, "", "", nil)
	return m, parsed, nil
}

// withPageLang adds Apple's page-language parameter (l=<lang>) to an
// apps.apple.com page URL when --lang is set. Apple picks the page language
// from l (its page CDN varies on accept-encoding only, so Accept-Language,
// still sent as a hint, does not select it), and the parameter makes the
// URL, and so the cache key, specific to the language. A URL that already
// names a language keeps it.
func withPageLang(rawURL, lang string) string {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Hostname(), "apps.apple.com") {
		return rawURL
	}
	q := u.Query()
	if q.Get("l") != "" {
		return rawURL
	}
	q.Set("l", lang)
	u.RawQuery = q.Encode()
	return u.String()
}

// ProductPageQuery selects an app page. URL (a canonical trackViewUrl)
// avoids the slug redirect; otherwise Country+AppID build the URL.
type ProductPageQuery struct {
	Country  string
	AppID    string
	Platform string
	URL      string
}

// ProductPage fetches and parses an app's product page.
func (c *Client) ProductPage(ctx context.Context, q ProductPageQuery) (*ProductPage, error) {
	pageURL := strings.TrimSpace(q.URL)
	if pageURL == "" {
		cc, err := c.resolveCountry(q.Country)
		if err != nil {
			return nil, err
		}
		id := strings.TrimPrefix(strings.TrimSpace(q.AppID), "id")
		if !IsAppID(id) {
			return nil, fmt.Errorf("invalid App Store id %q (want digits)", q.AppID)
		}
		platform := ""
		if q.Platform != "" {
			p, err := NormalizePlatform(q.Platform)
			if err != nil {
				return nil, err
			}
			platform = p
		}
		pageURL = ProductPageURL(cc, id, platform)
	} else {
		pageURL = CanonicalAppURL(pageURL)
		if q.Platform != "" {
			if p, err := NormalizePlatform(q.Platform); err == nil && p != "iphone" {
				sep := "?"
				if strings.Contains(pageURL, "?") {
					sep = "&"
				}
				pageURL += sep + "platform=" + url.QueryEscape(p)
			}
		}
	}
	_, parsed, err := c.fetchPage(ctx, pageURL, KindProductPage)
	if err != nil {
		return nil, err
	}
	p, ok := parsed.value.(ProductPage)
	if !ok {
		return nil, fmt.Errorf("product page did not parse as a product page")
	}
	return &p, nil
}

// ChartsPageQuery selects a web chart page.
type ChartsPageQuery struct {
	Country   string
	Platform  string // iphone (default), ipad, mac
	GenreID   string // default 36
	Chart     string // top-free (default) or top-paid
	AgeBandID string // Kids bands (genre 36)
}

// ChartsPage fetches and parses a web chart page (200 ranks per chart).
func (c *Client) ChartsPage(ctx context.Context, q ChartsPageQuery) (*ChartsPage, error) {
	cc, err := c.resolveCountry(q.Country)
	if err != nil {
		return nil, err
	}
	platform, err := NormalizePlatform(q.Platform)
	if err != nil {
		return nil, err
	}
	if platform != "iphone" && platform != "ipad" && platform != "mac" {
		return nil, fmt.Errorf("web charts exist for iphone, ipad and mac, not %q", platform)
	}
	genre := strings.TrimSpace(q.GenreID)
	if genre == "" {
		genre = AllGenreID
	}
	if !IsAppID(genre) {
		return nil, fmt.Errorf("invalid genre id %q", q.GenreID)
	}
	chart, err := NormalizeChart(q.Chart)
	if err != nil {
		return nil, err
	}
	if chart == "top-grossing" {
		return nil, fmt.Errorf("top-grossing has no web chart page; use ChartRSS with topgrossingapplications")
	}
	_, parsed, err := c.fetchPage(ctx, ChartsPageURL(cc, platform, genre, chart, q.AgeBandID), KindChartsPage)
	if err != nil {
		return nil, err
	}
	p, ok := parsed.value.(ChartsPage)
	if !ok {
		return nil, fmt.Errorf("chart page did not parse as a chart page")
	}
	if p.Platform == "" {
		p.Platform = platform
	}
	if p.Storefront == "" {
		p.Storefront = cc
	}
	if p.AgeBandID == "" {
		p.AgeBandID = q.AgeBandID
	}
	return &p, nil
}

// DeveloperPage fetches and parses a developer page. pageURL may be the
// canonical artistViewUrl from Lookup; otherwise the id URL is used.
func (c *Client) DeveloperPage(ctx context.Context, country, developerID, pageURL string) (*DeveloperPage, error) {
	if strings.TrimSpace(pageURL) == "" {
		cc, err := c.resolveCountry(country)
		if err != nil {
			return nil, err
		}
		id := strings.TrimPrefix(strings.TrimSpace(developerID), "id")
		if !IsAppID(id) {
			return nil, fmt.Errorf("invalid developer id %q (want digits)", developerID)
		}
		pageURL = DeveloperPageURL(cc, id)
	} else {
		pageURL = CanonicalAppURL(pageURL)
	}
	_, parsed, err := c.fetchPage(ctx, pageURL, KindDeveloperPage)
	if err != nil {
		return nil, err
	}
	p, ok := parsed.value.(DeveloperPage)
	if !ok {
		return nil, fmt.Errorf("developer page did not parse as a developer page")
	}
	if p.DeveloperID == "" {
		p.DeveloperID = strings.TrimPrefix(developerID, "id")
	}
	return &p, nil
}

// TodayPage fetches and parses the Today tab for a platform.
func (c *Client) TodayPage(ctx context.Context, country, platform string) (*TodayPage, error) {
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	p, err := NormalizePlatform(platform)
	if err != nil {
		return nil, err
	}
	_, parsed, err := c.fetchPage(ctx, TodayPageURL(cc, p), todayKindSuffix)
	if err != nil {
		return nil, err
	}
	page, ok := parsed.value.(TodayPage)
	if !ok {
		return nil, fmt.Errorf("today page did not parse as a Today page")
	}
	if page.Storefront == "" {
		page.Storefront = cc
	}
	if page.Platform == "" {
		page.Platform = p
	}
	return &page, nil
}

// SeeAllPage fetches and parses a product see-all page.
func (c *Client) SeeAllPage(ctx context.Context, country, appID, seeAll, platform string) (*SeeAllPage, error) {
	cc, err := c.resolveCountry(country)
	if err != nil {
		return nil, err
	}
	id := strings.TrimPrefix(strings.TrimSpace(appID), "id")
	if !IsAppID(id) {
		return nil, fmt.Errorf("invalid App Store id %q (want digits)", appID)
	}
	if strings.TrimSpace(seeAll) == "" {
		seeAll = "reviews"
	}
	plat := ""
	if platform != "" {
		if plat, err = NormalizePlatform(platform); err != nil {
			return nil, err
		}
	}
	_, parsed, err := c.fetchPage(ctx, SeeAllPageURL(cc, id, seeAll, plat), KindSeeAllPage)
	if err != nil {
		return nil, err
	}
	p, ok := parsed.value.(SeeAllPage)
	if !ok {
		return nil, fmt.Errorf("see-all page did not parse as a see-all page")
	}
	if p.AppID == "" {
		p.AppID = id
	}
	if p.SeeAll == "" {
		p.SeeAll = seeAll
	}
	return &p, nil
}
