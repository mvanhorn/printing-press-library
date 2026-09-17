// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "strings"

func normalizePhoneIdentity(value string) (string, bool) {
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	normalized := digits.String()
	if len(normalized) == 11 && normalized[0] == '1' {
		normalized = normalized[1:]
	}
	return normalized, len(normalized) >= 10
}
