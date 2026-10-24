// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"strings"
	"testing"
)

func TestStorefronts(t *testing.T) {
	if n := len(ActiveStorefrontCodes()); n != 175 {
		t.Fatalf("active storefronts = %d, want 175", n)
	}
	if n := len(Storefronts()); n != 177 {
		t.Fatalf("all storefronts = %d, want 177 (175 + BD, LI legacy)", n)
	}
	cases := []struct {
		code string
		id   int
	}{
		{"us", 143441}, {"pk", 143477}, {"gb", 143444}, {"jp", 143462}, {"in", 143467},
		{"US", 143441}, {"uk", 143444}, {"pakistan", 143477}, {"xk", 143624}, {"bd", 143490},
	}
	for _, tc := range cases {
		id, ok := StorefrontID(tc.code)
		if !ok || id != tc.id {
			t.Errorf("StorefrontID(%q) = %d, %v; want %d", tc.code, id, ok, tc.id)
		}
	}
	ids := map[int]string{}
	for _, sf := range Storefronts() {
		if len(sf.Code) != 2 || sf.Code != strings.ToLower(sf.Code) || sf.ID < 143441 || sf.Name == "" {
			t.Fatalf("bad storefront row %+v", sf)
		}
		if prev, dup := ids[sf.ID]; dup {
			t.Fatalf("duplicate storefront id %d (%s, %s)", sf.ID, prev, sf.Code)
		}
		ids[sf.ID] = sf.Code
	}
	if h, err := StoreFrontHeader("us", 12); err != nil || h != "143441,12" {
		t.Fatalf("StoreFrontHeader = %q, %v", h, err)
	}
}

func TestParseCountries(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"us,gb,pk", "us,gb,pk", false},
		{" US , uk ,us", "us,gb", false},
		{"jp", "jp", false},
		{"", "", true},
		{"us,zz", "", true},
		{",,", "", true},
	}
	for _, tc := range cases {
		got, err := ParseCountries(tc.in)
		if (err != nil) != tc.err || strings.Join(got, ",") != tc.want {
			t.Errorf("ParseCountries(%q) = %v, %v; want %q err=%v", tc.in, got, err, tc.want, tc.err)
		}
	}
	all, err := ParseCountries("all")
	if err != nil || len(all) != 175 {
		t.Fatalf("all = %d, %v", len(all), err)
	}
	for _, code := range all {
		if code == "bd" || code == "li" {
			t.Fatalf("legacy storefront %s in all", code)
		}
	}
}

func TestResolveGenre(t *testing.T) {
	cases := []struct {
		in   string
		id   string
		name string
	}{
		{"6014", "6014", "Games"},
		{"games", "6014", "Games"},
		{"Game", "6014", "Games"},
		{"Developer Tools", "6026", "Developer Tools"},
		{"devtools", "6026", "Developer Tools"},
		{"graphics & design", "6027", "Graphics & Design"},
		{"graphics and design", "6027", "Graphics & Design"},
		{"puzzle", "7012", "Puzzle"},
		{"puzzles", "7012", "Puzzle"},
		{"casual", "7003", "Casual"},
		{"arcade", "7003", "Casual"},
		{"rpg", "7014", "Role-Playing"},
		{"role-playing", "7014", "Role-Playing"},
		{"music", "6011", "Music"},
		{"games:music", "7011", "Music"},
		{"games/sports", "7016", "Sports"},
		{"all", "36", "All Apps"},
		{"36", "36", "All Apps"},
		{"food and drink", "6023", "Food & Drink"},
		{"social", "6005", "Social Networking"},
	}
	for _, tc := range cases {
		g, err := ResolveGenre(tc.in)
		if err != nil || g.ID != tc.id || g.Name != tc.name {
			t.Errorf("ResolveGenre(%q) = %+v, %v; want %s %s", tc.in, g, err, tc.id, tc.name)
		}
	}
	for _, bad := range []string{"", "9999", "notagenre", "games:banking"} {
		if _, err := ResolveGenre(bad); err == nil {
			t.Errorf("ResolveGenre(%q) accepted", bad)
		}
	}
	if GenreName("7003") != "Casual" || GenreName("12001") != "Business" || GenreName("1") != "" {
		t.Fatal("GenreName lookup broken")
	}
	games := 0
	for _, g := range Genres("ios") {
		if g.ParentID == GamesGenreID {
			games++
		}
	}
	if games != 18 {
		t.Fatalf("game subgenres = %d, want 18", games)
	}
}

func TestResolveAgeBand(t *testing.T) {
	cases := map[string]string{"kids": "0", "all": "0", "all kids": "0", "0": "0", "5-under": "10000", "6-8": "10001", "9-11": "10002", "10002": "10002"}
	for in, want := range cases {
		b, err := ResolveAgeBand(in)
		if err != nil || b.ID != want {
			t.Errorf("ResolveAgeBand(%q) = %+v, %v; want %s", in, b, err, want)
		}
	}
	if _, err := ResolveAgeBand("teens"); err == nil {
		t.Error("unknown band accepted")
	}
	// An empty value must never become the All Kids chart (ageBandId 0): a
	// caller resolving an unset --age-band would turn every overall chart
	// into the Kids chart.
	for _, in := range []string{"", "  "} {
		if b, err := ResolveAgeBand(in); err == nil {
			t.Errorf("ResolveAgeBand(%q) = %+v, want an error", in, b)
		}
	}
	if b, ok := KidsAgeBandByID("10001"); !ok || b.Name != "Ages 6-8" {
		t.Errorf("KidsAgeBandByID(10001) = %+v %v", b, ok)
	}
	if _, ok := KidsAgeBandByID(""); ok {
		t.Error("KidsAgeBandByID(\"\") found a band")
	}
}

func TestParseAppRef(t *testing.T) {
	cases := []struct {
		in      string
		id      string
		bundle  string
		country string
		err     bool
	}{
		{"324684580", "324684580", "", "", false},
		{"id324684580", "324684580", "", "", false},
		{"ID324684580", "324684580", "", "", false},
		{"https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580", "324684580", "", "us", false},
		{"https://apps.apple.com/pk/app/spotify-music-and-podcasts/id324684580?uo=4", "324684580", "", "pk", false},
		{"apps.apple.com/gb/app/id324684580", "324684580", "", "gb", false},
		{"https://apps.apple.com/us/app/324684580?see-all=reviews", "324684580", "", "us", false},
		{"https://itunes.apple.com/lookup?id=324684580", "324684580", "", "", false},
		{"com.spotify.client", "", "com.spotify.client", "", false},
		{"com.midasplayer.apps.candycrushsaga", "", "com.midasplayer.apps.candycrushsaga", "", false},
		{"", "", "", "", true},
		{"spotify", "", "", "", true},
		{"https://example.com/app/id1", "", "", "", true},
		{"https://apps.apple.com/us/iphone/charts/36", "", "", "", true},
	}
	for _, tc := range cases {
		ref, err := ParseAppRef(tc.in)
		if (err != nil) != tc.err {
			t.Errorf("ParseAppRef(%q) err = %v, want err=%v", tc.in, err, tc.err)
			continue
		}
		if err == nil && (ref.ID != tc.id || ref.BundleID != tc.bundle || ref.Country != tc.country) {
			t.Errorf("ParseAppRef(%q) = %+v; want id=%s bundle=%s country=%s", tc.in, ref, tc.id, tc.bundle, tc.country)
		}
	}
}

func TestPlatformsChartsAndSorts(t *testing.T) {
	plat := map[string]string{"": "iphone", "iPhone": "iphone", "ios": "iphone", "pad": "ipad", "macOS": "mac", "tv": "appletv", "watch": "watch", "visionpro": "vision"}
	for in, want := range plat {
		if got, err := NormalizePlatform(in); err != nil || got != want {
			t.Errorf("NormalizePlatform(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := NormalizePlatform("android"); err == nil {
		t.Error("android accepted")
	}
	feeds := []struct{ chart, platform, want string }{
		{"top-free", "iphone", "topfreeapplications"},
		{"top-paid", "iphone", "toppaidapplications"},
		{"top-grossing", "iphone", "topgrossingapplications"},
		{"grossing", "ipad", "topgrossingipadapplications"},
		{"free", "mac", "topfreemacapps"},
	}
	for _, f := range feeds {
		got, err := ChartFeedName(f.chart, f.platform)
		if err != nil || got != f.want || !LegacyChartFeeds[got] {
			t.Errorf("ChartFeedName(%s, %s) = %q, %v; want %q", f.chart, f.platform, got, err, f.want)
		}
	}
	if _, err := ChartFeedName("top-free", "watch"); err == nil {
		t.Error("watch chart feed accepted")
	}
	sorts := map[string]string{"": SortMostRecent, "recent": SortMostRecent, "mostrecent": SortMostRecent, "helpful": SortMostHelpful, "mosthelpful": SortMostHelpful}
	for in, want := range sorts {
		if got, err := NormalizeReviewSort(in); err != nil || got != want {
			t.Errorf("NormalizeReviewSort(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := NormalizeReviewSort("best"); err == nil {
		t.Error("unknown sort accepted")
	}
	if got := CanonicalAppURL("https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580?uo=4"); got != "https://apps.apple.com/us/app/spotify-music-and-podcasts/id324684580" {
		t.Errorf("CanonicalAppURL = %s", got)
	}
	if got := ChartsPageURL("us", "iphone", "36", "top-free", "10000"); got != "https://apps.apple.com/us/iphone/charts/36?ageBandId=10000&chart=top-free" {
		t.Errorf("ChartsPageURL = %s", got)
	}
	if got := ReviewsFeedURL("us", "6448311069", 1, SortMostRecent); got != "https://itunes.apple.com/us/rss/customerreviews/page=1/id=6448311069/sortby=mostrecent/json" {
		t.Errorf("ReviewsFeedURL = %s", got)
	}
	if got := ChartFeedURL("us", "topgrossingapplications", "6014", 100); got != "https://itunes.apple.com/us/rss/topgrossingapplications/limit=100/genre=6014/json" {
		t.Errorf("ChartFeedURL = %s", got)
	}
	if lookupLang("en-GB") != "" || lookupLang("ja") != "ja_jp" || lookupLang("en_us") != "en_us" {
		t.Error("lookupLang must only pass en_us and ja_jp")
	}
}
