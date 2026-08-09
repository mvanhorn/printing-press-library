// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestEIAAuthEnvironmentVariable(t *testing.T) {
	t.Setenv("EIA_ENERGY_API_KEY", "")
	t.Setenv("EIA_API_KEY", "")
	if got := eiaAuthEnvironmentVariable(); got != "" {
		t.Fatalf("unset environment = %q", got)
	}

	t.Setenv("EIA_API_KEY", "legacy")
	if got := eiaAuthEnvironmentVariable(); got != "EIA_API_KEY" {
		t.Fatalf("legacy environment = %q", got)
	}

	t.Setenv("EIA_ENERGY_API_KEY", "canonical")
	if got := eiaAuthEnvironmentVariable(); got != "EIA_ENERGY_API_KEY" {
		t.Fatalf("canonical environment precedence = %q", got)
	}
}
