package cli

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggstore"
)

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	root.SetArgs(args)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	return stdout.String(), err
}

// Digg removed /api/trending/status, so the commands built on it are gone.
func TestPipelineStatusAndTrendingAreRemoved(t *testing.T) {
	for _, args := range [][]string{{"pipeline", "status"}, {"trending"}} {
		if _, err := runRoot(t, args...); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("%v: err = %v, want unknown command", args, err)
		}
	}
}

func TestEventsListsStoredEvents(t *testing.T) {
	withTempHome(t)
	db, err := sql.Open("sqlite", defaultDBPath("digg-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if err := diggstore.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if err := diggstore.UpsertEvent(db, diggparse.Event{ID: "e1", Type: "fast_climb", ClusterID: "c1", Label: "old climb", At: at, DedupeKey: "k1"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, err := runRoot(t, "events", "--since", "1d", "--json")
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	if !strings.Contains(out, "old climb") {
		t.Fatalf("events output %q should include the stored event", out)
	}
}
