package traveloka

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
)

func temporaryStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "snapshots.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
func syntheticSnapshot(id, retrieved string) *Snapshot {
	return &Snapshot{ID: id, Kind: "hotel", Status: "OK", RetrievedAt: retrieved, Query: futureHotelQuery(), SearchComplete: true, Offers: []Offer{{ID: "source-rate", PropertyID: "9000000001714", PropertyName: "The Berkeley Hotel Pratunam", RoomName: "Synthetic Premier Room", Price: Price{Total: &Money{Currency: "SGD", MinorUnits: "47181", Amount: "471.81", Decimals: intPointer(2)}}, Details: map[string]any{"userContext": "synthetic-user-secret", "nested": map[string]any{"inventoryRateKey": "synthetic-rate-secret", "known": "beds"}}, Cancellation: map[string]any{"refundable": true, "marketingContextCapsule": "synthetic-marketing-secret"}}}, Coverage: map[string]any{"source_count": json.Number("1"), "sentinel": "synthetic-secret"}}
}
func TestSimulatedSQLiteSnapshotEmptyMissingHistoryAndSearch(t *testing.T) {
	ctx := context.Background()
	db := temporaryStore(t)
	if _, err := LoadSnapshot(ctx, db, "", "hotel"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cold store must be missing, got %v", err)
	}
	first := syntheticSnapshot("first", "2026-10-02T00:00:00Z")
	if err := SaveSnapshot(ctx, db, first); err != nil {
		t.Fatal(err)
	}
	second := syntheticSnapshot("second", "2026-10-02T01:00:00Z")
	second.Offers = nil
	second.Status = "NO_INVENTORY"
	if err := SaveSnapshot(ctx, db, second); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadSnapshot(ctx, db, "first", "hotel")
	if err != nil {
		t.Fatal(err)
	}
	if saved.RetrievedAt != first.RetrievedAt || saved.Freshness != "saved_snapshot" || !saved.Indicative || len(saved.Offers) != 1 {
		t.Fatal("history metadata lost")
	}
	latest, err := LoadSnapshot(ctx, db, "", "hotel")
	if err != nil || latest.ID != "second" || latest.Offers == nil || len(latest.Offers) != 0 || latest.Status != "NO_INVENTORY" {
		t.Fatalf("latest empty snapshot incorrect: %v", err)
	}
	if _, err = LoadSnapshot(ctx, db, "missing", "hotel"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("missing snapshot must be explicit")
	}
	if _, err = LoadSnapshot(ctx, db, "first", "flight"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("kind mismatch must be missing")
	}
	results, err := db.Search("Berkeley", 10, "traveloka_offers")
	if err != nil || len(results) != 1 {
		t.Fatalf("normalized searchable offer missing: count=%d err=%v", len(results), err)
	}
	var count int
	if err = db.DB().QueryRow(`SELECT count(*) FROM traveloka_snapshots`).Scan(&count); err != nil || count != 2 {
		t.Fatal("snapshot history must retain both retrievals")
	}
}
func TestSimulatedSQLiteSnapshotSanitizedPersistence(t *testing.T) {
	db := temporaryStore(t)
	snapshot := syntheticSnapshot("sanitized", "2026-10-02T00:00:00Z")
	if err := SaveSnapshot(context.Background(), db, snapshot); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.DB().QueryRow(`SELECT snapshot FROM traveloka_snapshots WHERE id='sanitized'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "synthetic-") || strings.Contains(raw, "inventoryRateKey") || !strings.Contains(raw, "47181") {
		t.Fatalf("public SQL snapshot leaked or lost exact price")
	}
	// Drain every row before any later store query; real SQLite runs with its generated connection constraints.
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type IN ('traveloka_snapshots','traveloka_offers')`)
	if err != nil {
		t.Fatal(err)
	}
	records := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		records = append(records, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatal("normalized resources not stored")
	}
	for _, record := range records {
		if strings.Contains(record, "synthetic-") {
			t.Fatal("generated resource leaks secrets")
		}
	}
	var fts string
	if err = db.DB().QueryRow(`SELECT group_concat(content) FROM resources_fts WHERE resource_type IN ('traveloka_snapshots','traveloka_offers')`).Scan(&fts); err != nil || strings.Contains(fts, "synthetic-") || !strings.Contains(fts, "Berkeley") {
		t.Fatal("FTS sanitization/search text incorrect")
	}
	if snapshot.Offers[0].Details["userContext"] != "synthetic-user-secret" {
		t.Fatal("save mutated caller's source-backed snapshot")
	}
}
func TestSimulatedSnapshotFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "public-quote.json")
	snapshot := syntheticSnapshot("file", "2026-10-02T00:00:00Z")
	if err := SaveSnapshotFile(path, snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadSnapshotFile(path)
	if err != nil || loaded.RetrievedAt != snapshot.RetrievedAt || loaded.Freshness != "saved_snapshot" {
		t.Fatal("file snapshot lost original retrieval context")
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "synthetic-") {
		t.Fatal("public file leaks credentials")
	}
	if _, err := ReadSnapshotFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
	if err := os.WriteFile(path, []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadSnapshotFile(path); err == nil {
		t.Fatal("malformed file accepted")
	}
	if err := SaveSnapshotFile("", snapshot); err == nil {
		t.Fatal("empty file path accepted")
	}
}
func TestSimulatedSnapshotRejectsInvalidAndPropagatesSQLiteErrors(t *testing.T) {
	db := temporaryStore(t)
	ctx := context.Background()
	for _, snapshot := range []*Snapshot{nil, {ID: "", Kind: "hotel", RetrievedAt: "2026-10-02T00:00:00Z"}, {ID: "id", Kind: "hotel", RetrievedAt: "invalid"}} {
		if err := SaveSnapshot(ctx, db, snapshot); err == nil {
			t.Fatal("invalid snapshot accepted")
		}
	}
	snapshot := syntheticSnapshot("failure", "2026-10-02T00:00:00Z")
	if err := SaveSnapshot(ctx, nil, snapshot); err == nil {
		t.Fatal("nil store accepted")
	}
	if _, err := LoadSnapshot(ctx, nil, "", "hotel"); err == nil {
		t.Fatal("nil load store accepted")
	}
	// FTS failure must abort both the snapshot and normalized-resource transaction.
	if _, err := db.DB().Exec(`DROP TABLE resources_fts`); err != nil {
		t.Fatal(err)
	}
	if err := SaveSnapshot(ctx, db, snapshot); err == nil {
		t.Fatal("FTS failure was swallowed")
	}
	var count int
	if err := db.DB().QueryRow(`SELECT count(*) FROM resources`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed transaction partially persisted resources")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := SaveSnapshot(canceled, db, snapshot); err == nil {
		t.Fatal("canceled transaction accepted")
	}
}
