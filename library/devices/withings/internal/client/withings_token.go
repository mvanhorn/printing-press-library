// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Withings OAuth2 token exchange + refresh. Withings' token endpoint is not a
// standard OAuth2 token URL: it is an action-RPC call
// (POST /v2/oauth2 action=requesttoken) returning the tokens wrapped in the
// usual {status, body} envelope, and refresh tokens are SINGLE-USE — each
// refresh returns a new one that must be persisted or the next refresh fails.
// This is why the generic oauth2_refresh codegen path can't be used and the
// flow is hand-authored here.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const withingsTokenPath = "/v2/oauth2" // #nosec G101 -- URL path, not a credential

// WithingsTokenResponse is the body of a successful requesttoken call.
type WithingsTokenResponse struct {
	UserID       json.RawMessage `json:"userid"`
	AccessToken  string          `json:"access_token"`
	RefreshToken string          `json:"refresh_token"`
	ExpiresIn    int             `json:"expires_in"`
	Scope        string          `json:"scope"`
	TokenType    string          `json:"token_type"`
}

// tokenRequest POSTs a form to the Withings token endpoint WITHOUT a bearer
// header and unwraps the envelope. Used by both ExchangeAuthCode and
// RefreshAccessToken.
func (c *Client) tokenRequest(ctx context.Context, form url.Values) (*WithingsTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+withingsTokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "withings-pp-cli/0.1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("token request: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Method: "POST", Path: withingsTokenPath, StatusCode: resp.StatusCode, Body: truncateBody(body)}
	}

	var env struct {
		Status int                   `json:"status"`
		Body   WithingsTokenResponse `json:"body"`
		Error  string                `json:"error"`
	}
	if err := json.Unmarshal(sanitizeJSONResponse(body), &env); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	if env.Status != 0 {
		return nil, &WithingsError{Status: env.Status, Message: withingsStatusMessage(env.Status, env.Error), Path: withingsTokenPath}
	}
	if env.Body.AccessToken == "" {
		return nil, fmt.Errorf("token response had status 0 but no access_token")
	}
	return &env.Body, nil
}

// ExchangeAuthCode exchanges an authorization code for tokens and persists them
// (client id/secret + access + rotated refresh) to the config file.
func (c *Client) ExchangeAuthCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*WithingsTokenResponse, error) {
	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	tok, err := c.tokenRequest(ctx, form)
	if err != nil {
		return nil, err
	}
	if err := c.persistTokens(clientID, clientSecret, tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// RefreshAccessToken uses the stored refresh token to mint a new access token,
// persisting the rotated refresh token. Called automatically by WithingsForm on
// a 401-class status and by `auth refresh`.
func (c *Client) RefreshAccessToken(ctx context.Context) error {
	if c.Config == nil {
		return fmt.Errorf("no config available for refresh")
	}
	clientID := c.Config.ClientID
	clientSecret := c.Config.ClientSecret
	refresh := c.Config.RefreshToken
	if clientID == "" || clientSecret == "" || refresh == "" {
		return fmt.Errorf("cannot refresh: run `withings-pp-cli auth login` first (need client id/secret + refresh token)")
	}
	form := url.Values{}
	form.Set("action", "requesttoken")
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refresh)

	tok, err := c.tokenRequest(ctx, form)
	if err != nil {
		return err
	}
	return c.persistTokens(clientID, clientSecret, tok)
}

// persistTokens writes tokens to config and updates the in-memory config so an
// in-flight retry uses the fresh access token immediately.
func (c *Client) persistTokens(clientID, clientSecret string, tok *WithingsTokenResponse) error {
	expiry := time.Time{}
	if tok.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	// Withings refresh tokens are single-use: the exchange consumed the old one
	// regardless of the response. An empty refresh_token is a malformed/partial
	// response, not a "keep the old one" signal — persisting the consumed token
	// would make the next refresh fail with an opaque invalid-token error, so
	// surface it now.
	if tok.RefreshToken == "" {
		return fmt.Errorf("token response had no refresh_token; the single-use refresh token was consumed by this request — re-run `withings-pp-cli auth login`")
	}
	refresh := tok.RefreshToken
	if err := c.Config.SaveTokens(clientID, clientSecret, tok.AccessToken, refresh, expiry); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}
	return nil
}

// TokenExpiringSoon reports whether the stored access token is expired or within
// the given window of expiry (so callers can proactively refresh).
func (c *Client) TokenExpiringSoon(within time.Duration) bool {
	if c.Config == nil || c.Config.TokenExpiry.IsZero() {
		return false
	}
	return time.Now().Add(within).After(c.Config.TokenExpiry)
}
