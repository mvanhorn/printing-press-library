package malhtml

import (
	"strings"
	"testing"
)

// Fixtures below are trimmed copies of real MyAnimeList markup (verified
// 2026-09-10). Keep them byte-faithful where the anchor pattern matters: the
// dark_text label/value pairs, the score-stats table shape, and the seasonal
// tile structure are what the parsers key on.

const statsFixture = `<html><head><title>Sousou no Frieren - Statistics - MyAnimeList.net</title></head><body>
<h1 class="h1">Sousou no Frieren</h1>
<h2 id="summary_stats">Summary Stats</h2>
<div class="spaceit_pad"><span class="dark_text">Watching:</span> 241,364</div>
<div class="spaceit_pad"><span class="dark_text">Completed:</span> 997,218</div>
<div class="spaceit_pad"><span class="dark_text">On-Hold:</span> 30,950</div>
<div class="spaceit_pad"><span class="dark_text">Dropped:</span> 25,326</div>
<div class="spaceit_pad"><span class="dark_text">Plan to Watch:</span> 216,933</div>
<div class="spaceit_pad"><span class="dark_text">Total:</span> 1,511,791</div>
<br><h2 id="score_stats">Score Stats</h2>
<table border="0" cellpadding="0" cellspacing="0" width="100%" class="score-stats">
<tr>
<td width="20" class="score-label score-10">10</td>
<td><div class="spaceit_pad"><div class="updatesBar" style="float: left; height: 15px; width: 52.9%;"></div><span>&nbsp;52.9% <small>(493686 votes)</small></span></div></td>
</tr>
<tr>
<td width="20" class="score-label score-9">9</td>
<td><div class="spaceit_pad"><div class="updatesBar" style="float: left; height: 15px; width: 25.8%;"></div><span>&nbsp;25.8% <small>(240922 votes)</small></span></div></td>
</tr>
<tr>
<td width="20" class="score-label score-8">8</td>
<td><div class="spaceit_pad"><div class="updatesBar" style="float: left; height: 15px; width: 11.7%;"></div><span>&nbsp;11.7% <small>(108870 votes)</small></span></div></td>
</tr>
<tr>
<td width="20" class="score-label score-1">1</td>
<td><div class="spaceit_pad"><div class="updatesBar" style="float: left; height: 15px; width: 3.1%;"></div><span>&nbsp;3.1% <small>(29319 votes)</small></span></div></td>
</tr>
<tr>
<td width="20" class="score-label score-2">2</td>
<td><div class="spaceit_pad"><div class="updatesBar" style="float: left; height: 15px; width: 0.2%;"></div><span>&nbsp;0.2% <small>(1840 votes)</small></span></div></td>
</tr>
</table>
<div class="spaceit_pad"><span class="dark_text">Score:</span> <span class="score-label">9.29</span> (scored by 1,234,567 users)</div>
</body></html>`

const detailFixture = `<html><head><title>Cowboy Bebop (TV 1998) - MyAnimeList.net</title></head><body>
<h1 class="h1"><strong>Cowboy Bebop</strong></h1>
<div class="leftside">
<div class="spaceit_pad"><span class="dark_text">Japanese:</span> カウボーイビバップ</div>
<div class="spaceit_pad"><span class="dark_text">English:</span> Cowboy Bebop</div>
<div class="spaceit_pad"><span class="dark_text">Type:</span> TV</div>
<div class="spaceit_pad"><span class="dark_text">Episodes:</span> 26</div>
<div class="spaceit_pad"><span class="dark_text">Status:</span> Finished Airing</div>
<div class="spaceit_pad"><span class="dark_text">Aired:</span> Apr 3, 1998 to Apr 24, 1999</div>
<div class="spaceit_pad"><span class="dark_text">Premiered:</span> <a href="/anime/season/1998/spring">Spring 1998</a></div>
<div class="spaceit_pad"><span class="dark_text">Broadcast:</span> Saturdays at 01:00 (JST)</div>
<div class="spaceit_pad"><span class="dark_text">Producers:</span> <a href="/anime/producer/23/Bandai_Visual">Bandai Visual</a>, <a href="/anime/producer/123/Victor_Entertainment">Victor Entertainment</a></div>
<div class="spaceit_pad"><span class="dark_text">Licensors:</span> <a href="/anime/producer/102/Funimation">Funimation</a></div>
<div class="spaceit_pad"><span class="dark_text">Studios:</span> <a href="/anime/producer/14/Sunrise">Sunrise</a></div>
<div class="spaceit_pad"><span class="dark_text">Source:</span> Original</div>
<div class="spaceit_pad"><span class="dark_text">Genres:</span> <a href="/anime/genre/1/Action">Action</a>, <a href="/anime/genre/24/Sci-Fi">Sci-Fi</a></div>
<div class="spaceit_pad"><span class="dark_text">Themes:</span> <a href="/anime/genre/50/Adult_Cast">Adult Cast</a>, <a href="/anime/genre/29/Space">Space</a></div>
<div class="spaceit_pad"><span class="dark_text">Duration:</span> 24 min. per ep.</div>
<div class="spaceit_pad"><span class="dark_text">Rating:</span> R - 17+ (violence &amp; profanity)</div>
<div class="spaceit_pad"><span class="dark_text">Score:</span> <span class="score-label">8.75</span> (scored by 1,074,920 users)</div>
<div class="spaceit_pad"><span class="dark_text">Ranked:</span> #50<sup>2</sup></div>
<div class="spaceit_pad"><span class="dark_text">Popularity:</span> #41</div>
<div class="spaceit_pad"><span class="dark_text">Members:</span> 2,084,414</div>
<div class="spaceit_pad"><span class="dark_text">Favorites:</span> 96,123</div>
</div>
<p itemprop="description">In the year 2071, humanity has colonized planets and moons.</p>
<h2 id="related_entries">Related Entries</h2></div></div><div class="related-entries">
<div class="entries-tile">
<div class="entry borderClass "><div class="image"><a href="https://myanimelist.net/anime/5/Cowboy_Bebop_Tengoku_no_Tobira"><img src="x"></a></div><div class="information"><div class="title"><a href="https://myanimelist.net/anime/5/Cowboy_Bebop_Tengoku_no_Tobira">Cowboy Bebop: Tengoku no Tobira</a></div><div class="spaceit_pad"><span>Sequel</span></div></div></div>
<div class="entry borderClass "><div class="image"><a href="https://myanimelist.net/manga/173/Cowboy_Bebop"><img src="x"></a></div><div class="information"><div class="title"><a href="https://myanimelist.net/manga/173/Cowboy_Bebop">Cowboy Bebop</a></div><div class="spaceit_pad"><span>Adaptation</span></div></div></div>
</div></div></body></html>`

func TestParseStats(t *testing.T) {
	s, err := ParseStats("anime", 52991, statsFixture)
	if err != nil {
		t.Fatalf("ParseStats: %v", err)
	}
	if s.Title != "Sousou no Frieren" {
		t.Errorf("title = %q", s.Title)
	}
	if len(s.Buckets) != 5 {
		t.Fatalf("buckets = %d, want 5", len(s.Buckets))
	}
	if s.Buckets[0].Score != 10 || s.Buckets[0].Votes != 493686 {
		t.Errorf("first bucket = %+v", s.Buckets[0])
	}
	if s.Watching != 241364 || s.Completed != 997218 || s.Dropped != 25326 {
		t.Errorf("status counts wrong: %+v", s)
	}
	if s.Total != 1511791 {
		t.Errorf("total = %d", s.Total)
	}
}

func TestParseStatsRejectsBasePage(t *testing.T) {
	if _, err := ParseStats("anime", 1, detailFixture); err == nil {
		t.Fatal("expected an error when handed a detail page instead of a stats page")
	}
}

func TestParseDetail(t *testing.T) {
	d, err := ParseDetail("anime", 1, detailFixture)
	if err != nil {
		t.Fatalf("ParseDetail: %v", err)
	}
	if d.Title != "Cowboy Bebop" {
		t.Errorf("title = %q", d.Title)
	}
	if d.Type != "TV" || d.Episodes != 26 {
		t.Errorf("type/episodes = %q/%d", d.Type, d.Episodes)
	}
	if d.Broadcast != "Saturdays at 01:00 (JST)" {
		t.Errorf("broadcast = %q", d.Broadcast)
	}
	if len(d.Genres) != 2 || d.Genres[0] != "Action" {
		t.Errorf("genres = %v", d.Genres)
	}
	if len(d.Studios) != 1 || d.Studios[0] != "Sunrise" {
		t.Errorf("studios = %v", d.Studios)
	}
	if d.Score != 8.75 {
		t.Errorf("score = %v", d.Score)
	}
	if d.Rank != 50 || d.Popularity != 41 || d.Members != 2084414 {
		t.Errorf("rank/popularity/members = %d/%d/%d", d.Rank, d.Popularity, d.Members)
	}
	if !strings.Contains(d.Synopsis, "2071") {
		t.Errorf("synopsis = %q", d.Synopsis)
	}
	if len(d.Related) != 2 || d.Related[0].Relation != "Sequel" || d.Related[1].Kind != "manga" {
		t.Errorf("related = %+v", d.Related)
	}
}

func TestDivisiveness(t *testing.T) {
	// Bimodal: half tens, half ones -> high index.
	bimodal := &Stats{Kind: "anime", ID: 1, Total: 1000, Buckets: []Bucket{
		{Score: 10, Percent: 40, Votes: 400},
		{Score: 1, Percent: 40, Votes: 400},
		{Score: 5, Percent: 20, Votes: 200},
	}}
	d, err := bimodal.Divisiveness()
	if err != nil {
		t.Fatalf("Divisiveness: %v", err)
	}
	if d.Index < 60 {
		t.Errorf("bimodal index = %v, want a high polarization reading", d.Index)
	}
	if d.Verdict != "bitterly split" {
		t.Errorf("bimodal verdict = %q", d.Verdict)
	}
	// Flat: everything at 8 -> low index.
	flat := &Stats{Kind: "anime", ID: 2, Total: 1000, Buckets: []Bucket{
		{Score: 10, Percent: 5, Votes: 50},
		{Score: 8, Percent: 90, Votes: 900},
		{Score: 1, Percent: 1, Votes: 10},
	}}
	f, err := flat.Divisiveness()
	if err != nil {
		t.Fatalf("Divisiveness: %v", err)
	}
	if f.Index >= d.Index {
		t.Errorf("flat index %v should be below bimodal index %v", f.Index, d.Index)
	}
	// Universally loved: a huge 9-10 share with almost no 1-2 votes must not
	// read as divisive just because enthusiasm is high.
	loved := &Stats{Kind: "anime", ID: 3, Total: 1000, Buckets: []Bucket{
		{Score: 10, Percent: 52.9, Votes: 529},
		{Score: 9, Percent: 23.1, Votes: 231},
		{Score: 8, Percent: 11.7, Votes: 117},
		{Score: 1, Percent: 3.1, Votes: 31},
		{Score: 2, Percent: 0.2, Votes: 2},
	}}
	l, err := loved.Divisiveness()
	if err != nil {
		t.Fatalf("Divisiveness: %v", err)
	}
	if l.Verdict == "bitterly split" || l.Index >= 50 {
		t.Errorf("loved title read as %q (index %v); a large 9-10 share is enthusiasm, not conflict", l.Verdict, l.Index)
	}
}

func TestDropRisk(t *testing.T) {
	s := &Stats{Total: 1000, Completed: 800, Dropped: 40, OnHold: 20, Watching: 100, PlanToWatch: 40}
	dr, err := s.DropRisk(12)
	if err != nil {
		t.Fatalf("DropRisk: %v", err)
	}
	if dr.DroppedShare != 4 {
		t.Errorf("dropped share = %v, want 4", dr.DroppedShare)
	}
	if dr.RiskBand != "moderate" {
		t.Errorf("risk band = %q, want moderate", dr.RiskBand)
	}
	if dr.CompletionShare != 80 {
		t.Errorf("completion share = %v", dr.CompletionShare)
	}
	if _, err := (&Stats{}).DropRisk(0); err == nil {
		t.Error("expected an error for a stats record with no total")
	}
}

func TestConsistency(t *testing.T) {
	eps := []Episode{
		{Number: 1, PollAverage: 4.5},
		{Number: 2, PollAverage: 4.6},
		{Number: 3, PollAverage: 4.4},
		{Number: 4, PollAverage: 0}, // unrated: must not count as zero
		{Number: 5, PollAverage: 3.0},
		{Number: 6, PollAverage: 2.8},
		{Number: 7, PollAverage: 2.9},
	}
	c, err := Consistency(99, eps)
	if err != nil {
		t.Fatalf("Consistency: %v", err)
	}
	if c.RatedEpisodes != 6 {
		t.Errorf("rated episodes = %d, want 6", c.RatedEpisodes)
	}
	if c.Verdict != "falls off" {
		t.Errorf("verdict = %q, want falls off (slope %v)", c.Verdict, c.Slope)
	}
	if c.WorstEpisode != 6 {
		t.Errorf("worst episode = %d, want 6", c.WorstEpisode)
	}
	if c.MeanPoll < 3.5 || c.MeanPoll > 3.8 {
		t.Errorf("mean poll = %v", c.MeanPoll)
	}
	if _, err := Consistency(1, []Episode{{Number: 1}}); err == nil {
		t.Error("expected an error when no episode has a poll average")
	}
}

func TestParseCharactersSplitsTables(t *testing.T) {
	page := `<html><body><h2>Characters &amp; Staff</h2>
<table><tr><td><a href="/character/1/Spike_Spiegel">Spike Spiegel</a><div><span>Main</span></div><a href="/people/1/Tomokazu_Seki">Tomokazu Seki</a><a href="/people/2/Steve_Blum">Steve Blum</a></td></tr></table>
<h2>Staff</h2>
<table><tr><td><a href="/people/9/Shinichiro_Watanabe">Shinichiro Watanabe</a><div><span>Director</span></div></td></tr></table>
</body></html>`
	chars, staff, err := ParseCharacters(page)
	if err != nil {
		t.Fatalf("ParseCharacters: %v", err)
	}
	if len(chars) != 1 || chars[0].Character != "Spike Spiegel" || chars[0].Role != "Main" {
		t.Fatalf("chars = %+v", chars)
	}
	if len(chars[0].VAs) != 2 || chars[0].VAs[0].Language != "Japanese" || chars[0].VAs[1].Language != "English" {
		t.Fatalf("VAs = %+v", chars[0].VAs)
	}
	if len(staff) != 1 || staff[0].Person != "Shinichiro Watanabe" || staff[0].Role != "Director" {
		t.Fatalf("staff = %+v", staff)
	}
}

func TestParseSeasonAndRanking(t *testing.T) {
	season := `<div class="js-categories-seasonal"><div class="seasonal-anime-list js-seasonal-anime-list js-seasonal-anime-list-key-1"> <div class="anime-header">TV (New)</div>
	<div class="js-anime-category-producer seasonal-anime js-seasonal-anime js-anime-type-all js-anime-type-1" data-genre="8,7">
	<div class="title"><div class="title-text"><h2 class="h2_anime_title"><a href="https://myanimelist.net/anime/61987/Kusuriya_no_Hitorigoto_3rd_Season" class="link-title">Kusuriya no Hitorigoto 3rd Season</a></h2></div>
	<span style="display: none;" class="js-members">137688</span><span style="display: none;" class="js-score">0</span><span style="display: none;" class="js-start_date">20261002</span></div>
	<div class="info"><span class="item">Oct 2, 2026</span></div><div class="genres js-genre"><div class="genres-inner js-genre-inner"><span class="genre"><a href="/anime/genre/8/Drama">Drama</a></span></div></div></div>
	</div></div>`
	entries, err := ParseSeason(season)
	if err != nil {
		t.Fatalf("ParseSeason: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	e := entries[0]
	if e.ID != 61987 || e.Category != "TV (New)" || e.Members != 137688 || e.StartDate != "2026-10-02" {
		t.Fatalf("entry = %+v", e)
	}
	if len(e.Genres) != 1 || e.Genres[0] != "Drama" {
		t.Fatalf("genres = %v", e.Genres)
	}

	ranking := `<table class="top-ranking-table"><tr class="ranking-list"><td class="rank ac" valign="top"><span class="lightLink top-anime-rank-text rank1">1</span></td><td class="title"><a href="https://myanimelist.net/anime/52991/Sousou_no_Frieren">Sousou no Frieren</a><div class="information di-ib mt4">TV (28 eps)<br>Sep 2023 - Mar 2024<br><span class="text">2,084,414 members</span></div></td><td class="score"><span>9.29</span></td></tr></table>`
	titles, err := ParseRanking(ranking)
	if err != nil {
		t.Fatalf("ParseRanking: %v", err)
	}
	if len(titles) != 1 || titles[0].ID != 52991 || titles[0].Rank != 1 || titles[0].Score != 9.29 || titles[0].StartDate != "Sep 2023" {
		t.Fatalf("titles = %+v", titles)
	}
}
