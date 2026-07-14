// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written tests for the transcendence command logic.

package cli

import (
	"math"
	"testing"
)

// hashtag fixture with a [{date,value}] popularity curve.
func testHashtag(name string, publish, pop float64, industries, creators []string) hashtagRow {
	curve := []any{
		map[string]any{"date": "2026-07-01", "value": float64(pop / 2)},
		map[string]any{"date": "2026-07-07", "value": float64(pop)},
	}
	return hashtagRow{
		ID:             name,
		Name:           name,
		PublishCnt:     publish,
		Popularity:     pop,
		PopularityLast: pop,
		PopularityPrev: pop / 2,
		IndustryIDs:    industries,
		TopCreators:    creators,
		CountryCode:    "US",
		raw: map[string]any{
			"popularityCurve": curve,
			"videoList": []any{
				map[string]any{"itemID": "v-" + name, "title": name + " video", "playCount": float64(pop)},
			},
		},
	}
}

func TestCurveMaxAndLast_Objects(t *testing.T) {
	curve := []any{
		map[string]any{"value": float64(10)},
		map[string]any{"value": float64(40)},
		map[string]any{"value": float64(25)},
	}
	max, last := curveMaxAndLast(curve)
	if max != 40 || last != 25 {
		t.Fatalf("max,last = %v,%v want 40,25", max, last)
	}
}

func TestCurveMaxAndLast_PairArrays(t *testing.T) {
	curve := []any{[]any{float64(1), float64(5)}, []any{float64(2), float64(9)}}
	max, last := curveMaxAndLast(curve)
	if max != 9 || last != 9 {
		t.Fatalf("max,last = %v,%v want 9,9", max, last)
	}
}

func TestCurveMaxAndLast_Empty(t *testing.T) {
	max, last := curveMaxAndLast(nil)
	if max != 0 || last != 0 {
		t.Fatalf("empty curve should be 0,0 got %v,%v", max, last)
	}
}

func TestDecodeHashtag_StringPublishCnt(t *testing.T) {
	obj := map[string]any{
		"hashtagID":   "123",
		"hashtagName": "gaming",
		"publishCnt":  "1500",
		"rankIndex":   "3",
		"popularityCurve": []any{
			map[string]any{"value": "200"},
		},
	}
	row := decodeHashtag(obj)
	if row.PublishCnt != 1500 || row.RankIndex != 3 || row.Popularity != 200 {
		t.Fatalf("decoded row = %+v", row)
	}
}

func TestOpportunityScore(t *testing.T) {
	row := testHashtag("niche", 100, 1000, nil, nil)
	if got := opportunityScore(row); got != 10 {
		t.Fatalf("opportunityScore = %v want 10", got)
	}
	// Zero publish count is clamped to 1 (pure-popularity signal).
	zero := testHashtag("zero", 0, 500, nil, nil)
	if got := opportunityScore(zero); got != 500 {
		t.Fatalf("opportunityScore zero-publish = %v want 500", got)
	}
}

func TestViralRank_OrderAndTruncate(t *testing.T) {
	rows := []hashtagRow{
		testHashtag("a", 1000, 100, nil, nil),  // score 0.1
		testHashtag("b", 10, 500, nil, nil),    // score 50
		testHashtag("c", 100, 1000, nil, nil),  // score 10
	}
	got := viralRank(rows, 2)
	if len(got) != 2 || got[0].Name != "b" || got[1].Name != "c" {
		t.Fatalf("viralRank order = %+v", got)
	}
}

func TestSlopeTrend(t *testing.T) {
	rising := testHashtag("r", 10, 100, nil, nil) // prev 50, last 100
	delta, label := slopeTrend(rising)
	if label != "rising" || delta != 50 {
		t.Fatalf("rising: delta=%v label=%v", delta, label)
	}
	falling := testHashtag("f", 10, 100, nil, nil)
	falling.PopularityPrev = 200
	falling.PopularityLast = 100
	delta, label = slopeTrend(falling)
	if label != "falling" || delta != -100 {
		t.Fatalf("falling: delta=%v label=%v", delta, label)
	}
}

func TestMatchNiche(t *testing.T) {
	row := testHashtag("marvelrivals", 10, 100, []string{"gaming", "marvel"}, nil)
	if !matchNiche(row, "marvel") {
		t.Fatal("should match by industry")
	}
	if !matchNiche(row, "rivals") {
		t.Fatal("should match by name substring")
	}
	if matchNiche(row, "cooking") {
		t.Fatal("should not match unrelated niche")
	}
	if !matchNiche(row, "") {
		t.Fatal("empty niche matches all")
	}
}

func TestSharedIndustriesAndCreators(t *testing.T) {
	a := testHashtag("a", 1, 1, []string{"gaming", "sports"}, []string{"u1", "u2"})
	b := testHashtag("b", 1, 1, []string{"gaming", "music"}, []string{"u2", "u3"})
	if got := sharedIndustries(a, b); len(got) != 1 || got[0] != "gaming" {
		t.Fatalf("sharedIndustries = %+v", got)
	}
	if got := sharedCreators(a, b); len(got) != 1 || got[0] != "u2" {
		t.Fatalf("sharedCreators = %+v", got)
	}
}

func TestRankSimilar(t *testing.T) {
	target := testHashtag("target", 1, 1, []string{"gaming"}, []string{"u1"})
	other := testHashtag("other", 1, 1, []string{"gaming"}, []string{"u1"})
	unrelated := testHashtag("unrelated", 1, 1, []string{"cooking"}, nil)
	got := rankSimilar(target, []hashtagRow{other, unrelated})
	if len(got) != 1 || got[0].Hashtag != "other" {
		t.Fatalf("rankSimilar = %+v", got)
	}
	if got[0].SimilarityReason != "shared_industries_and_creators" {
		t.Fatalf("reason = %q", got[0].SimilarityReason)
	}
}

func TestFindHashtag(t *testing.T) {
	rows := []hashtagRow{
		testHashtag("marvelrivalss9", 1, 1, nil, nil),
	}
	if _, ok := findHashtag(rows, "marvelrivalss9"); !ok {
		t.Fatal("exact name match failed")
	}
	if _, ok := findHashtag(rows, "marvel"); !ok {
		t.Fatal("substring name match failed")
	}
	if _, ok := findHashtag(rows, "nonexistent"); ok {
		t.Fatal("should not find nonexistent")
	}
}

func TestBuildNicheBrief(t *testing.T) {
	rows := []hashtagRow{
		testHashtag("marvelrivals", 100, 900, []string{"gaming"}, []string{"creator1"}),
		testHashtag("cooking", 50, 10, []string{"food"}, nil),
	}
	brief := buildNicheBrief("marvel", "US", rows)
	if len(brief.TrendingTags) != 1 || brief.TrendingTags[0].Hashtag != "marvelrivals" {
		t.Fatalf("trending tags = %+v", brief.TrendingTags)
	}
	if len(brief.TopCreators) != 1 || brief.TopCreators[0] != "creator1" {
		t.Fatalf("top creators = %+v", brief.TopCreators)
	}
	if len(brief.Representative) != 1 || brief.Representative[0].Hashtag != "marvelrivals" {
		t.Fatalf("representative = %+v", brief.Representative)
	}
}

func TestBuildContentFeed(t *testing.T) {
	rows := []hashtagRow{testHashtag("marvelrivals", 100, 900, []string{"gaming"}, nil)}
	ads := []topAdRow{{Title: "Marvel ad", Author: "creatorX", Popularity: 5000, Keywords: []string{"marvel"}}}
	items := buildContentFeed("marvel", rows, ads)
	if len(items) != 2 {
		t.Fatalf("content items = %d want 2", len(items))
	}
	// Top ad (5000) ranks above representative video (900).
	ranked := rankContentByPopularity(items, 10)
	if ranked[0].Source != "top_ad" {
		t.Fatalf("expected top_ad first, got %+v", ranked[0])
	}
}

func TestBuildContentFeed_NoNicheReturnsAll(t *testing.T) {
	rows := []hashtagRow{testHashtag("a", 1, 1, nil, nil)}
	ads := []topAdRow{{Title: "ad", Popularity: 1}}
	items := buildContentFeed("", rows, ads)
	if len(items) != 2 {
		t.Fatalf("empty niche should return all, got %d", len(items))
	}
}

func TestBuildCompetitorBrief(t *testing.T) {
	ads := []topAdRow{
		{Title: "Best marvel rivals tips", Author: "myketowersmtyk", Popularity: 8000, Keywords: []string{"marvel"}},
		{Title: "Cooking pasta", Author: "chef", Popularity: 100},
	}
	rows := []hashtagRow{testHashtag("marvelrivals", 1, 1, nil, nil)}
	brief := buildCompetitorBrief("myke", ads, rows)
	if len(brief.TopContent) != 1 || brief.TopContent[0].Title != "Best marvel rivals tips" {
		t.Fatalf("top content = %+v", brief.TopContent)
	}
	if len(brief.HashtagsRidden) != 1 || brief.HashtagsRidden[0] != "marvelrivals" {
		t.Fatalf("hashtags ridden = %+v", brief.HashtagsRidden)
	}
	if brief.Positioning == "" {
		t.Fatal("positioning should be non-empty")
	}
}

func TestBuildDecision(t *testing.T) {
	rows := []hashtagRow{
		testHashtag("marvelrivals", 100, 900, []string{"gaming"}, []string{"c1"}),
		testHashtag("marvelheroes", 5, 800, []string{"gaming"}, nil), // low publish = high opportunity
	}
	ads := []topAdRow{{Title: "Marvel tips video", Author: "x", Popularity: 7000, Keywords: []string{"marvel"}}}
	brief := buildDecision("marvel", "US", rows, ads)

	if len(brief.TrendingHashtags) == 0 {
		t.Fatal("expected trending hashtags")
	}
	if len(brief.WhiteSpace) == 0 {
		t.Fatal("expected white space entries")
	}
	// Lower-publish marvelheroes should outrank marvelrivals on opportunity.
	if brief.WhiteSpace[0].Hashtag != "marvelheroes" {
		t.Fatalf("top white space = %q want marvelheroes", brief.WhiteSpace[0].Hashtag)
	}
	if len(brief.Recommendation.HashtagsToRide) == 0 {
		t.Fatal("expected hashtags to ride")
	}
	if brief.Recommendation.OpenAccountAngle == "" {
		t.Fatal("expected open account angle")
	}
}

func TestBuildDecision_NoMatch(t *testing.T) {
	rows := []hashtagRow{testHashtag("cooking", 1, 1, []string{"food"}, nil)}
	brief := buildDecision("marvel", "US", rows, nil)
	if len(brief.TrendingHashtags) != 0 || len(brief.WhiteSpace) == 0 {
		// WhiteSpace spans ALL hashtags, so it may still be non-empty;
		// trending should be empty since no marvel match.
		if len(brief.TrendingHashtags) != 0 {
			t.Fatalf("expected no trending hashtags for unrelated niche, got %+v", brief.TrendingHashtags)
		}
	}
}

func TestWatchCheck_Crossing(t *testing.T) {
	list := []watchEntry{
		{Hashtag: "rising", Threshold: 500, LastPopularity: 100, HasBaseline: true},
		{Hashtag: "stable", Threshold: 5000, LastPopularity: 100, HasBaseline: true},
	}
	rows := []hashtagRow{
		testHashtag("rising", 1, 600, nil, nil),
		testHashtag("stable", 1, 200, nil, nil),
	}
	report, updated := checkWatchlist(list, rows)
	if len(report.Crossed) != 1 || report.Crossed[0].Hashtag != "rising" {
		t.Fatalf("crossed = %+v", report.Crossed)
	}
	// updated list refreshes lastPopularity to current.
	var rising watchEntry
	for _, e := range updated {
		if e.Hashtag == "rising" {
			rising = e
		}
	}
	if rising.LastPopularity != 600 {
		t.Fatalf("updated lastPopularity = %v want 600", rising.LastPopularity)
	}
}

// TestWatchCheck_NoFalseCrossingOnFirstCheck reproduces the bug where a
// freshly-added entry (no baseline yet) that's already above threshold used
// to report a false crossing on its first check, because LastPopularity==0
// was ambiguous with "never checked". It should now only establish a
// baseline and report stable.
func TestWatchCheck_NoFalseCrossingOnFirstCheck(t *testing.T) {
	list := []watchEntry{
		{Hashtag: "alreadyhigh", Threshold: 50}, // HasBaseline defaults to false
	}
	rows := []hashtagRow{
		testHashtag("alreadyhigh", 1, 90, nil, nil),
	}
	report, updated := checkWatchlist(list, rows)
	if len(report.Crossed) != 0 {
		t.Fatalf("expected no crossing on first check, got %+v", report.Crossed)
	}
	if len(report.Stable) != 1 || report.Stable[0].Hashtag != "alreadyhigh" {
		t.Fatalf("expected stable = [alreadyhigh], got %+v", report.Stable)
	}
	if !updated[0].HasBaseline {
		t.Fatalf("expected HasBaseline=true after first check")
	}
	// A second check with the same value now correctly has a baseline and
	// still should not cross (no new rise since baseline).
	report2, _ := checkWatchlist(updated, rows)
	if len(report2.Crossed) != 0 {
		t.Fatalf("expected no crossing on stable second check, got %+v", report2.Crossed)
	}
}

// TestWatchCheck_MissingRowDoesNotStampSyntheticBaseline reproduces the bug
// where a watched hashtag temporarily absent from the local rows (e.g. a
// scoped sync that omits it) was stamped with HasBaseline=true and
// LastPopularity=0 anyway. If the hashtag reappeared above threshold on a
// later sync, that synthetic zero baseline made it look like a real
// crossing even though nothing was actually observed rising through the
// threshold.
func TestWatchCheck_MissingRowDoesNotStampSyntheticBaseline(t *testing.T) {
	list := []watchEntry{
		{Hashtag: "sometimesmissing", Threshold: 50, HasBaseline: true, LastPopularity: 60},
	}
	// The hashtag is absent from this sync's rows.
	var rows []hashtagRow

	report, updated := checkWatchlist(list, rows)
	if len(report.Crossed) != 0 {
		t.Fatalf("expected no crossing when row is missing, got %+v", report.Crossed)
	}
	if !updated[0].HasBaseline || updated[0].LastPopularity != 60 {
		t.Fatalf("expected prior baseline preserved when row missing, got %+v", updated[0])
	}

	// Hashtag reappears above threshold on a later sync — this must NOT be
	// treated as a crossing from a synthetic zero baseline; it should
	// compare against the preserved real baseline (60 >= 50 already).
	rows = []hashtagRow{testHashtag("sometimesmissing", 1, 70, nil, nil)}
	report2, _ := checkWatchlist(updated, rows)
	if len(report2.Crossed) != 0 {
		t.Fatalf("expected no crossing on reappearance already-above-threshold, got %+v", report2.Crossed)
	}
}

func TestWatchUpsertAndRemove(t *testing.T) {
	list := []watchEntry{{Hashtag: "a", Threshold: 1}}
	list = upsertWatchEntry(list, watchEntry{Hashtag: "a", Threshold: 9})
	if len(list) != 1 || list[0].Threshold != 9 {
		t.Fatalf("upsert replace failed: %+v", list)
	}
	list = upsertWatchEntry(list, watchEntry{Hashtag: "b", Threshold: 2})
	if len(list) != 2 {
		t.Fatalf("upsert add failed: %+v", list)
	}
	list = removeWatchEntry(list, "a")
	if len(list) != 1 || list[0].Hashtag != "b" {
		t.Fatalf("remove failed: %+v", list)
	}
}

func TestParseDurationArg(t *testing.T) {
	cases := map[string]float64{"24h": 24, "7d": 7 * 24, "2w": 14 * 24, "1m": 30 * 24}
	for in, wantHours := range cases {
		d, err := parseDurationArg(in)
		if err != nil {
			t.Fatalf("parseDurationArg(%q) err: %v", in, err)
		}
		if got := d.Hours(); got != wantHours {
			t.Fatalf("parseDurationArg(%q) = %v hours want %v", in, got, wantHours)
		}
	}
	if _, err := parseDurationArg("bogus"); err == nil {
		t.Fatal("expected error for bogus duration")
	}
}

func TestMaskSecret(t *testing.T) {
	if got := maskSecret("abcdefgh"); got != "****efgh" {
		t.Fatalf("maskSecret = %q", got)
	}
	if got := maskSecret("ab"); got != "****" {
		t.Fatalf("short maskSecret = %q", got)
	}
	if got := maskSecret(""); got != "" {
		t.Fatalf("empty maskSecret = %q", got)
	}
}

func TestHeaderLookup(t *testing.T) {
	m := map[string]string{"X-CSRFToken": "tok123", "Cookie": "a=b"}
	if got := headerLookup(m, "x-csrftoken"); got != "tok123" {
		t.Fatalf("headerLookup = %q", got)
	}
	if got := headerLookup(m, "cookie"); got != "a=b" {
		t.Fatalf("headerLookup cookie = %q", got)
	}
	if got := headerLookup(m, "missing"); got != "" {
		t.Fatalf("missing headerLookup = %q", got)
	}
}

func TestAsFloat(t *testing.T) {
	if got := asFloat("12.5"); got != 12.5 {
		t.Fatalf("asFloat string = %v", got)
	}
	if got := asFloat(float64(3)); got != 3 {
		t.Fatalf("asFloat float = %v", got)
	}
	if !math.IsNaN(asFloat("nope")) {
		t.Fatal("asFloat non-numeric should be NaN")
	}
}

func TestTopAdsInNiche(t *testing.T) {
	ads := []topAdRow{
		{Title: "Marvel ad", Popularity: 100},
		{Title: "Cooking ad", Popularity: 50},
	}
	got := topAdsInNiche("marvel", ads, 10)
	if len(got) != 1 || got[0].Title != "Marvel ad" {
		t.Fatalf("topAdsInNiche = %+v", got)
	}
}
