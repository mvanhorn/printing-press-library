// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) composed-auth persistence for WeWork's 3-value
// credential set (bearer token + account uuid + member type). Lives in package
// config so it can drive the same credential-file save path set-token uses.

package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// ApplyWeworkAuthBootstrap applies the WeWork-specific environment contract
// without changing the generated Config loader. A persisted rotating token
// family always wins over stale bootstrap variables; otherwise
// WEWORK_REFRESH_TOKEN completes the existing generated env surface.
func (c *Config) ApplyWeworkAuthBootstrap() {
	if c == nil {
		return
	}
	if c.fileConfig != nil && c.fileConfig.RefreshToken != "" {
		c.RefreshToken = c.fileConfig.RefreshToken
		if c.fileConfig.WeworkToken != "" {
			c.WeworkToken = c.fileConfig.WeworkToken
		}
		delete(c.envOverrides, "RefreshToken")
		delete(c.envOverrides, "WeworkToken")
		c.AuthSource = "config"
		c.CredentialSource = "credentials file"
		return
	}
	if value := os.Getenv("WEWORK_REFRESH_TOKEN"); value != "" && c.RefreshToken == "" {
		c.RefreshToken = value
		c.markEnvOverride("RefreshToken")
		if c.AuthSource == "" {
			c.AuthSource = "env:WEWORK_REFRESH_TOKEN"
			c.CredentialSource = "env:WEWORK_REFRESH_TOKEN"
		}
	}
}

// SaveComposedAuth persists the legacy three-value bundle. New callers should
// use SaveWeworkAuth so a renewable session can be committed atomically.
func (c *Config) SaveComposedAuth(token, uuid, memberType string) error {
	return c.SaveWeworkAuth(token, "", uuid, memberType)
}

// SaveWeworkAuth persists WeWork's complete session bundle atomically. Empty
// arguments are left unchanged so callers can update a subset without
// clobbering a previously captured refresh token, uuid, or member type.
func (c *Config) SaveWeworkAuth(token, refreshToken, uuid, memberType string) error {
	if token != "" {
		// Clear any legacy auth_header so AuthHeader() uses the saved token
		// (mirrors set-token's shadow-clear).
		c.AuthHeaderVal = ""
		c.WeworkToken = token
		delete(c.envOverrides, "WeworkToken")
		c.updateFileConfigField("WeworkToken")
		if expiry := JWTExpiry(token); !expiry.IsZero() {
			c.TokenExpiry = expiry
			delete(c.envOverrides, "TokenExpiry")
			c.updateFileConfigField("TokenExpiry")
		}
	}
	if refreshToken != "" {
		c.RefreshToken = refreshToken
		delete(c.envOverrides, "RefreshToken")
		c.updateFileConfigField("RefreshToken")
	}
	if uuid != "" {
		c.WeworkUuid = uuid
		delete(c.envOverrides, "WeworkUuid")
		c.updateFileConfigField("WeworkUuid")
	}
	if memberType != "" {
		c.WeworkMemberType = memberType
		delete(c.envOverrides, "WeworkMemberType")
		c.updateFileConfigField("WeworkMemberType")
	}
	if err := c.saveCredentialsFirst(); err != nil {
		return err
	}
	return c.save()
}

// ComposedAuthStatus reports which of the three composed-auth values are present
// and, when the bearer token is a JWT, its expiry (zero time if not decodable).
func (c *Config) ComposedAuthStatus() (hasToken, hasUUID, hasMember bool, expiry time.Time) {
	tok := c.WeworkToken
	if tok == "" {
		tok = c.AccessToken
	}
	hasToken = tok != "" || c.AuthHeaderVal != ""
	hasUUID = c.WeworkUuid != ""
	hasMember = c.WeworkMemberType != ""
	expiry = JWTExpiry(tok)
	return
}

// JWTExpiry decodes a JWT's `exp` claim without verifying the signature and
// returns it as a time. Returns the zero time if the token is empty or not a
// decodable JWT (e.g. an opaque token).
func JWTExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some encoders include padding; tolerate it.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
