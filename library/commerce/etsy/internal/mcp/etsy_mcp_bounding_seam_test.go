// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

func boundingSnapshot(t *testing.T, db *sql.DB, newer bool) {
	t.Helper()
	var terms []string
	for i := 1; i <= 14; i++ {
		terms = append(terms, fmt.Sprintf("linen apron %02d", i))
	}
	terms = append(terms, "ceramic mug", "wool scarf")
	tj, _ := json.Marshal(terms)
	taken := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if newer {
		taken = taken.Add(24 * time.Hour)
	}
	r, err := db.Exec(`INSERT INTO market_snapshots (taken_at,terms_json,pages,sort_on,max_calls,calls_used,status,with_reviews,review_window_days,progress_json) VALUES (?,?,1,'score',200,0,'complete',0,365,'{"pages":[],"reviews_done":[]}')`, taken.Format(time.RFC3339), string(tj))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := r.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for _, shop := range []int64{9000001, 555001} {
		_, err = db.Exec(`INSERT INTO market_shops (snapshot_id,shop_id,transaction_sold_count,listing_active_count,create_date,review_average,review_count,is_vacation) VALUES (?,?,500,20,1600000000,4.8,10,0)`, snap, shop)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i, term := range terms {
		_, err = db.Exec(`INSERT INTO market_term_pages VALUES (?,?,1,4,4)`, snap, term)
		if err != nil {
			t.Fatal(err)
		}
		for j := 0; j < 4; j++ {
			title := fmt.Sprintf("Linen Apron Natural %02d", i*4+j)
			if newer {
				title += " 02"
			}
			_, err = db.Exec(`INSERT INTO market_results (snapshot_id,term,page,position,listing_id,shop_id,title,price_amount,price_divisor,currency,num_favorers,views,tags_json,taxonomy_id,original_creation_ts,is_digital,result_count) VALUES (?,?,1,?,?,?,?,2400,100,'USD',7,40,'[]',1234,NULL,0,500)`, snap, term, j+1, 1000000001+i*4+j, []int64{9000001, 555001}[j%2], title)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
func TestEtsyMCPBoundingSeam(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "etsy-pp-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/etsy-pp-cli")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	resetMCPPathEnv(t)
	for _, key := range []string{"ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_SHOP_ID", "ETSY_OAUTH2", "ETSY_CONFIG", "ETSY_MCP_READ_ONLY", "ETSY_VERIFY", "ETSY_DOGFOOD", "ETSY_BASE_URL"} {
		t.Setenv(key, "")
	}
	t.Setenv("ETSY_NO_LEARN", "true")
	t.Setenv("PP_MCP_MAX_BYTES", "4000")
	t.Setenv("ETSY_CLI_PATH", bin)
	cobratree.ResetSiblingCLIPathCache()
	t.Cleanup(cobratree.ResetSiblingCLIPathCache)
	dir, err := cliutil.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = store.EnsureMarketSchema(db.DB()); err != nil {
		t.Fatal(err)
	}
	boundingSnapshot(t, db.DB(), false)
	// Seed the shared resources mirror used by the other offline readers.
	now := time.Now()
	fixtures := []struct {
		kind string
		row  map[string]any
	}{
		{"listings", map[string]any{"listing_id": 1000000001, "shop_id": 9000001, "title": "linen apron", "state": "active", "views": 0, "num_favorers": 0, "ending_timestamp": now.Add(5 * 24 * time.Hour).Unix(), "original_creation_timestamp": now.Add(-180 * 24 * time.Hour).Unix()}},
		{"listings_inventory", map[string]any{"id": 1000000001, "listing_id": 1000000001, "products": []any{map[string]any{"product_id": 1000000001, "sku": "", "offerings": []any{map[string]any{"offering_id": 1000000001, "quantity": 0, "price": map[string]any{"amount": 2400, "divisor": 100, "currency_code": "USD"}}}}}}},
		{"receipts", map[string]any{"receipt_id": 1000000001, "created_timestamp": now.Add(-48 * time.Hour).Unix(), "is_paid": true, "is_shipped": false, "status": "paid"}},
		{"reviews", map[string]any{"review_id": 1000000001, "listing_id": 1000000001, "rating": 2, "created_timestamp": now.Add(-48 * time.Hour).Unix()}},
		{"transactions", map[string]any{"transaction_id": 1000000001, "listing_id": 1000000001, "shop_id": 9000001, "created_timestamp": now.Add(-180 * 24 * time.Hour).Unix()}},
		{"ledger_entries", map[string]any{"entry_id": 1000000001, "created_timestamp": now.Add(-48 * time.Hour).Unix(), "amount": -20, "currency": "USD", "ledger_type": "listing fee"}},
	}
	for _, fixture := range fixtures {
		raw, err := json.Marshal(fixture.row)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Upsert(fixture.kind, "1000000001", raw); err != nil {
			t.Fatal(err)
		}
		typed := map[string]func(json.RawMessage) error{"listings": db.UpsertListings, "receipts": db.UpsertReceipts, "reviews": db.UpsertReviews, "listings_inventory": db.UpsertListingInventory}
		if write := typed[fixture.kind]; write != nil {
			if err := write(raw); err != nil {
				t.Fatal(err)
			}
		}
	}
	direct := exec.Command(bin, "market", "demand", "--json")
	output, err := direct.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(output) <= 4000 {
		t.Fatalf("fixture too small: %d", len(output))
	}
	var full map[string]any
	if err = json.Unmarshal(output, &full); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("etsy", "test")
	RegisterTools(s)
	call := func(t *testing.T, name string, args map[string]any) (*mcplib.CallToolResult, string, map[string]any) {
		t.Helper()
		tool := s.ListTools()[name]
		if tool == nil {
			t.Fatalf("missing %s", name)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		r, e := tool.Handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: name, Arguments: args}})
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Content) == 0 {
			t.Fatal("missing content")
		}
		text := r.Content[0].(mcplib.TextContent).Text
		sum := 0
		for _, c := range r.Content {
			if v, ok := c.(mcplib.TextContent); ok {
				sum += len(v.Text)
			}
		}
		budget, _ := strconv.Atoi(os.Getenv("PP_MCP_MAX_BYTES"))
		if sum > budget {
			t.Fatalf("response bytes=%d", sum)
		}
		var m map[string]any
		if !r.IsError {
			if e = json.Unmarshal([]byte(text), &m); e != nil {
				t.Fatal(e)
			}
		}
		return r, text, m
	}

	for _, tool := range []string{"reviews_digest", "money_reconcile"} {
		t.Run(tool+"_overbudget_preview_cannot_be_paged", func(t *testing.T) {
			t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "")
			// Multiple populated sibling arrays exceed the budget even with the
			// dominant array removed, so these real command outputs use Preview.
			for i := 0; i < 100; i++ {
				id := 1000010001 + i
				rows := map[string]map[string]any{
					"reviews":        {"review_id": id, "listing_id": id, "rating": 2, "created_timestamp": now.Add(-48 * time.Hour).Unix()},
					"receipts":       {"receipt_id": id, "is_paid": true, "created_timestamp": now.Add(-48 * time.Hour).Unix()},
					"payments":       {"payment_id": id, "receipt_id": id + 1000, "created_timestamp": now.Add(-48 * time.Hour).Unix()},
					"ledger_entries": {"entry_id": id, "reference_type": "receipt", "reference_id": fmt.Sprint(id + 2000), "amount": 20, "currency": "USD", "created_timestamp": now.Add(-48 * time.Hour).Unix()},
				}
				for kind, row := range rows {
					if tool == "reviews_digest" && kind != "reviews" || tool == "money_reconcile" && kind == "reviews" {
						continue
					}
					raw, err := json.Marshal(row)
					if err != nil {
						t.Fatal(err)
					}
					if err := db.Upsert(kind, fmt.Sprint(id), raw); err != nil {
						t.Fatal(err)
					}
				}
			}
			r, text, m := call(t, tool, nil)
			if r.IsError || m["truncated"] != true {
				t.Fatal(text)
			}
			if _, ok := m["preview"]; !ok {
				t.Fatal("missing preview", text)
			}
			if _, ok := m["next_cursor"]; ok {
				t.Fatal("unexpected cursor", text)
			}
			if !strings.Contains(fmt.Sprint(m["note"]), "cannot be paged") {
				t.Fatal(text)
			}
		})
	}

	t.Run("orders_unshipped_stale_message_has_no_snapshot", func(t *testing.T) {
		for i := 0; i < 80; i++ {
			id := fmt.Sprint(1000000100 + i)
			raw, _ := json.Marshal(map[string]any{"receipt_id": 1000000100 + i, "created_timestamp": now.Add(-48 * time.Hour).Unix(), "is_paid": true, "is_shipped": false, "status": "paid"})
			if err := db.Upsert("receipts", id, raw); err != nil {
				t.Fatal(err)
			}
			if err := db.UpsertReceipts(raw); err != nil {
				t.Fatal(err)
			}
		}
		r, text, m := call(t, "orders_unshipped", nil)
		cursor, ok := m["next_cursor"].(string)
		if r.IsError || !ok {
			t.Fatalf("fixture must page: %s", text)
		}
		raw, _ := json.Marshal(map[string]any{"receipt_id": 1000000999, "created_timestamp": now.Unix(), "is_paid": true, "is_shipped": false, "status": "paid"})
		if err := db.Upsert("receipts", "1000000999", raw); err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertReceipts(raw); err != nil {
			t.Fatal(err)
		}
		r, text, _ = call(t, "orders_unshipped", map[string]any{"cursor": cursor})
		if !r.IsError || !strings.HasPrefix(text, "cursor_stale:") || !strings.Contains(text, "without `cursor`") || strings.Contains(text, "snapshot") {
			t.Fatal(text)
		}
	})

	t.Run("empty_cursor_equals_page_one", func(t *testing.T) {
		a, at, _ := call(t, "market_demand", nil)
		b, bt, _ := call(t, "market_demand", map[string]any{"cursor": ""})
		if a.IsError || b.IsError || at != bt {
			t.Fatalf("empty cursor differs: %s / %s", at, bt)
		}
	})
	t.Run("multi_arg_paging_window_compact_top", func(t *testing.T) {
		output, err := exec.Command(bin, "market", "demand", "--window=6mo", "--compact", "--top=2", "--json").Output()
		if err != nil {
			t.Fatal(err)
		}
		var direct map[string]any
		if err := json.Unmarshal(output, &direct); err != nil {
			t.Fatal(err)
		}
		args := map[string]any{"window": "6mo", "compact": true, "top": 2}
		// Repeated stateless requests must bind the same sorted forwarded argv.
		// Multiple requests exercise Go map iteration independently.
		expectedHash := bound.ArgsHash([]string{"market", "demand", "--compact", "--top=2", "--window=6mo"})
		for repeat := 0; repeat < 16; repeat++ {
			r, text, m := call(t, "market_demand", args)
			if r.IsError {
				t.Fatal(text)
			}
			cursor, ok := m["next_cursor"].(string)
			if !ok {
				t.Fatal("fixture must page")
			}
			if _, err := bound.DecodeMirrorCursor(cursor, "market_demand", expectedHash); err != nil {
				t.Fatal(err)
			}
		}
		var union []any
		for page := 0; page < 100; page++ {
			r, text, m := call(t, "market_demand", args)
			if r.IsError {
				t.Fatal(text)
			}
			union = append(union, m["results"].([]any)...)
			cursor, ok := m["next_cursor"].(string)
			if page == 0 && !ok {
				t.Fatal("fixture must page")
			}
			if !ok {
				if !reflect.DeepEqual(union, direct["results"]) {
					t.Fatal("multi argument union differs")
				}
				return
			}
			args["cursor"] = cursor
		}
		t.Fatal("paging did not finish")
	})
	t.Run("compact_true_projects_rows_over_mcp", func(t *testing.T) {
		output, err := exec.Command(bin, "market", "demand", "--compact", "--json").Output()
		if err != nil {
			t.Fatal(err)
		}
		var direct map[string]any
		if err := json.Unmarshal(output, &direct); err != nil {
			t.Fatal(err)
		}
		var union []any
		args := map[string]any{"compact": true}
		for page := 0; page < 100; page++ {
			r, text, m := call(t, "market_demand", args)
			if r.IsError {
				t.Fatal(text)
			}
			rows := m["results"].([]any)
			for _, row := range rows {
				obj := row.(map[string]any)
				for key := range obj {
					switch key {
					case "term", "state", "listings_total", "reviews_in_window", "note", "listings":
					default:
						t.Fatalf("unexpected key %s", key)
					}
				}
			}
			union = append(union, rows...)
			cursor, has := m["next_cursor"].(string)
			if page == 0 && !has {
				t.Fatal("fixture must page")
			}
			if !has {
				if !reflect.DeepEqual(union, direct["results"]) {
					t.Fatal("compact union differs")
				}
				return
			}
			args = map[string]any{"compact": true, "cursor": cursor}
		}
		t.Fatal("paging did not finish")
	})
	t.Run("inline_market_inputs", func(t *testing.T) {
		rules := `{"targets":[{"name":"linen apron","include":["linen apron"]},{"name":"wool scarf","include":["wool scarf"]}]}`
		r, text, m := call(t, "market_gaps", map[string]any{"rules-json": rules, "control-ids": "1000000001,1000000999"})
		if r.IsError {
			t.Fatal(text)
		}
		rows, ok := m["results"].([]any)
		if !ok || len(rows) != 2 || rows[0].(map[string]any)["matching_listings"] != float64(64) || rows[1].(map[string]any)["matching_listings"] != float64(0) || m["gap_claims_downgraded"] != true {
			t.Fatal(text)
		}
		for _, n := range []int{98304, 98305, 122881} {
			value := rules + strings.Repeat(" ", n-len(rules))
			r, text, _ := call(t, "market_gaps", map[string]any{"rules-json": value})
			if n == 98304 && r.IsError {
				t.Fatal(text)
			}
			if n > 98304 && (!r.IsError || !strings.Contains(text, "--rules-json")) {
				t.Fatal(text)
			}
			if n == 98305 && (!strings.Contains(text, "98304") || len(text) > 500) {
				t.Fatal(text)
			}
			if n == 122881 && !strings.Contains(text, "122880-byte per-argument limit") {
				t.Fatal(text)
			}
		}
		r, text, m = call(t, "market_price", map[string]any{"group-by": "class", "rules-json": rules})
		if r.IsError || m["group_by"] != "class" || len(m["results"].([]any)) != 1 {
			t.Fatal(text)
		}
		r, text, _ = call(t, "market_price", map[string]any{"rules-json": rules})
		if !r.IsError || !strings.Contains(text, "--rules-json applies only with --group-by class") {
			t.Fatal(text)
		}
	})
	t.Run("pages_to_completion_union_equals_cli_results", func(t *testing.T) {
		var union []any
		args := map[string]any{}
		for page := 0; page < 100; page++ {
			r, text, m := call(t, "market_demand", args)
			if r.IsError {
				t.Fatal(text)
			}
			rows, ok := m["results"].([]any)
			if !ok {
				t.Fatalf("missing results: %s", text)
			}
			union = append(union, rows...)
			for k, v := range full {
				if k != "results" && !reflect.DeepEqual(m[k], v) {
					t.Fatalf("sibling %s changed: %v != %v", k, m[k], v)
				}
			}
			cursor, has := m["next_cursor"].(string)
			if page == 0 && (m["truncated"] != true || !has) {
				t.Fatal("first page not an envelope")
			}
			if !has {
				if m["truncated"] != false {
					t.Fatal("final page not complete")
				}
				if !reflect.DeepEqual(union, full["results"]) {
					t.Fatal("union differs from CLI")
				}
				return
			}
			if m["truncated"] != true {
				t.Fatal("nonfinal page not truncated")
			}
			args = map[string]any{"cursor": cursor}
		}
		t.Fatal("paging did not finish")
	})
	t.Run("cursor_args_mismatch_and_invalid", func(t *testing.T) {
		r, text, m := call(t, "market_demand", map[string]any{"window": "6mo"})
		if r.IsError {
			t.Fatal(text)
		}
		cursor, ok := m["next_cursor"].(string)
		if !ok {
			t.Fatal("missing cursor")
		}
		for _, tc := range []struct{ cursor, code string }{{cursor, "cursor_args_mismatch:"}, {"garbage", "cursor_invalid:"}} {
			r, text, _ = call(t, "market_demand", map[string]any{"cursor": tc.cursor})
			if !r.IsError || !strings.HasPrefix(text, tc.code) || len(r.Content) != 1 {
				t.Fatalf("wrong cursor error: %s", text)
			}
		}
	})
	t.Run("market_demand_stale_message_has_snapshot_advice", func(t *testing.T) {
		r, text, m := call(t, "market_demand", nil)
		if r.IsError {
			t.Fatal(text)
		}
		if _, ok := m["snapshot_id"]; !ok {
			t.Fatal("plain page one missing snapshot_id", text)
		}
		ar, at, am := call(t, "market_demand", map[string]any{"agent": true})
		inner, ok := am["results"].(map[string]any)
		if ar.IsError || !ok {
			t.Fatal(at)
		}
		if _, ok := inner["snapshot_id"]; !ok {
			t.Fatal("agent page one missing results.snapshot_id", at)
		}
		cursor, ok := m["next_cursor"].(string)
		if !ok {
			t.Fatal("missing cursor")
		}
		boundingSnapshot(t, db.DB(), true)
		r, text, _ = call(t, "market_demand", map[string]any{"cursor": cursor})
		if !r.IsError || !strings.HasPrefix(text, "cursor_stale:") || !strings.Contains(text, "pass `snapshot`") || !strings.Contains(text, "`results.snapshot_id`") || !strings.Contains(text, "without `select`") {
			t.Fatalf("wrong stale response: %s", text)
		}
	})

	t.Run("agent_true_pages_to_completion", func(t *testing.T) {
		output, err := exec.Command(bin, "market", "demand", "--agent").Output()
		if err != nil {
			t.Fatal(err)
		}
		var full map[string]any
		if err = json.Unmarshal(output, &full); err != nil {
			t.Fatal(err)
		}
		var union []any
		args := map[string]any{"agent": true}
		for i := 0; i < 100; i++ {
			r, text, m := call(t, "market_demand", args)
			if r.IsError {
				t.Fatal(text)
			}
			inner, ok := m["results"].(map[string]any)
			if !ok {
				t.Fatal(text)
			}
			union = append(union, inner["results"].([]any)...)
			if !reflect.DeepEqual(full["meta"], m["meta"]) {
				t.Fatal("meta changed")
			}
			cursor, has := m["next_cursor"].(string)
			// Apply the documented rule literally: absent top-level truncated means complete.
			incomplete := m["truncated"] == true
			if has != incomplete {
				t.Fatal("top-level completeness rule disagrees with cursor")
			}
			if !has {
				if m["truncated"] != false || !reflect.DeepEqual(union, full["results"].(map[string]any)["results"]) {
					t.Fatal("final union differs")
				}
				return
			}
			args = map[string]any{"agent": true, "cursor": cursor}
		}
		t.Fatal("paging did not finish")
	})
	// A counting companion proves cursor rejection precedes spawning the child.
	noChild := func(t *testing.T, fn func()) {
		t.Helper()
		dir := t.TempDir()
		marker := filepath.Join(dir, "calls")
		helper := filepath.Join(dir, "etsy-pp-cli")
		if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf call >> '"+marker+"'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ETSY_CLI_PATH", helper)
		cobratree.ResetSiblingCLIPathCache()
		defer func() { os.Setenv("ETSY_CLI_PATH", bin); cobratree.ResetSiblingCLIPathCache() }()
		fn()
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("child spawned", err)
		}
	}
	t.Run("cursor_wrong_tool", func(t *testing.T) {
		r, text, m := call(t, "market_demand", nil)
		if r.IsError {
			t.Fatal(text)
		}
		noChild(t, func() {
			r, text, _ := call(t, "market_shops", map[string]any{"cursor": m["next_cursor"]})
			if !r.IsError || !strings.HasPrefix(text, "cursor_wrong_tool:") {
				t.Fatal(text)
			}
		})
	})
	t.Run("schema_assertions", func(t *testing.T) {
		check := func(s *server.MCPServer, name string, want bool) {
			t.Helper()
			tool := s.ListTools()[name]
			if tool == nil {
				t.Fatal("missing", name)
			}
			prop, ok := tool.Tool.InputSchema.Properties["cursor"]
			if ok != want {
				t.Fatal(name, prop)
			}
			if want && prop.(map[string]any)["type"] != "string" {
				t.Fatal(prop)
			}
		}
		snapshotTools := map[string]bool{"market_demand": true, "market_gaps": true, "market_price": true, "market_shops": true, "market_tags": true}
		for _, name := range []string{"inventory_audit", "listings_audit", "listings_dead_weight", "listings_expiring", "market_demand", "market_gaps", "market_price", "market_shops", "market_tags", "market_trend", "money_fees", "money_reconcile", "orders_unshipped", "reviews_digest", "shop_attention"} {
			check(s, name, true)
			_, has := s.ListTools()[name].Tool.InputSchema.Properties["snapshot"]
			if has != snapshotTools[name] {
				t.Fatalf("snapshot scope %s: %v", name, has)
			}
		}
		check(s, "market_demand", true)
		check(s, "market_snapshot", false)
		check(s, "money_summary", false)
		t.Setenv(cliutil.MCPReadOnlyEnvVar, "1")
		ro := server.NewMCPServer("etsy", "test")
		RegisterTools(ro)
		check(ro, "inventory_low_stock", false)
		check(ro, "market_demand", true)
	})
	t.Run("read_only_tool_without_local_source_rejects_cursor", func(t *testing.T) {
		var nonlocal []string
		root := cli.RootCmd()
		auto := 0
		var walk func(*cobra.Command)
		walk = func(cmd *cobra.Command) {
			name := strings.ReplaceAll(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" "), " ", "_")
			if s.ListTools()[name] != nil {
				if cmd.Flags().Lookup("cursor") != nil {
					t.Fatal("native cursor", name)
				}
				if cmd.Annotations["mcp:read-only"] == "true" && cmd.Annotations["pp:data-source"] != "local" {
					nonlocal = append(nonlocal, name)
					if cmd.Annotations["pp:data-source"] == "auto" {
						auto++
					}
				}
			}
			for _, child := range cmd.Commands() {
				walk(child)
			}
		}
		walk(root)
		if len(nonlocal) == 0 {
			t.Log("no registered nonlocal literal read-only mirror")
		}
		t.Logf("nonlocal literal read-only mirrors=%v auto=%d", nonlocal, auto)
		nonlocal = append(nonlocal, "market_snapshot")
		noChild(t, func() {
			for _, name := range nonlocal {
				r, text, _ := call(t, name, map[string]any{"cursor": "garbage"})
				if !r.IsError || !strings.Contains(text, "unknown MCP parameter") {
					t.Fatal(name, text)
				}
			}
		})
	})
	t.Run("determinism_table_each_cursor_tool_twice_equal_digests", func(t *testing.T) {
		t.Setenv("ETSY_SHOP_ID", "9000001")
		boundingSnapshot(t, db.DB(), true)
		t.Setenv("PP_MCP_MAX_BYTES", "16777216")
		var names []string
		for name, tool := range s.ListTools() {
			if _, ok := tool.Tool.InputSchema.Properties["cursor"]; ok {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) < 2 {
			t.Fatal("cursor generalisation missing")
		}
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				args := map[string]any{}
				if name == "market_gaps" {
					args["rules-json"] = `{"targets":[{"name":"linen apron","include":["linen apron"]}]}`
				}
				a, at, am := call(t, name, args)
				b, bt, _ := call(t, name, args)
				if len(am) == 0 {
					t.Fatal("empty output", name, at)
				}
				for key, value := range am {
					// Empty diagnostic notes do not mean the result fixture is empty.
					if rows, ok := value.([]any); ok && key != "notes" && len(rows) == 0 {
						t.Fatalf("empty_fixture tool=%s field=%s", name, key)
					}
				}
				if a.IsError != b.IsError || at != bt || !reflect.DeepEqual(a.Content, b.Content) {
					t.Fatalf("non_deterministic tool=%s first=%s second=%s", name, at, bt)
				}
				if a.IsError {
					t.Fatal("cursor tool failed", name, at)
				} else {
					t.Log("identical", name)
				}
			})
		}
	})
	t.Run("market_snapshot_overbudget_no_cursor_and_do_not_rerun_note", func(t *testing.T) {
		var calls atomic.Int32
		fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/v3/application/listings/active" || r.URL.Path == "/listings/active" {
				fmt.Fprint(w, `{"count":0,"results":[]}`)
				return
			}
			fmt.Fprint(w, `{}`)
		}))
		defer fake.Close()
		t.Setenv("ETSY_BASE_URL", fake.URL)
		t.Setenv("ETSY_API_KEY", "synthetic")
		t.Setenv("ETSY_SHARED_SECRET", "synthetic")
		var terms []any
		for i := 0; i < 12; i++ {
			terms = append(terms, fmt.Sprintf("linen apron %02d %s", i, strings.Repeat("generic ", 60)))
		}
		r, text, m := call(t, "market_snapshot", map[string]any{"term": terms, "per-page": 1, "max-calls": 20})
		if r.IsError {
			t.Fatal(text)
		}
		if m["truncated"] != true || m["preview"] == nil {
			t.Fatal(text)
		}
		if _, ok := m["next_cursor"]; ok {
			t.Fatal("unexpected cursor")
		}
		note, _ := m["note"].(string)
		if !strings.Contains(note, "Do not re-run") || strings.Contains(note, "--agent") || strings.Contains(note, "next_cursor") {
			t.Fatal(note)
		}
		// Every tool named by the actual emitted guidance must be registered.
		parts := strings.Split(note, "`")
		if len(parts) < 3 {
			t.Fatal("missing registered tool examples", note)
		}
		for i := 1; i < len(parts); i += 2 {
			if s.ListTools()[parts[i]] == nil {
				t.Fatalf("unregistered guidance tool %q", parts[i])
			}
		}
		t.Logf("fake_calls=%d", calls.Load())
		if n := calls.Load(); n == 0 || n > 40 {
			t.Fatal("unbounded fake calls", n)
		}
	})
	for _, tc := range []struct {
		name, tool string
		params     []string
	}{
		{"market_tags_overbudget_preview_note_names_limit", "market_tags", []string{"limit"}},
		{"market_trend_overbudget_preview_note_names_narrowing", "market_trend", []string{"compact", "select"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Two complete snapshots with disjoint listing IDs make trend's nested
			// entered/left sections large; shared synthetic tags make tags large.
			boundingSnapshot(t, db.DB(), false)
			boundingSnapshot(t, db.DB(), true)
			var latest int64
			if err := db.DB().QueryRow(`SELECT max(id) FROM market_snapshots`).Scan(&latest); err != nil {
				t.Fatal(err)
			}
			var tags []string
			var words []string
			for i := 0; i < 80; i++ {
				tags = append(tags, fmt.Sprintf("ceramic mug %03d", i))
				// Alphabetic suffixes survive title tokenization.
				words = append(words, fmt.Sprintf("linenapron%c%c", 'a'+rune(i/26), 'a'+rune(i%26)))
			}
			encoded, _ := json.Marshal(tags)
			if _, err := db.DB().Exec(`UPDATE market_results SET tags_json=?, title=? WHERE snapshot_id IN (?,?)`, string(encoded), strings.Join(words, " "), latest-1, latest); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().Exec(`UPDATE market_results SET listing_id=listing_id+10000 WHERE snapshot_id=?`, latest); err != nil {
				t.Fatal(err)
			}
			args := map[string]any{}
			if tc.tool == "market_tags" {
				args["limit"] = 0
			} else {
				args["from"] = latest - 1
				args["to"] = latest
			}
			r, text, m := call(t, tc.tool, args)
			if r.IsError || m["truncated"] != true || m["preview"] == nil {
				t.Fatal(text)
			}
			if _, ok := m["next_cursor"]; ok {
				t.Fatal("preview cursor", text)
			}
			note, _ := m["note"].(string)
			if strings.Contains(note, "next_cursor") || !strings.Contains(note, "cannot be paged") {
				t.Fatal(note)
			}
			if tc.tool == "market_trend" {
				properties := s.ListTools()[tc.tool].Tool.InputSchema.Properties
				if properties["from"] == nil || properties["to"] == nil || properties["snapshot"] != nil {
					t.Fatal(properties)
				}
				if strings.Contains(note, "`from`") || strings.Contains(note, "`to`") {
					t.Fatal(note)
				}
			}
			for _, param := range tc.params {
				if !strings.Contains(note, "`"+param+"`") {
					t.Fatal(note)
				}
			}
		})
	}

}
