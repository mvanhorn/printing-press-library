// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored cookie-auth config surface (not generator output). Kept in a
// separate file so a fresh print preserves it via regen-merge.

package config

// SaveHeaders persists the Headers map (used by the hand-authored cookie-auth
// `auth import-cookies` flow). Thin wrapper over the generated save().
func (c *Config) SaveHeaders() error {
	return c.save()
}
