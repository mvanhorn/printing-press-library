package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
)

func TestNovelPowerRankingsCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"power-rankings"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("computes power rankings from seeded SQLite store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_ = db.Upsert("teams", "449.l.365083.t.1", json.RawMessage(`{"team_key":"449.l.365083.t.1","name":"Gridiron Kings","rank":2,"wins":6,"losses":3,"points_for":1180.5}`))
		_ = db.Upsert("teams", "449.l.365083.t.2", json.RawMessage(`{"team_key":"449.l.365083.t.2","name":"Lucky Dogs","rank":1,"wins":7,"losses":2,"points_for":1020.0}`))
		_ = db.Close()

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"power-rankings", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Gridiron Kings") || !strings.Contains(stdout.String(), "all_play_wins") {
			t.Fatalf("expected Gridiron Kings power rankings in JSON, got: %s", stdout.String())
		}
	})
}
