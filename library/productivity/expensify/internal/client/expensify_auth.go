// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Expensify session-token minting from device (partner) credentials. New
// Expensify (NewDot) authenticates by holding a long-lived partnerUserID +
// partnerUserSecret pair and exchanging it for a short-lived session authToken
// via the Authenticate command. The www.expensify.com authToken COOKIE that
// `auth login --from-chrome` reads is a stale classic-session token for
// NewDot-primary users, so this gives the CLI the same self-minting path the
// app uses — and lets the client auto-refresh when a session expires
// (jsonCode 407). Recorded in .printing-press-patches.json as
// `auth-device-mint`.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Public chat-expensify-com partner identity used by the New Expensify web/app
// client. partnerPassword is a public client identifier (it ships in the
// open-source Expensify App), not a secret; both are overridable via env for
// alternate partner integrations.
const (
	defaultExpensifyPartnerName     = "chat-expensify-com"
	defaultExpensifyPartnerPassword = "e21965746fd75f82bb66"
)

// PartnerCredentialsAvailable reports whether device (partner) credentials are
// configured, i.e. whether MintSessionToken can run.
func (c *Client) PartnerCredentialsAvailable() bool {
	return c.Config != nil &&
		c.Config.ExpensifyPartnerUserId != "" &&
		c.Config.ExpensifyPartnerUserSecret != ""
}

// MintSessionToken exchanges the configured device (partner) credentials for a
// fresh Expensify session authToken via the Authenticate command. It issues its
// own direct request (not through do()) so it never recurses into the auth /
// auto-refresh path.
func (c *Client) MintSessionToken(ctx context.Context) (string, error) {
	if !c.PartnerCredentialsAvailable() {
		return "", fmt.Errorf("device credentials not set: export EXPENSIFY_PARTNER_USER_ID and EXPENSIFY_PARTNER_USER_SECRET (the partnerUserID/partnerUserSecret pair New Expensify stores under DEVICE_SESSION_CREDENTIALS)")
	}
	partnerName := os.Getenv("EXPENSIFY_PARTNER_NAME")
	if partnerName == "" {
		partnerName = defaultExpensifyPartnerName
	}
	partnerPassword := os.Getenv("EXPENSIFY_PARTNER_PASSWORD")
	if partnerPassword == "" {
		partnerPassword = defaultExpensifyPartnerPassword
	}

	form := url.Values{}
	form.Set("partnerName", partnerName)
	form.Set("partnerPassword", partnerPassword)
	form.Set("partnerUserID", c.Config.ExpensifyPartnerUserId)
	form.Set("partnerUserSecret", c.Config.ExpensifyPartnerUserSecret)
	form.Set("doNotRetry", "true")
	form.Set("platform", "web")
	form.Set("referer", "ecash")
	form.Set("api_setCookie", "false")

	targetURL := strings.TrimRight(c.BaseURL, "/") + "/Authenticate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf("Authenticate request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading Authenticate response: %w", err)
	}
	var parsed struct {
		JSONCode  float64 `json:"jsonCode"`
		Message   string  `json:"message"`
		AuthToken string  `json:"authToken"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing Authenticate response: %w", err)
	}
	if int(parsed.JSONCode) != 200 || parsed.AuthToken == "" {
		msg := parsed.Message
		if msg == "" {
			msg = fmt.Sprintf("jsonCode %d", int(parsed.JSONCode))
		}
		return "", fmt.Errorf("Authenticate rejected the device credentials: %s", msg)
	}
	return parsed.AuthToken, nil
}

// RefreshSessionToken mints a fresh authToken from device credentials, stores it
// on the client config, and persists it. Used by auth login --device and the
// client's jsonCode-407 auto-refresh.
func (c *Client) RefreshSessionToken(ctx context.Context) (string, error) {
	tok, err := c.MintSessionToken(ctx)
	if err != nil {
		return "", err
	}
	c.Config.ExpensifyAuthToken = tok
	if err := c.Config.SaveCredential(tok); err != nil {
		// The in-memory token is still usable for the current process even if
		// persistence fails; surface the write error without discarding it.
		return tok, fmt.Errorf("minted a fresh token but could not persist it: %w", err)
	}
	return tok, nil
}
