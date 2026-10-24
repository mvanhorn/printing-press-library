// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixtures in testdata/ are trimmed captures of real Apple responses from
// 2026-10-23 (built from the main session's probe captures): page models keep
// only the keys the parsers read, and every reviewer name, review title and
// review body is synthetic.

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func fixtureModel(t *testing.T, name string) *PageModel {
	t.Helper()
	m, err := ExtractPageModel(readFixture(t, name))
	if err != nil {
		t.Fatalf("extract %s: %v", name, err)
	}
	return m
}

func isRFC3339UTC(s string) bool {
	t, err := time.Parse(time.RFC3339, s)
	return err == nil && strings.HasSuffix(s, "Z") && t.Year() > 2000
}

func TestExtractPageModel(t *testing.T) {
	cases := []struct {
		fixture string
		kind    string
	}{
		{"product_spotify_us.html", KindProductPage},
		{"product_candycrush_us.html", KindProductPage},
		{"charts_us_iphone_36.html", KindChartsPage},
		{"developer_spotify_us.html", KindDeveloperPage},
		{"seeall_reviews_spotify_us.html", KindSeeAllPage},
		{"today_us_iphone.html", "LandingPage_iphone/today_Intent"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			m := fixtureModel(t, tc.fixture)
			if m.Kind != tc.kind {
				t.Fatalf("kind = %q, want %q", m.Kind, tc.kind)
			}
			if m.IntentString("storefront") == "" {
				t.Fatalf("intent has no storefront: %v", m.Intent)
			}
			if len(m.Data) == 0 {
				t.Fatal("page data is empty")
			}
		})
	}
	t.Run("bare array envelope", func(t *testing.T) {
		html := `<html><script id="serialized-server-data" type="application/json">[{"intent":{"$kind":"ChartsPageIntent","storefront":"us"},"data":{"segments":[]}}]</script></html>`
		m, err := ExtractPageModel([]byte(html))
		if err != nil {
			t.Fatal(err)
		}
		if m.Kind != KindChartsPage {
			t.Fatalf("kind = %q", m.Kind)
		}
	})
	t.Run("no model", func(t *testing.T) {
		if _, err := ExtractPageModel([]byte("<html><body>nothing</body></html>")); err == nil {
			t.Fatal("expected an error for HTML without a page model")
		}
	})
}

func TestParseProductPage(t *testing.T) {
	cases := []struct {
		fixture string
		check   func(t *testing.T, p ProductPage)
	}{
		{"product_spotify_us.html", func(t *testing.T, p ProductPage) {
			if p.AdamID != "324684580" || !strings.HasPrefix(p.Title, "Spotify") {
				t.Fatalf("identity = %s %q", p.AdamID, p.Title)
			}
			if p.Subtitle == "" || p.BundleID != "com.spotify.client" {
				t.Fatalf("subtitle %q bundle %q", p.Subtitle, p.BundleID)
			}
			if len(p.Histogram) != 5 {
				t.Fatalf("histogram has %d values, want 5", len(p.Histogram))
			}
			sum := p.HistogramSum()
			if p.TotalRatings <= 1_000_000 {
				t.Fatalf("totalRatings = %d, want > 1,000,000", p.TotalRatings)
			}
			if diff := math.Abs(float64(sum-p.TotalRatings)) / float64(p.TotalRatings); diff > 0.01 {
				t.Fatalf("histogram sum %d vs totalRatings %d differs by %.4f", sum, p.TotalRatings, diff)
			}
			if p.Histogram[0] <= p.Histogram[4] {
				t.Fatalf("histogram not 5-star first: %v", p.Histogram)
			}
			if len(p.Versions) != 25 {
				t.Fatalf("versions = %d, want 25", len(p.Versions))
			}
			for i, v := range p.Versions {
				if v.Version == "" || !isRFC3339UTC(v.ReleasedAt) {
					t.Fatalf("version %d = %+v, want version and RFC3339 UTC date", i, v)
				}
				if i > 0 && p.Versions[i-1].ReleasedAt < v.ReleasedAt {
					t.Fatalf("versions not newest first at %d: %s < %s", i, p.Versions[i-1].ReleasedAt, v.ReleasedAt)
				}
			}
			if p.Versions[0].Version != "9.1.92" || p.Versions[0].ReleasedAt != "2026-10-19T13:57:56Z" {
				t.Fatalf("newest version = %+v", p.Versions[0])
			}
			if len(p.Privacy.Types) < 1 || p.Privacy.PolicyURL == "" {
				t.Fatalf("privacy = %+v", p.Privacy)
			}
			total := 0
			for _, pt := range p.Privacy.Types {
				for _, c := range pt.Categories {
					total += len(c.DataTypes)
				}
			}
			if total == 0 {
				t.Fatal("privacy categories carry no data types (detail page not read)")
			}
			if p.Accessibility.Declared || len(p.Accessibility.Features) != 0 {
				t.Fatalf("Spotify accessibility = %+v, want not declared", p.Accessibility)
			}
			if p.ChartPosition == nil || p.ChartPosition.Chart != "top-free" || p.ChartPosition.GenreID != "6011" || p.ChartPosition.Position < 1 {
				t.Fatalf("chartPosition = %+v", p.ChartPosition)
			}
			if p.DeveloperID != "324684583" || p.DeveloperName != "Spotify" {
				t.Fatalf("developer = %s %s", p.DeveloperID, p.DeveloperName)
			}
			if len(p.Capabilities) == 0 || len(p.SimilarItems) == 0 || len(p.Media) == 0 {
				t.Fatalf("capabilities %d similar %d media %d", len(p.Capabilities), len(p.SimilarItems), len(p.Media))
			}
			if !strings.HasPrefix(p.Media[0].Screenshots[0], "https://") || strings.Contains(p.Media[0].Screenshots[0], "{w}") {
				t.Fatalf("screenshot URL not filled: %s", p.Media[0].Screenshots[0])
			}
			if len(p.InAppPurchases) != 0 {
				t.Fatalf("Spotify US lists no IAP annotation, got %d", len(p.InAppPurchases))
			}
		}},
		{"product_candycrush_us.html", func(t *testing.T, p ProductPage) {
			if p.AdamID != "553834731" {
				t.Fatalf("adamId = %s", p.AdamID)
			}
			if len(p.InAppPurchases) != 10 {
				t.Fatalf("IAP rows = %d, want 10", len(p.InAppPurchases))
			}
			for i, it := range p.InAppPurchases {
				if it.Position != i+1 || it.Name == "" || !strings.HasPrefix(it.Price, "$") {
					t.Fatalf("IAP %d = %+v", i, it)
				}
			}
			if !p.HasInAppPurchases {
				t.Fatal("hasInAppPurchases = false")
			}
			if len(p.Events) != 1 || p.Events[0].EventID == "" || !isRFC3339UTC(p.Events[0].StartDate) || !isRFC3339UTC(p.Events[0].EndDate) {
				t.Fatalf("events = %+v", p.Events)
			}
			if len(p.FeaturedIn) != 2 || p.FeaturedIn[0].StoryID == "" {
				t.Fatalf("featuredIn = %+v", p.FeaturedIn)
			}
			if p.AgeRating == nil || p.AgeRating.Rating != "13+" {
				t.Fatalf("ageRating = %+v", p.AgeRating)
			}
			if p.ChartPosition == nil || p.ChartPosition.GenreID != "7003" {
				t.Fatalf("chartPosition = %+v", p.ChartPosition)
			}
			if len(p.MoreByDeveloper) == 0 || p.MoreByDeveloper[0].DeveloperName != "King" {
				t.Fatalf("moreByDeveloper = %+v", p.MoreByDeveloper)
			}
			hasVideo := false
			for _, m := range p.Media {
				if len(m.Videos) > 0 {
					hasVideo = true
				}
			}
			if !hasVideo {
				t.Fatal("expected a preview video in the iPad media set")
			}
		}},
		{"product_soda_us.html", func(t *testing.T, p ProductPage) {
			if p.AdamID != "850417475" || len(p.Events) != 2 || len(p.InAppPurchases) != 10 {
				t.Fatalf("soda: id %s events %d iap %d", p.AdamID, len(p.Events), len(p.InAppPurchases))
			}
			if p.ChartPosition != nil {
				t.Fatalf("soda has a category badge, not a chart badge: %+v", p.ChartPosition)
			}
		}},
		{"product_pages_us.html", func(t *testing.T, p ProductPage) {
			if p.AdamID != "361309726" {
				t.Fatalf("adamId = %s", p.AdamID)
			}
			if !p.Accessibility.Declared || len(p.Accessibility.Features) < 1 {
				t.Fatalf("Pages accessibility = %+v, want declared with features", p.Accessibility)
			}
			if p.Accessibility.Features[0] != "VoiceOver" {
				t.Fatalf("first feature = %q", p.Accessibility.Features[0])
			}
			if len(p.Versions) != 25 {
				t.Fatalf("versions = %d", len(p.Versions))
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			p, err := ParseProductPage(fixtureModel(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, p)
		})
	}
	t.Run("localized annotation titles", func(t *testing.T) {
		m := &PageModel{
			Kind:   KindProductPage,
			Intent: map[string]any{"$kind": KindProductPage, "id": "553834731", "storefront": "jp"},
			Data: map[string]any{
				"lockup": map[string]any{"adamId": "553834731", "title": "キャンディークラッシュ"},
				"shelfMapping": map[string]any{"information": map[string]any{"items": []any{
					map[string]any{"$kind": "Annotation", "title": "App内課金有り", "summary": "あり", "items": []any{
						map[string]any{"$kind": "textPair", "leadingText": "ゴールドバー", "trailingText": "¥300"},
						map[string]any{"$kind": "textPair", "leadingText": "ライフ", "trailingText": "¥160"},
						map[string]any{"$kind": "button", "action": map[string]any{"title": "詳しく見る"}},
					}},
					map[string]any{"$kind": "Annotation", "title": "年齢制限指定", "summary": "12+", "items": []any{
						map[string]any{"$kind": "textEncapsulation", "text": "12+"},
					}},
				}}},
			},
		}
		p, err := ParseProductPage(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.InAppPurchases) != 2 || p.InAppPurchases[1].Price != "¥160" || !p.HasInAppPurchases {
			t.Fatalf("localized IAP rows = %+v", p.InAppPurchases)
		}
		if p.AgeRating == nil || p.AgeRating.Rating != "12+" {
			t.Fatalf("localized age rating = %+v", p.AgeRating)
		}
	})
	t.Run("wrong kind", func(t *testing.T) {
		_, err := ParseProductPage(fixtureModel(t, "charts_us_iphone_36.html"))
		var pk *PageKindError
		if err == nil || !errors.As(err, &pk) {
			t.Fatalf("want PageKindError, got %v", err)
		}
	})
}

func TestParseChartsPage(t *testing.T) {
	cases := []struct {
		fixture    string
		storefront string
		platform   string
		genre      string
		wantCats   []string
		pricePaid  string
	}{
		{"charts_us_iphone_36.html", "us", "iphone", "36", []string{"6026", "6027", "Kids"}, "$"},
		{"charts_pk_iphone_6014.html", "pk", "iphone", "6014", []string{"7003", "7012", "Kids"}, "Rs"},
		{"charts_us_mac_36.html", "us", "mac", "36", []string{"6026", "6014"}, "$"},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			p, err := ParseChartsPage(fixtureModel(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			if p.Storefront != tc.storefront || p.Platform != tc.platform || p.GenreID != tc.genre {
				t.Fatalf("key = %s/%s/%s", p.Storefront, p.Platform, p.GenreID)
			}
			if len(p.Segments) != 2 {
				t.Fatalf("segments = %d, want 2", len(p.Segments))
			}
			for _, chart := range []string{"top-free", "top-paid"} {
				s, ok := p.Segment(chart)
				if !ok {
					t.Fatalf("missing segment %s", chart)
				}
				if len(s.Lockups) != 25 || len(s.RemainingIDs) != 175 || len(s.RankedIDs()) != 200 {
					t.Fatalf("%s: lockups %d remaining %d ranked %d, want 25/175/200", chart, len(s.Lockups), len(s.RemainingIDs), len(s.RankedIDs()))
				}
				for i, l := range s.Lockups {
					if l.Position != i+1 || l.AdamID == "" || l.Title == "" {
						t.Fatalf("%s lockup %d = %+v", chart, i, l)
					}
				}
			}
			paid, _ := p.Segment("top-paid")
			if !strings.Contains(paid.Lockups[0].PriceText, tc.pricePaid) {
				t.Fatalf("paid #1 price %q, want it to contain %q", paid.Lockups[0].PriceText, tc.pricePaid)
			}
			have := map[string]bool{}
			for _, c := range p.FlattenCategories() {
				have[c.GenreID] = true
				have[c.Name] = true
			}
			for _, want := range tc.wantCats {
				if !have[want] {
					t.Fatalf("categories missing %q", want)
				}
			}
		})
	}
	t.Run("kids age bands", func(t *testing.T) {
		p, err := ParseChartsPage(fixtureModel(t, "charts_us_iphone_36.html"))
		if err != nil {
			t.Fatal(err)
		}
		bands := map[string]bool{}
		for _, c := range p.FlattenCategories() {
			if c.AgeBandID != "" {
				bands[c.AgeBandID] = true
			}
		}
		for _, id := range []string{"0", "10000", "10001", "10002"} {
			if !bands[id] {
				t.Fatalf("missing Kids age band %s (have %v)", id, bands)
			}
		}
	})
}

func TestParseDeveloperPage(t *testing.T) {
	p, err := ParseDeveloperPage(fixtureModel(t, "developer_spotify_us.html"))
	if err != nil {
		t.Fatal(err)
	}
	if p.DeveloperID != "324684583" || p.Name != "Spotify" {
		t.Fatalf("developer = %s %q", p.DeveloperID, p.Name)
	}
	wantShelves := map[string]string{"Latest Release": "", "iPad & iPhone": "iphone", "Apple TV": "appletv", "Apple Watch": "watch"}
	if len(p.Shelves) != len(wantShelves) {
		t.Fatalf("shelves = %d, want %d", len(p.Shelves), len(wantShelves))
	}
	for _, s := range p.Shelves {
		platform, ok := wantShelves[s.Title]
		if !ok || s.Platform != platform || len(s.Apps) == 0 {
			t.Fatalf("shelf %+v", s)
		}
	}
	ids := p.AppIDs()
	found := false
	for _, id := range ids {
		if id == "324684580" {
			found = true
		}
	}
	if !found || len(ids) < 4 {
		t.Fatalf("app ids = %v", ids)
	}
}

func TestParseTodayPage(t *testing.T) {
	p, err := ParseTodayPage(fixtureModel(t, "today_us_iphone.html"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Storefront != "us" || p.Platform != "iphone" {
		t.Fatalf("key = %s/%s", p.Storefront, p.Platform)
	}
	if len(p.Stories) != 32 {
		t.Fatalf("stories = %d, want 32", len(p.Stories))
	}
	distinct := map[string]bool{}
	for i, s := range p.Stories {
		if s.Position != i+1 || s.StoryID == "" || s.Title == "" {
			t.Fatalf("story %d = %+v", i, s)
		}
		if strings.Contains(s.Title, "<br>") || strings.Contains(s.Title, " ") {
			t.Fatalf("title not cleaned: %q", s.Title)
		}
		for _, id := range s.AppIDs() {
			distinct[id] = true
		}
	}
	if len(distinct) < 60 {
		t.Fatalf("distinct featured apps = %d, want >= 60", len(distinct))
	}
	if p.Stories[0].Heading == "" || len(p.Stories[0].Apps) == 0 {
		t.Fatalf("first story = %+v", p.Stories[0])
	}
}

func TestParseSeeAllPage(t *testing.T) {
	p, err := ParseSeeAllPage(fixtureModel(t, "seeall_reviews_spotify_us.html"))
	if err != nil {
		t.Fatal(err)
	}
	if p.AppID != "324684580" || p.SeeAll != "reviews" {
		t.Fatalf("see-all = %s %s", p.AppID, p.SeeAll)
	}
	if len(p.Reviews) != 10 {
		t.Fatalf("reviews = %d, want 10", len(p.Reviews))
	}
	for i, r := range p.Reviews {
		if r.ID == "" || r.Rating < 1 || r.Rating > 5 || !isRFC3339UTC(r.ReviewedAt) {
			t.Fatalf("review %d = %+v", i, r)
		}
		if !strings.HasPrefix(r.Author, "reviewer-") || !strings.HasPrefix(r.Title, "Sample title") {
			t.Fatalf("fixture review %d is not synthetic: %+v", i, r)
		}
	}
}

func TestParseSearchPageSynthetic(t *testing.T) {
	m := &PageModel{
		Kind:   "SearchResultsPageIntent",
		Intent: map[string]any{"$kind": "SearchResultsPageIntent", "term": "spotify"},
		Data: map[string]any{
			"shelves": []any{map[string]any{"items": []any{
				map[string]any{"$kind": "Lockup", "adamId": "324684580", "title": "Spotify"},
				map[string]any{"$kind": "Lockup", "adamId": "835599320", "title": "TikTok"},
			}}},
			"nextPage": map[string]any{"remainingContent": []any{map[string]any{"id": "389801252"}, map[string]any{"id": "324684580"}}},
		},
	}
	p, err := ParseSearchPage(m)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(p.RankedIDs(), ",")
	if got != "324684580,835599320,389801252" {
		t.Fatalf("ranked ids = %s", got)
	}
	if _, err := ParseSearchPage(&PageModel{Kind: KindChartsPage}); err == nil {
		t.Fatal("expected a kind error")
	}
}

func TestParseLookupBody(t *testing.T) {
	cases := []struct {
		fixture    string
		results    int
		firstID    string
		wantArtist bool
	}{
		{"lookup_spotify_us.json", 1, "324684580", false},
		{"lookup_batch_us.json", 30, "835599320", false},
		{"search_spotify_us.json", 5, "324684580", false},
		{"lookup_developer_spotify_us.json", 4, "324684580", true},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			results, artist, _, err := ParseLookupBody(readFixture(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != tc.results || results[0].AppID() != tc.firstID {
				t.Fatalf("results = %d first %s", len(results), results[0].AppID())
			}
			if (artist != nil) != tc.wantArtist {
				t.Fatalf("artist = %+v", artist)
			}
			for _, r := range results {
				if r.TrackID == 0 || r.TrackName == "" || r.BundleID == "" || r.Version == "" || r.Currency == "" {
					t.Fatalf("incomplete result %+v", r)
				}
				if len(r.Raw) < 40 {
					t.Fatalf("raw has %d keys", len(r.Raw))
				}
			}
		})
	}
	t.Run("44 typed fields", func(t *testing.T) {
		results, _, _, err := ParseLookupBody(readFixture(t, "lookup_spotify_us.json"))
		if err != nil {
			t.Fatal(err)
		}
		r := results[0]
		if len(LookupFieldNames) != 44 {
			t.Fatalf("LookupFieldNames = %d", len(LookupFieldNames))
		}
		for _, k := range LookupFieldNames {
			if _, ok := r.Raw[k]; !ok {
				t.Fatalf("raw missing %s", k)
			}
		}
		if r.UserRatingCount <= 1_000_000 || r.AverageUserRating < 4 || r.PrimaryGenreID != 6011 || r.ArtistID != 324684583 {
			t.Fatalf("typed values = %+v", r)
		}
		if r.CanonicalURL() != "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580" {
			t.Fatalf("canonical url = %s", r.CanonicalURL())
		}
	})
	t.Run("lenient types", func(t *testing.T) {
		results, _, _, err := ParseLookupBody([]byte(`{"resultCount":1,"results":[{"wrapperType":"software","trackId":"42","price":"1.99","userRatingCount":"7","trackName":"X"}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if results[0].TrackID != 42 || results[0].Price != 1.99 || results[0].UserRatingCount != 7 {
			t.Fatalf("coerced = %+v", results[0])
		}
	})
}

func TestParseChartFeed(t *testing.T) {
	cases := []struct {
		fixture string
		rows    int
		genre   string
	}{
		{"rss_topgrossing_us_6014.json", 100, "6014"},
		{"rss_topfree_us.json", 25, ""},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			updated, entries, err := ParseChartFeed(readFixture(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != tc.rows {
				t.Fatalf("rows = %d, want %d", len(entries), tc.rows)
			}
			if !isRFC3339UTC(updated) {
				t.Fatalf("updated = %q", updated)
			}
			seen := map[string]bool{}
			for i, e := range entries {
				if e.Rank != i+1 || !IsAppID(e.AppID) || e.Name == "" || e.Currency != "USD" {
					t.Fatalf("row %d = %+v", i, e)
				}
				if tc.genre != "" && e.GenreID != tc.genre {
					t.Fatalf("row %d genre %s", i, e.GenreID)
				}
				if seen[e.AppID] {
					t.Fatalf("duplicate app %s", e.AppID)
				}
				seen[e.AppID] = true
			}
		})
	}
	t.Run("single entry object and empty feed", func(t *testing.T) {
		_, one, err := ParseChartFeed([]byte(`{"feed":{"entry":{"id":{"label":"x","attributes":{"im:id":"1"}},"im:name":{"label":"A"}}}}`))
		if err != nil || len(one) != 1 {
			t.Fatalf("single entry: %v %d", err, len(one))
		}
		_, none, err := ParseChartFeed([]byte(`{"feed":{"updated":{"label":"2026-10-23T05:00:00-07:00"}}}`))
		if err != nil || len(none) != 0 {
			t.Fatalf("empty feed: %v %d", err, len(none))
		}
	})
}

func TestParseReviewsFeed(t *testing.T) {
	cases := []struct {
		fixture    string
		oldestYear int
	}{
		{"reviews_rss_spotify_us_p1_recent.json", 2026},
		{"reviews_rss_chatgpt_us_p10_helpful.json", 2023},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			reviews, err := ParseReviewsFeed(readFixture(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			if len(reviews) != 50 {
				t.Fatalf("reviews = %d, want 50", len(reviews))
			}
			ids := map[string]bool{}
			for i, r := range reviews {
				if r.Rating < 1 || r.Rating > 5 {
					t.Fatalf("review %d rating %d", i, r.Rating)
				}
				if r.ID == "" || ids[r.ID] || r.Version == "" || !isRFC3339UTC(r.ReviewedAt) {
					t.Fatalf("review %d = %+v", i, r)
				}
				ids[r.ID] = true
				if !strings.HasPrefix(r.Author, "reviewer-") || !strings.HasPrefix(r.Body, "Sample body") {
					t.Fatalf("fixture review %d is not synthetic", i)
				}
			}
			oldest, newest := reviewSpan(reviews)
			if oldest == "" || newest < oldest {
				t.Fatalf("span %s..%s", oldest, newest)
			}
			if y, _ := time.Parse(time.RFC3339, oldest); y.Year() != tc.oldestYear {
				t.Fatalf("oldest = %s, want year %d", oldest, tc.oldestYear)
			}
		})
	}
	t.Run("metadata entry skipped", func(t *testing.T) {
		body := `{"feed":{"entry":[{"im:name":{"label":"App"},"id":{"label":"324684580"}},{"id":{"label":"9"},"im:rating":{"label":"4"},"title":{"label":"t"},"content":{"label":"b"},"updated":{"label":"2026-10-21T20:55:47-07:00"}}]}}`
		reviews, err := ParseReviewsFeed([]byte(body))
		if err != nil || len(reviews) != 1 || reviews[0].ID != "9" || reviews[0].ReviewedAt != "2026-10-22T03:55:47Z" {
			t.Fatalf("got %v %+v", err, reviews)
		}
	})
}

func TestParseHintsPlist(t *testing.T) {
	hints, err := ParseHintsPlist(readFixture(t, "hints_insta_us.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(hints) < 5 {
		t.Fatalf("hints = %v, want >= 5", hints)
	}
	for _, h := range hints {
		if !strings.HasPrefix(h, "insta") {
			t.Fatalf("hint %q does not start with insta", h)
		}
	}
	if hints[0] != "instagram" {
		t.Fatalf("first hint = %q", hints[0])
	}
	if _, err := ParseHintsPlist([]byte("not xml")); err == nil {
		t.Fatal("expected an error for a non-plist body")
	}
}

func TestParseAPIFixtures(t *testing.T) {
	page, err := ParseAPIReviews(readFixture(t, "api_reviews_candycrush_us.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Reviews) != 10 || page.Next == "" {
		t.Fatalf("api reviews = %d next %q", len(page.Reviews), page.Next)
	}
	if page.Reviews[0].DeveloperResponse == "" || !isRFC3339UTC(page.Reviews[0].ReviewedAt) {
		t.Fatalf("first api review = %+v", page.Reviews[0])
	}
	app, err := ParseAPIApp(readFixture(t, "api_app_soda_us.json"))
	if err != nil {
		t.Fatal(err)
	}
	if app.AppID != "850417475" || len(app.InAppPurchases) != 10 || len(app.Tags) == 0 || len(app.Histogram) != 5 {
		t.Fatalf("api app = %+v", app)
	}
	if app.InAppPurchases[0].Price <= 0 || !strings.HasPrefix(app.InAppPurchases[0].PriceFormatted, "$") {
		t.Fatalf("first IAP = %+v", app.InAppPurchases[0])
	}
	if app.Histogram[0] < app.Histogram[4] {
		t.Fatalf("api histogram not converted to 5-star first: %v", app.Histogram)
	}
}

func TestParseAppleTime(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Mon Oct 19 2026 13:57:56 GMT+0000 (Coordinated Universal Time)", "2026-10-19T13:57:56Z"},
		{"Thu Oct 08 2026 07:37:17 GMT+0000 (Coordinated Universal Time)", "2026-10-08T07:37:17Z"},
		{"2026-10-12T07:00:00.000Z", "2026-10-12T07:00:00Z"},
		{"2026-10-21T20:55:47-07:00", "2026-10-22T03:55:47Z"},
		{"2026-10-19", "2026-10-19T00:00:00Z"},
		{"not a date", "not a date"},
	}
	for _, tc := range cases {
		if got := normalizeDate(tc.in); got != tc.want {
			t.Errorf("normalizeDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A product page positively shows "no in-app purchases" or "no privacy
// label" only when the section rendered with nothing in it; a section the
// parser cannot read, or a page without it, is not an observed absence.
func TestProductPageShowsNoIAPOrPrivacy(t *testing.T) {
	spotify, err := ParseProductPage(fixtureModel(t, "product_spotify_us.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !spotify.IAPNoneShown || len(spotify.InAppPurchases) != 0 {
		t.Fatalf("Spotify US lists no IAP and its offer has none: IAPNoneShown = %v", spotify.IAPNoneShown)
	}
	if spotify.Privacy.NoneShown || len(spotify.Privacy.Types) == 0 {
		t.Fatalf("Spotify declares a label: NoneShown = %v types %d", spotify.Privacy.NoneShown, len(spotify.Privacy.Types))
	}
	candy, err := ParseProductPage(fixtureModel(t, "product_candycrush_us.html"))
	if err != nil {
		t.Fatal(err)
	}
	if candy.IAPNoneShown || len(candy.InAppPurchases) != 10 {
		t.Fatalf("Candy Crush lists 10 IAPs: IAPNoneShown = %v", candy.IAPNoneShown)
	}

	// mutate edits the privacy shelves of a fresh Candy Crush model.
	mutate := func(edit func(sm map[string]any)) Privacy {
		t.Helper()
		m := fixtureModel(t, "product_candycrush_us.html")
		sm, _ := m.Data["shelfMapping"].(map[string]any)
		edit(sm)
		p, err := ParseProductPage(m)
		if err != nil {
			t.Fatal(err)
		}
		return p.Privacy
	}
	// "No Details Provided": the section renders its header and no types.
	none := mutate(func(sm map[string]any) {
		header, _ := sm["privacyHeader"].(map[string]any)
		page, _ := header["seeAllAction"].(map[string]any)["pageData"].(map[string]any)
		shelves, _ := page["shelves"].([]any)
		page["shelves"] = shelves[:1] // keep the PrivacyHeader shelf only
		sm["privacyTypes"].(map[string]any)["items"] = []any{}
	})
	if !none.NoneShown || len(none.Types) != 0 {
		t.Fatalf("an App Privacy section without types must be a positive absence: %+v", none)
	}
	// Items of a kind the parser does not know: a parse gap, not an absence.
	renamed := mutate(func(sm map[string]any) {
		header, _ := sm["privacyHeader"].(map[string]any)
		page, _ := header["seeAllAction"].(map[string]any)["pageData"].(map[string]any)
		for _, shelf := range page["shelves"].([]any) {
			for _, it := range shelf.(map[string]any)["items"].([]any) {
				if o := it.(map[string]any); o["$kind"] == "PrivacyType" {
					o["$kind"] = "PrivacyTypeV2"
				}
			}
		}
		for _, it := range sm["privacyTypes"].(map[string]any)["items"].([]any) {
			it.(map[string]any)["$kind"] = "PrivacyTypeV2"
		}
	})
	if renamed.NoneShown || len(renamed.Types) != 0 {
		t.Fatalf("unreadable label items must not count as no label: %+v", renamed)
	}
	// No App Privacy section at all: nothing observed.
	missing := mutate(func(sm map[string]any) {
		delete(sm, "privacyHeader")
		delete(sm, "privacyTypes")
	})
	if missing.NoneShown {
		t.Fatal("a page without the App Privacy section must not count as no label")
	}
}
