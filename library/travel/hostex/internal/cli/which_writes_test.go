package cli

import (
	"strings"
	"testing"
)

// The calendar write commands must be discoverable through `which` by a
// natural-language query, ahead of the read-side novel commands.
func TestWhichRanksCalendarWritesFirst(t *testing.T) {
	cases := map[string]string{
		"update prices of a listing":     "listings update-prices",
		"update restrictions":            "listings update-restrictions",
		"update inventories of a day":    "listings update-inventories",
		"update price":                   "listings update-prices",
		"update restriction":             "listings update-restrictions",
		"update inventory":               "listings update-inventories",
		"change the price":               "listings update-prices",
		"set a minimum stay restriction": "listings update-restrictions",
	}
	for q, want := range cases {
		got := rankWhich(whichIndex, q, 3)
		if len(got) == 0 || got[0].Entry.Command != want {
			t.Errorf("which %q: top match = %+v, want %q", q, got, want)
		}
	}
}

// A read-side query must never resolve to a write command, even when it
// shares nouns with one ("listing", "price").
func TestWhichReadQueriesDoNotRankWrites(t *testing.T) {
	cases := map[string]string{
		"check listing price drift": "price-parity",
		"price differences":         "price-parity",
		"prices across channels":    "price-parity",
		"price comparison":          "price-parity",
	}
	for q, want := range cases {
		got := rankWhich(whichIndex, q, 3)
		found := false
		for _, m := range got {
			if m.Entry.Command == want {
				found = true
			}
			if strings.Contains(m.Entry.Command, "update-") {
				t.Errorf("which %q returned write command %q", q, m.Entry.Command)
			}
		}
		if !found {
			t.Errorf("which %q: top matches = %+v, want %q among them", q, got, want)
		}
	}
}

// Plural and singular forms must stem to the same key: "prices", "rules" and
// "changes" are the singular plus "s", not "-es" plurals.
func TestWhichSingularStemsPluralsConsistently(t *testing.T) {
	cases := map[string]string{
		"prices": "price", "price": "price",
		"rules": "rule", "changes": "change",
		"restrictions": "restriction", "inventories": "inventory",
		"properties": "property", "expenses": "expense",
		"classes": "class", "taxes": "tax", "watches": "watch", "wishes": "wish",
	}
	for in, want := range cases {
		if got := whichSingular(in); got != want {
			t.Errorf("whichSingular(%q) = %q, want %q", in, got, want)
		}
	}
	for _, pair := range [][2]string{{"prices", "price"}, {"rules", "rule"}, {"changes", "change"}} {
		if !whichTokenMatch(pair[0], pair[1]) {
			t.Errorf("whichTokenMatch(%q, %q) = false, want true", pair[0], pair[1])
		}
	}
}

func TestWhichSingularFormsFindPriceCommands(t *testing.T) {
	writes := map[string]string{
		"update price":  "listings update-prices",
		"update prices": "listings update-prices",
		"change prices": "listings update-prices",
	}
	for q, want := range writes {
		if got := rankWhich(whichIndex, q, 3); len(got) == 0 || got[0].Entry.Command != want {
			t.Errorf("which %q: top match = %+v, want %q", q, got, want)
		}
	}
	for _, q := range []string{"price drift", "check listing price drift", "prices drift", "price rules"} {
		got := rankWhich(whichIndex, q, 3)
		if len(got) == 0 {
			t.Errorf("which %q: no match", q)
			continue
		}
		for _, m := range got {
			if strings.Contains(m.Entry.Command, "update-") {
				t.Errorf("which %q ranked write command %q", q, m.Entry.Command)
			}
		}
	}
	if got := rankWhich(whichIndex, "price drift", 3); len(got) == 0 || got[0].Entry.Command != "price-parity" {
		t.Errorf("which \"price drift\": top match = %+v, want price-parity", got)
	}
}
