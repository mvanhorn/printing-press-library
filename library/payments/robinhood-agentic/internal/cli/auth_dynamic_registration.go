// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written RFC 7591 dynamic client registration. Robinhood's Agentic MCP is
// an OAuth resource with NO pre-issued client_id — the standard generated auth
// flow assumes one exists, but here the client must self-register first. This
// registers a public client (no secret), persists the id via the normal token
// save, and the PKCE authorization-code flow proceeds as usual.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	robinhoodRegistrationURL = "https://agent.robinhood.com/oauth/trading/register"
	// robinhoodResource is the RFC 8707 resource indicator: the MCP endpoint the
	// issued token is audience-scoped to.
	robinhoodResource = "https://agent.robinhood.com/mcp/trading"
	robinhoodScope    = "internal"
)

// registerRobinhoodClient performs RFC 7591 dynamic client registration and
// returns the issued public client_id. The registration endpoint is
// unauthenticated and mints a public client (token_endpoint_auth_method "none"),
// so no secret is ever stored.
func registerRobinhoodClient(ctx context.Context, registrationURL, redirectURI string) (string, error) {
	payload := map[string]any{
		"client_name":                "robinhood-agentic-pp-cli",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      robinhoodScope,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("registration endpoint returned HTTP %d", resp.StatusCode)
	}
	var reg struct {
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return "", fmt.Errorf("decoding registration response: %w", err)
	}
	if reg.ClientID == "" {
		return "", fmt.Errorf("registration response had no client_id")
	}
	return reg.ClientID, nil
}
