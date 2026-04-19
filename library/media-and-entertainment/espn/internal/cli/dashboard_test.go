package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDashboardFavoritesMissingFile(t *testing.T) {
	dir := t.TempDir()
	favs, _, err := loadDashboardFavorites(filepath.Join(dir, "absent.toml"))
	if err != nil {
		t.Fatalf("missing file should yield nil error, got %v", err)
	}
	if favs != nil {
		t.Errorf("missing file should yield nil favorites, got %+v", favs)
	}
}

func TestLoadDashboardFavoritesParsesAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `
[favorites]
NFL = ["KC", "bal"]
nba = ["LAL"]
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	favs, _, err := loadDashboardFavorites(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("want 2 leagues, got %d", len(favs))
	}
	nfl := favs["nfl"]
	if len(nfl) != 2 || nfl[0] != "KC" || nfl[1] != "BAL" {
		t.Errorf("nfl normalization wrong: %+v", nfl)
	}
}

func TestLoadDashboardFavoritesMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("not = valid = toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadDashboardFavorites(path)
	if err == nil {
		t.Fatal("expected error for malformed TOML")
	}
}

func TestDashboardRowsForFiltersToFavorites(t *testing.T) {
	scoreboard := json.RawMessage(`{
		"events":[
			{"id":"1","shortName":"BOS @ LAL","status":{"type":{"state":"in","detail":"Q3 4:12"}},
			 "competitions":[{"competitors":[
				{"homeAway":"home","score":"100","team":{"abbreviation":"LAL"}},
				{"homeAway":"away","score":"95","team":{"abbreviation":"BOS"}}
			 ]}]},
			{"id":"2","shortName":"PHI @ DAL","status":{"type":{"state":"in","detail":"Q1 9:30"}},
			 "competitions":[{"competitors":[
				{"homeAway":"home","score":"10","team":{"abbreviation":"DAL"}},
				{"homeAway":"away","score":"5","team":{"abbreviation":"PHI"}}
			 ]}]}
		]
	}`)
	rows := dashboardRowsFor(scoreboard, []string{"LAL"})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row matching LAL, got %d", len(rows))
	}
	if rows[0].Team != "LAL" {
		t.Errorf("want team LAL, got %q", rows[0].Team)
	}
}

func TestDashboardRowsForEmpty(t *testing.T) {
	rows := dashboardRowsFor(json.RawMessage(`{"events":[]}`), []string{"KC"})
	if len(rows) != 0 {
		t.Errorf("empty scoreboard should yield no rows, got %d", len(rows))
	}
}
