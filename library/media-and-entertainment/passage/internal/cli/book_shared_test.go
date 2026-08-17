// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestValidWorkKey(t *testing.T) {
	good := []string{"OL45804W", "OL23919A", "OL1168083W", "2680", "1342", "/works/OL45804W", "books/2680"}
	for _, k := range good {
		if !validWorkKey(k) {
			t.Errorf("validWorkKey(%q) = false, want true", k)
		}
	}
	bad := []string{"__printing_press_invalid__", "", "not a key", "Meditations", "OL", "http://x"}
	for _, k := range bad {
		if validWorkKey(k) {
			t.Errorf("validWorkKey(%q) = true, want false", k)
		}
	}
}

func TestAtoiOrShared(t *testing.T) {
	if atoiOr("", 20) != 20 || atoiOr("7", 20) != 7 || atoiOr("x", 20) != 20 {
		t.Error("atoiOr behaved unexpectedly")
	}
}
