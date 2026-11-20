// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelListingsDeadWeightHelpWires smoke-tests that the listings dead-weight command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelListingsDeadWeightHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"listings", "dead-weight", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("listings dead-weight --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "dead-weight"} {
		if !strings.Contains(help, want) {
			t.Fatalf("listings dead-weight --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelListingsDeadWeightBehavior(t *testing.T) {
	for _, since := range []string{"90d", "400d"} {
		t.Run(since, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings",
				map[string]any{"listing_id": 1000000001, "title": "Linen apron", "state": "active", "num_favorers": 12, "views": 340},
				map[string]any{"listing_id": 1000000002, "state": "active", "num_favorers": 3},
				map[string]any{"listing_id": 1000000003, "state": "active"},
				map[string]any{"listing_id": 1000000004, "state": "inactive"},
				map[string]any{"listing_id": 1000000005, "state": "active", "num_favorers": 8},
				map[string]any{"listing_id": 1000000006, "state": "active", "views": 0})
			tx := map[string]any{"transaction_id": 1000000010, "listing_id": 1000000003, "paid_timestamp": shopxAgo(10 * shopxDay)}
			f.put("transactions", tx, map[string]any{"transaction_id": 1000000011, "listing_id": 1000000005, "created_timestamp": shopxAgo(200 * shopxDay)}, map[string]any{"transaction_id": 1000000012, "listing_id": 1000000002, "created_timestamp": "invalid"})
			f.put("listings_transactions", tx)
			out, _, err := shopxRun(t, f, "listings", "dead-weight", "--since", since)
			if err != nil {
				t.Fatal(err)
			}
			rows := shopxMustList(t, out)
			ids := make([]int64, 0)
			for _, r := range rows {
				id, _ := shopxInt(r["listing_id"])
				ids = append(ids, id)
				if r["window"] != since {
					t.Fatalf("window: %v", r)
				}
				switch id {
				case 1000000001:
					if r["title"] != "Linen apron" || r["views"] != float64(340) || r["num_favorers"] != float64(12) || r["last_sale_at"] != nil {
						t.Fatalf("metrics: %v", r)
					}
				case 1000000002:
					v, ok := r["views"]
					if !ok || v != nil {
						t.Fatalf("missing views must be null: %v", r)
					}
				case 1000000005:
					want := shopxFixedNow.Add(-200 * shopxDay).Format(time.RFC3339)
					if r["last_sale_at"] != want {
						t.Fatalf("last sale: %v", r)
					}
				case 1000000006:
					if r["views"] != float64(0) || r["num_favorers"] != nil {
						t.Fatalf("zero views: %v", r)
					}
				}
			}
			want := []int64{1000000001, 1000000005, 1000000002, 1000000006}
			if since == "400d" {
				want = []int64{1000000001, 1000000002, 1000000006}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("ids %v want %v", ids, want)
			}
		})
	}
}

func TestNovelListingsDeadWeightEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(shopxDeadWeightEmptyName(existing), func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "listings", "dead-weight")
			if err != nil {
				t.Fatal(err)
			}
			if rows := shopxMustList(t, out); len(rows) != 0 {
				t.Fatalf("want empty: %v", rows)
			}
		})
	}
}

func shopxDeadWeightEmptyName(existing bool) string {
	if existing {
		return "existing"
	}
	return "absent"
}

func TestNovelListingsDeadWeightBoundaryAndHints(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active"}, map[string]any{"listing_id": 1000000002, "state": "active"})
	f.put("listings_transactions", map[string]any{"transaction_id": 1000000010, "parent_id": 1000000001, "create_timestamp": shopxAgo(90 * shopxDay)})
	out, _, err := shopxRun(t, f, "listings", "dead-weight")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 1 || rows[0]["listing_id"] != float64(1000000002) {
		t.Fatalf("boundary/parent: %v", rows)
	}
	t.Run("no transactions", func(t *testing.T) {
		f := shopxNewFixture(t)
		f.put("listings", map[string]any{"listing_id": 1000000001, "state": "active"})
		out, stderr, err := shopxRun(t, f, "listings", "dead-weight")
		if err != nil {
			t.Fatal(err)
		}
		if len(shopxMustList(t, out)) != 0 || !strings.Contains(stderr, "transactions not synced") {
			t.Fatalf("missing result or hint: %s %s", out, stderr)
		}
	})
}

func TestNovelListingsDeadWeightInvalidWindow(t *testing.T) {
	for _, window := range []string{"bad", "0d", "-1d"} {
		_, _, err := shopxRun(t, nil, "listings", "dead-weight", "--since", window)
		if err == nil {
			t.Fatalf("accepted %q", window)
		}
	}
}

func TestNovelListingsDeadWeightTransactionAvailability(t *testing.T) {
	const note = "hint: transactions not synced (needs OAuth transactions_r: run 'etsy-pp-cli auth login' then 'etsy-pp-cli sync --resources transactions'); cannot tell which listings had no sales"
	for _, source := range []string{"never synced", "transactions", "listings_transactions", "synced empty", "dependent synced empty"} {
		t.Run(source, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings",
				map[string]any{"listing_id": 1000000001, "state": "active"},
				map[string]any{"listing_id": 1000000002, "state": "active"})
			want := 0
			switch source {
			case "transactions", "listings_transactions":
				f.put(source, map[string]any{"transaction_id": 1000000010, "listing_id": 1000000001, "paid_timestamp": shopxAgo(shopxDay)})
				want = 1
			case "synced empty":
				f.synced("transactions", time.Hour)
				want = 2
			case "dependent synced empty":
				f.synced("listings_transactions", time.Hour)
				want = 2
			}
			out, stderr, err := shopxRun(t, f, "listings", "dead-weight")
			if err != nil {
				t.Fatal(err)
			}
			rows := shopxMustList(t, out)
			if len(rows) != want {
				t.Fatalf("got %s; want %d listings", out, want)
			}
			if source == "never synced" {
				if len(rows) != 0 || !strings.Contains(stderr, note+"\n") {
					t.Fatalf("expected empty array and exact note: %q %q", out, stderr)
				}
				for _, id := range []string{"1000000001", "1000000002"} {
					if strings.Contains(out, id) {
						t.Fatalf("unknown sales returned listing %s: %s", id, out)
					}
				}
			} else if strings.Contains(stderr, note) {
				t.Fatalf("unexpected missing-data note: %s", stderr)
			}
			if want == 1 && rows[0]["listing_id"] != float64(1000000002) {
				t.Fatalf("wrong zero-sale listing: %s", out)
			}
		})
	}
}
