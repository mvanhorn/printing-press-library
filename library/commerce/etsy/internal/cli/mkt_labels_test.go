// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestMktWindowLabelPlainWords(t *testing.T) {
	cases := map[string]string{"1y": "1 year", "2y": "2 years", "6mo": "6 months", "1mo": "1 month", "30d": "1 month", "7d": "7 days", "1d": "1 day"}
	for in, want := range cases {
		d, err := mktParseWindow(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got := mktWindowLabel(d); got != want {
			t.Errorf("mktWindowLabel(%s) = %q, want %q", in, got, want)
		}
	}
	if got := mktWindowLabel(90 * time.Minute); got != "1h30m0s" {
		t.Errorf("sub-day window = %q, want Go duration fallback", got)
	}
}

func TestMktShopsDefinitionHasNoRawDurationOrAngleBrackets(t *testing.T) {
	d, _ := mktParseWindow("1y")
	rep := mktShopsReport(&mktSnapshot{ID: 1}, nil, map[int64]mktShop{}, 10, d, 100, time.Now())
	def := rep["crowd_out"].(map[string]any)["new_shop_definition"].(string)
	if def != "younger than 1 year, or fewer than 100 sales" {
		t.Fatalf("new_shop_definition = %q", def)
	}
	if strings.ContainsAny(def, "<>") || strings.Contains(def, "h0m0s") {
		t.Fatalf("definition leaks raw syntax: %q", def)
	}
}

func TestMktCountPluralizes(t *testing.T) {
	if mktCount(1, "listing") != "1 listing" || mktCount(0, "listing") != "0 listings" || mktCount(3, "sale") != "3 sales" {
		t.Fatal("mktCount pluralization wrong")
	}
}
