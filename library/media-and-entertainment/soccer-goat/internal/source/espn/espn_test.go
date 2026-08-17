package espn

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return body
}

func TestParseLookup_RealFixture(t *testing.T) {
	ctx, ok, err := parseLookup(readFixture(t, "search_v2_joao_neves.json"), "joao neves")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected to resolve João Neves, got no match")
	}
	if ctx.DisplayName != "João Neves" {
		t.Errorf("DisplayName = %q, want João Neves", ctx.DisplayName)
	}
	if ctx.AthleteID != "355061" {
		t.Errorf("AthleteID = %q, want 355061", ctx.AthleteID)
	}
	if ctx.Team != "Paris Saint-Germain" {
		t.Errorf("Team = %q, want Paris Saint-Germain", ctx.Team)
	}
	if got := ctx.Link; got == "" || got[:8] != "https://" {
		t.Errorf("Link = %q, want an https URL", got)
	}
}

func TestAthleteID(t *testing.T) {
	cases := []struct {
		uid, link, want string
	}{
		{"s:600~a:355061", "", "355061"},
		{"", "https://www.espn.com/soccer/player/_/id/355061/joao-neves", "355061"},
		{"", "https://www.espn.com/soccer/player/_/id/355061", "355061"},
		{"", "", ""},
	}
	for _, c := range cases {
		if got := athleteID(c.uid, c.link); got != c.want {
			t.Errorf("athleteID(%q,%q) = %q, want %q", c.uid, c.link, got, c.want)
		}
	}
}

func TestParseLookup_Guard(t *testing.T) {
	// First a non-soccer athlete (basketball), then a name mismatch, then the
	// correct soccer row. The guard must resolve the correct one, not the first.
	body := []byte(`{"results":[{"type":"player","contents":[
		{"displayName":"John Smith","sport":"basketball","uid":"s:40~a:999","link":{"web":"/nba/player/_/id/999/john-smith"}},
		{"displayName":"Other Person","sport":"soccer","defaultLeagueSlug":"eng.1","uid":"s:600~a:111","link":{"web":"/soccer/player/_/id/111/other"}},
		{"displayName":"Andreas Schjelderup","sport":"soccer","defaultLeagueSlug":"por.1","subtitle":"SL Benfica","uid":"s:600~a:260952","link":{"web":"/soccer/player/_/id/260952/andreas-schjelderup"}}
	]}]}`)
	ctx, ok, err := parseLookup(body, "schjelderup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || ctx.AthleteID != "260952" {
		t.Fatalf("guard resolved %+v (ok=%v), want AthleteID 260952", ctx, ok)
	}
}

func TestParseLookup_NoWrongAthlete(t *testing.T) {
	// Only a name-mismatched soccer row exists — do not resolve the wrong player.
	body := []byte(`{"results":[{"type":"player","contents":[
		{"displayName":"Lionel Messi","sport":"soccer","defaultLeagueSlug":"usa.1","uid":"s:600~a:45843","link":{"web":"/soccer/player/_/id/45843/lionel-messi"}}
	]}]}`)
	if _, ok, _ := parseLookup(body, "cristiano ronaldo"); ok {
		t.Error("resolved a mismatched athlete; guard should have rejected it")
	}
}

func TestParseLookup_NoPlayers(t *testing.T) {
	body := []byte(`{"results":[{"type":"article","contents":[{"displayName":"Some article"}]}]}`)
	if _, ok, _ := parseLookup(body, "joao neves"); ok {
		t.Error("resolved a player from an article-only result set")
	}
	if _, ok, _ := parseLookup([]byte(`{"results":[]}`), "joao neves"); ok {
		t.Error("resolved a player from empty results")
	}
}

func TestParseLookup_Diacritics(t *testing.T) {
	body := []byte(`{"results":[{"type":"player","contents":[
		{"displayName":"Kylian Mbappé","sport":"soccer","defaultLeagueSlug":"esp.1","uid":"s:600~a:231533","link":{"web":"/soccer/player/_/id/231533/kylian-mbappe"}}
	]}]}`)
	if _, ok, _ := parseLookup(body, "mbappe"); !ok {
		t.Error("diacritic-insensitive match failed for mbappe -> Mbappé")
	}
}

func TestParseOverview_RealFixture(t *testing.T) {
	enr, err := parseOverview(readFixture(t, "overview_joao_neves.json"), "355061")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enr == nil || enr.Stats == nil {
		t.Fatal("expected non-nil enrichment with stats")
	}
	// Season totals = sum across the two competition splits.
	if enr.Stats.Starts != 5 || enr.Stats.Goals != 1 || enr.Stats.Shots != 3 || enr.Stats.FoulsSuffered != 9 {
		t.Errorf("aggregate stats = %+v, want Starts5 Goals1 Shots3 FoulsSuffered9", *enr.Stats)
	}
	// R3: per-competition splits, both for Portugal (teamId 482).
	if len(enr.Splits) != 2 {
		t.Fatalf("splits = %d, want 2", len(enr.Splits))
	}
	for _, s := range enr.Splits {
		if s.TeamID != "482" {
			t.Errorf("split %q teamId = %q, want 482", s.DisplayName, s.TeamID)
		}
	}
	// R4: recent games, most-recent first.
	if len(enr.RecentGames) == 0 {
		t.Fatal("expected recent games")
	}
	first := enr.RecentGames[0]
	if first.Opponent != "Spain" || first.Result != "L" || first.Date == "" {
		t.Errorf("most-recent game = %+v, want opponent Spain result L with a date", first)
	}
	for i := 1; i < len(enr.RecentGames); i++ {
		if enr.RecentGames[i-1].Date < enr.RecentGames[i].Date {
			t.Errorf("recent games not sorted most-recent-first at %d", i)
		}
	}
}

func TestStatsFromNames_ColumnReorder(t *testing.T) {
	// KTD4: stats map by name, not column position. Permute the columns and the
	// goals value must still land in Goals.
	names := []string{"goalAssists", "totalGoals", "starts"}
	values := []string{"7", "3", "10"}
	s := statsFromNames(names, values)
	if s.Goals != 3 || s.Assists != 7 || s.Starts != 10 {
		t.Errorf("statsFromNames reorder = %+v, want Goals3 Assists7 Starts10", s)
	}
}

func TestParseOverview_TotalRowNotDoubleCounted(t *testing.T) {
	// A pre-summed "Total" row alongside real competition rows must not inflate
	// the aggregate, and must not appear as a competition split.
	body := []byte(`{"statistics":{"names":["totalGoals","goalAssists"],"splits":[
		{"displayName":"Premier League","teamId":"359","leagueId":"eng.1","stats":["5","2"]},
		{"displayName":"Champions League","teamId":"359","leagueId":"uefa.champions","stats":["3","1"]},
		{"displayName":"Total","teamId":"","leagueId":"","stats":["8","3"]}
	]}}`)
	enr, err := parseOverview(body, "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enr.Splits) != 2 {
		t.Errorf("splits = %d, want 2 (total row excluded)", len(enr.Splits))
	}
	if enr.Stats.Goals != 8 || enr.Stats.Assists != 3 {
		t.Errorf("aggregate = %+v, want Goals8 Assists3 (not double-counted)", *enr.Stats)
	}
}

func TestParseOverview_Empty(t *testing.T) {
	enr, err := parseOverview([]byte(`{}`), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enr != nil {
		t.Errorf("empty overview should yield nil enrichment, got %+v", enr)
	}
}
