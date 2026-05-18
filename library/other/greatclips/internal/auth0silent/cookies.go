// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

package auth0silent

import "strings"

// CookieHeader formats a cookie name->value map as a Cookie request
// header value (e.g. "auth0=abc; did=xyz"). Cross-platform; used by
// silent.go regardless of whether ExtractAuth0Cookies has a real
// implementation on this platform.
func CookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for name, value := range cookies {
		parts = append(parts, name+"="+value)
	}
	return strings.Join(parts, "; ")
}
