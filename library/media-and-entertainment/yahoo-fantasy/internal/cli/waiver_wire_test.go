package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
)

func TestNovelWaiverWireCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"waiver-wire"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("ranks waiver upgrades from seeded SQLite store and isolates leagues", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_ = db.Upsert("players", "449.l.365083:449.p.40001", json.RawMessage(`{"league_key":"449.l.365083","player_key":"449.p.40001","full_name":"Breakout RB","display_position":"RB","ownership_status":"FA","points_per_game":16.5,"percent_owned":40}`))
		_ = db.Upsert("players", "449.l.365083:449.p.30001", json.RawMessage(`{"league_key":"449.l.365083","player_key":"449.p.30001","full_name":"Bench RB","display_position":"RB","selected_position":"BN","owner_team_key":"449.l.365083.t.1","points_per_game":6.5}`))
		// Another league's player must not leak into 449.l.365083 waiver rankings
		_ = db.Upsert("players", "449.l.999999:449.p.99999", json.RawMessage(`{"league_key":"449.l.999999","player_key":"449.p.99999","full_name":"Other League RB","display_position":"RB","ownership_status":"FA","points_per_game":30.0,"percent_owned":99}`))
		_ = db.Close()

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"waiver-wire", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--pos", "RB", "--db", dbPath, "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Breakout RB") || !strings.Contains(stdout.String(), "449.p.40001") {
			t.Fatalf("expected Breakout RB in output, got: %s", stdout.String())
		}
		if strings.Contains(stdout.String(), "Other League RB") {
			t.Fatalf("expected other league player to be excluded, got: %s", stdout.String())
		}
	})
}
