package namethatui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

func TestSyncStoresMirror(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			entries := `[{"slug":"button","platform":"react","name":"Button","api":[],"parts":[]}]`
			w.Write([]byte("<script>" + push(t, `{"entries":`+entries+`}`) + "</script>"))
		case "/styles":
			w.Write([]byte(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"name":"A","url":"/styles/a"},{"name":"B","url":"/styles/b"}]}</script>`))
		default:
			w.Write([]byte("<h1>Style</h1><h2>Accessibility</h2><p>Readable.</p><script>" + push(t, `{"signals":[]}`) + "</script>"))
		}
	}))
	defer server.Close()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	report, err := Sync(context.Background(), server.Client(), server.URL, db, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Components != 1 || report.Styles != 2 {
		t.Fatalf("report %#v", report)
	}
	cs, _ := db.List("components", 0)
	ss, _ := db.List("style_details", 0)
	if len(cs) != 1 || len(ss) != 2 {
		t.Fatalf("stored %d/%d", len(cs), len(ss))
	}
	var c Component
	json.Unmarshal(cs[0], &c)
	if c.SourceURL != server.URL+"/react/button" {
		t.Fatalf("source %q", c.SourceURL)
	}
}

func TestSyncFailedStyleDoesNotAdvanceState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/styles" {
			w.Write([]byte(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"name":"Bad","url":"/styles/bad"}]}</script>`))
			return
		}
		http.Error(w, "broken", http.StatusInternalServerError)
	}))
	defer server.Close()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Sync(context.Background(), server.Client(), server.URL, db, false, true); err == nil {
		t.Fatal("expected error")
	}
	_, at, count, err := db.GetSyncState("style_details")
	if err != nil || !at.IsZero() || count != 0 {
		t.Fatalf("state %v %d %v", at, count, err)
	}
	_ = time.Second
}

func TestSyncLaterStyleFailureLeavesExistingMirrorSnapshotsAndStateUntouched(t *testing.T) {
	failSecondStyle := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/styles":
			if failSecondStyle {
				_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"name":"Existing","url":"/styles/existing"},{"name":"Later","url":"/styles/later"}]}</script>`))
				return
			}
			_, _ = w.Write([]byte(`<script type="application/ld+json">{"@type":"ItemList","itemListElement":[{"name":"Existing","url":"/styles/existing"}]}</script>`))
		case "/styles/existing":
			_, _ = w.Write([]byte("<h1>Existing</h1><h2>Accessibility</h2><p>Original guidance.</p><script>" + push(t, `{"signals":[]}`) + "</script>"))
		case "/styles/later":
			http.Error(w, "broken", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := Sync(context.Background(), server.Client(), server.URL, db, false, true); err != nil {
		t.Fatalf("initial style sync: %v", err)
	}
	beforeMirror, err := db.List("style_details", 0)
	if err != nil {
		t.Fatal(err)
	}
	beforeSnapshots, err := db.List("style_snapshots", 0)
	if err != nil {
		t.Fatal(err)
	}
	beforeCursor, beforeAt, beforeCount, err := db.GetSyncState("style_details")
	if err != nil {
		t.Fatal(err)
	}

	failSecondStyle = true
	if _, err := Sync(context.Background(), server.Client(), server.URL, db, false, true); err == nil {
		t.Fatal("expected second style fetch to fail")
	}
	afterMirror, err := db.List("style_details", 0)
	if err != nil {
		t.Fatal(err)
	}
	afterSnapshots, err := db.List("style_snapshots", 0)
	if err != nil {
		t.Fatal(err)
	}
	afterCursor, afterAt, afterCount, err := db.GetSyncState("style_details")
	if err != nil {
		t.Fatal(err)
	}
	if stringSlice(beforeMirror) != stringSlice(afterMirror) || stringSlice(beforeSnapshots) != stringSlice(afterSnapshots) {
		t.Fatalf("failed sync mutated mirror/snapshots: before=%q/%q after=%q/%q", beforeMirror, beforeSnapshots, afterMirror, afterSnapshots)
	}
	if beforeCursor != afterCursor || !beforeAt.Equal(afterAt) || beforeCount != afterCount {
		t.Fatalf("failed sync mutated state: before=%q/%s/%d after=%q/%s/%d", beforeCursor, beforeAt, beforeCount, afterCursor, afterAt, afterCount)
	}
}

func stringSlice(rows []json.RawMessage) string {
	parts := make([]string, len(rows))
	for i, row := range rows {
		parts[i] = string(row)
	}
	return strings.Join(parts, "\n")
}

func TestSyncSnapshotsDeduplicateUnchangedContentAndKeepChanges(t *testing.T) {
	name := "Button"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		entries := `[{"slug":"button","platform":"react","name":"` + name + `","api":[],"parts":[]}]`
		_, _ = w.Write([]byte("<script>" + push(t, `{"entries":`+entries+`}`) + "</script>"))
	}))
	defer server.Close()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if _, err := Sync(context.Background(), server.Client(), server.URL, db, true, false); err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := db.List("component_snapshots", 0)
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("unchanged snapshots = %d, %v", len(snapshots), err)
	}
	name = "Primary Button"
	if _, err := Sync(context.Background(), server.Client(), server.URL, db, true, false); err != nil {
		t.Fatal(err)
	}
	snapshots, err = db.List("component_snapshots", 0)
	if err != nil || len(snapshots) != 2 {
		t.Fatalf("changed snapshots = %d, %v", len(snapshots), err)
	}
	var snapshot struct {
		EntityID    string          `json:"entity_id"`
		SnapshotAt  string          `json:"snapshot_at"`
		ContentHash string          `json:"content_hash"`
		Data        json.RawMessage `json:"data"`
		SourceURL   string          `json:"source_url"`
	}
	if err := json.Unmarshal(snapshots[0], &snapshot); err != nil || snapshot.EntityID == "" || snapshot.SnapshotAt == "" || snapshot.ContentHash == "" || len(snapshot.Data) == 0 || snapshot.SourceURL == "" {
		t.Fatalf("snapshot = %#v, err=%v", snapshot, err)
	}
}
