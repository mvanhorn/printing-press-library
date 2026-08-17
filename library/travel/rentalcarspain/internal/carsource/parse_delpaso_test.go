// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"os"
	"testing"

	xhtml "golang.org/x/net/html"
)

func TestParseDelpasoOffers(t *testing.T) {
	f, err := os.Open("testdata/delpaso-offers.html")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	doc, err := xhtml.Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	offers := parseDelpasoOffers(doc)
	if len(offers) == 0 {
		t.Fatal("expected Delpaso offers, got 0")
	}
	for _, o := range offers {
		if o.Supplier != "Delpaso" {
			t.Errorf("supplier = %q", o.Supplier)
		}
		if !o.FullInsurance {
			t.Errorf("expected full insurance true for %s", o.Car)
		}
		if o.Total <= 0 && o.PerDay <= 0 {
			t.Errorf("offer %q has no price", o.Car)
		}
	}
	first := offers[0]
	if first.CarClass == "" {
		t.Errorf("expected car class (group), got empty for %+v", first)
	}
}
