package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func safariSecondsForTest(t time.Time) float64 {
	u := t.UTC()
	return float64(u.Unix()) + float64(u.Nanosecond())/float64(time.Second) - source.SafariEpochOffsetSeconds
}

func writeRoutingSnapshot(t *testing.T, path, rawURL, title string, origin int) {
	t.Helper()
	writeRoutingSnapshotRows(t, path, []routingVisit{{ID: 1, URL: rawURL, Title: title, Origin: origin, When: time.Now().UTC().Add(-time.Hour)}})
}

type routingVisit struct {
	ID             int
	URL            string
	Title          string
	Origin         int
	When           time.Time
	RedirectSource int
}

func writeRoutingSnapshotRows(t *testing.T, path string, visits []routingVisit) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer db.Close()
	stmts := []string{
		`CREATE TABLE history_items (id INTEGER PRIMARY KEY, url TEXT, domain_expansion TEXT, visit_count INTEGER)`,
		`CREATE TABLE history_visits (id INTEGER PRIMARY KEY, history_item INTEGER, visit_time REAL, title TEXT, origin INTEGER, redirect_source INTEGER)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	for _, v := range visits {
		visitTime := safariSecondsForTest(v.When)
		if _, err := db.Exec(`INSERT INTO history_items(id, url, domain_expansion, visit_count) VALUES(?, ?, ?, 1)`, v.ID, v.URL, source.DomainFromURL(v.URL)); err != nil {
			t.Fatalf("insert item: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO history_visits(id, history_item, visit_time, title, origin, redirect_source) VALUES(?, ?, ?, ?, ?, ?)`, v.ID, v.ID, visitTime, v.Title, v.Origin, v.RedirectSource); err != nil {
			t.Fatalf("insert visit: %v", err)
		}
	}
}

func runRootJSONBytes(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := NewRootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	runErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = orig
	data, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("execute %v: %v\n%s", args, runErr, data)
	}
	return data
}

func runRootJSONRows(t *testing.T, args ...string) []map[string]any {
	t.Helper()
	data := runRootJSONBytes(t, args...)
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("decode %q: %v", string(data), err)
	}
	return rows
}

func runRootJSONObject(t *testing.T, args ...string) map[string]any {
	t.Helper()
	data := runRootJSONBytes(t, args...)
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("decode object %q: %v", string(data), err)
	}
	return obj
}

func TestArchiveRoutingOutputCorrectness(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	snapshot, err := snapshotPath()
	if err != nil {
		t.Fatalf("snapshot path: %v", err)
	}
	archivePath, err := store.ArchivePath()
	if err != nil {
		t.Fatalf("archive path: %v", err)
	}

	now := time.Now().UTC()
	writeRoutingSnapshotRows(t, snapshot, []routingVisit{
		{ID: 1, URL: "https://archive.example/path", Title: "QuasarTitle Only", Origin: 1, When: now.Add(-2 * time.Hour)},
		{ID: 2, URL: "https://docs.example/guide", Title: "NebulaTitle Guide", Origin: 0, When: now.Add(-90 * time.Minute)},
	})
	if _, err := store.AccumulateFromSource(archivePath, snapshot, time.Now().UTC()); err != nil {
		t.Fatalf("accumulate archive: %v", err)
	}
	writeRoutingSnapshotRows(t, snapshot, []routingVisit{
		{ID: 1, URL: "https://referrer.example/start", Title: "Snapshot Referrer", Origin: 0, When: now.Add(-2 * time.Hour)},
		{ID: 2, URL: "https://snapshot.example/target", Title: "Snapshot Target", Origin: 0, When: now.Add(-time.Hour), RedirectSource: 1},
	})

	searchRows := runRootJSONRows(t, "--json", "search", "QuasarTitle")
	if len(searchRows) != 1 || searchRows[0]["url"] != "https://archive.example/path" {
		t.Fatalf("archive search rows = %#v, want title-only archive match", searchRows)
	}

	domainRows := runRootJSONRows(t, "--json", "domains", "--since", "7d")
	if len(domainRows) < 2 {
		t.Fatalf("domains rows = %#v, want archive domains", domainRows)
	}
	joinedDomains, _ := json.Marshal(domainRows)
	if !strings.Contains(string(joinedDomains), "archive.example") || !strings.Contains(string(joinedDomains), "docs.example") {
		t.Fatalf("domains did not report archive domains: %s", joinedDomains)
	}

	report := runRootJSONObject(t, "--json", "report", "--since", "7d")
	if perDay, ok := report["per_day"].([]any); !ok || len(perDay) == 0 {
		t.Fatalf("report per_day = %#v, want non-empty archive activity", report["per_day"])
	}
	if top, ok := report["top_domains"].([]any); !ok || len(top) == 0 {
		t.Fatalf("report top_domains = %#v, want non-empty archive domains", report["top_domains"])
	}

	timelineRows := runRootJSONRows(t, "--json", "timeline", "--since", "1d")
	if len(timelineRows) == 0 {
		t.Fatalf("timeline rows = %#v, want archive sessions", timelineRows)
	}
	timelineJSON, _ := json.Marshal(timelineRows)
	if !strings.Contains(string(timelineJSON), "archive.example") || strings.Contains(string(timelineJSON), "snapshot.example") {
		t.Fatalf("timeline archive routing mismatch: %s", timelineJSON)
	}

	listRows := runRootJSONRows(t, "--json", "--limit", "5", "list", "--since", "7d")
	if len(listRows) != 2 {
		t.Fatalf("list rows = %d, want 2 archive rows: %#v", len(listRows), listRows)
	}
	var foundArchive bool
	for _, row := range listRows {
		if row["url"] == "https://archive.example/path" {
			foundArchive = true
			if row["origin"] != "synced" {
				t.Fatalf("archive list origin = %v, want synced", row["origin"])
			}
		}
	}
	if !foundArchive {
		t.Fatalf("list did not include archive row: %#v", listRows)
	}

	visitedRows := runRootJSONRows(t, "--json", "visited", "snapshot.example")
	if len(visitedRows) != 1 || visitedRows[0]["found"] != true {
		t.Fatalf("visited rows = %#v, want snapshot target found", visitedRows)
	}
	referrers := fmt.Sprint(visitedRows[0]["referrer_examples"])
	if !strings.Contains(referrers, "https://referrer.example/start") {
		t.Fatalf("visited referrers = %v, want snapshot referrer chain", visitedRows[0]["referrer_examples"])
	}

	deviceRows := runRootJSONRows(t, "--json", "devices")
	if len(deviceRows) == 0 {
		t.Fatal("devices returned no rows")
	}
	joined, _ := json.Marshal(deviceRows)
	if !strings.Contains(string(joined), "snapshot.example") {
		t.Fatalf("devices did not read snapshot row: %s", joined)
	}
	if strings.Contains(string(joined), "archive.example") {
		t.Fatalf("devices unexpectedly read archive row: %s", joined)
	}
}
