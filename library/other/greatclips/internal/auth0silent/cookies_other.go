// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !darwin

package auth0silent

import "errors"

// ExtractAuth0Cookies returns an unsupported error on non-Darwin
// platforms. v0.3 targets macOS only; cross-platform Chrome cookie
// extraction is deferred to a follow-up plan.
func ExtractAuth0Cookies() (map[string]string, error) {
	return nil, errors.New("auth0silent.ExtractAuth0Cookies: only supported on macOS (darwin) in v0.3; use `auth set-token` to paste a JWT manually")
}
