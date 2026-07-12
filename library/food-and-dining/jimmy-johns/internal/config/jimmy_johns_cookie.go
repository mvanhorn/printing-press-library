// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored cookie-auth config surface (not generator output). Kept in a
// separate file so a fresh print preserves it via regen-merge.

package config

// SaveHeaders persists the Headers map (used by the hand-authored cookie-auth
// `auth import-cookies` flow). Thin wrapper over the generated save().
func (c *Config) SaveHeaders() error {
	return c.save()
}

// ClearSessionHeaders drops any persisted session-carrying request headers — the
// raw "Cookie" header saved by `auth import-cookies` — and persists the change so
// `auth logout` fully revokes an imported-cookie session. Without it, logout
// clears the token fields and cookie jar but the saved Cookie header still rides
// every later request. No-op when no session header is set.
func (c *Config) ClearSessionHeaders() error {
	if c.Headers == nil {
		return nil
	}
	if _, ok := c.Headers["Cookie"]; !ok {
		return nil
	}
	delete(c.Headers, "Cookie")
	return c.save()
}
