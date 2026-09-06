package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func seedLegacyResource(t *testing.T, db *store.Store, resourceType, id string, values map[string]any) {
	t.Helper()
	raw := rawObject(t, values)
	if _, err := db.DB().Exec(`INSERT INTO resources (id, resource_type, data) VALUES (?, ?, ?)`, id, resourceType, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func openLocalListTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func rawObject(t *testing.T, values map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestLocalListRowsTypedPushdownSourceAndTake(t *testing.T) {
	db := openLocalListTestStore(t)
	for _, row := range []map[string]any{{"ID": "a", "Source": "aeroplan"}, {"ID": "b", "Source": "united"}, {"ID": "c", "Source": "aeroplan"}} {
		if err := db.UpsertAvailability(rawObject(t, row)); err != nil {
			t.Fatal(err)
		}
	}
	rows, unsupported, err := localListRows(context.Background(), db, "availability", map[string]string{"source": "aeroplan", "take": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(unsupported) != 0 || len(rows) != 1 {
		t.Fatalf("rows=%d unsupported=%v", len(rows), unsupported)
	}
}

func TestLocalListRowsTypedPushdownReturnsOnlyRequestedSource(t *testing.T) {
	db := openLocalListTestStore(t)
	for _, row := range []map[string]any{{"ID": "u1", "Source": "united"}, {"ID": "a1", "Source": "aeroplan"}, {"ID": "u2", "Source": "united"}} {
		if err := db.UpsertAvailability(rawObject(t, row)); err != nil {
			t.Fatal(err)
		}
	}
	rows, _, err := localListRows(t.Context(), db, "availability", map[string]string{"source": "united", "take": "5"})
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, raw := range rows {
		var v struct {
			Source string `json:"Source"`
		}
		if err := json.Unmarshal(raw, &v); err != nil || v.Source != "united" {
			t.Fatalf("row=%s source=%q err=%v", raw, v.Source, err)
		}
	}
}

func TestLocalListRowsLegacyStoreFallsBackToResources(t *testing.T) {
	db := openLocalListTestStore(t)
	seedLegacyResource(t, db, "routes", "u1", map[string]any{"ID": "u1", "Source": "united"})
	seedLegacyResource(t, db, "routes", "a1", map[string]any{"ID": "a1", "Source": "aeroplan"})
	seedLegacyResource(t, db, "routes", "u2", map[string]any{"ID": "u2", "Source": "united"})
	seedLegacyResource(t, db, "routes", "u3", map[string]any{"ID": "u3", "Source": "united"})

	var hints bytes.Buffer
	ctx := withLocalListHintWriter(t.Context(), &hints)
	rows, unsupported, err := localListRows(ctx, db, "routes", map[string]string{"source": "united", "take": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(unsupported) != 0 {
		t.Fatalf("rows=%d unsupported=%v", len(rows), unsupported)
	}
	for _, raw := range rows {
		var row struct {
			Source string `json:"Source"`
		}
		if err := json.Unmarshal(raw, &row); err != nil || row.Source != "united" {
			t.Fatalf("row=%s source=%q err=%v", raw, row.Source, err)
		}
	}
	if got := hints.String(); !strings.Contains(got, "hint: local routes rows come from the pre-2.0 store layout") {
		t.Fatalf("hint=%q", got)
	}
}

func TestLocalListRowsTypedNonEmptyMissIsFinal(t *testing.T) {
	db := openLocalListTestStore(t)
	if err := db.UpsertRoutes(rawObject(t, map[string]any{"ID": "a1", "Source": "aeroplan"})); err != nil {
		t.Fatal(err)
	}
	visits := 0
	localListStreamVisitHook = func() { visits++ }
	t.Cleanup(func() { localListStreamVisitHook = nil })
	var hints bytes.Buffer
	rows, _, err := localListRows(withLocalListHintWriter(t.Context(), &hints), db, "routes", map[string]string{"source": "united"})
	if err != nil || len(rows) != 0 || visits != 0 {
		t.Fatalf("rows=%d visits=%d err=%v", len(rows), visits, err)
	}
	if hints.Len() != 0 {
		t.Fatalf("unexpected legacy hint: %q", hints.String())
	}
}

func TestLocalListRowsPartiallyMigratedStoreFallsBackToResources(t *testing.T) {
	db := openLocalListTestStore(t)
	for _, row := range []map[string]any{{"ID": "u1", "Source": "united"}, {"ID": "u2", "Source": "united"}} {
		if err := db.UpsertAvailability(rawObject(t, row)); err != nil {
			t.Fatal(err)
		}
	}
	seedLegacyResource(t, db, "availability", "a1", map[string]any{"ID": "a1", "Source": "aeroplan"})
	seedLegacyResource(t, db, "availability", "a2", map[string]any{"ID": "a2", "Source": "aeroplan"})

	var hints bytes.Buffer
	rows, unsupported, err := localListRows(withLocalListHintWriter(t.Context(), &hints), db, "availability", map[string]string{"source": "aeroplan"})
	if err != nil || len(rows) != 2 || len(unsupported) != 0 {
		t.Fatalf("rows=%d unsupported=%v err=%v", len(rows), unsupported, err)
	}
	for _, raw := range rows {
		var row struct {
			Source string `json:"Source"`
		}
		if err := json.Unmarshal(raw, &row); err != nil || row.Source != "aeroplan" {
			t.Fatalf("row=%s source=%q err=%v", raw, row.Source, err)
		}
	}
	if got := hints.String(); !strings.Contains(got, "typed table holds 2 of 4 rows") {
		t.Fatalf("hint=%q", got)
	}
}

func TestSeatsAeroLocalRoutesLegacyStoreEndToEnd(t *testing.T) {
	isolateNovelTest(t)
	db, err := store.Open(defaultDBPath("seats-aero-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	seedLegacyResource(t, db, "routes", "u1", map[string]any{"ID": "u1", "Source": "united"})
	seedLegacyResource(t, db, "routes", "a1", map[string]any{"ID": "a1", "Source": "aeroplan"})
	seedLegacyResource(t, db, "routes", "u2", map[string]any{"ID": "u2", "Source": "united"})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	out, stderr, err := executeRoot("routes", "--source", "united", "--data-source", "local", "--json")
	if err != nil {
		t.Fatalf("execute routes: %v (stderr=%q)", err, stderr.String())
	}
	var envelope struct {
		Results []struct {
			ID     string `json:"ID"`
			Source string `json:"Source"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	if len(envelope.Results) != 2 {
		t.Fatalf("results=%d, want 2: %s", len(envelope.Results), out.String())
	}
	for _, row := range envelope.Results {
		if row.Source != "united" {
			t.Fatalf("unexpected row: %+v", row)
		}
	}
	if got := stderr.String(); !strings.Contains(got, "hint: local routes rows come from the pre-2.0 store layout") {
		t.Fatalf("stderr lacks legacy hint: %q", got)
	}
}

func TestSeatsAeroLocalRoutesPartiallyMigratedStoreEndToEnd(t *testing.T) {
	isolateNovelTest(t)
	db, err := store.Open(defaultDBPath("seats-aero-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []map[string]any{{"ID": "u1", "Source": "united"}, {"ID": "u2", "Source": "united"}} {
		if err := db.UpsertRoutes(rawObject(t, row)); err != nil {
			t.Fatal(err)
		}
	}
	seedLegacyResource(t, db, "routes", "a1", map[string]any{"ID": "a1", "Source": "aeroplan"})
	seedLegacyResource(t, db, "routes", "a2", map[string]any{"ID": "a2", "Source": "aeroplan"})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	out, stderr, err := executeRoot("routes", "--source", "aeroplan", "--data-source", "local", "--json")
	if err != nil {
		t.Fatalf("execute routes: %v (stderr=%q)", err, stderr.String())
	}
	var envelope struct {
		Results []struct {
			Source string `json:"Source"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	if len(envelope.Results) != 2 {
		t.Fatalf("results=%d, want 2: %s", len(envelope.Results), out.String())
	}
	for _, row := range envelope.Results {
		if row.Source != "aeroplan" {
			t.Fatalf("unexpected row: %+v", row)
		}
	}
	if got := stderr.String(); !strings.Contains(got, "typed table holds 2 of 4 rows") {
		t.Fatalf("stderr lacks partial-migration hint: %q", got)
	}
}

func TestLocalListRowsMissingEqualityKeyDoesNotMatch(t *testing.T) {
	db := openLocalListTestStore(t)
	if err := db.Upsert("custom", "a", rawObject(t, map[string]any{"id": "a"})); err != nil {
		t.Fatal(err)
	}
	rows, unsupported, err := localListRows(t.Context(), db, "custom", map[string]string{"kind": "match"})
	if err != nil || len(rows) != 0 || len(unsupported) != 1 || unsupported[0] != "kind" {
		t.Fatalf("rows=%d unsupported=%v err=%v", len(rows), unsupported, err)
	}
}

func TestLocalListRowsStreamingStopsAndHonorsOffset(t *testing.T) {
	db := openLocalListTestStore(t)
	for _, id := range []string{"a", "b", "c", "d"} {
		if err := db.Upsert("custom", id, rawObject(t, map[string]any{"id": id, "kind": "match"})); err != nil {
			t.Fatal(err)
		}
	}
	visits := 0
	localListStreamVisitHook = func() { visits++ }
	t.Cleanup(func() { localListStreamVisitHook = nil })
	rows, _, err := localListRows(context.Background(), db, "custom", map[string]string{"kind": "match", "offset": "1", "limit": "1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || visits != 2 {
		t.Fatalf("rows=%d visits=%d, want 1/2", len(rows), visits)
	}
}

func TestLocalListRowsStreamingFallbackFiltersEquality(t *testing.T) {
	db := openLocalListTestStore(t)
	for _, row := range []map[string]any{
		{"ID": "a", "Source": "aeroplan", "Alliance": "oneworld"},
		{"ID": "b", "Source": "united", "Alliance": "star"},
		{"ID": "c", "Source": "alaska", "Alliance": "oneworld"},
		{"ID": "d", "Source": "aeroplan", "Alliance": "star"},
		{"ID": "e", "Source": "united", "Alliance": "skyteam"},
	} {
		if err := db.UpsertAvailability(rawObject(t, row)); err != nil {
			t.Fatal(err)
		}
	}

	visits := 0
	localListStreamVisitHook = func() { visits++ }
	t.Cleanup(func() { localListStreamVisitHook = nil })
	rows, unsupported, err := localListRows(context.Background(), db, "availability", map[string]string{"Alliance": "star", "take": "2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("unsupported=%v, want none", unsupported)
	}
	if visits == 0 {
		t.Fatal("streaming fallback was not used")
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d, want 2 (take limit)", len(rows))
	}
	for _, raw := range rows {
		var row map[string]any
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		if row["Alliance"] != "star" {
			t.Fatalf("row=%v, want Alliance=star", row)
		}
	}

	// Keep the shared predicate load-bearing too: buffered local results use it.
	if localItemMatchesEquality(map[string]any{"Alliance": "oneworld"}, map[string]string{"Alliance": "star"}) {
		t.Fatal("equality predicate accepted a non-matching value")
	}
}

func TestLocalListRowsTypedUnknownColumnFallsBackAndFilters(t *testing.T) {
	db := openLocalListTestStore(t)
	if err := db.Upsert("availability", "a", rawObject(t, map[string]any{"ID": "a", "custom": "yes"})); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("availability", "b", rawObject(t, map[string]any{"ID": "b", "custom": "no"})); err != nil {
		t.Fatal(err)
	}
	rows, _, err := localListRows(context.Background(), db, "availability", map[string]string{"custom": "yes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
}

func TestLocalListRowsReportsCursorUnsupported(t *testing.T) {
	db := openLocalListTestStore(t)
	if err := db.Upsert("custom", "a", rawObject(t, map[string]any{"id": "a"})); err != nil {
		t.Fatal(err)
	}
	_, unsupported, err := localListRows(context.Background(), db, "custom", map[string]string{"cursor": "next"})
	if err != nil {
		t.Fatal(err)
	}
	if len(unsupported) != 1 || unsupported[0] != "cursor" {
		t.Fatalf("unsupported=%v", unsupported)
	}
}
