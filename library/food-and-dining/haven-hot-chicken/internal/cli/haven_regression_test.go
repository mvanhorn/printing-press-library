package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/haven-hot-chicken/internal/haven"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHavenLimitPreservesIntegerPrecision(t *testing.T) {
	const amount int64 = 9007199254740993
	v, err := limitHavenResult(map[string]any{"subtotal_cents": amount, "rows": []int{1, 2}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if !bytes.Contains(b, []byte("9007199254740993")) {
		t.Fatalf("integer rounded: %s", b)
	}
}

func TestHavenReadsDoNotMigrateUnrelatedDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE unrelated(value TEXT); INSERT INTO unrelated VALUES('keep')`); err != nil {
		t.Fatal(err)
	}
	cmd := newHavenCommand(&rootFlags{asJSON: true}, "menu")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--location", "14208", "--db", path})
	if err = cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var tables int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table'`).Scan(&tables); err != nil || tables != 1 {
		t.Fatalf("read migrated database: %d %v", tables, err)
	}
}

func TestHavenMaxAgeAndDisabledWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := haven.Snapshot{LocationID: 14208, FetchedAt: time.Now().Add(-20 * time.Minute), Complete: true, Items: []haven.Item{}}
	if err = haven.Save(context.Background(), db, nil, []haven.Snapshot{snapshot}); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, age := range []time.Duration{0, 5 * time.Minute, 30 * time.Minute} {
		cmd := newHavenCommand(&rootFlags{asJSON: true, maxAge: age}, "menu")
		var out, stderr bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--location", "14208", "--db", path})
		if err = cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		want := age == 5*time.Minute
		if strings.Contains(stderr.String(), "exceeds --max-age") != want {
			t.Fatalf("age %s warning: %s", age, stderr.String())
		}
	}
}
