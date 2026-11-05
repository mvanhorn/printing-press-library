package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
)

func TestNovelFaabAuditCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"faab-audit"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("audits FAAB budgets from seeded SQLite store", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		_ = db.Upsert("teams", "449.l.365083.t.1", json.RawMessage(`{"team_key":"449.l.365083.t.1","name":"Alpha","faab_balance":70}`))
		_ = db.Upsert("teams", "449.l.365083.t.2", json.RawMessage(`{"team_key":"449.l.365083.t.2","name":"Bravo","faab_balance":45}`))
		_ = db.Upsert("transactions", "449.l.365083.tr.1", json.RawMessage(`{"transaction_key":"449.l.365083.tr.1","type":"add/drop","status":"successful","team_key":"449.l.365083.t.1","faab_bid":30,"add_player_key":"449.p.31862","add_player_name":"Puka Nacua","add_player_position":"WR"}`))
		// Other league's transaction must not leak
		_ = db.Upsert("transactions", "449.l.999999.tr.1", json.RawMessage(`{"transaction_key":"449.l.999999.tr.1","type":"add/drop","status":"successful","team_key":"449.l.999999.t.1","faab_bid":95,"add_player_key":"449.p.99999","add_player_name":"Foreign Player","add_player_position":"RB"}`))
		_ = db.Close()

		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"faab-audit", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "blocking_bid_over_next_rival") || !strings.Contains(stdout.String(), "Puka Nacua") {
			t.Fatalf("expected FAAB audit JSON output, got: %s", stdout.String())
		}
		if strings.Contains(stdout.String(), "Foreign Player") || strings.Contains(stdout.String(), "449.l.999999") {
			t.Fatalf("expected foreign league transaction to be excluded, got: %s", stdout.String())
		}
	})
}
