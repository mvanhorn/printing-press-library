// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored helper for Peerspace env handshake soft-fail.

package cli

import "strings"

func isHandshakeRequired(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "handshake required") ||
		strings.Contains(s, "handshake_required")
}
