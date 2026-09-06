package cli

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestOpenNovelStoreRequiresAvailabilityView(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "no-view.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE placeholder (id TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := openNovelStoreAt(t.Context(), path)
	if err != nil || db != nil {
		t.Fatalf("db=%v err=%v, want nil,nil", db, err)
	}
	out, _, err := executeRoot("new-since", "--db", path, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Meta    map[string]any `json:"meta"`
		Results []any          `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	if env.Meta["synced"] != false || env.Meta["last_synced_at"] != nil || len(env.Results) != 0 {
		t.Fatalf("envelope=%+v", env)
	}
}

func TestNovelLocalMetaSynced(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synced.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	want := time.Date(2026, 9, 6, 12, 34, 56, 0, time.UTC)
	if _, err := db.DB().Exec(`INSERT INTO sync_state(resource_type,last_synced_at,total_count) VALUES(?,?,1)`, "availability", want); err != nil {
		t.Fatal(err)
	}
	meta := novelLocalMeta(db)
	if meta["synced"] != true || meta["last_synced_at"] != want.Format(time.RFC3339) {
		t.Fatalf("meta=%v", meta)
	}
}

func TestNovelUsageErrorEnvelopes(t *testing.T) {
	isolateNovelTest(t)
	for _, args := range [][]string{
		{"new-since", "--cabin", "bogus", "--agent"},
		{"calendar", "--agent"},
		{"recheck", "--apply", "--data-source", "local", "--agent"},
	} {
		out, _, err := executeRoot(args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("args=%v err=%v code=%d", args, err, ExitCode(err))
		}
		var got map[string]any
		if decodeErr := json.Unmarshal(out.Bytes(), &got); decodeErr != nil {
			t.Fatalf("args=%v stdout=%q: %v", args, out.String(), decodeErr)
		}
		results, _ := got["results"].(map[string]any)
		if got["meta"] == nil || results["error"] == nil || results["usage"] == nil {
			t.Fatalf("args=%v envelope=%v", args, got)
		}
	}
}

func TestNovelReadOnlyStoreRequiresExtrasUpgrade(t *testing.T) {
	isolateNovelTest(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertAvailability(json.RawMessage(`{"ID":"row","Date":"2099-01-01","Source":"united","JAvailable":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`DROP TABLE store_extras_meta`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := executeRoot("new-since", "--db", path, "--since", "1h", "--json", "--no-learn")
	if err != nil || strings.TrimSpace(out.String()) != "[]" || !strings.Contains(stderr.String(), "finish the store upgrade") {
		t.Fatalf("out=%q stderr=%q err=%v", out, stderr, err)
	}
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	out, _, err = executeRoot("new-since", "--db", path, "--since", "1h", "--json", "--no-learn")
	var rows []newSinceRow
	decodeErr := json.Unmarshal(out.Bytes(), &rows)
	if err != nil || decodeErr != nil || len(rows) != 1 || rows[0].ID != "row" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
