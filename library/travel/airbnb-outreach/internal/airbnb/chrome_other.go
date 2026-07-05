// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

//go:build !windows

package airbnb

import "fmt"

// importChromeCookies is Windows-only for now. On macOS/Linux the Chrome cookie
// store is protected by the OS keychain (Keychain / kwallet / libsecret) with a
// different decryption path. Use `auth login --cookies "<paste>"` on those
// platforms.
func importChromeCookies(profile string) (map[string]string, error) {
	return nil, fmt.Errorf("auth login --chrome is currently supported on Windows only; use `auth login --cookies \"<cookie header>\"` on this OS")
}
