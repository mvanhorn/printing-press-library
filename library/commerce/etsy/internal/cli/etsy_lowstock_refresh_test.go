package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyDecodeLowStock(t *testing.T, out string, rows *[]s2LowStockRow) {
	t.Helper()
	var env struct {
		Results []s2LowStockRow
		Notes   []string
	}
	s2Decode(t, out, &env)
	if env.Results == nil || env.Notes == nil {
		t.Fatalf("null or absent arrays: %s", out)
	}
	*rows = env.Results
}
func etsyLowStockResults(t *testing.T, out string) []s2LowStockRow {
	var rows []s2LowStockRow
	etsyDecodeLowStock(t, out, &rows)
	return rows
}
func etsyRefreshFixture(t *testing.T, n int) string {
	return s2FixtureDB(t, func(db *store.Store) {
		for i := 1; i <= n; i++ {
			id := 1000000000 + i
			s2Upsert(t, db.UpsertListings, fmt.Sprintf(`{"listing_id":%d,"shop_id":9000001,"state":"active","title":"Juniper Ceramics"}`, id))
			s2Must(t, db.Upsert("listings_inventory", fmt.Sprint(id), json.RawMessage(fmt.Sprintf(`{"parent_id":"%d","listing_id":"%d","product_id":%d,"offerings":[{"quantity":1}]}`, id, id, id))))
		}
	})
}
func TestEtsyLowStockEnvelope(t *testing.T) {
	dEnv(t)
	for _, kind := range []string{"missing", "unset", "own", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("ETSY_SHOP_ID", "9000001")
			path := etsyRefreshFixture(t, 1)
			note := ""
			switch kind {
			case "missing":
				path = filepath.Join(t.TempDir(), "absent.db")
				note = shopxNoteNoStore
			case "unset":
				t.Setenv("ETSY_SHOP_ID", "")
				note = shopxNoteShopUnset
			case "foreign":
				db, err := store.Open(path)
				s2Must(t, err)
				s2Upsert(t, db.UpsertListings, `{"listing_id":1000000002,"shop_id":555001,"title":"Juniper Ceramics"}`)
				s2Must(t, db.Upsert("listings_inventory", "foreign", json.RawMessage(`{"listing_id":1000000002,"product_id":2,"offerings":[{"quantity":1}]}`)))
				s2Must(t, db.Close())
				note = "other shops ignored"
			}
			out, stderr, err := s2Run(t, "inventory", "low-stock", "--db", path, "--json")
			s2Must(t, err)
			etsyLowStockResults(t, out)
			if note != "" && !strings.Contains(out, note) {
				t.Fatal(out)
			}
			if kind == "own" && !strings.Contains(out, `"notes": []`) {
				t.Fatal(out)
			}
			if kind == "missing" && strings.Contains(stderr, "no local mirror") {
				t.Fatal(stderr)
			}
		})
	}
}
func TestEtsyLowStockAtomic(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			dEnv(t)
			s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool { dBatch(w, r, 2); return true })
			path := etsyRefreshFixture(t, 1)
			if fail {
				db, err := sql.Open("sqlite", path)
				s2Must(t, err)
				_, err = db.Exec(`CREATE TRIGGER etsy_fail_inv BEFORE INSERT ON resources WHEN NEW.resource_type='listings_inventory' BEGIN SELECT RAISE(ABORT,'boom'); END;`)
				s2Must(t, err)
				s2Must(t, db.Close())
			}
			_, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json")
			if fail && err == nil {
				t.Fatal("trigger should fail")
			}
			if !fail {
				s2Must(t, err)
			}
			db, err := store.OpenReadOnly(path)
			s2Must(t, err)
			defer db.Close()
			rows, err := s2LowStockRows(db, 3)
			s2Must(t, err)
			want := int64(2)
			if fail {
				want = 1
			}
			if len(rows) != 1 || rows[0].Quantity != want {
				t.Fatalf("rows=%+v", rows)
			}
		})
	}
}
func TestEtsyLowStockSplitBudget(t *testing.T) {
	for _, low := range []bool{false, true} {
		t.Run(fmt.Sprint(low), func(t *testing.T) {
			dEnv(t)
			srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
				if r.URL.Path != "/listings/batch/inventory" {
					t.Errorf("single path: %s", r.URL.Path)
				}
				if strings.Contains(r.URL.Query().Get("listing_ids"), "1000000003") {
					w.WriteHeader(404)
					return true
				}
				dBatch(w, r, 2)
				return true
			})
			path := etsyRefreshFixture(t, 4)
			if low {
				client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"100"}, "X-Remaining-Today": {"5"}}, etsyTestQuotaKey(t))
			}
			out, stderr, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json")
			if low {
				if ExitCode(err) != 7 || len(srv.all()) != 1 {
					t.Fatalf("err=%v calls=%v", err, srv.all())
				}
				return
			}
			s2Must(t, err)
			rows := etsyLowStockResults(t, out)
			fresh := 0
			for _, r := range rows {
				if r.Quantity == 2 {
					fresh++
				}
			}
			if fresh != 3 || !strings.Contains(stderr, "1 of 4 inventory reads failed") || len(srv.all()) > 7 {
				t.Fatalf("out=%s stderr=%s calls=%v", out, stderr, srv.all())
			}
		})
	}
}
func TestEtsyLowStockMCP(t *testing.T) {
	dEnv(t)
	cmd := newS2InventoryLowStockCmd(&rootFlags{})
	if cmd.Annotations["mcp:read-only-unless-flags"] != "refresh" || cmd.Annotations["mcp:read-only"] != "" {
		t.Fatal(cmd.Annotations)
	}
	t.Setenv("ETSY_MCP_CHILD", "1")
	_, stderr, err := s2Run(t, "inventory", "low-stock", "--refresh", "--refresh-limit", "0", "--dry-run")
	s2Must(t, err)
	if !strings.Contains(stderr, "MCP: --refresh-limit clamped") {
		t.Fatal(stderr)
	}
}
func TestEtsyLowStockContexts(t *testing.T) {
	dEnv(t)
	t.Setenv("ETSY_MCP_CHILD", "")
	old := etsyLowStockCallTimeout
	etsyLowStockCallTimeout = 50 * time.Millisecond
	t.Cleanup(func() { etsyLowStockCallTimeout = old })
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool { <-r.Context().Done(); return true })
	path := etsyRefreshFixture(t, 1)
	_, stderr, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json")
	s2Must(t, err)
	if !strings.Contains(stderr, "1 of 1 inventory reads failed") {
		t.Fatal(stderr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var flags rootFlags
	root := newRootCmd(&flags)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"inventory", "low-stock", "--refresh", "--db", path, "--json"})
	err = root.ExecuteContext(ctx)
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("err=%v calls=%v", err, srv.all())
	}
}
