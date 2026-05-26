// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestTailEmpty verifies tail on an empty corpus renders the
// hint message rather than erroring or emitting an empty table.
func TestTailEmpty(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()
	out, err := runCmd(t, "tail")
	if err != nil {
		t.Fatalf("tail: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "No recent rows") {
		t.Errorf("expected empty-state hint, got: %s", out)
	}
}

// TestTailProducts verifies that seeded products show up in tail output
// and that the JSON shape includes the by_resource map.
func TestTailProducts(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	seedProduct(t, s, "onyx", "ethiopia-1", map[string]any{
		"title": "Ethiopia Test", "origin": "Ethiopia", "in_stock": 1,
	})
	seedProduct(t, s, "sey", "colombia-1", map[string]any{
		"title": "Colombia Test", "origin": "Colombia", "in_stock": 1,
	})

	out, err := runCmd(t, "tail", "--json")
	if err != nil {
		t.Fatalf("tail --json: %v\nout=%s", err, out)
	}
	var resp struct {
		ByResource map[string][]map[string]any `json:"by_resource"`
		Total      int                         `json:"total"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if resp.Total < 2 {
		t.Errorf("expected total >= 2, got %d", resp.Total)
	}
	products, ok := resp.ByResource["products"]
	if !ok {
		t.Fatalf("expected by_resource.products key, got: %+v", resp.ByResource)
	}
	if len(products) < 2 {
		t.Errorf("expected at least 2 product rows, got %d", len(products))
	}
}

// TestTailResourceFilter verifies --resource scopes to a single table.
func TestTailResourceFilter(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	seedProduct(t, s, "onyx", "a", map[string]any{"title": "A"})

	out, err := runCmd(t, "tail", "--resource", "products", "--json")
	if err != nil {
		t.Fatalf("tail: %v\nout=%s", err, out)
	}
	var resp struct {
		ByResource map[string][]map[string]any `json:"by_resource"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, has := resp.ByResource["reviews"]; has {
		t.Error("--resource=products should not include reviews")
	}
	if _, has := resp.ByResource["videos"]; has {
		t.Error("--resource=products should not include videos")
	}
}

// TestParseSince covers the --since flag parser including the "d" suffix
// and stdlib durations.
func TestParseSince(t *testing.T) {
	cases := []struct {
		in       string
		valid    bool
		wantErr  bool
		maxYears float64 // sanity cap on parsed cutoff age
	}{
		{"", false, false, 0},
		{"24h", true, false, 1},
		{"7d", true, false, 1},
		{"1h", true, false, 1},
		{"30m", true, false, 1},
		{"banana", false, true, 0},
		{"0d", false, true, 0},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseSince(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseSince(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if got.Valid != c.valid {
				t.Errorf("parseSince(%q).Valid=%v, want %v", c.in, got.Valid, c.valid)
			}
			if c.valid {
				age := time.Since(got.Time)
				if age < 0 || age > time.Duration(c.maxYears*float64(365*24*time.Hour)) {
					t.Errorf("parseSince(%q) cutoff age %v looks wrong", c.in, age)
				}
			}
		})
	}
	// Sanity-check: invalid sql.NullTime is detectable.
	var zero sql.NullTime
	if zero.Valid {
		t.Error("zero NullTime should not be Valid")
	}
}
