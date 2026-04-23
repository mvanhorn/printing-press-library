// Copyright 2026 adrian-horning. Licensed under Apache-2.0. See LICENSE.

package cli

import "strings"

// NormalizeHandle strips a single leading '@' and trims surrounding whitespace.
// Accepts '@charlidamelio' and 'charlidamelio' identically so users do not have
// to remember which is expected. Idempotent.
func NormalizeHandle(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "@")
}

// NormalizeHashtag strips a single leading '#' and trims surrounding whitespace.
// Accepts '#fyp' and 'fyp' identically. Idempotent.
func NormalizeHashtag(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimPrefix(s, "#")
}
