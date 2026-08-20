// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package fx

import (
	"math"
	"testing"
)

const sampleECB = `<?xml version="1.0" encoding="UTF-8"?>
<gesmes:Envelope xmlns:gesmes="http://www.gesmes.org/xml/2002-08-01" xmlns="http://www.ecb.int/vocabulary/2002-08-01/eurofxref">
 <Cube>
  <Cube time="2026-07-14">
   <Cube currency="USD" rate="1.0856"/>
   <Cube currency="GBP" rate="0.8452"/>
   <Cube currency="JPY" rate="170.12"/>
  </Cube>
 </Cube>
</gesmes:Envelope>`

func TestParseAndConvert(t *testing.T) {
	r, err := ParseECB([]byte(sampleECB))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Date != "2026-07-14" {
		t.Errorf("date = %q", r.Date)
	}
	if !r.Supported("EUR") || !r.Supported("usd") || !r.Supported("GBP") {
		t.Error("EUR/USD/GBP should be supported")
	}
	if r.Supported("XYZ") {
		t.Error("XYZ should not be supported")
	}

	// EUR passes through.
	if got, ok := r.Convert(100, "EUR"); !ok || got != 100 {
		t.Errorf("EUR passthrough = %v, %v", got, ok)
	}
	// 289 EUR -> GBP at 0.8452.
	got, ok := r.Convert(289, "gbp")
	if !ok || math.Abs(got-244.26) > 0.01 {
		t.Errorf("289 EUR in GBP = %v (want ~244.26), ok=%v", got, ok)
	}
	// Unknown currency.
	if _, ok := r.Convert(100, "XYZ"); ok {
		t.Error("unknown currency should return ok=false")
	}
}

func TestParseECBEmpty(t *testing.T) {
	if _, err := ParseECB([]byte(`<x/>`)); err == nil {
		t.Error("expected error for feed with no rates")
	}
}
