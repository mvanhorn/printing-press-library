package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/bridge"
)

func TestValidateCloseAllFilter(t *testing.T) {
	cases := []struct {
		filter string
		ok     bool
		reason string // substring of expected error message, when !ok
	}{
		{"profit < 0", true, ""},
		{"symbol = 'EURUSD.s' AND volume <= 0.05", true, ""},
		{"magic = 1234 OR (symbol LIKE 'XAU%' AND profit < -10)", true, ""},
		{"", false, "empty"},
		{"   ", false, "empty"},
		{"1=1; DROP TABLE audit", false, ";"},
		{"profit < 0 -- malicious comment", false, "--"},
		{"profit < 0 /* block comment */", false, "/*"},
		{"profit < 0 */", false, "*/"},
	}
	for _, c := range cases {
		err := validateCloseAllFilter(c.filter)
		if c.ok {
			if err != nil {
				t.Errorf("validateCloseAllFilter(%q): got error %v, want OK", c.filter, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("validateCloseAllFilter(%q): got OK, want error containing %q", c.filter, c.reason)
			continue
		}
		if !strings.Contains(err.Error(), c.reason) {
			t.Errorf("validateCloseAllFilter(%q): error %v does not contain %q", c.filter, err, c.reason)
		}
	}
}

func TestRoundLotPerStep(t *testing.T) {
	cases := []struct {
		v, step, want float64
	}{
		// Standard FX: 0.01 step
		{0.123, 0.01, 0.12},
		{0.125, 0.01, 0.13}, // banker's rounding doesn't apply, math.Round rounds half-away-from-zero
		{0.155, 0.01, 0.16},
		// XAU / indices: 0.1 step
		{0.55, 0.1, 0.6},
		{0.04, 0.1, 0.0},
		{1.23, 0.1, 1.2},
		// Crypto: 0.0001 step
		{0.12345, 0.0001, 0.1235},
		// Fallback when step is 0 or negative
		{0.123, 0, 0.12},
		{0.123, -1, 0.12},
		// Volume already on step boundary
		{0.10, 0.01, 0.10},
		{1.0, 0.1, 1.0},
	}
	for _, c := range cases {
		got := roundLot(c.v, c.step)
		// Allow tiny FP epsilon
		if d := got - c.want; d > 1e-9 || d < -1e-9 {
			t.Errorf("roundLot(%g, step=%g) = %g, want %g", c.v, c.step, got, c.want)
		}
	}
}

func TestIsOKAcceptsNoChanges(t *testing.T) {
	cases := map[int]bool{
		10008: true,  // PLACED
		10009: true,  // DONE
		10025: true,  // NO_CHANGES — was previously rejected as broker error
		10004: false, // REQUOTE
		10013: false, // INVALID
		10019: false, // NO_MONEY
		0:     false,
	}
	for rc, want := range cases {
		if got := isOK(rc); got != want {
			t.Errorf("isOK(%d) = %v, want %v", rc, got, want)
		}
	}
}

func TestRetcodeNameForNoChanges(t *testing.T) {
	if got := retcodeName(10025); got != "NO_CHANGES" {
		t.Errorf("retcodeName(10025) = %q, want NO_CHANGES", got)
	}
}

func TestLooksLikeWriteDoesNotFlagPRAGMA(t *testing.T) {
	cases := map[string]bool{
		"SELECT * FROM deals":                    false,
		"PRAGMA table_info(deals)":               false, // read-only PRAGMA must pass
		"pragma  schema_version":                 false,
		"INSERT INTO deals VALUES (1)":           true,
		"  DELETE FROM audit  ":                  true,
		"WITH x AS (SELECT 1) DELETE FROM audit": false, // prefix-only by design; engine RO catches it
	}
	for q, want := range cases {
		if got := looksLikeWrite(q); got != want {
			t.Errorf("looksLikeWrite(%q) = %v, want %v", q, got, want)
		}
	}
}

func TestHashableIntentKeepsDeviationStripsPrice(t *testing.T) {
	// price moves with the market between dry-run and confirm — strip it.
	// deviation is the user's slippage tolerance — must be in the hash
	// or a confirm with wider deviation would slip through under a hash
	// shown for a tighter one.
	in := map[string]any{
		"symbol":    "EURUSD.s",
		"volume":    0.10,
		"type":      0,
		"price":     1.16234,
		"deviation": 5,
		"sl":        1.16000,
		"tp":        1.16500,
	}
	out := hashableIntent(in)
	if _, ok := out["price"]; ok {
		t.Error("hashableIntent should strip 'price' (moves with market)")
	}
	if d, ok := out["deviation"]; !ok || d != 5 {
		t.Errorf("hashableIntent must keep 'deviation' = 5, got %v ok=%v", d, ok)
	}
	for _, k := range []string{"symbol", "volume", "type", "sl", "tp"} {
		if _, ok := out[k]; !ok {
			t.Errorf("hashableIntent dropped %q which is user intent", k)
		}
	}
}

func TestRealizedPnLFromDealsFiltersClosingEntries(t *testing.T) {
	deals := []bridge.Deal{
		{Entry: 0, PositionID: 1, Profit: 100}, // opening trade: unrealized
		{Entry: 1, PositionID: 1, Profit: -50, Commission: -1, Swap: -2, Fee: -3},
		{Entry: 2, PositionID: 2, Profit: 25, Commission: -1},
		{Entry: 3, PositionID: 3, Profit: 10},
		{Entry: 1, PositionID: 0, Profit: -999}, // balance/credit style deal
	}
	if got, want := realizedPnLFromDeals(deals), -22.0; got != want {
		t.Errorf("realizedPnLFromDeals() = %g, want %g", got, want)
	}
}
