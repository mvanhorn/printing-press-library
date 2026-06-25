// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import "strings"

// NormalizeSlug converts a user-facing make/model string into the slug form
// AutoTempest's query params expect: lowercase, with every character that is
// not a-z or 0-9 removed. AutoTempest strips hyphens and spaces from its model
// slugs (F-150 -> "f150", CR-V -> "crv", "Alfa Romeo" -> "alfaromeo"), so the
// raw user text would otherwise miss the catalog and return zero results.
//
// Apply this ONLY to the make/model query-param values. Keep the original text
// for the keywords param and for the saved-search query display field.
func NormalizeSlug(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
