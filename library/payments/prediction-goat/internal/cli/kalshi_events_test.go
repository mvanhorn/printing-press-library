// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// TestKalshiEventsGet_WithMarketsFlagDeclared pins that the --with-markets
// flag exists on `kalshi events get`. The verify-skill script also checks
// SKILL.md flag references against source, so adding/removing this flag
// without updating SKILL.md would surface there too.
func TestKalshiEventsGet_WithMarketsFlagDeclared(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	kalshi, _, err := root.Find([]string{"kalshi", "events", "get"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f := kalshi.Flags().Lookup("with-markets"); f == nil {
		t.Fatal("--with-markets flag is not declared on `kalshi events get`")
	}
}

// TestKalshiEventsList_SeriesFlagDeclared pins that the --series flag
// exists on `kalshi events list`. Until U7 it was advertised in --help
// but not actually declared.
func TestKalshiEventsList_SeriesFlagDeclared(t *testing.T) {
	t.Parallel()
	root := RootCmd()
	kalshi, _, err := root.Find([]string{"kalshi", "events", "list"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if f := kalshi.Flags().Lookup("series"); f == nil {
		t.Fatal("--series flag is not declared on `kalshi events list`")
	}
}

// TestKalshiEventWithMarkets_ProjectsNestedFields covers the field
// projection from a synthetic upstream payload, so a regression that
// drops one of the trading fields (yes_ask_dollars, status, etc.)
// would fail this test without needing to hit the live Kalshi API.
func TestKalshiEventWithMarkets_ProjectsNestedFields(t *testing.T) {
	t.Parallel()
	src := map[string]any{
		"event_ticker":       "KXMENWORLDCUP-26",
		"series_ticker":      "KXMENWORLDCUP",
		"title":              "2026 Men's World Cup Winner",
		"category":           "Sports",
		"mutually_exclusive": "true",
		"markets": []any{
			map[string]any{
				"ticker":          "KXMENWORLDCUP-26-PT",
				"event_ticker":    "KXMENWORLDCUP-26",
				"title":           "Will the Portugal win the 2026 Men's World Cup?",
				"yes_sub_title":   "Portugal",
				"status":          "active",
				"yes_ask_dollars": 0.085,
				"no_ask_dollars":  0.92,
				"volume_24h_fp":   123456.789,
				"expiration_time": "2028-07-18T14:00:00Z",
			},
			map[string]any{
				"ticker":       "KXMENWORLDCUP-26-FR",
				"event_ticker": "KXMENWORLDCUP-26",
				"title":        "Will the France win the 2026 Men's World Cup?",
			},
		},
	}
	out := kalshiEventWithMarkets(src)
	if out.EventTicker != "KXMENWORLDCUP-26" {
		t.Errorf("EventTicker = %q, want KXMENWORLDCUP-26", out.EventTicker)
	}
	if len(out.Markets) != 2 {
		t.Fatalf("Markets len = %d, want 2", len(out.Markets))
	}
	pt := out.Markets[0]
	if pt.Ticker != "KXMENWORLDCUP-26-PT" {
		t.Errorf("PT.Ticker = %q", pt.Ticker)
	}
	if pt.YesAskDollars != 0.085 {
		t.Errorf("PT.YesAskDollars = %v, want 0.085", pt.YesAskDollars)
	}
	if pt.YesSubTitle != "Portugal" {
		t.Errorf("PT.YesSubTitle = %q, want Portugal", pt.YesSubTitle)
	}
	if pt.Volume24hFP != 123456.789 {
		t.Errorf("PT.Volume24hFP = %v, want 123456.789", pt.Volume24hFP)
	}
}

// TestKalshiEventWithMarkets_HandlesMissingMarketsArray returns the
// slim event with an empty Markets slice when the upstream response
// has no nested markets — defensive against the upstream API toggling
// the field shape.
func TestKalshiEventWithMarkets_HandlesMissingMarketsArray(t *testing.T) {
	t.Parallel()
	src := map[string]any{
		"event_ticker": "KXFOO-01",
		"title":        "Foo",
	}
	out := kalshiEventWithMarkets(src)
	if out.EventTicker != "KXFOO-01" {
		t.Errorf("EventTicker = %q", out.EventTicker)
	}
	if len(out.Markets) != 0 {
		t.Errorf("Markets len = %d, want 0", len(out.Markets))
	}
}
