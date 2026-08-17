// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestRequisitionsCompletedUsesFixedCompletedFilter(t *testing.T) {
	params := completedRequisitionParams()
	if got := params["pending"]; got != "false" {
		t.Fatalf("completed pending filter = %q, want false", got)
	}
	if len(params) != 1 {
		t.Fatalf("completed params = %#v, want only the fixed pending filter", params)
	}

	cmd := newRequisitionsCompletedCmd(&rootFlags{})
	if flag := cmd.Flags().Lookup("pending"); flag != nil {
		t.Fatalf("completed command must encode its filter, not expose --pending")
	}
	if got, want := cmd.Example, "  function-health-pp-cli requisitions completed"; got != want {
		t.Fatalf("completed example = %q, want %q", got, want)
	}
}
