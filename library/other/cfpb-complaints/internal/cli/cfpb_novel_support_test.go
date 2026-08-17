// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseWindowAcceptsDayAndWeekShorthand(t *testing.T) {
	for _, value := range []string{"7d", "4w", "24h"} {
		if _, err := parseWindow(value); err != nil {
			t.Fatalf("parseWindow(%q): %v", value, err)
		}
	}
}

func TestStandardCaveatsRejectRateInterpretation(t *testing.T) {
	if len(standardCaveats()) < 3 {
		t.Fatal("expected interpretation caveats")
	}
}
