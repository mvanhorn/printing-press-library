// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestPurchaseConfirmationExampleUsesAPIParameterFormats(t *testing.T) {
	cmd := newPurchaseConfirmationPromotedCmd(&rootFlags{})
	for _, want := range []string{"--cinema-id 123", "--film-id 456", "--date 2026-01-15", "--time 09:00"} {
		if !strings.Contains(cmd.Example, want) {
			t.Fatalf("example %q missing %q", cmd.Example, want)
		}
	}
	if strings.Contains(cmd.Example, "550e8400") {
		t.Fatalf("example still contains UUID identifiers: %q", cmd.Example)
	}
}
