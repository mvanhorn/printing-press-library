// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built Suno-specific config extensions. Kept in a separate file from the
// generated config.go so a regenerate of config.go does not clobber these
// fields. The generated Config struct is extended with the Clerk session
// material needed to re-mint JWTs (the __client cookie, the resolved session
// id, and the device id) — these are added to config.go's struct directly
// because Go cannot add toml-tagged fields to a struct from another file, but
// every accessor and the Suno-specific persistence helper live here.

package config

import "strings"

// DefaultDeviceID is the zero-UUID used as the Device-Id when no browser
// ajs_anonymous_id was found.
const DefaultDeviceID = "00000000-0000-0000-0000-000000000000"

// ClerkClientCookie returns the stored raw __client cookie value, or "".
func (c *Config) ClerkClientCookie() string {
	if c == nil {
		return ""
	}
	return c.ClerkClientCookieVal
}

// ClerkSessionID returns the stored Clerk session id, or "".
func (c *Config) ClerkSessionID() string {
	if c == nil {
		return ""
	}
	return c.ClerkSessionIDVal
}

// StudioCookieHeader returns the cached studio-api Cookie header, or "".
func (c *Config) StudioCookieHeader() string {
	if c == nil {
		return ""
	}
	return c.StudioCookieHeaderVal
}

// IsEnvAuth reports whether auth came from an environment variable
// (SUNO_TOKEN / SUNO_JWT). Env tokens are not ours to manage, so the session
// cache does not persist cookies for them.
func (c *Config) IsEnvAuth() bool {
	return c != nil && strings.HasPrefix(c.AuthSource, "env:")
}

// DeviceID returns the stored device id, falling back to the zero UUID.
func (c *Config) DeviceID() string {
	if c == nil || strings.TrimSpace(c.DeviceIDVal) == "" {
		return DefaultDeviceID
	}
	return c.DeviceIDVal
}

// DeviceIDFor loads the config at configPath and returns its device id,
// falling back to the zero UUID on any load error.
func DeviceIDFor(configPath string) string {
	cfg, err := Load(configPath)
	if err != nil {
		return DefaultDeviceID
	}
	return cfg.DeviceID()
}

// SaveStudioSession persists the studio Cookie header and the JWT expiry as the
// pair that matches the current SunoJwt. Called by auth.EnsureFreshSession after
// it (re)captures cookies from the browser.
func (c *Config) SaveStudioSession(cookieHeader string, jwtExpiry int64) error {
	c.StudioCookieHeaderVal = cookieHeader
	c.SunoJwtExpiry = jwtExpiry
	return c.save()
}

// SaveSunoSession persists the full Clerk-derived session: the minted JWT, the
// raw __client cookie, the resolved session id, and the device id. Reuses the
// generated save() mechanism so the on-disk format stays consistent. Any field
// passed empty is left untouched so callers can update a subset (e.g. refresh
// only the JWT) without wiping the rest.
func (c *Config) SaveSunoSession(jwt, clientCookie, sessionID, deviceID string) error {
	if jwt != "" {
		c.SunoJwt = jwt
	}
	if clientCookie != "" {
		c.ClerkClientCookieVal = clientCookie
	}
	if sessionID != "" {
		c.ClerkSessionIDVal = sessionID
	}
	if deviceID != "" {
		c.DeviceIDVal = deviceID
	}
	// A freshly minted JWT must win over any legacy auth_header that would
	// otherwise shadow it in AuthHeader().
	c.AuthHeaderVal = ""
	return c.save()
}

// SaveSunoJWTOnly persists a directly-supplied JWT and clears any stale Clerk
// session material so a later EnsureFreshJWT does not try to re-mint against a
// cookie that no longer matches the supplied token.
func (c *Config) SaveSunoJWTOnly(jwt string) error {
	c.SunoJwt = jwt
	c.ClerkClientCookieVal = ""
	c.ClerkSessionIDVal = ""
	c.AuthHeaderVal = ""
	c.StudioCookieHeaderVal = ""
	c.SunoJwtExpiry = 0
	return c.save()
}
