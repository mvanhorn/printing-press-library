// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (markerless) auth0 refresh-token support: lets the CLI mint a
// fresh access token from a stored refresh token (grant_type=refresh_token)
// against the token's own issuer, so an agent runtime can run autonomously
// after a one-time seed. Verified live: WeWork's SPA client accepts the
// public-client refresh grant with rotating refresh tokens.

package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SaveWeworkSession persists a fresh access token (+ rotated refresh token and
// expiry) to the credentials file.
func (c *Config) SaveWeworkSession(accessToken, refreshToken string, expiry time.Time) error {
	if accessToken != "" {
		c.AuthHeaderVal = ""
		c.WeworkToken = accessToken
		delete(c.envOverrides, "WeworkToken")
		c.updateFileConfigField("WeworkToken")
	}
	if refreshToken != "" {
		c.RefreshToken = refreshToken
		delete(c.envOverrides, "RefreshToken")
		c.updateFileConfigField("RefreshToken")
	}
	if !expiry.IsZero() {
		c.TokenExpiry = expiry
		delete(c.envOverrides, "TokenExpiry")
		c.updateFileConfigField("TokenExpiry")
	}
	if err := c.saveCredentialsFirst(); err != nil {
		return err
	}
	return c.save()
}

// jwtIssAzp decodes a JWT's `iss` (issuer) and `azp` (client_id) claims without
// verifying the signature.
// isWeworkIssuer reports whether an auth0 issuer URL is a WeWork host — i.e. its
// hostname is exactly wework.com or a subdomain of it, over https. Guards the
// refresh flow against sending the refresh token to a non-WeWork endpoint.
func isWeworkIssuer(iss string) bool {
	u, err := url.Parse(strings.TrimSpace(iss))
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "wework.com" || strings.HasSuffix(host, ".wework.com")
}

func jwtIssAzp(token string) (iss, azp string) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		if payload, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return "", ""
		}
	}
	var claims struct {
		Iss string `json:"iss"`
		Azp string `json:"azp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", ""
	}
	return claims.Iss, claims.Azp
}

// ValidateWeworkRenewableAccessToken proves that an imported access token has
// the trusted issuer and public-client metadata needed before the CLI accepts
// ownership of a rotating refresh-token chain. It does not expose token data.
func ValidateWeworkRenewableAccessToken(token string) error {
	iss, azp := jwtIssAzp(token)
	if azp == "" || !isWeworkIssuer(iss) {
		return fmt.Errorf("access token lacks a trusted WeWork issuer and client id")
	}
	return nil
}

// WeworkRefreshStatus reports whether a refresh token is present and whether
// the current access token contains enough trusted WeWork Auth0 metadata to use
// it. It never returns either secret.
func (c *Config) WeworkRefreshStatus() (hasRefresh, renewable bool) {
	if c == nil {
		return false, false
	}
	hasRefresh = c.RefreshToken != ""
	if !hasRefresh {
		return false, false
	}
	tok := c.WeworkToken
	if tok == "" {
		tok = c.AccessToken
	}
	iss, azp := jwtIssAzp(tok)
	return true, azp != "" && isWeworkIssuer(iss)
}

// RefreshDoer is the HTTP round-trip used for refresh; overridable in tests.
type RefreshDoer func(*http.Request) (*http.Response, error)

// RefreshWeworkTokenIfNeeded refreshes the access token when it is expired (or
// within the leeway window) and a refresh token is stored. Returns whether a
// refresh happened. The token endpoint and client_id are derived from the
// current token's `iss`/`azp` claims (auth0 convention: <iss>/oauth/token).
func (c *Config) RefreshWeworkTokenIfNeeded(doer RefreshDoer) (bool, error) {
	return c.refreshWeworkToken(doer, false)
}

// RefreshWeworkTokenNow exchanges the current refresh token even when the
// access token has not reached the normal refresh window.
func (c *Config) RefreshWeworkTokenNow(doer RefreshDoer) (bool, error) {
	return c.refreshWeworkToken(doer, true)
}

func (c *Config) refreshWeworkToken(doer RefreshDoer, force bool) (bool, error) {
	if !c.weworkRefreshNeeded(force) {
		return false, nil
	}
	release, err := c.acquireWeworkRefreshLock()
	if err != nil {
		return false, err
	}
	defer release()
	if err := c.reloadWeworkRotatingCredentials(); err != nil {
		return false, err
	}
	// A process that held the lock before us may already have persisted a fresh
	// access token. Re-check after reload so the rotating token is exchanged once.
	if !c.weworkRefreshNeeded(force) {
		return false, nil
	}
	return c.refreshWeworkTokenLocked(doer)
}

func (c *Config) weworkRefreshNeeded(force bool) bool {
	if c == nil {
		return false
	}
	tok := c.WeworkToken
	if tok == "" {
		tok = c.AccessToken
	}
	if tok == "" || c.RefreshToken == "" {
		return false
	}
	exp := JWTExpiry(tok)
	// Refresh a bit before expiry so a command never runs with a token that
	// expires mid-flight. If the token has no decodable expiry, refresh.
	if !force && !exp.IsZero() && time.Until(exp) > 90*time.Second {
		return false
	}
	return true
}

func (c *Config) refreshWeworkTokenLocked(doer RefreshDoer) (bool, error) {
	tok := c.WeworkToken
	if tok == "" {
		tok = c.AccessToken
	}
	iss, azp := jwtIssAzp(tok)
	if iss == "" || azp == "" {
		return false, nil
	}
	// Defense-in-depth: the refresh token is a long-lived secret, and the issuer
	// host comes from the (signature-unverified) token we hold. Never POST the
	// refresh token to a host that isn't WeWork's, so a planted/malicious token
	// can't redirect the secret to an attacker-controlled endpoint.
	if !isWeworkIssuer(iss) {
		return false, fmt.Errorf("refusing to refresh: token issuer %q is not a WeWork host", iss)
	}
	tokenURL := strings.TrimRight(iss, "/") + "/oauth/token"
	access, refresh, expiresIn, err := doAuth0Refresh(tokenURL, azp, c.RefreshToken, doer)
	if err != nil {
		return false, err
	}
	newExp := JWTExpiry(access)
	if newExp.IsZero() && expiresIn > 0 {
		newExp = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	if refresh == "" {
		refresh = c.RefreshToken // some responses omit rotation
	}
	return true, c.SaveWeworkSession(access, refresh, newExp)
}

// doAuth0Refresh performs the auth0 public-client refresh grant and returns the
// new access token, rotated refresh token, and expires_in. Pure (no disk), so
// it is unit-testable with a mock RefreshDoer.
func doAuth0Refresh(tokenURL, clientID, refreshToken string, doer RefreshDoer) (access, refresh string, expiresIn int64, err error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", 0, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")
	if doer == nil {
		doer = http.DefaultClient.Do
	}
	resp, err := doer(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", 0, fmt.Errorf("decoding refresh response: %w", err)
	}
	if out.AccessToken == "" {
		msg := out.Error
		if out.ErrorDesc != "" {
			msg += ": " + out.ErrorDesc
		}
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return "", "", 0, fmt.Errorf("refresh rejected (%s)", msg)
	}
	return out.AccessToken, out.RefreshToken, out.ExpiresIn, nil
}
