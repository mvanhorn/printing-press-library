// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"testing"
)

func TestNewSize(t *testing.T) {
	cases := []struct {
		label, detail string
		wantLabel     string
		price, count  int
	}{
		{"中", "400円/39個", "M", 400, 39},
		{"特大", "1,000円/6個", "XL", 1000, 6},
		{"小", "70個", "S", -1, 70},
		{"大", "1300円", "L", 1300, -1},
		{"特中", "600円", "", 600, -1},
	}
	for _, c := range cases {
		s := NewSize(c.label, c.detail)
		if c.wantLabel == "" {
			if s.Label != nil {
				t.Errorf("%s: label = %v, want nil", c.label, *s.Label)
			}
		} else if s.Label == nil || *s.Label != c.wantLabel {
			t.Errorf("%s: label = %v, want %s", c.label, s.Label, c.wantLabel)
		}
		if (c.price < 0) != (s.PriceYen == nil) || (s.PriceYen != nil && *s.PriceYen != c.price) {
			t.Errorf("%s %s: price = %v, want %d", c.label, c.detail, s.PriceYen, c.price)
		}
		if (c.count < 0) != (s.Count == nil) || (s.Count != nil && *s.Count != c.count) {
			t.Errorf("%s %s: count = %v, want %d", c.label, c.detail, s.Count, c.count)
		}
	}
}

func TestParsePayment(t *testing.T) {
	norm, raw, known, ic := ParsePayment(" 現金 &nbsp;  Suica &nbsp; ")
	if !known || ic == nil || !*ic {
		t.Fatalf("cash+Suica: known=%v ic=%v", known, ic)
	}
	if len(norm) != 2 || norm[0] != "cash" || norm[1] != "suica" || raw[1] != "Suica" {
		t.Fatalf("norm=%v raw=%v", norm, raw)
	}
	_, _, known, ic = ParsePayment(" 現金 &nbsp; ")
	if !known || ic == nil || *ic {
		t.Fatalf("cash only: want known and ic=false, got %v %v", known, ic)
	}
	_, _, known, ic = ParsePayment("情報なし")
	if known || ic != nil {
		t.Fatalf("unknown: want known=false ic=nil, got %v %v", known, ic)
	}
	_, _, known, ic = ParsePayment("")
	if known || ic != nil {
		t.Fatalf("empty: want unknown")
	}
}

func TestParseChangeMachine(t *testing.T) {
	if v, _ := ParseChangeMachine("両替機なし"); v == nil || *v {
		t.Fatal("両替機なし should be false")
	}
	if v, _ := ParseChangeMachine("両替機あり"); v == nil || !*v {
		t.Fatal("両替機あり should be true")
	}
	if v, r := ParseChangeMachine("情報なし"); v != nil || r != nil {
		t.Fatal("情報なし should be unknown")
	}
}

func TestServiceFromJA(t *testing.T) {
	if s, w := ServiceFromJA("コインロッカー"); s != "coin_locker" || w == nil || !*w {
		t.Fatalf("coin locker: %s %v", s, w)
	}
	if s, w := ServiceFromJA("ecbo cloak（エクボクローク）"); s != "ecbo_cloak" || w == nil || *w {
		t.Fatalf("ecbo: %s %v", s, w)
	}
	if s, w := ServiceFromJA("手荷物預かり"); s != "other" || w != nil {
		t.Fatalf("other: %s %v", s, w)
	}
}

func TestSizeRank(t *testing.T) {
	if SizeRank("xl") != 4 || SizeRank("S") != 1 || SizeRank("Q") != 0 {
		t.Fatal("rank mapping wrong")
	}
}
