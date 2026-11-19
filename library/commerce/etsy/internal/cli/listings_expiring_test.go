// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelListingsExpiringHelpWires smoke-tests that the listings expiring command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelListingsExpiringHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"listings", "expiring", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("listings expiring --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "expiring"} {
		if !strings.Contains(help, want) {
			t.Fatalf("listings expiring --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestShopxExpiringWindows(t *testing.T) {
	for _, tc := range []struct {
		name, within string
		ids          []int64
	}{
		{"default", "", []int64{1000000001, 1000000002}},
		{"week", "7d", []int64{1000000001}},
		{"quarter", "90d", []int64{1000000001, 1000000002, 1000000003}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := shopxNewFixture(t)
			f.put("listings",
				map[string]any{"listing_id": 1000000001, "title": "Linen apron", "state": "active", "ending_timestamp": shopxIn(5*shopxDay + 3*time.Hour), "should_auto_renew": true},
				map[string]any{"listing_id": 1000000002, "title": "Ceramic bowl", "state": "active", "ending_timestamp": shopxIn(20 * shopxDay), "should_auto_renew": false},
				map[string]any{"listing_id": 1000000003, "state": "active", "ending_timestamp": shopxIn(60 * shopxDay)},
				map[string]any{"listing_id": 1000000004, "state": "inactive", "ending_timestamp": shopxIn(3 * shopxDay)},
				map[string]any{"listing_id": 1000000005, "state": "active", "ending_timestamp": shopxAgo(10 * shopxDay)},
				map[string]any{"listing_id": 1000000006, "state": "active"},
				map[string]any{"listing_id": 1000000007, "state": "active", "ending_timestamp": "banana"},
				map[string]any{"listing_id": 1000000008, "state": "draft", "ending_timestamp": shopxIn(shopxDay)},
			)
			args := []string{"listings", "expiring"}
			if tc.within != "" {
				args = append(args, "--within", tc.within)
			}
			out, _, err := shopxRun(t, f, args...)
			if err != nil {
				t.Fatal(err)
			}
			rows := shopxMustList(t, out)
			if len(rows) != len(tc.ids) {
				t.Fatalf("rows = %s", out)
			}
			for i, id := range tc.ids {
				if rows[i]["listing_id"] != float64(id) {
					t.Fatalf("rows = %s", out)
				}
			}
			first := rows[0]
			if first["title"] != "Linen apron" || first["state"] != "active" || first["days_left"] != float64(5) || first["should_auto_renew"] != true || first["ending_timestamp"] != float64(shopxIn(5*shopxDay+3*time.Hour)) || first["ending_at"] != shopxFixedNow.Add(5*shopxDay+3*time.Hour).Format(time.RFC3339) {
				t.Fatalf("first = %#v", first)
			}
			if len(rows) > 1 && (rows[1]["days_left"] != float64(20) || rows[1]["should_auto_renew"] != false) {
				t.Fatalf("second = %#v", rows[1])
			}
			if len(rows) > 2 {
				value, present := rows[2]["should_auto_renew"]
				if !present || value != nil {
					t.Fatalf("missing renewal must be null: %#v", rows[2])
				}
			}
		})
	}
}

func TestShopxExpiringBoundariesAndOrder(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("listings",
		map[string]any{"listing_id": 1000000001, "state": "active", "ending_timestamp": shopxIn(7 * shopxDay)},
		map[string]any{"listing_id": 1000000002, "state": "active", "ending_timestamp": shopxIn(0)},
		map[string]any{"listing_id": 1000000003, "state": "active", "ending_timestamp": shopxIn(0)},
		map[string]any{"listing_id": 1000000004, "state": "active", "ending_timestamp": shopxIn(7*shopxDay + time.Second)},
		map[string]any{"listing_id": 1000000005, "state": "active", "ending_timestamp": shopxAgo(time.Second)},
	)
	out, _, err := shopxRun(t, f, "listings", "expiring", "--within", "7d")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 3 {
		t.Fatalf("rows = %s", out)
	}
	for i, id := range []int64{1000000002, 1000000003, 1000000001} {
		if rows[i]["listing_id"] != float64(id) {
			t.Fatalf("rows = %s", out)
		}
	}
	if rows[0]["days_left"] != float64(0) || rows[2]["days_left"] != float64(7) {
		t.Fatalf("rows = %s", out)
	}
}

func TestShopxExpiringEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "empty"}[existing], func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "listings", "expiring")
			if err != nil {
				t.Fatal(err)
			}
			if rows := shopxMustList(t, out); len(rows) != 0 {
				t.Fatalf("rows = %s", out)
			}
		})
	}
}

func TestShopxExpiringInvalidWindow(t *testing.T) {
	_, _, err := shopxRun(t, nil, "listings", "expiring", "--within", "banana")
	if err == nil || !strings.Contains(err.Error(), "invalid --within") {
		t.Fatalf("error = %v", err)
	}
}
