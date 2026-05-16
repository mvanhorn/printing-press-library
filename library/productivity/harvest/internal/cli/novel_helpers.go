// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
)

// typedExit is an error that signals cobra to exit with a specific non-zero
// code (e.g. 2 = "budget threshold crossed", 3 = "drift detected"). The runner
// reads ExitCode via errors.As. The message goes to stderr.
type typedExit struct {
	code int
	msg  string
}

func (e *typedExit) Error() string { return e.msg }
func (e *typedExit) ExitCode() int { return e.code }

// stringContainsFold reports whether substr (case-insensitive, with simple
// trimming) is contained within s.
func stringContainsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(s)), strings.ToLower(strings.TrimSpace(substr)))
}

// formatHoursDecimal formats hours to two decimals, dropping trailing zeros.
func formatHoursDecimal(h float64) string {
	out := fmt.Sprintf("%.2f", h)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	return out
}
