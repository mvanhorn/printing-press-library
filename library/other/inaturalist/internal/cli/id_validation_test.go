// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRequirePositiveNumericIDs(t *testing.T) {
	if err := requirePositiveNumericIDs("1,42", "id"); err != nil {
		t.Fatalf("valid IDs: %v", err)
	}
	for _, invalid := range []string{"", "0", "-1", "__printing_press_invalid__", "1,nope"} {
		if err := requirePositiveNumericIDs(invalid, "id"); err == nil {
			t.Fatalf("invalid IDs accepted: %q", invalid)
		}
	}
}
