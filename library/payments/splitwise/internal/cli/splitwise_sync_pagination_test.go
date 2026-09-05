// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

func TestSyncGetExpensesOffsetPagination(t *testing.T) {
	for _, tc := range []struct {
		name         string
		expenseCount int
		wantOffsets  []int
	}{
		{name: "short final page", expenseCount: 45, wantOffsets: []int{0, 20, 40}},
		{name: "empty final page after exact multiple", expenseCount: 40, wantOffsets: []int{0, 20, 40}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const firstID int64 = 3486481838
			var mu sync.Mutex
			var gotOffsets, gotLimits []int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/get_expenses" {
					http.NotFound(w, r)
					return
				}
				offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
				if err != nil && r.URL.Query().Get("offset") != "" {
					t.Errorf("invalid offset %q: %v", r.URL.Query().Get("offset"), err)
					return
				}
				limit := 20
				if raw := r.URL.Query().Get("limit"); raw != "" {
					limit, err = strconv.Atoi(raw)
					if err != nil {
						t.Errorf("invalid limit %q: %v", raw, err)
						return
					}
				}
				mu.Lock()
				gotOffsets = append(gotOffsets, offset)
				gotLimits = append(gotLimits, limit)
				mu.Unlock()

				end := offset + limit
				if end > tc.expenseCount {
					end = tc.expenseCount
				}
				expenses := make([]map[string]any, 0, end-offset)
				for i := offset; i < end; i++ {
					date := fmt.Sprintf("2026-08-%02dT00:00:00Z", i%9+1)
					expenses = append(expenses, map[string]any{
						"id": firstID + int64(i), "description": fmt.Sprintf("Fixture expense %d", i),
						"cost": "12.34", "currency_code": "USD", "date": date,
						"created_at": date, "updated_at": date, "deleted_at": nil,
						"payment": false, "group_id": 1, "users": []any{},
					})
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(map[string]any{"expenses": expenses}); err != nil {
					t.Errorf("encode response: %v", err)
				}
			}))
			defer srv.Close()

			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", "")
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv("XDG_STATE_HOME", "")
			t.Setenv("XDG_CACHE_HOME", "")
			t.Setenv("SPLITWISE_BASE_URL", srv.URL)
			t.Setenv("SPLITWISE_API_KEY", "test")
			t.Setenv("PRINTING_PRESS_DOGFOOD", "")
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			dbPath := filepath.Join(t.TempDir(), "data.db")

			out, stderr, err := runRootArgs(t, "sync", "--resources", "get-expenses", "--db", dbPath, "--full", "--json", "--no-input")
			if err != nil {
				t.Fatalf("sync failed: %v\nstdout=%s\nstderr=%s", err, out, stderr)
			}

			mu.Lock()
			offsets := append([]int(nil), gotOffsets...)
			limits := append([]int(nil), gotLimits...)
			mu.Unlock()
			if !reflect.DeepEqual(offsets, tc.wantOffsets) {
				t.Fatalf("offsets = %v, want %v", offsets, tc.wantOffsets)
			}
			if len(offsets) != len(distinctInts(offsets)) {
				t.Fatalf("offset sequence contains a repeat: %v", offsets)
			}
			wantLimits := make([]int, len(tc.wantOffsets))
			for i := range wantLimits {
				wantLimits[i] = 20
			}
			if !reflect.DeepEqual(limits, wantLimits) {
				t.Fatalf("limits = %v, want %v", limits, wantLimits)
			}

			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ids, err := db.ListIDs("get-expenses")
			if err != nil {
				t.Fatal(err)
			}
			wantIDs := make([]string, tc.expenseCount)
			for i := range wantIDs {
				wantIDs[i] = strconv.FormatInt(firstID+int64(i), 10)
			}
			sort.Strings(ids)
			sort.Strings(wantIDs)
			if !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("stored ids = %v, want exact decimal ids %v", ids, wantIDs)
			}
		})
	}
}

func distinctInts(values []int) map[int]struct{} {
	out := make(map[int]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
