package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
)

func TestNovelDraftRoiCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"draft-roi"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("computes draft ROI from seeded SQLite store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_ = db.Upsert("leagues", "449.l.365083.draft.1", json.RawMessage(`{"pick":101,"round":10,"cost":5,"team_key":"449.l.365083.t.1","player_key":"449.p.31862","player_name":"Puka Nacua","position":"WR"}`))
		_ = db.Upsert("players", "449.p.31862", json.RawMessage(`{"player_key":"449.p.31862","full_name":"Puka Nacua","display_position":"WR","points_total":215.4,"owner_team_key":"449.l.365083.t.1"}`))
		_ = db.Close()

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"draft-roi", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Puka Nacua") || !strings.Contains(stdout.String(), "surplus_points") {
			t.Fatalf("expected Puka Nacua draft ROI in JSON, got: %s", stdout.String())
		}
	})
}
