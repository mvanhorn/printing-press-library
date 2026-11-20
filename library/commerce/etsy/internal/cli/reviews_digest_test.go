// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelReviewsDigestHelpWires smoke-tests that the reviews digest command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelReviewsDigestHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"reviews", "digest", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviews digest --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "digest"} {
		if !strings.Contains(help, want) {
			t.Fatalf("reviews digest --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestShopxReviewsDigestContent(t *testing.T) {
	f := shopxNewFixture(t)
	rows := []map[string]any{}
	for i, rating := range []int{5, 5, 2, 4, 3, 1} {
		id := int64(1000000001)
		if i >= 4 {
			id++
		}
		row := map[string]any{"listing_id": id, "rating": rating, "created_timestamp": shopxAgo(shopxDay * time.Duration(i+2)), "transaction_id": i + 1, "buyer_name": "PRIVATE_SYNTHETIC"}
		if i == 2 {
			row["review"] = "Handle broke synthetic"
		}
		if i == 4 {
			row["review"] = "Okay"
		}
		rows = append(rows, row)
	}
	rows = append(rows,
		map[string]any{"listing_id": 1000000004, "rating": 1, "created_timestamp": shopxAgo(90 * shopxDay)},
		map[string]any{"listing_id": 1000000005, "rating": 0, "created_timestamp": shopxAgo(shopxDay)},
		map[string]any{"listing_id": 1000000006, "created_timestamp": shopxAgo(shopxDay)},
		map[string]any{"listing_id": 1000000007, "rating": 3},
	)
	f.put("reviews", rows...)
	duplicate := map[string]any{}
	for k, v := range rows[2] {
		duplicate[k] = v
	}
	duplicate["review_id"] = 1000000011
	f.put("listings_reviews", duplicate)
	out, _, err := shopxRun(t, f, "reviews", "digest")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Handle broke synthetic", "Okay", "PRIVATE_SYNTHETIC"} {
		if strings.Contains(out, secret) {
			t.Fatalf("text leaked: %s", out)
		}
	}
	obj := shopxMustObject(t, out)
	if obj["total"] != float64(6) || obj["average"] != 3.33 || obj["since"] != "30d" {
		t.Fatalf("digest: %s", out)
	}
	wantHist := map[string]any{"1": float64(1), "2": float64(1), "3": float64(1), "4": float64(1), "5": float64(2)}
	if !reflect.DeepEqual(obj["histogram"], wantHist) {
		t.Fatalf("histogram: %s", out)
	}
	wantLow := []any{
		map[string]any{"listing_id": float64(1000000001), "rating": float64(2), "date": "2026-11-12"},
		map[string]any{"listing_id": float64(1000000002), "rating": float64(3), "date": "2026-11-10"},
		map[string]any{"listing_id": float64(1000000002), "rating": float64(1), "date": "2026-11-09"},
	}
	if !reflect.DeepEqual(obj["low_rated"], wantLow) {
		t.Fatalf("low rated: %s", out)
	}
	wantListings := []any{
		map[string]any{"listing_id": float64(1000000001), "average": float64(4), "count": float64(4)},
		map[string]any{"listing_id": float64(1000000002), "average": float64(2), "count": float64(2)},
	}
	if !reflect.DeepEqual(obj["by_listing"], wantListings) {
		t.Fatalf("listing averages: %s", out)
	}
	out, _, err = shopxRun(t, f, "reviews", "digest", "--text")
	if err != nil {
		t.Fatal(err)
	}
	low := shopxMustObject(t, out)["low_rated"].([]any)
	if low[0].(map[string]any)["text"] != "Handle broke synthetic" || low[1].(map[string]any)["text"] != "Okay" {
		t.Fatalf("text: %s", out)
	}
	out, _, err = shopxRun(t, f, "reviews", "digest", "--since", "365d")
	if err != nil {
		t.Fatal(err)
	}
	obj = shopxMustObject(t, out)
	if obj["total"] != float64(7) || obj["low_rated"].([]any)[3].(map[string]any)["listing_id"] != float64(1000000004) {
		t.Fatalf("long window: %s", out)
	}
}

func TestShopxReviewsDigestEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "reviews", "digest")
			if err != nil {
				t.Fatal(err)
			}
			obj := shopxMustObject(t, out)
			want := map[string]any{"since": "30d", "total": float64(0), "average": nil, "histogram": map[string]any{"1": float64(0), "2": float64(0), "3": float64(0), "4": float64(0), "5": float64(0)}, "low_rated": []any{}, "by_listing": []any{}}
			if !reflect.DeepEqual(obj, want) {
				t.Fatalf("empty: %s", out)
			}
		})
	}
}

func TestShopxReviewsDigestBoundaryAndFallback(t *testing.T) {
	f := shopxNewFixture(t)
	boundary := map[string]any{"listing_id": 1000000003, "rating": 3, "create_timestamp": shopxAgo(30 * shopxDay)}
	f.put("reviews", boundary, map[string]any{"rating": 5, "created_timestamp": shopxAgo(shopxDay)}, map[string]any{"listing_id": 1000000008, "rating": 1, "created_timestamp": shopxAgo(30*shopxDay) - 1})
	duplicate := map[string]any{}
	for k, v := range boundary {
		duplicate[k] = v
	}
	duplicate["review_id"] = 1000000012
	f.put("listings_reviews", duplicate)
	out, _, err := shopxRun(t, f, "reviews", "digest")
	if err != nil {
		t.Fatal(err)
	}
	obj := shopxMustObject(t, out)
	if obj["total"] != float64(2) || obj["average"] != float64(4) {
		t.Fatalf("boundary/dedupe: %s", out)
	}
	listings := obj["by_listing"].([]any)
	if listings[0].(map[string]any)["listing_id"] != float64(0) || listings[1].(map[string]any)["listing_id"] != float64(1000000003) {
		t.Fatalf("membership/order: %s", out)
	}
	_, _, err = shopxRun(t, nil, "reviews", "digest", "--since", "bad")
	if err == nil {
		t.Fatal("invalid since accepted")
	}
}
