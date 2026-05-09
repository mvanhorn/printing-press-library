// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import "fmt"

// fmtSscanInt2 is the actual Sscanf wrapper. Split into a separate file so
// the import of fmt is isolated and other helper files don't need it.
func fmtSscanInt2(s string, dst *int) (int, error) {
	return fmt.Sscanf(s, "%d", dst)
}
