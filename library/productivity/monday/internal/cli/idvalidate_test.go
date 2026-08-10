// Copyright 2026 bobe. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRequireNumericID(t *testing.T) {
	valid := []string{"1", "12345", "1234567890"}
	for _, v := range valid {
		if err := requireNumericID("item id", v); err != nil {
			t.Errorf("requireNumericID(%q) = %v, want nil", v, err)
		}
	}

	invalid := []string{"", "__printing_press_invalid__", "12a", "1.5", "abc", "-1", " 12"}
	for _, v := range invalid {
		if err := requireNumericID("item id", v); err == nil {
			t.Errorf("requireNumericID(%q) = nil, want error", v)
		}
	}
}
