// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"fmt"
	"strings"
)

// HumanBytes renders a byte count as a short human-readable string using
// the binary (1024-based) convention, e.g. 2214592512 -> "2.06 GB".
func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	units := [...]string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.2f %s", float64(n)/float64(div), units[exp])
}

// isTerminalControl reports whether r is a character SanitizeTerminal
// must strip: C0 controls (below 0x20, except tab), DEL (0x7F), and the
// C1 range (0x80–0x9F, which includes the single-byte CSI U+009B that
// some terminals honor as an escape introducer).
func isTerminalControl(r rune) bool {
	return (r < 0x20 && r != '\t') || (r >= 0x7f && r <= 0x9f)
}

// SanitizeTerminal strips terminal control characters (see
// isTerminalControl) from s so remote-controlled strings — repo paths,
// asset names, error bodies — cannot smuggle escape sequences or
// carriage-return overwrites into a human's terminal. Use it on
// HUMAN-mode prints only; JSON output is already safe because
// encoding/json escapes control characters.
func SanitizeTerminal(s string) string {
	if !strings.ContainsFunc(s, isTerminalControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isTerminalControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
