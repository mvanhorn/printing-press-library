// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !kooky

package auth

import (
	"errors"
	"testing"
)

// TestExtractFromChrome_StubIsUnavailable pins the default-build contract: the
// Chrome path is not compiled in, so it returns ErrChromeExtractUnavailable with
// guidance rather than touching Chrome or the Keychain. This is what keeps the
// hermetic test suite passing on every platform with no Keychain access.
func TestExtractFromChrome_StubIsUnavailable(t *testing.T) {
	_, err := ExtractFromChrome()
	if !errors.Is(err, ErrChromeExtractUnavailable) {
		t.Fatalf("default build: ExtractFromChrome should return ErrChromeExtractUnavailable, got %v", err)
	}
}
