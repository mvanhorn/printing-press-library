// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestIPExprMatches(t *testing.T) {
	row := map[string]any{"close": 120.0, "sma50": 100.0, "sma200": 90.0, "rs": 75.0, "sector": "IT", "rsi14": 40.0}
	cases := []struct {
		expr string
		want bool
	}{
		{"close > sma50 and sma50 > sma200 and rs >= 70", true},
		{"close > sma50 and rs >= 80", false},
		{"close > sma50 or rs >= 80", true},
		{"not (rs < 50)", true},
		{"!(rs > 50)", false},
		{"close / sma50 > 1.1", true},
		{"close - sma50 == 20", true},
		{"rs = 75", true},
		{"sector == 'IT'", true},
		{"sector == 'it'", true},
		{"sector != \"FMCG\"", true},
		{"price > 100 && rsi < 50", true}, // aliases price->close, rsi->rsi14
		{"-rs < 0", true},
		{"missing > 1", false},
		{"rs > 1 + 2 * 3", true},
		{"(rs > 80) or (rs > 70 and rsi14 < 45)", true},
	}
	for _, tc := range cases {
		e, err := parseIPExpr(tc.expr)
		if err != nil {
			t.Fatalf("%q: parse error: %v", tc.expr, err)
		}
		if got := e.matches(row); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestIPExprParseErrors(t *testing.T) {
	for _, src := range []string{"", "close >", "(close > 1", "close > 1)", "close $ 2", "'abc", "rs >= 70 70"} {
		if _, err := parseIPExpr(src); err == nil {
			t.Errorf("%q: expected a parse error", src)
		}
	}
}

func TestIPExprValidateUnknownField(t *testing.T) {
	e, err := parseIPExpr("close > sma50 and bogus > 1")
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]bool{"close": true, "sma50": true, "rs": true}
	if err := e.validate(valid); err == nil {
		t.Fatal("expected unknown-field error")
	}
	e2, _ := parseIPExpr("close > sma50")
	if err := e2.validate(valid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := e.fields(); len(got) != 3 {
		t.Fatalf("fields = %v", got)
	}
}

func TestIPExprDivisionByZeroIsNoMatch(t *testing.T) {
	e, _ := parseIPExpr("close / vol > 1")
	if e.matches(map[string]any{"close": 5.0, "volume": 0.0, "vol": 0.0}) {
		t.Fatal("division by zero must not match")
	}
}
