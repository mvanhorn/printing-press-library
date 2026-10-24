// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ChartsPage is a parsed apps.apple.com chart page. One page carries both
// segments (top-free and top-paid), each with 25 lockups and the remaining
// 175 ranked ids. FetchedAt is when Apple served the page (a cached page
// keeps its original fetch time): the chart snapshot time.
type ChartsPage struct {
	Storefront string          `json:"storefront"`
	Language   string          `json:"language,omitempty"`
	Platform   string          `json:"platform"`
	GenreID    string          `json:"genreId"`
	AgeBandID  string          `json:"ageBandId,omitempty"`
	Chart      string          `json:"chart,omitempty"`
	Title      string          `json:"title,omitempty"`
	Categories []ChartCategory `json:"categories"`
	Segments   []ChartSegment  `json:"segments"`
	FetchedAt  time.Time       `json:"-"`
	FromCache  bool            `json:"-"`
}

// rankCount is the number of ranked app ids across the page's segments.
func (p ChartsPage) rankCount() int {
	n := 0
	for _, s := range p.Segments {
		n += len(s.RankedIDs())
	}
	return n
}

// ChartCategory is one entry of the chart category picker. Kids entries
// carry an ageBandId with genre 36.
type ChartCategory struct {
	Name      string          `json:"name"`
	GenreID   string          `json:"genreId"`
	AgeBandID string          `json:"ageBandId,omitempty"`
	Children  []ChartCategory `json:"children,omitempty"`
}

// ChartSegment is one chart (top-free or top-paid) of a chart page.
type ChartSegment struct {
	Chart        string      `json:"chart"`
	ShortName    string      `json:"shortName,omitempty"`
	LongName     string      `json:"longName,omitempty"`
	Lockups      []AppLockup `json:"lockups"`
	RemainingIDs []string    `json:"remainingIds"`
}

// RankedIDs returns every app id of the segment in rank order (lockups
// first, then the remaining ids), de-duplicated.
func (s ChartSegment) RankedIDs() []string {
	out := make([]string, 0, len(s.Lockups)+len(s.RemainingIDs))
	seen := map[string]bool{}
	for _, l := range s.Lockups {
		if l.AdamID != "" && !seen[l.AdamID] {
			seen[l.AdamID] = true
			out = append(out, l.AdamID)
		}
	}
	for _, id := range s.RemainingIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// RankedApp is one chart entry and its rank.
type RankedApp struct {
	ID   string
	Rank int
}

// RankedApps returns the segment's apps in rank order with their ranks. A lockup
// keeps its page ordinal, so a card the parser skipped (an entry without an
// app id) leaves a gap instead of moving every later app up one place; the
// remaining ids follow the highest lockup rank. Duplicates are dropped.
func (s ChartSegment) RankedApps() []RankedApp {
	out := make([]RankedApp, 0, len(s.Lockups)+len(s.RemainingIDs))
	seen := map[string]bool{}
	last := 0
	for _, l := range s.Lockups {
		if l.AdamID == "" || seen[l.AdamID] {
			continue
		}
		seen[l.AdamID] = true
		rank := max(l.Position, last+1)
		last = rank
		out = append(out, RankedApp{ID: l.AdamID, Rank: rank})
	}
	for _, id := range s.RemainingIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		last++
		out = append(out, RankedApp{ID: id, Rank: last})
	}
	return out
}

// Segment returns the segment for a chart name ("top-free", "top-paid").
func (p ChartsPage) Segment(chart string) (ChartSegment, bool) {
	for _, s := range p.Segments {
		if s.Chart == chart {
			return s, true
		}
	}
	return ChartSegment{}, false
}

func parseChartCategories(items []any) []ChartCategory {
	out := make([]ChartCategory, 0, len(items))
	for _, it := range items {
		c := ChartCategory{
			Name:      cleanText(firstNonEmpty(strAt(it, "name"), strAt(it, "title"))),
			GenreID:   strAt(it, "genreId"),
			AgeBandID: strAt(it, "ageBandId"),
		}
		if kids := arrAt(it, "children"); len(kids) > 0 {
			c.Children = parseChartCategories(kids)
		}
		out = append(out, c)
	}
	return out
}

// ParseChartsPage parses a ChartsPageIntent model. A segment whose shelves
// hold cards but no app Lockup (a renamed card kind), or whose next page
// lists entries without ids, is an error rather than an empty chart: ranks
// would otherwise be lost or shifted. A segment with no cards and no next
// page is an explicitly empty chart (0 ranks).
func ParseChartsPage(m *PageModel) (ChartsPage, error) {
	if err := expectKind(m, KindChartsPage); err != nil {
		return ChartsPage{}, err
	}
	d := m.Data
	p := ChartsPage{
		Storefront: m.IntentString("storefront"),
		Language:   m.IntentString("language"),
		Platform:   m.IntentString("platform"),
		GenreID:    firstNonEmpty(strAt(d, "genreId"), m.IntentString("genreId")),
		AgeBandID:  m.IntentString("ageBandId"),
		Chart:      m.IntentString("chart"),
		Title:      cleanText(strAt(d, "title")),
		Categories: parseChartCategories(arrAt(d, "categories")),
		Segments:   make([]ChartSegment, 0, 2),
		FetchedAt:  m.FetchedAt,
		FromCache:  m.FromCache,
	}
	for _, seg := range arrAt(d, "segments") {
		s := ChartSegment{
			Chart:        strAt(seg, "chart"),
			ShortName:    cleanText(strAt(seg, "shortName")),
			LongName:     cleanText(strAt(seg, "longName")),
			Lockups:      make([]AppLockup, 0, 25),
			RemainingIDs: make([]string, 0, 175),
		}
		cards := 0
		for _, shelf := range arrAt(seg, "shelves") {
			for _, it := range arrAt(shelf, "items") {
				cards++
				if kindOf(it) != "Lockup" || strAt(it, "adamId") == "" {
					continue
				}
				l := lockupFrom(it)
				if ord, err := strconv.Atoi(strAt(it, "ordinal")); err == nil && ord > 0 {
					l.Position = ord
				} else {
					l.Position = len(s.Lockups) + 1
				}
				s.Lockups = append(s.Lockups, l)
			}
		}
		remaining := arrAt(seg, "nextPage", "remainingContent")
		for _, rc := range remaining {
			if id := strAt(rc, "id"); id != "" {
				s.RemainingIDs = append(s.RemainingIDs, id)
			}
		}
		name := firstNonEmpty(s.Chart, "unnamed")
		if cards > 0 && len(s.Lockups) == 0 {
			return ChartsPage{}, fmt.Errorf("chart segment %s has %d cards but no app Lockup (unrecognized card kind)", name, cards)
		}
		if len(remaining) > 0 && len(s.RemainingIDs) == 0 {
			return ChartsPage{}, fmt.Errorf("chart segment %s lists %d further entries without app ids", name, len(remaining))
		}
		p.Segments = append(p.Segments, s)
	}
	if len(p.Segments) == 0 {
		return ChartsPage{}, fmt.Errorf("chart page model has no chart segments")
	}
	return p, nil
}

// DeveloperPage is a parsed developer page: one shelf per platform group.
type DeveloperPage struct {
	DeveloperID string           `json:"developerId"`
	Name        string           `json:"name"`
	Storefront  string           `json:"storefront,omitempty"`
	URL         string           `json:"url,omitempty"`
	Shelves     []DeveloperShelf `json:"shelves"`
	FetchedAt   time.Time        `json:"-"`
	FromCache   bool             `json:"-"`
}

// DeveloperShelf is one developer-page shelf ("Latest Release", "iPad &
// iPhone", "Mac", "Apple TV", "Apple Watch").
type DeveloperShelf struct {
	Title    string      `json:"title"`
	Platform string      `json:"platform,omitempty"`
	Apps     []AppLockup `json:"apps"`
}

// AppIDs returns the distinct app ids across shelves in first-seen order.
func (p DeveloperPage) AppIDs() []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, s := range p.Shelves {
		for _, a := range s.Apps {
			if !seen[a.AdamID] {
				seen[a.AdamID] = true
				out = append(out, a.AdamID)
			}
		}
	}
	return out
}

func platformFromShelfTitle(title string) string {
	t := strings.ToLower(title)
	switch {
	case strings.Contains(t, "iphone") || strings.Contains(t, "ipad"):
		return "iphone"
	case strings.Contains(t, "mac"):
		return "mac"
	case strings.Contains(t, "tv"):
		return "appletv"
	case strings.Contains(t, "watch"):
		return "watch"
	case strings.Contains(t, "vision"):
		return "vision"
	}
	return ""
}

// ParseDeveloperPage parses a DeveloperPageIntent model. Shelves that hold
// cards but not one app Lockup (a renamed card kind) are an error rather
// than a developer without apps.
func ParseDeveloperPage(m *PageModel) (DeveloperPage, error) {
	if err := expectKind(m, KindDeveloperPage); err != nil {
		return DeveloperPage{}, err
	}
	d := m.Data
	p := DeveloperPage{
		DeveloperID: m.IntentString("id"),
		Name:        cleanText(strAt(d, "title")),
		Storefront:  m.IntentString("storefront"),
		URL:         strAt(d, "canonicalURL"),
		Shelves:     make([]DeveloperShelf, 0),
		FetchedAt:   m.FetchedAt,
		FromCache:   m.FromCache,
	}
	cards := 0
	for _, shelf := range arrAt(d, "shelves") {
		items := arrAt(shelf, "items")
		cards += len(items)
		apps := parseLockups(items)
		if len(apps) == 0 {
			continue
		}
		title := cleanText(strAt(shelf, "title"))
		platform := ""
		if !strings.EqualFold(title, "Latest Release") {
			platform = platformFromShelfTitle(title)
			if platform == "" {
				platform = apps[0].Platform
			}
		}
		p.Shelves = append(p.Shelves, DeveloperShelf{Title: title, Platform: platform, Apps: apps})
	}
	if cards > 0 && len(p.Shelves) == 0 {
		return DeveloperPage{}, fmt.Errorf("developer page has %d shelf cards but no app Lockup (unrecognized card kind)", cards)
	}
	return p, nil
}

// TodayPage is the parsed Today tab: editorial stories in display order.
type TodayPage struct {
	Storefront string       `json:"storefront"`
	Platform   string       `json:"platform,omitempty"`
	Stories    []TodayStory `json:"stories"`
	FetchedAt  time.Time    `json:"-"`
	FromCache  bool         `json:"-"`
}

// todayNonStoryKinds are Today shelf cards that are not stories (the
// sign-in/promo message at the top of the tab).
var todayNonStoryKinds = map[string]bool{"UnifiedMessage": true}

// TodayStory is one Today card and the apps it features.
type TodayStory struct {
	Position int         `json:"position"`
	StoryID  string      `json:"storyId"`
	Title    string      `json:"title"`
	Heading  string      `json:"heading,omitempty"`
	Apps     []AppLockup `json:"apps"`
}

// AppIDs returns the story's featured app ids in order.
func (s TodayStory) AppIDs() []string {
	out := make([]string, 0, len(s.Apps))
	for _, a := range s.Apps {
		out = append(out, a.AdamID)
	}
	return out
}

// collectLockups walks a node depth-first and returns every Lockup card in
// document order, de-duplicated by adamId.
func collectLockups(node any) []AppLockup {
	out := make([]AppLockup, 0)
	seen := map[string]bool{}
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if kindOf(v) == "Lockup" {
				if id := strOf(v["adamId"]); id != "" && !seen[id] {
					seen[id] = true
					l := lockupFrom(v)
					l.Position = len(out) + 1
					out = append(out, l)
				}
			}
			// Deterministic order: walk the keys that hold lockups first.
			for _, k := range []string{"overlay", "lockup", "lockups", "media", "items"} {
				if child, ok := v[k]; ok {
					walk(child)
				}
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(node)
	return out
}

// ParseTodayPage parses a Today landing page model. A page without a single
// TodayCard is empty only when its other cards are known non-story kinds;
// cards of an unknown kind (a renamed story card) make it an error.
func ParseTodayPage(m *PageModel) (TodayPage, error) {
	if err := expectKind(m, todayKindSuffix); err != nil {
		return TodayPage{}, err
	}
	platform := ""
	if k := strings.TrimPrefix(m.Kind, todayKindPrefix); k != m.Kind {
		platform = strings.TrimSuffix(strings.TrimSuffix(k, todayKindSuffix), "/")
	}
	p := TodayPage{Storefront: m.IntentString("storefront"), Platform: platform, Stories: make([]TodayStory, 0), FetchedAt: m.FetchedAt, FromCache: m.FromCache}
	unknown := map[string]int{}
	for _, shelf := range arrAt(m.Data, "shelves") {
		for _, it := range arrAt(shelf, "items") {
			if k := kindOf(it); k != "TodayCard" {
				if !todayNonStoryKinds[k] {
					unknown[k]++
				}
				continue
			}
			p.Stories = append(p.Stories, TodayStory{
				Position: len(p.Stories) + 1,
				StoryID:  storyIDOf(it),
				Title:    cleanText(strAt(it, "title")),
				Heading:  cleanText(strAt(it, "heading")),
				Apps:     collectLockups(it),
			})
		}
	}
	if len(p.Stories) == 0 && len(unknown) > 0 {
		kinds := make([]string, 0, len(unknown))
		for k, n := range unknown {
			kinds = append(kinds, fmt.Sprintf("%q x%d", k, n))
		}
		sort.Strings(kinds)
		return TodayPage{}, fmt.Errorf("today page has no TodayCard but cards of unknown kinds %s", strings.Join(kinds, ", "))
	}
	return p, nil
}

// SeeAllPage is a parsed product see-all page (reviews or a lockup list).
type SeeAllPage struct {
	AppID     string      `json:"appId"`
	SeeAll    string      `json:"seeAll"`
	Title     string      `json:"title,omitempty"`
	Reviews   []Review    `json:"reviews"`
	Lockups   []AppLockup `json:"lockups"`
	FetchedAt time.Time   `json:"-"`
	FromCache bool        `json:"-"`
}

// ParseSeeAllPage parses a SeeAllPageIntent model. For see-all=reviews it
// returns the (up to 10) most-helpful reviews the page embeds; for app lists
// (customers-also-bought-apps, developer-other-apps) the lockups.
func ParseSeeAllPage(m *PageModel) (SeeAllPage, error) {
	if err := expectKind(m, KindSeeAllPage); err != nil {
		return SeeAllPage{}, err
	}
	p := SeeAllPage{
		AppID:     m.IntentString("id"),
		SeeAll:    m.IntentString("see-all"),
		Title:     cleanText(strAt(m.Data, "title")),
		Reviews:   make([]Review, 0),
		Lockups:   make([]AppLockup, 0),
		FetchedAt: m.FetchedAt,
		FromCache: m.FromCache,
	}
	sm := objAt(m.Data, "shelfMapping")
	for _, it := range arrAt(sm, "allProductReviews", "items") {
		r := objAt(it, "review")
		if r == nil {
			continue
		}
		p.Reviews = append(p.Reviews, Review{
			ID:                firstNonEmpty(strAt(r, "id"), strAt(it, "id")),
			Rating:            int(intOf(r["rating"])),
			Title:             cleanText(strAt(r, "title")),
			Body:              cleanMultiline(strAt(r, "contents")),
			Author:            cleanText(strAt(r, "reviewerName")),
			ReviewedAt:        normalizeDate(strAt(r, "date")),
			DeveloperResponse: cleanMultiline(firstNonEmpty(strAt(r, "response", "contents"), strAt(r, "developerResponse", "contents"))),
		})
	}
	if p.SeeAll != "reviews" {
		for _, shelf := range arrAt(m.Data, "shelves") {
			for _, l := range parseLockups(arrAt(shelf, "items")) {
				l.Position = len(p.Lockups) + 1
				p.Lockups = append(p.Lockups, l)
			}
		}
		if len(p.Lockups) == 0 && sm != nil {
			for _, key := range []string{"similarItems", "moreByDeveloper"} {
				for _, l := range parseLockups(arrAt(sm, key, "items")) {
					l.Position = len(p.Lockups) + 1
					p.Lockups = append(p.Lockups, l)
				}
			}
		}
	}
	return p, nil
}

// SearchPage is the opt-in web search result page: app ids in the order the
// web store shows them (closer to on-device order than the Search API).
type SearchPage struct {
	Term      string      `json:"term"`
	Lockups   []AppLockup `json:"lockups"`
	NextIDs   []string    `json:"nextIds"`
	FetchedAt time.Time   `json:"-"`
	FromCache bool        `json:"-"`
}

// RankedIDs returns lockup ids followed by the next-page ids, de-duplicated.
func (p SearchPage) RankedIDs() []string {
	out := make([]string, 0, len(p.Lockups)+len(p.NextIDs))
	seen := map[string]bool{}
	for _, l := range p.Lockups {
		if !seen[l.AdamID] {
			seen[l.AdamID] = true
			out = append(out, l.AdamID)
		}
	}
	for _, id := range p.NextIDs {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ParseSearchPage parses a web search results page model. The search page
// kind is not pinned (no capture in this run), so any model whose kind
// mentions "Search" is accepted; lockups are read in document order and the
// nextPage.remainingContent ids are appended.
func ParseSearchPage(m *PageModel) (SearchPage, error) {
	if m == nil {
		return SearchPage{}, fmt.Errorf("nil App Store page model")
	}
	if !isSearchKind(m.Kind) {
		return SearchPage{}, &PageKindError{URL: firstNonEmpty(m.FinalURL, m.URL), Want: searchKindWant, Got: m.Kind}
	}
	p := SearchPage{
		Term:      firstNonEmpty(m.IntentString("term"), m.IntentString("searchTerm")),
		Lockups:   make([]AppLockup, 0),
		NextIDs:   make([]string, 0),
		FetchedAt: m.FetchedAt,
		FromCache: m.FromCache,
	}
	seen := map[string]bool{}
	next := make([]string, 0)
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if kindOf(v) == "Lockup" {
				if id := strOf(v["adamId"]); id != "" && !seen[id] {
					seen[id] = true
					l := lockupFrom(v)
					l.Position = len(p.Lockups) + 1
					p.Lockups = append(p.Lockups, l)
				}
				return
			}
			if np := objAt(v, "nextPage"); np != nil {
				for _, rc := range arrAt(np, "remainingContent") {
					if id := strAt(rc, "id"); id != "" {
						next = append(next, id)
					}
				}
			}
			for _, k := range []string{"shelves", "items", "lockup", "lockups", "segments", "results", "data"} {
				if child, ok := v[k]; ok {
					walk(child)
				}
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(m.Data)
	// Next-page ids rank after every lockup; drop ids already shown.
	for _, id := range next {
		if !seen[id] {
			seen[id] = true
			p.NextIDs = append(p.NextIDs, id)
		}
	}
	return p, nil
}
