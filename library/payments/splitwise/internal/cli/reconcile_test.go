// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

func TestReconcileOffsetPagingAndFindings(t *testing.T) {
	var hits atomic.Int64
	updated := "2026-09-03T12:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/get_expenses" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("limit") != "200" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		if r.URL.Query().Get("updated_after") == "" {
			t.Error("updated_after is empty")
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		end := offset + 200
		if end > 243 {
			end = 243
		}
		expenses := make([]map[string]any, 0)
		for id := offset + 1; id <= end; id++ {
			expenses = append(expenses, map[string]any{"id": id, "description": fmt.Sprintf("e %d", id), "updated_at": updated, "deleted_at": nil})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"expenses": expenses})
	}))
	defer server.Close()
	t.Setenv("SPLITWISE_BASE_URL", server.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")

	t.Run("dry run avoids HTTP", func(t *testing.T) {
		before := hits.Load()
		out, stderr, err := runRootArgs(t, "reconcile", "--dry-run", "--json")
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		if !strings.Contains(out, `"dry_run":true`) {
			t.Fatalf("output=%s", out)
		}
		if hits.Load() != before {
			t.Fatalf("HTTP hits changed from %d to %d", before, hits.Load())
		}
	})

	t.Run("finds missing and stale across second page", func(t *testing.T) {
		path := seedReconcileStore(t, updated, false)
		out, stderr, err := runRootArgs(t, "reconcile", "--since", "30d", "--json", "--db", path)
		if ExitCode(err) != 3 {
			t.Fatalf("exit=%d err=%v stderr=%s", ExitCode(err), err, stderr)
		}
		var got reconcileOutput
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("decode: %v raw=%s", err, out)
		}
		if got.ScannedLive != 243 || got.ScannedPages != 2 || got.ScanCapHit {
			t.Fatalf("scan=%d pages=%d cap=%v", got.ScannedLive, got.ScannedPages, got.ScanCapHit)
		}
		if ids := findingIDs(got.MissingLocally); fmt.Sprint(ids) != "[241 242 243]" {
			t.Fatalf("missing=%v", ids)
		}
		if ids := findingIDs(got.StaleLocally); fmt.Sprint(ids) != "[1]" {
			t.Fatalf("stale=%v", ids)
		}
		if got.InSync {
			t.Fatal("in_sync=true")
		}
	})

	t.Run("matching mirror exits zero", func(t *testing.T) {
		path := seedReconcileStore(t, updated, true)
		out, stderr, err := runRootArgs(t, "reconcile", "--since", "30d", "--json", "--db", path)
		if err != nil {
			t.Fatalf("err=%v stderr=%s", err, stderr)
		}
		var got reconcileOutput
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if !got.InSync {
			t.Fatalf("not in sync: %+v", got)
		}
	})

	t.Run("page cap is explicit", func(t *testing.T) {
		path := seedReconcileStore(t, updated, true)
		out, _, err := runRootArgs(t, "reconcile", "--since", "30d", "--max-scan-pages", "1", "--json", "--db", path)
		if ExitCode(err) != 3 {
			t.Fatalf("exit=%d err=%v", ExitCode(err), err)
		}
		var got reconcileOutput
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatal(err)
		}
		if !got.ScanCapHit || !strings.Contains(got.Note, "--max-scan-pages") {
			t.Fatalf("cap=%v note=%q", got.ScanCapHit, got.Note)
		}
	})
}

func seedReconcileStore(t *testing.T, updated string, complete bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "reconcile.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	count := 240
	if complete {
		count = 243
	}
	for id := 1; id <= count; id++ {
		ts := updated
		if !complete && id == 1 {
			ts = "2026-09-02T12:00:00Z"
		}
		body := fmt.Sprintf(`{"id":%d,"description":"e %d","updated_at":%q,"deleted_at":null}`, id, id, ts)
		if err := s.Upsert("get-expenses", strconv.Itoa(id), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveSyncState("get-expenses", "", count); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func findingIDs(rows []reconcileFinding) []int {
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

func TestReconcileClassifiesRemoteDeletionAndLocalOnly(t *testing.T) {
	deleted := "2026-09-03T13:00:00Z"
	local := []Expense{{ID: 1, UpdatedAt: "2026-09-03T12:00:00Z"}, {ID: 2, UpdatedAt: "2026-09-03T12:00:00Z"}}
	live := []Expense{{ID: 1, UpdatedAt: "2026-09-03T12:00:00Z", DeletedAt: &deleted}}
	out := emptyReconcileOutput("", "")
	reconcileCompare(&out, local, live, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), 50)
	if fmt.Sprint(findingIDs(out.DeletedRemotely)) != "[1]" || fmt.Sprint(findingIDs(out.LocalOnly)) != "[2]" || out.InSync {
		t.Fatalf("output=%+v", out)
	}
}

func TestReconcileHelpWires(t *testing.T) {
	out, _, err := runRootArgs(t, "reconcile", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Usage:", "reconcile", "Exit code 3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q:\n%s", want, out)
		}
	}
}
