// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import "regexp"

// clipIDRE matches a Suno clip id (a UUID, e.g. 550e8400-e29b-41d4-a716-446655440000).
var clipIDRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// looksLikeClipID reports whether s has the shape of a Suno clip id. Single-id
// read commands use it to reject obviously-malformed input with a usage error
// instead of a pointless API round-trip — some endpoints (attribution, parent)
// return 200 with empty data for unknown ids, which would otherwise mask a typo.
func looksLikeClipID(s string) bool {
	return clipIDRE.MatchString(s)
}
