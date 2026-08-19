// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Clay uses two credentials for two APIs behind one host.
// CLAY_API_KEY authenticates /public/v0 only; /v3 needs the stored browser
// session. This reports whether such a session exists on disk.

package config

import "strings"

// BrowserSessionStored reports whether a browser-session credential (the Clay
// `claysession` cookie) has been captured via `auth login`.
func (c *Config) BrowserSessionStored() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.AccessToken) != "" && strings.TrimSpace(c.CredentialDomain) != ""
}
