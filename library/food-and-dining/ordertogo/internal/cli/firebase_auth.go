// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written: ordertogo.com authenticates writes (postmicmeshorder) with a
// short-lived Firebase ID token in the Authorization header, NOT cookies. The
// token expires hourly, so the CLI mints a fresh one from the long-lived
// refresh token + Firebase web API key on each order.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
)

const secureTokenEndpoint = "https://securetoken.googleapis.com/v1/token"

// firebaseAuthToken returns a bearer-ready Firebase ID token for the order POST.
// Precedence: an explicit override (env/config auth_header style), then a fresh
// mint from the stored refresh token. Returns "" with a nil error when no
// Firebase credentials are configured so the caller can fall back / error
// clearly.
func firebaseAuthToken(cfg *config.Config) (string, error) {
	if cfg.FirebaseRefreshToken == "" || cfg.FirebaseAPIKey == "" {
		return "", nil
	}
	return mintFirebaseIDToken(cfg.FirebaseAPIKey, cfg.FirebaseRefreshToken)
}

// mintFirebaseIDToken exchanges a refresh token for a fresh ID token via
// Google's Secure Token service.
func mintFirebaseIDToken(apiKey, refreshToken string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequest(http.MethodPost, secureTokenEndpoint+"?key="+url.QueryEscape(apiKey), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("minting firebase token: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		IDToken string `json:"id_token"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding firebase token response: %w", err)
	}
	if out.IDToken == "" {
		msg := out.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("firebase token refresh failed: %s (re-run `auth login` to refresh credentials)", msg)
	}
	return out.IDToken, nil
}
