package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
)

func TestNovelMatchupRadarCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"matchup-radar"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("renders matchup radar from seeded SQLite store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_ = db.Upsert("leagues", "449.l.365083.sb.9", json.RawMessage(`{"week":9,"status":"midevent","team_key":"449.l.365083.t.1","team_name":"Alpha","team_points":95.2,"team_projected_points":122.0,"opponent_key":"449.l.365083.t.2","opponent_name":"Bravo","opponent_points":81.0,"opponent_projected_points":110.5}`))
		_ = db.Upsert("rosters", "449.l.365083.t.1:roster", json.RawMessage(`{"league_key":"449.l.365083","players":[{"player_key":"449.p.31862","full_name":"Josh Allen","selected_position":"QB","projected_points":24.5,"points_total":21.2,"owner_team_key":"449.l.365083.t.1"}]}`))
		_ = db.Upsert("rosters", "449.l.365083.t.2:roster", json.RawMessage(`{"league_key":"449.l.365083","players":[{"player_key":"449.p.30123","full_name":"Lamar Jackson","selected_position":"QB","projected_points":22.0,"points_total":18.4,"owner_team_key":"449.l.365083.t.2"}]}`))
		_ = db.Close()

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"matchup-radar", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--week", "9", "--db", dbPath, "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Alpha") || !strings.Contains(stdout.String(), "Bravo") || !strings.Contains(stdout.String(), "Josh Allen") {
			t.Fatalf("expected Alpha vs Bravo with starter slot comparisons in matchup radar JSON, got: %s", stdout.String())
		}
	})
}
