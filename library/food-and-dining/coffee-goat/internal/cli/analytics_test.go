// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAnalyticsOriginsEmpty verifies the command handles an empty corpus
// gracefully — count=0, empty list, no error.
func TestAnalyticsOriginsEmpty(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()

	out, err := runCmd(t, "analytics", "origins", "--json")
	if err != nil {
		t.Fatalf("analytics origins: %v\nout=%s", err, out)
	}
	var resp struct {
		Origins []map[string]any `json:"origins"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if resp.Count != 0 {
		t.Errorf("expected count=0 for empty corpus, got %d", resp.Count)
	}
}

// TestAnalyticsOriginsAggregates verifies the aggregation grouping.
// Seeds 3 Ethiopian + 2 Colombian + 1 Kenyan; expects Ethiopia first.
func TestAnalyticsOriginsAggregates(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	for _, h := range []string{"ethiopia-1", "ethiopia-2", "ethiopia-3"} {
		seedProduct(t, s, "onyx", h, map[string]any{
			"title": "Ethiopia " + h, "origin": "Ethiopia", "in_stock": 1,
		})
	}
	for _, h := range []string{"colombia-1", "colombia-2"} {
		seedProduct(t, s, "sey", h, map[string]any{
			"title": "Colombia " + h, "origin": "Colombia", "in_stock": 1,
		})
	}
	seedProduct(t, s, "partners", "kenya-1", map[string]any{
		"title": "Kenya 1", "origin": "Kenya", "in_stock": 0,
	})

	out, err := runCmd(t, "analytics", "origins", "--json")
	if err != nil {
		t.Fatalf("analytics origins: %v\nout=%s", err, out)
	}
	var resp struct {
		Origins []struct {
			Origin  string `json:"origin"`
			Total   int    `json:"total"`
			InStock int    `json:"in_stock"`
		} `json:"origins"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if len(resp.Origins) < 3 {
		t.Fatalf("expected 3 origins, got %d (%+v)", len(resp.Origins), resp.Origins)
	}
	// Ethiopia should come first (highest count).
	if resp.Origins[0].Origin != "Ethiopia" || resp.Origins[0].Total != 3 {
		t.Errorf("expected Ethiopia=3 first, got origin=%q total=%d", resp.Origins[0].Origin, resp.Origins[0].Total)
	}
	// Find Kenya — should show 1 total, 0 in_stock.
	for _, o := range resp.Origins {
		if o.Origin == "Kenya" {
			if o.Total != 1 || o.InStock != 0 {
				t.Errorf("Kenya: expected total=1 in_stock=0, got total=%d in_stock=%d", o.Total, o.InStock)
			}
		}
	}
}

// TestAnalyticsRoasters verifies grouping by roaster + in-stock computation.
func TestAnalyticsRoasters(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	seedProduct(t, s, "onyx", "a", map[string]any{"title": "A", "in_stock": 1, "price_cents": 2000, "weight_g": 250})
	seedProduct(t, s, "onyx", "b", map[string]any{"title": "B", "in_stock": 0, "price_cents": 2200, "weight_g": 250})
	seedProduct(t, s, "sey", "c", map[string]any{"title": "C", "in_stock": 1, "price_cents": 2500, "weight_g": 250})

	out, err := runCmd(t, "analytics", "roasters", "--json")
	if err != nil {
		t.Fatalf("analytics roasters: %v\nout=%s", err, out)
	}
	var resp struct {
		Roasters []struct {
			Roaster string `json:"roaster"`
			Total   int    `json:"total"`
			InStock int    `json:"in_stock"`
		} `json:"roasters"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	// onyx (2 products) should appear ahead of sey (1).
	if len(resp.Roasters) < 2 {
		t.Fatalf("expected at least 2 roasters, got %d", len(resp.Roasters))
	}
	if resp.Roasters[0].Roaster != "onyx" || resp.Roasters[0].Total != 2 {
		t.Errorf("expected onyx=2 first, got %q=%d", resp.Roasters[0].Roaster, resp.Roasters[0].Total)
	}
	if resp.Roasters[0].InStock != 1 {
		t.Errorf("expected onyx in_stock=1, got %d", resp.Roasters[0].InStock)
	}
}

// TestAnalyticsBrewsByMonthEmpty verifies the brews-by-month subcommand
// handles an empty brew log without erroring.
func TestAnalyticsBrewsByMonthEmpty(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()
	out, err := runCmd(t, "analytics", "brews-by-month", "--json")
	if err != nil {
		t.Fatalf("brews-by-month: %v\nout=%s", err, out)
	}
	var resp struct {
		Months []map[string]any `json:"months"`
		Count  int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if resp.Count != 0 {
		t.Errorf("expected count=0 for empty brews, got %d", resp.Count)
	}
}
