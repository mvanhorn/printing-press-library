// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

func seedHistory(t *testing.T, path string, at ...time.Time) {
	t.Helper()
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows := make([]store.WaitRow, 0, len(at))
	for i, ts := range at {
		rows = append(rows, store.WaitRow{ParkID: 275, RideID: i + 1, RideName: "Ride", IsOpen: true, WaitMinutes: 20, SourceUpdatedAt: ts, FetchedAt: ts})
	}
	if _, err := s.InsertWaitRows(ctx, rows); err != nil {
		t.Fatal(err)
	}
}

func stampSchema(t *testing.T, path string, v int) {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.DB().Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
		t.Fatal(err)
	}
}

func TestCollectCacheReportStatuses(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	missing := collectCacheReport(ctx, filepath.Join(dir, "none.db"), 30*time.Minute)
	if missing["status"] != "unknown" {
		t.Fatalf("missing db: %+v", missing)
	}

	empty := filepath.Join(dir, "empty.db")
	seedHistory(t, empty)
	if rep := collectCacheReport(ctx, empty, 30*time.Minute); rep["status"] != "empty" || rep["rows"] != 0 {
		t.Fatalf("empty db: %+v", rep)
	}

	fresh := filepath.Join(dir, "fresh.db")
	seedHistory(t, fresh, now.Add(-2*time.Hour), now.Add(-5*time.Minute))
	rep := collectCacheReport(ctx, fresh, 30*time.Minute)
	if rep["status"] != "fresh" || rep["rows"] != 2 || rep["last_snapshot"] != jst(now.Add(-5*time.Minute)) || rep["first_snapshot"] != jst(now.Add(-2*time.Hour)) {
		t.Fatalf("fresh db: %+v", rep)
	}
	parkRows, _ := rep["parks"].([]map[string]any)
	if len(parkRows) != 1 || parkRows[0]["park"] != "tds" || parkRows[0]["rows"] != 2 {
		t.Fatalf("parks: %+v", rep["parks"])
	}
	if err := doctorExitForFailOn("stale", map[string]any{"cache": rep}); err != nil {
		t.Fatalf("fresh history tripped --fail-on stale: %v", err)
	}

	stale := collectCacheReport(ctx, fresh, time.Minute)
	if stale["status"] != "stale" || stale["hint"] == nil {
		t.Fatalf("stale verdict: %+v", stale)
	}
	if err := doctorExitForFailOn("stale", map[string]any{"cache": stale}); err == nil {
		t.Fatal("stale history did not trip --fail-on stale")
	}
	if off := collectCacheReport(ctx, fresh, 0); off["status"] != "ok" {
		t.Fatalf("--max-age 0: %+v", off)
	}

	var out bytes.Buffer
	renderCacheReport(&out, stale)
	for _, want := range []string{"Snapshot History: stale", "rows: 2", "tds: 2 rows", "hint:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("render missing %q:\n%s", want, out.String())
		}
	}
}

func TestCollectCacheReportNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	seedHistory(t, path, time.Now())
	stampSchema(t, path, store.StoreSchemaVersion+1)
	rep := collectCacheReport(context.Background(), path, 30*time.Minute)
	if rep["status"] != "error" || !strings.Contains(fmt.Sprint(rep["error"]), "newer than supported") {
		t.Fatalf("newer schema: %+v", rep)
	}
	if err := doctorExitForFailOn("error", map[string]any{"cache": rep}); err == nil {
		t.Fatal("newer schema did not trip --fail-on error")
	}
}

// typical and waits read history through a read-only open, which skips the
// writable open's version gate; loadHistory must refuse a newer schema
// instead of reading a layout it does not know.
func TestLoadHistoryRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	seedHistory(t, path, time.Now())
	stampSchema(t, path, store.StoreSchemaVersion+1)
	var errOut bytes.Buffer
	_, err := loadHistory(context.Background(), &errOut, &rootFlags{}, path, store.WaitQuery{ParkID: 275})
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("loadHistory = %v, want newer-schema error", err)
	}
	if _, err := loadHistory(context.Background(), &errOut, &rootFlags{}, filepath.Join(t.TempDir(), "absent.db"), store.WaitQuery{ParkID: 275}); err != nil {
		t.Fatalf("missing history must not fail: %v", err)
	}
}

func TestDoctorJSONIncludesHistory(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	t.Setenv("JAPAN_THEME_PARKS_BASE_URL", srv.URL)
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"doctor", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor --json: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"cache"`) || !strings.Contains(out.String(), `"store_schema_version"`) {
		t.Fatalf("doctor --json has no history section:\n%s", out.String())
	}
}
